package domain

import (
	"time"

	"github.com/google/uuid"
)

// TaskKind tells where a task came from: set by a teacher, agreed by the
// group, or a personal to-do of its author.
type TaskKind string

// Task kinds.
const (
	TaskTeacher  TaskKind = "TEACHER"
	TaskGroup    TaskKind = "GROUP"
	TaskPersonal TaskKind = "PERSONAL"
)

// AllTaskKinds lists kinds in display order.
var AllTaskKinds = []TaskKind{TaskTeacher, TaskGroup, TaskPersonal}

// TaskStatus is the state of a task, shared by the group status and by every
// member's own progress.
type TaskStatus string

// Task statuses.
const (
	TaskTodo       TaskStatus = "TODO"
	TaskInProgress TaskStatus = "IN_PROGRESS"
	TaskInReview   TaskStatus = "IN_REVIEW"
	TaskDone       TaskStatus = "DONE"
	TaskCancelled  TaskStatus = "CANCELLED"
)

// AllTaskStatuses lists statuses in board order.
var AllTaskStatuses = []TaskStatus{TaskTodo, TaskInProgress, TaskInReview, TaskDone, TaskCancelled}

// Closed reports whether the status needs no more work.
func (s TaskStatus) Closed() bool { return s == TaskDone || s == TaskCancelled }

// TaskPriority orders tasks inside a column.
type TaskPriority string

// Task priorities.
const (
	PriorityLow    TaskPriority = "LOW"
	PriorityNormal TaskPriority = "NORMAL"
	PriorityHigh   TaskPriority = "HIGH"
)

// AllTaskPriorities lists priorities from low to high.
var AllTaskPriorities = []TaskPriority{PriorityLow, PriorityNormal, PriorityHigh}

// TaskAssignMode says who the task is for: everyone in the group, the listed
// members, or only its author.
type TaskAssignMode string

// Assign modes.
const (
	AssignAll      TaskAssignMode = "ALL"
	AssignSelected TaskAssignMode = "SELECTED"
	AssignSelf     TaskAssignMode = "SELF"
)

// AllTaskAssignModes lists the modes.
var AllTaskAssignModes = []TaskAssignMode{AssignAll, AssignSelected, AssignSelf}

// TaskVisibility hides a personal task from the rest of the group.
type TaskVisibility string

// Task visibilities.
const (
	TaskVisibleGroup   TaskVisibility = "GROUP"
	TaskVisiblePrivate TaskVisibility = "PRIVATE"
)

// AllTaskVisibilities lists the values.
var AllTaskVisibilities = []TaskVisibility{TaskVisibleGroup, TaskVisiblePrivate}

// Task is a piece of work with an optional deadline.
type Task struct {
	ID          uuid.UUID
	GroupID     uuid.UUID
	SubjectID   *uuid.UUID
	CreatedBy   *uuid.UUID
	ClientID    *uuid.UUID
	Title       string
	Description string
	Kind        TaskKind
	Status      TaskStatus
	Priority    TaskPriority
	AssignMode  TaskAssignMode
	Visibility  TaskVisibility
	DueAt       *time.Time
	// NotifiedOffsets are the deadline reminders (minutes before DueAt) the
	// scanner has already announced.
	NotifiedOffsets []int32
	PinnedBy        *uuid.UUID
	PinnedAt        *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// Overdue reports whether the deadline has passed with the task still open.
func (t *Task) Overdue(now time.Time) bool {
	return t.DueAt != nil && !t.Status.Closed() && t.DueAt.Before(now)
}

// TaskAssignment is one member's own progress on a task.
type TaskAssignment struct {
	TaskID      uuid.UUID
	UserID      uuid.UUID
	Status      TaskStatus
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TaskView is a task with the numbers the lists show.
type TaskView struct {
	Task Task
	// MyStatus is the actor's own progress, nil when they never touched it.
	MyStatus      *TaskStatus
	AssigneeIDs   []uuid.UUID
	AttachmentIDs []uuid.UUID
	// DoneCount counts members who finished; AssignedCount is how many the
	// task was handed to (0 for AssignAll: everybody).
	DoneCount     int32
	AssignedCount int32
}

// TaskFilter selects tasks for the list and the board.
type TaskFilter struct {
	GroupID uuid.UUID
	// ActorID is whose own progress and private tasks are included.
	ActorID   uuid.UUID
	SubjectID *uuid.UUID
	NoSubject bool
	Kind      *TaskKind
	Statuses  []TaskStatus
	// Mine keeps tasks the actor created, was assigned or already started.
	Mine bool
	// Open drops tasks whose group status is closed.
	Open bool
	// Overdue keeps open tasks whose deadline has passed.
	Overdue bool
	// DueBefore keeps tasks with a deadline no later than this.
	DueBefore *time.Time
	Query     string
	Limit     int32
}

// TaskCounts summarises the group's board.
type TaskCounts struct {
	ByStatus map[TaskStatus]int32
	// Open, Overdue and DueSoon count the group's open tasks; Mine counts the
	// open ones that concern the actor.
	Open    int32
	Overdue int32
	DueSoon int32
	Mine    int32
}

// DueTask is a task whose deadline reminder is due, with the offsets (minutes
// before the deadline) that fired.
type DueTask struct {
	Task    Task
	Offsets []int32
}

// ManagedBy reports whether a user may edit the task, change its group status
// and attach files: its author always may, and so may those whose role moves
// group statuses (groupStatusRight).
func (t *Task) ManagedBy(userID uuid.UUID, groupStatusRight bool) bool {
	if t.CreatedBy != nil && *t.CreatedBy == userID {
		return true
	}
	return groupStatusRight
}

// VisibleTo reports whether a member sees the task: private tasks belong to
// their author, deleted ones to nobody.
func (t *Task) VisibleTo(userID uuid.UUID) bool {
	if t.DeletedAt != nil {
		return false
	}
	return t.Visibility == TaskVisibleGroup || (t.CreatedBy != nil && *t.CreatedBy == userID)
}
