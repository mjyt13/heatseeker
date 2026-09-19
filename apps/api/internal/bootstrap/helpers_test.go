//go:build integration

package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"heatseeker/api/internal/adapters/gdrive"
	"heatseeker/api/internal/adapters/media"
	"heatseeker/api/internal/adapters/queue"
	"heatseeker/api/internal/bootstrap"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/db"
)

// env is a running API over the dev database with fake Drive and queue.
type env struct {
	cfg   *config.Config
	svc   *bootstrap.Services
	srv   *httptest.Server
	c     *client
	drive *gdrive.Fake
	oauth *gdrive.FakeOAuth // the publishing account signs in as publisher@example.com
	queue *queue.Recorder
}

func testConfig(dsn string) *config.Config {
	cfg := &config.Config{}
	cfg.App.Env = "test"
	cfg.App.CORSOrigins = []string{"http://localhost"}
	cfg.DB.URL = dsn
	cfg.DB.MaxConns = 4
	cfg.Auth.JWTAccessSecret = "integration-test-secret-integration-test-secret"
	cfg.Auth.JWTAccessTTL = 15 * time.Minute
	cfg.Auth.RefreshTokenTTLDays = 30
	cfg.Auth.RefreshTokenPepper = "integration-pepper-integration"
	cfg.Auth.InviteDefaultTTLDays = 30
	cfg.Auth.RegisterRatePerMinute = 1000
	cfg.Auth.LoginRatePerMinute = 1000
	cfg.Groups.DefaultJoinPolicy = "open"
	cfg.Groups.DefaultMediaMode = "cache"
	cfg.Events.RetentionDays = 90
	cfg.Media = config.Media{
		ProxyEnabled: true, PresignTTLSec: 900, TmpUploadTTLHours: 24, UploadMaxSizeMB: 1,
		UploadAllowedExt: []string{"pdf", "docx", "txt", "png"}, HardDeleteAfterDays: 0, // purge immediately when asked
		MaterialHashMaxMB: 10, StreamTokenTTLMinute: 60, OfficePreviewMaxMB: 1,
	}
	cfg.Storage.Driver = "local"
	cfg.App.EncryptionKey = "integration-test-encryption-key-0123456789"
	cfg.GDrive = config.GDrive{
		SyncIntervalSec: 600, FullRescanIntervalSec: 86400, ClassifyMinConfidence: 0.6,
		DeletePolicy: "flag", FeatureUpload: true,
	}
	return cfg
}

func setup(t *testing.T) *env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://heatseeker:heatseeker@localhost:5433/heatseeker_test?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := db.Migrate(ctx, dsn, "up", log); err != nil {
		t.Skipf("database not available (%v); skipping integration test", err)
	}
	cfg := testConfig(dsn)
	store, err := media.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fake := gdrive.NewFake()
	oauth := gdrive.NewFakeOAuth(fake, "publisher@example.com")
	rec := &queue.Recorder{}
	// The API base URL is only known once the test server listens, and the
	// services need it for signed links: start the listener first.
	srv := httptest.NewUnstartedServer(nil)
	cfg.App.BaseURL = "http://" + srv.Listener.Addr().String()
	svc, err := bootstrap.BuildWith(ctx, cfg, log, bootstrap.Options{Queue: rec, DriveClient: fake, DriveAuthorizer: oauth, Converter: fakeConverter{}, Media: store})
	if err != nil {
		t.Fatal(err)
	}
	srv.Config.Handler = svc.HTTPServer(cfg, log).Router
	srv.Start()
	t.Cleanup(func() {
		srv.Close()
		svc.Close()
	})
	return &env{cfg: cfg, svc: svc, srv: srv, c: &client{t: t, base: srv.URL + "/api/v1"}, drive: fake, oauth: oauth, queue: rec}
}

type client struct {
	t    *testing.T
	base string
}

func (c *client) do(method, path, token string, body any, wantStatus int, out any) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != wantStatus {
		c.t.Fatalf("%s %s: status %d, want %d\n%s", method, path, res.StatusCode, wantStatus, raw)
	}
	if out != nil && len(raw) > 0 {
		// Decode into a zero value: json keeps stale fields that the new
		// document omits (omitempty), which would leak between calls.
		target := reflect.ValueOf(out).Elem()
		target.Set(reflect.Zero(target.Type()))
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode: %v\n%s", method, path, err, raw)
		}
	}
}

// raw performs a request against an absolute URL and returns status, headers
// and body.
func (c *client) raw(method, url string, body io.Reader, headers map[string]string) (int, http.Header, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		c.t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, data
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// fakeConverter stands in for Gotenberg: the "PDF" names the extension and
// repeats the content; content "broken" is rejected like a corrupt file.
type fakeConverter struct{}

func (fakeConverter) ConvertToPDF(_ context.Context, fileName string, r io.Reader) (io.ReadCloser, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if string(body) == "broken" {
		return nil, fmt.Errorf("%w: cannot convert", domain.ErrInvalid)
	}
	pdf := "%PDF-1.7 preview of " + strings.ToLower(path.Ext(fileName)) + ": " + string(body)
	return io.NopCloser(strings.NewReader(pdf)), nil
}
