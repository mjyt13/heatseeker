package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type authRepo struct{ s *Store }

func toIdentity(i sqlcgen.AuthIdentity) *domain.AuthIdentity {
	return &domain.AuthIdentity{
		ID:             i.ID,
		UserID:         i.UserID,
		Provider:       domain.IdentityProvider(i.Provider),
		ProviderUserID: i.ProviderUserID,
		Email:          i.Email,
		CreatedAt:      i.CreatedAt,
	}
}

func toDevice(d sqlcgen.Device) *domain.Device {
	var provider *domain.PushProvider
	if d.PushProvider != nil {
		p := domain.PushProvider(*d.PushProvider)
		provider = &p
	}
	return &domain.Device{
		ID:               d.ID,
		UserID:           d.UserID,
		Platform:         domain.DevicePlatform(d.Platform),
		Name:             d.Name,
		PushProvider:     provider,
		PushToken:        d.PushToken,
		PushSubscription: json.RawMessage(d.PushSubscription),
		Enabled:          d.Enabled,
		LastSeenAt:       d.LastSeenAt,
		CreatedAt:        d.CreatedAt,
	}
}

func toRefreshToken(t sqlcgen.RefreshToken) *domain.RefreshToken {
	return &domain.RefreshToken{
		ID:         t.ID,
		UserID:     t.UserID,
		DeviceID:   t.DeviceID,
		TokenHash:  t.TokenHash,
		ExpiresAt:  t.ExpiresAt,
		RevokedAt:  t.RevokedAt,
		ReplacedBy: t.ReplacedBy,
		CreatedAt:  t.CreatedAt,
	}
}

func (r *authRepo) CreateIdentity(ctx context.Context, id domain.AuthIdentity) (*domain.AuthIdentity, error) {
	i, err := r.s.queries(ctx).CreateIdentity(ctx, sqlcgen.CreateIdentityParams{
		ID:             id.ID,
		UserID:         id.UserID,
		Provider:       string(id.Provider),
		ProviderUserID: id.ProviderUserID,
		Email:          id.Email,
	})
	if err != nil {
		return nil, mapErr(err, "identity")
	}
	return toIdentity(i), nil
}

func (r *authRepo) GetIdentity(ctx context.Context, provider domain.IdentityProvider, providerUserID string) (*domain.AuthIdentity, error) {
	i, err := r.s.queries(ctx).GetIdentityByProvider(ctx, sqlcgen.GetIdentityByProviderParams{
		Provider:       string(provider),
		ProviderUserID: providerUserID,
	})
	if err != nil {
		return nil, mapErr(err, "identity")
	}
	return toIdentity(i), nil
}

func (r *authRepo) ListIdentities(ctx context.Context, userID uuid.UUID) ([]domain.AuthIdentity, error) {
	rows, err := r.s.queries(ctx).ListIdentitiesByUser(ctx, userID)
	if err != nil {
		return nil, mapErr(err, "identity")
	}
	out := make([]domain.AuthIdentity, len(rows))
	for i, row := range rows {
		out[i] = *toIdentity(row)
	}
	return out, nil
}

func (r *authRepo) CreateDevice(ctx context.Context, d domain.Device) (*domain.Device, error) {
	row, err := r.s.queries(ctx).CreateDevice(ctx, sqlcgen.CreateDeviceParams{
		ID:       d.ID,
		UserID:   d.UserID,
		Platform: string(d.Platform),
		Name:     d.Name,
	})
	if err != nil {
		return nil, mapErr(err, "device")
	}
	return toDevice(row), nil
}

func (r *authRepo) GetDevice(ctx context.Context, id, userID uuid.UUID) (*domain.Device, error) {
	row, err := r.s.queries(ctx).GetDevice(ctx, sqlcgen.GetDeviceParams{ID: id, UserID: userID})
	if err != nil {
		return nil, mapErr(err, "device")
	}
	return toDevice(row), nil
}

func (r *authRepo) ListDevices(ctx context.Context, userID uuid.UUID) ([]domain.Device, error) {
	rows, err := r.s.queries(ctx).ListDevicesByUser(ctx, userID)
	if err != nil {
		return nil, mapErr(err, "device")
	}
	out := make([]domain.Device, len(rows))
	for i, row := range rows {
		out[i] = *toDevice(row)
	}
	return out, nil
}

func (r *authRepo) TouchDevice(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.s.queries(ctx).TouchDevice(ctx, id), "device")
}

func (r *authRepo) UpdateDevicePush(ctx context.Context, id, userID uuid.UUID, provider *domain.PushProvider, token *string, subscription json.RawMessage) (*domain.Device, error) {
	var p *string
	if provider != nil {
		s := string(*provider)
		p = &s
	}
	row, err := r.s.queries(ctx).UpdateDevicePush(ctx, sqlcgen.UpdateDevicePushParams{
		ID:               id,
		UserID:           userID,
		PushProvider:     p,
		PushToken:        token,
		PushSubscription: subscription,
	})
	if err != nil {
		return nil, mapErr(err, "device")
	}
	return toDevice(row), nil
}

func (r *authRepo) DeleteDevice(ctx context.Context, id, userID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).DeleteDevice(ctx, sqlcgen.DeleteDeviceParams{ID: id, UserID: userID}), "device")
}

func (r *authRepo) CreateRefreshToken(ctx context.Context, t domain.RefreshToken) (*domain.RefreshToken, error) {
	row, err := r.s.queries(ctx).CreateRefreshToken(ctx, sqlcgen.CreateRefreshTokenParams{
		ID:        t.ID,
		UserID:    t.UserID,
		DeviceID:  t.DeviceID,
		TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt,
	})
	if err != nil {
		return nil, mapErr(err, "refresh token")
	}
	return toRefreshToken(row), nil
}

func (r *authRepo) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	row, err := r.s.queries(ctx).GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, mapErr(err, "refresh token")
	}
	return toRefreshToken(row), nil
}

func (r *authRepo) RevokeRefreshToken(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error {
	return mapErr(r.s.queries(ctx).RevokeRefreshToken(ctx, sqlcgen.RevokeRefreshTokenParams{ID: id, ReplacedBy: replacedBy}), "refresh token")
}

func (r *authRepo) RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).RevokeAllUserRefreshTokens(ctx, userID), "refresh token")
}

func (r *authRepo) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	n, err := r.s.queries(ctx).DeleteExpiredRefreshTokens(ctx)
	return n, mapErr(err, "refresh token")
}
