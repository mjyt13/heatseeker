package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventKind names what happened. Kinds are dotted "<entity>.<verb>".
type EventKind string

// Event kinds emitted in the first stage. Later stages add more.
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
)

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
