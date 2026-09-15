package slug

import "testing"

func TestMake(t *testing.T) {
	tests := map[string]string{
		"Математический анализ": "matematicheskiy-analiz",
		"  Линал 2 семестр ":    "linal-2-semestr",
		"Data Science / ML":     "data-science-ml",
		"---":                   "item",
		"":                      "item",
		"Объектно-ориентированное": "obektno-orientirovannoe",
		"Философия (лекции)":       "filosofiya-lektsii",
	}
	for in, want := range tests {
		if got := Make(in); got != want {
			t.Errorf("Make(%q) = %q, want %q", in, got, want)
		}
	}
}
