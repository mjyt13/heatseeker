//go:build integration

package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/ids"
)

// Runs against MinIO from infra/compose (S3_TEST_ENDPOINT to override).
func TestS3Store(t *testing.T) {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:9100"
	}
	cfg := config.Storage{
		Driver: "s3", S3Endpoint: endpoint, S3PublicEndpoint: endpoint, S3Region: "us-east-1",
		S3Bucket: "heatseeker-test", S3ForcePathStyle: true,
		S3AccessKeyID: envOr("S3_TEST_ACCESS_KEY_ID", "heatseeker"), S3SecretKey: envOr("S3_TEST_SECRET_ACCESS_KEY", "heatseeker-dev-secret"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := NewS3(ctx, cfg)
	if err != nil {
		t.Skipf("MinIO not available (%v)", err)
	}
	prefix := "test/" + ids.Code(8) + "/"

	// direct upload with a presigned URL, as the app does
	put, err := store.PresignPut(ctx, prefix+"tmp.pdf", "application/pdf", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(put.Method, put.URL, strings.NewReader("%PDF-1.4 hello"))
	for k, v := range put.Headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("presigned PUT: %d", res.StatusCode)
	}

	if err := store.Move(ctx, prefix+"tmp.pdf", prefix+"final/Лекция 1.pdf"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, prefix+"tmp.pdf"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("moved source still exists: %v", err)
	}
	info, err := store.Stat(ctx, prefix+"final/Лекция 1.pdf")
	if err != nil || info.Size != 14 {
		t.Fatalf("stat: %+v %v", info, err)
	}
	r, _, err := store.Open(ctx, prefix+"final/Лекция 1.pdf", 9, 5)
	if err != nil {
		t.Fatal(err)
	}
	part, _ := io.ReadAll(r)
	_ = r.Close()
	if string(part) != "hello" {
		t.Fatalf("range = %q", part)
	}

	get, err := store.PresignGet(ctx, prefix+"final/Лекция 1.pdf", "Лекция 1.pdf", false, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.Get(get.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 || string(body) != "%PDF-1.4 hello" || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("presigned GET: %d %q %v", res.StatusCode, body, res.Header)
	}

	if err := store.Put(ctx, prefix+"stream.txt", strings.NewReader("unknown size"), -1, "text/plain"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{prefix + "final/Лекция 1.pdf", prefix + "stream.txt", prefix + "missing"} {
		if err := store.Delete(ctx, k); err != nil {
			t.Fatalf("delete %s: %v", k, err)
		}
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
