package domain

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role is a per-group role. A membership carries a set of roles; permissions
// are the union of what each role allows (see package authz).
type Role string

// Roles.
const (
	RoleOwner     Role = "OWNER"
	RoleAdmin     Role = "ADMIN"
	RoleModerator Role = "MODERATOR"
	RoleHeadman   Role = "HEADMAN" // староста
	RoleStudent   Role = "STUDENT"
	RoleGuest     Role = "GUEST"
)

// AllRoles lists every role in display order.
var AllRoles = []Role{RoleOwner, RoleAdmin, RoleModerator, RoleHeadman, RoleStudent, RoleGuest}

// ParseRole validates a role string.
func ParseRole(s string) (Role, bool) {
	r := Role(strings.ToUpper(strings.TrimSpace(s)))
	return r, slices.Contains(AllRoles, r)
}

// GroupKind distinguishes cohorts that share the same logic but different defaults.
type GroupKind string

// Group kinds.
const (
	GroupKindMasters GroupKind = "MASTERS"
	GroupKindDPO     GroupKind = "DPO"
	GroupKindOther   GroupKind = "OTHER"
)

// JoinPolicy controls how people enter a group.
type JoinPolicy string

// Join policies. APPROVAL is modelled but not offered in the first release.
const (
	JoinOpen     JoinPolicy = "OPEN"
	JoinInvite   JoinPolicy = "INVITE"
	JoinApproval JoinPolicy = "APPROVAL"
)

// MediaMode selects how material files are served (see docs/adr/0003).
type MediaMode string

// Media modes.
const (
	MediaLink   MediaMode = "LINK"
	MediaCache  MediaMode = "CACHE"
	MediaImport MediaMode = "IMPORT"
)

// Group is the tenant: every other entity belongs to exactly one group.
type Group struct {
	ID         uuid.UUID
	Name       string
	Slug       string
	Kind       GroupKind
	JoinPolicy JoinPolicy
	JoinCode   *string
	PublicRead bool
	MediaMode  MediaMode
	Settings   json.RawMessage
	LastSeq    int64
	CreatedBy  uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ArchivedAt *time.Time
}

// MembershipStatus is the state of a person inside a group.
type MembershipStatus string

// Membership statuses.
const (
	MembershipActive  MembershipStatus = "ACTIVE"
	MembershipPending MembershipStatus = "PENDING"
	MembershipBanned  MembershipStatus = "BANNED"
)

// JoinedVia records how the membership was created.
type JoinedVia string

// Join channels.
const (
	JoinedViaLink     JoinedVia = "LINK"
	JoinedViaInvite   JoinedVia = "INVITE"
	JoinedViaApproval JoinedVia = "APPROVAL"
	JoinedViaCreator  JoinedVia = "CREATOR"
)

// Membership ties a user to a group with a set of roles.
type Membership struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	GroupID   uuid.UUID
	Roles     []Role
	Status    MembershipStatus
	JoinedVia JoinedVia
	JoinedAt  time.Time
}

// HasRole reports whether the membership includes r.
func (m *Membership) HasRole(r Role) bool { return slices.Contains(m.Roles, r) }

// IsActive reports whether the member may act in the group.
func (m *Membership) IsActive() bool { return m != nil && m.Status == MembershipActive }

// GroupWithMembership pairs a group with the caller's membership in it.
type GroupWithMembership struct {
	Group      Group
	Membership Membership
}

// GroupSearchHit is a group found by name, as seen by the searching user.
type GroupSearchHit struct {
	Group       Group
	MemberCount int64
	IsMember    bool
}

// Member is a membership joined with the user's public profile.
type Member struct {
	Membership Membership
	User       User
}

// Invite is a code that grants membership with preset roles.
type Invite struct {
	ID        uuid.UUID
	GroupID   uuid.UUID
	Code      string
	Roles     []Role
	ExpiresAt *time.Time
	MaxUses   *int32
	Uses      int32
	CreatedBy uuid.UUID
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Usable reports whether the invite can still be redeemed at time now.
func (i *Invite) Usable(now time.Time) bool {
	if i.RevokedAt != nil {
		return false
	}
	if i.ExpiresAt != nil && !now.Before(*i.ExpiresAt) {
		return false
	}
	if i.MaxUses != nil && i.Uses >= *i.MaxUses {
		return false
	}
	return true
}

// RolesToStrings converts roles for storage.
func RolesToStrings(roles []Role) []string {
	out := make([]string, len(roles))
	for i, r := range roles {
		out[i] = string(r)
	}
	return out
}

// RolesFromStrings converts stored roles, dropping unknown values.
func RolesFromStrings(values []string) []Role {
	out := make([]Role, 0, len(values))
	for _, v := range values {
		if r, ok := ParseRole(v); ok {
			out = append(out, r)
		}
	}
	return out
}
