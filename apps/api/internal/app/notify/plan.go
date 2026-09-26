package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

// notice is one fact and the members it should reach. The text is rendered
// per recipient, because dates are shown in each member's own timezone.
type notice struct {
	Type       domain.NotificationType
	Seq        int64
	DedupeKey  string
	Data       map[string]any
	Recipients []domain.Recipient
	Render     func(loc *time.Location) (title, body string)
}

// plan turns a page of the group log into notices, collapsing a burst of new
// materials into one line.
func (s *Service) plan(ctx context.Context, group *domain.Group, events []domain.Event) ([]notice, error) {
	var out []notice
	var materials []domain.Event
	for i := range events {
		e := events[i]
		var (
			n   *notice
			err error
		)
		switch e.Kind {
		case domain.EventMessageCreated:
			out, err = s.planMessage(ctx, group, e, out)
			if err != nil {
				return nil, err
			}
			continue
		case domain.EventMaterialAdded:
			materials = append(materials, e)
			continue
		case domain.EventTaskCreated, domain.EventTaskPinned, domain.EventTaskDueSoon,
			domain.EventTaskOverdue, domain.EventTaskStatusChanged:
			n, err = s.planTask(ctx, group, e)
		case domain.EventScheduleCreated, domain.EventScheduleUpdated, domain.EventScheduleDeleted,
			domain.EventScheduleCancelled, domain.EventScheduleChanged, domain.EventScheduleReset:
			n, err = s.planSchedule(ctx, group, e)
		case domain.EventAnnouncementCreated:
			n, err = s.planAnnouncement(ctx, group, e)
		case domain.EventMemberJoined:
			n, err = s.planMemberJoined(ctx, group, e)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		if n != nil && len(n.Recipients) > 0 {
			out = append(out, *n)
		}
	}
	batch, err := s.planMaterials(ctx, group, materials)
	if err != nil {
		return nil, err
	}
	return append(out, batch...), nil
}

// planMessage decides who hears about a new message. Subject discussions and
// the group chat reach everybody (D48); a discussion attached to a material
// or a task reaches the people in it and the owner of the thing discussed;
// an answer to somebody's own message always reaches its author.
func (s *Service) planMessage(ctx context.Context, group *domain.Group, e domain.Event, out []notice) ([]notice, error) {
	if e.EntityID == nil {
		return out, nil
	}
	msg, err := s.Notify.Message(ctx, *e.EntityID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return out, nil
		}
		return nil, err
	}
	if msg.Deleted || msg.Body == "" {
		return out, nil
	}
	exclude := []uuid.UUID{}
	if msg.AuthorID != nil {
		exclude = append(exclude, *msg.AuthorID)
	}
	data := map[string]any{
		"screen": "thread", "thread_id": msg.ThreadID,
		"target_type": msg.TargetType, "target_id": msg.TargetID,
	}
	if msg.SubjectID != nil {
		data["subject_id"] = *msg.SubjectID
	}
	where := s.threadLabel(msg)

	// The author of the message being answered hears about it first, so the
	// group notification does not take their place.
	if msg.ReplyAuthorID != nil && (msg.AuthorID == nil || *msg.ReplyAuthorID != *msg.AuthorID) {
		to, err := s.recipients(ctx, group, domain.NotifyMessageReply, exclude, msg.SubjectID, &msg.ThreadID, e.Seq)
		if err != nil {
			return nil, err
		}
		to = only(to, []uuid.UUID{*msg.ReplyAuthorID})
		if len(to) > 0 {
			exclude = append(exclude, *msg.ReplyAuthorID)
			out = append(out, notice{
				Type: domain.NotifyMessageReply, Seq: e.Seq, DedupeKey: dedupe(domain.NotifyMessageReply, *e.EntityID, e.Seq),
				Data: data, Recipients: to,
				Render: func(*time.Location) (string, string) {
					return fmt.Sprintf("Ответ от %s", authorName(msg)), msg.Body
				},
			})
		}
	}

	to, err := s.recipients(ctx, group, domain.NotifyMessageNew, exclude, msg.SubjectID, &msg.ThreadID, e.Seq)
	if err != nil {
		return nil, err
	}
	if to, err = s.limitToInvolved(ctx, msg, to); err != nil {
		return nil, err
	}
	if len(to) == 0 {
		return out, nil
	}
	return append(out, notice{
		Type: domain.NotifyMessageNew, Seq: e.Seq, DedupeKey: dedupe(domain.NotifyMessageNew, *e.EntityID, e.Seq),
		Data: data, Recipients: to,
		Render: func(*time.Location) (string, string) {
			return where, fmt.Sprintf("%s: %s", authorName(msg), msg.Body)
		},
	}), nil
}

// limitToInvolved narrows a discussion that is not the whole group's business
// (open question №7) to the people writing in it and the owner of the
// material or task it hangs on.
func (s *Service) limitToInvolved(ctx context.Context, msg *domain.NotifyMessage, to []domain.Recipient) ([]domain.Recipient, error) {
	switch msg.TargetType {
	case domain.ThreadSubject, domain.ThreadGeneral:
		return to, nil
	}
	involved, err := s.Notify.ThreadParticipants(ctx, msg.ThreadID)
	if err != nil {
		return nil, err
	}
	switch msg.TargetType {
	case domain.ThreadMaterial:
		owner, err := s.Notify.MaterialOwner(ctx, msg.TargetID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		if owner != nil {
			involved = append(involved, *owner)
		}
	case domain.ThreadTask:
		task, err := s.Notify.Task(ctx, msg.TargetID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		if task != nil && task.CreatedBy != nil {
			involved = append(involved, *task.CreatedBy)
		}
	}
	return only(to, involved), nil
}

// materialPayload is what a material.added event carries.
type materialPayload struct {
	Title     string     `json:"title"`
	SubjectID *uuid.UUID `json:"subject_id"`
	// TaskID marks a file that belongs to a task only: the feed stays quiet
	// and so do we (D43).
	TaskID *uuid.UUID `json:"task_id"`
}

// planMaterials announces new materials, as one line per file or — from
// MaterialBatchMin files on — a single "N new materials".
func (s *Service) planMaterials(ctx context.Context, group *domain.Group, events []domain.Event) ([]notice, error) {
	type item struct {
		event   domain.Event
		payload materialPayload
	}
	items := make([]item, 0, len(events))
	for _, e := range events {
		var p materialPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, fmt.Errorf("material.added payload: %w", err)
		}
		if p.TaskID != nil || e.EntityID == nil {
			continue
		}
		items = append(items, item{e, p})
	}
	if len(items) == 0 {
		return nil, nil
	}
	last := items[len(items)-1].event
	if len(items) >= s.set.MaterialBatchMin {
		to, err := s.recipients(ctx, group, domain.NotifyMaterialAdded, actorOf(last), nil, nil, last.Seq)
		if err != nil || len(to) == 0 {
			return nil, err
		}
		count := len(items)
		return []notice{{
			Type: domain.NotifyMaterialBatch, Seq: last.Seq,
			DedupeKey: fmt.Sprintf("%s:%d", domain.NotifyMaterialBatch, last.Seq),
			Data:      map[string]any{"screen": "materials"}, Recipients: to,
			Render: func(*time.Location) (string, string) {
				return "Новые материалы", fmt.Sprintf("Добавлено %d %s", count, plural(count, "файл", "файла", "файлов"))
			},
		}}, nil
	}
	out := make([]notice, 0, len(items))
	for _, it := range items {
		to, err := s.recipients(ctx, group, domain.NotifyMaterialAdded, actorOf(it.event), it.payload.SubjectID, nil, it.event.Seq)
		if err != nil {
			return nil, err
		}
		if len(to) == 0 {
			continue
		}
		subject := s.subjectName(ctx, it.payload.SubjectID)
		title := it.payload.Title
		out = append(out, notice{
			Type: domain.NotifyMaterialAdded, Seq: it.event.Seq,
			DedupeKey:  dedupe(domain.NotifyMaterialAdded, *it.event.EntityID, it.event.Seq),
			Data:       map[string]any{"screen": "material", "material_id": *it.event.EntityID},
			Recipients: to,
			Render: func(*time.Location) (string, string) {
				return "Новый материал", withSubject(title, subject)
			},
		})
	}
	return out, nil
}

// taskPayload is what task events carry.
type taskPayload struct {
	Title     string     `json:"title"`
	SubjectID *uuid.UUID `json:"subject_id"`
	DueAt     *time.Time `json:"due_at"`
	Status    string     `json:"status"`
}

// planTask announces a task to the people it is for: everybody, the chosen
// members, or — for a deadline — only those who have not finished it yet.
func (s *Service) planTask(ctx context.Context, group *domain.Group, e domain.Event) (*notice, error) {
	if e.EntityID == nil {
		return nil, nil
	}
	var p taskPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return nil, fmt.Errorf("task payload: %w", err)
	}
	task, err := s.Notify.Task(ctx, *e.EntityID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if task.Visibility == domain.TaskVisiblePrivate || task.AssignMode == domain.AssignSelf {
		return nil, nil // a personal task is nobody else's business
	}
	var kind domain.NotificationType
	var title string
	switch e.Kind {
	case domain.EventTaskCreated:
		kind, title = domain.NotifyTaskCreated, "Новая задача"
	case domain.EventTaskPinned:
		kind, title = domain.NotifyTaskPinned, "Задача закреплена"
	case domain.EventTaskDueSoon:
		kind, title = domain.NotifyTaskDueSoon, "Скоро срок"
	case domain.EventTaskOverdue:
		kind, title = domain.NotifyTaskOverdue, "Срок прошёл"
	case domain.EventTaskStatusChanged:
		kind, title = domain.NotifyTaskStatus, "Статус задачи изменился"
	default:
		return nil, nil
	}
	to, err := s.recipients(ctx, group, kind, actorOf(e), task.SubjectID, nil, e.Seq)
	if err != nil {
		return nil, err
	}
	assignees, err := s.Notify.TaskAssignees(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if task.AssignMode == domain.AssignSelected {
		to = only(to, assigneeIDs(assignees, false))
	}
	if kind == domain.NotifyTaskDueSoon || kind == domain.NotifyTaskOverdue {
		// Somebody who has already handed it in is left alone.
		to = without(to, assigneeIDs(assignees, true))
	}
	if len(to) == 0 {
		return nil, nil
	}
	due := task.DueAt
	name := task.Title
	return &notice{
		Type: kind, Seq: e.Seq, DedupeKey: dedupe(kind, task.ID, e.Seq),
		Data:       map[string]any{"screen": "task", "task_id": task.ID},
		Recipients: to,
		Render: func(loc *time.Location) (string, string) {
			body := name
			if due != nil && (kind == domain.NotifyTaskDueSoon || kind == domain.NotifyTaskCreated) {
				body = fmt.Sprintf("%s — до %s", name, dateTime(*due, loc))
			}
			return title, body
		},
	}, nil
}

// schedulePayload is what schedule events carry.
type schedulePayload struct {
	Title     string     `json:"title"`
	Kind      string     `json:"kind"`
	SubjectID *uuid.UUID `json:"subject_id"`
	StartsAt  *time.Time `json:"starts_at"`
	// Date is the planned day of the one class an exception touches.
	Date string `json:"date"`
}

// planSchedule announces a change of the timetable to the whole group.
func (s *Service) planSchedule(ctx context.Context, group *domain.Group, e domain.Event) (*notice, error) {
	if e.EntityID == nil {
		return nil, nil
	}
	var p schedulePayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return nil, fmt.Errorf("schedule payload: %w", err)
	}
	var title string
	switch e.Kind {
	case domain.EventScheduleCreated:
		title = "Новое занятие"
	case domain.EventScheduleUpdated:
		title = "Занятие изменено"
	case domain.EventScheduleDeleted:
		title = "Занятие удалено"
	case domain.EventScheduleCancelled:
		title = "Занятие отменено"
	case domain.EventScheduleChanged:
		title = "Занятие перенесено"
	case domain.EventScheduleReset:
		title = "Занятие вернули в расписание"
	default:
		return nil, nil
	}
	to, err := s.recipients(ctx, group, domain.NotifyScheduleChanged, actorOf(e), p.SubjectID, nil, e.Seq)
	if err != nil || len(to) == 0 {
		return nil, err
	}
	data := map[string]any{"screen": "schedule", "event_id": *e.EntityID}
	if p.Date != "" {
		data["screen"], data["date"] = "class", p.Date
	}
	name := p.Title
	if name == "" {
		name = s.subjectName(ctx, p.SubjectID)
	}
	starts := p.StartsAt
	return &notice{
		Type: domain.NotifyScheduleChanged, Seq: e.Seq,
		DedupeKey: dedupe(domain.NotifyScheduleChanged, *e.EntityID, e.Seq),
		Data:      data, Recipients: to,
		Render: func(loc *time.Location) (string, string) {
			if starts != nil {
				return title, fmt.Sprintf("%s · %s", name, dateTime(*starts, loc))
			}
			return title, name
		},
	}, nil
}

// announcementPayload is what announcement.created carries. The text is
// group news, so it travels with the event.
type announcementPayload struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Urgent bool   `json:"urgent"`
}

// planAnnouncement tells the whole group what the headman said. An urgent one
// reaches the phone even during quiet hours.
func (s *Service) planAnnouncement(ctx context.Context, group *domain.Group, e domain.Event) (*notice, error) {
	if e.EntityID == nil {
		return nil, nil
	}
	var p announcementPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return nil, fmt.Errorf("announcement payload: %w", err)
	}
	to, err := s.recipients(ctx, group, domain.NotifyAnnouncement, actorOf(e), nil, nil, e.Seq)
	if err != nil || len(to) == 0 {
		return nil, err
	}
	title := "Объявление"
	if p.Urgent {
		title = "Срочное объявление"
	}
	body := p.Title
	if p.Body != "" {
		body = fmt.Sprintf("%s: %s", p.Title, p.Body)
	}
	return &notice{
		Type: domain.NotifyAnnouncement, Seq: e.Seq,
		DedupeKey: dedupe(domain.NotifyAnnouncement, *e.EntityID, e.Seq),
		Data: map[string]any{
			"screen": "announcement", "announcement_id": *e.EntityID, "urgent": p.Urgent,
		},
		Recipients: to,
		Render:     func(*time.Location) (string, string) { return title, body },
	}, nil
}

// memberPayload is what member.joined carries.
type memberPayload struct {
	UserID *uuid.UUID `json:"user_id"`
}

func (s *Service) planMemberJoined(ctx context.Context, group *domain.Group, e domain.Event) (*notice, error) {
	var p memberPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return nil, fmt.Errorf("member.joined payload: %w", err)
	}
	if p.UserID == nil {
		return nil, nil
	}
	to, err := s.recipients(ctx, group, domain.NotifyMemberJoined, []uuid.UUID{*p.UserID}, nil, nil, e.Seq)
	if err != nil || len(to) == 0 {
		return nil, err
	}
	name, err := s.Notify.UserName(ctx, *p.UserID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	return &notice{
		Type: domain.NotifyMemberJoined, Seq: e.Seq,
		DedupeKey:  dedupe(domain.NotifyMemberJoined, *p.UserID, e.Seq),
		Data:       map[string]any{"screen": "activity"},
		Recipients: to,
		Render: func(*time.Location) (string, string) {
			return "Новый участник", fmt.Sprintf("%s теперь в группе", name)
		},
	}, nil
}

// recipients asks the repository who still wants this kind of notification.
func (s *Service) recipients(ctx context.Context, group *domain.Group, t domain.NotificationType,
	exclude []uuid.UUID, subject, thread *uuid.UUID, seq int64,
) ([]domain.Recipient, error) {
	return s.Notify.Recipients(ctx, domain.NotifyRecipients{
		GroupID: group.ID, Type: t, Exclude: exclude,
		Default:   t.DefaultOn(group.Kind, s.set.DPOSilent),
		SubjectID: subject, ThreadID: thread, Seq: seq, Now: s.Clock.Now(),
	})
}

func (s *Service) subjectName(ctx context.Context, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	name, err := s.Notify.SubjectName(ctx, *id)
	if err != nil {
		return ""
	}
	return name
}

// threadLabel is the headline of a notification: which discussion it is.
func (s *Service) threadLabel(msg *domain.NotifyMessage) string {
	switch msg.TargetType {
	case domain.ThreadGeneral:
		return "Общий чат"
	case domain.ThreadSubject:
		if msg.SubjectName != "" {
			return msg.SubjectName
		}
	}
	if msg.Title != "" {
		return msg.Title
	}
	if msg.SubjectName != "" {
		return msg.SubjectName
	}
	return "Обсуждение"
}

func authorName(msg *domain.NotifyMessage) string {
	if msg.AuthorName == "" {
		return "Участник"
	}
	return msg.AuthorName
}

func actorOf(e domain.Event) []uuid.UUID {
	if e.ActorID == nil {
		return nil
	}
	return []uuid.UUID{*e.ActorID}
}

func assigneeIDs(assignees []domain.TaskAssignee, doneOnly bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(assignees))
	for _, a := range assignees {
		if doneOnly == (a.Status == domain.TaskDone) {
			out = append(out, a.UserID)
		}
	}
	return out
}

// only keeps the recipients that are in the allowed list.
func only(to []domain.Recipient, allowed []uuid.UUID) []domain.Recipient {
	set := make(map[uuid.UUID]bool, len(allowed))
	for _, id := range allowed {
		set[id] = true
	}
	out := make([]domain.Recipient, 0, len(to))
	for _, r := range to {
		if set[r.UserID] {
			out = append(out, r)
		}
	}
	return out
}

// without drops the listed recipients.
func without(to []domain.Recipient, ids []uuid.UUID) []domain.Recipient {
	set := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	out := make([]domain.Recipient, 0, len(to))
	for _, r := range to {
		if !set[r.UserID] {
			out = append(out, r)
		}
	}
	return out
}

// dedupe keys one fact per user; a repeated pass over the log writes nothing.
func dedupe(t domain.NotificationType, entity uuid.UUID, seq int64) string {
	return fmt.Sprintf("%s:%s:%d", t, entity, seq)
}
