// Package config loads and validates process configuration from environment
// variables. It is the single source of runtime settings: nothing else in the
// codebase reads os.Getenv.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the fully parsed configuration for both api and worker processes.
type Config struct {
	App    App
	DB     DB
	Redis  Redis
	Jobs   Jobs
	Auth   Auth
	Groups Groups
	Events Events
}

// App holds process-level settings.
type App struct {
	Env           string   `env:"APP_ENV" envDefault:"dev"`
	Port          int      `env:"APP_PORT" envDefault:"8080"`
	BaseURL       string   `env:"APP_BASE_URL" envDefault:"http://localhost:8080"`
	WebBaseURL    string   `env:"WEB_BASE_URL" envDefault:"http://localhost:5173"`
	CORSOrigins   []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:5173,http://localhost:8081"`
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

// Events configures the group event log.
type Events struct {
	RetentionDays int `env:"EVENT_LOG_RETENTION_DAYS" envDefault:"90"`
}

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
	if c.Events.RetentionDays <= 0 {
		errs = append(errs, errors.New("EVENT_LOG_RETENTION_DAYS must be positive"))
	}
	return errors.Join(errs...)
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
