package http

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"heatseeker/api/internal/app/announcements"
	"heatseeker/api/internal/app/reminders"
	"heatseeker/api/internal/domain"
)

// AnnouncementDTO is one announcement as the group reads it.
type AnnouncementDTO struct {
	ID         uuid.UUID  `json:"id"`
	GroupID    uuid.UUID  `json:"group_id"`
	AuthorID   *uuid.UUID `json:"author_id,omitempty"`
	AuthorName string     `json:"author_name,omitempty"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	Urgent     bool       `json:"urgent" doc:"Срочное приходит на телефон даже в тихие часы."`
	Pinned     bool       `json:"pinned"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func toAnnouncementDTO(a *domain.Announcement, authorName string) AnnouncementDTO {
	return AnnouncementDTO{
		ID: a.ID, GroupID: a.GroupID, AuthorID: a.AuthorID, AuthorName: authorName,
		Title: a.Title, Body: a.Body, Urgent: a.Urgent, Pinned: a.Pinned,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// ReminderDTO is one personal reminder.
type ReminderDTO struct {
	ID         uuid.UUID  `json:"id"`
	GroupID    uuid.UUID  `json:"group_id"`
	Title      string     `json:"title"`
	Note       string     `json:"note"`
	RemindAt   time.Time  `json:"remind_at"`
	Repeat     string     `json:"repeat" enum:"NONE,DAILY,WEEKLY,MONTHLY"`
	TargetType *string    `json:"target_type,omitempty" enum:"TASK,MATERIAL,SCHEDULE"`
	TargetID   *uuid.UUID `json:"target_id,omitempty"`
	Status     string     `json:"status" enum:"SCHEDULED,SENT,DONE"`
	LastFired  *time.Time `json:"last_fired_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toReminderDTO(r *domain.Reminder) ReminderDTO {
	out := ReminderDTO{
		ID: r.ID, GroupID: r.GroupID, Title: r.Title, Note: r.Note, RemindAt: r.RemindAt,
		Repeat: string(r.Repeat), TargetID: r.TargetID, Status: string(r.Status),
		LastFired: r.LastFired, CreatedAt: r.CreatedAt,
	}
	if r.TargetType != nil {
		t := string(*r.TargetType)
		out.TargetType = &t
	}
	return out
}

// AnnouncementBody is the announcement form.
type AnnouncementBody struct {
	Title  string `json:"title" minLength:"1" maxLength:"200"`
	Body   string `json:"body,omitempty" maxLength:"5000" doc:"Markdown."`
	Urgent bool   `json:"urgent,omitempty" doc:"Придёт на телефон даже в тихие часы."`
	Pinned bool   `json:"pinned,omitempty" doc:"Держать наверху списка."`
}

func (b AnnouncementBody) input() announcements.Input {
	return announcements.Input{Title: b.Title, Body: b.Body, Urgent: b.Urgent, Pinned: b.Pinned}
}

type announcementsInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Limit   int32  `query:"limit" minimum:"1" maximum:"100" default:"50"`
}

type announcementsOutput struct {
	Body struct {
		Items []AnnouncementDTO `json:"items"`
	}
}

type createAnnouncementInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    AnnouncementBody
}

type updateAnnouncementInput struct {
	AnnouncementID string `path:"announcementId" format:"uuid"`
	Body           AnnouncementBody
}

type announcementIDInput struct {
	AnnouncementID string `path:"announcementId" format:"uuid"`
}

type announcementOutput struct {
	Body AnnouncementDTO
}

// ReminderBody is the reminder form.
type ReminderBody struct {
	Title      string    `json:"title" minLength:"1" maxLength:"200"`
	Note       string    `json:"note,omitempty" maxLength:"2000"`
	RemindAt   time.Time `json:"remind_at" doc:"Момент времени; на телефоне — по его часам."`
	Repeat     *string   `json:"repeat,omitempty" enum:"NONE,DAILY,WEEKLY,MONTHLY"`
	TargetType *string   `json:"target_type,omitempty" enum:"TASK,MATERIAL,SCHEDULE" doc:"К чему привязано; вместе с target_id."`
	TargetID   *string   `json:"target_id,omitempty" format:"uuid"`
}

func (b ReminderBody) input() (reminders.Input, error) {
	in := reminders.Input{Title: b.Title, Note: b.Note, RemindAt: b.RemindAt}
	if b.Repeat != nil {
		in.Repeat = domain.ReminderRepeat(*b.Repeat)
	}
	if b.TargetType != nil {
		t := domain.ReminderTarget(*b.TargetType)
		in.TargetType = &t
	}
	if b.TargetID != nil {
		id, err := parseOptionalID("target_id", *b.TargetID)
		if err != nil {
			return in, err
		}
		in.TargetID = id
	}
	return in, nil
}

type remindersInput struct {
	GroupID  string `path:"groupId" format:"uuid"`
	OpenOnly bool   `query:"open_only" doc:"Только незакрытые."`
	Limit    int32  `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type remindersOutput struct {
	Body struct {
		Items []ReminderDTO `json:"items"`
	}
}

type createReminderInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    ReminderBody
}

type updateReminderInput struct {
	ReminderID string `path:"reminderId" format:"uuid"`
	Body       ReminderBody
}

type reminderIDInput struct {
	ReminderID string `path:"reminderId" format:"uuid"`
}

type snoozeReminderInput struct {
	ReminderID string `path:"reminderId" format:"uuid"`
	Body       struct {
		Minutes int32 `json:"minutes" minimum:"1" maximum:"43200" default:"60" doc:"Через сколько напомнить снова."`
	}
}

type doneReminderInput struct {
	ReminderID string `path:"reminderId" format:"uuid"`
	Body       struct {
		Done bool `json:"done" doc:"false — вернуть напоминание в работу."`
	}
}

type reminderOutput struct {
	Body ReminderDTO
}

func registerAnnouncements(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "announcements-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/announcements",
		Tags: []string{"announcements"}, Security: bearer,
		Summary: "Объявления группы", Description: "Закреплённые сверху, дальше — по времени.",
	}, func(ctx context.Context, in *announcementsInput) (*announcementsOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		items, err := d.Announcements.List(ctx, p.UserID, groupID, in.Limit)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &announcementsOutput{}
		out.Body.Items = make([]AnnouncementDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toAnnouncementDTO(&items[i].Announcement, items[i].AuthorName)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "announcement-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/announcements",
		Tags: []string{"announcements"}, Security: bearer, DefaultStatus: nethttp.StatusCreated,
		Summary:     "Объявить группе",
		Description: "Староста или админ с защищённым аккаунтом. Уведомление получают все; срочное — даже в тихие часы.",
	}, func(ctx context.Context, in *createAnnouncementInput) (*announcementOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		a, err := d.Announcements.Create(ctx, p.UserID, groupID, in.Body.input())
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &announcementOutput{Body: toAnnouncementDTO(a, "")}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "announcement-update", Method: nethttp.MethodPatch, Path: "/announcements/{announcementId}",
		Tags: []string{"announcements"}, Security: bearer,
		Summary: "Изменить объявление", Description: "Повторного уведомления не будет: группа его уже слышала.",
	}, func(ctx context.Context, in *updateAnnouncementInput) (*announcementOutput, error) {
		p, id, err := principalAndID(ctx, "announcementId", in.AnnouncementID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		a, err := d.Announcements.Update(ctx, p.UserID, id, in.Body.input())
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &announcementOutput{Body: toAnnouncementDTO(a, "")}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "announcement-delete", Method: nethttp.MethodDelete, Path: "/announcements/{announcementId}",
		Tags: []string{"announcements"}, Security: bearer, DefaultStatus: nethttp.StatusNoContent,
		Summary: "Снять объявление",
	}, func(ctx context.Context, in *announcementIDInput) (*emptyOutput, error) {
		p, id, err := principalAndID(ctx, "announcementId", in.AnnouncementID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Announcements.Delete(ctx, p.UserID, id); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})
}

func registerReminders(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "reminders-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/reminders",
		Tags: []string{"reminders"}, Security: bearer,
		Summary: "Мои напоминания", Description: "Личные: чужие не видны никому, включая старосту.",
	}, func(ctx context.Context, in *remindersInput) (*remindersOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		items, err := d.Reminders.List(ctx, p.UserID, groupID, in.OpenOnly, in.Limit)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &remindersOutput{}
		out.Body.Items = make([]ReminderDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toReminderDTO(&items[i])
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reminder-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/reminders",
		Tags: []string{"reminders"}, Security: bearer, DefaultStatus: nethttp.StatusCreated,
		Summary: "Поставить напоминание",
	}, func(ctx context.Context, in *createReminderInput) (*reminderOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		body, err := in.Body.input()
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		r, err := d.Reminders.Create(ctx, p.UserID, groupID, body)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &reminderOutput{Body: toReminderDTO(r)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reminder-update", Method: nethttp.MethodPatch, Path: "/reminders/{reminderId}",
		Tags: []string{"reminders"}, Security: bearer,
		Summary: "Изменить напоминание", Description: "Поля заменяются целиком; напоминание снова встаёт в очередь.",
	}, func(ctx context.Context, in *updateReminderInput) (*reminderOutput, error) {
		p, id, err := principalAndID(ctx, "reminderId", in.ReminderID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		body, err := in.Body.input()
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		r, err := d.Reminders.Update(ctx, p.UserID, id, body)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &reminderOutput{Body: toReminderDTO(r)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reminder-snooze", Method: nethttp.MethodPost, Path: "/reminders/{reminderId}/snooze",
		Tags: []string{"reminders"}, Security: bearer,
		Summary: "Отложить",
	}, func(ctx context.Context, in *snoozeReminderInput) (*reminderOutput, error) {
		p, id, err := principalAndID(ctx, "reminderId", in.ReminderID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		r, err := d.Reminders.Snooze(ctx, p.UserID, id, time.Duration(in.Body.Minutes)*time.Minute)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &reminderOutput{Body: toReminderDTO(r)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reminder-done", Method: nethttp.MethodPost, Path: "/reminders/{reminderId}/done",
		Tags: []string{"reminders"}, Security: bearer,
		Summary: "Готово", Description: "Повторяющееся напоминание закрывается совсем.",
	}, func(ctx context.Context, in *doneReminderInput) (*reminderOutput, error) {
		p, id, err := principalAndID(ctx, "reminderId", in.ReminderID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		r, err := d.Reminders.Done(ctx, p.UserID, id, in.Body.Done)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &reminderOutput{Body: toReminderDTO(r)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reminder-delete", Method: nethttp.MethodDelete, Path: "/reminders/{reminderId}",
		Tags: []string{"reminders"}, Security: bearer, DefaultStatus: nethttp.StatusNoContent,
		Summary: "Удалить напоминание",
	}, func(ctx context.Context, in *reminderIDInput) (*emptyOutput, error) {
		p, id, err := principalAndID(ctx, "reminderId", in.ReminderID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Reminders.Delete(ctx, p.UserID, id); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})
}
