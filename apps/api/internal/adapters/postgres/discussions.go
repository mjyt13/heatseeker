package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type discussionRepo struct{ s *Store }

func toThread(t sqlcgen.Thread) *domain.Thread {
	return &domain.Thread{
		ID:            t.ID,
		GroupID:       t.GroupID,
		TargetType:    domain.ThreadTarget(t.TargetType),
		TargetID:      t.TargetID,
		SubjectID:     t.SubjectID,
		Title:         t.Title,
		CreatedBy:     t.CreatedBy,
		MessageCount:  t.MessageCount,
		LastMessageAt: t.LastMessageAt,
		LastSeq:       t.LastSeq,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

func toMessage(m sqlcgen.Message) *domain.Message {
	return &domain.Message{
		ID:             m.ID,
		GroupID:        m.GroupID,
		ThreadID:       m.ThreadID,
		AuthorID:       m.AuthorID,
		ClientID:       m.ClientID,
		Seq:            m.Seq,
		Body:           m.Body,
		ReplyToID:      m.ReplyToID,
		EditedAt:       m.EditedAt,
		DeletedAt:      m.DeletedAt,
		HiddenForAllBy: m.HiddenForAllBy,
		HiddenForAllAt: m.HiddenForAllAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

// toMessageView converts any of the message view rows: they share one shape.
func toMessageView(row sqlcgen.GetMessageViewRow) domain.MessageView {
	v := domain.MessageView{Message: *toMessage(row.Message), AuthorName: row.AuthorName, HiddenByMe: row.HiddenByMe}
	if row.ReplyID != nil {
		v.Reply = &domain.MessageReply{
			ID: *row.ReplyID, AuthorID: row.ReplyAuthorID, AuthorName: row.ReplyAuthorName, Body: row.ReplyBody,
			Deleted: row.ReplyDeleted, HiddenForAll: row.ReplyHiddenForAll,
		}
	}
	return v
}

func (r *discussionRepo) GetThread(ctx context.Context, groupID uuid.UUID, target domain.ThreadTarget, targetID uuid.UUID) (*domain.Thread, error) {
	row, err := r.s.queries(ctx).GetThreadByTarget(ctx, sqlcgen.GetThreadByTargetParams{
		GroupID: groupID, TargetType: string(target), TargetID: targetID,
	})
	if err != nil {
		return nil, mapErr(err, "thread")
	}
	return toThread(row), nil
}

func (r *discussionRepo) GetThreadByID(ctx context.Context, id uuid.UUID) (*domain.Thread, error) {
	row, err := r.s.queries(ctx).GetThread(ctx, id)
	if err != nil {
		return nil, mapErr(err, "thread")
	}
	return toThread(row), nil
}

func (r *discussionRepo) EnsureThread(ctx context.Context, t domain.Thread) (*domain.Thread, error) {
	row, err := r.s.queries(ctx).EnsureThread(ctx, sqlcgen.EnsureThreadParams{
		ID: t.ID, GroupID: t.GroupID, TargetType: string(t.TargetType), TargetID: t.TargetID,
		SubjectID: t.SubjectID, Title: t.Title, CreatedBy: t.CreatedBy,
	})
	if err != nil {
		return nil, mapErr(err, "thread")
	}
	return toThread(row), nil
}

func (r *discussionRepo) ListThreads(ctx context.Context, groupID, viewerID uuid.UUID) ([]domain.ThreadSummary, error) {
	rows, err := r.s.queries(ctx).ListThreadSummaries(ctx, sqlcgen.ListThreadSummariesParams{GroupID: groupID, ViewerID: viewerID})
	if err != nil {
		return nil, mapErr(err, "thread")
	}
	out := make([]domain.ThreadSummary, len(rows))
	for i, row := range rows {
		out[i] = domain.ThreadSummary{Thread: *toThread(row.Thread), Unread: row.Unread}
		if row.HasLast {
			out[i].Last = &domain.MessagePreview{
				AuthorID: row.LastAuthorID, AuthorName: row.LastAuthorName, Body: row.LastBody, CreatedAt: row.LastCreatedAt,
			}
		}
	}
	return out, nil
}

func (r *discussionRepo) TouchThread(ctx context.Context, threadID uuid.UUID, seq int64, at time.Time) error {
	err := r.s.queries(ctx).TouchThread(ctx, sqlcgen.TouchThreadParams{ID: threadID, Seq: seq, At: at})
	return mapErr(err, "thread")
}

func (r *discussionRepo) AdjustMessageCount(ctx context.Context, threadID uuid.UUID, delta int32) error {
	err := r.s.queries(ctx).AdjustThreadMessageCount(ctx, sqlcgen.AdjustThreadMessageCountParams{ID: threadID, Delta: delta})
	return mapErr(err, "thread")
}

func (r *discussionRepo) CreateMessage(ctx context.Context, m domain.Message) (*domain.Message, error) {
	row, err := r.s.queries(ctx).CreateMessage(ctx, sqlcgen.CreateMessageParams{
		ID: m.ID, GroupID: m.GroupID, ThreadID: m.ThreadID, AuthorID: m.AuthorID, ClientID: m.ClientID,
		Seq: m.Seq, Body: m.Body, ReplyToID: m.ReplyToID,
	})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	return toMessage(row), nil
}

func (r *discussionRepo) GetMessage(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	row, err := r.s.queries(ctx).GetMessage(ctx, id)
	if err != nil {
		return nil, mapErr(err, "message")
	}
	return toMessage(row), nil
}

func (r *discussionRepo) GetMessageByClientID(ctx context.Context, threadID, clientID uuid.UUID) (*domain.Message, error) {
	row, err := r.s.queries(ctx).GetMessageByClientID(ctx, sqlcgen.GetMessageByClientIDParams{ThreadID: threadID, ClientID: clientID})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	return toMessage(row), nil
}

func (r *discussionRepo) GetMessageView(ctx context.Context, id, viewerID uuid.UUID) (*domain.MessageView, error) {
	row, err := r.s.queries(ctx).GetMessageView(ctx, sqlcgen.GetMessageViewParams{ID: id, ViewerID: viewerID})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	v := toMessageView(row)
	return &v, nil
}

// ListMessages returns the page in thread order (oldest first) whichever
// direction it was read in.
func (r *discussionRepo) ListMessages(ctx context.Context, f domain.MessageFilter) ([]domain.MessageView, error) {
	q := r.s.queries(ctx)
	if f.AfterSeq != nil {
		rows, err := q.ListMessagesAfter(ctx, sqlcgen.ListMessagesAfterParams{
			ViewerID: f.ViewerID, ThreadID: f.ThreadID, AfterSeq: *f.AfterSeq, MaxRows: f.Limit,
		})
		if err != nil {
			return nil, mapErr(err, "message")
		}
		out := make([]domain.MessageView, len(rows))
		for i, row := range rows {
			out[i] = toMessageView(sqlcgen.GetMessageViewRow(row))
		}
		return out, nil
	}
	rows, err := q.ListMessagesBefore(ctx, sqlcgen.ListMessagesBeforeParams{
		ViewerID: f.ViewerID, ThreadID: f.ThreadID, BeforeSeq: f.BeforeSeq, MaxRows: f.Limit,
	})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	out := make([]domain.MessageView, len(rows))
	for i, row := range rows {
		out[len(rows)-1-i] = toMessageView(sqlcgen.GetMessageViewRow(row))
	}
	return out, nil
}

func (r *discussionRepo) EditMessage(ctx context.Context, id uuid.UUID, body string, at time.Time) (*domain.Message, error) {
	row, err := r.s.queries(ctx).EditMessage(ctx, sqlcgen.EditMessageParams{ID: id, Body: body, EditedAt: &at})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	return toMessage(row), nil
}

func (r *discussionRepo) SoftDeleteMessage(ctx context.Context, id uuid.UUID, at time.Time) (*domain.Message, error) {
	row, err := r.s.queries(ctx).SoftDeleteMessage(ctx, sqlcgen.SoftDeleteMessageParams{ID: id, DeletedAt: &at})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	return toMessage(row), nil
}

func (r *discussionRepo) UndeleteMessage(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	row, err := r.s.queries(ctx).UndeleteMessage(ctx, id)
	if err != nil {
		return nil, mapErr(err, "message")
	}
	return toMessage(row), nil
}

func (r *discussionRepo) SetHiddenForAll(ctx context.Context, id uuid.UUID, by *uuid.UUID, at *time.Time) (*domain.Message, error) {
	row, err := r.s.queries(ctx).SetMessageHiddenForAll(ctx, sqlcgen.SetMessageHiddenForAllParams{
		ID: id, HiddenForAllBy: by, HiddenForAllAt: at,
	})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	return toMessage(row), nil
}

func (r *discussionRepo) HideMessage(ctx context.Context, messageID, userID uuid.UUID) error {
	err := r.s.queries(ctx).HideMessage(ctx, sqlcgen.HideMessageParams{MessageID: messageID, UserID: userID})
	return mapErr(err, "message")
}

func (r *discussionRepo) UnhideMessage(ctx context.Context, messageID, userID uuid.UUID) error {
	err := r.s.queries(ctx).UnhideMessage(ctx, sqlcgen.UnhideMessageParams{MessageID: messageID, UserID: userID})
	return mapErr(err, "message")
}

func (r *discussionRepo) ListHiddenByUser(ctx context.Context, groupID, userID uuid.UUID, limit int32) ([]domain.MessageView, error) {
	rows, err := r.s.queries(ctx).ListHiddenByUser(ctx, sqlcgen.ListHiddenByUserParams{UserID: userID, GroupID: groupID, MaxRows: limit})
	if err != nil {
		return nil, mapErr(err, "message")
	}
	out := make([]domain.MessageView, len(rows))
	for i, row := range rows {
		out[i] = domain.MessageView{
			Message: *toMessage(row.Message), AuthorName: row.AuthorName, HiddenByMe: true, Thread: toThread(row.Thread),
		}
	}
	return out, nil
}

func (r *discussionRepo) MarkRead(ctx context.Context, threadID, userID uuid.UUID, seq int64) error {
	err := r.s.queries(ctx).MarkThreadRead(ctx, sqlcgen.MarkThreadReadParams{ThreadID: threadID, UserID: userID, LastReadSeq: seq})
	return mapErr(err, "thread")
}

func (r *discussionRepo) ReadSeq(ctx context.Context, threadID, userID uuid.UUID) (int64, error) {
	seq, err := r.s.queries(ctx).GetThreadReadSeq(ctx, sqlcgen.GetThreadReadSeqParams{ThreadID: threadID, UserID: userID})
	return seq, mapErr(err, "thread")
}
