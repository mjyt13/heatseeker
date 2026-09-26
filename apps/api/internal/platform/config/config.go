// Package config loads and validates process configuration from environment
// variables. It is the single source of runtime settings: nothing else in the
// codebase reads os.Getenv.
package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the fully parsed configuration for both api and worker processes.
type Config struct {
	App     App
	DB      DB
	Redis   Redis
	Jobs    Jobs
	Auth    Auth
	Groups  Groups
	Tasks   Tasks
	Events  Events
	Notify  Notify
	Media   Media
	Storage Storage
	GDrive  GDrive
}

// App holds process-level settings.
type App struct {
	Env           string   `env:"APP_ENV" envDefault:"dev"`
	Port          int      `env:"APP_PORT" envDefault:"8000"`
	BaseURL       string   `env:"APP_BASE_URL" envDefault:"http://localhost:8000"`
	WebBaseURL    string   `env:"WEB_BASE_URL" envDefault:"http://localhost:5173"`
	CORSOrigins   []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:5173,http://localhost:4173"`
	LogLevel      string   `env:"LOG_LEVEL" envDefault:"info"`
	EncryptionKey string   `env:"APP_ENCRYPTION_KEY"`
	// TrustedProxyCount is how many reverse proxies (Caddy, load balancer) sit in
	// front of the API. 0 — trust only the TCP peer address; 1 — behind Caddy.
	TrustedProxyCount int `env:"TRUSTED_PROXY_COUNT" envDefault:"0"`
}

// DB holds PostgreSQL settings.
type DB struct {
	URL      string `env:"DATABASE_URL,required"`
	MaxConns int32  `env:"DB_MAX_CONNS" envDefault:"10"`
}

// Redis holds Redis settings (queues, pub/sub, cache).
type Redis struct {
	URL string `env:"REDIS_URL,required"`
}

// Jobs configures the asynq worker.
type Jobs struct {
	Concurrency int    `env:"JOBS_CONCURRENCY" envDefault:"8"`
	Queues      string `env:"JOBS_QUEUES" envDefault:"critical:6,default:3,low:1"`
}

// Auth configures tokens and identity providers.
type Auth struct {
	JWTAccessSecret       string        `env:"JWT_ACCESS_SECRET,required"`
	JWTAccessTTL          time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	RefreshTokenTTLDays   int           `env:"REFRESH_TOKEN_TTL_DAYS" envDefault:"180"`
	RefreshTokenPepper    string        `env:"REFRESH_TOKEN_PEPPER,required"`
	GoogleClientID        string        `env:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleClientSecret    string        `env:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GoogleAndroidClientID string        `env:"GOOGLE_OAUTH_ANDROID_CLIENT_ID"`
	GoogleIOSClientID     string        `env:"GOOGLE_OAUTH_IOS_CLIENT_ID"`
	RequireSecuredFor     []string      `env:"AUTH_REQUIRE_SECURED_FOR" envSeparator:"," envDefault:"admin,moderation,headman,drive_upload"`
	PublicReadEnabled     bool          `env:"PUBLIC_READ_ENABLED" envDefault:"false"`
	RegisterRatePerMinute int           `env:"AUTH_REGISTER_RATE_PER_MINUTE" envDefault:"10"`
	LoginRatePerMinute    int           `env:"AUTH_LOGIN_RATE_PER_MINUTE" envDefault:"20"`
	InviteDefaultTTLDays  int           `env:"INVITE_DEFAULT_TTL_DAYS" envDefault:"30"`
	GoogleAudiences       []string      `env:"-"`
}

// Groups holds defaults for newly created groups.
type Groups struct {
	DefaultJoinPolicy string `env:"GROUP_DEFAULT_JOIN_POLICY" envDefault:"open"`
	DefaultMediaMode  string `env:"MEDIA_MODE_DEFAULT" envDefault:"cache"`
}

// Tasks configures deadline reminders.
type Tasks struct {
	// DeadlineOffsets tells how long before a deadline the group is reminded
	// ("7d", "3h", "45m"); "0" announces the deadline itself.
	DeadlineOffsets []string `env:"DEADLINE_REMINDER_OFFSETS" envSeparator:"," envDefault:"7d,3d,1d,3h,0"`
	// ReminderGraceHours: deadlines older than this are never announced (a
	// task entered long after the fact).
	ReminderGraceHours int `env:"TASK_REMINDER_GRACE_HOURS" envDefault:"24"`
	// DueSoonDays is the window the "soon" counter on the board covers.
	DueSoonDays int `env:"TASK_DUE_SOON_DAYS" envDefault:"7"`
	// ScanLimit bounds one run of the deadline scanner.
	ScanLimit int `env:"TASK_DEADLINE_SCAN_LIMIT" envDefault:"500"`
}

// Offsets parses DeadlineOffsets into minutes before the deadline, sorted from
// the earliest reminder to the deadline itself.
func (t Tasks) Offsets() ([]int32, error) {
	out := make([]int32, 0, len(t.DeadlineOffsets))
	seen := map[int32]bool{}
	for _, raw := range t.DeadlineOffsets {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		minutes, err := parseOffset(raw)
		if err != nil {
			return nil, err
		}
		if !seen[minutes] {
			seen[minutes] = true
			out = append(out, minutes)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] > out[j] })
	return out, nil
}

// parseOffset reads "7d", "3h", "45m" or "0" as minutes.
func parseOffset(raw string) (int32, error) {
	unit := time.Minute
	value := raw
	switch {
	case strings.HasSuffix(raw, "d"):
		unit, value = 24*time.Hour, strings.TrimSuffix(raw, "d")
	case strings.HasSuffix(raw, "h"):
		unit, value = time.Hour, strings.TrimSuffix(raw, "h")
	case strings.HasSuffix(raw, "m"):
		value = strings.TrimSuffix(raw, "m")
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("DEADLINE_REMINDER_OFFSETS: %q must be a count of d|h|m, for example 3d", raw)
	}
	return int32(time.Duration(n) * unit / time.Minute), nil
}

// Events configures the group event log.
type Events struct {
	RetentionDays int `env:"EVENT_LOG_RETENTION_DAYS" envDefault:"90"`
}

// Notify configures notifications and their delivery (docs/PLAN.md §8).
type Notify struct {
	// Provider picks the push service; "none" keeps notifications in-app only.
	Provider string `env:"PUSH_PROVIDER" envDefault:"expo"`
	// ExpoAccessToken is needed only for Expo projects with enhanced security.
	ExpoAccessToken string `env:"EXPO_ACCESS_TOKEN"`
	// DelaySec is how long an event waits before it becomes a notification:
	// long enough for somebody already reading the discussion to be left alone.
	DelaySec int `env:"NOTIFY_DELAY_SEC" envDefault:"10"`
	// MaterialBatchMin is how many new files collapse into one line.
	MaterialBatchMin int `env:"NOTIFY_MATERIAL_BATCH_MIN" envDefault:"3"`
	// ScanLimit bounds one pass over a group's log.
	ScanLimit int `env:"NOTIFY_SCAN_LIMIT" envDefault:"500"`
	// RetentionDays is how long a notification stays in the list.
	RetentionDays int `env:"NOTIFY_RETENTION_DAYS" envDefault:"60"`
	// QuietHours silences push in the member's own timezone; empty — never.
	QuietHours string `env:"QUIET_HOURS_DEFAULT" envDefault:"23:00-08:00"`
	// DPODefault off starts DPO groups silent but for announcements.
	DPODefault string `env:"DPO_NOTIFICATIONS_DEFAULT" envDefault:"off"`
}

// PushEnabled reports whether notifications are also sent to devices.
func (n Notify) PushEnabled() bool { return strings.EqualFold(n.Provider, "expo") }

// DPOSilent reports whether DPO groups start with notifications off.
func (n Notify) DPOSilent() bool { return !strings.EqualFold(n.DPODefault, "on") }

// Quiet parses QUIET_HOURS_DEFAULT ("23:00-08:00") into minutes from
// midnight. Empty or "off" means no quiet hours.
func (n Notify) Quiet() (from, to *int16, err error) {
	spec := strings.TrimSpace(n.QuietHours)
	if spec == "" || strings.EqualFold(spec, "off") {
		return nil, nil, nil
	}
	left, right, ok := strings.Cut(spec, "-")
	if !ok {
		return nil, nil, fmt.Errorf("QUIET_HOURS_DEFAULT: %q must be HH:MM-HH:MM", spec)
	}
	start, err := parseClock(left)
	if err != nil {
		return nil, nil, err
	}
	end, err := parseClock(right)
	if err != nil {
		return nil, nil, err
	}
	return &start, &end, nil
}

// parseClock turns "23:00" into minutes from midnight.
func parseClock(v string) (int16, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("QUIET_HOURS_DEFAULT: %q must be HH:MM", v)
	}
	return int16(t.Hour()*60 + t.Minute()), nil //nolint:gosec // below 1440
}

// Media configures how material files are uploaded and served.
type Media struct {
	ProxyEnabled         bool     `env:"MEDIA_PROXY_ENABLED" envDefault:"true"`
	PresignTTLSec        int      `env:"PRESIGN_TTL_SEC" envDefault:"900"`
	TmpUploadTTLHours    int      `env:"TMP_UPLOAD_TTL_HOURS" envDefault:"24"`
	UploadMaxSizeMB      int      `env:"UPLOAD_MAX_SIZE_MB" envDefault:"200"`
	UploadAllowedExt     []string `env:"UPLOAD_ALLOWED_EXT" envSeparator:"," envDefault:"pdf,doc,docx,ppt,pptx,xls,xlsx,rtf,odt,odp,ods,txt,md,csv,png,jpg,jpeg,webp,heic,zip,mp3,m4a,ogg,flac,wav,mp4,mov,webm,mkv"`
	HardDeleteAfterDays  int      `env:"MATERIAL_HARD_DELETE_AFTER_DAYS" envDefault:"30"`
	MaterialHashMaxMB    int      `env:"MATERIAL_HASH_MAX_MB" envDefault:"500"`
	StreamTokenTTLMinute int      `env:"MEDIA_STREAM_TOKEN_TTL_MIN" envDefault:"60"`
	// OfficePreviewURL is the Gotenberg endpoint that turns office files into
	// PDF previews; empty disables previews (files are downloaded instead).
	OfficePreviewURL   string `env:"OFFICE_PREVIEW_URL"`
	OfficePreviewMaxMB int    `env:"OFFICE_PREVIEW_MAX_MB" envDefault:"50"`
}

// PresignTTL is the lifetime of direct upload/download URLs.
func (m Media) PresignTTL() time.Duration { return time.Duration(m.PresignTTLSec) * time.Second }

// UploadMaxBytes is the per-file size limit.
func (m Media) UploadMaxBytes() int64 { return int64(m.UploadMaxSizeMB) << 20 }

// Storage selects and configures the MediaStore driver.
type Storage struct {
	Driver           string `env:"STORAGE_DRIVER" envDefault:"s3"`
	LocalRoot        string `env:"STORAGE_LOCAL_ROOT" envDefault:"/var/lib/heatseeker/media"`
	S3Endpoint       string `env:"S3_ENDPOINT"`
	S3PublicEndpoint string `env:"S3_PUBLIC_ENDPOINT"`
	S3Region         string `env:"S3_REGION" envDefault:"us-east-1"`
	S3Bucket         string `env:"S3_BUCKET"`
	S3AccessKeyID    string `env:"S3_ACCESS_KEY_ID"`
	S3SecretKey      string `env:"S3_SECRET_ACCESS_KEY"`
	S3ForcePathStyle bool   `env:"S3_FORCE_PATH_STYLE" envDefault:"true"`
}

// GDrive configures the service-account Drive integration.
type GDrive struct {
	ServiceAccountJSONBase64 string  `env:"GDRIVE_SERVICE_ACCOUNT_JSON_BASE64"`
	SyncIntervalSec          int     `env:"GDRIVE_SYNC_INTERVAL_SEC" envDefault:"600"`
	FullRescanIntervalSec    int     `env:"GDRIVE_FULL_RESCAN_INTERVAL_SEC" envDefault:"86400"`
	ClassifyMinConfidence    float64 `env:"GDRIVE_CLASSIFY_MIN_CONFIDENCE" envDefault:"0.6"`
	DeletePolicy             string  `env:"GDRIVE_DELETE_POLICY" envDefault:"flag"`
	FeatureUpload            bool    `env:"FEATURE_DRIVE_UPLOAD" envDefault:"false"`
	// OAuthRedirectURL is where Google returns after the publishing account
	// signs in (D34). Empty: localhost in development (Google accepts no LAN
	// IPs), APP_BASE_URL otherwise.
	OAuthRedirectURL string `env:"GDRIVE_OAUTH_REDIRECT_URL"`
}

// DriveOAuthCallbackPath is the API route Google redirects to.
const DriveOAuthCallbackPath = "/api/v1/drive/oauth/callback"

// PublisherOAuthEnabled reports whether a publishing Google account can be
// connected: an OAuth web client is configured next to the service account.
func (c *Config) PublisherOAuthEnabled() bool {
	return c.GDrive.Enabled() && c.Auth.GoogleClientID != "" && c.Auth.GoogleClientSecret != ""
}

// Enabled reports whether a service account key is configured.
func (g GDrive) Enabled() bool { return strings.TrimSpace(g.ServiceAccountJSONBase64) != "" }

// IsDev reports whether the process runs in development mode.
func (c *Config) IsDev() bool { return c.App.Env == "dev" }

// Load reads .env (development only, when present) and the process
// environment, then validates the result. It fails fast on any problem.
func Load() (*Config, error) {
	if os.Getenv("APP_ENV") == "" || os.Getenv("APP_ENV") == "dev" {
		// Ignore a missing .env: production sets variables directly.
		_ = godotenv.Load()
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cfg.Auth.GoogleAudiences = compact(cfg.Auth.GoogleClientID, cfg.Auth.GoogleAndroidClientID, cfg.Auth.GoogleIOSClientID)
	return &cfg, nil
}

func (c *Config) validate() error {
	var errs []error
	switch c.App.Env {
	case "dev", "prod", "test":
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be dev|prod|test, got %q", c.App.Env))
	}
	if len(c.Auth.JWTAccessSecret) < 32 {
		errs = append(errs, errors.New("JWT_ACCESS_SECRET must be at least 32 characters"))
	}
	if len(c.Auth.RefreshTokenPepper) < 16 {
		errs = append(errs, errors.New("REFRESH_TOKEN_PEPPER must be at least 16 characters"))
	}
	if c.Auth.RefreshTokenTTLDays <= 0 {
		errs = append(errs, errors.New("REFRESH_TOKEN_TTL_DAYS must be positive"))
	}
	if c.Auth.JWTAccessTTL < time.Minute {
		errs = append(errs, errors.New("JWT_ACCESS_TTL must be at least 1m"))
	}
	if c.App.EncryptionKey != "" && len(c.App.EncryptionKey) < 32 {
		errs = append(errs, errors.New("APP_ENCRYPTION_KEY must be at least 32 characters when set"))
	}
	switch strings.ToLower(c.Groups.DefaultJoinPolicy) {
	case "open", "invite", "approval":
	default:
		errs = append(errs, fmt.Errorf("GROUP_DEFAULT_JOIN_POLICY must be open|invite|approval, got %q", c.Groups.DefaultJoinPolicy))
	}
	switch strings.ToLower(c.Groups.DefaultMediaMode) {
	case "link", "cache", "import":
	default:
		errs = append(errs, fmt.Errorf("MEDIA_MODE_DEFAULT must be link|cache|import, got %q", c.Groups.DefaultMediaMode))
	}
	if _, err := c.Tasks.Offsets(); err != nil {
		errs = append(errs, err)
	}
	if c.Tasks.DueSoonDays <= 0 {
		errs = append(errs, errors.New("TASK_DUE_SOON_DAYS must be positive"))
	}
	if c.Events.RetentionDays <= 0 {
		errs = append(errs, errors.New("EVENT_LOG_RETENTION_DAYS must be positive"))
	}
	errs = append(errs, c.Notify.validate()...)
	errs = append(errs, c.Media.validate()...)
	errs = append(errs, c.Storage.validate()...)
	errs = append(errs, c.GDrive.validate()...)
	if c.Auth.GoogleClientSecret != "" && c.Auth.GoogleClientID == "" {
		errs = append(errs, errors.New("GOOGLE_OAUTH_CLIENT_SECRET requires GOOGLE_OAUTH_CLIENT_ID"))
	}
	if c.PublisherOAuthEnabled() && c.App.EncryptionKey == "" {
		errs = append(errs, errors.New("APP_ENCRYPTION_KEY is required to store the publishing Google account's token (GOOGLE_OAUTH_CLIENT_SECRET is set)"))
	}
	if c.GDrive.OAuthRedirectURL == "" {
		base := strings.TrimRight(c.App.BaseURL, "/")
		if c.IsDev() {
			base = fmt.Sprintf("http://localhost:%d", c.App.Port)
		}
		c.GDrive.OAuthRedirectURL = base + DriveOAuthCallbackPath
	}
	return errors.Join(errs...)
}

func (m *Media) validate() []error {
	var errs []error
	if m.PresignTTLSec < 60 || m.PresignTTLSec > 7*24*3600 {
		errs = append(errs, errors.New("PRESIGN_TTL_SEC must be between 60 and 604800"))
	}
	if m.TmpUploadTTLHours <= 0 {
		errs = append(errs, errors.New("TMP_UPLOAD_TTL_HOURS must be positive"))
	}
	if m.UploadMaxSizeMB <= 0 {
		errs = append(errs, errors.New("UPLOAD_MAX_SIZE_MB must be positive"))
	}
	if m.HardDeleteAfterDays < 0 {
		errs = append(errs, errors.New("MATERIAL_HARD_DELETE_AFTER_DAYS must not be negative"))
	}
	if m.StreamTokenTTLMinute <= 0 {
		errs = append(errs, errors.New("MEDIA_STREAM_TOKEN_TTL_MIN must be positive"))
	}
	m.OfficePreviewURL = strings.TrimRight(strings.TrimSpace(m.OfficePreviewURL), "/")
	if m.OfficePreviewURL != "" && !strings.HasPrefix(m.OfficePreviewURL, "http://") && !strings.HasPrefix(m.OfficePreviewURL, "https://") {
		errs = append(errs, errors.New("OFFICE_PREVIEW_URL must be an http(s) URL"))
	}
	if m.OfficePreviewMaxMB <= 0 {
		errs = append(errs, errors.New("OFFICE_PREVIEW_MAX_MB must be positive"))
	}
	exts := make([]string, 0, len(m.UploadAllowedExt))
	for _, e := range m.UploadAllowedExt {
		if e = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(e), ".")); e != "" {
			exts = append(exts, e)
		}
	}
	if len(exts) == 0 {
		errs = append(errs, errors.New("UPLOAD_ALLOWED_EXT must list at least one extension"))
	}
	m.UploadAllowedExt = exts
	return errs
}

func (s *Storage) validate() []error {
	var errs []error
	s.Driver = strings.ToLower(strings.TrimSpace(s.Driver))
	switch s.Driver {
	case "s3":
		for _, kv := range [][2]string{
			{"S3_ENDPOINT", s.S3Endpoint}, {"S3_BUCKET", s.S3Bucket},
			{"S3_ACCESS_KEY_ID", s.S3AccessKeyID}, {"S3_SECRET_ACCESS_KEY", s.S3SecretKey},
		} {
			if strings.TrimSpace(kv[1]) == "" {
				errs = append(errs, fmt.Errorf("%s is required when STORAGE_DRIVER=s3", kv[0]))
			}
		}
		if s.S3PublicEndpoint == "" {
			s.S3PublicEndpoint = s.S3Endpoint
		}
	case "local":
		if strings.TrimSpace(s.LocalRoot) == "" {
			errs = append(errs, errors.New("STORAGE_LOCAL_ROOT is required when STORAGE_DRIVER=local"))
		}
	default:
		errs = append(errs, fmt.Errorf("STORAGE_DRIVER must be s3|local, got %q", s.Driver))
	}
	return errs
}

func (g *GDrive) validate() []error {
	var errs []error
	if g.SyncIntervalSec < 60 {
		errs = append(errs, errors.New("GDRIVE_SYNC_INTERVAL_SEC must be at least 60"))
	}
	if g.FullRescanIntervalSec < g.SyncIntervalSec {
		errs = append(errs, errors.New("GDRIVE_FULL_RESCAN_INTERVAL_SEC must not be shorter than GDRIVE_SYNC_INTERVAL_SEC"))
	}
	if g.ClassifyMinConfidence < 0 || g.ClassifyMinConfidence > 1 {
		errs = append(errs, errors.New("GDRIVE_CLASSIFY_MIN_CONFIDENCE must be within 0..1"))
	}
	g.DeletePolicy = strings.ToLower(strings.TrimSpace(g.DeletePolicy))
	if g.DeletePolicy != "flag" && g.DeletePolicy != "archive" {
		errs = append(errs, fmt.Errorf("GDRIVE_DELETE_POLICY must be flag|archive, got %q", g.DeletePolicy))
	}
	if g.FeatureUpload && !g.Enabled() {
		errs = append(errs, errors.New("FEATURE_DRIVE_UPLOAD requires GDRIVE_SERVICE_ACCOUNT_JSON_BASE64"))
	}
	return errs
}

func compact(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (n *Notify) validate() []error {
	var errs []error
	switch strings.ToLower(n.Provider) {
	case "expo", "none":
	default:
		errs = append(errs, fmt.Errorf("PUSH_PROVIDER must be expo|none, got %q", n.Provider))
	}
	if _, _, err := n.Quiet(); err != nil {
		errs = append(errs, err)
	}
	if n.DelaySec < 0 || n.DelaySec > 600 {
		errs = append(errs, errors.New("NOTIFY_DELAY_SEC must be between 0 and 600"))
	}
	if n.MaterialBatchMin < 2 {
		errs = append(errs, errors.New("NOTIFY_MATERIAL_BATCH_MIN must be at least 2"))
	}
	if n.RetentionDays <= 0 {
		errs = append(errs, errors.New("NOTIFY_RETENTION_DAYS must be positive"))
	}
	return errs
}
