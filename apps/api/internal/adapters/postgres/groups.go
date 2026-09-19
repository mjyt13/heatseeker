package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type groupRepo struct{ s *Store }

func toGroup(g sqlcgen.Group) *domain.Group {
	return &domain.Group{
		ID:         g.ID,
		Name:       g.Name,
		Slug:       g.Slug,
		Kind:       domain.GroupKind(g.Kind),
		JoinPolicy: domain.JoinPolicy(g.JoinPolicy),
		JoinCode:   g.JoinCode,
		PublicRead: g.PublicRead,
		MediaMode:  domain.MediaMode(g.MediaMode),
		Settings:   json.RawMessage(rawJSON(g.Settings)),
		LastSeq:    g.LastSeq,
		CreatedBy:  g.CreatedBy,
		CreatedAt:  g.CreatedAt,
		UpdatedAt:  g.UpdatedAt,
		ArchivedAt: g.ArchivedAt,
	}
}

func toMembership(m sqlcgen.Membership) *domain.Membership {
	return &domain.Membership{
		ID:        m.ID,
		UserID:    m.UserID,
		GroupID:   m.GroupID,
		Roles:     domain.RolesFromStrings(m.Roles),
		Status:    domain.MembershipStatus(m.Status),
		JoinedVia: domain.JoinedVia(m.JoinedVia),
		JoinedAt:  m.JoinedAt,
	}
}

func toInvite(i sqlcgen.Invite) *domain.Invite {
	return &domain.Invite{
		ID:        i.ID,
		GroupID:   i.GroupID,
		Code:      i.Code,
		Roles:     domain.RolesFromStrings(i.Roles),
		ExpiresAt: i.ExpiresAt,
		MaxUses:   i.MaxUses,
		Uses:      i.Uses,
		CreatedBy: i.CreatedBy,
		RevokedAt: i.RevokedAt,
		CreatedAt: i.CreatedAt,
	}
}

func (r *groupRepo) Create(ctx context.Context, p domain.CreateGroupParams) (*domain.Group, error) {
	g, err := r.s.queries(ctx).CreateGroup(ctx, sqlcgen.CreateGroupParams{
		ID:         p.ID,
		Name:       p.Name,
		Slug:       p.Slug,
		Kind:       string(p.Kind),
		JoinPolicy: string(p.JoinPolicy),
		JoinCode:   p.JoinCode,
		MediaMode:  string(p.MediaMode),
		Settings:   rawJSON(p.Settings),
		CreatedBy:  p.CreatedBy,
	})
	if err != nil {
		return nil, mapErr(err, "group")
	}
	return toGroup(g), nil
}

func (r *groupRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Group, error) {
	g, err := r.s.queries(ctx).GetGroup(ctx, id)
	if err != nil {
		return nil, mapErr(err, "group")
	}
	return toGroup(g), nil
}

func (r *groupRepo) GetBySlug(ctx context.Context, slug string) (*domain.Group, error) {
	g, err := r.s.queries(ctx).GetGroupBySlug(ctx, slug)
	if err != nil {
		return nil, mapErr(err, "group")
	}
	return toGroup(g), nil
}

func (r *groupRepo) GetByJoinCode(ctx context.Context, code string) (*domain.Group, error) {
	g, err := r.s.queries(ctx).GetGroupByJoinCode(ctx, &code)
	if err != nil {
		return nil, mapErr(err, "group")
	}
	return toGroup(g), nil
}

func (r *groupRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.GroupWithMembership, error) {
	rows, err := r.s.queries(ctx).ListGroupsForUser(ctx, userID)
	if err != nil {
		return nil, mapErr(err, "group")
	}
	out := make([]domain.GroupWithMembership, len(rows))
	for i, row := range rows {
		out[i] = domain.GroupWithMembership{Group: *toGroup(row.Group), Membership: *toMembership(row.Membership)}
	}
	return out, nil
}

func (r *groupRepo) SearchOpen(ctx context.Context, userID uuid.UUID, query string, limit int32) ([]domain.GroupSearchHit, error) {
	rows, err := r.s.queries(ctx).SearchOpenGroups(ctx, sqlcgen.SearchOpenGroupsParams{
		UserID: userID, Pattern: likeEscaper.Replace(query), Query: query, MaxResults: limit,
	})
	if err != nil {
		return nil, mapErr(err, "group")
	}
	out := make([]domain.GroupSearchHit, len(rows))
	for i, row := range rows {
		out[i] = domain.GroupSearchHit{Group: *toGroup(row.Group), MemberCount: row.MemberCount, IsMember: row.IsMember}
	}
	return out, nil
}

func (r *groupRepo) Update(ctx context.Context, p domain.UpdateGroupParams) (*domain.Group, error) {
	g, err := r.s.queries(ctx).UpdateGroup(ctx, sqlcgen.UpdateGroupParams{
		ID:         p.ID,
		Name:       p.Name,
		Kind:       string(p.Kind),
		JoinPolicy: string(p.JoinPolicy),
		PublicRead: p.PublicRead,
		MediaMode:  string(p.MediaMode),
		Settings:   rawJSON(p.Settings),
	})
	if err != nil {
		return nil, mapErr(err, "group")
	}
	return toGroup(g), nil
}

func (r *groupRepo) SetJoinCode(ctx context.Context, id uuid.UUID, code *string) (*domain.Group, error) {
	g, err := r.s.queries(ctx).SetGroupJoinCode(ctx, sqlcgen.SetGroupJoinCodeParams{ID: id, JoinCode: code})
	if err != nil {
		return nil, mapErr(err, "group")
	}
	return toGroup(g), nil
}

func (r *groupRepo) Archive(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.s.queries(ctx).ArchiveGroup(ctx, id), "group")
}

func (r *groupRepo) NextSeq(ctx context.Context, id uuid.UUID) (int64, error) {
	seq, err := r.s.queries(ctx).NextGroupSeq(ctx, id)
	return seq, mapErr(err, "group")
}

func (r *groupRepo) SlugExists(ctx context.Context, slug string) (bool, error) {
	ok, err := r.s.queries(ctx).GroupSlugExists(ctx, slug)
	return ok, mapErr(err, "group")
}

// --- memberships ---

type membershipRepo struct{ s *Store }

func (r *membershipRepo) Create(ctx context.Context, m domain.Membership) (*domain.Membership, error) {
	row, err := r.s.queries(ctx).CreateMembership(ctx, sqlcgen.CreateMembershipParams{
		ID:        m.ID,
		UserID:    m.UserID,
		GroupID:   m.GroupID,
		Roles:     domain.RolesToStrings(m.Roles),
		Status:    string(m.Status),
		JoinedVia: string(m.JoinedVia),
	})
	if err != nil {
		return nil, mapErr(err, "membership")
	}
	return toMembership(row), nil
}

func (r *membershipRepo) Get(ctx context.Context, userID, groupID uuid.UUID) (*domain.Membership, error) {
	row, err := r.s.queries(ctx).GetMembership(ctx, sqlcgen.GetMembershipParams{UserID: userID, GroupID: groupID})
	if err != nil {
		return nil, mapErr(err, "membership")
	}
	return toMembership(row), nil
}

func (r *membershipRepo) List(ctx context.Context, groupID uuid.UUID) ([]domain.Member, error) {
	rows, err := r.s.queries(ctx).ListMembers(ctx, groupID)
	if err != nil {
		return nil, mapErr(err, "membership")
	}
	out := make([]domain.Member, len(rows))
	for i, row := range rows {
		out[i] = domain.Member{Membership: *toMembership(row.Membership), User: *toUser(row.User)}
	}
	return out, nil
}

func (r *membershipRepo) UpdateRoles(ctx context.Context, userID, groupID uuid.UUID, roles []domain.Role) (*domain.Membership, error) {
	row, err := r.s.queries(ctx).UpdateMembershipRoles(ctx, sqlcgen.UpdateMembershipRolesParams{
		UserID:  userID,
		GroupID: groupID,
		Roles:   domain.RolesToStrings(roles),
	})
	if err != nil {
		return nil, mapErr(err, "membership")
	}
	return toMembership(row), nil
}

func (r *membershipRepo) UpdateStatus(ctx context.Context, userID, groupID uuid.UUID, status domain.MembershipStatus) (*domain.Membership, error) {
	row, err := r.s.queries(ctx).UpdateMembershipStatus(ctx, sqlcgen.UpdateMembershipStatusParams{
		UserID:  userID,
		GroupID: groupID,
		Status:  string(status),
	})
	if err != nil {
		return nil, mapErr(err, "membership")
	}
	return toMembership(row), nil
}

func (r *membershipRepo) CountActive(ctx context.Context, groupID uuid.UUID) (int64, error) {
	n, err := r.s.queries(ctx).CountActiveMembers(ctx, groupID)
	return n, mapErr(err, "membership")
}

func (r *membershipRepo) ListActiveUserIDs(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := r.s.queries(ctx).ListActiveMemberUserIDs(ctx, groupID)
	return ids, mapErr(err, "membership")
}

// --- invites ---

type inviteRepo struct{ s *Store }

func (r *inviteRepo) Create(ctx context.Context, i domain.Invite) (*domain.Invite, error) {
	row, err := r.s.queries(ctx).CreateInvite(ctx, sqlcgen.CreateInviteParams{
		ID:        i.ID,
		GroupID:   i.GroupID,
		Code:      i.Code,
		Roles:     domain.RolesToStrings(i.Roles),
		ExpiresAt: i.ExpiresAt,
		MaxUses:   i.MaxUses,
		CreatedBy: i.CreatedBy,
	})
	if err != nil {
		return nil, mapErr(err, "invite")
	}
	return toInvite(row), nil
}

func (r *inviteRepo) GetByCode(ctx context.Context, code string) (*domain.Invite, error) {
	row, err := r.s.queries(ctx).GetInviteByCode(ctx, code)
	if err != nil {
		return nil, mapErr(err, "invite")
	}
	return toInvite(row), nil
}

func (r *inviteRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]domain.Invite, error) {
	rows, err := r.s.queries(ctx).ListInvitesByGroup(ctx, groupID)
	if err != nil {
		return nil, mapErr(err, "invite")
	}
	out := make([]domain.Invite, len(rows))
	for i, row := range rows {
		out[i] = *toInvite(row)
	}
	return out, nil
}

func (r *inviteRepo) IncrementUses(ctx context.Context, id uuid.UUID) (*domain.Invite, error) {
	row, err := r.s.queries(ctx).IncrementInviteUses(ctx, id)
	if err != nil {
		return nil, mapErr(err, "invite")
	}
	return toInvite(row), nil
}

func (r *inviteRepo) Revoke(ctx context.Context, id, groupID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).RevokeInvite(ctx, sqlcgen.RevokeInviteParams{ID: id, GroupID: groupID}), "invite")
}
