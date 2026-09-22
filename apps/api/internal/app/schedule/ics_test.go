package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

func TestBuildICS(t *testing.T) {
	msk, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	subjectID := uuid.New()
	until := time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC)
	series := domain.ScheduleEvent{
		ID: uuid.New(), SubjectID: &subjectID, Kind: domain.ScheduleSeminar, Timezone: "Europe/Moscow",
		StartsAt: time.Date(2026, 9, 7, 10, 0, 0, 0, msk), EndsAt: time.Date(2026, 9, 7, 11, 30, 0, 0, msk),
		Repeat: &domain.Recurrence{IntervalWeeks: 2}, Until: &until, Location: "301, корпус Б",
		Note: strings.Repeat("длинная заметка ", 10),
	}
	oneOff := domain.ScheduleEvent{
		ID: uuid.New(), Title: "Экзамен", Kind: domain.ScheduleExam, Timezone: "Europe/Moscow",
		StartsAt: time.Date(2027, 1, 15, 9, 0, 0, 0, msk), EndsAt: time.Date(2027, 1, 15, 12, 0, 0, 0, msk),
	}
	moved := time.Date(2026, 9, 22, 14, 0, 0, 0, msk)
	movedEnd := moved.Add(90 * time.Minute)
	exceptions := map[uuid.UUID][]domain.ScheduleException{
		series.ID: {
			{EventID: series.ID, Date: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), Kind: domain.ExceptionCancelled},
			{EventID: series.ID, Date: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), Kind: domain.ExceptionChanged,
				StartsAt: &moved, EndsAt: &movedEnd, Note: "перенос"},
		},
		oneOff.ID: {{EventID: oneOff.ID, Date: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), Kind: domain.ExceptionCancelled}},
	}
	out := string(buildICS("Группа", []domain.ScheduleEvent{series, oneOff}, exceptions,
		map[uuid.UUID]string{subjectID: "Матан"}, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)))

	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n",
		"DTSTART;TZID=Europe/Moscow:20260907T100000\r\n",
		"RRULE:FREQ=WEEKLY;INTERVAL=2;UNTIL=20261228T205959Z\r\n",
		"EXDATE;TZID=Europe/Moscow:20260907T100000\r\n",
		"RECURRENCE-ID;TZID=Europe/Moscow:20260921T100000\r\n",
		"DTSTART;TZID=Europe/Moscow:20260922T140000\r\n",
		"SUMMARY:Матан · семинар\r\n",
		`LOCATION:301\, корпус Б`,
		"STATUS:CANCELLED\r\n",
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("calendar lacks %q", want)
		}
	}
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line longer than 75 octets: %q", line)
		}
	}
	if strings.Count(out, "BEGIN:VEVENT") != 3 {
		t.Errorf("want 3 VEVENTs (series, its change, the exam):\n%s", out)
	}
}
