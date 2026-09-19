package domain

import (
	"path"
	"strings"
	"testing"
)

// likeMatch mirrors SQL LIKE for the patterns used here ('%' only).
func likeMatch(pattern, s string) bool {
	ok, _ := path.Match(strings.ReplaceAll(pattern, "%", "*"), s)
	return ok
}

func fileTypeOf(mime string) FileType {
	for _, ft := range AllFileTypes {
		patterns, exclude := ft.MimePatterns()
		matched := false
		for _, p := range patterns {
			matched = matched || likeMatch(p, mime)
		}
		if matched != exclude {
			return ft
		}
	}
	return ""
}

func TestFileTypeMimePatterns(t *testing.T) {
	cases := map[string]FileType{
		"application/pdf": FileDocument,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": FileDocument,
		"application/vnd.google-apps.document":                                    FileDocument,
		"application/vnd.ms-excel":                                                FileDocument,
		"text/plain":                                                              FileDocument,
		"image/png":                                                               FileImage,
		"audio/flac":                                                              FileAudio,
		"video/mp4":                                                               FileVideo,
		"application/zip":                                                         FileArchive,
		"application/octet-stream":                                                FileOther,
	}
	for mime, want := range cases {
		if got := fileTypeOf(mime); got != want {
			t.Errorf("%s: got %q, want %q", mime, got, want)
		}
	}
}
