package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type taskRepo struct{ s *Store }

func toTask(t sqlcgen.Task) *domain.Task {
	offsets := t.NotifiedOffsets
	if offsets == nil {
		offsets = []int32{}
	}
	return &domain.Task{
		ID:              t.ID,
		GroupID:         t.GroupID,
		SubjectID:       t.SubjectID,
		CreatedBy:       t.CreatedBy,
		ClientID:        t.ClientID,
		Title:           t.Title,
		Description:     t.Description,
		Kind:            domain.TaskKind(t.Kind),
		Status:          domain.TaskStatus(t.Status),
		Priority:        domain.TaskPriority(t.Priority),
		AssignMode:      domain.TaskAssignMode(t.AssignMode),
		Visibility:      domain.TaskVisibility(t.Visibility),
		DueAt:           t.DueAt,
		NotifiedOffsets: offsets,
		PinnedBy:        t.PinnedBy,
		PinnedAt:        t.PinnedAt,
		CompletedAt:     t.CompletedAt,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
		DeletedAt:       t.DeletedAt,
	}
}

func toAssignment(a sqlcgen.TaskAssignment) domain.TaskAssignment {
	return domain.TaskAssignment{
		TaskID:      a.TaskID,
		UserID:      a.UserID,
		Status:      domain.TaskStatus(a.Status),
		CompletedAt: a.CompletedAt,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}

// myStatus turns the empty marker of the view queries into "not touched".
func myStatus(raw string) *domain.TaskStatus {
	if raw == "" {
		return nil
	}
	st := domain.TaskStatus(raw)
	return &st
}

func (r *taskRepo) Create(ctx context.Context, t domain.Task) (*domain.Task, error) {
	row, err := r.s.queries(ctx).CreateTask(ctx, sqlcgen.CreateTaskParams{
		ID: t.ID, GroupID: t.GroupID, SubjectID: t.SubjectID, CreatedBy: t.CreatedBy, ClientID: t.ClientID,
		Title: t.Title, Description: t.Description, Kind: string(t.Kind), Status: string(t.Status),
		Priority: string(t.Priority), AssignMode: string(t.AssignMode), Visibility: string(t.Visibility), DueAt: t.DueAt,
	})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return toTask(row), nil
}

func (r *taskRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	row, err := r.s.queries(ctx).GetTask(ctx, id)
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return toTask(row), nil
}

func (r *taskRepo) GetByClientID(ctx context.Context, groupID, clientID uuid.UUID) (*domain.Task, error) {
	row, err := r.s.queries(ctx).GetTaskByClientID(ctx, sqlcgen.GetTaskByClientIDParams{GroupID: groupID, ClientID: &clientID})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return toTask(row), nil
}

func (r *taskRepo) GetView(ctx context.Context, id, actorID uuid.UUID) (*domain.TaskView, error) {
	row, err := r.s.queries(ctx).GetTaskView(ctx, sqlcgen.GetTaskViewParams{ID: id, ActorID: actorID})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return &domain.TaskView{
		Task: *toTask(row.Task), MyStatus: myStatus(row.MyStatus), AssigneeIDs: row.AssigneeIds,
		AttachmentIDs: row.AttachmentIds, DoneCount: row.DoneCount, AssignedCount: row.AssignedCount,
	}, nil
}

func (r *taskRepo) List(ctx context.Context, f domain.TaskFilter) ([]domain.TaskView, error) {
	params := sqlcgen.ListTasksParams{
		ActorID: f.ActorID, GroupID: f.GroupID, SubjectID: f.SubjectID, NoSubject: f.NoSubject,
		Statuses: make([]string, 0, len(f.Statuses)), OpenOnly: f.Open, Overdue: f.Overdue,
		Now: time.Now().UTC(), DueBefore: f.DueBefore, Mine: f.Mine, MaxRows: f.Limit,
	}
	if f.Kind != nil {
		kind := string(*f.Kind)
		params.Kind = &kind
	}
	for _, st := range f.Statuses {
		params.Statuses = append(params.Statuses, string(st))
	}
	if query := strings.TrimSpace(f.Query); query != "" {
		pattern := likeEscaper.Replace(query)
		params.LikePattern = &pattern
	}
	rows, err := r.s.queries(ctx).ListTasks(ctx, params)
	if err != nil {
		return nil, mapErr(err, "task")
	}
	out := make([]domain.TaskView, len(rows))
	for i, row := range rows {
		out[i] = domain.TaskView{
			Task: *toTask(row.Task), MyStatus: myStatus(row.MyStatus), AssigneeIDs: row.AssigneeIds,
			AttachmentIDs: row.AttachmentIds, DoneCount: row.DoneCount, AssignedCount: row.AssignedCount,
		}
	}
	return out, nil
}

func (r *taskRepo) Counts(ctx context.Context, groupID, actorID uuid.UUID, now, dueSoonBefore time.Time) (*domain.TaskCounts, error) {
	row, err := r.s.queries(ctx).CountTasks(ctx, sqlcgen.CountTasksParams{
		GroupID: groupID, ActorID: &actorID, Now: now, DueSoonBefore: dueSoonBefore,
	})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return &domain.TaskCounts{
		ByStatus: map[domain.TaskStatus]int32{
			domain.TaskTodo:       row.TodoCount,
			domain.TaskInProgress: row.InProgressCount,
			domain.TaskInReview:   row.InReviewCount,
			domain.TaskDone:       row.DoneCount,
			domain.TaskCancelled:  row.CancelledCount,
		},
		Open: row.OpenCount, Overdue: row.OverdueCount, DueSoon: row.DueSoonCount, Mine: row.MineCount,
	}, nil
}

func (r *taskRepo) Update(ctx context.Context, p domain.UpdateTaskParams) (*domain.Task, error) {
	row, err := r.s.queries(ctx).UpdateTask(ctx, sqlcgen.UpdateTaskParams{
		ID: p.ID, SubjectID: p.SubjectID, Title: p.Title, Description: p.Description, Kind: string(p.Kind),
		Priority: string(p.Priority), AssignMode: string(p.AssignMode), Visibility: string(p.Visibility),
		DueAt: p.DueAt, ResetNotified: p.ResetNotified,
	})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return toTask(row), nil
}

func (r *taskRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.TaskStatus, completedAt *time.Time) (*domain.Task, error) {
	row, err := r.s.queries(ctx).SetTaskStatus(ctx, sqlcgen.SetTaskStatusParams{ID: id, Status: string(status), CompletedAt: completedAt})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return toTask(row), nil
}

func (r *taskRepo) SetPinned(ctx context.Context, id uuid.UUID, by *uuid.UUID, at *time.Time) (*domain.Task, error) {
	row, err := r.s.queries(ctx).SetTaskPinned(ctx, sqlcgen.SetTaskPinnedParams{ID: id, PinnedBy: by, PinnedAt: at})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return toTask(row), nil
}

func (r *taskRepo) SoftDelete(ctx context.Context, id uuid.UUID, at time.Time) error {
	return mapErr(r.s.queries(ctx).SoftDeleteTask(ctx, sqlcgen.SoftDeleteTaskParams{ID: id, DeletedAt: at}), "task")
}

func (r *taskRepo) SetAssignees(ctx context.Context, taskID uuid.UUID, userIDs []uuid.UUID) error {
	q := r.s.queries(ctx)
	if userIDs == nil {
		userIDs = []uuid.UUID{}
	}
	if err := q.DeleteTaskAssignments(ctx, sqlcgen.DeleteTaskAssignmentsParams{TaskID: taskID, UserIds: userIDs}); err != nil {
		return mapErr(err, "task assignment")
	}
	if len(userIDs) == 0 {
		return nil
	}
	return mapErr(q.InsertTaskAssignments(ctx, sqlcgen.InsertTaskAssignmentsParams{TaskID: taskID, UserIds: userIDs}), "task assignment")
}

func (r *taskRepo) ListAssignments(ctx context.Context, taskID uuid.UUID) ([]domain.TaskAssignment, error) {
	rows, err := r.s.queries(ctx).ListTaskAssignments(ctx, taskID)
	if err != nil {
		return nil, mapErr(err, "task assignment")
	}
	out := make([]domain.TaskAssignment, len(rows))
	for i, row := range rows {
		out[i] = toAssignment(row)
	}
	return out, nil
}

func (r *taskRepo) SetMyStatus(ctx context.Context, taskID, userID uuid.UUID, status domain.TaskStatus, completedAt *time.Time) (*domain.TaskAssignment, error) {
	row, err := r.s.queries(ctx).UpsertTaskAssignmentStatus(ctx, sqlcgen.UpsertTaskAssignmentStatusParams{
		TaskID: taskID, UserID: userID, Status: string(status), CompletedAt: completedAt,
	})
	if err != nil {
		return nil, mapErr(err, "task assignment")
	}
	a := toAssignment(row)
	return &a, nil
}

func (r *taskRepo) AddAttachment(ctx context.Context, taskID, materialID uuid.UUID) error {
	err := r.s.queries(ctx).AddTaskAttachment(ctx, sqlcgen.AddTaskAttachmentParams{TaskID: taskID, MaterialID: materialID})
	return mapErr(err, "task attachment")
}

func (r *taskRepo) SetAttachments(ctx context.Context, taskID, groupID uuid.UUID, materialIDs []uuid.UUID) error {
	q := r.s.queries(ctx)
	if materialIDs == nil {
		materialIDs = []uuid.UUID{}
	}
	if err := q.DeleteTaskAttachments(ctx, sqlcgen.DeleteTaskAttachmentsParams{TaskID: taskID, MaterialIds: materialIDs}); err != nil {
		return mapErr(err, "task attachment")
	}
	if len(materialIDs) == 0 {
		return nil
	}
	return mapErr(q.InsertTaskAttachments(ctx, sqlcgen.InsertTaskAttachmentsParams{
		TaskID: taskID, MaterialIds: materialIDs, GroupID: groupID,
	}), "task attachment")
}

func (r *taskRepo) ListDueReminders(ctx context.Context, now time.Time, offsets []int32, graceMinutes, limit int32) ([]domain.DueTask, error) {
	rows, err := r.s.queries(ctx).ListTaskDueReminders(ctx, sqlcgen.ListTaskDueRemindersParams{
		Offsets: offsets, Now: now, GraceMinutes: graceMinutes, MaxRows: limit,
	})
	if err != nil {
		return nil, mapErr(err, "task")
	}
	// One row per fired offset; collapse them into one entry per task.
	out := make([]domain.DueTask, 0, len(rows))
	index := map[uuid.UUID]int{}
	for _, row := range rows {
		i, ok := index[row.Task.ID]
		if !ok {
			index[row.Task.ID] = len(out)
			out = append(out, domain.DueTask{Task: *toTask(row.Task)})
			i = len(out) - 1
		}
		out[i].Offsets = append(out[i].Offsets, row.OffsetMinutes)
	}
	return out, nil
}

func (r *taskRepo) MarkNotified(ctx context.Context, id uuid.UUID, offsets []int32) error {
	return mapErr(r.s.queries(ctx).MarkTaskNotified(ctx, sqlcgen.MarkTaskNotifiedParams{ID: id, Offsets: offsets}), "task")
}
