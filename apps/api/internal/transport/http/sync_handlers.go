package http

import (
	"context"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"
)

type syncInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Since   int64  `query:"since" minimum:"0" doc:"Последний полученный seq; 0 — с начала."`
	Limit   int32  `query:"limit" minimum:"1" maximum:"1000" default:"200"`
}

type syncOutput struct {
	Body struct {
		Events  []EventDTO `json:"events"`
		NextSeq int64      `json:"next_seq" doc:"Курсор для следующего вызова."`
		Latest  int64      `json:"latest" doc:"Последний seq группы на момент ответа."`
		HasMore bool       `json:"has_more"`
	}
}

type activityInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Before  int64  `query:"before" minimum:"0" doc:"Вернуть события с seq меньше этого; 0 — с самых новых."`
	Limit   int32  `query:"limit" minimum:"1" maximum:"200" default:"50"`
}

type activityOutput struct {
	Body struct {
		Items []EventDTO `json:"items"`
	}
}

func registerSync(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "sync-since", Method: nethttp.MethodGet, Path: "/groups/{groupId}/sync", Tags: []string{"sync"}, Security: bearer,
		Summary:     "Дельта-синхронизация журнала группы",
		Description: "Возвращает события с seq > since. 410 Gone — история за курсором уже удалена: клиент должен перезагрузить данные с нуля.",
	}, func(ctx context.Context, in *syncInput) (*syncOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		res, err := d.Sync.Since(ctx, p.UserID, groupID, in.Since, in.Limit)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &syncOutput{}
		out.Body.Events = make([]EventDTO, len(res.Events))
		for i := range res.Events {
			out.Body.Events[i] = toEventDTO(&res.Events[i])
		}
		out.Body.NextSeq = res.NextSeq
		out.Body.Latest = res.Latest
		out.Body.HasMore = res.HasMore
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "activity-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/activity", Tags: []string{"sync"}, Security: bearer,
		Summary: "Лента активности (кто что когда)",
	}, func(ctx context.Context, in *activityInput) (*activityOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		var before *int64
		if in.Before > 0 {
			before = &in.Before
		}
		items, err := d.Sync.Activity(ctx, p.UserID, groupID, before, in.Limit)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &activityOutput{}
		out.Body.Items = make([]EventDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toEventDTO(&items[i])
		}
		return out, nil
	})
}
