// Package jobs runs background work on Redis via asynq: periodic maintenance
// now, Drive sync / media processing / notification delivery in later stages.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/config"
)

// Task type names. Namespaced by module.
const (
	TaskAuthCleanupRefreshTokens = "auth:cleanup_refresh_tokens"
	TaskEventsPrune              = "events:prune"
)

// Queue names in priority order.
const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

// Deps are the collaborators handlers need.
type Deps struct {
	Auth           domain.AuthRepo
	Events         domain.EventRepo
	EventRetention time.Duration
	Log            *slog.Logger
}

// ParseQueues turns "critical:6,default:3,low:1" into asynq's priority map.
func ParseQueues(spec string) (map[string]int, error) {
	out := map[string]int{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, weight, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("JOBS_QUEUES: %q must be name:weight", part)
		}
		w, err := strconv.Atoi(strings.TrimSpace(weight))
		if err != nil || w <= 0 {
			return nil, fmt.Errorf("JOBS_QUEUES: bad weight in %q", part)
		}
		out[strings.TrimSpace(name)] = w
	}
	if len(out) == 0 {
		out[QueueDefault] = 1
	}
	return out, nil
}

// NewServer builds the asynq worker with all task handlers registered.
func NewServer(opt asynq.RedisClientOpt, cfg config.Jobs, d Deps) (*asynq.Server, error) {
	queues, err := ParseQueues(cfg.Queues)
	if err != nil {
		return nil, err
	}
	srv := asynq.NewServer(opt, asynq.Config{
		Concurrency: cfg.Concurrency,
		Queues:      queues,
		Logger:      asynqLogger{d.Log},
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, task *asynq.Task, err error) {
			d.Log.Error("task failed", "type", task.Type(), "err", err)
		}),
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskAuthCleanupRefreshTokens, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Auth.DeleteExpiredRefreshTokens(ctx)
		if err != nil {
			return err
		}
		d.Log.Info("expired refresh tokens deleted", "count", n)
		return nil
	})
	mux.HandleFunc(TaskEventsPrune, func(ctx context.Context, _ *asynq.Task) error {
		cutoff := time.Now().UTC().Add(-d.EventRetention)
		n, err := d.Events.DeleteBefore(ctx, cutoff)
		if err != nil {
			return err
		}
		d.Log.Info("old group events pruned", "count", n, "before", cutoff)
		return nil
	})
	return srv, srv.Start(mux)
}

// NewScheduler registers periodic tasks (cron, UTC).
func NewScheduler(opt asynq.RedisClientOpt, log *slog.Logger) (*asynq.Scheduler, error) {
	sched := asynq.NewScheduler(opt, &asynq.SchedulerOpts{Location: time.UTC, Logger: asynqLogger{log}})
	entries := []struct {
		spec string
		task *asynq.Task
		opts []asynq.Option
	}{
		{"15 3 * * *", asynq.NewTask(TaskAuthCleanupRefreshTokens, nil), []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(2)}},
		{"45 3 * * *", asynq.NewTask(TaskEventsPrune, nil), []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(2)}},
	}
	for _, e := range entries {
		if _, err := sched.Register(e.spec, e.task, e.opts...); err != nil {
			return nil, fmt.Errorf("register %s: %w", e.task.Type(), err)
		}
	}
	return sched, nil
}

// asynqLogger adapts slog to asynq's logger interface.
type asynqLogger struct{ log *slog.Logger }

func (l asynqLogger) Debug(args ...any) { l.log.Debug(fmt.Sprint(args...)) }
func (l asynqLogger) Info(args ...any)  { l.log.Info(fmt.Sprint(args...)) }
func (l asynqLogger) Warn(args ...any)  { l.log.Warn(fmt.Sprint(args...)) }
func (l asynqLogger) Error(args ...any) { l.log.Error(fmt.Sprint(args...)) }
func (l asynqLogger) Fatal(args ...any) { l.log.Error(fmt.Sprint(args...)) }
