package gotenberg

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heatseeker/api/internal/domain"
)

func TestConvertToPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/libreoffice/convert" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		file, hdr, err := r.FormFile("files")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(file)
		switch {
		case string(body) == "broken":
			http.Error(w, "LibreOffice failed to process a document", http.StatusBadRequest)
		case string(body) == "busy":
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		default:
			_, _ = io.WriteString(w, "%PDF-1.7 "+hdr.Filename+" "+string(body))
		}
	}))
	defer srv.Close()
	c := New(srv.URL + "/")
	ctx := context.Background()

	out, err := c.ConvertToPDF(ctx, "Лекция 1: \"введение\".PPTX", strings.NewReader("slides"))
	if err != nil {
		t.Fatal(err)
	}
	pdf, _ := io.ReadAll(out)
	_ = out.Close()
	if string(pdf) != "%PDF-1.7 document.pptx slides" {
		t.Fatalf("pdf = %q", pdf)
	}

	if _, err := c.ConvertToPDF(ctx, "a.docx", strings.NewReader("broken")); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("broken file: %v", err)
	}
	if _, err := c.ConvertToPDF(ctx, "a.docx", strings.NewReader("busy")); err == nil || errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("busy server must be retryable: %v", err)
	}
	down := New("http://127.0.0.1:1")
	if _, err := down.ConvertToPDF(ctx, "a.docx", strings.NewReader("x")); err == nil || errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("unreachable server must be retryable: %v", err)
	}
}
