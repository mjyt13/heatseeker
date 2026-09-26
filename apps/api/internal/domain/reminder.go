package domain

import (
	"time"

	"github.com/google/uuid"
)

// Announcement is a message from the headman to the whole group. Unlike a
// discussion it is one-way and always reaches everybody.
type Announcement struct {
	ID       uuid.UUID
	GroupID  uuid.UUID
	AuthorID *uuid.UUID
	Title    string
	Body     string
	// Urgent announcements reach the phone even during quiet hours.
	Urgent bool
	// Pinned ones stay on top until taken down.
	Pinned    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// AnnouncementView is an announcement with its author's name.
type AnnouncementView struct {
	Announcement
	AuthorName string
}

// ReminderRepeat says whether a reminder comes back.
type ReminderRepeat string

// Repeats. A full RRULE is not needed here: a reminder is a note to self.
const (
	RepeatNone    ReminderRepeat = "NONE"
	RepeatDaily   ReminderRepeat = "DAILY"
	RepeatWeekly  ReminderRepeat = "WEEKLY"
	RepeatMonthly ReminderRepeat = "MONTHLY"
)

// AllReminderRepeats lists the repeats, exported to packages/shared.
var AllReminderRepeats = []ReminderRepeat{RepeatNone, RepeatDaily, RepeatWeekly, RepeatMonthly}

// Valid reports whether r is a known repeat.
func (r ReminderRepeat) Valid() bool {
	for _, k := range AllReminderRepeats {
		if k == r {
			return true
		}
	}
	return false
}

// Next moves a moment one period forward in loc, keeping the wall clock:
// a daily reminder at 9:00 stays at 9:00 across a DST change.
func (r ReminderRepeat) Next(at time.Time, loc *time.Location) (time.Time, bool) {
	local := at.In(loc)
	switch r {
	case RepeatDaily:
		return local.AddDate(0, 0, 1), true
	case RepeatWeekly:
		return local.AddDate(0, 0, 7), true
	case RepeatMonthly:
		return local.AddDate(0, 1, 0), true
	default:
		return at, false
	}
}

// ReminderStatus is where a reminder stands.
type ReminderStatus string

// Reminder statuses.
const (
	ReminderScheduled ReminderStatus = "SCHEDULED"
	ReminderSent      ReminderStatus = "SENT"
	ReminderDone      ReminderStatus = "DONE"
)

// AllReminderStatuses lists the statuses, exported to packages/shared.
var AllReminderStatuses = []ReminderStatus{ReminderScheduled, ReminderSent, ReminderDone}

// ReminderTarget is what a reminder is about, for the deep link.
type ReminderTarget string

// Reminder targets.
const (
	ReminderTask     ReminderTarget = "TASK"
	ReminderMaterial ReminderTarget = "MATERIAL"
	ReminderSchedule ReminderTarget = "SCHEDULE"
)

// AllReminderTargets lists the targets, exported to packages/shared.
var AllReminderTargets = []ReminderTarget{ReminderTask, ReminderMaterial, ReminderSchedule}

// Valid reports whether t is a known target.
func (t ReminderTarget) Valid() bool {
	for _, k := range AllReminderTargets {
		if k == t {
			return true
		}
	}
	return false
}

// Reminder is one person's own note to self at a moment in time. Its text is
// personal: it never enters the group log.
type Reminder struct {
	ID         uuid.UUID
	GroupID    uuid.UUID
	UserID     uuid.UUID
	Title      string
	Note       string
	RemindAt   time.Time
	Repeat     ReminderRepeat
	TargetType *ReminderTarget
	TargetID   *uuid.UUID
	Status     ReminderStatus
	LastFired  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
