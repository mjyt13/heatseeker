package http

import (
	"context"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"heatseeker/api/internal/app/discussions"
	"heatseeker/api/internal/domain"
)

// DiscussionPath addresses a discussion by what it is about: the thread
// itself appears with the first message (D40).
type DiscussionPath struct {
	GroupID    string `path:"groupId" format:"uuid"`
	TargetType string `path:"targetType" enum:"SUBJECT,LESSON,MATERIAL,TASK,PROPOSAL,GENERAL" doc:"О чём обсуждение; для GENERAL targetId — id группы."`
	TargetID   string `path:"targetId" format:"uuid"`
}

func (p DiscussionPath) parse(ctx context.Context) (Principal, uuid.UUID, discussions.Target, error) {
	principal, groupID, err := principalAndID(ctx, "groupId", p.GroupID)
	if err != nil {
		return principal, groupID, discussions.Target{}, err
	}
	targetID, err := parseID("targetId", p.TargetID)
	if err != nil {
		return principal, groupID, discussions.Target{}, err
	}
	return principal, groupID, discussions.Target{Type: domain.ThreadTarget(p.TargetType), ID: targetID}, nil
}

type listMessagesInput struct {
	DiscussionPath
	BeforeSeq     int64 `query:"before_seq" minimum:"0" doc:"Более старые сообщения, чем этот seq."`
	AfterSeq      int64 `query:"after_seq" minimum:"0" doc:"Более новые сообщения, чем этот seq (догрузка после разрыва)."`
	Limit         int32 `query:"limit" minimum:"1" maximum:"200" default:"50"`
	IncludeHidden bool  `query:"include_hidden" doc:"Показывать текст скрытых мной сообщений."`
}

type postMessageInput struct {
	DiscussionPath
	Body struct {
		ClientID  string  `json:"client_id" format:"uuid" doc:"Идемпотентность: повтор с тем же id вернёт уже отправленное сообщение."`
		Body      string  `json:"body" minLength:"1" maxLength:"16000" doc:"Markdown, до 4000 символов."`
		ReplyToID *string `json:"reply_to_id,omitempty" format:"uuid"`
	}
}

type markReadInput struct {
	DiscussionPath
	Body struct {
		Seq int64 `json:"seq,omitempty" minimum:"0" doc:"До какого сообщения прочитано; 0 — до последнего."`
	}
}

type messageIDInput struct {
	MessageID string `path:"messageId" format:"uuid"`
}

type editMessageInput struct {
	MessageID string `path:"messageId" format:"uuid"`
	Body      struct {
		Body string `json:"body" minLength:"1" maxLength:"16000"`
	}
}

type messageOutput struct {
	Body MessageDTO
}

type messagePageOutput struct {
	Body MessagePageDTO
}

type discussionsOutput struct {
	Body DiscussionsDTO
}

type messagesOutput struct {
	Body struct {
		Items []MessageDTO `json:"items"`
	}
}

func registerDiscussions(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "discussions-overview", Method: nethttp.MethodGet, Path: "/groups/{groupId}/discussions", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Экран «Обсуждения»", Description: "Общий тред, строка на каждый предмет и обсуждения материалов/задач с непрочитанным.",
	}, func(ctx context.Context, in *groupIDInput) (*discussionsOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		o, err := d.Discussions.Overview(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &discussionsOutput{Body: toDiscussionsDTO(o)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "discussions-messages", Method: nethttp.MethodGet, Path: "/groups/{groupId}/discussions/{targetType}/{targetId}/messages",
		Tags: []string{"discussions"}, Security: bearer,
		Summary:     "Сообщения обсуждения",
		Description: "Без курсора — последняя страница; before_seq — старее, after_seq — новее. Сообщения всегда от старых к новым.",
	}, func(ctx context.Context, in *listMessagesInput) (*messagePageOutput, error) {
		p, groupID, target, err := in.parse(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		li := discussions.ListInput{Limit: in.Limit, IncludeHidden: in.IncludeHidden}
		if in.BeforeSeq > 0 {
			li.BeforeSeq = &in.BeforeSeq
		}
		if in.AfterSeq > 0 {
			li.AfterSeq = &in.AfterSeq
		}
		page, err := d.Discussions.List(ctx, p.UserID, groupID, target, li)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &messagePageOutput{Body: MessagePageDTO{
			Thread: toThreadRefDTO(page.Thread), Items: toMessageDTOs(page.Messages, p.UserID),
			HasMore: page.HasMore, LastReadSeq: page.LastReadSeq,
		}}
		if page.Thread != nil {
			out.Body.LastSeq = page.Thread.LastSeq
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "discussions-post", Method: nethttp.MethodPost, Path: "/groups/{groupId}/discussions/{targetType}/{targetId}/messages",
		Tags: []string{"discussions"}, Security: bearer, DefaultStatus: nethttp.StatusCreated,
		Summary: "Написать сообщение", Description: "Первое сообщение создаёт тред. Гости не пишут.",
	}, func(ctx context.Context, in *postMessageInput) (*messageOutput, error) {
		p, groupID, target, err := in.parse(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		pi := discussions.PostInput{Body: in.Body.Body}
		if pi.ClientID, err = parseID("client_id", in.Body.ClientID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		if in.Body.ReplyToID != nil {
			if pi.ReplyToID, err = parseOptionalID("reply_to_id", *in.Body.ReplyToID); err != nil {
				return nil, apiErr(d.Log, err)
			}
		}
		msg, err := d.Discussions.Post(ctx, p.UserID, groupID, target, pi)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &messageOutput{Body: toMessageDTO(msg, p.UserID)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "discussions-read", Method: nethttp.MethodPost, Path: "/groups/{groupId}/discussions/{targetType}/{targetId}/read",
		Tags: []string{"discussions"}, Security: bearer, DefaultStatus: nethttp.StatusNoContent,
		Summary: "Отметить прочитанным", Description: "Отметка только двигается вперёд.",
	}, func(ctx context.Context, in *markReadInput) (*struct{}, error) {
		p, groupID, target, err := in.parse(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Discussions.MarkRead(ctx, p.UserID, groupID, target, in.Body.Seq); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "discussions-hidden", Method: nethttp.MethodGet, Path: "/groups/{groupId}/messages/hidden", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Скрытые мной", Description: "Сообщения, которые я скрыл для себя, — чтобы быстро вернуть.",
	}, func(ctx context.Context, in *groupIDInput) (*messagesOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		msgs, err := d.Discussions.HiddenByMe(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &messagesOutput{}
		out.Body.Items = toMessageDTOs(msgs, p.UserID)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "messages-edit", Method: nethttp.MethodPatch, Path: "/messages/{messageId}", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Изменить своё сообщение",
	}, func(ctx context.Context, in *editMessageInput) (*messageOutput, error) {
		p, id, err := principalAndID(ctx, "messageId", in.MessageID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		msg, err := d.Discussions.Edit(ctx, p.UserID, id, in.Body.Body)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &messageOutput{Body: toMessageDTO(msg, p.UserID)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "messages-delete", Method: nethttp.MethodDelete, Path: "/messages/{messageId}", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Удалить своё сообщение", Description: "В треде остаётся пометка «сообщение удалено».", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *messageIDInput) (*struct{}, error) {
		p, id, err := principalAndID(ctx, "messageId", in.MessageID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Discussions.Delete(ctx, p.UserID, id); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "messages-restore", Method: nethttp.MethodPost, Path: "/messages/{messageId}/restore", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Восстановить своё удалённое сообщение",
	}, func(ctx context.Context, in *messageIDInput) (*messageOutput, error) {
		p, id, err := principalAndID(ctx, "messageId", in.MessageID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		msg, err := d.Discussions.Restore(ctx, p.UserID, id)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &messageOutput{Body: toMessageDTO(msg, p.UserID)}, nil
	})

	hide := func(forAll, hidden bool) func(context.Context, *messageIDInput) (*messageOutput, error) {
		return func(ctx context.Context, in *messageIDInput) (*messageOutput, error) {
			p, id, err := principalAndID(ctx, "messageId", in.MessageID)
			if err != nil {
				return nil, apiErr(d.Log, err)
			}
			var msg *domain.MessageView
			if forAll {
				msg, err = d.Discussions.SetHiddenForAll(ctx, p.UserID, id, hidden)
			} else {
				msg, err = d.Discussions.SetHiddenForMe(ctx, p.UserID, id, hidden)
			}
			if err != nil {
				return nil, apiErr(d.Log, err)
			}
			return &messageOutput{Body: toMessageDTO(msg, p.UserID)}, nil
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "messages-hide", Method: nethttp.MethodPost, Path: "/messages/{messageId}/hide", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Скрыть для себя", Description: "Сообщение сворачивается только у меня.",
	}, hide(false, true))
	huma.Register(api, huma.Operation{
		OperationID: "messages-unhide", Method: nethttp.MethodDelete, Path: "/messages/{messageId}/hide", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Вернуть скрытое мной",
	}, hide(false, false))
	huma.Register(api, huma.Operation{
		OperationID: "messages-moderate", Method: nethttp.MethodPost, Path: "/messages/{messageId}/moderate", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Скрыть для всех", Description: "Модератор или админ. Автор и модераторы продолжают видеть текст с пометкой.",
	}, hide(true, true))
	huma.Register(api, huma.Operation{
		OperationID: "messages-unmoderate", Method: nethttp.MethodDelete, Path: "/messages/{messageId}/moderate", Tags: []string{"discussions"}, Security: bearer,
		Summary: "Вернуть скрытое модератором",
	}, hide(true, false))
}
