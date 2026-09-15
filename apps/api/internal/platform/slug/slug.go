// Package slug builds URL-safe identifiers from human names, transliterating
// Cyrillic so that group and tag slugs stay readable in links.
package slug

import (
	"strings"
	"unicode"
)

var translit = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu",
	'я': "ya",
}

// Make returns a lower-case ASCII slug: letters, digits and single dashes.
// Empty input yields "item".
func Make(s string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		var piece string
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			piece = string(r)
		case translit[r] != "":
			piece = translit[r]
		case r == 'ъ' || r == 'ь':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Other scripts: keep as-is (lower-cased); URLs tolerate them.
			piece = string(r)
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteString(piece)
		lastDash = false
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "item"
	}
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-")
	}
	return out
}
