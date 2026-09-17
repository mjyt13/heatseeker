package classify

import (
	"reflect"
	"testing"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

var (
	matan   = Subject{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Name: "Математический анализ", ShortName: "Матан", Teacher: "Петров А. В.", Aliases: []string{"матан"}}
	os      = Subject{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Name: "Операционные системы", ShortName: "ОС"}
	db      = Subject{ID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), Name: "Базы данных", ShortName: "БД"}
	physics = Subject{ID: uuid.MustParse("00000000-0000-0000-0000-000000000004"), Name: "Физика"}
	solid   = Subject{ID: uuid.MustParse("00000000-0000-0000-0000-000000000005"), Name: "Физика твердого тела"}
	english = Subject{ID: uuid.MustParse("00000000-0000-0000-0000-000000000006"), Name: "Иностранный язык", Aliases: []string{"английский", "english"}}
	mni     = Subject{ID: uuid.MustParse("00000000-0000-0000-0000-000000000007"), Name: "Методология научных исследований", ShortName: "МНИ"}

	allSubjects = []Subject{matan, os, db, physics, solid, english, mni}
)

func intPtr(n int) *int { return &n }

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		in       Input
		subject  *Subject
		minConf  float64
		maxConf  float64
		kind     domain.MaterialKind
		topic    *int
		semester *int
	}{
		{
			name:    "transliterated alias in file name",
			in:      Input{FileName: "Tema3_Lektsia2_Matan.pdf"},
			subject: &matan, minConf: 0.7, maxConf: 0.7, kind: domain.KindLecture, topic: intPtr(3),
		},
		{
			name:    "subject folder with type subfolder",
			in:      Input{FileName: "Praktika_tema_1.pdf", Folders: []string{"Базы данных", "Практики"}},
			subject: &db, minConf: 0.9, maxConf: 0.9, kind: domain.KindAssignment, topic: intPtr(1),
		},
		{
			name:    "cyrillic lecture inside subject folder",
			in:      Input{FileName: "Лекция 5.pdf", Folders: []string{"Операционные системы"}},
			subject: &os, minConf: 0.9, maxConf: 0.9, kind: domain.KindLecture,
		},
		{
			name: "no subject anywhere",
			in:   Input{FileName: "Лекция 5.pdf", Folders: []string{"Лекции"}},
			kind: domain.KindLecture,
		},
		{
			name:    "more specific subject wins over its prefix",
			in:      Input{FileName: "Физика твердого тела - лк1.pdf"},
			subject: &solid, minConf: 0.8, maxConf: 0.8, kind: domain.KindLecture,
		},
		{
			name:    "single word subject with report kind",
			in:      Input{FileName: "Doklad_Fizika.pptx"},
			subject: &physics, minConf: 0.7, maxConf: 0.7, kind: domain.KindReport,
		},
		{
			name:    "two subjects in the name conflict",
			in:      Input{FileName: "Матан_БД.pdf"},
			subject: &matan, minConf: 0.3, maxConf: 0.4, kind: domain.KindOther,
		},
		{
			name:    "folder outweighs a different subject in the name",
			in:      Input{FileName: "OS_lab2.pdf", Folders: []string{"Физика"}},
			subject: &physics, minConf: 0.9, maxConf: 0.9, kind: domain.KindAssignment,
		},
		{
			name:    "teacher surname",
			in:      Input{FileName: "Petrov_zadanie_2.docx"},
			subject: &matan, minConf: 0.6, maxConf: 0.6, kind: domain.KindAssignment,
		},
		{
			name:    "folder and name agree",
			in:      Input{FileName: "matan_konspekt.pdf", Folders: []string{"Матан"}},
			subject: &matan, minConf: 0.98, maxConf: 0.98, kind: domain.KindNotes,
		},
		{
			name:    "long transliterated name with inflection",
			in:      Input{FileName: "Metodologiya_nauchnykh_issledovanii_RPZ.docx"},
			subject: &mni, minConf: 0.8, maxConf: 0.8, kind: domain.KindCalc,
		},
		{
			name:    "acronym",
			in:      Input{FileName: "МНИ_отчёт.pdf"},
			subject: &mni, minConf: 0.7, maxConf: 0.7, kind: domain.KindReport,
		},
		{
			name:    "alias folder below a semester folder",
			in:      Input{FileName: "Grammar.pdf", Folders: []string{"Семестр 2", "Английский"}},
			subject: &english, minConf: 0.9, maxConf: 0.9, kind: domain.KindOther, semester: intPtr(2),
		},
		{
			name:    "semester before the word",
			in:      Input{FileName: "2 семестр БД.pdf"},
			subject: &db, minConf: 0.7, maxConf: 0.7, kind: domain.KindOther, semester: intPtr(2),
		},
		{
			name:    "full name in latin",
			in:      Input{FileName: "Matematicheskiy_analiz_lekcia.pdf"},
			subject: &matan, minConf: 0.8, maxConf: 0.8, kind: domain.KindLecture,
		},
		{
			name:    "inflected folder name",
			in:      Input{FileName: "scan.pdf", Folders: []string{"Материалы к математическому анализу"}},
			subject: &matan, minConf: 0.9, maxConf: 0.9, kind: domain.KindOther,
		},
		{
			name:    "description alone is weak",
			in:      Input{FileName: "file.pdf", Description: "Загрузил: Иван. Предмет: Базы данных"},
			subject: &db, minConf: 0.5, maxConf: 0.5, kind: domain.KindOther,
		},
		{
			name:    "kind from folder, subject from parent folder",
			in:      Input{FileName: "scan001.jpg", Folders: []string{"Физика", "Лабораторные работы"}},
			subject: &physics, minConf: 0.9, maxConf: 0.9, kind: domain.KindAssignment,
		},
		{
			name:    "course project abbreviation",
			in:      Input{FileName: "RPZ_final_v3.docx", Folders: []string{"ОС"}},
			subject: &os, minConf: 0.9, maxConf: 0.9, kind: domain.KindCalc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.in, allSubjects)
			if tt.subject == nil {
				if got.SubjectID != nil {
					t.Fatalf("subject = %v, want none (signals %v)", *got.SubjectID, got.Signals)
				}
			} else {
				if got.SubjectID == nil || *got.SubjectID != tt.subject.ID {
					t.Fatalf("subject = %v, want %s (signals %v)", got.SubjectID, tt.subject.Name, got.Signals)
				}
				if got.Confidence < tt.minConf || got.Confidence > tt.maxConf {
					t.Fatalf("confidence = %.2f, want %.2f..%.2f (signals %v)", got.Confidence, tt.minConf, tt.maxConf, got.Signals)
				}
			}
			if got.Kind != tt.kind {
				t.Errorf("kind = %s, want %s", got.Kind, tt.kind)
			}
			if !reflect.DeepEqual(got.Topic, tt.topic) {
				t.Errorf("topic = %v, want %v", deref(got.Topic), deref(tt.topic))
			}
			if !reflect.DeepEqual(got.Semester, tt.semester) {
				t.Errorf("semester = %v, want %v", deref(got.Semester), deref(tt.semester))
			}
			if got.Method != domain.ClassifyAuto {
				t.Errorf("method = %s", got.Method)
			}
		})
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestClassifyWithoutSubjects(t *testing.T) {
	got := Classify(Input{FileName: "Lektsia_1.pdf", Folders: []string{"Матан"}}, nil)
	if got.SubjectID != nil || got.Confidence != 0 || got.Kind != domain.KindLecture {
		t.Fatalf("got %+v", got)
	}
}

func TestTokens(t *testing.T) {
	tests := map[string][]string{
		"Tema3_Lektsia2_Matan.pdf": {"tema", "3", "lekcia", "2", "matan"},
		"Лекция №2 (копия).docx":   {"lekcia", "2"},
		"Lekciya":                  {"lekcia"},
		"Lekcija":                  {"lekcia"},
		"Математический":           {"matematicheski"},
		"Matematicheskii":          {"matematicheski"},
		"Научных":                  {"nauchnih"},
		"Nauchnykh":                {"nauchnih"},
		"Щука-Shchuka":             {"shuka", "shuka"},
		"":                         {},
	}
	for in, want := range tests {
		got := Tokens(in)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Tokens(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSameWord(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"analiz", "analiza", true},
		{"matematicheski", "matematicheskomu", true},
		{"sistemi", "sistem", true},
		{"matematika", "matematka", true},
		{"set", "seti", false},
		{"fizika", "himia", false},
		{"lekcia", "lektor", false},
	}
	for _, tt := range tests {
		if got := sameWord(tt.a, tt.b); got != tt.want {
			t.Errorf("sameWord(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestTitleFromFileName(t *testing.T) {
	tests := map[string]string{
		"Tema3_Lektsia2_Matan.pdf": "Tema3 Lektsia2 Matan",
		"  Лекция  1.docx ":        "Лекция 1",
		"Отчёт v1.2 final":         "Отчёт v1.2 final",
		".pdf":                     ".pdf",
		"Конспект_по_физике.backup": "Конспект по физике.backup",
	}
	for in, want := range tests {
		if got := TitleFromFileName(in); got != want {
			t.Errorf("TitleFromFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchScore(t *testing.T) {
	if s := MatchScore("Базы данных (2 курс)", db); s != 1 {
		t.Errorf("full match = %v", s)
	}
	if s := MatchScore("Физика", solid); s != 0 {
		t.Errorf("partial match below threshold = %v", s)
	}
	if s := MatchScore("Разное", db); s != 0 {
		t.Errorf("no match = %v", s)
	}
}
