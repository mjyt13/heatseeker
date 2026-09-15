// Package groups implements group lifecycle, membership, roles and invites.
package groups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
	"heatseeker/api/internal/platform/slug"
)

// Defaults for new groups, taken from configuration.
type Defaults struct {
	JoinPolicy       domain.JoinPolicy
	MediaMode        domain.MediaMode
	InviteTTLDays    int
	PublicReadSwitch bool // whether public_read may be turned on at all
}

// Service is the groups use-case layer.
type Service struct {
	groups   domain.GroupRepo
	members  domain.MembershipRepo
	invites  domain.InviteRepo
	access   *access.Service
	events   *events.Publisher
	tx       domain.TxManager
	defaults Defaults
	clock    clock.Clock
}

// NewService wires the service.
func NewService(groups domain.GroupRepo, members domain.MembershipRepo, invites domain.InviteRepo, acc *access.Service, pub *events.Publisher, tx domain.TxManager, defaults Defaults, clk clock.Clock) *Service {
	return &Service{groups: groups, members: members, invites: invites, access: acc, events: pub, tx: tx, defaults: defaults, clock: clk}
}

// CreateInput is the form for a new group.
type CreateInput struct {
	Name string
	Kind domain.GroupKind
}

// Create makes a group; the creator becomes OWNER and STUDENT.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (*domain.GroupWithMembership, error) {
	name := strings.Join(strings.Fields(in.Name), " ")
	if n := utf8.RuneCountInString(name); n < 2 || n > 80 {
		return nil, domain.Invalid("name", "must be 2–80 characters")
	}
	kind := in.Kind
	if kind == "" {
		kind = domain.GroupKindMasters
	}
	if !slices.Contains([]domain.GroupKind{domain.GroupKindMasters, domain.GroupKindDPO, domain.GroupKindOther}, kind) {
		return nil, domain.Invalid("kind", "unknown group kind")
	}
	if _, err := s.access.User(ctx, userID); err != nil {
		return nil, err
	}

	var out *domain.GroupWithMembership
	err := s.tx.RunInTx(ctx, func(ctx context.Context) error {
		slugValue, err := s.uniqueSlug(ctx, name)
		if err != nil {
			return err
		}
		code := ids.Code(10)
		settings := json.RawMessage(`{}`)
		if kind == domain.GroupKindDPO {
			settings = json.RawMessage(`{"notifications_default":"off"}`)
		}
		group, err := s.groups.Create(ctx, domain.CreateGroupParams{
			ID: ids.New(), Name: name, Slug: slugValue, Kind: kind,
			JoinPolicy: s.defaults.JoinPolicy, JoinCode: &code, MediaMode: s.defaults.MediaMode,
			Settings: settings, CreatedBy: userID,
		})
		if err != nil {
			return err
		}
		m, err := s.members.Create(ctx, domain.Membership{
			ID: ids.New(), UserID: userID, GroupID: group.ID,
			Roles: []domain.Role{domain.RoleOwner, domain.RoleStudent}, Status: domain.MembershipActive, JoinedVia: domain.JoinedViaCreator,
		})
		if err != nil {
			return err
		}
		if err := s.emit(ctx, group.ID, domain.EventGroupCreated, &userID, "group", &group.ID, map[string]any{"name": group.Name}); err != nil {
			return err
		}
		if err := s.emit(ctx, group.ID, domain.EventMemberJoined, &userID, "membership", &m.ID, map[string]any{"user_id": userID, "roles": m.Roles}); err != nil {
			return err
		}
		out = &domain.GroupWithMembership{Group: *group, Membership: *m}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListMine returns the caller's active groups.
func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]domain.GroupWithMembership, error) {
	return s.groups.ListForUser(ctx, userID)
}

// Get returns a group the caller belongs to together with their membership.
func (s *Service) Get(ctx context.Context, userID, groupID uuid.UUID) (*access.Actor, error) {
	actor, err := s.access.Actor(ctx, userID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	return actor, nil
}

// Preview describes what a join code leads to, for the "join group?" screen.
type Preview struct {
	Group       domain.Group
	Roles       []domain.Role
	MemberCount int64
	ViaInvite   bool
}

// Preview resolves a code without joining. No authentication required.
func (s *Service) Preview(ctx context.Context, code string) (*Preview, error) {
	group, roles, invite, err := s.resolveCode(ctx, code)
	if err != nil {
		return nil, err
	}
	count, err := s.members.CountActive(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	return &Preview{Group: *group, Roles: roles, MemberCount: count, ViaInvite: invite != nil}, nil
}

// Join adds the user to the group referenced by an invite code or a group join
// code. Joining twice is idempotent.
func (s *Service) Join(ctx context.Context, userID uuid.UUID, code string) (*domain.GroupWithMembership, error) {
	if _, err := s.access.User(ctx, userID); err != nil {
		return nil, err
	}
	var out *domain.GroupWithMembership
	err := s.tx.RunInTx(ctx, func(ctx context.Context) error {
		group, roles, invite, err := s.resolveCode(ctx, code)
		if err != nil {
			return err
		}
		existing, err := s.members.Get(ctx, userID, group.ID)
		switch {
		case err == nil:
			if existing.Status == domain.MembershipBanned {
				return domain.Forbidden("you are banned from this group")
			}
			out = &domain.GroupWithMembership{Group: *group, Membership: *existing}
			return nil
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		via := domain.JoinedViaLink
		if invite != nil {
			via = domain.JoinedViaInvite
			if _, err := s.invites.IncrementUses(ctx, invite.ID); err != nil {
				return err
			}
		}
		m, err := s.members.Create(ctx, domain.Membership{
			ID: ids.New(), UserID: userID, GroupID: group.ID, Roles: roles, Status: domain.MembershipActive, JoinedVia: via,
		})
		if err != nil {
			return err
		}
		if err := s.emit(ctx, group.ID, domain.EventMemberJoined, &userID, "membership", &m.ID, map[string]any{"user_id": userID, "roles": roles, "via": via}); err != nil {
			return err
		}
		out = &domain.GroupWithMembership{Group: *group, Membership: *m}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// resolveCode maps a code to (group, roles, invite). Invite codes win over
// join codes; join codes require an OPEN policy.
func (s *Service) resolveCode(ctx context.Context, code string) (*domain.Group, []domain.Role, *domain.Invite, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return nil, nil, nil, domain.Invalid("code", "required")
	}
	invite, err := s.invites.GetByCode(ctx, code)
	switch {
	case err == nil:
		if !invite.Usable(s.clock.Now()) {
			return nil, nil, nil, fmt.Errorf("%w: invite is no longer valid", domain.ErrGone)
		}
		group, err := s.groups.Get(ctx, invite.GroupID)
		if err != nil {
			return nil, nil, nil, err
		}
		if group.ArchivedAt != nil {
			return nil, nil, nil, domain.NotFound("group")
		}
		roles := invite.Roles
		if len(roles) == 0 {
			roles = []domain.Role{domain.RoleStudent}
		}
		return group, roles, invite, nil
	case !errors.Is(err, domain.ErrNotFound):
		return nil, nil, nil, err
	}
	group, err := s.groups.GetByJoinCode(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil, nil, domain.NotFound("invite or group code")
		}
		return nil, nil, nil, err
	}
	if group.JoinPolicy != domain.JoinOpen {
		return nil, nil, nil, domain.Forbidden("this group joins by invitation only")
	}
	return group, []domain.Role{domain.RoleStudent}, nil, nil
}

// MembersResult carries members plus what the caller may see about them.
type MembersResult struct {
	Members  []domain.Member
	Extended bool
}

// Members lists group members.
func (s *Service) Members(ctx context.Context, userID, groupID uuid.UUID) (*MembersResult, error) {
	actor, err := s.access.Actor(ctx, userID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	members, err := s.members.List(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return &MembersResult{Members: members, Extended: actor.Can(authz.MemberViewExt)}, nil
}

// UpdateRoles replaces a member's role set.
func (s *Service) UpdateRoles(ctx context.Context, actorID, groupID, targetID uuid.UUID, roles []domain.Role) (*domain.Membership, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.MemberManage); err != nil {
		return nil, err
	}
	roles = dedupeRoles(roles)
	if len(roles) == 0 {
		return nil, domain.Invalid("roles", "at least one role is required")
	}
	target, err := s.members.Get(ctx, targetID, groupID)
	if err != nil {
		return nil, err
	}
	actorIsOwner := actor.Membership.HasRole(domain.RoleOwner)
	if (target.HasRole(domain.RoleOwner) || slices.Contains(roles, domain.RoleOwner)) && !actorIsOwner {
		return nil, domain.Forbidden("only an owner can change ownership")
	}
	if target.HasRole(domain.RoleOwner) && !slices.Contains(roles, domain.RoleOwner) {
		if err := s.ensureAnotherOwner(ctx, groupID, targetID); err != nil {
			return nil, err
		}
	}
	var updated *domain.Membership
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		updated, err = s.members.UpdateRoles(ctx, targetID, groupID, roles)
		if err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventMemberRoles, &actorID, "membership", &updated.ID, map[string]any{"user_id": targetID, "roles": roles})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SetStatus bans or reinstates a member.
func (s *Service) SetStatus(ctx context.Context, actorID, groupID, targetID uuid.UUID, status domain.MembershipStatus) (*domain.Membership, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.MemberManage); err != nil {
		return nil, err
	}
	if status != domain.MembershipActive && status != domain.MembershipBanned {
		return nil, domain.Invalid("status", "must be ACTIVE or BANNED")
	}
	if targetID == actorID {
		return nil, domain.Invalid("user_id", "cannot change your own status")
	}
	target, err := s.members.Get(ctx, targetID, groupID)
	if err != nil {
		return nil, err
	}
	if target.HasRole(domain.RoleOwner) {
		return nil, domain.Forbidden("owners cannot be banned")
	}
	var updated *domain.Membership
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		updated, err = s.members.UpdateStatus(ctx, targetID, groupID, status)
		if err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventMemberStatus, &actorID, "membership", &updated.ID, map[string]any{"user_id": targetID, "status": status})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// InviteInput describes a new invite.
type InviteInput struct {
	Roles       []domain.Role
	ExpiresDays *int
	MaxUses     *int32
}

// CreateInvite issues an invite code. Headmen may only invite students and guests.
func (s *Service) CreateInvite(ctx context.Context, actorID, groupID uuid.UUID, in InviteInput) (*domain.Invite, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.InviteCreate); err != nil {
		return nil, err
	}
	roles := dedupeRoles(in.Roles)
	if len(roles) == 0 {
		roles = []domain.Role{domain.RoleStudent}
	}
	if slices.Contains(roles, domain.RoleOwner) {
		return nil, domain.Invalid("roles", "ownership cannot be granted by invite")
	}
	if !actor.Can(authz.MemberManage) {
		for _, r := range roles {
			if r != domain.RoleStudent && r != domain.RoleGuest {
				return nil, domain.Forbidden("you may only invite students and guests")
			}
		}
	}
	days := s.defaults.InviteTTLDays
	if in.ExpiresDays != nil {
		days = *in.ExpiresDays
	}
	var expires *time.Time
	if days > 0 {
		t := s.clock.Now().Add(time.Duration(days) * 24 * time.Hour)
		expires = &t
	}
	if in.MaxUses != nil && *in.MaxUses <= 0 {
		return nil, domain.Invalid("max_uses", "must be positive")
	}
	var invite *domain.Invite
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		invite, err = s.invites.Create(ctx, domain.Invite{
			ID: ids.New(), GroupID: groupID, Code: ids.Code(12), Roles: roles, ExpiresAt: expires, MaxUses: in.MaxUses, CreatedBy: actorID,
		})
		if err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventInviteCreated, &actorID, "invite", &invite.ID, map[string]any{"roles": roles})
	})
	if err != nil {
		return nil, err
	}
	return invite, nil
}

// ListInvites returns active invites.
func (s *Service) ListInvites(ctx context.Context, actorID, groupID uuid.UUID) ([]domain.Invite, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.InviteCreate); err != nil {
		return nil, err
	}
	return s.invites.ListByGroup(ctx, groupID)
}

// RevokeInvite disables an invite.
func (s *Service) RevokeInvite(ctx context.Context, actorID, groupID, inviteID uuid.UUID) error {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.InviteCreate); err != nil {
		return err
	}
	return s.tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.invites.Revoke(ctx, inviteID, groupID); err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventInviteRevoked, &actorID, "invite", &inviteID, nil)
	})
}

// RotateJoinCode replaces the group's permanent join code.
func (s *Service) RotateJoinCode(ctx context.Context, actorID, groupID uuid.UUID) (*domain.Group, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupSettings); err != nil {
		return nil, err
	}
	var group *domain.Group
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		code := ids.Code(10)
		group, err = s.groups.SetJoinCode(ctx, groupID, &code)
		if err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventJoinCodeRotated, &actorID, "group", &groupID, nil)
	})
	if err != nil {
		return nil, err
	}
	return group, nil
}

// UpdateInput changes group settings. Nil fields are left unchanged.
type UpdateInput struct {
	Name       *string
	Kind       *domain.GroupKind
	JoinPolicy *domain.JoinPolicy
	PublicRead *bool
	MediaMode  *domain.MediaMode
	Settings   json.RawMessage
}

// Update edits group settings.
func (s *Service) Update(ctx context.Context, actorID, groupID uuid.UUID, in UpdateInput) (*domain.Group, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupSettings); err != nil {
		return nil, err
	}
	g := actor.Group
	p := domain.UpdateGroupParams{ID: g.ID, Name: g.Name, Kind: g.Kind, JoinPolicy: g.JoinPolicy, PublicRead: g.PublicRead, MediaMode: g.MediaMode, Settings: g.Settings}
	if in.Name != nil {
		name := strings.Join(strings.Fields(*in.Name), " ")
		if n := utf8.RuneCountInString(name); n < 2 || n > 80 {
			return nil, domain.Invalid("name", "must be 2–80 characters")
		}
		p.Name = name
	}
	if in.Kind != nil {
		p.Kind = *in.Kind
	}
	if in.JoinPolicy != nil {
		if *in.JoinPolicy == domain.JoinApproval {
			return nil, domain.Invalid("join_policy", "APPROVAL is not available yet")
		}
		p.JoinPolicy = *in.JoinPolicy
	}
	if in.PublicRead != nil {
		if *in.PublicRead && !s.defaults.PublicReadSwitch {
			return nil, domain.Invalid("public_read", "public reading is disabled on this server")
		}
		p.PublicRead = *in.PublicRead
	}
	if in.MediaMode != nil {
		p.MediaMode = *in.MediaMode
	}
	if len(in.Settings) > 0 {
		if !json.Valid(in.Settings) {
			return nil, domain.Invalid("settings", "must be valid JSON")
		}
		p.Settings = in.Settings
	}
	var updated *domain.Group
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		updated, err = s.groups.Update(ctx, p)
		if err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventGroupUpdated, &actorID, "group", &groupID, map[string]any{"name": updated.Name})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) ensureAnotherOwner(ctx context.Context, groupID, exceptUser uuid.UUID) error {
	members, err := s.members.List(ctx, groupID)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.Membership.UserID != exceptUser && m.Membership.IsActive() && m.Membership.HasRole(domain.RoleOwner) {
			return nil
		}
	}
	return domain.Invalid("roles", "the group must keep at least one owner")
}

func (s *Service) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := slug.Make(name)
	candidate := base
	for i := 0; i < 5; i++ {
		exists, err := s.groups.SlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = base + "-" + ids.Code(4)
	}
	return base + "-" + ids.Code(8), nil
}

func (s *Service) emit(ctx context.Context, groupID uuid.UUID, kind domain.EventKind, actor *uuid.UUID, entityType string, entityID *uuid.UUID, payload any) error {
	e, err := domain.NewEvent(groupID, kind, actor, entityType, entityID, payload)
	return s.events.Emit(ctx, e, err)
}

func dedupeRoles(roles []domain.Role) []domain.Role {
	out := make([]domain.Role, 0, len(roles))
	for _, r := range roles {
		if _, ok := domain.ParseRole(string(r)); ok && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}
