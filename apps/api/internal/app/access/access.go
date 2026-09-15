// Package access resolves the acting user inside a group and checks
// permissions. Every group-scoped use case starts here.
package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
)

// Service loads actors.
type Service struct {
	users   domain.UserRepo
	groups  domain.GroupRepo
	members domain.MembershipRepo
}

// New wires the service.
func New(users domain.UserRepo, groups domain.GroupRepo, members domain.MembershipRepo) *Service {
	return &Service{users: users, groups: groups, members: members}
}

// Actor is a user acting inside a specific group.
type Actor struct {
	User       *domain.User
	Group      *domain.Group
	Membership *domain.Membership
}

// Require returns ErrForbidden unless the actor may perform action.
func (a *Actor) Require(action authz.Action) error {
	return authz.Check(a.Membership, a.User, action)
}

// Can reports whether the actor may perform action.
func (a *Actor) Can(action authz.Action) bool {
	return a.Require(action) == nil
}

// Actor loads user, group and membership. It fails with ErrForbidden when the
// user is not an active member and with ErrNotFound when the group is archived
// or missing.
func (s *Service) Actor(ctx context.Context, userID, groupID uuid.UUID) (*Actor, error) {
	user, err := s.User(ctx, userID)
	if err != nil {
		return nil, err
	}
	group, err := s.groups.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group.ArchivedAt != nil {
		return nil, domain.NotFound("group")
	}
	m, err := s.members.Get(ctx, userID, groupID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.Forbidden("not a member of this group")
		}
		return nil, err
	}
	if !m.IsActive() {
		return nil, domain.Forbidden(fmt.Sprintf("membership is %s", m.Status))
	}
	return &Actor{User: user, Group: group, Membership: m}, nil
}

// User loads the current user or fails with ErrUnauthorized.
func (s *Service) User(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: user no longer exists", domain.ErrUnauthorized)
		}
		return nil, err
	}
	return user, nil
}
