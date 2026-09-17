package classify

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

// Subject is what the classifier knows about a course.
type Subject struct {
	ID        uuid.UUID
	Name      string
	ShortName string
	Teacher   string
	Aliases   []string
}

// FromDomain converts subjects for the classifier.
func FromDomain(list []domain.Subject) []Subject {
	out := make([]Subject, 0, len(list))
	for _, s := range list {
		c := Subject{ID: s.ID, Name: s.Name, Aliases: s.Aliases}
		if s.ShortName != nil {
			c.ShortName = *s.ShortName
		}
		if s.Teacher != nil {
			c.Teacher = *s.Teacher
		}
		out = append(out, c)
	}
	return out
}

// Input is everything known about a file.
type Input struct {
	FileName    string
	Folders     []string // folder names from the connection root (exclusive) to the parent
	Description string
	Text        string // leading text of the content, when extracted (stage 5)
}

// Signal weights (docs/PLAN.md §6.4).
const (
	weightPath        = 0.9
	weightNameMulti   = 0.8
	weightNameSingle  = 0.7
	weightTeacher     = 0.6
	weightDescription = 0.5
	weightText        = 0.4
	supportBonus      = 0.1
	conflictPenalty   = 0.35
	maxConfidence     = 0.98
	minPhraseScore    = 0.5
)

type phrase struct {
	tokens  []string
	teacher bool
	label   string
}

type compiled struct {
	id      uuid.UUID
	phrases []phrase
}

func compile(subjects []Subject) []compiled {
	out := make([]compiled, 0, len(subjects))
	for _, s := range subjects {
		c := compiled{id: s.ID}
		add := func(text, label string, teacher bool) {
			toks := words(Tokens(text))
			if len(toks) > 0 {
				c.phrases = append(c.phrases, phrase{tokens: toks, teacher: teacher, label: label})
			}
		}
		add(s.Name, s.Name, false)
		add(s.ShortName, s.ShortName, false)
		for _, a := range s.Aliases {
			add(a, a, false)
		}
		if nameToks := words(Tokens(s.Name)); len(nameToks) >= 2 {
			var acr strings.Builder
			for _, t := range nameToks {
				if len(t) >= 3 {
					acr.WriteByte(t[0])
				}
			}
			if acr.Len() >= 2 {
				c.phrases = append(c.phrases, phrase{tokens: []string{acr.String()}, label: strings.ToUpper(acr.String())})
			}
		}
		if s.Teacher != "" {
			// "Иванов И.И." → surname only; initials are too ambiguous.
			if toks := words(Tokens(s.Teacher)); len(toks) > 0 && len(toks[0]) >= 4 {
				c.phrases = append(c.phrases, phrase{tokens: toks[:1], teacher: true, label: s.Teacher})
			}
		}
		out = append(out, c)
	}
	return out
}

// hit is the best phrase of one subject found in one piece of text.
type hit struct {
	id      uuid.UUID
	score   float64 // share of phrase tokens present
	matched int
	multi   bool
	teacher bool
	label   string
}

// matchPhrase returns the share and number of phrase tokens found in text.
// Tokens of up to three letters (acronyms, "ОС") must match exactly.
func matchPhrase(ph phrase, text []string) (float64, int) {
	matched := 0
	for _, pt := range ph.tokens {
		for _, tt := range text {
			if pt == tt || (len(pt) > 3 && sameWord(pt, tt)) {
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(len(ph.tokens)), matched
}

// findSubjects returns, per subject, the best phrase match in text. When one
// subject's full match is a strict subset of another's ("Физика" inside
// "Физика твердого тела"), only the more specific subject is kept.
func findSubjects(subjects []compiled, text []string) []hit {
	if len(text) == 0 {
		return nil
	}
	var hits []hit
	for _, s := range subjects {
		best := hit{id: s.id}
		for _, ph := range s.phrases {
			score, matched := matchPhrase(ph, text)
			if score < minPhraseScore {
				continue
			}
			if score > best.score || (score == best.score && matched > best.matched) {
				best = hit{id: s.id, score: score, matched: matched, multi: len(ph.tokens) > 1, teacher: ph.teacher, label: ph.label}
			}
		}
		if best.score > 0 {
			hits = append(hits, best)
		}
	}
	maxFull := 0
	for _, h := range hits {
		if h.score == 1 && h.matched > maxFull {
			maxFull = h.matched
		}
	}
	out := hits[:0]
	for _, h := range hits {
		if h.score == 1 && h.matched < maxFull {
			continue
		}
		out = append(out, h)
	}
	return out
}

type candidate struct {
	best    float64
	sources map[string]bool
	signals []string
}

// Classify returns the most likely subject and kind with a confidence in
// [0, 1]. A nil SubjectID means no subject matched.
func Classify(in Input, subjects []Subject) domain.Classification {
	compiledSubjects := compile(subjects)
	cands := map[uuid.UUID]*candidate{}
	note := func(h hit, source string, weight float64) {
		c := cands[h.id]
		if c == nil {
			c = &candidate{sources: map[string]bool{}}
			cands[h.id] = c
		}
		conf := weight * h.score
		if conf > c.best {
			c.best = conf
		}
		if !c.sources[source] {
			c.sources[source] = true
			c.signals = append(c.signals, fmt.Sprintf("%s:%s", source, h.label))
		}
	}

	// 1. Path: the deepest folder that names a subject wins.
	for i := len(in.Folders) - 1; i >= 0; i-- {
		hits := findSubjects(compiledSubjects, words(Tokens(in.Folders[i])))
		for _, h := range hits {
			note(h, "path", weightPath)
		}
		if len(hits) > 0 {
			break
		}
	}

	// 2. File name.
	nameTokens := Tokens(StripExtension(in.FileName))
	for _, h := range findSubjects(compiledSubjects, words(nameTokens)) {
		switch {
		case h.teacher:
			note(h, "teacher", weightTeacher)
		case h.multi:
			note(h, "name", weightNameMulti)
		default:
			note(h, "name", weightNameSingle)
		}
	}

	// 3. Metadata and 4. content: weak on their own, supportive otherwise.
	for _, h := range findSubjects(compiledSubjects, words(Tokens(in.Description))) {
		note(h, "description", weightDescription)
	}
	if in.Text != "" {
		text := in.Text
		if len(text) > 2000 {
			text = text[:2000]
		}
		for _, h := range findSubjects(compiledSubjects, words(Tokens(text))) {
			note(h, "text", weightText)
		}
	}

	result := domain.Classification{Kind: DetectKind(in.FileName, in.Folders), Method: domain.ClassifyAuto}
	result.Topic, result.Semester = detectNumbers(nameTokens, in.Folders)

	type scored struct {
		id   uuid.UUID
		conf float64
		c    *candidate
	}
	ranked := make([]scored, 0, len(cands))
	for id, c := range cands {
		conf := c.best + supportBonus*float64(len(c.sources)-1)
		ranked = append(ranked, scored{id: id, conf: math.Min(conf, maxConfidence), c: c})
	}
	if len(ranked) == 0 {
		return result
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].conf != ranked[j].conf {
			return ranked[i].conf > ranked[j].conf
		}
		return ranked[i].id.String() < ranked[j].id.String()
	})
	top := ranked[0]
	conf := top.conf
	signals := append([]string(nil), top.c.signals...)
	if len(ranked) > 1 && ranked[1].conf >= top.conf-0.15 {
		// Two subjects fit about equally well: let a human decide.
		conf -= conflictPenalty
		signals = append(signals, "conflict")
	}
	id := top.id
	result.SubjectID = &id
	result.Confidence = math.Round(math.Max(conf, 0)*100) / 100
	result.Signals = signals
	return result
}

// MatchScore reports how well text names the subject (0..1). Used to pick an
// upload folder and to decide whether a folder name is worth learning as an
// alias.
func MatchScore(text string, s Subject) float64 {
	hits := findSubjects(compile([]Subject{s}), words(Tokens(text)))
	if len(hits) == 0 {
		return 0
	}
	return hits[0].score
}

type kindRule struct {
	kind     domain.MaterialKind
	prefixes []string
	exact    []string
}

// Order matters: the first rule matching a token wins.
var kindRules = []kindRule{
	{domain.KindCalc, []string{"rasch", "kursov", "calc"}, []string{"rpz", "rgr", "kp"}},
	{domain.KindReport, []string{"doklad", "prezent", "present", "referat", "report", "otchet", "slaid", "slide"}, nil},
	{domain.KindAssignment, []string{"laborat", "prakt", "practic", "zadani", "semin", "kontroln", "homework", "assign", "variant"}, []string{"lab", "lr", "pz", "pr", "dz", "kr", "hw"}},
	{domain.KindNotes, []string{"konspekt", "zametk", "note", "shpargal", "shpor"}, nil},
	{domain.KindLecture, []string{"lekc", "lectur"}, []string{"lk", "lek", "lec"}},
}

// DetectKind infers the material kind from the file name, falling back to
// folder names from the deepest up.
func DetectKind(fileName string, folders []string) domain.MaterialKind {
	if k, ok := kindOf(Tokens(StripExtension(fileName))); ok {
		return k
	}
	for i := len(folders) - 1; i >= 0; i-- {
		if k, ok := kindOf(Tokens(folders[i])); ok {
			return k
		}
	}
	return domain.KindOther
}

func kindOf(tokens []string) (domain.MaterialKind, bool) {
	for _, t := range tokens {
		for _, r := range kindRules {
			for _, e := range r.exact {
				if t == e {
					return r.kind, true
				}
			}
			for _, p := range r.prefixes {
				if strings.HasPrefix(t, p) {
					return r.kind, true
				}
			}
		}
	}
	return "", false
}

var (
	topicRe    = regexp.MustCompile(`\b(?:tema|topic|t)\s(\d{1,2})\b`)
	semesterRe = regexp.MustCompile(`\b(?:semestr|semester|sem)\s(\d{1,2})\b|\b(\d{1,2})\s(?:semestr|semester|sem)\b`)
)

func detectNumbers(nameTokens []string, folders []string) (topic, semester *int) {
	texts := []string{strings.Join(nameTokens, " ")}
	for i := len(folders) - 1; i >= 0; i-- {
		texts = append(texts, strings.Join(Tokens(folders[i]), " "))
	}
	for _, t := range texts {
		if topic == nil {
			if m := topicRe.FindStringSubmatch(t); m != nil {
				topic = atoiPtr(m[1])
			}
		}
		if semester == nil {
			if m := semesterRe.FindStringSubmatch(t); m != nil {
				v := m[1]
				if v == "" {
					v = m[2]
				}
				semester = atoiPtr(v)
			}
		}
	}
	return topic, semester
}

func atoiPtr(s string) *int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}
