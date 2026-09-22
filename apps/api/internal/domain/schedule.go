package domain

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ScheduleKind is what kind of class an event is.
type ScheduleKind string

// Schedule kinds.
const (
	ScheduleLecture      ScheduleKind = "LECTURE"
	ScheduleSeminar      ScheduleKind = "SEMINAR"
	ScheduleLab          ScheduleKind = "LAB"
	ScheduleExam         ScheduleKind = "EXAM"
	ScheduleConsultation ScheduleKind = "CONSULTATION"
	ScheduleOther        ScheduleKind = "OTHER"
)

// AllScheduleKinds lists kinds in display order.
var AllScheduleKinds = []ScheduleKind{
	ScheduleLecture, ScheduleSeminar, ScheduleLab, ScheduleExam, ScheduleConsultation, ScheduleOther,
}

// DateLayout is how civil dates travel in URLs and payloads.
const DateLayout = "2006-01-02"

// CivilDate is the calendar date of t as seen on its own clock, as midnight
// UTC: the representation of every "date without time" in the schedule.
func CivilDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ParseDate reads a YYYY-MM-DD date.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}

// Recurrence repeats a class every IntervalWeeks weeks on the given days.
// It is stored as an RFC 5545 RRULE subset: FREQ=WEEKLY;INTERVAL=n;BYDAY=MO,TH.
type Recurrence struct {
	// IntervalWeeks is 1 for every week, 2 for every other week.
	IntervalWeeks int
	// Weekdays in week order; empty — the weekday of the first class.
	Weekdays []time.Weekday
}

// MaxIntervalWeeks bounds the interval: more is not a weekly class.
const MaxIntervalWeeks = 4

var weekdayCodes = map[time.Weekday]string{
	time.Monday: "MO", time.Tuesday: "TU", time.Wednesday: "WE", time.Thursday: "TH",
	time.Friday: "FR", time.Saturday: "SA", time.Sunday: "SU",
}

// WeekdayCode is the RFC 5545 name of a day ("MO").
func WeekdayCode(d time.Weekday) string { return weekdayCodes[d] }

// ParseWeekday reads an RFC 5545 day name.
func ParseWeekday(code string) (time.Weekday, bool) {
	for d, c := range weekdayCodes {
		if c == code {
			return d, true
		}
	}
	return 0, false
}

// mondayIndex orders days from Monday.
func mondayIndex(d time.Weekday) int { return (int(d) + 6) % 7 }

// Normalize validates the rule and sorts its days from Monday.
func (r *Recurrence) Normalize() error {
	if r.IntervalWeeks == 0 {
		r.IntervalWeeks = 1
	}
	if r.IntervalWeeks < 1 || r.IntervalWeeks > MaxIntervalWeeks {
		return Invalid("repeat.interval_weeks", fmt.Sprintf("must be 1–%d", MaxIntervalWeeks))
	}
	days := make([]time.Weekday, 0, len(r.Weekdays))
	for _, d := range r.Weekdays {
		if d < time.Sunday || d > time.Saturday {
			return Invalid("repeat.weekdays", "unknown weekday")
		}
		if !slices.Contains(days, d) {
			days = append(days, d)
		}
	}
	sort.Slice(days, func(i, j int) bool { return mondayIndex(days[i]) < mondayIndex(days[j]) })
	r.Weekdays = days
	return nil
}

// RRule encodes the rule for storage and calendar export (without UNTIL).
func (r Recurrence) RRule() string {
	parts := []string{"FREQ=WEEKLY"}
	if r.IntervalWeeks > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(r.IntervalWeeks))
	}
	if len(r.Weekdays) > 0 {
		codes := make([]string, len(r.Weekdays))
		for i, d := range r.Weekdays {
			codes[i] = WeekdayCode(d)
		}
		parts = append(parts, "BYDAY="+strings.Join(codes, ","))
	}
	return strings.Join(parts, ";")
}

// ParseRRule reads the supported RRULE subset.
func ParseRRule(s string) (*Recurrence, error) {
	r := &Recurrence{IntervalWeeks: 1}
	weekly := false
	for _, part := range strings.Split(strings.TrimSpace(s), ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("rrule %q: bad part %q", s, part)
		}
		switch strings.ToUpper(key) {
		case "FREQ":
			if strings.ToUpper(value) != "WEEKLY" {
				return nil, fmt.Errorf("rrule %q: only weekly rules are supported", s)
			}
			weekly = true
		case "INTERVAL":
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("rrule %q: bad interval", s)
			}
			r.IntervalWeeks = n
		case "BYDAY":
			for _, code := range strings.Split(value, ",") {
				d, ok := ParseWeekday(strings.ToUpper(code))
				if !ok {
					return nil, fmt.Errorf("rrule %q: bad day %q", s, code)
				}
				r.Weekdays = append(r.Weekdays, d)
			}
		default:
			return nil, fmt.Errorf("rrule %q: %s is not supported", s, key)
		}
	}
	if !weekly {
		return nil, fmt.Errorf("rrule %q: FREQ is required", s)
	}
	if err := r.Normalize(); err != nil {
		return nil, fmt.Errorf("rrule %q: %w", s, err)
	}
	return r, nil
}

// ScheduleEvent is a class: one-off, or a weekly series starting with the
// first occurrence StartsAt–EndsAt.
type ScheduleEvent struct {
	ID        uuid.UUID
	GroupID   uuid.UUID
	SubjectID *uuid.UUID
	ClientID  *uuid.UUID
	Title     string
	Kind      ScheduleKind
	StartsAt  time.Time
	EndsAt    time.Time
	// Timezone is the IANA zone whose wall clock the series follows.
	Timezone string
	Location string
	Teacher  string
	Note     string
	// Repeat is nil for a one-off class.
	Repeat *Recurrence
	// Until is the last date of a series (civil, inclusive); nil — no end.
	Until     *time.Time
	CreatedBy *uuid.UUID
	UpdatedBy *uuid.UUID
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Loc is the event's time zone.
func (e *ScheduleEvent) Loc() (*time.Location, error) {
	return time.LoadLocation(e.Timezone)
}

// FirstDate is the civil date of the first occurrence.
func (e *ScheduleEvent) FirstDate(loc *time.Location) time.Time {
	return CivilDate(e.StartsAt.In(loc))
}

// OccursOn reports whether the class is planned on a civil date.
func (e *ScheduleEvent) OccursOn(date time.Time, loc *time.Location) bool {
	first := e.FirstDate(loc)
	if date.Before(first) {
		return false
	}
	if e.Repeat == nil {
		return date.Equal(first)
	}
	if e.Until != nil && date.After(*e.Until) {
		return false
	}
	days := e.Repeat.Weekdays
	if len(days) == 0 {
		days = []time.Weekday{first.Weekday()}
	}
	if !slices.Contains(days, date.Weekday()) {
		return false
	}
	weekOf := func(d time.Time) time.Time { return d.AddDate(0, 0, -mondayIndex(d.Weekday())) }
	weeks := int(weekOf(date).Sub(weekOf(first)).Hours() / 24 / 7)
	return weeks%e.Repeat.IntervalWeeks == 0
}

// PlannedAt is when the class on a civil date starts and ends by the plan:
// the first class's wall clock on that date.
func (e *ScheduleEvent) PlannedAt(date time.Time, loc *time.Location) (time.Time, time.Time) {
	first := e.StartsAt.In(loc)
	start := time.Date(date.Year(), date.Month(), date.Day(), first.Hour(), first.Minute(), first.Second(), 0, loc)
	return start, start.Add(e.EndsAt.Sub(e.StartsAt))
}

// ScheduleExceptionKind says what happened to one occurrence.
type ScheduleExceptionKind string

// Exception kinds.
const (
	ExceptionCancelled ScheduleExceptionKind = "CANCELLED"
	ExceptionChanged   ScheduleExceptionKind = "CHANGED"
)

// ScheduleException cancels or changes the occurrence planned on Date.
type ScheduleException struct {
	EventID uuid.UUID
	// Date is the civil date the occurrence was planned for.
	Date time.Time
	Kind ScheduleExceptionKind
	// Overrides; nil keeps the series value.
	StartsAt  *time.Time
	EndsAt    *time.Time
	Location  *string
	Teacher   *string
	Note      string
	UpdatedBy *uuid.UUID
	UpdatedAt time.Time
}

// OccurrenceStatus is how an occurrence stands against the plan.
type OccurrenceStatus string

// Occurrence statuses.
const (
	OccurrenceScheduled OccurrenceStatus = "SCHEDULED"
	OccurrenceCancelled OccurrenceStatus = "CANCELLED"
	OccurrenceChanged   OccurrenceStatus = "CHANGED"
)

// Occurrence is one class on the calendar.
type Occurrence struct {
	Event *ScheduleEvent
	// Date is the civil date it was planned for: with EventID, its identity.
	Date     time.Time
	StartsAt time.Time
	EndsAt   time.Time
	Location string
	Teacher  string
	Status   OccurrenceStatus
	// ChangeNote explains a cancellation or a change.
	ChangeNote string
	// PlannedStartsAt differs from StartsAt when the class was moved.
	PlannedStartsAt time.Time
}

// Moved reports whether the class takes place at another time than planned.
func (o *Occurrence) Moved() bool { return !o.StartsAt.Equal(o.PlannedStartsAt) }

// Occurrence builds the class planned on a civil date with its exception
// applied (ex may be nil).
func (e *ScheduleEvent) Occurrence(date time.Time, loc *time.Location, ex *ScheduleException) Occurrence {
	start, end := e.PlannedAt(date, loc)
	o := Occurrence{
		Event: e, Date: date, StartsAt: start, EndsAt: end, Location: e.Location, Teacher: e.Teacher,
		Status: OccurrenceScheduled, PlannedStartsAt: start,
	}
	if ex == nil {
		return o
	}
	o.ChangeNote = ex.Note
	if ex.Kind == ExceptionCancelled {
		o.Status = OccurrenceCancelled
		return o
	}
	o.Status = OccurrenceChanged
	if ex.StartsAt != nil && ex.EndsAt != nil {
		o.StartsAt, o.EndsAt = *ex.StartsAt, *ex.EndsAt
	}
	if ex.Location != nil {
		o.Location = *ex.Location
	}
	if ex.Teacher != nil {
		o.Teacher = *ex.Teacher
	}
	return o
}

// Occurrences expands the event into the classes overlapping [from, to),
// cancelled ones included, with exceptions applied: a class moved into the
// window from a date outside it is there, one moved out of it is not.
func (e *ScheduleEvent) Occurrences(from, to time.Time, exceptions []ScheduleException) ([]Occurrence, error) {
	loc, err := e.Loc()
	if err != nil {
		return nil, fmt.Errorf("event %s: %w", e.ID, err)
	}
	byDate := make(map[string]*ScheduleException, len(exceptions))
	for i := range exceptions {
		byDate[exceptions[i].Date.Format(DateLayout)] = &exceptions[i]
	}
	overlaps := func(o *Occurrence) bool { return o.StartsAt.Before(to) && o.EndsAt.After(from) }

	var out []Occurrence
	seen := map[string]bool{}
	// A class may start the day before the window and still overlap it.
	first := CivilDate(from.Add(-e.EndsAt.Sub(e.StartsAt)).In(loc)).AddDate(0, 0, -1)
	if fd := e.FirstDate(loc); first.Before(fd) {
		first = fd
	}
	last := CivilDate(to.In(loc)).AddDate(0, 0, 1)
	if e.Repeat == nil {
		last = minTime(last, e.FirstDate(loc))
	} else if e.Until != nil {
		last = minTime(last, *e.Until)
	}
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		if !e.OccursOn(d, loc) {
			continue
		}
		key := d.Format(DateLayout)
		seen[key] = true
		if o := e.Occurrence(d, loc, byDate[key]); overlaps(&o) {
			out = append(out, o)
		}
	}
	for key, ex := range byDate {
		if seen[key] || !e.OccursOn(ex.Date, loc) {
			continue
		}
		if o := e.Occurrence(ex.Date, loc, ex); overlaps(&o) {
			out = append(out, o)
		}
	}
	return out, nil
}

// SortOccurrences orders classes by start, then by title.
func SortOccurrences(list []Occurrence) {
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].StartsAt.Equal(list[j].StartsAt) {
			return list[i].StartsAt.Before(list[j].StartsAt)
		}
		return list[i].Event.Title < list[j].Event.Title
	})
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
