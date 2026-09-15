package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type eventRepo struct{ s *Store }

func toEvent(e sqlcgen.GroupEvent) *domain.Event {
	return &domain.Event{
		ID:         e.ID,
		GroupID:    e.GroupID,
		Seq:        e.Seq,
		Kind:       domain.EventKind(e.Kind),
		ActorID:    e.ActorID,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		Payload:    json.RawMessage(rawJSON(e.Payload)),
		Audit:      e.Audit,
		CreatedAt:  e.CreatedAt,
	}
}

func (r *eventRepo) Insert(ctx context.Context, e domain.Event) (*domain.Event, error) {
	row, err := r.s.queries(ctx).InsertGroupEvent(ctx, sqlcgen.InsertGroupEventParams{
		ID:         e.ID,
		GroupID:    e.GroupID,
		Seq:        e.Seq,
		Kind:       string(e.Kind),
		ActorID:    e.ActorID,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		Payload:    rawJSON(e.Payload),
		Audit:      e.Audit,
	})
	if err != nil {
		return nil, mapErr(err, "event")
	}
	return toEvent(row), nil
}

func (r *eventRepo) ListSince(ctx context.Context, groupID uuid.UUID, since int64, limit int32) ([]domain.Event, error) {
	rows, err := r.s.queries(ctx).ListGroupEventsSince(ctx, sqlcgen.ListGroupEventsSinceParams{GroupID: groupID, Seq: since, Limit: limit})
	if err != nil {
		return nil, mapErr(err, "event")
	}
	return toEvents(rows), nil
}

func (r *eventRepo) ListAudit(ctx context.Context, groupID uuid.UUID, beforeSeq *int64, limit int32) ([]domain.Event, error) {
	rows, err := r.s.queries(ctx).ListGroupAuditEvents(ctx, sqlcgen.ListGroupAuditEventsParams{GroupID: groupID, Limit: limit, BeforeSeq: beforeSeq})
	if err != nil {
		return nil, mapErr(err, "event")
	}
	return toEvents(rows), nil
}

func (r *eventRepo) OldestSeq(ctx context.Context, groupID uuid.UUID) (int64, error) {
	seq, err := r.s.queries(ctx).OldestGroupEventSeq(ctx, groupID)
	return seq, mapErr(err, "event")
}

func (r *eventRepo) DeleteBefore(ctx context.Context, t time.Time) (int64, error) {
	n, err := r.s.queries(ctx).DeleteGroupEventsBefore(ctx, t)
	return n, mapErr(err, "event")
}

func toEvents(rows []sqlcgen.GroupEvent) []domain.Event {
	out := make([]domain.Event, len(rows))
	for i, row := range rows {
		out[i] = *toEvent(row)
	}
	return out
}
