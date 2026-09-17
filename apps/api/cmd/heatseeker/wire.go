package main

import (
	"context"
	"fmt"
	"log/slog"

	"heatseeker/api/internal/bootstrap"
	"heatseeker/api/internal/jobs"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/db"
	"heatseeker/api/internal/platform/redisx"
)

func runAPI(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	svc, err := bootstrap.Build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer svc.Close()
	return svc.HTTPServer(cfg, log).ListenAndServe(ctx, cfg.App.Port, log)
}

func runWorker(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	svc, err := bootstrap.Build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer svc.Close()

	opt, err := redisx.AsynqOpt(cfg.Redis.URL)
	if err != nil {
		return err
	}
	rdb, err := redisx.NewClient(ctx, cfg.Redis.URL)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	server, err := jobs.NewServer(opt, cfg.Jobs, svc.JobDeps(cfg, log))
	if err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	sched, err := jobs.NewScheduler(opt, log)
	if err != nil {
		return err
	}
	if err := sched.Start(); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	log.Info("worker started", "concurrency", cfg.Jobs.Concurrency, "queues", cfg.Jobs.Queues)
	<-ctx.Done()
	sched.Shutdown()
	server.Shutdown()
	return nil
}

func runMigrate(ctx context.Context, cfg *config.Config, log *slog.Logger, args []string) error {
	command := "up"
	if len(args) > 0 {
		command = args[0]
	}
	return db.Migrate(ctx, cfg.DB.URL, command, log)
}
