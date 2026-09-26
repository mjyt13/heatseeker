package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// NotificationType says what happened. The client renders an icon per type and
// the member switches types on and off per group.
type NotificationType string

// Notification types. The log reader emits the ones marked "now"; the rest are
// reserved for later stages so preferences stay stable.
const (
	NotifyMessageNew      NotificationType = "MESSAGE_NEW"   // now
	NotifyMessageReply    NotificationType = "MESSAGE_REPLY" // now
	NotifyMaterialAdded   NotificationType = "MATERIAL_ADDED"
	NotifyMaterialBatch   NotificationType = "MATERIAL_BATCH"
	NotifyTaskCreated     NotificationType = "TASK_CREATED"
	NotifyTaskPinned      NotificationType = "TASK_PINNED"
	NotifyTaskDueSoon     NotificationType = "TASK_DUE_SOON"
	NotifyTaskOverdue     NotificationType = "TASK_OVERDUE"
	NotifyTaskStatus      NotificationType = "TASK_STATUS_CHANGED"
	NotifyScheduleChanged NotificationType = "SCHEDULE_CHANGED"
	NotifyMemberJoined    NotificationType = "MEMBER_JOINED"
	NotifyAnnouncement    NotificationType = "ANNOUNCEMENT" // stage 4
	NotifyReminder        NotificationType = "REMINDER"     // stage 4
	NotifyProposalNew     NotificationType = "PROPOSAL_NEW" // stage 4
	NotifyModeration      NotificationType = "MODERATION"   // stage 5
)

// AllNotificationTypes lists every type, exported to packages/shared.
var AllNotificationTypes = []NotificationType{
	NotifyMessageNew, NotifyMessageReply, NotifyMaterialAdded, NotifyMaterialBatch,
	NotifyTaskCreated, NotifyTaskPinned, NotifyTaskDueSoon, NotifyTaskOverdue, NotifyTaskStatus,
	NotifyScheduleChanged, NotifyMemberJoined, NotifyAnnouncement, NotifyReminder,
	NotifyProposalNew, NotifyModeration,
}

// NotificationTypesInUse are the types the log reader can produce today; the
// settings screen offers exactly these.
var NotificationTypesInUse = []NotificationType{
	NotifyMessageNew, NotifyMessageReply, NotifyMaterialAdded, NotifyMaterialBatch,
	NotifyTaskCreated, NotifyTaskPinned, NotifyTaskDueSoon, NotifyTaskOverdue, NotifyTaskStatus,
	NotifyScheduleChanged, NotifyMemberJoined,
}

// Valid reports whether t is a known type.
func (t NotificationType) Valid() bool {
	for _, k := range AllNotificationTypes {
		if k == t {
			return true
		}
	}
	return false
}

// DefaultOn is what a member hears about before touching any setting. A reply
// to one's own message always comes through; everything else follows the type.
// A DPO group starts silent except for announcements (docs/PLAN.md §8.2).
func (t NotificationType) DefaultOn(kind GroupKind, dpoSilent bool) bool {
	if kind == GroupKindDPO && dpoSilent {
		return t == NotifyAnnouncement || t == NotifyMessageReply
	}
	switch t {
	// The group is small: joining and somebody else's progress are noise.
	case NotifyMemberJoined, NotifyTaskStatus:
		return false
	default:
		return true
	}
}

// MuteScope is what a mute silences.
type MuteScope string

// Mute scopes. A mute always ends: "until" is required (docs/PLAN.md §8.2).
const (
	MuteGroup   MuteScope = "GROUP"
	MuteSubject MuteScope = "SUBJECT"
	MuteThread  MuteScope = "THREAD"
	MuteType    MuteScope = "TYPE"
)

// AllMuteScopes lists every scope, exported to packages/shared.
var AllMuteScopes = []MuteScope{MuteGroup, MuteSubject, MuteThread, MuteType}

// Valid reports whether s is a known scope.
func (s MuteScope) Valid() bool {
	for _, k := range AllMuteScopes {
		if k == s {
			return true
		}
	}
	return false
}

// NotifyChannel is a way to reach a member. In-app needs no row: the
// notification itself is the in-app copy.
type NotifyChannel string

// Delivery channels.
const (
	ChannelPush     NotifyChannel = "PUSH"
	ChannelWebPush  NotifyChannel = "WEBPUSH"
	ChannelEmail    NotifyChannel = "EMAIL"
	ChannelTelegram NotifyChannel = "TELEGRAM"
)

// DeliveryStatus is how one attempt ended.
type DeliveryStatus string

// Delivery statuses.
const (
	DeliveryQueued  DeliveryStatus = "QUEUED"
	DeliverySent    DeliveryStatus = "SENT"
	DeliveryFailed  DeliveryStatus = "FAILED"
	DeliverySkipped DeliveryStatus = "SKIPPED"
)

// Notification is one fact addressed to one member.
type Notification struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	GroupID uuid.UUID
	Type    NotificationType
	Title   string
	Body    string
	// Data is the deep link: {"screen":"thread","thread_id":"…"}.
	Data json.RawMessage
	// Seq is the group event that produced it.
	Seq int64
	// DedupeKey makes a repeated pass over the log a no-op.
	DedupeKey string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// NotificationPref switches one type on or off in one group.
type NotificationPref struct {
	UserID  uuid.UUID
	GroupID uuid.UUID
	Type    NotificationType
	Enabled bool
}

// NotificationMute silences a scope until a moment in time.
type NotificationMute struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	GroupID   uuid.UUID
	Scope     MuteScope
	ScopeID   string
	Until     time.Time
	CreatedAt time.Time
}

// NotificationSettings are the personal delivery rules. QuietFrom/QuietTo are
// minutes from midnight in the member's own timezone; nil means no quiet hours.
type NotificationSettings struct {
	UserID      uuid.UUID
	PushEnabled bool
	QuietFrom   *int16
	QuietTo     *int16
	// UrgentInQuiet lets an urgent announcement through the quiet hours.
	// Off by default (D51).
	UrgentInQuiet bool
}

// Quiet reports whether t falls inside the quiet hours read in loc. A window
// whose start is after its end crosses midnight (23:00–08:00).
func (s NotificationSettings) Quiet(t time.Time, loc *time.Location) bool {
	if s.QuietFrom == nil || s.QuietTo == nil || *s.QuietFrom == *s.QuietTo {
		return false
	}
	local := t.In(loc)
	minute := int16(local.Hour()*60 + local.Minute())
	from, to := *s.QuietFrom, *s.QuietTo
	if from < to {
		return minute >= from && minute < to
	}
	return minute >= from || minute < to
}

// PushTarget is one device a notification can be pushed to, with its owner's
// delivery rules attached.
type PushTarget struct {
	DeviceID uuid.UUID
	UserID   uuid.UUID
	Platform string
	Token    string
	Timezone string
	Settings NotificationSettings
}

// NotifyRecipients asks for the members a notification should reach.
type NotifyRecipients struct {
	GroupID uuid.UUID
	Type    NotificationType
	// Exclude is the actor and anybody already handled (a reply's author).
	Exclude []uuid.UUID
	// Default applies to members who never touched this setting.
	Default bool
	// SubjectID and ThreadID, when set, let subject and discussion mutes apply.
	SubjectID *uuid.UUID
	ThreadID  *uuid.UUID
	// Seq of the message: members whose read mark already passed it are left
	// alone — they have seen it on screen.
	Seq int64
	Now time.Time
}

// Recipient is a member a notification goes to; dates in its text are
// rendered in their own timezone.
type Recipient struct {
	UserID   uuid.UUID
	Timezone string
}

// TaskAssignee is one member's part of a task, as reminders see it.
type TaskAssignee struct {
	UserID uuid.UUID
	Status TaskStatus
}

// NotificationFilter pages the in-app list, newest first.
type NotificationFilter struct {
	UserID     uuid.UUID
	GroupID    *uuid.UUID
	UnreadOnly bool
	Before     *time.Time
	BeforeID   *uuid.UUID
	Limit      int32
}

// NotifyMessage is a message as a notification needs it: who wrote what, where.
type NotifyMessage struct {
	ID            uuid.UUID
	ThreadID      uuid.UUID
	Seq           int64
	AuthorID      *uuid.UUID
	AuthorName    string
	Body          string
	ReplyToID     *uuid.UUID
	ReplyAuthorID *uuid.UUID
	Deleted       bool
	TargetType    ThreadTarget
	TargetID      uuid.UUID
	SubjectID     *uuid.UUID
	SubjectName   string
	Title         string
}

// NotifyTask is a task as a notification needs it.
type NotifyTask struct {
	ID         uuid.UUID
	GroupID    uuid.UUID
	SubjectID  *uuid.UUID
	CreatedBy  *uuid.UUID
	Title      string
	AssignMode TaskAssignMode
	Visibility TaskVisibility
	DueAt      *time.Time
}
