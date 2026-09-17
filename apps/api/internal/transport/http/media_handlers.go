package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"

	"heatseeker/api/internal/app/materials"
	"heatseeker/api/internal/platform/filenames"
)

type streamInput struct {
	MaterialID string `path:"materialId" format:"uuid"`
	Token      string `query:"token" required:"true" doc:"Из ответа /materials/{materialId}/open."`
	Range      string `header:"Range"`
}

type mediaGetInput struct {
	Token string `path:"token"`
	Range string `header:"Range"`
}

// mediaPutInput streams the request body straight to storage: it has no Body
// field, so huma leaves the body unread.
type mediaPutInput struct {
	Token string `path:"token"`
	body  io.Reader
}

// Resolve implements huma.Resolver.
func (m *mediaPutInput) Resolve(ctx huma.Context) []error {
	m.body = ctx.BodyReader()
	return nil
}

var binarySchema = &huma.Schema{Type: "string", Format: "binary"}

func binaryResponses(description string) map[string]*huma.Response {
	content := map[string]*huma.MediaType{"application/octet-stream": {Schema: binarySchema}}
	return map[string]*huma.Response{
		"200": {Description: description, Content: content},
		"206": {Description: "Часть файла (Range).", Content: content},
	}
}

func registerMedia(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "materials-stream", Method: nethttp.MethodGet, Path: "/materials/{materialId}/stream", Tags: []string{"media"},
		Summary:     "Файл с Google Диска через сервер",
		Description: "Ссылка берётся из /materials/{materialId}/open. Поддерживает Range; документы Google отдаются в PDF.",
		Responses:   binaryResponses("Содержимое файла."),
	}, func(ctx context.Context, in *streamInput) (*huma.StreamResponse, error) {
		id, err := parseID("materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		content, err := d.Materials.Stream(ctx, id, in.Token, in.Range)
		if err != nil {
			return nil, mediaErr(d, err)
		}
		return streamContent(d, content), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "media-get", Method: nethttp.MethodGet, Path: "/media/{token}", Tags: []string{"media"},
		Summary:   "Файл из локального хранилища по подписанной ссылке",
		Responses: binaryResponses("Содержимое файла."),
	}, func(ctx context.Context, in *mediaGetInput) (*huma.StreamResponse, error) {
		content, err := d.Materials.LocalGet(ctx, in.Token, in.Range)
		if err != nil {
			return nil, mediaErr(d, err)
		}
		return streamContent(d, content), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "media-put", Method: nethttp.MethodPut, Path: "/media/{token}", Tags: []string{"media"},
		Summary:       "Загрузка файла в локальное хранилище по подписанной ссылке",
		Description:   "Используется, когда сервер работает без S3 (STORAGE_DRIVER=local): ссылку выдаёт /groups/{groupId}/materials/uploads.",
		DefaultStatus: nethttp.StatusNoContent,
		RequestBody: &huma.RequestBody{
			Required: true,
			Content:  map[string]*huma.MediaType{"application/octet-stream": {Schema: binarySchema}},
		},
	}, func(ctx context.Context, in *mediaPutInput) (*emptyOutput, error) {
		if in.body == nil {
			return nil, huma.Error400BadRequest("request body is required")
		}
		if err := d.Materials.LocalPut(ctx, in.Token, in.body); err != nil {
			return nil, mediaErr(d, err)
		}
		return &emptyOutput{}, nil
	})
}

func mediaErr(d Deps, err error) error {
	if materials.IsRangeError(err) {
		return huma.NewError(nethttp.StatusRequestedRangeNotSatisfiable, "range not satisfiable")
	}
	return apiErr(d.Log, err)
}

// streamContent writes a file body with the headers browsers and media
// players expect.
func streamContent(d Deps, c *materials.Content) *huma.StreamResponse {
	return &huma.StreamResponse{Body: func(ctx huma.Context) {
		defer func() { _ = c.Body.Close() }()
		// Large files outlive the server's default write timeout.
		if _, w := humachi.Unwrap(ctx); w != nil {
			_ = nethttp.NewResponseController(w).SetWriteDeadline(time.Time{})
		}
		contentType := c.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		ctx.SetHeader("Content-Type", contentType)
		ctx.SetHeader("Content-Disposition", filenames.ContentDisposition(c.FileName, c.Inline))
		ctx.SetHeader("X-Content-Type-Options", "nosniff")
		if contentType != "application/pdf" {
			// Browser PDF viewers refuse to run inside a sandboxed document.
			ctx.SetHeader("Content-Security-Policy", "default-src 'none'; sandbox")
		}
		ctx.SetHeader("Cache-Control", "private, max-age=300")
		if c.ContentRange != "" || c.Partial || c.ContentLength >= 0 {
			ctx.SetHeader("Accept-Ranges", "bytes")
		}
		if c.ContentLength >= 0 {
			ctx.SetHeader("Content-Length", strconv.FormatInt(c.ContentLength, 10))
		}
		status := nethttp.StatusOK
		if c.Partial {
			status = nethttp.StatusPartialContent
			if c.ContentRange != "" {
				ctx.SetHeader("Content-Range", c.ContentRange)
			}
		}
		ctx.SetStatus(status)
		if _, err := io.Copy(ctx.BodyWriter(), c.Body); err != nil && !errors.Is(err, context.Canceled) {
			d.Log.Debug("media stream interrupted", "err", fmt.Sprint(err))
		}
	}}
}
