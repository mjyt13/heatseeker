package http

import (
	"context"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"

	"heatseeker/api/internal/domain"
)

type driveStatusOutput struct {
	Body DriveStatusDTO
}

type connectDriveInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    struct {
		Folder string `json:"folder" minLength:"10" maxLength:"500" doc:"Ссылка на папку Google Диска или её id."`
	}
}

type driveConnectionOutput struct {
	Body DriveConnectionDTO
}

type driveSyncInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    *struct {
		Full bool `json:"full,omitempty" doc:"Полный перескан папки вместо ленты изменений."`
	}
}

type driveItemsInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	State   string `query:"state" enum:"NEW,LINKED,IMPORTED,SKIPPED,ERROR,DELETED"`
	Limit   int32  `query:"limit" minimum:"1" maximum:"200" default:"100"`
	Offset  int32  `query:"offset" minimum:"0"`
}

type driveItemsOutput struct {
	Body struct {
		Items []DriveItemDTO `json:"items"`
	}
}

func registerDrive(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "drive-status", Method: nethttp.MethodGet, Path: "/groups/{groupId}/drive/status", Tags: []string{"drive"}, Security: bearer,
		Summary: "Подключение Google Диска", Description: "Адрес сервисного аккаунта, папка, состояние синхронизации и счётчики.",
	}, func(ctx context.Context, in *groupIDInput) (*driveStatusOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		st, err := d.Drive.Status(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &driveStatusOutput{Body: toDriveStatusDTO(st)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "drive-connect", Method: nethttp.MethodPut, Path: "/groups/{groupId}/drive/connection", Tags: []string{"drive"}, Security: bearer,
		Summary:     "Подключить папку Google Диска",
		Description: "Папка должна быть открыта для service_account_email (для загрузки из приложения — с правами редактора). Первая индексация запускается сразу.",
	}, func(ctx context.Context, in *connectDriveInput) (*driveConnectionOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		conn, err := d.Drive.Connect(ctx, p.UserID, groupID, in.Body.Folder)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &driveConnectionOutput{Body: *toDriveConnectionDTO(conn)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "drive-disconnect", Method: nethttp.MethodDelete, Path: "/groups/{groupId}/drive/connection", Tags: []string{"drive"}, Security: bearer,
		Summary: "Отключить Google Диск", Description: "Индекс удаляется, материалы остаются.", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *groupIDInput) (*emptyOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Drive.Disconnect(ctx, p.UserID, groupID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "drive-sync", Method: nethttp.MethodPost, Path: "/groups/{groupId}/drive/sync", Tags: []string{"drive"}, Security: bearer,
		Summary: "Синхронизировать сейчас", DefaultStatus: nethttp.StatusAccepted,
	}, func(ctx context.Context, in *driveSyncInput) (*emptyOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		full := in.Body != nil && in.Body.Full
		if err := d.Drive.RequestSync(ctx, p.UserID, groupID, full); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "drive-items", Method: nethttp.MethodGet, Path: "/groups/{groupId}/drive/items", Tags: []string{"drive"}, Security: bearer,
		Summary: "Файлы в индексе Диска", Description: "Для диагностики: что найдено и что с каждым файлом сделано.",
	}, func(ctx context.Context, in *driveItemsInput) (*driveItemsOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		var state *domain.DriveItemState
		if in.State != "" {
			s := domain.DriveItemState(in.State)
			state = &s
		}
		items, err := d.Drive.ListItems(ctx, p.UserID, groupID, state, in.Limit, in.Offset)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &driveItemsOutput{}
		out.Body.Items = make([]DriveItemDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toDriveItemDTO(&items[i])
		}
		return out, nil
	})
}
