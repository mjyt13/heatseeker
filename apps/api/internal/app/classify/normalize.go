// Package classify guesses the subject and kind of a file from its Drive path,
// name and metadata (docs/PLAN.md §6.4). Everything here is a pure function so
// the rules are covered by table tests on real-world file names.
package classify

import (
	"path"
	"strings"
	"unicode"
)

var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "i", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "c",
	'ч': "ch", 'ш': "sh", 'щ': "sh", 'ъ': "", 'ы': "i", 'ь': "", 'э': "e", 'ю': "u",
	'я': "a", 'і': "i", 'ї': "i", 'є': "e",
}

// squash folds the many ways people transliterate Russian into one spelling,
// so "Lektsiya", "Lekciya" and "лекция" all become "lekcia". Order matters.
var squash = strings.NewReplacer(
	"shch", "sh", "sch", "sh", "kh", "h", "ts", "c", "tz", "c",
	"yo", "e", "yu", "u", "ya", "a", "iy", "i", "ij", "i", "ii", "i",
	"y", "i", "j", "i", "w", "v", "x", "ks", "q", "k",
)

// noise tokens carry no signal about subject or kind.
var noise = map[string]bool{
	"pdf": true, "doc": true, "docx": true, "ppt": true, "pptx": true, "xls": true, "xlsx": true,
	"odt": true, "rtf": true, "txt": true, "zip": true, "rar": true, "png": true, "jpg": true, "jpeg": true,
	"copy": true, "kopia": true, "kopiia": true, "final": true, "new": true, "novi": true,
	"the": true, "and": true, "for": true, "of": true,
	"po": true, "na": true, "dla": true, "ot": true, "iz": true, "pri": true, "ili": true,
}

// Tokens normalises s into lower-case ASCII tokens: Cyrillic is
// transliterated, letters and digits are separated ("tema3" → "tema", "3"),
// punctuation splits words, and transliteration variants are folded.
func Tokens(s string) []string {
	var b strings.Builder
	prevDigit, prevLetter := false, false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			if prevDigit {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			prevLetter, prevDigit = true, false
		case unicode.IsDigit(r):
			if prevLetter {
				b.WriteByte(' ')
			}
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
			prevLetter, prevDigit = false, true
		default:
			if t, ok := cyrillic[r]; ok {
				if prevDigit {
					b.WriteByte(' ')
				}
				b.WriteString(t)
				prevLetter, prevDigit = true, false
				continue
			}
			b.WriteByte(' ')
			prevLetter, prevDigit = false, false
		}
	}
	fields := strings.Fields(b.String())
	out := fields[:0]
	for _, f := range fields {
		if noise[f] {
			continue
		}
		if !isDigits(f) {
			f = squash.Replace(f)
		}
		if f == "" || noise[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// words drops digits and one-letter tokens: what is left can match subjects.
func words(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if len(t) >= 2 && !isDigits(t) {
			out = append(out, t)
		}
	}
	return out
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// StripExtension removes a short file extension ("x.pdf" → "x").
func StripExtension(name string) string {
	ext := path.Ext(name)
	if ext == "" || len(ext) > 6 || strings.ContainsAny(ext, " _") {
		return name
	}
	return strings.TrimSuffix(name, ext)
}

// TitleFromFileName turns a file name into a readable title:
// "Tema3_Lektsia2_Matan.pdf" → "Tema3 Lektsia2 Matan".
func TitleFromFileName(name string) string {
	base := StripExtension(strings.TrimSpace(name))
	base = strings.NewReplacer("_", " ", " ", " ").Replace(base)
	base = strings.Join(strings.Fields(base), " ")
	if base == "" {
		return strings.TrimSpace(name)
	}
	return base
}

// sameWord compares normalised tokens tolerating Russian inflection
// ("analiz"/"analiza") and single typos in long words.
func sameWord(a, b string) bool {
	if a == b {
		return true
	}
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}
	if len(short) < 4 {
		return false
	}
	if len(long)-len(short) <= 4 && commonPrefix(short, long) >= max(4, len(short)-2) {
		return true
	}
	return len(short) >= 6 && len(long)-len(short) <= 1 && levenshtein(short, long) <= 1
}

func commonPrefix(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// levenshtein is the edit distance of two ASCII strings.
func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
