package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventKind names what happened. Kinds are dotted "<entity>.<verb>".
type EventKind string

// Event kinds. Later stages add more; keep AllEventKinds in sync.
const (
	EventGroupCreated    EventKind = "group.created"
	EventGroupUpdated    EventKind = "group.updated"
	EventMemberJoined    EventKind = "member.joined"
	EventMemberRoles     EventKind = "member.roles_changed"
	EventMemberStatus    EventKind = "member.status_changed"
	EventSubjectCreated  EventKind = "subject.created"
	EventSubjectUpdated  EventKind = "subject.updated"
	EventSubjectArchived EventKind = "subject.archived"
	EventSubjectRestored EventKind = "subject.restored"
	EventTagCreated      EventKind = "tag.created"
	EventTagUpdated      EventKind = "tag.updated"
	EventTagDeleted      EventKind = "tag.deleted"
	EventInviteCreated   EventKind = "invite.created"
	EventInviteRevoked   EventKind = "invite.revoked"
	EventJoinCodeRotated EventKind = "group.join_code_rotated"

	EventMaterialAdded      EventKind = "material.added"
	EventMaterialUpdated    EventKind = "material.updated"
	EventMaterialClassified EventKind = "material.classified"
	// EventMaterialsBulkClassified summarises an Inbox bulk action.
	EventMaterialsBulkClassified EventKind = "material.bulk_classified"
	EventMaterialArchived        EventKind = "material.archived"
	EventMaterialRestored        EventKind = "material.restored"
	EventMaterialDeleted         EventKind = "material.deleted"
	EventDriveConnected          EventKind = "drive.connected"
	EventDriveDisconnected       EventKind = "drive.disconnected"
	EventDriveSynced             EventKind = "drive.synced"
	// A publishing Google account was connected to or removed from the group.
	EventDrivePublisherConnected    EventKind = "drive.publisher_connected"
	EventDrivePublisherDisconnected EventKind = "drive.publisher_disconnected"
	// EventDriveReclassified summarises Inbox files sorted automatically after
	// subjects or aliases changed.
	EventDriveReclassified EventKind = "drive.reclassified"

	EventTaskCreated       EventKind = "task.created"
	EventTaskUpdated       EventKind = "task.updated"
	EventTaskStatusChanged EventKind = "task.status_changed"
	EventTaskPinned        EventKind = "task.pinned"
	EventTaskUnpinned      EventKind = "task.unpinned"
	EventTaskDeleted       EventKind = "task.deleted"
	// EventTaskDueSoon and EventTaskOverdue are announced by the deadline
	// scanner, once per offset per task.
	EventTaskDueSoon EventKind = "task.due_soon"
	EventTaskOverdue EventKind = "task.overdue"

	// Message events carry ids only, never the text (D40); message.created,
	// message.updated and message.deleted stay out of the activity feed.
	EventMessageCreated EventKind = "message.created"
	EventMessageUpdated EventKind = "message.updated"
	EventMessageDeleted EventKind = "message.deleted"
	// The author brought back their deleted message.
	EventMessageUndeleted EventKind = "message.undeleted"
	// A moderator hid a message for everybody, or brought it back.
	EventMessageHidden   EventKind = "message.hidden"
	EventMessageRestored EventKind = "message.restored"

	// A class (one-off or a series) was added, edited or removed; editing
	// "from this date on" ends the old series and adds a new one.
	EventScheduleCreated EventKind = "schedule.created"
	EventScheduleUpdated EventKind = "schedule.updated"
	EventScheduleDeleted EventKind = "schedule.deleted"
	// One class was cancelled, changed (time, room, teacher) or put back as planned.
	EventScheduleCancelled EventKind = "schedule.cancelled"
	EventScheduleChanged   EventKind = "schedule.changed"
	EventScheduleReset     EventKind = "schedule.reset"
)

// AllEventKinds lists every kind, exported to packages/shared.
var AllEventKinds = []EventKind{
	EventGroupCreated, EventGroupUpdated, EventMemberJoined, EventMemberRoles, EventMemberStatus,
	EventSubjectCreated, EventSubjectUpdated, EventSubjectArchived, EventSubjectRestored,
	EventTagCreated, EventTagUpdated, EventTagDeleted, EventInviteCreated, EventInviteRevoked,
	EventJoinCodeRotated,
	EventMaterialAdded, EventMaterialUpdated, EventMaterialClassified, EventMaterialsBulkClassified, EventMaterialArchived,
	EventMaterialRestored, EventMaterialDeleted,
	EventDriveConnected, EventDriveDisconnected, EventDriveSynced, EventDriveReclassified,
	EventDrivePublisherConnected, EventDrivePublisherDisconnected,
	EventTaskCreated, EventTaskUpdated, EventTaskStatusChanged, EventTaskPinned, EventTaskUnpinned,
	EventTaskDeleted, EventTaskDueSoon, EventTaskOverdue,
	EventMessageCreated, EventMessageUpdated, EventMessageDeleted, EventMessageUndeleted, EventMessageHidden,
	EventMessageRestored,
	EventScheduleCreated, EventScheduleUpdated, EventScheduleDeleted, EventScheduleCancelled, EventScheduleChanged,
	EventScheduleReset,
}

// Event is one row of the append-only group log. Seq is monotonic per group
// and is the cursor for realtime and offline sync.
type Event struct {
	ID         uuid.UUID
	GroupID    uuid.UUID
	Seq        int64
	Kind       EventKind
	ActorID    *uuid.UUID
	EntityType string
	EntityID   *uuid.UUID
	Payload    json.RawMessage
	Audit      bool
	CreatedAt  time.Time
}

// NewEvent builds an event without Seq/ID; the publisher assigns them.
func NewEvent(groupID uuid.UUID, kind EventKind, actor *uuid.UUID, entityType string, entityID *uuid.UUID, payload any) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	if payload == nil {
		raw = json.RawMessage("{}")
	}
	return Event{
		GroupID:    groupID,
		Kind:       kind,
		ActorID:    actor,
		EntityType: entityType,
		EntityID:   entityID,
		Payload:    raw,
		Audit:      true,
	}, nil
}
