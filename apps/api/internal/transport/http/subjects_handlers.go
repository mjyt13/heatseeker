package http

import (
	"context"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"

	"heatseeker/api/internal/app/subjects"
)

type subjectBody struct {
	Name           string   `json:"name" minLength:"1" maxLength:"120"`
	ShortName      *string  `json:"short_name,omitempty" maxLength:"30"`
	Teacher        *string  `json:"teacher,omitempty" maxLength:"120"`
	TeacherContact *string  `json:"teacher_contact,omitempty" maxLength:"200"`
	Color          *string  `json:"color,omitempty" maxLength:"16" doc:"Цвет чипа, например #3B82F6."`
	Semester       *string  `json:"semester,omitempty" maxLength:"30"`
	Aliases        []string `json:"aliases,omitempty" doc:"Синонимы для классификатора файлов с Диска."`
	SortOrder      *int32   `json:"sort_order,omitempty"`
}

func (b subjectBody) input() subjects.Input {
	return subjects.Input{
		Name: b.Name, ShortName: b.ShortName, Teacher: b.Teacher, TeacherContact: b.TeacherContact,
		Color: b.Color, Semester: b.Semester, Aliases: b.Aliases, SortOrder: b.SortOrder,
	}
}

type createSubjectInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    subjectBody
}

type updateSubjectInput struct {
	GroupID   string `path:"groupId" format:"uuid"`
	SubjectID string `path:"subjectId" format:"uuid"`
	Body      subjectBody
}

type subjectIDInput struct {
	GroupID   string `path:"groupId" format:"uuid"`
	SubjectID string `path:"subjectId" format:"uuid"`
}

type listSubjectsInput struct {
	GroupID         string `path:"groupId" format:"uuid"`
	IncludeArchived bool   `query:"include_archived"`
}

type subjectOutput struct {
	Body SubjectDTO
}

type subjectsOutput struct {
	Body struct {
		Items []SubjectDTO `json:"items"`
	}
}

type aliasesInput struct {
	GroupID   string `path:"groupId" format:"uuid"`
	SubjectID string `path:"subjectId" format:"uuid"`
	Body      struct {
		Aliases []string `json:"aliases" minItems:"1"`
	}
}

func registerSubjects(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "subjects-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/subjects", Tags: []string{"subjects"}, Security: bearer,
		Summary: "Предметы группы",
	}, func(ctx context.Context, in *listSubjectsInput) (*subjectsOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		items, err := d.Subjects.List(ctx, p.UserID, groupID, in.IncludeArchived)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &subjectsOutput{}
		out.Body.Items = make([]SubjectDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toSubjectDTO(&items[i])
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "subjects-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/subjects", Tags: []string{"subjects"}, Security: bearer,
		Summary: "Добавить предмет", DefaultStatus: nethttp.StatusCreated,
		Description: "Вместе с предметом создаётся его тег (kind=SUBJECT) и тред обсуждения (со 2-го этапа).",
	}, func(ctx context.Context, in *createSubjectInput) (*subjectOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		s, err := d.Subjects.Create(ctx, p.UserID, groupID, in.Body.input())
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &subjectOutput{Body: toSubjectDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "subjects-get", Method: nethttp.MethodGet, Path: "/groups/{groupId}/subjects/{subjectId}", Tags: []string{"subjects"}, Security: bearer,
		Summary: "Предмет",
	}, func(ctx context.Context, in *subjectIDInput) (*subjectOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, subjectID, err := parseGroupSubject(in.GroupID, in.SubjectID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		s, err := d.Subjects.Get(ctx, p.UserID, groupID, subjectID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &subjectOutput{Body: toSubjectDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "subjects-update", Method: nethttp.MethodPut, Path: "/groups/{groupId}/subjects/{subjectId}", Tags: []string{"subjects"}, Security: bearer,
		Summary: "Изменить предмет",
	}, func(ctx context.Context, in *updateSubjectInput) (*subjectOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, subjectID, err := parseGroupSubject(in.GroupID, in.SubjectID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		s, err := d.Subjects.Update(ctx, p.UserID, groupID, subjectID, in.Body.input())
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &subjectOutput{Body: toSubjectDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "subjects-archive", Method: nethttp.MethodPost, Path: "/groups/{groupId}/subjects/{subjectId}/archive", Tags: []string{"subjects"}, Security: bearer,
		Summary: "Архивировать предмет", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *subjectIDInput) (*emptyOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, subjectID, err := parseGroupSubject(in.GroupID, in.SubjectID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Subjects.Archive(ctx, p.UserID, groupID, subjectID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "subjects-restore", Method: nethttp.MethodPost, Path: "/groups/{groupId}/subjects/{subjectId}/restore", Tags: []string{"subjects"}, Security: bearer,
		Summary: "Восстановить предмет", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *subjectIDInput) (*emptyOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, subjectID, err := parseGroupSubject(in.GroupID, in.SubjectID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Subjects.Restore(ctx, p.UserID, groupID, subjectID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "subjects-add-aliases", Method: nethttp.MethodPost, Path: "/groups/{groupId}/subjects/{subjectId}/aliases", Tags: []string{"subjects"}, Security: bearer,
		Summary: "Добавить синонимы предмета", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *aliasesInput) (*emptyOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, subjectID, err := parseGroupSubject(in.GroupID, in.SubjectID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Subjects.AddAliases(ctx, p.UserID, groupID, subjectID, in.Body.Aliases); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})
}

func parseGroupSubject(groupRaw, subjectRaw string) (groupID, subjectID uuidValue, err error) {
	groupID, err = parseID("groupId", groupRaw)
	if err != nil {
		return
	}
	subjectID, err = parseID("subjectId", subjectRaw)
	return
}
