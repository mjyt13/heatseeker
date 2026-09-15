// Package db manages the PostgreSQL connection pool and schema migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
	"github.com/pressly/goose/v3"

	migrations "heatseeker/api/db"
)

// NewPool opens a pgx pool and verifies connectivity.
func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.MaxConnLifetime = time.Hour
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate runs goose with the embedded migrations. Supported commands:
// up, down, status, version, redo.
func Migrate(ctx context.Context, dsn, command string, log *slog.Logger) error {
	goose.SetBaseFS(migrations.Migrations)
	goose.SetLogger(gooseLogger{log})
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = conn.Close() }()

	switch command {
	case "up", "":
		return goose.UpContext(ctx, conn, "migrations")
	case "down":
		return goose.DownContext(ctx, conn, "migrations")
	case "redo":
		return goose.RedoContext(ctx, conn, "migrations")
	case "status":
		return goose.StatusContext(ctx, conn, "migrations")
	case "version":
		return goose.VersionContext(ctx, conn, "migrations")
	default:
		return fmt.Errorf("unknown migrate command %q (use up|down|redo|status|version)", command)
	}
}

type gooseLogger struct{ log *slog.Logger }

func (g gooseLogger) Fatalf(format string, v ...any) {
	g.log.Error(fmt.Sprintf(format, v...))
	panic(fmt.Sprintf(format, v...))
}

func (g gooseLogger) Printf(format string, v ...any) {
	g.log.Info("goose: " + trimNewline(fmt.Sprintf(format, v...)))
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
