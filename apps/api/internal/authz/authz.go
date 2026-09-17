// Package authz is the single source of truth for what each group role may do.
// The matrix is exported to packages/shared so clients can hide controls the
// server would reject. Ownership rules ("edit only your own material") are
// enforced in application services on top of these coarse permissions.
package authz

import (
	"slices"
	"sort"

	"heatseeker/api/internal/domain"
)

// Action is a permission identifier of the form "<resource>.<verb>[.<scope>]".
type Action string

// Actions. Keep this list in sync with docs/PLAN.md §4.6.
const (
	GroupRead        Action = "group.read"
	GroupSettings    Action = "group.settings"
	GroupArchive     Action = "group.archive"
	MemberManage     Action = "member.manage"
	MemberViewExt    Action = "member.view_extended"
	MemberViewAuthor Action = "member.view_author"
	InviteCreate     Action = "invite.create"
	SubjectManage    Action = "subject.manage"
	TagManage        Action = "tag.manage"

	MaterialUpload   Action = "material.upload"
	MaterialEditOwn  Action = "material.edit.own"
	MaterialModerate Action = "material.moderate"

	TaskCreate      Action = "task.create"
	TaskPin         Action = "task.pin"
	TaskStatusOwn   Action = "task.status.own"
	TaskStatusGroup Action = "task.status.group"

	ScheduleRead    Action = "schedule.read"
	SchedulePropose Action = "schedule.propose"
	ScheduleEdit    Action = "schedule.edit"
	ScheduleApprove Action = "schedule.approve"

	ThreadWrite     Action = "thread.write"
	MessageHideSelf Action = "message.hide.self"
	MessageModerate Action = "message.moderate"

	ProposalCreate   Action = "proposal.create"
	ProposalModerate Action = "proposal.moderate"

	AnnouncementSend Action = "announcement.send"
	ReminderManage   Action = "reminder.manage"
	BookmarkManage   Action = "bookmark.manage"
	DriveManage      Action = "drive.manage"
	DriveUpload      Action = "drive.upload"
)

// Matrix maps every action to the roles that may perform it.
var Matrix = map[Action][]domain.Role{
	GroupRead:        {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent, domain.RoleGuest},
	GroupSettings:    {domain.RoleOwner, domain.RoleAdmin},
	GroupArchive:     {domain.RoleOwner},
	MemberManage:     {domain.RoleOwner, domain.RoleAdmin},
	MemberViewExt:    {domain.RoleOwner, domain.RoleAdmin},
	MemberViewAuthor: {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator},
	InviteCreate:     {domain.RoleOwner, domain.RoleAdmin, domain.RoleHeadman},
	SubjectManage:    {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman},
	TagManage:        {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman},

	MaterialUpload:   {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	MaterialEditOwn:  {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	MaterialModerate: {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator},

	TaskCreate:      {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	TaskPin:         {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman},
	TaskStatusOwn:   {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	TaskStatusGroup: {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman},

	ScheduleRead:    {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent, domain.RoleGuest},
	SchedulePropose: {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	ScheduleEdit:    {domain.RoleOwner, domain.RoleAdmin, domain.RoleHeadman},
	ScheduleApprove: {domain.RoleOwner, domain.RoleAdmin, domain.RoleHeadman},

	ThreadWrite:     {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	MessageHideSelf: {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	MessageModerate: {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator},

	ProposalCreate:   {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
	ProposalModerate: {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator},

	AnnouncementSend: {domain.RoleOwner, domain.RoleAdmin, domain.RoleHeadman},
	ReminderManage:   {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent, domain.RoleGuest},
	BookmarkManage:   {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent, domain.RoleGuest},
	DriveManage:      {domain.RoleOwner, domain.RoleAdmin, domain.RoleHeadman},
	DriveUpload:      {domain.RoleOwner, domain.RoleAdmin, domain.RoleModerator, domain.RoleHeadman, domain.RoleStudent},
}

// SecuredActions require the actor to hold a secured (L2) account, because
// they carry authority over other people's data.
var SecuredActions = map[Action]bool{
	GroupSettings:    true,
	GroupArchive:     true,
	MemberManage:     true,
	MemberViewExt:    true,
	MaterialModerate: true,
	MessageModerate:  true,
	ProposalModerate: true,
	ScheduleEdit:     true,
	ScheduleApprove:  true,
	AnnouncementSend: true,
	DriveManage:      true,
	// Files land on Drive under the service account's name: only people who
	// can be identified later may publish there (AUTH_REQUIRE_SECURED_FOR).
	DriveUpload: true,
}

// Can reports whether any of roles grants action.
func Can(roles []domain.Role, action Action) bool {
	allowed, ok := Matrix[action]
	if !ok {
		return false
	}
	for _, r := range roles {
		if slices.Contains(allowed, r) {
			return true
		}
	}
	return false
}

// Check returns ErrForbidden unless the membership is active and one of its
// roles grants action. When the action is in SecuredActions the user must also
// have a secured account.
func Check(m *domain.Membership, user *domain.User, action Action) error {
	if m == nil || !m.IsActive() {
		return domain.Forbidden("not an active member")
	}
	if !Can(m.Roles, action) {
		return domain.Forbidden(string(action))
	}
	if SecuredActions[action] && (user == nil || !user.Secured()) {
		return domain.Forbidden("secure your account to perform " + string(action))
	}
	return nil
}

// Allowed returns the sorted list of actions the roles grant.
func Allowed(roles []domain.Role) []Action {
	out := make([]Action, 0, len(Matrix))
	for a := range Matrix {
		if Can(roles, a) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Actions returns every known action, sorted.
func Actions() []Action {
	out := make([]Action, 0, len(Matrix))
	for a := range Matrix {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
