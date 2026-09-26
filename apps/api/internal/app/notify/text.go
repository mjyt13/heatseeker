package notify

import (
	"fmt"
	"strings"
	"time"
)

// monthsGenitive name the month the way a date reads in Russian: «23 сентября».
var monthsGenitive = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// dateTime renders a moment in the reader's own timezone: «23 сентября, 14:30».
func dateTime(t time.Time, loc *time.Location) string {
	local := t.In(loc)
	return fmt.Sprintf("%d %s, %02d:%02d", local.Day(), monthsGenitive[local.Month()-1], local.Hour(), local.Minute())
}

// plural picks the Russian form for a count: 1 файл, 2 файла, 5 файлов.
func plural(n int, one, few, many string) string {
	mod100 := n % 100
	if mod100 >= 11 && mod100 <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

// withSubject prefixes a title with its subject when there is one.
func withSubject(title, subject string) string {
	if subject == "" {
		return title
	}
	return strings.TrimSpace(subject) + ": " + title
}
