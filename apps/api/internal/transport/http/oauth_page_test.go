package http

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"heatseeker/api/internal/domain"
)

func renderOAuthPage(t *testing.T, status int, r oauthResult) (int, string, string) {
	t.Helper()
	w := httptest.NewRecorder()
	ctx := humatest.NewContext(nil, httptest.NewRequest("GET", "/", nil), w)
	oauthPage(status, r).Body(ctx)
	return w.Code, w.Header().Get("Content-Type"), w.Body.String()
}

func TestOAuthPage(t *testing.T) {
	code, ctype, body := renderOAuthPage(t, 200, oauthResult{Email: `<script>alert(1)</script>@example.com`})
	if code != 200 || !strings.HasPrefix(ctype, "text/html") || !strings.Contains(body, "Аккаунт подключён") {
		t.Fatalf("success page: %d %s %s", code, ctype, body)
	}
	if strings.Contains(body, "<script>") {
		t.Fatal("e-mail is not escaped")
	}
	_, _, body = renderOAuthPage(t, 400, oauthResult{Failed: true, Code: domain.CodeDrivePublisherNoAccess})
	if !strings.Contains(body, "Аккаунт не подключён") || !strings.Contains(body, "прав") {
		t.Fatalf("no access page: %s", body)
	}
	_, _, body = renderOAuthPage(t, 400, oauthResult{Failed: true, Code: "something_else"})
	if !bytes.Contains([]byte(body), []byte("заново")) {
		t.Fatalf("generic failure page: %s", body)
	}
}
