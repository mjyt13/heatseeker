package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TxManager runs fn inside a database transaction. Repositories called with the
// derived context participate in it. Functions registered via AfterCommit run
// once the transaction commits (or immediately when no transaction is open).
type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
	AfterCommit(ctx context.Context, fn func())
}

// CreateUserParams is the input for UserRepo.Create.
type CreateUserParams struct {
	ID           uuid.UUID
	Name         string
	Email        *string
	PasswordHash *string
	Locale       string
	Timezone     string
	Secured      bool
}

// UserRepo persists users.
type UserRepo interface {
	Create(ctx context.Context, p CreateUserParams) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	// GetByEmail also returns the password hash (empty when none) for login.
	GetByEmail(ctx context.Context, email string) (*User, string, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, name, locale, timezone string, settings json.RawMessage) (*User, error)
	SetCredentials(ctx context.Context, id uuid.UUID, email, passwordHash *string) (*User, error)
	MarkSecured(ctx context.Context, id uuid.UUID) error
}

// AuthRepo persists identities, devices and refresh tokens.
type AuthRepo interface {
	CreateIdentity(ctx context.Context, id AuthIdentity) (*AuthIdentity, error)
	GetIdentity(ctx context.Context, provider IdentityProvider, providerUserID string) (*AuthIdentity, error)
	ListIdentities(ctx context.Context, userID uuid.UUID) ([]AuthIdentity, error)

	CreateDevice(ctx context.Context, d Device) (*Device, error)
	GetDevice(ctx context.Context, id, userID uuid.UUID) (*Device, error)
	ListDevices(ctx context.Context, userID uuid.UUID) ([]Device, error)
	TouchDevice(ctx context.Context, id uuid.UUID) error
	UpdateDevicePush(ctx context.Context, id, userID uuid.UUID, provider *PushProvider, token *string, subscription json.RawMessage) (*Device, error)
	DeleteDevice(ctx context.Context, id, userID uuid.UUID) error

	CreateRefreshToken(ctx context.Context, t RefreshToken) (*RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error
	RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredRefreshTokens(ctx context.Context) (int64, error)
}

// CreateGroupParams is the input for GroupRepo.Create.
type CreateGroupParams struct {
	ID         uuid.UUID
	Name       string
	Slug       string
	Kind       GroupKind
	JoinPolicy JoinPolicy
	JoinCode   *string
	MediaMode  MediaMode
	Settings   json.RawMessage
	CreatedBy  uuid.UUID
}

// UpdateGroupParams is the input for GroupRepo.Update.
type UpdateGroupParams struct {
	ID         uuid.UUID
	Name       string
	Kind       GroupKind
	JoinPolicy JoinPolicy
	PublicRead bool
	MediaMode  MediaMode
	Settings   json.RawMessage
}

// GroupRepo persists groups.
type GroupRepo interface {
	Create(ctx context.Context, p CreateGroupParams) (*Group, error)
	Get(ctx context.Context, id uuid.UUID) (*Group, error)
	GetBySlug(ctx context.Context, slug string) (*Group, error)
	GetByJoinCode(ctx context.Context, code string) (*Group, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]GroupWithMembership, error)
	Update(ctx context.Context, p UpdateGroupParams) (*Group, error)
	SetJoinCode(ctx context.Context, id uuid.UUID, code *string) (*Group, error)
	Archive(ctx context.Context, id uuid.UUID) error
	NextSeq(ctx context.Context, id uuid.UUID) (int64, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
}

// MembershipRepo persists memberships.
type MembershipRepo interface {
	Create(ctx context.Context, m Membership) (*Membership, error)
	Get(ctx context.Context, userID, groupID uuid.UUID) (*Membership, error)
	List(ctx context.Context, groupID uuid.UUID) ([]Member, error)
	UpdateRoles(ctx context.Context, userID, groupID uuid.UUID, roles []Role) (*Membership, error)
	UpdateStatus(ctx context.Context, userID, groupID uuid.UUID, status MembershipStatus) (*Membership, error)
	CountActive(ctx context.Context, groupID uuid.UUID) (int64, error)
	ListActiveUserIDs(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error)
}

// InviteRepo persists invites.
type InviteRepo interface {
	Create(ctx context.Context, i Invite) (*Invite, error)
	GetByCode(ctx context.Context, code string) (*Invite, error)
	ListByGroup(ctx context.Context, groupID uuid.UUID) ([]Invite, error)
	IncrementUses(ctx context.Context, id uuid.UUID) (*Invite, error)
	Revoke(ctx context.Context, id, groupID uuid.UUID) error
}

// SubjectRepo persists subjects.
type SubjectRepo interface {
	Create(ctx context.Context, s Subject) (*Subject, error)
	Get(ctx context.Context, id, groupID uuid.UUID) (*Subject, error)
	List(ctx context.Context, groupID uuid.UUID, includeArchived bool) ([]Subject, error)
	Update(ctx context.Context, s Subject) (*Subject, error)
	Archive(ctx context.Context, id, groupID uuid.UUID) error
	Restore(ctx context.Context, id, groupID uuid.UUID) error
	AddAliases(ctx context.Context, id, groupID uuid.UUID, aliases []string) error
}

// TagRepo persists tags.
type TagRepo interface {
	Create(ctx context.Context, t Tag) (*Tag, error)
	Get(ctx context.Context, id, groupID uuid.UUID) (*Tag, error)
	GetBySlug(ctx context.Context, groupID uuid.UUID, slug string) (*Tag, error)
	GetBySubject(ctx context.Context, subjectID uuid.UUID) (*Tag, error)
	List(ctx context.Context, groupID uuid.UUID) ([]Tag, error)
	Update(ctx context.Context, id, groupID uuid.UUID, name, slug string, color *string) (*Tag, error)
	Delete(ctx context.Context, id, groupID uuid.UUID) error
}

// EventRepo persists the group event log.
type EventRepo interface {
	Insert(ctx context.Context, e Event) (*Event, error)
	ListSince(ctx context.Context, groupID uuid.UUID, since int64, limit int32) ([]Event, error)
	ListAudit(ctx context.Context, groupID uuid.UUID, beforeSeq *int64, limit int32) ([]Event, error)
	OldestSeq(ctx context.Context, groupID uuid.UUID) (int64, error)
	DeleteBefore(ctx context.Context, t time.Time) (int64, error)
}
