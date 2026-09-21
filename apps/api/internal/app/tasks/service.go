// Package tasks implements group to-dos: homework from a teacher, work the
// group agreed on and personal reminders, each with an optional deadline,
// a group status and every member's own progress (docs/PLAN.md §4.6, §8).
package tasks

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
)

// Settings come from configuration.
type Settings struct {
	// DeadlineOffsets are minutes before a deadline at which the group is
	// reminded, from the earliest to 0 (the deadline itself).
	DeadlineOffsets []int32
	// ReminderGrace bounds how long after a deadline it may still be announced.
	ReminderGrace time.Duration
	// DueSoonWindow is what the board counts as "due soon".
	DueSoonWindow time.Duration
	// ScanLimit bounds one run of the deadline scanner.
	ScanLimit int32
}

// Deps are the collaborators of the service.
type Deps struct {
	Tasks    domain.TaskRepo
	Subjects domain.SubjectRepo
	Members  domain.MembershipRepo
	Access   *access.Service
	Events   *events.Publisher
	Tx       domain.TxManager
	Clock    clock.Clock
	Log      *slog.Logger
}

// Service is the tasks use-case layer.
type Service struct {
	Deps
	cfg Settings
}

// NewService wires the service.
func NewService(d Deps, cfg Settings) *Service {
	if cfg.ScanLimit <= 0 {
		cfg.ScanLimit = 500
	}
	if cfg.DueSoonWindow <= 0 {
		cfg.DueSoonWindow = 7 * 24 * time.Hour
	}
	return &Service{Deps: d, cfg: cfg}
}

// Input is the task form. It fully replaces the stored values on update;
// nil pointers fall back to the defaults (create) or keep them (update).
type Input struct {
	// ClientID makes creation idempotent when the phone repeats the request.
	ClientID    *uuid.UUID
	SubjectID   *uuid.UUID
	Title       string
	Description string
	Kind        *domain.TaskKind
	Priority    *domain.TaskPriority
	AssignMode  *domain.TaskAssignMode
	Visibility  *domain.TaskVisibility
	DueAt       *time.Time
	// AssigneeIDs matter in the SELECTED mode; other modes ignore them.
	AssigneeIDs []uuid.UUID
	// MaterialIDs are the attached materials, replaced as a whole; on update
	// nil keeps them (the edit form does not touch attachments).
	MaterialIDs []uuid.UUID
}

// ListInput filters the task list.
type ListInput struct {
	SubjectID *uuid.UUID
	NoSubject bool
	Kind      *domain.TaskKind
	Statuses  []domain.TaskStatus
	Mine      bool
	Open      bool
	Overdue   bool
	DueBefore *time.Time
	Query     string
	Limit     int32
}

const (
	defaultLimit = 100
	maxLimit     = 200
)

// Create adds a task. Any member may create one (authz.TaskCreate).
func (s *Service) Create(ctx context.Context, actorID, groupID uuid.UUID, in Input) (*domain.TaskView, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.TaskCreate); err != nil {
		return nil, err
	}
	if in.ClientID != nil {
		switch existing, err := s.Tasks.GetByClientID(ctx, groupID, *in.ClientID); {
		case err == nil:
			return s.Tasks.GetView(ctx, existing.ID, actorID)
		case !errors.Is(err, domain.ErrNotFound):
			return nil, err
		}
	}
	title, err := normalizeTitle(in.Title)
	if err != nil {
		return nil, err
	}
	description, err := normalizeDescription(in.Description)
	if err != nil {
		return nil, err
	}
	kind := valueOr(in.Kind, domain.TaskGroup)
	priority := valueOr(in.Priority, domain.PriorityNormal)
	assignMode := valueOr(in.AssignMode, domain.AssignAll)
	visibility := valueOr(in.Visibility, domain.TaskVisibleGroup)
	if err := validateEnums(kind, priority, assignMode, visibility); err != nil {
		return nil, err
	}
	if err := s.checkSubject(ctx, groupID, in.SubjectID); err != nil {
		return nil, err
	}
	assignees, err := s.assignees(ctx, groupID, actorID, assignMode, in.AssigneeIDs)
	if err != nil {
		return nil, err
	}
	task := domain.Task{
		ID: ids.New(), GroupID: groupID, SubjectID: in.SubjectID, CreatedBy: &actorID, ClientID: in.ClientID,
		Title: title, Description: description, Kind: kind, Status: domain.TaskTodo, Priority: priority,
		AssignMode: assignMode, Visibility: visibility, DueAt: normalizeDue(in.DueAt),
	}
	var view *domain.TaskView
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		created, err := s.Tasks.Create(ctx, task)
		if err != nil {
			return err
		}
		if err := s.Tasks.SetAssignees(ctx, created.ID, assignees); err != nil {
			return err
		}
		if err := s.Tasks.SetAttachments(ctx, created.ID, groupID, in.MaterialIDs); err != nil {
			return err
		}
		if err := s.emit(ctx, created, domain.EventTaskCreated, &actorID, nil); err != nil {
			return err
		}
		view, err = s.Tasks.GetView(ctx, created.ID, actorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// List returns the group's tasks, pinned first, then by deadline.
func (s *Service) List(ctx context.Context, actorID, groupID uuid.UUID, in ListInput) ([]domain.TaskView, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	for _, st := range in.Statuses {
		if !slices.Contains(domain.AllTaskStatuses, st) {
			return nil, domain.Invalid("status", "unknown task status "+string(st))
		}
	}
	if in.Kind != nil && !slices.Contains(domain.AllTaskKinds, *in.Kind) {
		return nil, domain.Invalid("kind", "unknown task kind "+string(*in.Kind))
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return s.Tasks.List(ctx, domain.TaskFilter{
		GroupID: groupID, ActorID: actorID, SubjectID: in.SubjectID, NoSubject: in.NoSubject, Kind: in.Kind,
		Statuses: in.Statuses, Mine: in.Mine, Open: in.Open, Overdue: in.Overdue, DueBefore: in.DueBefore,
		Query: in.Query, Limit: limit,
	})
}

// Counts summarises the board: how many tasks sit in each status, how many
// are open, overdue, due soon and concern the actor.
func (s *Service) Counts(ctx context.Context, actorID, groupID uuid.UUID) (*domain.TaskCounts, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	return s.Tasks.Counts(ctx, groupID, actorID, now, now.Add(s.cfg.DueSoonWindow))
}

// Get returns one task with the actor's own progress.
func (s *Service) Get(ctx context.Context, actorID, taskID uuid.UUID) (*domain.TaskView, error) {
	view, _, err := s.load(ctx, actorID, taskID)
	return view, err
}

// load reads a task the actor may see, together with the actor.
func (s *Service) load(ctx context.Context, actorID, taskID uuid.UUID) (*domain.TaskView, *access.Actor, error) {
	task, err := s.Tasks.Get(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	actor, err := s.Access.Actor(ctx, actorID, task.GroupID)
	if err != nil {
		return nil, nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, nil, err
	}
	if task.Visibility == domain.TaskVisiblePrivate && (task.CreatedBy == nil || *task.CreatedBy != actorID) {
		return nil, nil, domain.NotFound("task")
	}
	view, err := s.Tasks.GetView(ctx, taskID, actorID)
	if err != nil {
		return nil, nil, err
	}
	return view, actor, nil
}

// canManage reports whether the actor may edit or delete the task: its author
// always may, and so do the roles that move the group status.
func canManage(actor *access.Actor, task *domain.Task, actorID uuid.UUID) bool {
	return task.ManagedBy(actorID, actor.Can(authz.TaskStatusGroup))
}

// Update replaces the editable fields of a task.
func (s *Service) Update(ctx context.Context, actorID, taskID uuid.UUID, in Input) (*domain.TaskView, error) {
	view, actor, err := s.load(ctx, actorID, taskID)
	if err != nil {
		return nil, err
	}
	current := view.Task
	if !canManage(actor, &current, actorID) {
		return nil, domain.Forbidden("only the author, the headman or a moderator can edit this task")
	}
	title, err := normalizeTitle(in.Title)
	if err != nil {
		return nil, err
	}
	description, err := normalizeDescription(in.Description)
	if err != nil {
		return nil, err
	}
	kind := valueOr(in.Kind, current.Kind)
	priority := valueOr(in.Priority, current.Priority)
	assignMode := valueOr(in.AssignMode, current.AssignMode)
	visibility := valueOr(in.Visibility, current.Visibility)
	if err := validateEnums(kind, priority, assignMode, visibility); err != nil {
		return nil, err
	}
	if err := s.checkSubject(ctx, current.GroupID, in.SubjectID); err != nil {
		return nil, err
	}
	assignees, err := s.assignees(ctx, current.GroupID, actorID, assignMode, in.AssigneeIDs)
	if err != nil {
		return nil, err
	}
	dueAt := normalizeDue(in.DueAt)
	var updated *domain.TaskView
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		task, err := s.Tasks.Update(ctx, domain.UpdateTaskParams{
			ID: taskID, SubjectID: in.SubjectID, Title: title, Description: description, Kind: kind,
			Priority: priority, AssignMode: assignMode, Visibility: visibility, DueAt: dueAt,
			// A moved deadline is announced again.
			ResetNotified: !equalTime(current.DueAt, dueAt),
		})
		if err != nil {
			return err
		}
		if err := s.Tasks.SetAssignees(ctx, taskID, assignees); err != nil {
			return err
		}
		if in.MaterialIDs != nil {
			if err := s.Tasks.SetAttachments(ctx, taskID, current.GroupID, in.MaterialIDs); err != nil {
				return err
			}
		}
		if err := s.emit(ctx, task, domain.EventTaskUpdated, &actorID, nil); err != nil {
			return err
		}
		updated, err = s.Tasks.GetView(ctx, taskID, actorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SetMaterials replaces the materials attached to a task. Whoever may edit
// the task may attach; deleted materials and other groups' ones are skipped.
func (s *Service) SetMaterials(ctx context.Context, actorID, taskID uuid.UUID, materialIDs []uuid.UUID) (*domain.TaskView, error) {
	view, actor, err := s.load(ctx, actorID, taskID)
	if err != nil {
		return nil, err
	}
	current := view.Task
	if !canManage(actor, &current, actorID) {
		return nil, domain.Forbidden("only the author, the headman or a moderator can attach files to this task")
	}
	if materialIDs == nil {
		materialIDs = []uuid.UUID{}
	}
	var updated *domain.TaskView
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.Tasks.SetAttachments(ctx, taskID, current.GroupID, materialIDs); err != nil {
			return err
		}
		if err := s.emit(ctx, &current, domain.EventTaskUpdated, &actorID, nil); err != nil {
			return err
		}
		updated, err = s.Tasks.GetView(ctx, taskID, actorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SetStatus moves the group status of a task (authz.TaskStatusGroup); the
// author may always move their own task.
func (s *Service) SetStatus(ctx context.Context, actorID, taskID uuid.UUID, status domain.TaskStatus) (*domain.TaskView, error) {
	view, actor, err := s.load(ctx, actorID, taskID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(domain.AllTaskStatuses, status) {
		return nil, domain.Invalid("status", "unknown task status "+string(status))
	}
	current := view.Task
	if !canManage(actor, &current, actorID) {
		return nil, domain.Forbidden("only the author, the headman or a moderator can change the group status")
	}
	if current.Status == status {
		return view, nil
	}
	var completedAt *time.Time
	if status.Closed() {
		now := s.Clock.Now()
		completedAt = &now
	}
	var updated *domain.TaskView
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		task, err := s.Tasks.SetStatus(ctx, taskID, status, completedAt)
		if err != nil {
			return err
		}
		if err := s.emit(ctx, task, domain.EventTaskStatusChanged, &actorID, map[string]any{
			"status": string(status), "was": string(current.Status),
		}); err != nil {
			return err
		}
		updated, err = s.Tasks.GetView(ctx, taskID, actorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SetMyStatus records the actor's own progress (authz.TaskStatusOwn). It does
// not touch the group status and produces no event: personal progress is not
// group news.
func (s *Service) SetMyStatus(ctx context.Context, actorID, taskID uuid.UUID, status domain.TaskStatus) (*domain.TaskView, error) {
	_, actor, err := s.load(ctx, actorID, taskID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.TaskStatusOwn); err != nil {
		return nil, err
	}
	if !slices.Contains(domain.AllTaskStatuses, status) {
		return nil, domain.Invalid("status", "unknown task status "+string(status))
	}
	var completedAt *time.Time
	if status.Closed() {
		now := s.Clock.Now()
		completedAt = &now
	}
	if _, err := s.Tasks.SetMyStatus(ctx, taskID, actorID, status, completedAt); err != nil {
		return nil, err
	}
	return s.Tasks.GetView(ctx, taskID, actorID)
}

// SetPinned pins a task to the top of the list (authz.TaskPin).
func (s *Service) SetPinned(ctx context.Context, actorID, taskID uuid.UUID, pinned bool) (*domain.TaskView, error) {
	view, actor, err := s.load(ctx, actorID, taskID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.TaskPin); err != nil {
		return nil, err
	}
	if view.Task.Visibility == domain.TaskVisiblePrivate {
		return nil, domain.Invalid("task", "a private task cannot be pinned for the group")
	}
	var by *uuid.UUID
	var at *time.Time
	kind := domain.EventTaskUnpinned
	if pinned {
		now := s.Clock.Now()
		by, at, kind = &actorID, &now, domain.EventTaskPinned
	}
	var updated *domain.TaskView
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		task, err := s.Tasks.SetPinned(ctx, taskID, by, at)
		if err != nil {
			return err
		}
		if err := s.emit(ctx, task, kind, &actorID, nil); err != nil {
			return err
		}
		updated, err = s.Tasks.GetView(ctx, taskID, actorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes a task (soft delete, author or moderator).
func (s *Service) Delete(ctx context.Context, actorID, taskID uuid.UUID) error {
	view, actor, err := s.load(ctx, actorID, taskID)
	if err != nil {
		return err
	}
	task := view.Task
	if !canManage(actor, &task, actorID) {
		return domain.Forbidden("only the author, the headman or a moderator can delete this task")
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.Tasks.SoftDelete(ctx, taskID, s.Clock.Now()); err != nil {
			return err
		}
		return s.emit(ctx, &task, domain.EventTaskDeleted, &actorID, nil)
	})
}

// checkSubject makes sure the subject belongs to the group.
func (s *Service) checkSubject(ctx context.Context, groupID uuid.UUID, subjectID *uuid.UUID) error {
	if subjectID == nil {
		return nil
	}
	if _, err := s.Subjects.Get(ctx, *subjectID, groupID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Invalid("subject_id", "no such subject in this group")
		}
		return err
	}
	return nil
}

// assignees resolves who the task is handed to: nobody in particular (ALL),
// the listed active members (SELECTED) or the author alone (SELF).
func (s *Service) assignees(ctx context.Context, groupID, actorID uuid.UUID, mode domain.TaskAssignMode, chosen []uuid.UUID) ([]uuid.UUID, error) {
	switch mode {
	case domain.AssignAll:
		return nil, nil
	case domain.AssignSelf:
		return []uuid.UUID{actorID}, nil
	}
	if len(chosen) == 0 {
		return nil, domain.Invalid("assignee_ids", "choose at least one member or hand the task to everybody")
	}
	members, err := s.Members.ListActiveUserIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(chosen))
	for _, id := range chosen {
		if !slices.Contains(members, id) {
			return nil, domain.Invalid("assignee_ids", "one of the chosen people is not an active member")
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *Service) emit(ctx context.Context, task *domain.Task, kind domain.EventKind, actor *uuid.UUID, extra map[string]any) error {
	payload := map[string]any{"title": task.Title, "status": string(task.Status)}
	if task.SubjectID != nil {
		payload["subject_id"] = task.SubjectID.String()
	}
	if task.DueAt != nil {
		payload["due_at"] = task.DueAt.UTC().Format(time.RFC3339)
	}
	for k, v := range extra {
		payload[k] = v
	}
	e, err := domain.NewEvent(task.GroupID, kind, actor, "task", &task.ID, payload)
	return s.Events.Emit(ctx, e, err)
}

func normalizeTitle(v string) (string, error) {
	v = strings.Join(strings.Fields(v), " ")
	if n := utf8.RuneCountInString(v); n < 1 || n > 200 {
		return "", domain.Invalid("title", "must be 1–200 characters")
	}
	return v, nil
}

func normalizeDescription(v string) (string, error) {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > 5000 {
		return "", domain.Invalid("description", "must be at most 5000 characters")
	}
	return v, nil
}

// normalizeDue keeps deadlines in UTC, truncated to a minute: the scanner
// works in minutes.
func normalizeDue(due *time.Time) *time.Time {
	if due == nil {
		return nil
	}
	v := due.UTC().Truncate(time.Minute)
	return &v
}

func validateEnums(kind domain.TaskKind, priority domain.TaskPriority, mode domain.TaskAssignMode, visibility domain.TaskVisibility) error {
	if !slices.Contains(domain.AllTaskKinds, kind) {
		return domain.Invalid("kind", "unknown task kind "+string(kind))
	}
	if !slices.Contains(domain.AllTaskPriorities, priority) {
		return domain.Invalid("priority", "unknown priority "+string(priority))
	}
	if !slices.Contains(domain.AllTaskAssignModes, mode) {
		return domain.Invalid("assign_mode", "unknown assign mode "+string(mode))
	}
	if !slices.Contains(domain.AllTaskVisibilities, visibility) {
		return domain.Invalid("visibility", "unknown visibility "+string(visibility))
	}
	return nil
}

func valueOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

func equalTime(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	}
	return a.Equal(*b)
}
