package tasks

import (
	"context"
	"time"

	"heatseeker/api/internal/domain"
)

// ScanDeadlines announces tasks whose deadline is approaching, once per
// configured offset. Until notifications land (stage 2), the announcement is a
// group event: it reaches clients through the sync log and the activity feed
// (D38). It returns how many tasks were announced.
func (s *Service) ScanDeadlines(ctx context.Context) (int, error) {
	if len(s.cfg.DeadlineOffsets) == 0 {
		return 0, nil
	}
	now := s.Clock.Now()
	grace := int32(s.cfg.ReminderGrace / time.Minute)
	due, err := s.Tasks.ListDueReminders(ctx, now, s.cfg.DeadlineOffsets, grace, s.cfg.ScanLimit)
	if err != nil {
		return 0, err
	}
	announced := 0
	for i := range due {
		d := due[i]
		if err := s.announce(ctx, d, now); err != nil {
			// One broken task must not stop the rest of the scan.
			s.Log.Error("announce task deadline", "task", d.Task.ID, "err", err)
			continue
		}
		announced++
	}
	return announced, nil
}

// announce emits one reminder for a task and marks every offset that fired, so
// a task created shortly before its deadline is announced once, not five times.
func (s *Service) announce(ctx context.Context, d domain.DueTask, now time.Time) error {
	nearest := d.Offsets[0]
	for _, o := range d.Offsets {
		if o < nearest {
			nearest = o
		}
	}
	kind := domain.EventTaskDueSoon
	if d.Task.DueAt != nil && !d.Task.DueAt.After(now) {
		kind = domain.EventTaskOverdue
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.emit(ctx, &d.Task, kind, nil, map[string]any{"offset_minutes": nearest}); err != nil {
			return err
		}
		return s.Tasks.MarkNotified(ctx, d.Task.ID, d.Offsets)
	})
}
