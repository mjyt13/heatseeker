package notify

import (
	"testing"
	"time"
)

func TestDateTimeUsesReaderTimezone(t *testing.T) {
	moment := time.Date(2026, time.September, 23, 21, 30, 0, 0, time.UTC)
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Skipf("no tzdata: %v", err)
	}
	if got, want := dateTime(moment, time.UTC), "23 сентября, 21:30"; got != want {
		t.Errorf("UTC: got %q, want %q", got, want)
	}
	// Same moment, next day for a reader three hours to the east.
	if got, want := dateTime(moment, moscow), "24 сентября, 00:30"; got != want {
		t.Errorf("Moscow: got %q, want %q", got, want)
	}
}

func TestPlural(t *testing.T) {
	cases := map[int]string{1: "файл", 2: "файла", 4: "файла", 5: "файлов", 11: "файлов", 21: "файл", 112: "файлов"}
	for n, want := range cases {
		if got := plural(n, "файл", "файла", "файлов"); got != want {
			t.Errorf("plural(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestWithSubject(t *testing.T) {
	if got := withSubject("Лекция 3", "Матан"); got != "Матан: Лекция 3" {
		t.Errorf("got %q", got)
	}
	if got := withSubject("Лекция 3", ""); got != "Лекция 3" {
		t.Errorf("got %q", got)
	}
}
