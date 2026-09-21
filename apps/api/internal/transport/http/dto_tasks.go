package http

import (
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

// TaskDTO is a task as the lists and the card show it.
type TaskDTO struct {
	ID          uuid.UUID   `json:"id"`
	GroupID     uuid.UUID   `json:"group_id"`
	SubjectID   *uuid.UUID  `json:"subject_id,omitempty"`
	CreatedBy   *uuid.UUID  `json:"created_by,omitempty"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Kind        string      `json:"kind" enum:"TEACHER,GROUP,PERSONAL"`
	Status      string      `json:"status" enum:"TODO,IN_PROGRESS,IN_REVIEW,DONE,CANCELLED" doc:"Статус задачи для всей группы."`
	Priority    string      `json:"priority" enum:"LOW,NORMAL,HIGH"`
	AssignMode  string      `json:"assign_mode" enum:"ALL,SELECTED,SELF" doc:"Кому задача: всей группе, выбранным участникам или только автору."`
	Visibility  string      `json:"visibility" enum:"GROUP,PRIVATE"`
	DueAt       *time.Time  `json:"due_at,omitempty"`
	Overdue     bool        `json:"overdue" doc:"Срок прошёл, а задача открыта."`
	PinnedBy    *uuid.UUID  `json:"pinned_by,omitempty"`
	PinnedAt    *time.Time  `json:"pinned_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	MyStatus    string      `json:"my_status,omitempty" enum:"TODO,IN_PROGRESS,IN_REVIEW,DONE,CANCELLED" doc:"Мой личный прогресс; пусто — я к задаче не притрагивался."`
	AssigneeIDs []uuid.UUID `json:"assignee_ids"`
	MaterialIDs []uuid.UUID `json:"material_ids" doc:"Прикреплённые материалы."`
	// DoneCount and AssignedCount are 0 when the task is for everybody.
	DoneCount     int32 `json:"done_count"`
	AssignedCount int32 `json:"assigned_count"`
}

func toTaskDTO(v *domain.TaskView, now time.Time) TaskDTO {
	t := v.Task
	dto := TaskDTO{
		ID: t.ID, GroupID: t.GroupID, SubjectID: t.SubjectID, CreatedBy: t.CreatedBy, Title: t.Title,
		Description: t.Description, Kind: string(t.Kind), Status: string(t.Status), Priority: string(t.Priority),
		AssignMode: string(t.AssignMode), Visibility: string(t.Visibility), DueAt: t.DueAt, Overdue: t.Overdue(now),
		PinnedBy: t.PinnedBy, PinnedAt: t.PinnedAt, CompletedAt: t.CompletedAt, CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt, AssigneeIDs: nonNilIDs(v.AssigneeIDs), MaterialIDs: nonNilIDs(v.AttachmentIDs),
		DoneCount: v.DoneCount, AssignedCount: v.AssignedCount,
	}
	if v.MyStatus != nil {
		dto.MyStatus = string(*v.MyStatus)
	}
	return dto
}

// TaskCountsDTO is the board summary.
type TaskCountsDTO struct {
	ByStatus map[string]int32 `json:"by_status"`
	Open     int32            `json:"open"`
	Overdue  int32            `json:"overdue"`
	DueSoon  int32            `json:"due_soon" doc:"Открытые задачи со сроком в ближайшие TASK_DUE_SOON_DAYS дней."`
	Mine     int32            `json:"mine" doc:"Открытые задачи, которые касаются меня и не закрыты мной лично."`
}

func toTaskCountsDTO(c *domain.TaskCounts) TaskCountsDTO {
	byStatus := make(map[string]int32, len(c.ByStatus))
	for st, n := range c.ByStatus {
		byStatus[string(st)] = n
	}
	return TaskCountsDTO{ByStatus: byStatus, Open: c.Open, Overdue: c.Overdue, DueSoon: c.DueSoon, Mine: c.Mine}
}

func nonNilIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}
