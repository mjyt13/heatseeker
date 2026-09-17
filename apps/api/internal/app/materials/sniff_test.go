package materials

import (
	"errors"
	"testing"

	"heatseeker/api/internal/domain"
)

func TestCheckContent(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		head string
		ok   bool
	}{
		{"pdf", "pdf", "%PDF-1.7\n...", true},
		{"pdf with preamble", "pdf", "\xef\xbb\xbf  %PDF-1.4", true},
		{"html renamed to pdf", "pdf", "<html><script>", false},
		{"docx", "docx", "PK\x03\x04\x14\x00", true},
		{"docx that is not a zip", "docx", "hello", false},
		{"legacy doc", "doc", "\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1rest", true},
		{"png", "png", "\x89PNG\r\n\x1a\nIHDR", true},
		{"jpeg", "jpg", "\xFF\xD8\xFF\xE0JFIF", true},
		{"webp", "webp", "RIFF\x10\x00\x00\x00WEBPVP8 ", true},
		{"heic", "heic", "\x00\x00\x00\x18ftypheic", true},
		{"rtf", "rtf", `{\rtf1\ansi`, true},
		{"utf-8 text", "txt", "Лекция 1. Введение", true},
		{"cut multibyte rune", "md", "Тема\xd0", true},
		{"cp1251 csv", "csv", "\xcf\xf0\xe5\xe4\xec\xe5\xf2;\xc1\xe0\xeb\xeb\r\n", true},
		{"binary as text", "txt", "abc\x00def", false},
		{"windows exe renamed", "pdf", "MZ\x90\x00%PDF-", false},
		{"elf renamed to txt", "txt", "\x7fELF\x02\x01", false},
		{"shell script as md", "md", "#!/bin/sh\nrm -rf /", false},
		{"empty", "pdf", "", false},
		{"unknown allowed ext is not sniffed", "ods2", "anything", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckContent(tt.ext, []byte(tt.head))
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok && !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("expected ErrInvalid, got %v", err)
			}
		})
	}
}

func TestMimeFor(t *testing.T) {
	tests := []struct{ ext, claimed, want string }{
		{"pdf", "text/html", "application/pdf"},
		{"docx", "", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"unknownext", "application/x-custom; charset=binary", "application/x-custom"},
		{"unknownext", "text/html", "application/octet-stream"},
		{"unknownext", "", "application/octet-stream"},
	}
	for _, tt := range tests {
		if got := MimeFor(tt.ext, tt.claimed); got != tt.want {
			t.Errorf("MimeFor(%q, %q) = %q, want %q", tt.ext, tt.claimed, got, tt.want)
		}
	}
}

func TestExtension(t *testing.T) {
	for in, want := range map[string]string{"A.PDF": "pdf", "noext": "", "arch.tar.gz": "gz", "\tx.Docx\n": "docx"} {
		if got := Extension(in); got != want {
			t.Errorf("Extension(%q) = %q, want %q", in, got, want)
		}
	}
}
