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
