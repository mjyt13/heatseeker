// Package auth implements registration, login, token rotation and account
// hardening (docs/adr/0004): a light account needs only a name; credentials
// are added later and unlock privileged roles.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
)

// Joiner joins a user to a group by invite or join code. Implemented by the
// groups service; declared here to avoid an import cycle.
type Joiner interface {
	Join(ctx context.Context, userID uuid.UUID, code string) (*domain.GroupWithMembership, error)
}

// Service is the authentication use-case layer.
type Service struct {
	users  domain.UserRepo
	auth   domain.AuthRepo
	tx     domain.TxManager
	tokens *Tokens
	google GoogleVerifier
	joiner Joiner
	clock  clock.Clock
	log    *slog.Logger
}

// NewService wires the service. joiner may be nil in tests.
func NewService(users domain.UserRepo, authRepo domain.AuthRepo, tx domain.TxManager, tokens *Tokens, google GoogleVerifier, joiner Joiner, clk clock.Clock, log *slog.Logger) *Service {
	return &Service{users: users, auth: authRepo, tx: tx, tokens: tokens, google: google, joiner: joiner, clock: clk, log: log}
}

// DeviceInfo describes the client creating a session.
type DeviceInfo struct {
	Platform domain.DevicePlatform
	Name     *string
}

// Session is what a client receives after any successful authentication.
type Session struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
	User            *domain.User
	DeviceID        uuid.UUID
	Joined          *domain.GroupWithMembership
}

// RegisterInput is the light registration form.
type RegisterInput struct {
	Name       string
	InviteCode string
	Locale     string
	Timezone   string
	Device     DeviceInfo
}

// Register creates a level-1 account and, optionally, joins a group.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*Session, error) {
	name, err := normalizeName(in.Name)
	if err != nil {
		return nil, err
	}
	var session *Session
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		user, err := s.users.Create(ctx, domain.CreateUserParams{
			ID:       ids.New(),
			Name:     name,
			Locale:   orDefault(in.Locale, "ru"),
			Timezone: orDefault(in.Timezone, "UTC"),
		})
		if err != nil {
			return err
		}
		session, err = s.openSession(ctx, user, in.Device)
		if err != nil {
			return err
		}
		if code := strings.TrimSpace(in.InviteCode); code != "" && s.joiner != nil {
			joined, err := s.joiner.Join(ctx, user.ID, code)
			if err != nil {
				return err
			}
			session.Joined = joined
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Login authenticates a secured account with email and password.
func (s *Service) Login(ctx context.Context, email, password string, device DeviceInfo) (*Session, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
		}
		return nil, err
	}
	if hash == "" {
		return nil, fmt.Errorf("%w: this account has no password; use another sign-in method", domain.ErrUnauthorized)
	}
	ok, err := VerifyPassword(hash, password)
	if err != nil {
		return nil, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
	}
	var session *Session
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		session, err = s.openSession(ctx, user, device)
		return err
	})
	return session, err
}

// Refresh rotates a refresh token. Reuse of an already-rotated token is
// treated as theft: every session of the user is revoked.
func (s *Service) Refresh(ctx context.Context, rawRefresh string) (*Session, error) {
	hash := s.tokens.HashRefresh(strings.TrimSpace(rawRefresh))

	// Validate outside the rotation transaction: a reuse must revoke every
	// session and that revocation has to survive the error we return.
	stored, err := s.auth.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: unknown refresh token", domain.ErrUnauthorized)
		}
		return nil, err
	}
	if stored.RevokedAt != nil {
		s.log.Warn("refresh token reuse detected; revoking all sessions", "user_id", stored.UserID)
		if err := s.auth.RevokeAllRefreshTokens(ctx, stored.UserID); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: refresh token already used", domain.ErrUnauthorized)
	}
	if !stored.Active(s.clock.Now()) {
		return nil, fmt.Errorf("%w: refresh token expired", domain.ErrUnauthorized)
	}

	var session *Session
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		user, err := s.users.GetByID(ctx, stored.UserID)
		if err != nil {
			return fmt.Errorf("%w: %v", domain.ErrUnauthorized, err)
		}
		deviceID := uuid.Nil
		if stored.DeviceID != nil {
			deviceID = *stored.DeviceID
			_ = s.auth.TouchDevice(ctx, deviceID)
		}
		raw, newHash, exp, err := s.tokens.NewRefresh()
		if err != nil {
			return err
		}
		created, err := s.auth.CreateRefreshToken(ctx, domain.RefreshToken{
			ID: ids.New(), UserID: user.ID, DeviceID: stored.DeviceID, TokenHash: newHash, ExpiresAt: exp,
		})
		if err != nil {
			return err
		}
		if err := s.auth.RevokeRefreshToken(ctx, stored.ID, &created.ID); err != nil {
			return err
		}
		access, accessExp, err := s.tokens.IssueAccess(user.ID, deviceID)
		if err != nil {
			return err
		}
		session = &Session{AccessToken: access, AccessExpiresAt: accessExp, RefreshToken: raw, User: user, DeviceID: deviceID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Logout revokes a refresh token. Unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, rawRefresh string) error {
	stored, err := s.auth.GetRefreshTokenByHash(ctx, s.tokens.HashRefresh(strings.TrimSpace(rawRefresh)))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.auth.RevokeRefreshToken(ctx, stored.ID, nil)
}

// LogoutAll revokes every session of the user.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.auth.RevokeAllRefreshTokens(ctx, userID)
}

// Google signs in with a Google ID token. When currentUser is set the Google
// identity is linked to that account; otherwise an existing linked account is
// logged in, or a new secured account is created.
func (s *Service) Google(ctx context.Context, idToken string, currentUser *uuid.UUID, device DeviceInfo) (*Session, error) {
	if s.google == nil {
		return nil, fmt.Errorf("%w: google sign-in is not configured", domain.ErrInvalid)
	}
	claims, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return nil, err
	}
	var session *Session
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		identity, err := s.auth.GetIdentity(ctx, domain.ProviderGoogle, claims.Subject)
		switch {
		case err == nil:
			if currentUser != nil && *currentUser != identity.UserID {
				return domain.Conflict("this Google account is linked to another user")
			}
			user, err := s.users.GetByID(ctx, identity.UserID)
			if err != nil {
				return err
			}
			session, err = s.openSession(ctx, user, device)
			return err
		case errors.Is(err, domain.ErrNotFound):
			// continue below
		default:
			return err
		}

		var user *domain.User
		if currentUser != nil {
			user, err = s.users.GetByID(ctx, *currentUser)
			if err != nil {
				return err
			}
		} else {
			if claims.Email != "" && claims.EmailVerified {
				if _, _, err := s.users.GetByEmail(ctx, claims.Email); err == nil {
					return domain.Conflict("an account with this email exists; sign in and link Google in settings")
				} else if !errors.Is(err, domain.ErrNotFound) {
					return err
				}
			}
			name, err := normalizeName(claims.Name)
			if err != nil {
				name = "Пользователь"
			}
			var email *string
			if claims.Email != "" && claims.EmailVerified {
				e := strings.ToLower(claims.Email)
				email = &e
			}
			user, err = s.users.Create(ctx, domain.CreateUserParams{
				ID: ids.New(), Name: name, Email: email, Locale: "ru", Timezone: "UTC", Secured: true,
			})
			if err != nil {
				return err
			}
		}
		var email *string
		if claims.Email != "" {
			e := strings.ToLower(claims.Email)
			email = &e
		}
		if _, err := s.auth.CreateIdentity(ctx, domain.AuthIdentity{
			ID: ids.New(), UserID: user.ID, Provider: domain.ProviderGoogle, ProviderUserID: claims.Subject, Email: email,
		}); err != nil {
			return err
		}
		if err := s.users.MarkSecured(ctx, user.ID); err != nil {
			return err
		}
		user, err = s.users.GetByID(ctx, user.ID)
		if err != nil {
			return err
		}
		session, err = s.openSession(ctx, user, device)
		return err
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// SetCredentials adds an email and/or password, upgrading the account to L2.
func (s *Service) SetCredentials(ctx context.Context, userID uuid.UUID, email, password *string) (*domain.User, error) {
	var emailPtr, hashPtr *string
	if email != nil {
		e := strings.TrimSpace(strings.ToLower(*email))
		if _, err := mail.ParseAddress(e); err != nil || strings.ContainsAny(e, " <>") {
			return nil, domain.Invalid("email", "invalid email address")
		}
		emailPtr = &e
	}
	if password != nil {
		if utf8.RuneCountInString(*password) < 8 {
			return nil, domain.Invalid("password", "must be at least 8 characters")
		}
		h, err := HashPassword(*password)
		if err != nil {
			return nil, err
		}
		hashPtr = &h
	}
	if emailPtr == nil && hashPtr == nil {
		return nil, domain.Invalid("", "provide email and/or password")
	}
	user, err := s.users.SetCredentials(ctx, userID, emailPtr, hashPtr)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateProfile changes display settings.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, name, locale, timezone string, settings json.RawMessage) (*domain.User, error) {
	current, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if name != "" {
		if name, err = normalizeName(name); err != nil {
			return nil, err
		}
	} else {
		name = current.Name
	}
	locale = orDefault(locale, current.Locale)
	timezone = orDefault(timezone, current.Timezone)
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, domain.Invalid("timezone", "unknown timezone")
	}
	if len(settings) == 0 {
		settings = current.Settings
	} else if !json.Valid(settings) {
		return nil, domain.Invalid("settings", "must be a JSON object")
	}
	return s.users.UpdateProfile(ctx, userID, name, locale, timezone, settings)
}

// Devices lists the user's devices.
func (s *Service) Devices(ctx context.Context, userID uuid.UUID) ([]domain.Device, error) {
	return s.auth.ListDevices(ctx, userID)
}

// RegisterPush stores a push token/subscription for a device.
func (s *Service) RegisterPush(ctx context.Context, userID, deviceID uuid.UUID, provider domain.PushProvider, token *string, subscription json.RawMessage) (*domain.Device, error) {
	switch provider {
	case domain.PushExpo:
		if token == nil || strings.TrimSpace(*token) == "" {
			return nil, domain.Invalid("push_token", "required for expo")
		}
	case domain.PushWebPush:
		if len(subscription) == 0 || !json.Valid(subscription) {
			return nil, domain.Invalid("push_subscription", "required for webpush")
		}
	default:
		return nil, domain.Invalid("push_provider", "unknown provider")
	}
	return s.auth.UpdateDevicePush(ctx, deviceID, userID, &provider, token, subscription)
}

// RemoveDevice deletes a device (its refresh tokens lose their device link).
func (s *Service) RemoveDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	return s.auth.DeleteDevice(ctx, deviceID, userID)
}

// openSession creates a device record and issues tokens.
func (s *Service) openSession(ctx context.Context, user *domain.User, info DeviceInfo) (*Session, error) {
	platform := info.Platform
	if platform == "" {
		platform = domain.PlatformWeb
	}
	device, err := s.auth.CreateDevice(ctx, domain.Device{ID: ids.New(), UserID: user.ID, Platform: platform, Name: info.Name})
	if err != nil {
		return nil, err
	}
	raw, hash, exp, err := s.tokens.NewRefresh()
	if err != nil {
		return nil, err
	}
	if _, err := s.auth.CreateRefreshToken(ctx, domain.RefreshToken{
		ID: ids.New(), UserID: user.ID, DeviceID: &device.ID, TokenHash: hash, ExpiresAt: exp,
	}); err != nil {
		return nil, err
	}
	access, accessExp, err := s.tokens.IssueAccess(user.ID, device.ID)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: access, AccessExpiresAt: accessExp, RefreshToken: raw, User: user, DeviceID: device.ID}, nil
}

func normalizeName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	n := utf8.RuneCountInString(name)
	if n < 1 {
		return "", domain.Invalid("name", "required")
	}
	if n > 80 {
		return "", domain.Invalid("name", "must be at most 80 characters")
	}
	return name, nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
