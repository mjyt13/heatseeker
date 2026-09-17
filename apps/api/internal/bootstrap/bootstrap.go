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
	"heatseeker/api/internal/adapters/media"
	"heatseeker/api/internal/adapters/postgres"
	"heatseeker/api/internal/adapters/queue"
	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/app/drive"
	"heatseeker/api/internal/app/groups"
	"heatseeker/api/internal/app/materials"
	"heatseeker/api/internal/app/subjects"
	appsync "heatseeker/api/internal/app/sync"
	"heatseeker/api/internal/app/tags"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/jobs"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/db"
	"heatseeker/api/internal/platform/redisx"
	httptransport "heatseeker/api/internal/transport/http"
)

// Services is the assembled application.
type Services struct {
	Pool      *pgxpool.Pool
	Store     *postgres.Store
	Bus       *events.Bus
	Tokens    *auth.Tokens
	Auth      *auth.Service
	Groups    *groups.Service
	Subjects  *subjects.Service
	Tags      *tags.Service
	Sync      *appsync.Service
	Materials *materials.Service
	Drive     *drive.Service
	Media     domain.MediaStore
	Queue     domain.JobQueue

	closers []func()
}

// Options override infrastructure adapters (tests, tools). Zero values mean
// "build from configuration".
type Options struct {
	Queue       domain.JobQueue
	DriveClient domain.DriveClient
	Media       domain.MediaStore
	Clock       clock.Clock
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
	svc := wire(pool, cfg, log, opts)
	svc.closers = closers
	return svc, nil
}

func wire(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger, opts Options) *Services {
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
		Drive: store.Drive(), Store: opts.Media, Client: opts.DriveClient, Access: acc, Events: publisher,
		Tx: store, Queue: opts.Queue, Clock: clk, Log: log,
	}, materials.Settings{
		APIBaseURL:         strings.TrimRight(cfg.App.BaseURL, "/") + "/api/v1",
		PresignTTL:         cfg.Media.PresignTTL(),
		UploadTTL:          time.Duration(cfg.Media.TmpUploadTTLHours) * time.Hour,
		MaxUploadBytes:     cfg.Media.UploadMaxBytes(),
		AllowedExt:         cfg.Media.UploadAllowedExt,
		HardDeleteAfter:    time.Duration(cfg.Media.HardDeleteAfterDays) * 24 * time.Hour,
		ProxyEnabled:       cfg.Media.ProxyEnabled,
		StreamTTL:          time.Duration(cfg.Media.StreamTokenTTLMinute) * time.Minute,
		DriveUploadEnabled: cfg.GDrive.FeatureUpload,
		HashMaxBytes:       int64(cfg.Media.MaterialHashMaxMB) << 20,
		MinConfidence:      cfg.GDrive.ClassifyMinConfidence,
		SigningSecret:      cfg.Auth.JWTAccessSecret,
	})
	driveSvc := drive.NewService(drive.Deps{
		Repo: store.Drive(), Materials: store.Materials(), Subjects: store.Subjects(), Users: store.Users(),
		Store: opts.Media, Client: opts.DriveClient, Access: acc, Events: publisher, Tx: store,
		Queue: opts.Queue, Clock: clk, Log: log,
	}, drive.Settings{
		SyncInterval:       time.Duration(cfg.GDrive.SyncIntervalSec) * time.Second,
		FullRescanInterval: time.Duration(cfg.GDrive.FullRescanIntervalSec) * time.Second,
		MinConfidence:      cfg.GDrive.ClassifyMinConfidence,
		DeletePolicy:       cfg.GDrive.DeletePolicy,
		UploadEnabled:      cfg.GDrive.FeatureUpload,
	})

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
		Media:     opts.Media,
		Queue:     opts.Queue,
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
		Log:            log,
	}
}

// HTTPServer builds the HTTP transport over the services.
func (s *Services) HTTPServer(cfg *config.Config, log *slog.Logger) *httptransport.Server {
	return httptransport.NewServer(httptransport.Deps{
		Cfg:       cfg,
		Log:       log,
		Tokens:    s.Tokens,
		Auth:      s.Auth,
		Groups:    s.Groups,
		Subjects:  s.Subjects,
		Tags:      s.Tags,
		Sync:      s.Sync,
		Materials: s.Materials,
		Drive:     s.Drive,
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
