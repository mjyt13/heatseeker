package http

import (
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/schedule"
	"heatseeker/api/internal/domain"
)

// RepeatDTO is a weekly rule.
type RepeatDTO struct {
	IntervalWeeks int      `json:"interval_weeks" minimum:"1" maximum:"4" doc:"1 — каждую неделю, 2 — через неделю (числитель/знаменатель)."`
	Weekdays      []string `json:"weekdays,omitempty" maxItems:"7" enum:"MO,TU,WE,TH,FR,SA,SU" doc:"Дни недели; пусто — день первого занятия (он добавляется всегда)."`
}

// ScheduleExceptionDTO is one class cancelled or changed.
type ScheduleExceptionDTO struct {
	Date     string     `json:"date" format:"date" doc:"День, на который занятие было запланировано."`
	Kind     string     `json:"kind" enum:"CANCELLED,CHANGED"`
	StartsAt *time.Time `json:"starts_at,omitempty"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
	Location *string    `json:"location,omitempty"`
	Teacher  *string    `json:"teacher,omitempty"`
	Note     string     `json:"note,omitempty"`
}

// ScheduleEventDTO is a class or a series as the editor shows it.
type ScheduleEventDTO struct {
	ID         uuid.UUID              `json:"id"`
	GroupID    uuid.UUID              `json:"group_id"`
	SubjectID  *uuid.UUID             `json:"subject_id,omitempty"`
	Title      string                 `json:"title" doc:"Пусто — показывается название предмета."`
	Kind       string                 `json:"kind" enum:"LECTURE,SEMINAR,LAB,EXAM,CONSULTATION,OTHER"`
	StartsAt   time.Time              `json:"starts_at" doc:"Первое занятие."`
	EndsAt     time.Time              `json:"ends_at"`
	Timezone   string                 `json:"timezone"`
	Location   string                 `json:"location"`
	Teacher    string                 `json:"teacher"`
	Note       string                 `json:"note"`
	Repeat     *RepeatDTO             `json:"repeat,omitempty" doc:"Нет — разовое занятие."`
	Until      *string                `json:"until,omitempty" format:"date" doc:"Последний день серии включительно; нет — без конца."`
	Version    int32                  `json:"version" doc:"Передаётся при правке и удалении."`
	CreatedBy  *uuid.UUID             `json:"created_by,omitempty"`
	UpdatedBy  *uuid.UUID             `json:"updated_by,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Exceptions []ScheduleExceptionDTO `json:"exceptions"`
}

// OccurrenceDTO is one class on the calendar.
type OccurrenceDTO struct {
	EventID         uuid.UUID  `json:"event_id"`
	Date            string     `json:"date" format:"date" doc:"День по плану; вместе с event_id — адрес занятия."`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          time.Time  `json:"ends_at"`
	PlannedStartsAt *time.Time `json:"planned_starts_at,omitempty" doc:"Есть, когда занятие перенесено на другое время."`
	Timezone        string     `json:"timezone"`
	SubjectID       *uuid.UUID `json:"subject_id,omitempty"`
	Title           string     `json:"title"`
	Kind            string     `json:"kind" enum:"LECTURE,SEMINAR,LAB,EXAM,CONSULTATION,OTHER"`
	Location        string     `json:"location"`
	Teacher         string     `json:"teacher"`
	Note            string     `json:"note"`
	Status          string     `json:"status" enum:"SCHEDULED,CANCELLED,CHANGED"`
	ChangeNote      string     `json:"change_note,omitempty" doc:"Почему отменено или изменено."`
	Recurring       bool       `json:"recurring"`
	Version         int32      `json:"version" doc:"Версия серии — для правки из карточки занятия."`
}

func toRepeatDTO(r *domain.Recurrence) *RepeatDTO {
	if r == nil {
		return nil
	}
	out := &RepeatDTO{IntervalWeeks: r.IntervalWeeks}
	for _, d := range r.Weekdays {
		out.Weekdays = append(out.Weekdays, domain.WeekdayCode(d))
	}
	return out
}

func toScheduleEventDTO(d *schedule.Details) ScheduleEventDTO {
	e := d.Event
	out := ScheduleEventDTO{
		ID: e.ID, GroupID: e.GroupID, SubjectID: e.SubjectID, Title: e.Title, Kind: string(e.Kind),
		StartsAt: e.StartsAt, EndsAt: e.EndsAt, Timezone: e.Timezone, Location: e.Location, Teacher: e.Teacher,
		Note: e.Note, Repeat: toRepeatDTO(e.Repeat), Version: e.Version, CreatedBy: e.CreatedBy, UpdatedBy: e.UpdatedBy,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Exceptions: make([]ScheduleExceptionDTO, len(d.Exceptions)),
	}
	if e.Until != nil {
		v := e.Until.Format(domain.DateLayout)
		out.Until = &v
	}
	for i, x := range d.Exceptions {
		out.Exceptions[i] = ScheduleExceptionDTO{
			Date: x.Date.Format(domain.DateLayout), Kind: string(x.Kind), StartsAt: x.StartsAt, EndsAt: x.EndsAt,
			Location: x.Location, Teacher: x.Teacher, Note: x.Note,
		}
	}
	return out
}

func toOccurrenceDTO(o *domain.Occurrence) OccurrenceDTO {
	e := o.Event
	out := OccurrenceDTO{
		EventID: e.ID, Date: o.Date.Format(domain.DateLayout), StartsAt: o.StartsAt, EndsAt: o.EndsAt,
		Timezone: e.Timezone, SubjectID: e.SubjectID, Title: e.Title, Kind: string(e.Kind), Location: o.Location,
		Teacher: o.Teacher, Note: e.Note, Status: string(o.Status), ChangeNote: o.ChangeNote,
		Recurring: e.Repeat != nil, Version: e.Version,
	}
	if o.Moved() {
		planned := o.PlannedStartsAt
		out.PlannedStartsAt = &planned
	}
	return out
}
