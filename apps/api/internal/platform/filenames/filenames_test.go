package filenames

import (
	"strings"
	"testing"
)

func TestSafeName(t *testing.T) {
	tests := map[string]string{
		"Лекция 1.pdf":          "Лекция_1.pdf",
		"../../etc/passwd":      "passwd",
		`C:\Users\a\report.doc`: "report.doc",
		"   ":                   "file",
		"a\x00b?.txt":           "ab_.txt",
		".hidden":               "hidden",
	}
	for in, want := range tests {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("я", 300) + ".pdf"
	if got := []rune(SafeName(long)); len(got) != 120 || string(got[len(got)-4:]) != ".pdf" {
		t.Errorf("long name not truncated with extension: %d", len(got))
	}
}

func TestContentDisposition(t *testing.T) {
	if got := ContentDisposition("Лекция.pdf", true); !strings.HasPrefix(got, "inline; filename*=utf-8''") {
		t.Errorf("non-ascii inline = %q", got)
	}
	if got := ContentDisposition("a.pdf", false); got != "attachment; filename=a.pdf" {
		t.Errorf("ascii attachment = %q", got)
	}
}

func TestInline(t *testing.T) {
	tests := map[string]bool{
		"application/pdf":    true,
		"image/png":          true,
		"image/svg+xml":      false,
		"audio/flac":         true,
		"audio/mpeg":         true,
		"video/mp4":          true,
		"text/plain":         true,
		"text/html":          false,
		"application/zip":    false,
		"application/msword": false,
	}
	for mime, want := range tests {
		if got := Inline(mime); got != want {
			t.Errorf("Inline(%q) = %v, want %v", mime, got, want)
		}
	}
}
