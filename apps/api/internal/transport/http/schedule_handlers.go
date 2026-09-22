package http

import (
	"context"
	nethttp "net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"heatseeker/api/internal/app/schedule"
	"heatseeker/api/internal/domain"
)

// ScheduleEventBody is the class form.
type ScheduleEventBody struct {
	ClientID  *string    `json:"client_id,omitempty" format:"uuid" doc:"Идемпотентность создания: повтор с тем же id вернёт созданное занятие."`
	SubjectID *string    `json:"subject_id,omitempty" format:"uuid"`
	Title     string     `json:"title,omitempty" maxLength:"200" doc:"Можно не заполнять, если выбран предмет."`
	Kind      *string    `json:"kind,omitempty" enum:"LECTURE,SEMINAR,LAB,EXAM,CONSULTATION,OTHER"`
	StartsAt  time.Time  `json:"starts_at" doc:"Первое занятие серии."`
	EndsAt    time.Time  `json:"ends_at"`
	Timezone  string     `json:"timezone" minLength:"1" maxLength:"64" doc:"Часовой пояс IANA (Europe/Moscow): серия держит время по его часам."`
	Location  string     `json:"location,omitempty" maxLength:"200"`
	Teacher   string     `json:"teacher,omitempty" maxLength:"200"`
	Note      string     `json:"note,omitempty" maxLength:"2000"`
	Repeat    *RepeatDTO `json:"repeat,omitempty" doc:"Нет — разовое занятие."`
	Until     *string    `json:"until,omitempty" format:"date" doc:"Последний день серии включительно."`
}

func (b *ScheduleEventBody) input() (schedule.Input, error) {
	in := schedule.Input{
		Title: b.Title, StartsAt: b.StartsAt, EndsAt: b.EndsAt, Timezone: b.Timezone,
		Location: b.Location, Teacher: b.Teacher, Note: b.Note,
	}
	var err error
	if b.ClientID != nil {
		if in.ClientID, err = parseOptionalID("client_id", *b.ClientID); err != nil {
			return in, err
		}
	}
	if b.SubjectID != nil {
		if in.SubjectID, err = parseOptionalID("subject_id", *b.SubjectID); err != nil {
			return in, err
		}
	}
	if b.Kind != nil {
		k := domain.ScheduleKind(*b.Kind)
		in.Kind = &k
	}
	if b.Repeat != nil {
		r := &domain.Recurrence{IntervalWeeks: b.Repeat.IntervalWeeks}
		for _, code := range b.Repeat.Weekdays {
			d, ok := domain.ParseWeekday(code)
			if !ok {
				return in, domain.Invalid("repeat.weekdays", "unknown weekday "+code)
			}
			r.Weekdays = append(r.Weekdays, d)
		}
		in.Repeat = r
	}
	if in.Until, err = parseOptionalDate("until", b.Until); err != nil {
		return in, err
	}
	return in, nil
}

func parseOptionalDate(field string, raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	d, err := domain.ParseDate(*raw)
	if err != nil {
		return nil, domain.Invalid(field, "must be a date YYYY-MM-DD")
	}
	return &d, nil
}

type scheduleWindowInput struct {
	GroupID string    `path:"groupId" format:"uuid"`
	From    time.Time `query:"from" required:"true" doc:"Начало окна (момент времени; обычно начало недели по часам телефона)."`
	To      time.Time `query:"to" required:"true" doc:"Конец окна, не включая. Окно — до 190 дней."`
}

type createScheduleEventInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    ScheduleEventBody
}

type scheduleEventIDInput struct {
	EventID string `path:"eventId" format:"uuid"`
}

type updateScheduleEventInput struct {
	EventID string `path:"eventId" format:"uuid"`
	Body    struct {
		ScheduleEventBody
		Version int32   `json:"version" minimum:"1" doc:"Версия, которую видел редактор; иначе 409."`
		Scope   string  `json:"scope,omitempty" enum:"ALL,FOLLOWING" default:"ALL" doc:"ALL — вся серия; FOLLOWING — занятия начиная с from (серия делится на две)."`
		From    *string `json:"from,omitempty" format:"date" doc:"Первый день, который меняется, при scope=FOLLOWING."`
	}
}

type deleteScheduleEventInput struct {
	EventID string `path:"eventId" format:"uuid"`
	Version int32  `query:"version" required:"true" minimum:"1"`
	Scope   string `query:"scope" enum:"ALL,FOLLOWING" default:"ALL"`
	From    string `query:"from" format:"date" doc:"Первый удаляемый день при scope=FOLLOWING."`
}

type occurrenceInput struct {
	EventID string `path:"eventId" format:"uuid"`
	Date    string `path:"date" format:"date" doc:"День, на который занятие запланировано."`
}

type setOccurrenceInput struct {
	EventID string `path:"eventId" format:"uuid"`
	Date    string `path:"date" format:"date"`
	Body    struct {
		Cancelled bool       `json:"cancelled,omitempty" doc:"Отменить занятие; остальные поля, кроме note, не нужны."`
		StartsAt  *time.Time `json:"starts_at,omitempty" doc:"Новое время (вместе с ends_at)."`
		EndsAt    *time.Time `json:"ends_at,omitempty"`
		Location  *string    `json:"location,omitempty" maxLength:"200"`
		Teacher   *string    `json:"teacher,omitempty" maxLength:"200"`
		Note      string     `json:"note,omitempty" maxLength:"500" doc:"Причина: «перенос из-за праздника»."`
	}
}

type calendarInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Token   string `query:"token" required:"true" doc:"Из /groups/{groupId}/schedule/calendar-link."`
}

type occurrencesOutput struct {
	Body struct {
		Items []OccurrenceDTO `json:"items"`
	}
}

type occurrenceOutput struct {
	Body OccurrenceDTO
}

type scheduleEventOutput struct {
	Body ScheduleEventDTO
}

type calendarLinkOutput struct {
	Body struct {
		URL       string    `json:"url" doc:"Ссылка для подписки в календаре телефона или Google Календаре."`
		ExpiresAt time.Time `json:"expires_at"`
	}
}

func registerSchedule(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "schedule-window", Method: nethttp.MethodGet, Path: "/groups/{groupId}/schedule", Tags: []string{"schedule"}, Security: bearer,
		Summary:     "Занятия за период",
		Description: "Серии разворачиваются в занятия; отменённые тоже возвращаются (status=CANCELLED), перенесённые — на новом месте.",
	}, func(ctx context.Context, in *scheduleWindowInput) (*occurrencesOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		list, err := d.Schedule.Window(ctx, p.UserID, groupID, in.From, in.To)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &occurrencesOutput{}
		out.Body.Items = make([]OccurrenceDTO, len(list))
		for i := range list {
			out.Body.Items[i] = toOccurrenceDTO(&list[i])
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/schedule/events", Tags: []string{"schedule"}, Security: bearer,
		Summary: "Добавить занятие", Description: "Разовое или еженедельное (каждую неделю, через неделю). Староста или админ с защищённым аккаунтом.",
		DefaultStatus: nethttp.StatusCreated,
	}, func(ctx context.Context, in *createScheduleEventInput) (*scheduleEventOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		si, err := in.Body.input()
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		details, err := d.Schedule.Create(ctx, p.UserID, groupID, si)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &scheduleEventOutput{Body: toScheduleEventDTO(details)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-get", Method: nethttp.MethodGet, Path: "/schedule/events/{eventId}", Tags: []string{"schedule"}, Security: bearer,
		Summary: "Занятие или серия", Description: "С отменами и изменениями отдельных занятий.",
	}, func(ctx context.Context, in *scheduleEventIDInput) (*scheduleEventOutput, error) {
		p, id, err := principalAndID(ctx, "eventId", in.EventID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		details, err := d.Schedule.Get(ctx, p.UserID, id)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &scheduleEventOutput{Body: toScheduleEventDTO(details)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-update", Method: nethttp.MethodPatch, Path: "/schedule/events/{eventId}", Tags: []string{"schedule"}, Security: bearer,
		Summary:     "Изменить занятие или серию",
		Description: "Поля заменяются целиком. scope=FOLLOWING меняет занятия с from: старая серия заканчивается накануне, ответ — новая серия.",
	}, func(ctx context.Context, in *updateScheduleEventInput) (*scheduleEventOutput, error) {
		p, id, err := principalAndID(ctx, "eventId", in.EventID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		si, err := in.Body.input()
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		from, err := parseOptionalDate("from", in.Body.From)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		details, err := d.Schedule.Update(ctx, p.UserID, id, schedule.UpdateInput{
			Input: si, Version: in.Body.Version, Scope: schedule.Scope(in.Body.Scope), From: from,
		})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &scheduleEventOutput{Body: toScheduleEventDTO(details)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-delete", Method: nethttp.MethodDelete, Path: "/schedule/events/{eventId}", Tags: []string{"schedule"}, Security: bearer,
		Summary:       "Удалить занятие или серию",
		Description:   "scope=FOLLOWING удаляет занятия начиная с from; чтобы убрать одно занятие, его отменяют.",
		DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *deleteScheduleEventInput) (*struct{}, error) {
		p, id, err := principalAndID(ctx, "eventId", in.EventID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		from, err := parseOptionalDate("from", &in.From)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Schedule.Delete(ctx, p.UserID, id, in.Version, schedule.Scope(in.Scope), from); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-occurrence-get", Method: nethttp.MethodGet, Path: "/schedule/events/{eventId}/occurrences/{date}", Tags: []string{"schedule"}, Security: bearer,
		Summary: "Одно занятие", Description: "С отменой или изменением, если они есть.",
	}, func(ctx context.Context, in *occurrenceInput) (*occurrenceOutput, error) {
		p, id, date, err := occurrenceAddress(ctx, in.EventID, in.Date)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		o, err := d.Schedule.GetOccurrence(ctx, p.UserID, id, date)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &occurrenceOutput{Body: toOccurrenceDTO(o)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-occurrence-set", Method: nethttp.MethodPut, Path: "/schedule/events/{eventId}/occurrences/{date}", Tags: []string{"schedule"}, Security: bearer,
		Summary:     "Отменить или изменить одно занятие",
		Description: "Время, аудитория, преподаватель или причина; остальные занятия серии не меняются.",
	}, func(ctx context.Context, in *setOccurrenceInput) (*occurrenceOutput, error) {
		p, id, date, err := occurrenceAddress(ctx, in.EventID, in.Date)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		b := in.Body
		o, err := d.Schedule.SetOccurrence(ctx, p.UserID, id, date, schedule.OccurrenceInput{
			Cancelled: b.Cancelled, StartsAt: b.StartsAt, EndsAt: b.EndsAt, Location: b.Location, Teacher: b.Teacher, Note: b.Note,
		})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &occurrenceOutput{Body: toOccurrenceDTO(o)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-occurrence-reset", Method: nethttp.MethodDelete, Path: "/schedule/events/{eventId}/occurrences/{date}", Tags: []string{"schedule"}, Security: bearer,
		Summary: "Вернуть занятие как по плану", Description: "Снимает отмену или изменение одного занятия.",
	}, func(ctx context.Context, in *occurrenceInput) (*occurrenceOutput, error) {
		p, id, date, err := occurrenceAddress(ctx, in.EventID, in.Date)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		o, err := d.Schedule.ResetOccurrence(ctx, p.UserID, id, date)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &occurrenceOutput{Body: toOccurrenceDTO(o)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-calendar-link", Method: nethttp.MethodGet, Path: "/groups/{groupId}/schedule/calendar-link", Tags: []string{"schedule"}, Security: bearer,
		Summary: "Ссылка на расписание для календаря", Description: "Личная ссылка на ICS; работает, пока вы в группе.",
	}, func(ctx context.Context, in *groupIDInput) (*calendarLinkOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		link, exp, err := d.Schedule.CalendarLink(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &calendarLinkOutput{}
		out.Body.URL, out.Body.ExpiresAt = link, exp
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-calendar", Method: nethttp.MethodGet, Path: "/groups/{groupId}/schedule.ics", Tags: []string{"schedule"},
		Summary:   "Расписание в формате iCalendar",
		Responses: map[string]*huma.Response{"200": {Description: "text/calendar", Content: map[string]*huma.MediaType{"text/calendar": {Schema: &huma.Schema{Type: "string"}}}}},
	}, func(ctx context.Context, in *calendarInput) (*huma.StreamResponse, error) {
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		body, err := d.Schedule.Calendar(ctx, groupID, in.Token)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &huma.StreamResponse{Body: func(ctx huma.Context) {
			ctx.SetHeader("Content-Type", "text/calendar; charset=utf-8")
			ctx.SetHeader("Content-Disposition", `inline; filename="schedule.ics"`)
			ctx.SetHeader("Cache-Control", "private, max-age=300")
			ctx.SetHeader("Content-Length", strconv.Itoa(len(body)))
			ctx.SetStatus(nethttp.StatusOK)
			_, _ = ctx.BodyWriter().Write(body)
		}}, nil
	})
}

func occurrenceAddress(ctx context.Context, rawID, rawDate string) (Principal, uuid.UUID, time.Time, error) {
	p, id, err := principalAndID(ctx, "eventId", rawID)
	if err != nil {
		return p, id, time.Time{}, err
	}
	date, err := domain.ParseDate(rawDate)
	if err != nil {
		return p, id, time.Time{}, domain.Invalid("date", "must be a date YYYY-MM-DD")
	}
	return p, id, date, nil
}
