package http

import (
	"context"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"

	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/domain"
)

var bearer = []map[string][]string{{"bearer": {}}}

// DeviceInput is embedded in authentication requests.
type DeviceInput struct {
	Platform   string  `json:"platform,omitempty" enum:"IOS,ANDROID,WEB" doc:"Платформа клиента; по умолчанию WEB."`
	DeviceName *string `json:"device_name,omitempty" maxLength:"80"`
}

func (d DeviceInput) info() auth.DeviceInfo {
	return auth.DeviceInfo{Platform: domain.DevicePlatform(d.Platform), Name: d.DeviceName}
}

type registerInput struct {
	Body struct {
		Name       string  `json:"name" minLength:"1" maxLength:"80" doc:"Имя — единственное обязательное поле."`
		InviteCode *string `json:"invite_code,omitempty" maxLength:"64" doc:"Код приглашения или код группы — сразу вступить."`
		Locale     *string `json:"locale,omitempty" maxLength:"8"`
		Timezone   *string `json:"timezone,omitempty" maxLength:"64"`
		DeviceInput
	}
}

type sessionOutput struct {
	Body SessionDTO
}

type loginInput struct {
	Body struct {
		Email    string `json:"email" format:"email"`
		Password string `json:"password" minLength:"1" maxLength:"256"`
		DeviceInput
	}
}

type refreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token" minLength:"1"`
	}
}

type googleInput struct {
	Body struct {
		IDToken string `json:"id_token" minLength:"1"`
		DeviceInput
	}
}

type emptyOutput struct{}

func registerAuth(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-register", Method: nethttp.MethodPost, Path: "/auth/register", Tags: []string{"auth"},
		Summary: "Регистрация по имени", DefaultStatus: nethttp.StatusCreated,
		Description: "Создаёт лёгкий аккаунт (уровень L1). Email и пароль не нужны; их можно добавить позже через /me/credentials.",
	}, func(ctx context.Context, in *registerInput) (*sessionOutput, error) {
		s, err := d.Auth.Register(ctx, auth.RegisterInput{
			Name: in.Body.Name, InviteCode: deref(in.Body.InviteCode), Locale: deref(in.Body.Locale), Timezone: deref(in.Body.Timezone),
			Device: in.Body.info(),
		})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &sessionOutput{Body: toSessionDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-login", Method: nethttp.MethodPost, Path: "/auth/login", Tags: []string{"auth"},
		Summary: "Вход по email и паролю (защищённый аккаунт)",
	}, func(ctx context.Context, in *loginInput) (*sessionOutput, error) {
		s, err := d.Auth.Login(ctx, in.Body.Email, in.Body.Password, in.Body.info())
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &sessionOutput{Body: toSessionDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-refresh", Method: nethttp.MethodPost, Path: "/auth/refresh", Tags: []string{"auth"},
		Summary: "Обновить токены", Description: "Refresh-токен одноразовый: выдаётся новая пара, старый отзывается. Повторное использование отзывает все сессии пользователя.",
	}, func(ctx context.Context, in *refreshInput) (*sessionOutput, error) {
		s, err := d.Auth.Refresh(ctx, in.Body.RefreshToken)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &sessionOutput{Body: toSessionDTO(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-logout", Method: nethttp.MethodPost, Path: "/auth/logout", Tags: []string{"auth"},
		Summary: "Выход (отозвать refresh-токен)", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *refreshInput) (*emptyOutput, error) {
		if err := d.Auth.Logout(ctx, in.Body.RefreshToken); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-google", Method: nethttp.MethodPost, Path: "/auth/google", Tags: []string{"auth"},
		Summary:     "Вход или привязка через Google",
		Description: "Без bearer-токена: вход в привязанный аккаунт или создание нового защищённого аккаунта. С bearer-токеном: привязка Google к текущему аккаунту.",
	}, func(ctx context.Context, in *googleInput) (*sessionOutput, error) {
		var current *uuidPtr
		if p := optionalPrincipal(ctx); p != nil {
			current = &uuidPtr{p.UserID}
		}
		s, err := d.Auth.Google(ctx, in.Body.IDToken, current.value(), in.Body.info())
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &sessionOutput{Body: toSessionDTO(s)}, nil
	})
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
