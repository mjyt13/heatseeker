// Package discussions implements threads and messages: a discussion per
// subject, one for the whole group, and one per material or task
// (docs/PLAN.md §7.1, D11, D40).
package discussions

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
)

// Deps are the collaborators of the service.
type Deps struct {
	Discussions domain.DiscussionRepo
	Subjects    domain.SubjectRepo
	Materials   domain.MaterialRepo
	Tasks       domain.TaskRepo
	Access      *access.Service
	Events      *events.Publisher
	Tx          domain.TxManager
	Clock       clock.Clock
	Log         *slog.Logger
}

// Service is the discussions use-case layer.
type Service struct{ Deps }

// NewService wires the service.
func NewService(d Deps) *Service { return &Service{Deps: d} }

// Target addresses a discussion: clients never need a thread id, because a
// thread exists only after its first message.
type Target struct {
	Type domain.ThreadTarget
	ID   uuid.UUID
}

// PostInput is a new message.
type PostInput struct {
	// ClientID makes sending idempotent: the offline outbox repeats it.
	ClientID  uuid.UUID
	Body      string
	ReplyToID *uuid.UUID
}

// Page is a slice of a thread in thread order.
type Page struct {
	// Thread is nil while nobody has written in the discussion.
	Thread   *domain.Thread
	Messages []domain.MessageView
	// HasMore reports more messages in the direction that was read.
	HasMore bool
	// LastReadSeq is the reader's mark before this call.
	LastReadSeq int64
}

// ListInput pages a thread. Neither cursor means the newest page.
type ListInput struct {
	BeforeSeq *int64
	AfterSeq  *int64
	Limit     int32
	// IncludeHidden returns the text of messages the reader hid.
	IncludeHidden bool
}

// Overview is the "Discussions" screen: the group thread, one row per
// subject and the material and task threads people have written in.
type Overview struct {
	General  Entry
	Subjects []SubjectEntry
	Others   []domain.ThreadSummary
	Unread   int32
}

// Entry is one discussion with the reader's unread count; Thread is nil
// while the discussion is empty.
type Entry struct {
	Target Target
	Thread *domain.ThreadSummary
}

// SubjectEntry is a subject's own discussion plus unread messages in the
// discussions of its materials and tasks.
type SubjectEntry struct {
	Entry
	Subject      domain.Subject
	NestedUnread int32
}

const (
	maxBodyRunes = 4000
	defaultLimit = 50
	maxLimit     = 200
	hiddenLimit  = 200
)

// Overview lists the group's discussions for the reader.
func (s *Service) Overview(ctx context.Context, actorID, groupID uuid.UUID) (*Overview, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	threads, err := s.Discussions.ListThreads(ctx, groupID, actorID)
	if err != nil {
		return nil, err
	}
	subjects, err := s.Subjects.List(ctx, groupID, false)
	if err != nil {
		return nil, err
	}
	out := &Overview{
		General:  Entry{Target: Target{Type: domain.ThreadGeneral, ID: groupID}},
		Subjects: make([]SubjectEntry, len(subjects)),
		Others:   []domain.ThreadSummary{},
	}
	bySubject := make(map[uuid.UUID]int, len(subjects))
	for i, sub := range subjects {
		out.Subjects[i] = SubjectEntry{Entry: Entry{Target: Target{Type: domain.ThreadSubject, ID: sub.ID}}, Subject: sub}
		bySubject[sub.ID] = i
	}
	for i := range threads {
		t := &threads[i]
		out.Unread += t.Unread
		switch t.TargetType {
		case domain.ThreadGeneral:
			out.General.Thread = t
		case domain.ThreadSubject:
			if idx, ok := bySubject[t.TargetID]; ok {
				out.Subjects[idx].Thread = t
			} else {
				// An archived subject: its discussion is still reachable.
				out.Others = append(out.Others, *t)
			}
		default:
			if t.SubjectID != nil {
				if idx, ok := bySubject[*t.SubjectID]; ok {
					out.Subjects[idx].NestedUnread += t.Unread
				}
			}
			out.Others = append(out.Others, *t)
		}
	}
	return out, nil
}

// List reads a page of a discussion.
func (s *Service) List(ctx context.Context, actorID, groupID uuid.UUID, target Target, in ListInput) (*Page, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	if _, err := s.resolve(ctx, actor, target); err != nil {
		return nil, err
	}
	if in.BeforeSeq != nil && in.AfterSeq != nil {
		return nil, domain.Invalid("after_seq", "use either before_seq or after_seq")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	page := &Page{Messages: []domain.MessageView{}}
	thread, err := s.Discussions.GetThread(ctx, groupID, target.Type, target.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return page, nil
	case err != nil:
		return nil, err
	}
	page.Thread = thread
	if page.LastReadSeq, err = s.Discussions.ReadSeq(ctx, thread.ID, actorID); err != nil {
		return nil, err
	}
	msgs, err := s.Discussions.ListMessages(ctx, domain.MessageFilter{
		ThreadID: thread.ID, ViewerID: actorID, BeforeSeq: in.BeforeSeq, AfterSeq: in.AfterSeq, Limit: limit + 1,
	})
	if err != nil {
		return nil, err
	}
	if int32(len(msgs)) > limit {
		page.HasMore = true
		if in.AfterSeq != nil {
			msgs = msgs[:limit]
		} else {
			msgs = msgs[1:]
		}
	}
	for i := range msgs {
		mask(&msgs[i], actor, in.IncludeHidden)
	}
	page.Messages = msgs
	return page, nil
}

// Post writes a message, creating the thread with its first message.
func (s *Service) Post(ctx context.Context, actorID, groupID uuid.UUID, target Target, in PostInput) (*domain.MessageView, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.ThreadWrite); err != nil {
		return nil, err
	}
	if in.ClientID == uuid.Nil {
		return nil, domain.Invalid("client_id", "is required")
	}
	resolved, err := s.resolve(ctx, actor, target)
	if err != nil {
		return nil, err
	}
	// A repeated request returns the message it already created.
	if thread, err := s.Discussions.GetThread(ctx, groupID, target.Type, target.ID); err == nil {
		switch existing, err := s.Discussions.GetMessageByClientID(ctx, thread.ID, in.ClientID); {
		case err == nil:
			return s.view(ctx, actor, existing.ID)
		case !errors.Is(err, domain.ErrNotFound):
			return nil, err
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	body, err := normalizeBody(in.Body)
	if err != nil {
		return nil, err
	}
	var messageID uuid.UUID
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		thread, err := s.Discussions.EnsureThread(ctx, domain.Thread{
			ID: ids.New(), GroupID: groupID, TargetType: target.Type, TargetID: target.ID,
			SubjectID: resolved.subjectID, Title: resolved.title, CreatedBy: &actorID,
		})
		if err != nil {
			return err
		}
		if in.ReplyToID != nil {
			reply, err := s.Discussions.GetMessage(ctx, *in.ReplyToID)
			if errors.Is(err, domain.ErrNotFound) || (err == nil && (reply.ThreadID != thread.ID || reply.DeletedAt != nil)) {
				return domain.Invalid("reply_to_id", "is not a message of this discussion")
			}
			if err != nil {
				return err
			}
		}
		msg := domain.Message{
			ID: ids.New(), GroupID: groupID, ThreadID: thread.ID, AuthorID: &actorID, ClientID: in.ClientID,
			Body: body, ReplyToID: in.ReplyToID,
		}
		ev, err := s.publish(ctx, domain.EventMessageCreated, &actorID, thread, &msg, false)
		if err != nil {
			return err
		}
		msg.Seq = ev.Seq
		created, err := s.Discussions.CreateMessage(ctx, msg)
		if err != nil {
			return err
		}
		if err := s.Discussions.TouchThread(ctx, thread.ID, created.Seq, created.CreatedAt); err != nil {
			return err
		}
		// The author has read what they wrote.
		if err := s.Discussions.MarkRead(ctx, thread.ID, actorID, created.Seq); err != nil {
			return err
		}
		messageID = created.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.view(ctx, actor, messageID)
}

// Edit replaces the text of the actor's own message.
func (s *Service) Edit(ctx context.Context, actorID, messageID uuid.UUID, body string) (*domain.MessageView, error) {
	msg, actor, thread, err := s.load(ctx, actorID, messageID)
	if err != nil {
		return nil, err
	}
	if !isAuthor(msg, actorID) {
		return nil, domain.Forbidden("only the author can edit a message")
	}
	if msg.DeletedAt != nil {
		return nil, domain.NotFound("message")
	}
	if msg.HiddenForAllAt != nil {
		return nil, domain.Conflict("the message was hidden by a moderator")
	}
	body, err = normalizeBody(body)
	if err != nil {
		return nil, err
	}
	if body == msg.Body {
		return s.view(ctx, actor, msg.ID)
	}
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		edited, err := s.Discussions.EditMessage(ctx, msg.ID, body, s.Clock.Now())
		if err != nil {
			return err
		}
		_, err = s.publish(ctx, domain.EventMessageUpdated, &actorID, thread, edited, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.view(ctx, actor, msg.ID)
}

// Delete removes the actor's own message; replies keep a "deleted" stub.
func (s *Service) Delete(ctx context.Context, actorID, messageID uuid.UUID) error {
	msg, _, thread, err := s.load(ctx, actorID, messageID)
	if err != nil {
		return err
	}
	if !isAuthor(msg, actorID) {
		return domain.Forbidden("only the author can delete a message")
	}
	if msg.DeletedAt != nil {
		return nil
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		deleted, err := s.Discussions.SoftDeleteMessage(ctx, msg.ID, s.Clock.Now())
		if err != nil {
			return err
		}
		if err := s.Discussions.AdjustMessageCount(ctx, thread.ID, -1); err != nil {
			return err
		}
		_, err = s.publish(ctx, domain.EventMessageDeleted, &actorID, thread, deleted, false)
		return err
	})
}

// Restore brings back the actor's own deleted message.
func (s *Service) Restore(ctx context.Context, actorID, messageID uuid.UUID) (*domain.MessageView, error) {
	msg, actor, thread, err := s.load(ctx, actorID, messageID)
	if err != nil {
		return nil, err
	}
	if !isAuthor(msg, actorID) {
		return nil, domain.Forbidden("only the author can restore a message")
	}
	if msg.DeletedAt == nil {
		return s.view(ctx, actor, msg.ID)
	}
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		restored, err := s.Discussions.UndeleteMessage(ctx, msg.ID)
		if err != nil {
			return err
		}
		if err := s.Discussions.AdjustMessageCount(ctx, thread.ID, 1); err != nil {
			return err
		}
		_, err = s.publish(ctx, domain.EventMessageUndeleted, &actorID, thread, restored, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.view(ctx, actor, msg.ID)
}

// SetHiddenForAll hides a message for everybody or brings it back
// (moderators and admins). The author and moderators still see the text.
func (s *Service) SetHiddenForAll(ctx context.Context, actorID, messageID uuid.UUID, hidden bool) (*domain.MessageView, error) {
	msg, actor, thread, err := s.load(ctx, actorID, messageID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.MessageModerate); err != nil {
		return nil, err
	}
	if msg.DeletedAt != nil {
		return nil, domain.NotFound("message")
	}
	if hidden == (msg.HiddenForAllAt != nil) {
		return s.view(ctx, actor, msg.ID)
	}
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		by, at, kind := &actorID, ptr(s.Clock.Now()), domain.EventMessageHidden
		if !hidden {
			by, at, kind = nil, nil, domain.EventMessageRestored
		}
		updated, err := s.Discussions.SetHiddenForAll(ctx, msg.ID, by, at)
		if err != nil {
			return err
		}
		// Moderation is visible in the activity feed.
		_, err = s.publish(ctx, kind, &actorID, thread, updated, true)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.view(ctx, actor, msg.ID)
}

// SetHiddenForMe folds a message away for the actor only. It is personal
// and leaves no trace in the group log.
func (s *Service) SetHiddenForMe(ctx context.Context, actorID, messageID uuid.UUID, hidden bool) (*domain.MessageView, error) {
	msg, actor, _, err := s.load(ctx, actorID, messageID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.MessageHideSelf); err != nil {
		return nil, err
	}
	if msg.DeletedAt != nil {
		return nil, domain.NotFound("message")
	}
	if hidden {
		err = s.Discussions.HideMessage(ctx, msg.ID, actorID)
	} else {
		err = s.Discussions.UnhideMessage(ctx, msg.ID, actorID)
	}
	if err != nil {
		return nil, err
	}
	return s.view(ctx, actor, msg.ID)
}

// HiddenByMe lists the messages the actor hid, newest hide first.
func (s *Service) HiddenByMe(ctx context.Context, actorID, groupID uuid.UUID) ([]domain.MessageView, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	msgs, err := s.Discussions.ListHiddenByUser(ctx, groupID, actorID, hiddenLimit)
	if err != nil {
		return nil, err
	}
	for i := range msgs {
		mask(&msgs[i], actor, true)
	}
	return msgs, nil
}

// MarkRead moves the reader's mark in a discussion; seq 0 means "up to the
// newest message". An empty discussion has nothing to mark.
func (s *Service) MarkRead(ctx context.Context, actorID, groupID uuid.UUID, target Target, seq int64) error {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return err
	}
	if seq < 0 {
		return domain.Invalid("seq", "must be >= 0")
	}
	thread, err := s.Discussions.GetThread(ctx, groupID, target.Type, target.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil
	case err != nil:
		return err
	}
	if seq == 0 || seq > thread.LastSeq {
		seq = thread.LastSeq
	}
	return s.Discussions.MarkRead(ctx, thread.ID, actorID, seq)
}

// resolved is what a target contributes to its thread.
type resolved struct {
	subjectID *uuid.UUID
	title     *string
}

// resolve checks that the target exists in the group and may be discussed.
func (s *Service) resolve(ctx context.Context, actor *access.Actor, target Target) (resolved, error) {
	groupID := actor.Group.ID
	switch target.Type {
	case domain.ThreadGeneral:
		if target.ID != groupID {
			return resolved{}, domain.NotFound("thread")
		}
		return resolved{}, nil
	case domain.ThreadSubject:
		sub, err := s.Subjects.Get(ctx, target.ID, groupID)
		if err != nil {
			return resolved{}, err
		}
		return resolved{subjectID: &sub.ID}, nil
	case domain.ThreadMaterial:
		m, err := s.Materials.Get(ctx, target.ID)
		if err != nil {
			return resolved{}, err
		}
		if m.GroupID != groupID || m.Status == domain.MaterialDeleted {
			return resolved{}, domain.NotFound("material")
		}
		if m.TaskID != nil {
			// Its name must not surface in the group's discussion list (D43).
			return resolved{}, domain.Invalid("target_id", "a file kept in a task is discussed in the task")
		}
		return resolved{subjectID: m.SubjectID, title: &m.Title}, nil
	case domain.ThreadTask:
		t, err := s.Tasks.Get(ctx, target.ID)
		if err != nil {
			return resolved{}, err
		}
		if t.GroupID != groupID {
			return resolved{}, domain.NotFound("task")
		}
		if t.Visibility == domain.TaskVisiblePrivate {
			if t.CreatedBy == nil || *t.CreatedBy != actor.User.ID {
				return resolved{}, domain.NotFound("task")
			}
			return resolved{}, domain.Invalid("target_id", "a private task has no discussion")
		}
		return resolved{subjectID: t.SubjectID, title: &t.Title}, nil
	case domain.ThreadLesson, domain.ThreadProposal:
		return resolved{}, domain.Invalid("target_type", "not available yet")
	default:
		return resolved{}, domain.Invalid("target_type", "unknown target "+string(target.Type))
	}
}

// load fetches a message with the actor's membership in its group.
func (s *Service) load(ctx context.Context, actorID, messageID uuid.UUID) (*domain.Message, *access.Actor, *domain.Thread, error) {
	msg, err := s.Discussions.GetMessage(ctx, messageID)
	if err != nil {
		return nil, nil, nil, err
	}
	actor, err := s.Access.Actor(ctx, actorID, msg.GroupID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, nil, nil, err
	}
	thread, err := s.Discussions.GetThreadByID(ctx, msg.ThreadID)
	if err != nil {
		return nil, nil, nil, err
	}
	return msg, actor, thread, nil
}

func (s *Service) view(ctx context.Context, actor *access.Actor, messageID uuid.UUID) (*domain.MessageView, error) {
	v, err := s.Discussions.GetMessageView(ctx, messageID, actor.User.ID)
	if err != nil {
		return nil, err
	}
	mask(v, actor, true)
	return v, nil
}

// publish appends a message event. The payload names the discussion but
// never carries the text (D40): moderation and deletion must not leave it
// in the log.
func (s *Service) publish(ctx context.Context, kind domain.EventKind, actorID *uuid.UUID, thread *domain.Thread, msg *domain.Message, audit bool) (*domain.Event, error) {
	payload := map[string]any{
		"thread_id":   thread.ID,
		"target_type": thread.TargetType,
		"target_id":   thread.TargetID,
		"subject_id":  thread.SubjectID,
		"author_id":   msg.AuthorID,
	}
	if msg.ReplyToID != nil {
		payload["reply_to_id"] = msg.ReplyToID
	}
	if thread.Title != nil {
		payload["title"] = *thread.Title
	}
	e, err := domain.NewEvent(thread.GroupID, kind, actorID, "message", &msg.ID, payload)
	if err != nil {
		return nil, err
	}
	e.Audit = audit
	return s.Events.Publish(ctx, e)
}

// mask blanks out what the reader may not see: deleted text, text hidden by
// a moderator (except for its author and moderators) and, unless asked for,
// text the reader folded away.
func mask(v *domain.MessageView, actor *access.Actor, includeHidden bool) {
	moderator := actor.Can(authz.MessageModerate)
	switch {
	case v.DeletedAt != nil:
		v.Body = ""
	case v.HiddenForAllAt != nil && !moderator && !isAuthor(&v.Message, actor.User.ID):
		v.Body = ""
	case v.HiddenByMe && !includeHidden:
		v.Body = ""
	}
	if !moderator {
		// Who hid the message is for moderators to know.
		v.HiddenForAllBy = nil
	}
	if r := v.Reply; r != nil {
		if r.Deleted || (r.HiddenForAll && !moderator && (r.AuthorID == nil || *r.AuthorID != actor.User.ID)) {
			r.Body = ""
		}
	}
}

func isAuthor(m *domain.Message, userID uuid.UUID) bool {
	return m.AuthorID != nil && *m.AuthorID == userID
}

func normalizeBody(raw string) (string, error) {
	body := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if body == "" {
		return "", domain.Invalid("body", "must not be empty")
	}
	if utf8.RuneCountInString(body) > maxBodyRunes {
		return "", domain.Invalid("body", "must be at most 4000 characters")
	}
	return body, nil
}

func ptr[T any](v T) *T { return &v }
