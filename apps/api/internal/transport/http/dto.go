package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
)

// UserDTO is the caller's own profile.
type UserDTO struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Email       *string         `json:"email,omitempty"`
	Locale      string          `json:"locale"`
	Timezone    string          `json:"timezone"`
	AvatarKey   *string         `json:"avatar_key,omitempty"`
	GlobalRole  string          `json:"global_role" enum:"USER,SUPERADMIN"`
	Secured     bool            `json:"secured" doc:"Аккаунт защищён паролем/email/Google (уровень L2)."`
	HasPassword bool            `json:"has_password"`
	Settings    json.RawMessage `json:"settings"`
	CreatedAt   time.Time       `json:"created_at"`
}

func toUserDTO(u *domain.User) UserDTO {
	return UserDTO{
		ID: u.ID, Name: u.Name, Email: u.Email, Locale: u.Locale, Timezone: u.Timezone, AvatarKey: u.AvatarKey,
		GlobalRole: string(u.GlobalRole), Secured: u.Secured(), HasPassword: u.HasPassword,
		Settings: nonEmptyJSON(u.Settings), CreatedAt: u.CreatedAt,
	}
}

// PublicUserDTO is what other members see.
type PublicUserDTO struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	AvatarKey *string   `json:"avatar_key,omitempty"`
	// Extended fields are present only for admins.
	Email     *string    `json:"email,omitempty"`
	Secured   *bool      `json:"secured,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

func toPublicUserDTO(u *domain.User, extended bool) PublicUserDTO {
	dto := PublicUserDTO{ID: u.ID, Name: u.Name, AvatarKey: u.AvatarKey}
	if extended {
		secured := u.Secured()
		created := u.CreatedAt
		dto.Email = u.Email
		dto.Secured = &secured
		dto.CreatedAt = &created
	}
	return dto
}

// SessionDTO is returned by every authentication endpoint.
type SessionDTO struct {
	AccessToken     string                  `json:"access_token"`
	AccessExpiresAt time.Time               `json:"access_expires_at"`
	RefreshToken    string                  `json:"refresh_token"`
	DeviceID        uuid.UUID               `json:"device_id"`
	User            UserDTO                 `json:"user"`
	Joined          *GroupWithMembershipDTO `json:"joined,omitempty" doc:"Группа, к которой присоединились по коду при регистрации."`
}

func toSessionDTO(s *auth.Session) SessionDTO {
	dto := SessionDTO{
		AccessToken: s.AccessToken, AccessExpiresAt: s.AccessExpiresAt, RefreshToken: s.RefreshToken,
		DeviceID: s.DeviceID, User: toUserDTO(s.User),
	}
	if s.Joined != nil {
		j := toGroupWithMembershipDTO(*s.Joined)
		dto.Joined = &j
	}
	return dto
}

// GroupDTO describes a group.
type GroupDTO struct {
	ID         uuid.UUID       `json:"id"`
	Name       string          `json:"name"`
	Slug       string          `json:"slug"`
	Kind       string          `json:"kind" enum:"MASTERS,DPO,OTHER"`
	JoinPolicy string          `json:"join_policy" enum:"OPEN,INVITE,APPROVAL"`
	JoinCode   *string         `json:"join_code,omitempty" doc:"Постоянный код для вступления (политика OPEN)."`
	PublicRead bool            `json:"public_read"`
	MediaMode  string          `json:"media_mode" enum:"LINK,CACHE,IMPORT"`
	Settings   json.RawMessage `json:"settings"`
	LastSeq    int64           `json:"last_seq" doc:"Последний seq журнала событий группы."`
	CreatedAt  time.Time       `json:"created_at"`
	ArchivedAt *time.Time      `json:"archived_at,omitempty"`
}

func toGroupDTO(g *domain.Group) GroupDTO {
	return GroupDTO{
		ID: g.ID, Name: g.Name, Slug: g.Slug, Kind: string(g.Kind), JoinPolicy: string(g.JoinPolicy), JoinCode: g.JoinCode,
		PublicRead: g.PublicRead, MediaMode: string(g.MediaMode), Settings: nonEmptyJSON(g.Settings), LastSeq: g.LastSeq,
		CreatedAt: g.CreatedAt, ArchivedAt: g.ArchivedAt,
	}
}

// MembershipDTO describes a user's roles in a group.
type MembershipDTO struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	GroupID     uuid.UUID `json:"group_id"`
	Roles       []string  `json:"roles"`
	Status      string    `json:"status" enum:"ACTIVE,PENDING,BANNED"`
	JoinedVia   string    `json:"joined_via"`
	JoinedAt    time.Time `json:"joined_at"`
	Permissions []string  `json:"permissions,omitempty" doc:"Действия, доступные этому участнику (только для собственного членства)."`
}

func toMembershipDTO(m *domain.Membership, withPermissions bool) MembershipDTO {
	dto := MembershipDTO{
		ID: m.ID, UserID: m.UserID, GroupID: m.GroupID, Roles: domain.RolesToStrings(m.Roles),
		Status: string(m.Status), JoinedVia: string(m.JoinedVia), JoinedAt: m.JoinedAt,
	}
	if withPermissions {
		for _, a := range authz.Allowed(m.Roles) {
			dto.Permissions = append(dto.Permissions, string(a))
		}
	}
	return dto
}

// GroupWithMembershipDTO pairs a group with the caller's membership.
type GroupWithMembershipDTO struct {
	Group      GroupDTO      `json:"group"`
	Membership MembershipDTO `json:"membership"`
}

func toGroupWithMembershipDTO(gm domain.GroupWithMembership) GroupWithMembershipDTO {
	return GroupWithMembershipDTO{Group: toGroupDTO(&gm.Group), Membership: toMembershipDTO(&gm.Membership, true)}
}

// MemberDTO is a member as seen in the member list.
type MemberDTO struct {
	User       PublicUserDTO `json:"user"`
	Membership MembershipDTO `json:"membership"`
}

// InviteDTO describes an invite.
type InviteDTO struct {
	ID        uuid.UUID  `json:"id"`
	GroupID   uuid.UUID  `json:"group_id"`
	Code      string     `json:"code"`
	Roles     []string   `json:"roles"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	MaxUses   *int32     `json:"max_uses,omitempty"`
	Uses      int32      `json:"uses"`
	CreatedBy uuid.UUID  `json:"created_by"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func toInviteDTO(i *domain.Invite) InviteDTO {
	return InviteDTO{
		ID: i.ID, GroupID: i.GroupID, Code: i.Code, Roles: domain.RolesToStrings(i.Roles), ExpiresAt: i.ExpiresAt,
		MaxUses: i.MaxUses, Uses: i.Uses, CreatedBy: i.CreatedBy, RevokedAt: i.RevokedAt, CreatedAt: i.CreatedAt,
	}
}

// SubjectDTO describes a subject.
type SubjectDTO struct {
	ID             uuid.UUID  `json:"id"`
	GroupID        uuid.UUID  `json:"group_id"`
	Name           string     `json:"name"`
	ShortName      *string    `json:"short_name,omitempty"`
	Teacher        *string    `json:"teacher,omitempty"`
	TeacherContact *string    `json:"teacher_contact,omitempty"`
	Color          *string    `json:"color,omitempty"`
	Semester       *string    `json:"semester,omitempty"`
	Aliases        []string   `json:"aliases"`
	SortOrder      int32      `json:"sort_order"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ArchivedAt     *time.Time `json:"archived_at,omitempty"`
}

func toSubjectDTO(s *domain.Subject) SubjectDTO {
	return SubjectDTO{
		ID: s.ID, GroupID: s.GroupID, Name: s.Name, ShortName: s.ShortName, Teacher: s.Teacher, TeacherContact: s.TeacherContact,
		Color: s.Color, Semester: s.Semester, Aliases: s.Aliases, SortOrder: s.SortOrder, CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt, ArchivedAt: s.ArchivedAt,
	}
}

// TagDTO describes a tag.
type TagDTO struct {
	ID        uuid.UUID  `json:"id"`
	GroupID   uuid.UUID  `json:"group_id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	Color     *string    `json:"color,omitempty"`
	Kind      string     `json:"kind" enum:"SUBJECT,TOPIC,TYPE,SYSTEM,CUSTOM"`
	SubjectID *uuid.UUID `json:"subject_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func toTagDTO(t *domain.Tag) TagDTO {
	return TagDTO{ID: t.ID, GroupID: t.GroupID, Name: t.Name, Slug: t.Slug, Color: t.Color, Kind: string(t.Kind), SubjectID: t.SubjectID, CreatedAt: t.CreatedAt}
}

// QuickTagDTO is a home-screen chip.
type QuickTagDTO struct {
	Key       string     `json:"key" doc:"Стабильный ключ: system:<name> или subject:<id>."`
	Label     string     `json:"label" doc:"Для system — ключ i18n, для предметов — название."`
	Color     *string    `json:"color,omitempty"`
	Kind      string     `json:"kind"`
	TagID     *uuid.UUID `json:"tag_id,omitempty"`
	SubjectID *uuid.UUID `json:"subject_id,omitempty"`
	Order     int        `json:"order"`
}

func toQuickTagDTO(q domain.QuickTag) QuickTagDTO {
	return QuickTagDTO{Key: q.Key, Label: q.Label, Color: q.Color, Kind: string(q.Kind), TagID: q.TagID, SubjectID: q.Subject, Order: q.Order}
}

// EventDTO is one entry of the group log.
type EventDTO struct {
	ID         uuid.UUID       `json:"id"`
	GroupID    uuid.UUID       `json:"group_id"`
	Seq        int64           `json:"seq"`
	Kind       string          `json:"kind"`
	ActorID    *uuid.UUID      `json:"actor_id,omitempty"`
	EntityType string          `json:"entity_type"`
	EntityID   *uuid.UUID      `json:"entity_id,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  time.Time       `json:"created_at"`
}

func toEventDTO(e *domain.Event) EventDTO {
	return EventDTO{ID: e.ID, GroupID: e.GroupID, Seq: e.Seq, Kind: string(e.Kind), ActorID: e.ActorID, EntityType: e.EntityType, EntityID: e.EntityID, Payload: nonEmptyJSON(e.Payload), CreatedAt: e.CreatedAt}
}

// DeviceDTO describes a client installation.
type DeviceDTO struct {
	ID           uuid.UUID `json:"id"`
	Platform     string    `json:"platform" enum:"IOS,ANDROID,WEB"`
	Name         *string   `json:"name,omitempty"`
	PushProvider *string   `json:"push_provider,omitempty"`
	PushEnabled  bool      `json:"push_enabled"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func toDeviceDTO(d *domain.Device) DeviceDTO {
	var provider *string
	if d.PushProvider != nil {
		p := string(*d.PushProvider)
		provider = &p
	}
	return DeviceDTO{ID: d.ID, Platform: string(d.Platform), Name: d.Name, PushProvider: provider, PushEnabled: d.Enabled && d.PushProvider != nil, LastSeenAt: d.LastSeenAt, CreatedAt: d.CreatedAt}
}

func nonEmptyJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

func parseID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, domain.Invalid(field, "must be a UUID")
	}
	return id, nil
}

func parseRoles(values []string) ([]domain.Role, error) {
	out := make([]domain.Role, 0, len(values))
	for _, v := range values {
		r, ok := domain.ParseRole(v)
		if !ok {
			return nil, domain.Invalid("roles", "unknown role "+v)
		}
		out = append(out, r)
	}
	return out, nil
}
