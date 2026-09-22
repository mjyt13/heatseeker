package domain

import (
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// date is a civil date of 2026, the year the tests live in.
func date(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

func TestRRuleRoundTrip(t *testing.T) {
	tests := []struct {
		in, out string
		weeks   int
	}{
		{"FREQ=WEEKLY", "FREQ=WEEKLY", 1},
		{"FREQ=WEEKLY;INTERVAL=2;BYDAY=TH,MO", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,TH", 2},
		{"freq=weekly;byday=su,mo", "FREQ=WEEKLY;BYDAY=MO,SU", 1},
	}
	for _, tt := range tests {
		r, err := ParseRRule(tt.in)
		if err != nil {
			t.Fatalf("%s: %v", tt.in, err)
		}
		if got := r.RRule(); got != tt.out {
			t.Errorf("%s: RRule() = %s, want %s", tt.in, got, tt.out)
		}
		if r.IntervalWeeks != tt.weeks {
			t.Errorf("%s: interval = %d", tt.in, r.IntervalWeeks)
		}
	}
	for _, bad := range []string{"", "FREQ=DAILY", "FREQ=WEEKLY;INTERVAL=9", "FREQ=WEEKLY;BYDAY=XX", "FREQ=WEEKLY;COUNT=3"} {
		if _, err := ParseRRule(bad); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

// Mondays 10:00 Moscow time, every other week, starting 2026-09-07.
func biweekly(t *testing.T) *ScheduleEvent {
	msk := mustLoc(t, "Europe/Moscow")
	until := date(12, 28)
	return &ScheduleEvent{
		Title: "Матан", Timezone: "Europe/Moscow",
		StartsAt: time.Date(2026, 9, 7, 10, 0, 0, 0, msk), EndsAt: time.Date(2026, 9, 7, 11, 30, 0, 0, msk),
		Repeat: &Recurrence{IntervalWeeks: 2}, Until: &until, Location: "301",
	}
}

func TestOccurrencesEveryOtherWeek(t *testing.T) {
	e := biweekly(t)
	msk := mustLoc(t, "Europe/Moscow")
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, msk)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, msk)
	got, err := e.Occurrences(from, to, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Time{date(9, 7), date(9, 21)}
	if len(got) != len(want) {
		t.Fatalf("got %d occurrences, want %d", len(got), len(want))
	}
	for i, o := range got {
		if !o.Date.Equal(want[i]) {
			t.Errorf("#%d date = %s, want %s", i, o.Date.Format(DateLayout), want[i].Format(DateLayout))
		}
		if h := o.StartsAt.In(msk).Hour(); h != 10 {
			t.Errorf("#%d starts at %d:00", i, h)
		}
		if o.EndsAt.Sub(o.StartsAt) != 90*time.Minute {
			t.Errorf("#%d lasts %s", i, o.EndsAt.Sub(o.StartsAt))
		}
	}
}

func TestOccurrencesStopAtUntil(t *testing.T) {
	e := biweekly(t)
	msk := mustLoc(t, "Europe/Moscow")
	got, err := e.Occurrences(time.Date(2026, 12, 1, 0, 0, 0, 0, msk), time.Date(2027, 2, 1, 0, 0, 0, 0, msk), nil)
	if err != nil {
		t.Fatal(err)
	}
	// 2026-12-14 and 2026-12-28 are in the series; January is past its end.
	if len(got) != 2 || !got[1].Date.Equal(date(12, 28)) {
		t.Fatalf("got %v", got)
	}
}

func TestOccurrencesSeveralWeekdays(t *testing.T) {
	msk := mustLoc(t, "Europe/Moscow")
	e := &ScheduleEvent{
		Timezone: "Europe/Moscow",
		StartsAt: time.Date(2026, 9, 8, 9, 0, 0, 0, msk), EndsAt: time.Date(2026, 9, 8, 10, 30, 0, 0, msk),
		Repeat: &Recurrence{IntervalWeeks: 1, Weekdays: []time.Weekday{time.Tuesday, time.Thursday}},
	}
	// The week of 2026-09-07: Tuesday 8 and Thursday 10.
	got, err := e.Occurrences(time.Date(2026, 9, 7, 0, 0, 0, 0, msk), time.Date(2026, 9, 14, 0, 0, 0, 0, msk), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Date.Equal(date(9, 8)) || !got[1].Date.Equal(date(9, 10)) {
		t.Fatalf("got %+v", got)
	}
}

func TestOccurrencesKeepWallClockAcrossDST(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")
	e := &ScheduleEvent{
		Timezone: "Europe/Berlin",
		StartsAt: time.Date(2026, 10, 19, 10, 0, 0, 0, berlin), EndsAt: time.Date(2026, 10, 19, 11, 0, 0, 0, berlin),
		Repeat: &Recurrence{IntervalWeeks: 1},
	}
	// Summer time ends on 2026-10-25: the next Monday is still 10:00 local.
	got, err := e.Occurrences(time.Date(2026, 10, 26, 0, 0, 0, 0, berlin), time.Date(2026, 10, 27, 0, 0, 0, 0, berlin), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].StartsAt.In(berlin).Hour() != 10 {
		t.Fatalf("got %+v", got)
	}
	if got[0].StartsAt.Sub(e.StartsAt) != 7*24*time.Hour+time.Hour {
		t.Fatalf("offset did not change: %s", got[0].StartsAt.Sub(e.StartsAt))
	}
}

func TestOccurrencesWithExceptions(t *testing.T) {
	e := biweekly(t)
	msk := mustLoc(t, "Europe/Moscow")
	moved := time.Date(2026, 10, 1, 14, 0, 0, 0, msk) // from Monday 2026-09-21 to Thursday 2026-10-01
	movedEnd := moved.Add(90 * time.Minute)
	room := "Онлайн"
	exceptions := []ScheduleException{
		{Date: date(9, 7), Kind: ExceptionCancelled, Note: "праздник"},
		{Date: date(9, 21), Kind: ExceptionChanged, StartsAt: &moved, EndsAt: &movedEnd, Location: &room},
		{Date: date(9, 14), Kind: ExceptionCancelled}, // not a class date: ignored
	}
	// September: the cancelled class is still shown; the moved one left.
	sept, err := e.Occurrences(time.Date(2026, 9, 1, 0, 0, 0, 0, msk), time.Date(2026, 9, 30, 0, 0, 0, 0, msk), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(sept) != 1 || sept[0].Status != OccurrenceCancelled || sept[0].ChangeNote != "праздник" {
		t.Fatalf("september: %+v", sept)
	}
	// The first days of October hold the moved class.
	oct, err := e.Occurrences(time.Date(2026, 10, 1, 0, 0, 0, 0, msk), time.Date(2026, 10, 3, 0, 0, 0, 0, msk), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(oct) != 1 {
		t.Fatalf("october: %+v", oct)
	}
	o := oct[0]
	if o.Status != OccurrenceChanged || !o.Moved() || o.Location != "Онлайн" || !o.Date.Equal(date(9, 21)) {
		t.Fatalf("moved class: %+v", o)
	}
}

func TestOneOffEvent(t *testing.T) {
	msk := mustLoc(t, "Europe/Moscow")
	e := &ScheduleEvent{
		Timezone: "Europe/Moscow",
		StartsAt: time.Date(2026, 12, 20, 23, 30, 0, 0, msk), EndsAt: time.Date(2026, 12, 21, 1, 0, 0, 0, msk),
	}
	// It crosses midnight: the window of the next day still sees it.
	got, err := e.Occurrences(time.Date(2026, 12, 21, 0, 0, 0, 0, msk), time.Date(2026, 12, 22, 0, 0, 0, 0, msk), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Date.Equal(date(12, 20)) {
		t.Fatalf("got %+v", got)
	}
	if !e.OccursOn(date(12, 20), msk) || e.OccursOn(date(12, 27), msk) {
		t.Fatal("a one-off class occurs on its own date only")
	}
}
