// Package bootstrap assembles the application graph from configuration. It is
// shared by the api/worker commands and by integration tests, so wiring is
// defined exactly once.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"heatseeker/api/internal/adapters/gdrive"
	googleadapter "heatseeker/api/internal/adapters/google"
	"heatseeker/api/internal/adapters/gotenberg"
	"heatseeker/api/internal/adapters/media"
	"heatseeker/api/internal/adapters/postgres"
	"heatseeker/api/internal/adapters/queue"
	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/app/discussions"
	"heatseeker/api/internal/app/drive"
	"heatseeker/api/internal/app/groups"
	"heatseeker/api/internal/app/materials"
	"heatseeker/api/internal/app/schedule"
	"heatseeker/api/internal/app/subjects"
	appsync "heatseeker/api/internal/app/sync"
	"heatseeker/api/internal/app/tags"
	"heatseeker/api/internal/app/tasks"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/jobs"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/db"
	"heatseeker/api/internal/platform/redisx"
	"heatseeker/api/internal/platform/secretbox"
	httptransport "heatseeker/api/internal/transport/http"
)

// Services is the assembled application.
type Services struct {
	Pool        *pgxpool.Pool
	Store       *postgres.Store
	Bus         *events.Bus
	Tokens      *auth.Tokens
	Auth        *auth.Service
	Groups      *groups.Service
	Subjects    *subjects.Service
	Tags        *tags.Service
	Sync        *appsync.Service
	Materials   *materials.Service
	Drive       *drive.Service
	Tasks       *tasks.Service
	Discussions *discussions.Service
	Schedule    *schedule.Service
	Media       domain.MediaStore
	Queue       domain.JobQueue

	closers []func()
}

// Options override infrastructure adapters (tests, tools). Zero values mean
// "build from configuration".
type Options struct {
	Queue       domain.JobQueue
	DriveClient domain.DriveClient
	// DriveAuthorizer connects the publishing Google account (D34).
	DriveAuthorizer domain.DriveAuthorizer
	// Converter makes PDF previews of office files.
	Converter domain.DocumentConverter
	Media     domain.MediaStore
	Clock     clock.Clock
}

// Build connects to Postgres and wires every service from configuration.
func Build(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Services, error) {
	return BuildWith(ctx, cfg, log, Options{})
}

// BuildWith is Build with adapter overrides.
func BuildWith(ctx context.Context, cfg *config.Config, log *slog.Logger, opts Options) (*Services, error) {
	var closers []func()
	fail := func(err error) (*Services, error) {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
		return nil, err
	}
	pool, err := db.NewPool(ctx, cfg.DB.URL, cfg.DB.MaxConns)
	if err != nil {
		return nil, err
	}
	closers = append(closers, pool.Close)

	if opts.Media == nil {
		if opts.Media, err = media.New(ctx, cfg.Storage); err != nil {
			return fail(fmt.Errorf("storage: %w", err))
		}
	}
	if opts.DriveClient == nil && cfg.GDrive.Enabled() {
		client, err := gdrive.New(ctx, cfg.GDrive.ServiceAccountJSONBase64)
		if err != nil {
			return fail(err)
		}
		opts.DriveClient = client
		checkDrive(ctx, client, log)
	}
	if opts.DriveAuthorizer == nil && cfg.PublisherOAuthEnabled() {
		opts.DriveAuthorizer = gdrive.NewOAuth(cfg.Auth.GoogleClientID, cfg.Auth.GoogleClientSecret, cfg.GDrive.OAuthRedirectURL)
		// This exact URL must be listed in the OAuth client's redirect URIs.
		log.Info("google drive publishing account can be connected", "redirect_url", cfg.GDrive.OAuthRedirectURL)
	}
	if opts.Converter == nil && cfg.Media.OfficePreviewURL != "" {
		opts.Converter = gotenberg.New(cfg.Media.OfficePreviewURL)
	}
	if opts.Queue == nil {
		redisOpt, err := redisx.AsynqOpt(cfg.Redis.URL)
		if err != nil {
			return fail(err)
		}
		q := queue.NewAsynq(redisOpt)
		closers = append(closers, func() { _ = q.Close() })
		opts.Queue = q
	}
	svc, err := wire(pool, cfg, log, opts)
	if err != nil {
		return fail(err)
	}
	svc.closers = closers
	return svc, nil
}

func wire(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger, opts Options) (*Services, error) {
	store := postgres.NewStore(pool)
	clk := opts.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	bus := events.NewBus()
	publisher := events.NewPublisher(store.Groups(), store.Events(), store, bus, log)
	acc := access.New(store.Users(), store.Groups(), store.Memberships())

	groupsSvc := groups.NewService(store.Groups(), store.Memberships(), store.Invites(), acc, publisher, store, groups.Defaults{
		JoinPolicy:       domain.JoinPolicy(strings.ToUpper(cfg.Groups.DefaultJoinPolicy)),
		MediaMode:        domain.MediaMode(strings.ToUpper(cfg.Groups.DefaultMediaMode)),
		InviteTTLDays:    cfg.Auth.InviteDefaultTTLDays,
		PublicReadSwitch: cfg.Auth.PublicReadEnabled,
	}, clk)

	tokens := auth.NewTokens(cfg.Auth.JWTAccessSecret, cfg.Auth.JWTAccessTTL, cfg.Auth.RefreshTokenTTLDays, cfg.Auth.RefreshTokenPepper, clk)
	var google auth.GoogleVerifier
	if len(cfg.Auth.GoogleAudiences) > 0 {
		google = googleadapter.NewVerifier(cfg.Auth.GoogleAudiences)
	}
	authSvc := auth.NewService(store.Users(), store.Auth(), store, tokens, google, groupsSvc, clk, log)

	materialsSvc := materials.NewService(materials.Deps{
		Materials: store.Materials(), Uploads: store.Uploads(), Subjects: store.Subjects(), Tags: store.Tags(),
		Drive: store.Drive(), Tasks: store.Tasks(), Store: opts.Media, Client: opts.DriveClient, Converter: opts.Converter, Access: acc, Events: publisher,
		Tx: store, Queue: opts.Queue, Clock: clk, Log: log,
	}, materials.Settings{
		APIBaseURL:          strings.TrimRight(cfg.App.BaseURL, "/") + "/api/v1",
		PresignTTL:          cfg.Media.PresignTTL(),
		UploadTTL:           time.Duration(cfg.Media.TmpUploadTTLHours) * time.Hour,
		MaxUploadBytes:      cfg.Media.UploadMaxBytes(),
		AllowedExt:          cfg.Media.UploadAllowedExt,
		HardDeleteAfter:     time.Duration(cfg.Media.HardDeleteAfterDays) * 24 * time.Hour,
		ProxyEnabled:        cfg.Media.ProxyEnabled,
		StreamTTL:           time.Duration(cfg.Media.StreamTokenTTLMinute) * time.Minute,
		DriveUploadEnabled:  cfg.GDrive.FeatureUpload,
		DrivePublisherOAuth: opts.DriveAuthorizer != nil,
		HashMaxBytes:        int64(cfg.Media.MaterialHashMaxMB) << 20,
		PreviewMaxBytes:     int64(cfg.Media.OfficePreviewMaxMB) << 20,
		MinConfidence:       cfg.GDrive.ClassifyMinConfidence,
		SigningSecret:       cfg.Auth.JWTAccessSecret,
	})
	// The publishing account's refresh token is sealed with APP_ENCRYPTION_KEY
	// (config validation requires it together with the OAuth client).
	var secrets *secretbox.Box
	if opts.DriveAuthorizer != nil {
		var err error
		if secrets, err = secretbox.New(cfg.App.EncryptionKey); err != nil {
			return nil, fmt.Errorf("APP_ENCRYPTION_KEY: %w", err)
		}
	}
	driveSvc := drive.NewService(drive.Deps{
		Repo: store.Drive(), Materials: store.Materials(), Subjects: store.Subjects(), Users: store.Users(),
		Store: opts.Media, Client: opts.DriveClient, Authorizer: opts.DriveAuthorizer, Secrets: secrets,
		Access: acc, Events: publisher, Tx: store, Queue: opts.Queue, Clock: clk, Log: log,
	}, drive.Settings{
		SyncInterval:       time.Duration(cfg.GDrive.SyncIntervalSec) * time.Second,
		FullRescanInterval: time.Duration(cfg.GDrive.FullRescanIntervalSec) * time.Second,
		MinConfidence:      cfg.GDrive.ClassifyMinConfidence,
		DeletePolicy:       cfg.GDrive.DeletePolicy,
		UploadEnabled:      cfg.GDrive.FeatureUpload,
		StateSecret:        cfg.Auth.JWTAccessSecret,
	})

	offsets, err := cfg.Tasks.Offsets()
	if err != nil {
		return nil, err
	}
	tasksSvc := tasks.NewService(tasks.Deps{
		Tasks: store.Tasks(), Subjects: store.Subjects(), Members: store.Memberships(),
		Access: acc, Events: publisher, Tx: store, Clock: clk, Log: log,
	}, tasks.Settings{
		DeadlineOffsets: offsets,
		ReminderGrace:   time.Duration(cfg.Tasks.ReminderGraceHours) * time.Hour,
		DueSoonWindow:   time.Duration(cfg.Tasks.DueSoonDays) * 24 * time.Hour,
		ScanLimit:       int32(cfg.Tasks.ScanLimit),
	})

	bus.Subscribe(driveSvc.OnEvent)

	return &Services{
		Pool:      pool,
		Store:     store,
		Bus:       bus,
		Tokens:    tokens,
		Auth:      authSvc,
		Groups:    groupsSvc,
		Subjects:  subjects.NewService(store.Subjects(), store.Tags(), acc, publisher, store),
		Tags:      tags.NewService(store.Tags(), store.Subjects(), acc, publisher, store),
		Sync:      appsync.NewService(store.Events(), acc),
		Materials: materialsSvc,
		Drive:     driveSvc,
		Tasks:     tasksSvc,
		Discussions: discussions.NewService(discussions.Deps{
			Discussions: store.Discussions(), Subjects: store.Subjects(), Materials: store.Materials(), Tasks: store.Tasks(),
			Access: acc, Events: publisher, Tx: store, Clock: clk, Log: log,
		}),
		Schedule: schedule.NewService(schedule.Deps{
			Schedule: store.Schedule(), Subjects: store.Subjects(), Access: acc, Events: publisher, Tx: store,
			Clock: clk, Log: log,
		}, schedule.Settings{
			APIBaseURL:    strings.TrimRight(cfg.App.BaseURL, "/") + "/api/v1",
			SigningSecret: cfg.Auth.JWTAccessSecret,
		}),
		Media: opts.Media,
		Queue: opts.Queue,
	}, nil
}

// checkDrive reports a misconfigured service account at startup instead of at
// the first folder connection. It never fails the start: Drive may recover.
func checkDrive(ctx context.Context, client *gdrive.Client, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := client.Ping(ctx)
	switch {
	case err == nil:
		log.Info("google drive ready", "service_account", client.ServiceAccountEmail())
	case domain.ErrorCode(err) == domain.CodeDriveAPIDisabled:
		log.Warn("google drive API is disabled: enable it in the Cloud console (see docs/GOOGLE-DRIVE.md)",
			"service_account", client.ServiceAccountEmail(), "err", err)
	default:
		log.Warn("google drive check failed", "service_account", client.ServiceAccountEmail(), "err", err)
	}
}

// Close releases resources.
func (s *Services) Close() {
	if s == nil {
		return
	}
	for i := len(s.closers) - 1; i >= 0; i-- {
		s.closers[i]()
	}
	s.closers = nil
}

// JobDeps returns what the background worker needs.
func (s *Services) JobDeps(cfg *config.Config, log *slog.Logger) jobs.Deps {
	return jobs.Deps{
		Auth:           s.Store.Auth(),
		Events:         s.Store.Events(),
		EventRetention: time.Duration(cfg.Events.RetentionDays) * 24 * time.Hour,
		Drive:          s.Drive,
		Materials:      s.Materials,
		Tasks:          s.Tasks,
		Log:            log,
	}
}

// HTTPServer builds the HTTP transport over the services.
func (s *Services) HTTPServer(cfg *config.Config, log *slog.Logger) *httptransport.Server {
	return httptransport.NewServer(httptransport.Deps{
		Cfg:         cfg,
		Log:         log,
		Tokens:      s.Tokens,
		Auth:        s.Auth,
		Groups:      s.Groups,
		Subjects:    s.Subjects,
		Tags:        s.Tags,
		Sync:        s.Sync,
		Materials:   s.Materials,
		Drive:       s.Drive,
		Tasks:       s.Tasks,
		Discussions: s.Discussions,
		Schedule:    s.Schedule,
		Health: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if err := s.Pool.Ping(pingCtx); err != nil {
				return errors.New("database unreachable")
			}
			return nil
		},
	})
}
