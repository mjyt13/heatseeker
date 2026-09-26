package http

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"heatseeker/api/internal/app/notify"
	"heatseeker/api/internal/domain"
)

type notificationsInput struct {
	GroupID    string `query:"group_id" format:"uuid" doc:"Только эта группа; пусто — все мои группы."`
	UnreadOnly bool   `query:"unread_only" doc:"Только непрочитанные."`
	Cursor     string `query:"cursor"`
	Limit      int32  `query:"limit" minimum:"1" maximum:"100" default:"30"`
}

type notificationsOutput struct {
	Body struct {
		Items      []NotificationDTO `json:"items"`
		Unread     int               `json:"unread" doc:"Сколько всего непрочитанных в этой выборке, не только на странице."`
		NextCursor string            `json:"next_cursor,omitempty"`
	}
}

type unreadOutput struct {
	Body struct {
		Unread int `json:"unread"`
	}
}

type readNotificationsInput struct {
	Body struct {
		IDs     []string `json:"ids,omitempty" maxItems:"200" doc:"Что пометить прочитанным."`
		All     bool     `json:"all,omitempty" doc:"Пометить всё непрочитанное (в группе, если указана)."`
		GroupID *string  `json:"group_id,omitempty" format:"uuid"`
	}
}

type readNotificationsOutput struct {
	Body struct {
		Marked int `json:"marked"`
		Unread int `json:"unread"`
	}
}

type prefsInput struct {
	GroupID string `path:"groupId" format:"uuid"`
}

type prefsOutput struct {
	Body struct {
		Items []NotificationPrefDTO `json:"items"`
	}
}

type setPrefsInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    struct {
		Items []struct {
			Type    string `json:"type" enum:"MESSAGE_NEW,MESSAGE_REPLY,MATERIAL_ADDED,MATERIAL_BATCH,TASK_CREATED,TASK_PINNED,TASK_DUE_SOON,TASK_OVERDUE,TASK_STATUS_CHANGED,SCHEDULE_CHANGED,MEMBER_JOINED,ANNOUNCEMENT,REMINDER,PROPOSAL_NEW,MODERATION"`
			Enabled bool   `json:"enabled"`
		} `json:"items" maxItems:"40"`
	}
}

type settingsOutput struct {
	Body NotificationSettingsDTO
}

type saveSettingsInput struct {
	Body NotificationSettingsDTO
}

type mutesInput struct {
	GroupID string `query:"group_id" format:"uuid" doc:"Только эта группа; пусто — все."`
}

type mutesOutput struct {
	Body struct {
		Items []MuteDTO `json:"items"`
	}
}

type muteInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    struct {
		ScopeType string    `json:"scope_type" enum:"GROUP,SUBJECT,THREAD,TYPE"`
		ScopeID   string    `json:"scope_id,omitempty" doc:"id предмета или обсуждения; для TYPE — тип уведомления; для GROUP не нужен."`
		Until     time.Time `json:"until" doc:"До какого момента молчать: тишина всегда заканчивается."`
	}
}

type muteOutput struct {
	Body MuteDTO
}

type unmuteInput struct {
	GroupID   string `path:"groupId" format:"uuid"`
	ScopeType string `query:"scope_type" required:"true" enum:"GROUP,SUBJECT,THREAD,TYPE"`
	ScopeID   string `query:"scope_id"`
}

func registerNotifications(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "notifications-list", Method: nethttp.MethodGet, Path: "/me/notifications", Tags: []string{"notifications"}, Security: bearer,
		Summary:     "Мои уведомления",
		Description: "Новые сверху. Уведомление хранится для каждого участника отдельно; в списке только свои.",
	}, func(ctx context.Context, in *notificationsInput) (*notificationsOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseOptionalID("group_id", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		page, err := d.Notify.List(ctx, p.UserID, groupID, in.UnreadOnly, in.Cursor, in.Limit)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &notificationsOutput{}
		out.Body.Items = make([]NotificationDTO, len(page.Items))
		for i := range page.Items {
			out.Body.Items[i] = toNotificationDTO(&page.Items[i])
		}
		out.Body.Unread, out.Body.NextCursor = page.Unread, page.Next
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-unread", Method: nethttp.MethodGet, Path: "/me/notifications/unread", Tags: []string{"notifications"}, Security: bearer,
		Summary: "Сколько непрочитанных", Description: "Для бейджа на иконке.",
	}, func(ctx context.Context, in *mutesInput) (*unreadOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseOptionalID("group_id", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		n, err := d.Notify.Unread(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &unreadOutput{}
		out.Body.Unread = n
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-read", Method: nethttp.MethodPost, Path: "/me/notifications/read", Tags: []string{"notifications"}, Security: bearer,
		Summary: "Пометить прочитанными",
	}, func(ctx context.Context, in *readNotificationsInput) (*readNotificationsOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		var groupID *uuid.UUID
		if in.Body.GroupID != nil {
			if groupID, err = parseOptionalID("group_id", *in.Body.GroupID); err != nil {
				return nil, apiErr(d.Log, err)
			}
		}
		ids, err := parseIDs("ids", in.Body.IDs)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		marked, err := d.Notify.MarkRead(ctx, p.UserID, groupID, ids, in.Body.All)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		unread, err := d.Notify.Unread(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &readNotificationsOutput{}
		out.Body.Marked, out.Body.Unread = marked, unread
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notification-prefs", Method: nethttp.MethodGet, Path: "/groups/{groupId}/notification-prefs", Tags: []string{"notifications"}, Security: bearer,
		Summary:     "Что присылать в этой группе",
		Description: "Все типы уведомлений со значением по умолчанию, если участник его не менял.",
	}, func(ctx context.Context, in *prefsInput) (*prefsOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		prefs, err := d.Notify.Prefs(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &prefsOutput{}
		out.Body.Items = make([]NotificationPrefDTO, len(prefs))
		for i, pref := range prefs {
			out.Body.Items[i] = toPrefDTO(pref)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notification-prefs-set", Method: nethttp.MethodPut, Path: "/groups/{groupId}/notification-prefs", Tags: []string{"notifications"}, Security: bearer,
		Summary: "Изменить, что присылать",
	}, func(ctx context.Context, in *setPrefsInput) (*prefsOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		want := make(map[domain.NotificationType]bool, len(in.Body.Items))
		for _, item := range in.Body.Items {
			want[domain.NotificationType(item.Type)] = item.Enabled
		}
		prefs, err := d.Notify.SetPrefs(ctx, p.UserID, groupID, want)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &prefsOutput{}
		out.Body.Items = make([]NotificationPrefDTO, len(prefs))
		for i, pref := range prefs {
			out.Body.Items[i] = toPrefDTO(pref)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notification-settings", Method: nethttp.MethodGet, Path: "/me/notification-settings", Tags: []string{"notifications"}, Security: bearer,
		Summary: "Push и тихие часы",
	}, func(ctx context.Context, _ *struct{}) (*settingsOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		s, err := d.Notify.Settings(ctx, p.UserID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &settingsOutput{Body: toSettingsDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notification-settings-save", Method: nethttp.MethodPut, Path: "/me/notification-settings", Tags: []string{"notifications"}, Security: bearer,
		Summary: "Изменить push и тихие часы",
	}, func(ctx context.Context, in *saveSettingsInput) (*settingsOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		s, err := d.Notify.SaveSettings(ctx, p.UserID, notify.SettingsInput{
			PushEnabled: in.Body.PushEnabled, QuietFrom: in.Body.QuietFrom, QuietTo: in.Body.QuietTo,
			UrgentInQuiet: in.Body.UrgentInQuiet,
		})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &settingsOutput{Body: toSettingsDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notification-mutes", Method: nethttp.MethodGet, Path: "/me/notification-mutes", Tags: []string{"notifications"}, Security: bearer,
		Summary: "Что сейчас молчит",
	}, func(ctx context.Context, in *mutesInput) (*mutesOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseOptionalID("group_id", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		mutes, err := d.Notify.Mutes(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &mutesOutput{}
		out.Body.Items = make([]MuteDTO, len(mutes))
		for i := range mutes {
			out.Body.Items[i] = toMuteDTO(&mutes[i])
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notification-mute", Method: nethttp.MethodPost, Path: "/groups/{groupId}/notification-mutes", Tags: []string{"notifications"}, Security: bearer,
		Summary:     "Приглушить на время",
		Description: "Предмет, обсуждение, тип уведомлений или всю группу — до указанного момента (не дольше года).",
	}, func(ctx context.Context, in *muteInput) (*muteOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		m, err := d.Notify.Mute(ctx, p.UserID, groupID, domain.MuteScope(in.Body.ScopeType), in.Body.ScopeID, in.Body.Until)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &muteOutput{Body: toMuteDTO(m)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notification-unmute", Method: nethttp.MethodDelete, Path: "/groups/{groupId}/notification-mutes", Tags: []string{"notifications"}, Security: bearer,
		Summary: "Вернуть звук", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *unmuteInput) (*emptyOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Notify.Unmute(ctx, p.UserID, groupID, domain.MuteScope(in.ScopeType), in.ScopeID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})
}
