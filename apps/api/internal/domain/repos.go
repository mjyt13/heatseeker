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
	// RegisterClientID makes sign-up idempotent (see auth.Service.Register).
	RegisterClientID *uuid.UUID
}

// UserRepo persists users.
type UserRepo interface {
	Create(ctx context.Context, p CreateUserParams) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	// GetByEmail also returns the password hash (empty when none) for login.
	GetByEmail(ctx context.Context, email string) (*User, string, error)
	GetByRegisterClientID(ctx context.Context, clientID uuid.UUID) (*User, error)
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
	// SearchOpen finds open groups whose name contains query (all of them when query is empty).
	SearchOpen(ctx context.Context, userID uuid.UUID, query string, limit int32) ([]GroupSearchHit, error)
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

// UpdateMaterialParams is the editable part of a material.
type UpdateMaterialParams struct {
	ID             uuid.UUID
	Title          string
	Description    string
	SubjectID      *uuid.UUID
	Kind           MaterialKind
	Classification Classification
	NeedsReview    bool
	ReviewReason   *ReviewReason
}

// MaterialRepo persists materials and their versions.
type MaterialRepo interface {
	Create(ctx context.Context, m Material) (*Material, error)
	Get(ctx context.Context, id uuid.UUID) (*Material, error)
	GetView(ctx context.Context, id uuid.UUID) (*MaterialView, error)
	List(ctx context.Context, f MaterialFilter) ([]MaterialView, error)
	CountInbox(ctx context.Context, groupID uuid.UUID) (int64, error)
	Update(ctx context.Context, p UpdateMaterialParams) (*Material, error)
	SetStatus(ctx context.Context, id uuid.UUID, status MaterialStatus, actor *uuid.UUID) (*Material, error)
	IncrementDownloads(ctx context.Context, id uuid.UUID) error
	SetTags(ctx context.Context, id uuid.UUID, tagIDs []uuid.UUID) error
	ListPurgeable(ctx context.Context, deletedBefore time.Time, limit int32) ([]Material, error)
	HardDelete(ctx context.Context, id uuid.UUID) error

	CreateVersion(ctx context.Context, v MaterialVersion) (*MaterialVersion, error)
	SetCurrentVersion(ctx context.Context, materialID, versionID uuid.UUID) error
	GetVersion(ctx context.Context, id uuid.UUID) (*MaterialVersion, error)
	ListVersions(ctx context.Context, materialID uuid.UUID) ([]MaterialVersion, error)
	// UpdateVersionFile refreshes file metadata of a Drive-backed version in place.
	UpdateVersionFile(ctx context.Context, v MaterialVersion) error
	// ClaimVersionPreview marks a preview requested (again, after a failure);
	// false when it is already pending, ready or skipped.
	ClaimVersionPreview(ctx context.Context, versionID uuid.UUID) (bool, error)
	SetVersionPreview(ctx context.Context, versionID uuid.UUID, status PreviewStatus, key, errMsg *string) error
	// SetVersionDriveRevision remembers the Drive revision matched to a version.
	SetVersionDriveRevision(ctx context.Context, versionID uuid.UUID, revisionID string) error
	SetVersionDriveUpload(ctx context.Context, versionID uuid.UUID, status DriveUploadStatus, errMsg, fileID, webViewLink *string) error
	SetVersionHash(ctx context.Context, versionID uuid.UUID, sha256 string) error
	// Share turns a file kept in a task into a group material (D43).
	Share(ctx context.Context, id uuid.UUID) (*Material, error)
}

// UploadRepo persists pending direct uploads.
type UploadRepo interface {
	Create(ctx context.Context, u Upload) (*Upload, error)
	Get(ctx context.Context, id uuid.UUID) (*Upload, error)
	// Complete marks a pending upload as completed; false when it was not pending.
	Complete(ctx context.Context, id, materialID uuid.UUID) (bool, error)
	ListExpired(ctx context.Context, now time.Time, limit int32) ([]Upload, error)
	MarkExpired(ctx context.Context, id uuid.UUID) error
}

// FinishSyncParams records the outcome of a sync run.
type FinishSyncParams struct {
	ID        uuid.UUID
	Status    DriveConnectionStatus
	LastError *string
	// LastErrorCode is the domain error code of the failure, when it has one.
	LastErrorCode *string
	PageToken     *string
	FullScan      bool
	Succeeded     bool
	CompletedAt   time.Time
}

// DriveRepo persists Drive connections and their file index.
type DriveRepo interface {
	UpsertConnection(ctx context.Context, c DriveConnection) (*DriveConnection, error)
	GetConnection(ctx context.Context, id uuid.UUID) (*DriveConnection, error)
	GetConnectionByGroup(ctx context.Context, groupID uuid.UUID) (*DriveConnection, error)
	DeleteConnection(ctx context.Context, id uuid.UUID) error
	// ListDue returns connections whose next incremental sync is due at now.
	ListDue(ctx context.Context, now time.Time) ([]DriveConnection, error)
	// BeginSync atomically marks a connection SYNCING unless another run holds
	// it and started after staleBefore.
	BeginSync(ctx context.Context, id uuid.UUID, now, staleBefore time.Time) (bool, error)
	FinishSync(ctx context.Context, p FinishSyncParams) error

	GetItem(ctx context.Context, connectionID uuid.UUID, fileID string) (*DriveItem, error)
	GetItemByMaterial(ctx context.Context, materialID uuid.UUID) (*DriveItem, error)
	// UpsertItem writes file metadata and seen_at; state, material and
	// classification are only set on insert.
	UpsertItem(ctx context.Context, item DriveItem) (*DriveItem, error)
	UpdateItemState(ctx context.Context, id uuid.UUID, state DriveItemState, materialID *uuid.UUID, c Classification, lastError *string) error
	ListItems(ctx context.Context, connectionID uuid.UUID, state *DriveItemState, limit, offset int32) ([]DriveItem, error)
	ListFolders(ctx context.Context, connectionID uuid.UUID) ([]DriveItem, error)
	// ListReclassifyCandidates returns the group's Drive files that sit in the
	// Inbox only because automatic classification was unsure.
	ListReclassifyCandidates(ctx context.Context, groupID uuid.UUID) ([]DriveItem, error)
	ListUnseen(ctx context.Context, connectionID uuid.UUID, before time.Time) ([]DriveItem, error)
	DeleteItems(ctx context.Context, connectionID uuid.UUID) error
	Stats(ctx context.Context, connectionID uuid.UUID) (*DriveStats, error)

	// UpsertPublisher stores (or replaces) the group's publishing account and
	// clears its error.
	UpsertPublisher(ctx context.Context, p DrivePublisher) (*DrivePublisher, error)
	GetPublisher(ctx context.Context, groupID uuid.UUID) (*DrivePublisher, error)
	DeletePublisher(ctx context.Context, groupID uuid.UUID) error
	SetPublisherError(ctx context.Context, groupID uuid.UUID, msg *string) error
}

// TaskRepo persists tasks, personal progress and attachments.
type TaskRepo interface {
	Create(ctx context.Context, t Task) (*Task, error)
	Get(ctx context.Context, id uuid.UUID) (*Task, error)
	GetByClientID(ctx context.Context, groupID, clientID uuid.UUID) (*Task, error)
	GetView(ctx context.Context, id, actorID uuid.UUID) (*TaskView, error)
	List(ctx context.Context, f TaskFilter) ([]TaskView, error)
	Counts(ctx context.Context, groupID, actorID uuid.UUID, now time.Time, dueSoonBefore time.Time) (*TaskCounts, error)
	Update(ctx context.Context, p UpdateTaskParams) (*Task, error)
	SetStatus(ctx context.Context, id uuid.UUID, status TaskStatus, completedAt *time.Time) (*Task, error)
	SetPinned(ctx context.Context, id uuid.UUID, by *uuid.UUID, at *time.Time) (*Task, error)
	SoftDelete(ctx context.Context, id uuid.UUID, at time.Time) error

	SetAssignees(ctx context.Context, taskID uuid.UUID, userIDs []uuid.UUID) error
	ListAssignments(ctx context.Context, taskID uuid.UUID) ([]TaskAssignment, error)
	SetMyStatus(ctx context.Context, taskID, userID uuid.UUID, status TaskStatus, completedAt *time.Time) (*TaskAssignment, error)
	// AddAttachment attaches one material without touching the others.
	AddAttachment(ctx context.Context, taskID, materialID uuid.UUID) error
	SetAttachments(ctx context.Context, taskID, groupID uuid.UUID, materialIDs []uuid.UUID) error

	// ListDueReminders returns tasks whose deadline reminder is due now, for
	// the configured offsets (minutes before the deadline). Deadlines older
	// than graceMinutes are left alone.
	ListDueReminders(ctx context.Context, now time.Time, offsets []int32, graceMinutes, limit int32) ([]DueTask, error)
	// MarkNotified remembers offsets already announced for a task.
	MarkNotified(ctx context.Context, id uuid.UUID, offsets []int32) error
}

// UpdateTaskParams is the editable part of a task.
type UpdateTaskParams struct {
	ID          uuid.UUID
	SubjectID   *uuid.UUID
	Title       string
	Description string
	Kind        TaskKind
	Priority    TaskPriority
	AssignMode  TaskAssignMode
	Visibility  TaskVisibility
	DueAt       *time.Time
	// ResetNotified clears the announced reminders after the deadline moved.
	ResetNotified bool
}

// DiscussionRepo persists threads, messages, personal hides and read marks.
type DiscussionRepo interface {
	GetThread(ctx context.Context, groupID uuid.UUID, target ThreadTarget, targetID uuid.UUID) (*Thread, error)
	GetThreadByID(ctx context.Context, id uuid.UUID) (*Thread, error)
	// EnsureThread creates the thread of a target or returns the existing one.
	EnsureThread(ctx context.Context, t Thread) (*Thread, error)
	// ListThreads returns the group's threads with the viewer's unread counts.
	ListThreads(ctx context.Context, groupID, viewerID uuid.UUID) ([]ThreadSummary, error)
	// TouchThread records a new message and adjusts the message count.
	TouchThread(ctx context.Context, threadID uuid.UUID, seq int64, at time.Time) error
	AdjustMessageCount(ctx context.Context, threadID uuid.UUID, delta int32) error

	CreateMessage(ctx context.Context, m Message) (*Message, error)
	GetMessage(ctx context.Context, id uuid.UUID) (*Message, error)
	GetMessageByClientID(ctx context.Context, threadID, clientID uuid.UUID) (*Message, error)
	GetMessageView(ctx context.Context, id, viewerID uuid.UUID) (*MessageView, error)
	ListMessages(ctx context.Context, f MessageFilter) ([]MessageView, error)
	EditMessage(ctx context.Context, id uuid.UUID, body string, at time.Time) (*Message, error)
	SoftDeleteMessage(ctx context.Context, id uuid.UUID, at time.Time) (*Message, error)
	UndeleteMessage(ctx context.Context, id uuid.UUID) (*Message, error)
	SetHiddenForAll(ctx context.Context, id uuid.UUID, by *uuid.UUID, at *time.Time) (*Message, error)

	HideMessage(ctx context.Context, messageID, userID uuid.UUID) error
	UnhideMessage(ctx context.Context, messageID, userID uuid.UUID) error
	// ListHiddenByUser is the "hidden by me" screen, newest hide first.
	ListHiddenByUser(ctx context.Context, groupID, userID uuid.UUID, limit int32) ([]MessageView, error)

	// MarkRead moves the read mark forward; it never goes back.
	MarkRead(ctx context.Context, threadID, userID uuid.UUID, seq int64) error
	// ReadSeq is the user's read mark in a thread, 0 before the first read.
	ReadSeq(ctx context.Context, threadID, userID uuid.UUID) (int64, error)
}

// ScheduleRepo persists classes and their exceptions. Edits of an event name
// the version they started from and fail with ErrConflict when it moved on.
type ScheduleRepo interface {
	Create(ctx context.Context, e ScheduleEvent) (*ScheduleEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*ScheduleEvent, error)
	GetByClientID(ctx context.Context, groupID, clientID uuid.UUID) (*ScheduleEvent, error)
	// ListInWindow returns the events that may have a class overlapping [from, to).
	ListInWindow(ctx context.Context, groupID uuid.UUID, from, to time.Time) ([]ScheduleEvent, error)
	ListAll(ctx context.Context, groupID uuid.UUID) ([]ScheduleEvent, error)
	Update(ctx context.Context, e ScheduleEvent) (*ScheduleEvent, error)
	// EndSeries makes until the last date of a series.
	EndSeries(ctx context.Context, id uuid.UUID, version int32, until time.Time, by uuid.UUID) (*ScheduleEvent, error)
	SoftDelete(ctx context.Context, id uuid.UUID, version int32, at time.Time, by uuid.UUID) error

	ListExceptions(ctx context.Context, eventIDs []uuid.UUID) ([]ScheduleException, error)
	GetException(ctx context.Context, eventID uuid.UUID, date time.Time) (*ScheduleException, error)
	PutException(ctx context.Context, x ScheduleException) (*ScheduleException, error)
	// DeleteException fails with ErrNotFound when there was none.
	DeleteException(ctx context.Context, eventID uuid.UUID, date time.Time) error
	// MoveExceptions hands the exceptions from a date on to another event.
	MoveExceptions(ctx context.Context, fromEventID, toEventID uuid.UUID, fromDate time.Time) error
	DeleteExceptionsFrom(ctx context.Context, eventID uuid.UUID, fromDate time.Time) error
}

// NotifyRepo persists notifications, the reader's position in each group's
// log and every member's delivery preferences.
type NotifyRepo interface {
	// Cursor is the last group event the reader has handled. The second
	// result is false when the group has never been read: nobody is notified
	// about what happened before the reader existed.
	Cursor(ctx context.Context, groupID uuid.UUID) (int64, bool, error)
	SetCursor(ctx context.Context, groupID uuid.UUID, seq int64) error
	// GroupsWithPending lists groups whose log has moved past the reader.
	GroupsWithPending(ctx context.Context, limit int32) ([]uuid.UUID, error)

	// Recipients are the members a notification should reach.
	Recipients(ctx context.Context, q NotifyRecipients) ([]Recipient, error)
	// ThreadParticipants are the members who have written in a discussion.
	ThreadParticipants(ctx context.Context, threadID uuid.UUID) ([]uuid.UUID, error)
	// TaskAssignees are the members with their own part of a task, done or not.
	TaskAssignees(ctx context.Context, taskID uuid.UUID) ([]TaskAssignee, error)

	// Insert stores one notification, or nothing when its dedupe key is taken.
	Insert(ctx context.Context, n Notification) (*Notification, error)
	List(ctx context.Context, f NotificationFilter) ([]Notification, error)
	Unread(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID) (int, error)
	// MarkRead marks the listed notifications, or every unread one when all is set.
	MarkRead(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID, ids []uuid.UUID, all bool) (int, error)
	DeleteOlderThan(ctx context.Context, t time.Time) (int64, error)

	Prefs(ctx context.Context, userID, groupID uuid.UUID) ([]NotificationPref, error)
	SetPref(ctx context.Context, p NotificationPref) error
	Settings(ctx context.Context, userID uuid.UUID) (*NotificationSettings, error)
	SaveSettings(ctx context.Context, s NotificationSettings) (*NotificationSettings, error)

	Mutes(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID) ([]NotificationMute, error)
	SetMute(ctx context.Context, m NotificationMute) (*NotificationMute, error)
	DeleteMute(ctx context.Context, userID, groupID uuid.UUID, scope MuteScope, scopeID string) error
	DeleteExpiredMutes(ctx context.Context, now time.Time) (int64, error)

	// Message and Task fill in what an event's payload does not carry.
	Message(ctx context.Context, id uuid.UUID) (*NotifyMessage, error)
	Task(ctx context.Context, id uuid.UUID) (*NotifyTask, error)
	UserName(ctx context.Context, id uuid.UUID) (string, error)
	SubjectName(ctx context.Context, id uuid.UUID) (string, error)
	// MaterialOwner is who uploaded a material, when anybody did.
	MaterialOwner(ctx context.Context, id uuid.UUID) (*uuid.UUID, error)

	// PushTargets are the devices of the given members that can take a push.
	PushTargets(ctx context.Context, userIDs []uuid.UUID) ([]PushTarget, error)
	Notifications(ctx context.Context, ids []uuid.UUID) ([]Notification, error)
	RecordDelivery(ctx context.Context, d NotificationDelivery) error
	// DisableDevicePush forgets a token the push service rejected for good.
	DisableDevicePush(ctx context.Context, deviceID uuid.UUID) error
}

// NotificationDelivery is one attempt to put a notification on a device.
type NotificationDelivery struct {
	ID             uuid.UUID
	NotificationID uuid.UUID
	Channel        NotifyChannel
	DeviceID       *uuid.UUID
	Status         DeliveryStatus
	Error          string
	SentAt         *time.Time
}

// Pusher sends notifications to devices (Adapter). Implementations are Expo
// today, Web Push and Telegram later.
type Pusher interface {
	// Push delivers one notification to the given devices and reports, per
	// device, whether it went out. A token the service rejected for good is
	// returned in dead so its device can be cleaned up.
	Push(ctx context.Context, n Notification, targets []PushTarget) (results []PushResult, err error)
}

// PushResult is the outcome for one device.
type PushResult struct {
	DeviceID uuid.UUID
	Sent     bool
	// Dead means the token will never work again (the app was uninstalled).
	Dead  bool
	Error string
}

// AnnouncementRepo persists announcements.
type AnnouncementRepo interface {
	Create(ctx context.Context, a Announcement) (*Announcement, error)
	Get(ctx context.Context, id uuid.UUID) (*Announcement, error)
	List(ctx context.Context, groupID uuid.UUID, limit int32) ([]AnnouncementView, error)
	Update(ctx context.Context, a Announcement) (*Announcement, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// ReminderRepo persists personal reminders.
type ReminderRepo interface {
	Create(ctx context.Context, r Reminder) (*Reminder, error)
	Get(ctx context.Context, id uuid.UUID) (*Reminder, error)
	List(ctx context.Context, userID, groupID uuid.UUID, openOnly bool, limit int32) ([]Reminder, error)
	Update(ctx context.Context, r Reminder) (*Reminder, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	// Due lists reminders whose moment has come.
	Due(ctx context.Context, now time.Time, limit int32) ([]Reminder, error)
}
