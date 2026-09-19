package config

import (
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_URL", "redis://x")
	t.Setenv("JWT_ACCESS_SECRET", strings.Repeat("s", 32))
	t.Setenv("REFRESH_TOKEN_PEPPER", strings.Repeat("p", 16))
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("STORAGE_LOCAL_ROOT", t.TempDir())
}

func TestLoadDefaults(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("UPLOAD_ALLOWED_EXT", " PDF, .docx ,,txt")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.Media.UploadAllowedExt, ","); got != "pdf,docx,txt" {
		t.Errorf("allowed ext = %q", got)
	}
	if cfg.Media.UploadMaxBytes() != 200<<20 || cfg.Media.PresignTTL().Seconds() != 900 {
		t.Errorf("media defaults = %+v", cfg.Media)
	}
	if cfg.GDrive.Enabled() || cfg.GDrive.DeletePolicy != "flag" || cfg.GDrive.SyncIntervalSec != 600 {
		t.Errorf("drive defaults = %+v", cfg.GDrive)
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"s3 without credentials", map[string]string{"STORAGE_DRIVER": "s3"}, "S3_BUCKET is required"},
		{"unknown driver", map[string]string{"STORAGE_DRIVER": "ftp"}, "STORAGE_DRIVER must be"},
		{"upload flag without key", map[string]string{"FEATURE_DRIVE_UPLOAD": "true"}, "FEATURE_DRIVE_UPLOAD requires"},
		{"bad delete policy", map[string]string{"GDRIVE_DELETE_POLICY": "purge"}, "GDRIVE_DELETE_POLICY"},
		{"oauth secret without id", map[string]string{"GOOGLE_OAUTH_CLIENT_SECRET": "x"}, "requires GOOGLE_OAUTH_CLIENT_ID"},
		{"publisher oauth without encryption key", map[string]string{
			"GDRIVE_SERVICE_ACCOUNT_JSON_BASE64": "e30=", "GOOGLE_OAUTH_CLIENT_ID": "id", "GOOGLE_OAUTH_CLIENT_SECRET": "x",
		}, "APP_ENCRYPTION_KEY is required"},
		{"sync too often", map[string]string{"GDRIVE_SYNC_INTERVAL_SEC": "5"}, "at least 60"},
		{"confidence out of range", map[string]string{"GDRIVE_CLASSIFY_MIN_CONFIDENCE": "1.5"}, "within 0..1"},
		{"empty allowlist", map[string]string{"UPLOAD_ALLOWED_EXT": " , "}, "at least one extension"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestDriveOAuthRedirect(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		// Google rejects LAN IPs, so development always returns to localhost.
		{"dev uses localhost", map[string]string{"APP_ENV": "dev", "APP_BASE_URL": "http://192.168.1.5:8000", "APP_PORT": "8001"},
			"http://localhost:8001/api/v1/drive/oauth/callback"},
		{"prod uses the base url", map[string]string{"APP_ENV": "prod", "APP_BASE_URL": "https://hs.example/"},
			"https://hs.example/api/v1/drive/oauth/callback"},
		{"explicit", map[string]string{"GDRIVE_OAUTH_REDIRECT_URL": "https://x.example/cb"}, "https://x.example/cb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if tt.env["APP_ENV"] == "dev" {
				t.Chdir(t.TempDir()) // no .env to pick up
			}
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.GDrive.OAuthRedirectURL != tt.want {
				t.Fatalf("redirect = %q, want %q", cfg.GDrive.OAuthRedirectURL, tt.want)
			}
		})
	}
}
