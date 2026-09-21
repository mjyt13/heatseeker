package http

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heatseeker/api/internal/app/tasks"
	"heatseeker/api/internal/domain"
)

type taskBody struct {
	ClientID    *string  `json:"client_id,omitempty" format:"uuid" doc:"Идемпотентность: повтор с тем же id вернёт уже созданную задачу."`
	Title       string   `json:"title" minLength:"1" maxLength:"200"`
	Description string   `json:"description,omitempty" maxLength:"5000" doc:"Markdown."`
	SubjectID   *string  `json:"subject_id,omitempty" format:"uuid"`
	Kind        *string  `json:"kind,omitempty" enum:"TEACHER,GROUP,PERSONAL"`
	Priority    *string  `json:"priority,omitempty" enum:"LOW,NORMAL,HIGH"`
	AssignMode  *string  `json:"assign_mode,omitempty" enum:"ALL,SELECTED,SELF"`
	Visibility  *string  `json:"visibility,omitempty" enum:"GROUP,PRIVATE"`
	DueAt       *string  `json:"due_at,omitempty" format:"date-time" doc:"Срок; пусто — без срока."`
	AssigneeIDs []string `json:"assignee_ids,omitempty" maxItems:"100" doc:"Кому задача при assign_mode=SELECTED."`
	MaterialIDs []string `json:"material_ids,omitempty" maxItems:"20" doc:"Прикреплённые материалы (заменяются целиком); не передано — при правке остаются как были."`
}

// input converts the body, parsing ids and the deadline.
func (b taskBody) input() (tasks.Input, error) {
	in := tasks.Input{Title: b.Title, Description: b.Description}
	var err error
	if b.ClientID != nil {
		if in.ClientID, err = parseOptionalID("client_id", *b.ClientID); err != nil {
			return in, err
		}
	}
	if b.SubjectID != nil {
		if in.SubjectID, err = parseOptionalID("subject_id", *b.SubjectID); err != nil {
			return in, err
		}
	}
	if in.AssigneeIDs, err = parseIDs("assignee_ids", b.AssigneeIDs); err != nil {
		return in, err
	}
	// Omitted material_ids keep the attachments on update.
	if b.MaterialIDs != nil {
		if in.MaterialIDs, err = parseIDs("material_ids", b.MaterialIDs); err != nil {
			return in, err
		}
	}
	if b.Kind != nil {
		k := domain.TaskKind(*b.Kind)
		in.Kind = &k
	}
	if b.Priority != nil {
		p := domain.TaskPriority(*b.Priority)
		in.Priority = &p
	}
	if b.AssignMode != nil {
		m := domain.TaskAssignMode(*b.AssignMode)
		in.AssignMode = &m
	}
	if b.Visibility != nil {
		v := domain.TaskVisibility(*b.Visibility)
		in.Visibility = &v
	}
	if b.DueAt != nil && *b.DueAt != "" {
		due, err := time.Parse(time.RFC3339, *b.DueAt)
		if err != nil {
			return in, domain.Invalid("due_at", "must be a date and time in RFC 3339")
		}
		in.DueAt = &due
	}
	return in, nil
}

type createTaskInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    taskBody
}

type updateTaskInput struct {
	TaskID string `path:"taskId" format:"uuid"`
	Body   taskBody
}

type listTasksInput struct {
	GroupID   string   `path:"groupId" format:"uuid"`
	SubjectID string   `query:"subject_id" format:"uuid"`
	NoSubject bool     `query:"no_subject" doc:"Только задачи без предмета."`
	Kind      string   `query:"kind" enum:"TEACHER,GROUP,PERSONAL"`
	Status    []string `query:"status" doc:"Один или несколько статусов."`
	Mine      bool     `query:"mine" doc:"Только мои: созданные мной, выданные мне или всей группе."`
	Open      bool     `query:"open" doc:"Без закрытых и отменённых."`
	Overdue   bool     `query:"overdue" doc:"Только просроченные."`
	DueBefore string   `query:"due_before" format:"date-time" doc:"Срок не позже указанного."`
	Query     string   `query:"q" maxLength:"200"`
	Limit     int32    `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type taskIDInput struct {
	TaskID string `path:"taskId" format:"uuid"`
}

type taskStatusInput struct {
	TaskID string `path:"taskId" format:"uuid"`
	Body   struct {
		Status string `json:"status" enum:"TODO,IN_PROGRESS,IN_REVIEW,DONE,CANCELLED"`
	}
}

type taskMaterialsInput struct {
	TaskID string `path:"taskId" format:"uuid"`
	Body   struct {
		MaterialIDs []string `json:"material_ids" maxItems:"20" doc:"Все прикреплённые материалы; пустой список открепляет всё."`
	}
}

type taskOutput struct {
	Body TaskDTO
}

type tasksOutput struct {
	Body struct {
		Items []TaskDTO `json:"items"`
	}
}

type taskCountsOutput struct {
	Body TaskCountsDTO
}

func registerTasks(api huma.API, d Deps) {
	now := func() time.Time { return time.Now().UTC() }

	huma.Register(api, huma.Operation{
		OperationID: "tasks-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/tasks", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Задачи группы", Description: "Сначала закреплённые, затем по сроку. Личные задачи видит только их автор.",
	}, func(ctx context.Context, in *listTasksInput) (*tasksOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		li := tasks.ListInput{
			NoSubject: in.NoSubject, Mine: in.Mine, Open: in.Open, Overdue: in.Overdue,
			Query: in.Query, Limit: in.Limit,
		}
		if li.SubjectID, err = parseOptionalID("subject_id", in.SubjectID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		if in.Kind != "" {
			k := domain.TaskKind(in.Kind)
			li.Kind = &k
		}
		for _, st := range in.Status {
			li.Statuses = append(li.Statuses, domain.TaskStatus(st))
		}
		if in.DueBefore != "" {
			due, err := time.Parse(time.RFC3339, in.DueBefore)
			if err != nil {
				return nil, apiErr(d.Log, domain.Invalid("due_before", "must be a date and time in RFC 3339"))
			}
			li.DueBefore = &due
		}
		items, err := d.Tasks.List(ctx, p.UserID, groupID, li)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &tasksOutput{}
		at := now()
		out.Body.Items = make([]TaskDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toTaskDTO(&items[i], at)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-board", Method: nethttp.MethodGet, Path: "/groups/{groupId}/tasks/board", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Сводка по задачам", Description: "Счётчики для доски и бейджей: по статусам, открытые, просроченные, ближайшие и мои.",
	}, func(ctx context.Context, in *groupIDInput) (*taskCountsOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		counts, err := d.Tasks.Counts(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &taskCountsOutput{Body: toTaskCountsDTO(counts)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/tasks", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Создать задачу", Description: "Может любой участник. Повтор с тем же client_id возвращает созданную задачу.",
		DefaultStatus: nethttp.StatusCreated,
	}, func(ctx context.Context, in *createTaskInput) (*taskOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		ti, err := in.Body.input()
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Tasks.Create(ctx, p.UserID, groupID, ti)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &taskOutput{Body: toTaskDTO(view, now())}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-get", Method: nethttp.MethodGet, Path: "/tasks/{taskId}", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Задача",
	}, func(ctx context.Context, in *taskIDInput) (*taskOutput, error) {
		p, id, err := principalAndID(ctx, "taskId", in.TaskID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Tasks.Get(ctx, p.UserID, id)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &taskOutput{Body: toTaskDTO(view, now())}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-update", Method: nethttp.MethodPatch, Path: "/tasks/{taskId}", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Изменить задачу", Description: "Автор, староста, модератор или админ. Поля заменяются целиком; перенос срока снова включает напоминания.",
	}, func(ctx context.Context, in *updateTaskInput) (*taskOutput, error) {
		p, id, err := principalAndID(ctx, "taskId", in.TaskID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		ti, err := in.Body.input()
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Tasks.Update(ctx, p.UserID, id, ti)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &taskOutput{Body: toTaskDTO(view, now())}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-status", Method: nethttp.MethodPatch, Path: "/tasks/{taskId}/status", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Статус задачи для группы",
	}, func(ctx context.Context, in *taskStatusInput) (*taskOutput, error) {
		p, id, err := principalAndID(ctx, "taskId", in.TaskID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Tasks.SetStatus(ctx, p.UserID, id, domain.TaskStatus(in.Body.Status))
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &taskOutput{Body: toTaskDTO(view, now())}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-my-status", Method: nethttp.MethodPatch, Path: "/tasks/{taskId}/me/status", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Мой прогресс по задаче", Description: "Личный статус; общий статус задачи не меняется.",
	}, func(ctx context.Context, in *taskStatusInput) (*taskOutput, error) {
		p, id, err := principalAndID(ctx, "taskId", in.TaskID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Tasks.SetMyStatus(ctx, p.UserID, id, domain.TaskStatus(in.Body.Status))
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &taskOutput{Body: toTaskDTO(view, now())}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-pin", Method: nethttp.MethodPost, Path: "/tasks/{taskId}/pin", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Закрепить задачу", Description: "Староста, модератор или админ.",
	}, func(ctx context.Context, in *taskIDInput) (*taskOutput, error) {
		return pinTask(ctx, d, in.TaskID, true, now())
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-unpin", Method: nethttp.MethodDelete, Path: "/tasks/{taskId}/pin", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Открепить задачу",
	}, func(ctx context.Context, in *taskIDInput) (*taskOutput, error) {
		return pinTask(ctx, d, in.TaskID, false, now())
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-materials", Method: nethttp.MethodPut, Path: "/tasks/{taskId}/materials", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Прикреплённые файлы задачи", Description: "Заменяет набор целиком. Автор, староста, модератор или админ.",
	}, func(ctx context.Context, in *taskMaterialsInput) (*taskOutput, error) {
		p, id, err := principalAndID(ctx, "taskId", in.TaskID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		ids, err := parseIDs("material_ids", in.Body.MaterialIDs)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Tasks.SetMaterials(ctx, p.UserID, id, ids)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &taskOutput{Body: toTaskDTO(view, now())}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tasks-delete", Method: nethttp.MethodDelete, Path: "/tasks/{taskId}", Tags: []string{"tasks"}, Security: bearer,
		Summary: "Удалить задачу", Description: "Автор, староста, модератор или админ.", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *taskIDInput) (*struct{}, error) {
		p, id, err := principalAndID(ctx, "taskId", in.TaskID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Tasks.Delete(ctx, p.UserID, id); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return nil, nil
	})
}

func pinTask(ctx context.Context, d Deps, raw string, pinned bool, at time.Time) (*taskOutput, error) {
	p, id, err := principalAndID(ctx, "taskId", raw)
	if err != nil {
		return nil, apiErr(d.Log, err)
	}
	view, err := d.Tasks.SetPinned(ctx, p.UserID, id, pinned)
	if err != nil {
		return nil, apiErr(d.Log, err)
	}
	return &taskOutput{Body: toTaskDTO(view, at)}, nil
}
