// Package queue implements domain.JobQueue on asynq (Redis) plus an
// in-memory recorder for tests.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/hibiken/asynq"

	"heatseeker/api/internal/domain"
)

// Asynq enqueues jobs into Redis.
type Asynq struct {
	client *asynq.Client
}

// NewAsynq wraps a client connection.
func NewAsynq(opt asynq.RedisConnOpt) *Asynq {
	return &Asynq{client: asynq.NewClient(opt)}
}

// Close releases the Redis connection.
func (q *Asynq) Close() error { return q.client.Close() }

// Enqueue implements domain.JobQueue. Duplicates within UniqueFor are
// silently dropped: the work is already scheduled.
func (q *Asynq) Enqueue(ctx context.Context, job domain.Job) error {
	task, opts, err := Task(job)
	if err != nil {
		return err
	}
	if _, err := q.client.EnqueueContext(ctx, task, opts...); err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) || errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("enqueue %s: %w", job.Type, err)
	}
	return nil
}

// Task converts a job into an asynq task with options.
func Task(job domain.Job) (*asynq.Task, []asynq.Option, error) {
	var payload []byte
	if job.Payload != nil {
		raw, err := json.Marshal(job.Payload)
		if err != nil {
			return nil, nil, fmt.Errorf("encode %s payload: %w", job.Type, err)
		}
		payload = raw
	}
	var opts []asynq.Option
	if job.Queue != "" {
		opts = append(opts, asynq.Queue(job.Queue))
	}
	if job.MaxRetry > 0 {
		opts = append(opts, asynq.MaxRetry(job.MaxRetry))
	}
	if job.UniqueFor > 0 {
		opts = append(opts, asynq.Unique(job.UniqueFor))
	}
	if job.Delay > 0 {
		opts = append(opts, asynq.ProcessIn(job.Delay))
	}
	return asynq.NewTask(job.Type, payload), opts, nil
}

// Recorder keeps enqueued jobs in memory (tests, `gen`).
type Recorder struct {
	mu   sync.Mutex
	jobs []domain.Job
}

// Enqueue implements domain.JobQueue.
func (r *Recorder) Enqueue(_ context.Context, job domain.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, job)
	return nil
}

// Drain returns and forgets the recorded jobs.
func (r *Recorder) Drain() []domain.Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.jobs
	r.jobs = nil
	return out
}
