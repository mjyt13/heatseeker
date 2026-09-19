package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type userRepo struct{ s *Store }

func toUser(u sqlcgen.User) *domain.User {
	return &domain.User{
		ID:          u.ID,
		Name:        u.Name,
		Email:       u.Email,
		Locale:      u.Locale,
		Timezone:    u.Timezone,
		AvatarKey:   u.AvatarKey,
		GlobalRole:  domain.GlobalRole(u.GlobalRole),
		Settings:    json.RawMessage(rawJSON(u.Settings)),
		SecuredAt:   u.SecuredAt,
		HasPassword: u.PasswordHash != nil && *u.PasswordHash != "",
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func (r *userRepo) GetByRegisterClientID(ctx context.Context, clientID uuid.UUID) (*domain.User, error) {
	u, err := r.s.queries(ctx).GetUserByRegisterClientID(ctx, &clientID)
	if err != nil {
		return nil, mapErr(err, "user")
	}
	return toUser(u), nil
}

func (r *userRepo) Create(ctx context.Context, p domain.CreateUserParams) (*domain.User, error) {
	var securedAt *time.Time
	if p.Secured {
		now := time.Now().UTC()
		securedAt = &now
	}
	u, err := r.s.queries(ctx).CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:           p.ID,
		Name:         p.Name,
		Email:        p.Email,
		PasswordHash: p.PasswordHash,
		Locale:       p.Locale,
		Timezone:     p.Timezone,
		SecuredAt:    securedAt,

		RegisterClientID: p.RegisterClientID,
	})
	if err != nil {
		return nil, mapErr(err, "user")
	}
	return toUser(u), nil
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, err := r.s.queries(ctx).GetUserByID(ctx, id)
	if err != nil {
		return nil, mapErr(err, "user")
	}
	return toUser(u), nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, string, error) {
	u, err := r.s.queries(ctx).GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", mapErr(err, "user")
	}
	hash := ""
	if u.PasswordHash != nil {
		hash = *u.PasswordHash
	}
	return toUser(u), hash, nil
}

func (r *userRepo) UpdateProfile(ctx context.Context, id uuid.UUID, name, locale, timezone string, settings json.RawMessage) (*domain.User, error) {
	u, err := r.s.queries(ctx).UpdateUserProfile(ctx, sqlcgen.UpdateUserProfileParams{
		ID:       id,
		Name:     name,
		Locale:   locale,
		Timezone: timezone,
		Settings: rawJSON(settings),
	})
	if err != nil {
		return nil, mapErr(err, "user")
	}
	return toUser(u), nil
}

func (r *userRepo) SetCredentials(ctx context.Context, id uuid.UUID, email, passwordHash *string) (*domain.User, error) {
	u, err := r.s.queries(ctx).SetUserCredentials(ctx, sqlcgen.SetUserCredentialsParams{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, mapErr(err, "user")
	}
	return toUser(u), nil
}

func (r *userRepo) MarkSecured(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.s.queries(ctx).MarkUserSecured(ctx, id), "user")
}
