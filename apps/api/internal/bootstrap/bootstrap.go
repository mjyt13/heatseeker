// Package bootstrap assembles the application graph from configuration. It is
// shared by the api/worker commands and by integration tests, so wiring is
// defined exactly once.
package bootstrap

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	googleadapter "heatseeker/api/internal/adapters/google"
	"heatseeker/api/internal/adapters/postgres"
	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/app/groups"
	"heatseeker/api/internal/app/subjects"
	appsync "heatseeker/api/internal/app/sync"
	"heatseeker/api/internal/app/tags"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/db"
	httptransport "heatseeker/api/internal/transport/http"
)

// Services is the assembled application.
type Services struct {
	Pool     *pgxpool.Pool
	Store    *postgres.Store
	Bus      *events.Bus
	Tokens   *auth.Tokens
	Auth     *auth.Service
	Groups   *groups.Service
	Subjects *subjects.Service
	Tags     *tags.Service
	Sync     *appsync.Service
}

// Build connects to Postgres and wires every service.
func Build(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Services, error) {
	pool, err := db.NewPool(ctx, cfg.DB.URL, cfg.DB.MaxConns)
	if err != nil {
		return nil, err
	}
	return BuildWithPool(pool, cfg, log), nil
}

// BuildWithPool wires services on an existing pool (tests).
func BuildWithPool(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) *Services {
	store := postgres.NewStore(pool)
	clk := clock.Real{}
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

	return &Services{
		Pool:     pool,
		Store:    store,
		Bus:      bus,
		Tokens:   tokens,
		Auth:     authSvc,
		Groups:   groupsSvc,
		Subjects: subjects.NewService(store.Subjects(), store.Tags(), acc, publisher, store),
		Tags:     tags.NewService(store.Tags(), store.Subjects(), acc, publisher, store),
		Sync:     appsync.NewService(store.Events(), acc),
	}
}

// Close releases resources.
func (s *Services) Close() {
	if s != nil && s.Pool != nil {
		s.Pool.Close()
	}
}

// HTTPServer builds the HTTP transport over the services.
func (s *Services) HTTPServer(cfg *config.Config, log *slog.Logger) *httptransport.Server {
	return httptransport.NewServer(httptransport.Deps{
		Cfg:      cfg,
		Log:      log,
		Tokens:   s.Tokens,
		Auth:     s.Auth,
		Groups:   s.Groups,
		Subjects: s.Subjects,
		Tags:     s.Tags,
		Sync:     s.Sync,
		Health: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			return s.Pool.Ping(pingCtx)
		},
	})
}
