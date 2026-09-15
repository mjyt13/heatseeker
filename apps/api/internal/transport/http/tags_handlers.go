package http

import (
	"context"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"heatseeker/api/internal/app/tags"
	"heatseeker/api/internal/domain"
)

// uuidValue is an alias so helper signatures read naturally.
type uuidValue = uuid.UUID

type tagBody struct {
	Name  string  `json:"name" minLength:"1" maxLength:"60"`
	Color *string `json:"color,omitempty" maxLength:"16"`
	Kind  string  `json:"kind,omitempty" enum:"TOPIC,TYPE,CUSTOM" doc:"По умолчанию CUSTOM."`
}

type createTagInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    tagBody
}

type updateTagInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	TagID   string `path:"tagId" format:"uuid"`
	Body    tagBody
}

type tagIDInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	TagID   string `path:"tagId" format:"uuid"`
}

type tagOutput struct {
	Body TagDTO
}

type tagsOutput struct {
	Body struct {
		Items []TagDTO `json:"items"`
	}
}

type quickTagsOutput struct {
	Body struct {
		Items []QuickTagDTO `json:"items"`
	}
}

func registerTags(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "tags-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/tags", Tags: []string{"tags"}, Security: bearer,
		Summary: "Теги группы",
	}, func(ctx context.Context, in *groupIDInput) (*tagsOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		items, err := d.Tags.List(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &tagsOutput{}
		out.Body.Items = make([]TagDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toTagDTO(&items[i])
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tags-quick", Method: nethttp.MethodGet, Path: "/groups/{groupId}/quick-tags", Tags: []string{"tags"}, Security: bearer,
		Summary:     "Быстрые теги для главного экрана",
		Description: "Системные фильтры («Мои», «Сохранённое», «Непрочитанное») и активные предметы в порядке сортировки.",
	}, func(ctx context.Context, in *groupIDInput) (*quickTagsOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		items, err := d.Tags.QuickTags(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &quickTagsOutput{}
		out.Body.Items = make([]QuickTagDTO, len(items))
		for i, q := range items {
			out.Body.Items[i] = toQuickTagDTO(q)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tags-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/tags", Tags: []string{"tags"}, Security: bearer,
		Summary: "Создать тег", DefaultStatus: nethttp.StatusCreated,
	}, func(ctx context.Context, in *createTagInput) (*tagOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		t, err := d.Tags.Create(ctx, p.UserID, groupID, tags.Input{Name: in.Body.Name, Color: in.Body.Color, Kind: domain.TagKind(in.Body.Kind)})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &tagOutput{Body: toTagDTO(t)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tags-update", Method: nethttp.MethodPut, Path: "/groups/{groupId}/tags/{tagId}", Tags: []string{"tags"}, Security: bearer,
		Summary: "Изменить тег",
	}, func(ctx context.Context, in *updateTagInput) (*tagOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		tagID, err := parseID("tagId", in.TagID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		t, err := d.Tags.Update(ctx, p.UserID, groupID, tagID, tags.Input{Name: in.Body.Name, Color: in.Body.Color})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &tagOutput{Body: toTagDTO(t)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "tags-delete", Method: nethttp.MethodDelete, Path: "/groups/{groupId}/tags/{tagId}", Tags: []string{"tags"}, Security: bearer,
		Summary: "Удалить тег", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *tagIDInput) (*emptyOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		tagID, err := parseID("tagId", in.TagID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Tags.Delete(ctx, p.UserID, groupID, tagID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})
}
