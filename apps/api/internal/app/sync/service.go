// Package sync serves the group event log to clients: delta sync for offline
// clients and the human-readable activity feed.
package sync

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
)

// Service reads the event log.
type Service struct {
	events domain.EventRepo
	access *access.Service
}

// NewService wires the service.
func NewService(events domain.EventRepo, acc *access.Service) *Service {
	return &Service{events: events, access: acc}
}

// Result is a page of the log after a cursor.
type Result struct {
	Events  []domain.Event
	NextSeq int64 // cursor for the next call
	Latest  int64 // newest seq in the group at the time of the call
	HasMore bool
}

const (
	defaultLimit = 200
	maxLimit     = 1000
)

// Since returns events with seq > since. ErrGone signals that the cursor is
// older than retained history and the client must reload from scratch.
func (s *Service) Since(ctx context.Context, actorID, groupID uuid.UUID, since int64, limit int32) (*Result, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	if since < 0 {
		return nil, domain.Invalid("since", "must be >= 0")
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	oldest, err := s.events.OldestSeq(ctx, groupID)
	if err != nil {
		return nil, err
	}
	// Retention pruned events the client has not seen: gap between since and
	// the oldest retained seq means data loss unless since >= oldest-1.
	if since > 0 && oldest > 1 && since < oldest-1 {
		return nil, fmt.Errorf("%w: history before seq %d is no longer available", domain.ErrGone, oldest)
	}
	items, err := s.events.ListSince(ctx, groupID, since, limit+1)
	if err != nil {
		return nil, err
	}
	res := &Result{Events: items, NextSeq: since, Latest: actor.Group.LastSeq}
	if int32(len(items)) > limit {
		res.HasMore = true
		res.Events = items[:limit]
	}
	if n := len(res.Events); n > 0 {
		res.NextSeq = res.Events[n-1].Seq
	}
	return res, nil
}

// Activity returns the audit feed, newest first.
func (s *Service) Activity(ctx context.Context, actorID, groupID uuid.UUID, beforeSeq *int64, limit int32) ([]domain.Event, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.events.ListAudit(ctx, groupID, beforeSeq, limit)
}
