package domain

import (
	"time"

	"github.com/google/uuid"
)

// Subject is a course inside a group. Materials, tasks, schedule events and
// discussion threads hang off subjects; aliases feed the Drive classifier.
type Subject struct {
	ID             uuid.UUID
	GroupID        uuid.UUID
	Name           string
	ShortName      *string
	Teacher        *string
	TeacherContact *string
	Color          *string
	Semester       *string
	Aliases        []string
	SortOrder      int32
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ArchivedAt     *time.Time
}

// TagKind classifies tags. SUBJECT tags mirror subjects one-to-one and are
// managed automatically.
type TagKind string

// Tag kinds.
const (
	TagKindSubject TagKind = "SUBJECT"
	TagKindTopic   TagKind = "TOPIC"
	TagKindType    TagKind = "TYPE"
	TagKindSystem  TagKind = "SYSTEM"
	TagKindCustom  TagKind = "CUSTOM"
)

// Tag is a label attachable to materials, tasks and proposals.
type Tag struct {
	ID        uuid.UUID
	GroupID   uuid.UUID
	Name      string
	Slug      string
	Color     *string
	Kind      TagKind
	SubjectID *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// QuickTag is a chip shown on the home screen: either a subject tag or a
// virtual system filter such as "mine" or "saved".
type QuickTag struct {
	Key     string // stable identifier: "subject:<id>" or "system:<name>"
	Label   string // display name (system labels are i18n keys)
	Color   *string
	Kind    TagKind
	TagID   *uuid.UUID
	Subject *uuid.UUID
	Order   int
}
