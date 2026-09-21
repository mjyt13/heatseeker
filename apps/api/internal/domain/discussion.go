package domain

import (
	"time"

	"github.com/google/uuid"
)

// ThreadTarget is what a discussion is about.
type ThreadTarget string

// Thread targets. LESSON arrives with the schedule (stage 3) and PROPOSAL
// with proposals (stage 4); the rest are served now.
const (
	ThreadSubject  ThreadTarget = "SUBJECT"
	ThreadLesson   ThreadTarget = "LESSON"
	ThreadMaterial ThreadTarget = "MATERIAL"
	ThreadTask     ThreadTarget = "TASK"
	ThreadProposal ThreadTarget = "PROPOSAL"
	ThreadGeneral  ThreadTarget = "GENERAL"
)

// AllThreadTargets lists every target, exported to packages/shared.
var AllThreadTargets = []ThreadTarget{ThreadSubject, ThreadLesson, ThreadMaterial, ThreadTask, ThreadProposal, ThreadGeneral}

// Thread is one discussion. It exists once somebody has written in it.
type Thread struct {
	ID            uuid.UUID
	GroupID       uuid.UUID
	TargetType    ThreadTarget
	TargetID      uuid.UUID
	SubjectID     *uuid.UUID
	Title         *string
	CreatedBy     *uuid.UUID
	MessageCount  int32
	LastMessageAt *time.Time
	LastSeq       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Message is one post in a thread. Seq is the seq of its message.created
// event and orders the thread.
type Message struct {
	ID             uuid.UUID
	GroupID        uuid.UUID
	ThreadID       uuid.UUID
	AuthorID       *uuid.UUID
	ClientID       uuid.UUID
	Seq            int64
	Body           string
	ReplyToID      *uuid.UUID
	EditedAt       *time.Time
	DeletedAt      *time.Time
	HiddenForAllBy *uuid.UUID
	HiddenForAllAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// MessageReply is the quoted message a reply points to.
type MessageReply struct {
	ID           uuid.UUID
	AuthorID     *uuid.UUID
	AuthorName   string
	Body         string
	Deleted      bool
	HiddenForAll bool
}

// MessageView is a message as one member reads it, before the service masks
// what that member may not see.
type MessageView struct {
	Message
	AuthorName string
	HiddenByMe bool
	Reply      *MessageReply
	// Thread is set by the "hidden by me" list, which spans threads.
	Thread *Thread
}

// MessageFilter pages a thread by seq: BeforeSeq loads older messages,
// AfterSeq newer ones; neither means the newest page.
type MessageFilter struct {
	ThreadID  uuid.UUID
	ViewerID  uuid.UUID
	BeforeSeq *int64
	AfterSeq  *int64
	Limit     int32
}

// MessagePreview is the latest visible message of a thread.
type MessagePreview struct {
	AuthorID   *uuid.UUID
	AuthorName string
	Body       string
	CreatedAt  time.Time
}

// ThreadSummary is a thread with one member's unread count.
type ThreadSummary struct {
	Thread
	Unread int32
	Last   *MessagePreview
}
