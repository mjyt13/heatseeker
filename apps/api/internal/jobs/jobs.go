// Package jobs runs background work on Redis via asynq: periodic maintenance,
// Drive sync, publishing uploads to Drive and storage housekeeping.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"heatseeker/api/internal/app/drive"
	"heatseeker/api/internal/app/materials"
	"heatseeker/api/internal/app/notify"
	"heatseeker/api/internal/app/reminders"
	"heatseeker/api/internal/app/tasks"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/config"
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
	Drive          *drive.Service
	Materials      *materials.Service
	Tasks          *tasks.Service
	Notify         *notify.Service
	Reminders      *reminders.Service
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
	return srv, srv.Start(NewMux(d))
}

// NewMux registers every handler; exported for tests.
func NewMux(d Deps) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(domain.JobAuthCleanupRefreshTokens, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Auth.DeleteExpiredRefreshTokens(ctx)
		if err != nil {
			return err
		}
		d.Log.Info("expired refresh tokens deleted", "count", n)
		return nil
	})
	mux.HandleFunc(domain.JobEventsPrune, func(ctx context.Context, _ *asynq.Task) error {
		cutoff := time.Now().UTC().Add(-d.EventRetention)
		n, err := d.Events.DeleteBefore(ctx, cutoff)
		if err != nil {
			return err
		}
		d.Log.Info("old group events pruned", "count", n, "before", cutoff)
		return nil
	})
	mux.HandleFunc(domain.JobDriveSyncDue, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Drive.EnqueueDue(ctx)
		if n > 0 {
			d.Log.Debug("drive syncs scheduled", "count", n)
		}
		return err
	})
	mux.HandleFunc(domain.JobDriveSync, func(ctx context.Context, t *asynq.Task) error {
		var p domain.DriveSyncPayload
		id, err := decodeID(t, &p, func() string { return p.ConnectionID })
		if err != nil {
			return err
		}
		_, err = d.Drive.Sync(ctx, id, p.Full)
		return permanent(err)
	})
	mux.HandleFunc(domain.JobDriveUpload, func(ctx context.Context, t *asynq.Task) error {
		var p domain.MaterialVersionPayload
		versionID, err := decodeID(t, &p, func() string { return p.VersionID })
		if err != nil {
			return err
		}
		materialID, err := uuid.Parse(p.MaterialID)
		if err != nil {
			return fmt.Errorf("%s: bad material id: %w", t.Type(), asynq.SkipRetry)
		}
		err = d.Drive.UploadVersion(ctx, materialID, versionID)
		if errors.Is(err, drive.ErrPermanent) {
			d.Log.Warn("drive upload failed", "material", materialID, "err", err)
			return nil
		}
		return err
	})
	mux.HandleFunc(domain.JobMaterialHash, func(ctx context.Context, t *asynq.Task) error {
		var p domain.MaterialVersionPayload
		id, err := decodeID(t, &p, func() string { return p.VersionID })
		if err != nil {
			return err
		}
		return d.Materials.HashVersion(ctx, id)
	})
	mux.HandleFunc(domain.JobMaterialPreview, func(ctx context.Context, t *asynq.Task) error {
		var p domain.MaterialVersionPayload
		id, err := decodeID(t, &p, func() string { return p.VersionID })
		if err != nil {
			return err
		}
		err = d.Materials.BuildPreview(ctx, id)
		if err != nil && lastAttempt(ctx) {
			// Leave a final state instead of an endless PENDING.
			if ferr := d.Materials.PreviewGaveUp(context.WithoutCancel(ctx), id, err); ferr != nil {
				d.Log.Error("mark preview failed", "version", id, "err", ferr)
			}
		}
		return err
	})
	mux.HandleFunc(domain.JobMaterialsReclassify, func(ctx context.Context, t *asynq.Task) error {
		var p domain.GroupPayload
		id, err := decodeID(t, &p, func() string { return p.GroupID })
		if err != nil {
			return err
		}
		_, err = d.Drive.Reclassify(ctx, id)
		return err
	})
	mux.HandleFunc(domain.JobTasksDeadlineScan, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Tasks.ScanDeadlines(ctx)
		if n > 0 {
			d.Log.Info("task deadlines announced", "count", n)
		}
		return err
	})
	mux.HandleFunc(domain.JobNotifyScan, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Notify.ScanPending(ctx)
		if n > 0 {
			d.Log.Debug("notification fan-out scheduled", "groups", n)
		}
		return err
	})
	mux.HandleFunc(domain.JobNotifyFanout, func(ctx context.Context, t *asynq.Task) error {
		var p domain.GroupPayload
		id, err := decodeID(t, &p, func() string { return p.GroupID })
		if err != nil {
			return err
		}
		n, err := d.Notify.Fanout(ctx, id)
		if n > 0 {
			d.Log.Info("notifications written", "group", id, "count", n)
		}
		return permanent(err)
	})
	mux.HandleFunc(domain.JobNotifyPush, func(ctx context.Context, t *asynq.Task) error {
		var p domain.NotifyPushPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%s: bad payload: %v: %w", t.Type(), err, asynq.SkipRetry)
		}
		ids := make([]uuid.UUID, 0, len(p.IDs))
		for _, raw := range p.IDs {
			id, err := uuid.Parse(raw)
			if err != nil {
				return fmt.Errorf("%s: bad id: %w", t.Type(), asynq.SkipRetry)
			}
			ids = append(ids, id)
		}
		n, err := d.Notify.Deliver(ctx, ids)
		if n > 0 {
			d.Log.Debug("push sent", "count", n)
		}
		return err
	})
	mux.HandleFunc(domain.JobRemindersScan, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Reminders.Scan(ctx)
		if n > 0 {
			d.Log.Info("reminders delivered", "count", n)
		}
		return err
	})
	mux.HandleFunc(domain.JobNotifyCleanup, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Notify.Cleanup(ctx)
		if n > 0 {
			d.Log.Info("old notifications removed", "count", n)
		}
		return err
	})
	mux.HandleFunc(domain.JobUploadsCleanup, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Materials.CleanupUploads(ctx)
		if n > 0 {
			d.Log.Info("expired uploads removed", "count", n)
		}
		return err
	})
	mux.HandleFunc(domain.JobMaterialsPurge, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.Materials.PurgeDeleted(ctx)
		if n > 0 {
			d.Log.Info("deleted materials purged", "count", n)
		}
		return err
	})
	return mux
}

func decodeID(t *asynq.Task, payload any, field func() string) (uuid.UUID, error) {
	if err := json.Unmarshal(t.Payload(), payload); err != nil {
		return uuid.Nil, fmt.Errorf("%s: bad payload: %v: %w", t.Type(), err, asynq.SkipRetry)
	}
	id, err := uuid.Parse(field())
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s: bad id: %w", t.Type(), asynq.SkipRetry)
	}
	return id, nil
}

// permanent stops retries for errors a retry cannot fix.
func permanent(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrUnavailable) || errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	return err
}

// NewScheduler registers periodic tasks (cron, UTC).
func NewScheduler(opt asynq.RedisClientOpt, log *slog.Logger) (*asynq.Scheduler, error) {
	sched := asynq.NewScheduler(opt, &asynq.SchedulerOpts{Location: time.UTC, Logger: asynqLogger{log}})
	for _, e := range Schedule() {
		if _, err := sched.Register(e.Spec, e.Task, e.Opts...); err != nil {
			return nil, fmt.Errorf("register %s: %w", e.Task.Type(), err)
		}
	}
	return sched, nil
}

// Entry is one periodic task.
type Entry struct {
	Spec string
	Task *asynq.Task
	Opts []asynq.Option
}

// Schedule lists the periodic tasks.
func Schedule() []Entry {
	return []Entry{
		{"15 3 * * *", asynq.NewTask(domain.JobAuthCleanupRefreshTokens, nil), []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(2)}},
		{"45 3 * * *", asynq.NewTask(domain.JobEventsPrune, nil), []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(2)}},
		{"* * * * *", asynq.NewTask(domain.JobDriveSyncDue, nil), []asynq.Option{asynq.Queue(QueueDefault), asynq.MaxRetry(0), asynq.Unique(50 * time.Second)}},
		{"* * * * *", asynq.NewTask(domain.JobTasksDeadlineScan, nil), []asynq.Option{asynq.Queue(QueueDefault), asynq.MaxRetry(0), asynq.Unique(50 * time.Second)}},
		{"* * * * *", asynq.NewTask(domain.JobNotifyScan, nil), []asynq.Option{asynq.Queue(QueueDefault), asynq.MaxRetry(0), asynq.Unique(50 * time.Second)}},
		{"* * * * *", asynq.NewTask(domain.JobRemindersScan, nil), []asynq.Option{asynq.Queue(QueueDefault), asynq.MaxRetry(0), asynq.Unique(50 * time.Second)}},
		{"50 4 * * *", asynq.NewTask(domain.JobNotifyCleanup, nil), []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(2)}},
		{"20 * * * *", asynq.NewTask(domain.JobUploadsCleanup, nil), []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(2)}},
		{"30 4 * * *", asynq.NewTask(domain.JobMaterialsPurge, nil), []asynq.Option{asynq.Queue(QueueLow), asynq.MaxRetry(2)}},
	}
}

// lastAttempt reports whether a failing task will not be retried again.
func lastAttempt(ctx context.Context) bool {
	retried, ok1 := asynq.GetRetryCount(ctx)
	maxRetry, ok2 := asynq.GetMaxRetry(ctx)
	return ok1 && ok2 && retried >= maxRetry
}

// asynqLogger adapts slog to asynq's logger interface.
type asynqLogger struct{ log *slog.Logger }

func (l asynqLogger) Debug(args ...any) { l.log.Debug(fmt.Sprint(args...)) }
func (l asynqLogger) Info(args ...any)  { l.log.Info(fmt.Sprint(args...)) }
func (l asynqLogger) Warn(args ...any)  { l.log.Warn(fmt.Sprint(args...)) }
func (l asynqLogger) Error(args ...any) { l.log.Error(fmt.Sprint(args...)) }
func (l asynqLogger) Fatal(args ...any) { l.log.Error(fmt.Sprint(args...)) }
