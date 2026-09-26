// Package notify turns the group event log into notifications: one reader
// walks each group's log from its cursor, decides who should hear about an
// event and writes one row per recipient (docs/PLAN.md §8). Delivery to a
// device happens afterwards and never blocks the in-app copy.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
)

// Deps are the collaborators the service needs.
type Deps struct {
	Notify domain.NotifyRepo
	Events domain.EventRepo
	Groups domain.GroupRepo
	Access *access.Service
	Queue  domain.JobQueue
	Push   domain.Pusher
	Tx     domain.TxManager
	Clock  clock.Clock
	Log    *slog.Logger
}

// Settings tune the reader.
type Settings struct {
	// Delay is how long an event waits before it is turned into
	// notifications, so somebody who is already reading the discussion is
	// left alone and a burst of uploads arrives as one line.
	Delay time.Duration
	// BatchWindow bounds how many events one pass takes.
	BatchLimit int32
	// MaterialBatchMin is how many new materials collapse into one line.
	MaterialBatchMin int
	// DPOSilent starts DPO groups with everything but announcements off.
	DPOSilent bool
	// QuietFrom and QuietTo are the quiet hours a member gets before touching
	// the setting: minutes from midnight in their own timezone.
	QuietFrom *int16
	QuietTo   *int16
	// Retention is how long a read notification is kept.
	Retention time.Duration
}

// Service reads the log and serves the member's own notifications.
type Service struct {
	Deps
	set Settings
}

// NewService wires the service.
func NewService(d Deps, s Settings) *Service {
	if s.BatchLimit <= 0 {
		s.BatchLimit = 500
	}
	if s.MaterialBatchMin <= 0 {
		s.MaterialBatchMin = 3
	}
	return &Service{Deps: d, set: s}
}

// jobGrace keeps the delayed fan-out from arriving a hair too early.
const jobGrace = time.Second

// notifyingKinds are the event kinds the reader can turn into a notification.
// Anything else only moves the cursor.
var notifyingKinds = map[domain.EventKind]bool{
	domain.EventMessageCreated:    true,
	domain.EventMaterialAdded:     true,
	domain.EventTaskCreated:       true,
	domain.EventTaskPinned:        true,
	domain.EventTaskDueSoon:       true,
	domain.EventTaskOverdue:       true,
	domain.EventTaskStatusChanged: true,
	domain.EventScheduleCreated:   true,
	domain.EventScheduleUpdated:   true,
	domain.EventScheduleDeleted:   true,
	domain.EventScheduleCancelled: true,
	domain.EventScheduleChanged:   true,
	domain.EventScheduleReset:     true,
	domain.EventMemberJoined:      true,
}

// OnEvent is the bus subscriber: it asks for a pass over the group's log a
// little later, so a burst of events is handled once.
func (s *Service) OnEvent(e domain.Event) {
	if !notifyingKinds[e.Kind] || s.Queue == nil {
		return
	}
	job := domain.Job{
		Type:     domain.JobNotifyFanout,
		Payload:  domain.GroupPayload{GroupID: e.GroupID.String()},
		Queue:    "default",
		MaxRetry: 3,
		// A second of grace: the job must find the event already older than
		// the pause, otherwise it leaves it for the next minute's scan.
		Delay:     s.set.Delay + jobGrace,
		UniqueFor: s.set.Delay + time.Minute,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Queue.Enqueue(ctx, job); err != nil {
		s.Log.Error("schedule notification fan-out", "group", e.GroupID, "err", err)
	}
}

// ScanPending asks for a pass over every group whose log has moved on. It is
// the safety net for events whose fan-out job was lost.
func (s *Service) ScanPending(ctx context.Context) (int, error) {
	groups, err := s.Notify.GroupsWithPending(ctx, s.set.BatchLimit)
	if err != nil {
		return 0, err
	}
	for _, id := range groups {
		job := domain.Job{
			Type: domain.JobNotifyFanout, Payload: domain.GroupPayload{GroupID: id.String()},
			Queue: "default", MaxRetry: 3, UniqueFor: time.Minute,
		}
		if err := s.Queue.Enqueue(ctx, job); err != nil {
			return 0, err
		}
	}
	return len(groups), nil
}

// Fanout walks one group's log from the cursor and writes notifications.
// It is idempotent: a repeated pass writes nothing new.
func (s *Service) Fanout(ctx context.Context, groupID uuid.UUID) (int, error) {
	group, err := s.Groups.Get(ctx, groupID)
	if err != nil {
		return 0, err
	}
	cursor, known, err := s.Notify.Cursor(ctx, groupID)
	if err != nil {
		return 0, err
	}
	if !known {
		// First pass over a group that existed before notifications did:
		// start at the head instead of replaying its whole history.
		return 0, s.Notify.SetCursor(ctx, groupID, group.LastSeq)
	}
	events, err := s.Events.ListSince(ctx, groupID, cursor, s.set.BatchLimit)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	// An event younger than the delay waits: its own job will come.
	ready := events
	cutoff := s.Clock.Now().Add(-s.set.Delay)
	for i, e := range events {
		if e.CreatedAt.After(cutoff) {
			ready = events[:i]
			break
		}
	}
	if len(ready) == 0 {
		return 0, nil
	}
	notices, err := s.plan(ctx, group, ready)
	if err != nil {
		return 0, err
	}
	written := make([]uuid.UUID, 0, len(notices))
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		for _, n := range notices {
			ids, err := s.store(ctx, group, n)
			if err != nil {
				return err
			}
			written = append(written, ids...)
		}
		return s.Notify.SetCursor(ctx, groupID, ready[len(ready)-1].Seq)
	})
	if err != nil {
		return 0, err
	}
	if len(written) > 0 && s.Queue != nil {
		job := domain.Job{
			Type: domain.JobNotifyPush, Payload: domain.NotifyPushPayload{IDs: idStrings(written)},
			Queue: "default", MaxRetry: 3,
		}
		if err := s.Queue.Enqueue(ctx, job); err != nil {
			s.Log.Error("schedule push", "group", groupID, "err", err)
		}
	}
	return len(written), nil
}

// store writes one notice for every recipient it still applies to.
func (s *Service) store(ctx context.Context, group *domain.Group, n notice) ([]uuid.UUID, error) {
	data, err := json.Marshal(n.Data)
	if err != nil {
		return nil, fmt.Errorf("notification data: %w", err)
	}
	out := make([]uuid.UUID, 0, len(n.Recipients))
	for _, r := range n.Recipients {
		title, body := n.Render(location(r.Timezone))
		saved, err := s.Notify.Insert(ctx, domain.Notification{
			ID: ids.New(), UserID: r.UserID, GroupID: group.ID, Type: n.Type,
			Title: title, Body: body, Data: data, Seq: n.Seq, DedupeKey: n.DedupeKey,
		})
		if err != nil {
			return nil, err
		}
		if saved != nil {
			out = append(out, saved.ID)
		}
	}
	return out, nil
}

// location resolves a member's timezone, falling back to UTC.
func location(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
