package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

type uuidPtr struct{ id uuid.UUID }

func (u *uuidPtr) value() *uuid.UUID {
	if u == nil {
		return nil
	}
	return &u.id
}

type userOutput struct {
	Body UserDTO
}

type updateProfileInput struct {
	Body struct {
		Name     *string         `json:"name,omitempty" minLength:"1" maxLength:"80"`
		Locale   *string         `json:"locale,omitempty" maxLength:"8"`
		Timezone *string         `json:"timezone,omitempty" maxLength:"64"`
		Settings json.RawMessage `json:"settings,omitempty" doc:"Произвольные настройки клиента (набор быстрых тегов и т.п.)."`
	}
}

type credentialsInput struct {
	Body struct {
		Email    *string `json:"email,omitempty" format:"email"`
		Password *string `json:"password,omitempty" minLength:"8" maxLength:"256"`
	}
}

type devicesOutput struct {
	Body struct {
		Items []DeviceDTO `json:"items"`
	}
}

type pushInput struct {
	DeviceID string `path:"deviceId" format:"uuid"`
	Body     struct {
		Provider     string          `json:"provider" enum:"EXPO,WEBPUSH"`
		Token        *string         `json:"token,omitempty" doc:"Expo push token."`
		Subscription json.RawMessage `json:"subscription,omitempty" doc:"Web Push subscription JSON."`
	}
}

type deviceOutput struct {
	Body DeviceDTO
}

type deviceIDInput struct {
	DeviceID string `path:"deviceId" format:"uuid"`
}

type groupsListOutput struct {
	Body struct {
		Items []GroupWithMembershipDTO `json:"items"`
	}
}

func registerMe(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "me-get", Method: nethttp.MethodGet, Path: "/me", Tags: []string{"me"}, Security: bearer,
		Summary: "Текущий пользователь",
	}, func(ctx context.Context, _ *struct{}) (*userOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		u, err := d.Auth.UpdateProfile(ctx, p.UserID, "", "", "", nil)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &userOutput{Body: toUserDTO(u)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me-update", Method: nethttp.MethodPatch, Path: "/me", Tags: []string{"me"}, Security: bearer,
		Summary: "Обновить профиль",
	}, func(ctx context.Context, in *updateProfileInput) (*userOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		u, err := d.Auth.UpdateProfile(ctx, p.UserID, deref(in.Body.Name), deref(in.Body.Locale), deref(in.Body.Timezone), in.Body.Settings)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &userOutput{Body: toUserDTO(u)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me-set-credentials", Method: nethttp.MethodPost, Path: "/me/credentials", Tags: []string{"me"}, Security: bearer,
		Summary:     "Защитить аккаунт: задать email и/или пароль",
		Description: "Переводит аккаунт на уровень L2 — вход с других устройств, восстановление, роли админа/модератора/старосты.",
	}, func(ctx context.Context, in *credentialsInput) (*userOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		u, err := d.Auth.SetCredentials(ctx, p.UserID, in.Body.Email, in.Body.Password)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &userOutput{Body: toUserDTO(u)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me-logout-all", Method: nethttp.MethodPost, Path: "/me/logout-all", Tags: []string{"me"}, Security: bearer,
		Summary: "Завершить все сессии", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Auth.LogoutAll(ctx, p.UserID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me-devices", Method: nethttp.MethodGet, Path: "/me/devices", Tags: []string{"me"}, Security: bearer,
		Summary: "Устройства пользователя",
	}, func(ctx context.Context, _ *struct{}) (*devicesOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		devices, err := d.Auth.Devices(ctx, p.UserID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &devicesOutput{}
		out.Body.Items = make([]DeviceDTO, len(devices))
		for i := range devices {
			out.Body.Items[i] = toDeviceDTO(&devices[i])
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me-device-push", Method: nethttp.MethodPut, Path: "/me/devices/{deviceId}/push", Tags: []string{"me"}, Security: bearer,
		Summary: "Зарегистрировать push-токен устройства",
	}, func(ctx context.Context, in *pushInput) (*deviceOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		deviceID, err := parseID("deviceId", in.DeviceID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		dev, err := d.Auth.RegisterPush(ctx, p.UserID, deviceID, domain.PushProvider(in.Body.Provider), in.Body.Token, in.Body.Subscription)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &deviceOutput{Body: toDeviceDTO(dev)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me-device-delete", Method: nethttp.MethodDelete, Path: "/me/devices/{deviceId}", Tags: []string{"me"}, Security: bearer,
		Summary: "Удалить устройство", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *deviceIDInput) (*emptyOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		deviceID, err := parseID("deviceId", in.DeviceID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Auth.RemoveDevice(ctx, p.UserID, deviceID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me-groups", Method: nethttp.MethodGet, Path: "/me/groups", Tags: []string{"me", "groups"}, Security: bearer,
		Summary: "Мои группы",
	}, func(ctx context.Context, _ *struct{}) (*groupsListOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		items, err := d.Groups.ListMine(ctx, p.UserID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &groupsListOutput{}
		out.Body.Items = make([]GroupWithMembershipDTO, len(items))
		for i, gm := range items {
			out.Body.Items[i] = toGroupWithMembershipDTO(gm)
		}
		return out, nil
	})
}
