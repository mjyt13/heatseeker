package authz

import (
	"errors"
	"testing"
	"time"

	"heatseeker/api/internal/domain"
)

func TestCan(t *testing.T) {
	tests := []struct {
		name   string
		roles  []domain.Role
		action Action
		want   bool
	}{
		{"student reads", []domain.Role{domain.RoleStudent}, GroupRead, true},
		{"student cannot moderate", []domain.Role{domain.RoleStudent}, MaterialModerate, false},
		{"guest cannot write", []domain.Role{domain.RoleGuest}, ThreadWrite, false},
		{"guest can bookmark", []domain.Role{domain.RoleGuest}, BookmarkManage, true},
		{"headman edits schedule", []domain.Role{domain.RoleHeadman}, ScheduleEdit, true},
		{"moderator does not edit schedule", []domain.Role{domain.RoleModerator}, ScheduleEdit, false},
		{"multi-role union", []domain.Role{domain.RoleStudent, domain.RoleHeadman}, TaskPin, true},
		{"owner manages members", []domain.Role{domain.RoleOwner}, MemberManage, true},
		{"unknown action", []domain.Role{domain.RoleOwner}, Action("nope"), false},
		{"no roles", nil, GroupRead, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Can(tc.roles, tc.action); got != tc.want {
				t.Fatalf("Can(%v, %s) = %v, want %v", tc.roles, tc.action, got, tc.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	now := time.Now()
	secured := &domain.User{SecuredAt: &now}
	light := &domain.User{}
	active := &domain.Membership{Roles: []domain.Role{domain.RoleAdmin}, Status: domain.MembershipActive}
	banned := &domain.Membership{Roles: []domain.Role{domain.RoleAdmin}, Status: domain.MembershipBanned}

	if err := Check(active, secured, MemberManage); err != nil {
		t.Fatalf("secured admin should manage members: %v", err)
	}
	if err := Check(active, light, MemberManage); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("light admin must be forbidden for secured action, got %v", err)
	}
	if err := Check(active, light, GroupRead); err != nil {
		t.Fatalf("light admin may read: %v", err)
	}
	if err := Check(banned, secured, GroupRead); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("banned member must be forbidden, got %v", err)
	}
	if err := Check(nil, secured, GroupRead); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("nil membership must be forbidden, got %v", err)
	}
}

func TestMatrixCoversEveryRoleSomewhere(t *testing.T) {
	for _, r := range domain.AllRoles {
		if len(Allowed([]domain.Role{r})) == 0 {
			t.Fatalf("role %s grants nothing", r)
		}
	}
	// Every role listed in the matrix must be a known role.
	for a, roles := range Matrix {
		for _, r := range roles {
			if _, ok := domain.ParseRole(string(r)); !ok {
				t.Fatalf("action %s references unknown role %q", a, r)
			}
		}
	}
	// Secured actions must exist in the matrix.
	for a := range SecuredActions {
		if _, ok := Matrix[a]; !ok {
			t.Fatalf("secured action %s is not in Matrix", a)
		}
	}
}
