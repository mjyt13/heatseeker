// Package reminders keeps everybody's own notes to self: text, a moment, an
// optional repeat and a link to what it is about (docs/PLAN.md §8.2). The
// text is personal, so it never enters the group log — when the moment comes,
// the notification goes straight to its owner.
package reminders

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/app/notify"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
)

const (
	// MaxList bounds one page of reminders.
	MaxList = 200
	// MaxAhead is how far in the future a reminder may be set.
	MaxAhead = 5 * 365 * 24 * time.Hour
	// SnoozeMax bounds a single "remind me later".
	SnoozeMax = 30 * 24 * time.Hour
)

// Deps are the collaborators the service needs.
type Deps struct {
	Repo   domain.ReminderRepo
	Users  domain.UserRepo
	Access *access.Service
	Notify *notify.Service
	Tx     domain.TxManager
	Clock  clock.Clock
	Log    *slog.Logger
}

// Settings tune the scanner.
type Settings struct {
	// ScanLimit bounds one run of the scanner.
	ScanLimit int32
}

// Service is the reminders use case.
type Service struct {
	Deps
	set Settings
}

// NewService wires the service.
func NewService(d Deps, s Settings) *Service {
	if s.ScanLimit <= 0 {
		s.ScanLimit = 200
	}
	return &Service{Deps: d, set: s}
}

// Input is the reminder form.
type Input struct {
	Title      string
	Note       string
	RemindAt   time.Time
	Repeat     domain.ReminderRepeat
	TargetType *domain.ReminderTarget
	TargetID   *uuid.UUID
}

// List returns the member's own reminders in one group.
func (s *Service) List(ctx context.Context, actorID, groupID uuid.UUID, openOnly bool, limit int32) ([]domain.Reminder, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.ReminderManage); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > MaxList {
		limit = MaxList
	}
	return s.Repo.List(ctx, actorID, groupID, openOnly, limit)
}

// Create sets a reminder.
func (s *Service) Create(ctx context.Context, actorID, groupID uuid.UUID, in Input) (*domain.Reminder, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.ReminderManage); err != nil {
		return nil, err
	}
	r, err := s.build(in)
	if err != nil {
		return nil, err
	}
	r.ID, r.GroupID, r.UserID = ids.New(), groupID, actorID
	r.Status = domain.ReminderScheduled
	return s.Repo.Create(ctx, r)
}

// Update edits a reminder and puts it back on the schedule.
func (s *Service) Update(ctx context.Context, actorID, id uuid.UUID, in Input) (*domain.Reminder, error) {
	current, err := s.own(ctx, actorID, id)
	if err != nil {
		return nil, err
	}
	next, err := s.build(in)
	if err != nil {
		return nil, err
	}
	current.Title, current.Note, current.RemindAt = next.Title, next.Note, next.RemindAt
	current.Repeat, current.TargetType, current.TargetID = next.Repeat, next.TargetType, next.TargetID
	current.Status = domain.ReminderScheduled
	return s.Repo.Update(ctx, *current)
}

// Snooze moves a reminder later: "remind me in an hour".
func (s *Service) Snooze(ctx context.Context, actorID, id uuid.UUID, d time.Duration) (*domain.Reminder, error) {
	current, err := s.own(ctx, actorID, id)
	if err != nil {
		return nil, err
	}
	if d <= 0 || d > SnoozeMax {
		return nil, domain.Invalid("minutes", "must be between 1 minute and 30 days")
	}
	current.RemindAt = s.Clock.Now().Add(d).UTC()
	current.Status = domain.ReminderScheduled
	return s.Repo.Update(ctx, *current)
}

// Done closes a reminder. A repeating one is closed for good: its next round
// is no longer scheduled.
func (s *Service) Done(ctx context.Context, actorID, id uuid.UUID, done bool) (*domain.Reminder, error) {
	current, err := s.own(ctx, actorID, id)
	if err != nil {
		return nil, err
	}
	current.Status = domain.ReminderDone
	if !done {
		current.Status = domain.ReminderScheduled
	}
	return s.Repo.Update(ctx, *current)
}

// Delete removes a reminder.
func (s *Service) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	if _, err := s.own(ctx, actorID, id); err != nil {
		return err
	}
	return s.Repo.Delete(ctx, id, actorID)
}

// Scan delivers the reminders whose moment has come and moves repeating ones
// to their next round. It runs every minute.
func (s *Service) Scan(ctx context.Context) (int, error) {
	now := s.Clock.Now()
	due, err := s.Repo.Due(ctx, now, s.set.ScanLimit)
	if err != nil {
		return 0, err
	}
	sent := 0
	for i := range due {
		if err := s.fire(ctx, due[i], now); err != nil {
			s.Log.Error("deliver reminder", "reminder", due[i].ID, "err", err)
			continue
		}
		sent++
	}
	return sent, nil
}

// fire notifies the owner and decides what happens to the reminder next.
func (s *Service) fire(ctx context.Context, r domain.Reminder, now time.Time) error {
	data := map[string]any{"screen": "reminders", "reminder_id": r.ID}
	if r.TargetType != nil && r.TargetID != nil {
		switch *r.TargetType {
		case domain.ReminderTask:
			data["screen"], data["task_id"] = "task", *r.TargetID
		case domain.ReminderMaterial:
			data["screen"], data["material_id"] = "material", *r.TargetID
		case domain.ReminderSchedule:
			data["screen"], data["event_id"] = "schedule", *r.TargetID
		}
	}
	raw, err := notify.Data(data)
	if err != nil {
		return err
	}
	fired := r.RemindAt
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.Notify.Send(ctx, domain.Notification{
			UserID: r.UserID, GroupID: r.GroupID, Type: domain.NotifyReminder,
			Title: "Напоминание", Body: r.Title, Data: raw,
			DedupeKey: "REMINDER:" + r.ID.String() + ":" + fired.UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		r.LastFired = &fired
		r.Status = domain.ReminderSent
		if next, ok := r.Repeat.Next(fired, s.location(ctx, r.UserID)); ok {
			// Skip rounds missed while the worker was down.
			for !next.After(now) {
				next, _ = r.Repeat.Next(next, s.location(ctx, r.UserID))
			}
			r.RemindAt, r.Status = next.UTC(), domain.ReminderScheduled
		}
		_, err := s.Repo.Update(ctx, r)
		return err
	})
}

// location is the owner's timezone: a daily reminder keeps its wall clock.
func (s *Service) location(ctx context.Context, userID uuid.UUID) *time.Location {
	u, err := s.Users.GetByID(ctx, userID)
	if err != nil || u.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// own loads a reminder and refuses anybody else's.
func (s *Service) own(ctx context.Context, actorID, id uuid.UUID) (*domain.Reminder, error) {
	r, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.UserID != actorID {
		// Somebody else's reminder is not theirs to know about.
		return nil, domain.NotFound("reminder")
	}
	if _, err := s.Access.Actor(ctx, actorID, r.GroupID); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Service) build(in Input) (domain.Reminder, error) {
	title := strings.Join(strings.Fields(in.Title), " ")
	if n := utf8.RuneCountInString(title); n < 1 || n > 200 {
		return domain.Reminder{}, domain.Invalid("title", "must be 1–200 characters")
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > 2000 {
		return domain.Reminder{}, domain.Invalid("note", "must be at most 2000 characters")
	}
	if in.RemindAt.IsZero() {
		return domain.Reminder{}, domain.Invalid("remind_at", "say when to remind")
	}
	if in.RemindAt.After(s.Clock.Now().Add(MaxAhead)) {
		return domain.Reminder{}, domain.Invalid("remind_at", "must be within five years")
	}
	repeat := in.Repeat
	if repeat == "" {
		repeat = domain.RepeatNone
	}
	if !repeat.Valid() {
		return domain.Reminder{}, domain.Invalid("repeat", "unknown repeat "+string(repeat))
	}
	if (in.TargetType == nil) != (in.TargetID == nil) {
		return domain.Reminder{}, domain.Invalid("target_id", "name both the kind and the id, or neither")
	}
	if in.TargetType != nil && !in.TargetType.Valid() {
		return domain.Reminder{}, domain.Invalid("target_type", "unknown target "+string(*in.TargetType))
	}
	return domain.Reminder{
		Title: title, Note: note, RemindAt: in.RemindAt.UTC(), Repeat: repeat,
		TargetType: in.TargetType, TargetID: in.TargetID,
	}, nil
}
