package schedule

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

// kindLabels name the kinds in the calendar feed, which calendar apps show
// as is (the feed has no client to translate it).
var kindLabels = map[domain.ScheduleKind]string{
	domain.ScheduleLecture:      "лекция",
	domain.ScheduleSeminar:      "семинар",
	domain.ScheduleLab:          "лабораторная",
	domain.ScheduleExam:         "экзамен",
	domain.ScheduleConsultation: "консультация",
	domain.ScheduleOther:        "",
}

const (
	icsLocal = "20060102T150405"
	icsUTC   = "20060102T150405Z"
)

// buildICS renders events as an RFC 5545 calendar: a series is one VEVENT
// with RRULE, cancelled classes are EXDATEs and changed ones are overriding
// VEVENTs with RECURRENCE-ID.
func buildICS(name string, events []domain.ScheduleEvent, exceptions map[uuid.UUID][]domain.ScheduleException, subjects map[uuid.UUID]string, now time.Time) []byte {
	w := &icsWriter{}
	w.line("BEGIN:VCALENDAR")
	w.line("VERSION:2.0")
	w.line("PRODID:-//Heatseeker//Schedule//RU")
	w.line("CALSCALE:GREGORIAN")
	w.line("METHOD:PUBLISH")
	w.prop("X-WR-CALNAME", name)
	stamp := now.UTC().Format(icsUTC)
	for i := range events {
		e := &events[i]
		loc, err := e.Loc()
		if err != nil {
			continue
		}
		summary := summaryOf(e, subjects)
		local := func(t time.Time) string { return t.In(loc).Format(icsLocal) }
		var changed []domain.Occurrence
		cancelled := false
		var exdates []string
		for j := range exceptions[e.ID] {
			x := &exceptions[e.ID][j]
			if !e.OccursOn(x.Date, loc) {
				continue
			}
			o := e.Occurrence(x.Date, loc, x)
			switch {
			case e.Repeat == nil && o.Status == domain.OccurrenceCancelled:
				cancelled = true
			case e.Repeat == nil:
				changed = append(changed, o)
			case o.Status == domain.OccurrenceCancelled:
				exdates = append(exdates, local(o.PlannedStartsAt))
			default:
				changed = append(changed, o)
			}
		}

		// A one-off class carries its own change.
		main := e.Occurrence(e.FirstDate(loc), loc, nil)
		if e.Repeat == nil && len(changed) == 1 {
			main, changed = changed[0], nil
		}
		w.line("BEGIN:VEVENT")
		w.line("UID:" + e.ID.String() + "@heatseeker")
		w.line("DTSTAMP:" + stamp)
		w.line("DTSTART;TZID=" + e.Timezone + ":" + local(main.StartsAt))
		w.line("DTEND;TZID=" + e.Timezone + ":" + local(main.EndsAt))
		if e.Repeat != nil {
			rule := e.Repeat.RRule()
			if e.Until != nil {
				// UNTIL is in UTC when DTSTART has a time zone: the end of the last day.
				end := time.Date(e.Until.Year(), e.Until.Month(), e.Until.Day(), 23, 59, 59, 0, loc)
				rule += ";UNTIL=" + end.UTC().Format(icsUTC)
			}
			w.line("RRULE:" + rule)
			for _, d := range exdates {
				w.line("EXDATE;TZID=" + e.Timezone + ":" + d)
			}
		}
		if cancelled {
			w.line("STATUS:CANCELLED")
		}
		w.body(summary, &main, e.Note)
		w.line("END:VEVENT")

		for k := range changed {
			o := &changed[k]
			w.line("BEGIN:VEVENT")
			w.line("UID:" + e.ID.String() + "@heatseeker")
			w.line("DTSTAMP:" + stamp)
			w.line("RECURRENCE-ID;TZID=" + e.Timezone + ":" + local(o.PlannedStartsAt))
			w.line("DTSTART;TZID=" + e.Timezone + ":" + local(o.StartsAt))
			w.line("DTEND;TZID=" + e.Timezone + ":" + local(o.EndsAt))
			w.body(summary, o, e.Note)
			w.line("END:VEVENT")
		}
	}
	w.line("END:VCALENDAR")
	return []byte(w.b.String())
}

// summaryOf is the class title (or its subject) with the kind.
func summaryOf(e *domain.ScheduleEvent, subjects map[uuid.UUID]string) string {
	title := e.Title
	if title == "" && e.SubjectID != nil {
		title = subjects[*e.SubjectID]
	}
	if label := kindLabels[e.Kind]; label != "" {
		title += " · " + label
	}
	return title
}

type icsWriter struct{ b strings.Builder }

// body writes what the calendar shows about one class.
func (w *icsWriter) body(summary string, o *domain.Occurrence, note string) {
	w.prop("SUMMARY", summary)
	if o.Location != "" {
		w.prop("LOCATION", o.Location)
	}
	var desc []string
	if o.Teacher != "" {
		desc = append(desc, "Преподаватель: "+o.Teacher)
	}
	if o.ChangeNote != "" {
		desc = append(desc, o.ChangeNote)
	}
	if note != "" {
		desc = append(desc, note)
	}
	if len(desc) > 0 {
		w.prop("DESCRIPTION", strings.Join(desc, "\n"))
	}
}

// prop writes a text property, escaped.
func (w *icsWriter) prop(name, value string) {
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`)
	w.line(name + ":" + r.Replace(value))
}

// line writes a content line folded at 75 octets, never inside a character.
func (w *icsWriter) line(s string) {
	limit := 75
	for len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		w.b.WriteString(s[:cut])
		w.b.WriteString("\r\n ")
		s = s[cut:]
		limit = 74 // the leading space counts
	}
	w.b.WriteString(s)
	w.b.WriteString("\r\n")
}
