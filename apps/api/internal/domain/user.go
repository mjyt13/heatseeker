package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// GlobalRole is a platform-wide role, independent of any group.
type GlobalRole string

// Global roles.
const (
	GlobalRoleUser       GlobalRole = "USER"
	GlobalRoleSuperadmin GlobalRole = "SUPERADMIN"
)

// User is a person using the platform. Only Name is required: accounts start
// as "light" (level L1) and become "secured" (L2) once credentials are added.
type User struct {
	ID          uuid.UUID
	Name        string
	Email       *string
	Locale      string
	Timezone    string
	AvatarKey   *string
	GlobalRole  GlobalRole
	Settings    json.RawMessage
	SecuredAt   *time.Time
	HasPassword bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Secured reports whether the account has credentials (password, email or an
// external identity) and therefore satisfies level L2.
func (u *User) Secured() bool { return u.SecuredAt != nil }

// IdentityProvider is an external login provider.
type IdentityProvider string

// Supported providers.
const (
	ProviderGoogle IdentityProvider = "GOOGLE"
	ProviderApple  IdentityProvider = "APPLE"
)

// AuthIdentity links a user to an external provider account.
type AuthIdentity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       IdentityProvider
	ProviderUserID string
	Email          *string
	CreatedAt      time.Time
}

// DevicePlatform is where a device runs.
type DevicePlatform string

// Platforms.
const (
	PlatformIOS     DevicePlatform = "IOS"
	PlatformAndroid DevicePlatform = "ANDROID"
	PlatformWeb     DevicePlatform = "WEB"
)

// PushProvider delivers push notifications to a device.
type PushProvider string

// Push providers.
const (
	PushExpo    PushProvider = "EXPO"
	PushWebPush PushProvider = "WEBPUSH"
)

// Device is a client installation that holds a session and may receive push.
type Device struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Platform         DevicePlatform
	Name             *string
	PushProvider     *PushProvider
	PushToken        *string
	PushSubscription json.RawMessage
	Enabled          bool
	LastSeenAt       time.Time
	CreatedAt        time.Time
}

// RefreshToken is an opaque, rotating long-lived credential. Only its hash is
// stored.
type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DeviceID   *uuid.UUID
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
	CreatedAt  time.Time
}

// Active reports whether the token can still be exchanged at time now.
func (t *RefreshToken) Active(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
