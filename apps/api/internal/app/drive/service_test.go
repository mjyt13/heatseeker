package drive

import (
	"errors"
	"testing"

	"heatseeker/api/internal/domain"
)

func TestParseFolderRef(t *testing.T) {
	const id = "1AbCdEfGhIjKlMnOpQrStUvWxYz_-012"
	tests := map[string]string{
		"https://drive.google.com/drive/folders/" + id:                    id,
		"https://drive.google.com/drive/folders/" + id + "?usp=sharing":   id,
		"https://drive.google.com/drive/u/1/folders/" + id + "?usp=drive": id,
		"https://drive.google.com/open?id=" + id:                          id,
		"https://drive.google.com/open?usp=x&id=" + id:                    id,
		"  " + id + "  ": id,
		"https://drive.google.com/drive/mobile/folders/" + id + "/?pli=1": id,
	}
	for in, want := range tests {
		got, err := ParseFolderRef(in)
		if err != nil || got != want {
			t.Errorf("ParseFolderRef(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "short", "https://example.com/", "https://drive.google.com/drive/my-drive", "id with spaces inside"} {
		if _, err := ParseFolderRef(bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("ParseFolderRef(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestAppendPath(t *testing.T) {
	base := []string{"Матан"}
	got := appendPath(base, "Лекции/2026")
	if joinPath(got) != "Матан/Лекции∕2026" {
		t.Fatalf("path = %q", joinPath(got))
	}
	// the input slice must not be modified or shared
	_ = appendPath(base, "Другое")
	if joinPath(got) != "Матан/Лекции∕2026" || len(base) != 1 {
		t.Fatal("appendPath aliases its input")
	}
}
