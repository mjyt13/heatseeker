package http

import (
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/discussions"
	"heatseeker/api/internal/domain"
)

// ThreadRefDTO names the discussion a message belongs to.
type ThreadRefDTO struct {
	ID         uuid.UUID  `json:"id"`
	TargetType string     `json:"target_type" enum:"SUBJECT,LESSON,MATERIAL,TASK,PROPOSAL,GENERAL"`
	TargetID   uuid.UUID  `json:"target_id"`
	SubjectID  *uuid.UUID `json:"subject_id,omitempty"`
	Title      *string    `json:"title,omitempty" doc:"Название материала или задачи на момент первого сообщения."`
}

func toThreadRefDTO(t *domain.Thread) *ThreadRefDTO {
	if t == nil {
		return nil
	}
	return &ThreadRefDTO{ID: t.ID, TargetType: string(t.TargetType), TargetID: t.TargetID, SubjectID: t.SubjectID, Title: t.Title}
}

// MessageReplyDTO is the quoted message of a reply.
type MessageReplyDTO struct {
	ID           uuid.UUID  `json:"id"`
	AuthorID     *uuid.UUID `json:"author_id,omitempty"`
	AuthorName   string     `json:"author_name"`
	Body         string     `json:"body" doc:"Начало текста; пусто, если сообщение удалено или скрыто модератором."`
	Deleted      bool       `json:"deleted"`
	HiddenForAll bool       `json:"hidden_for_all"`
}

// MessageDTO is a message as the reader may see it.
type MessageDTO struct {
	ID             uuid.UUID        `json:"id"`
	ThreadID       uuid.UUID        `json:"thread_id"`
	ClientID       uuid.UUID        `json:"client_id"`
	Seq            int64            `json:"seq" doc:"Порядок в треде; совпадает с seq события message.created."`
	AuthorID       *uuid.UUID       `json:"author_id,omitempty"`
	AuthorName     string           `json:"author_name"`
	Body           string           `json:"body" doc:"Markdown; пусто для удалённого, скрытого модератором (кроме автора и модераторов) или скрытого мной без include_hidden."`
	ReplyTo        *MessageReplyDTO `json:"reply_to,omitempty"`
	Mine           bool             `json:"mine"`
	Deleted        bool             `json:"deleted"`
	HiddenForAll   bool             `json:"hidden_for_all" doc:"Скрыто модератором для всех."`
	HiddenForAllBy *uuid.UUID       `json:"hidden_for_all_by,omitempty" doc:"Кто скрыл; видят только модераторы."`
	HiddenByMe     bool             `json:"hidden_by_me"`
	EditedAt       *time.Time       `json:"edited_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	Thread         *ThreadRefDTO    `json:"thread,omitempty" doc:"Только в списке «Скрытые мной»."`
}

func toMessageDTO(v *domain.MessageView, viewerID uuid.UUID) MessageDTO {
	dto := MessageDTO{
		ID: v.ID, ThreadID: v.ThreadID, ClientID: v.ClientID, Seq: v.Seq, AuthorID: v.AuthorID, AuthorName: v.AuthorName,
		Body: v.Body, Mine: v.AuthorID != nil && *v.AuthorID == viewerID, Deleted: v.DeletedAt != nil,
		HiddenForAll: v.HiddenForAllAt != nil, HiddenForAllBy: v.HiddenForAllBy, HiddenByMe: v.HiddenByMe,
		EditedAt: v.EditedAt, CreatedAt: v.CreatedAt, Thread: toThreadRefDTO(v.Thread),
	}
	if r := v.Reply; r != nil {
		dto.ReplyTo = &MessageReplyDTO{
			ID: r.ID, AuthorID: r.AuthorID, AuthorName: r.AuthorName, Body: r.Body, Deleted: r.Deleted, HiddenForAll: r.HiddenForAll,
		}
	}
	return dto
}

func toMessageDTOs(views []domain.MessageView, viewerID uuid.UUID) []MessageDTO {
	out := make([]MessageDTO, len(views))
	for i := range views {
		out[i] = toMessageDTO(&views[i], viewerID)
	}
	return out
}

// MessagePreviewDTO is the latest visible message of a discussion.
type MessagePreviewDTO struct {
	AuthorID   *uuid.UUID `json:"author_id,omitempty"`
	AuthorName string     `json:"author_name"`
	Body       string     `json:"body" doc:"Начало текста."`
	CreatedAt  time.Time  `json:"created_at"`
}

// DiscussionDTO is one row of the discussions screen.
type DiscussionDTO struct {
	TargetType    string             `json:"target_type" enum:"SUBJECT,LESSON,MATERIAL,TASK,PROPOSAL,GENERAL"`
	TargetID      uuid.UUID          `json:"target_id"`
	SubjectID     *uuid.UUID         `json:"subject_id,omitempty"`
	Title         *string            `json:"title,omitempty"`
	ThreadID      *uuid.UUID         `json:"thread_id,omitempty" doc:"Пусто, пока в обсуждении никто не писал."`
	MessageCount  int32              `json:"message_count"`
	Unread        int32              `json:"unread"`
	NestedUnread  int32              `json:"nested_unread" doc:"Непрочитанное в обсуждениях материалов и задач предмета."`
	LastMessageAt *time.Time         `json:"last_message_at,omitempty"`
	LastMessage   *MessagePreviewDTO `json:"last_message,omitempty"`
}

func toDiscussionDTO(target discussions.Target, t *domain.ThreadSummary) DiscussionDTO {
	dto := DiscussionDTO{TargetType: string(target.Type), TargetID: target.ID}
	if t == nil {
		return dto
	}
	dto.SubjectID, dto.Title, dto.ThreadID = t.SubjectID, t.Title, &t.ID
	dto.MessageCount, dto.Unread, dto.LastMessageAt = t.MessageCount, t.Unread, t.LastMessageAt
	if l := t.Last; l != nil {
		dto.LastMessage = &MessagePreviewDTO{AuthorID: l.AuthorID, AuthorName: l.AuthorName, Body: l.Body, CreatedAt: l.CreatedAt}
	}
	return dto
}

// DiscussionsDTO is the discussions screen.
type DiscussionsDTO struct {
	General  DiscussionDTO   `json:"general" doc:"Общий тред группы."`
	Subjects []DiscussionDTO `json:"subjects" doc:"По строке на каждый активный предмет, в порядке предметов."`
	Others   []DiscussionDTO `json:"others" doc:"Обсуждения материалов, задач и архивных предметов, где уже писали."`
	Unread   int32           `json:"unread" doc:"Всего непрочитанных сообщений."`
}

func toDiscussionsDTO(o *discussions.Overview) DiscussionsDTO {
	dto := DiscussionsDTO{
		General:  toDiscussionDTO(o.General.Target, o.General.Thread),
		Subjects: make([]DiscussionDTO, len(o.Subjects)),
		Others:   make([]DiscussionDTO, len(o.Others)),
		Unread:   o.Unread,
	}
	for i, s := range o.Subjects {
		row := toDiscussionDTO(s.Target, s.Thread)
		row.SubjectID = &o.Subjects[i].Subject.ID
		row.NestedUnread = s.NestedUnread
		dto.Subjects[i] = row
	}
	for i := range o.Others {
		t := &o.Others[i]
		dto.Others[i] = toDiscussionDTO(discussions.Target{Type: t.TargetType, ID: t.TargetID}, t)
	}
	return dto
}

// MessagePageDTO is a slice of a discussion, oldest message first.
type MessagePageDTO struct {
	Thread      *ThreadRefDTO `json:"thread,omitempty" doc:"Пусто, пока в обсуждении никто не писал."`
	Items       []MessageDTO  `json:"items"`
	HasMore     bool          `json:"has_more" doc:"Есть ещё сообщения в направлении чтения."`
	LastReadSeq int64         `json:"last_read_seq" doc:"Моя отметка прочтения до этого запроса."`
	LastSeq     int64         `json:"last_seq" doc:"seq последнего сообщения треда."`
}
