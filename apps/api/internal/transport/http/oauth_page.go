package http

import (
	"bytes"
	"html/template"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"heatseeker/api/internal/domain"
)

// oauthResult is what the page after Google's redirect tells the user. The
// browser tab is outside the app, so the page carries its own (Russian) text.
type oauthResult struct {
	Email     string
	Failed    bool
	Cancelled bool
	Expired   bool
	Code      string
}

var oauthMessages = map[string]string{
	domain.CodeDriveScopeMissing:       "Google не выдал доступ к Диску: на экране согласия нужно отметить пункт про Google Диск. Попробуйте ещё раз.",
	domain.CodeDrivePublisherNoAccess:  "У этого аккаунта нет прав на добавление файлов в папку группы. Войдите владельцем папки или выдайте аккаунту права редактора.",
	domain.CodeDriveOAuthNotConfigured: "Подключение аккаунта Google не настроено на сервере.",
}

var oauthTmpl = template.Must(template.New("oauth").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Heatseeker — Google Диск</title>
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:32rem;margin:15vh auto;padding:0 1rem;color:#222}
h1{font-size:1.3rem}@media(prefers-color-scheme:dark){body{background:#111;color:#eee}}</style></head>
<body>{{if .Failed}}<h1>Аккаунт не подключён</h1><p>{{.Message}}</p>
{{else}}<h1>Аккаунт подключён</h1><p>Загрузки группы будут публиковаться на Google Диск от имени <b>{{.Email}}</b>.</p>{{end}}
<p>Эту вкладку можно закрыть и вернуться в Heatseeker.</p></body></html>`))

// oauthPage renders the result page; the URL carries a one-time code, so the
// page is neither cached nor allowed to leak it through Referer.
func oauthPage(status int, r oauthResult) *huma.StreamResponse {
	msg := oauthMessages[r.Code]
	switch {
	case msg != "":
	case r.Cancelled:
		msg = "Вход в Google отменён."
	case r.Expired:
		msg = "Ссылка для входа устарела. Начните подключение заново в приложении."
	default:
		msg = "Не удалось завершить вход в Google. Начните подключение заново в приложении."
	}
	var buf bytes.Buffer
	_ = oauthTmpl.Execute(&buf, struct {
		oauthResult
		Message string
	}{r, msg})
	return &huma.StreamResponse{Body: func(ctx huma.Context) {
		ctx.SetHeader("Content-Type", "text/html; charset=utf-8")
		ctx.SetHeader("Content-Length", strconv.Itoa(buf.Len()))
		ctx.SetHeader("Cache-Control", "no-store")
		ctx.SetHeader("Referrer-Policy", "no-referrer")
		ctx.SetHeader("X-Content-Type-Options", "nosniff")
		ctx.SetHeader("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
		ctx.SetStatus(status)
		_, _ = ctx.BodyWriter().Write(buf.Bytes())
	}}
}
