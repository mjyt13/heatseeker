// Package gotenberg adapts a Gotenberg server (LibreOffice behind an HTTP API)
// to domain.DocumentConverter: office files become PDF previews.
package gotenberg

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"heatseeker/api/internal/domain"
)

// convertTimeout bounds one conversion; large presentations take a while.
const convertTimeout = 3 * time.Minute

// Client implements domain.DocumentConverter.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client for the Gotenberg server at baseURL.
func New(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: convertTimeout}}
}

// ConvertToPDF implements domain.DocumentConverter. The file is streamed to
// Gotenberg as a multipart form without buffering it in memory.
func (c *Client) ConvertToPDF(ctx context.Context, fileName string, r io.Reader) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	go func() {
		part, err := form.CreateFormFile("files", safeName(fileName))
		if err == nil {
			_, err = io.Copy(part, r)
		}
		if err == nil {
			err = form.Close()
		}
		_ = pw.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/forms/libreoffice/convert", pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		return nil, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		_ = pr.CloseWithError(err)
		return nil, fmt.Errorf("gotenberg: %w", err)
	}
	if res.StatusCode == http.StatusOK {
		return res.Body, nil
	}
	defer func() { _ = res.Body.Close() }()
	msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	detail := strings.TrimSpace(string(msg))
	if res.StatusCode >= 400 && res.StatusCode < 500 && res.StatusCode != http.StatusTooManyRequests {
		// A broken or password-protected file: retrying will not help.
		return nil, fmt.Errorf("%w: gotenberg cannot convert the file: %d %s", domain.ErrInvalid, res.StatusCode, detail)
	}
	return nil, fmt.Errorf("gotenberg: status %d: %s", res.StatusCode, detail)
}

// safeName keeps the extension LibreOffice needs and drops everything that
// could confuse the multipart header.
func safeName(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if ext == "" || len(ext) > 6 {
		ext = ".bin"
	}
	return "document" + ext
}

var _ domain.DocumentConverter = (*Client)(nil)
