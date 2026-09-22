package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type scheduleRepo struct{ s *Store }

func toScheduleEvent(e sqlcgen.ScheduleEvent) (*domain.ScheduleEvent, error) {
	out := &domain.ScheduleEvent{
		ID: e.ID, GroupID: e.GroupID, SubjectID: e.SubjectID, ClientID: e.ClientID, Title: e.Title,
		Kind: domain.ScheduleKind(e.Kind), StartsAt: e.StartsAt, EndsAt: e.EndsAt, Timezone: e.Timezone,
		Location: e.Location, Teacher: e.Teacher, Note: e.Note, Until: e.RruleUntil,
		CreatedBy: e.CreatedBy, UpdatedBy: e.UpdatedBy, Version: e.Version,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
	if e.Rrule != nil {
		r, err := domain.ParseRRule(*e.Rrule)
		if err != nil {
			return nil, fmt.Errorf("schedule event %s: %w", e.ID, err)
		}
		out.Repeat = r
	}
	return out, nil
}

func toScheduleEvents(rows []sqlcgen.ScheduleEvent) ([]domain.ScheduleEvent, error) {
	out := make([]domain.ScheduleEvent, 0, len(rows))
	for _, row := range rows {
		e, err := toScheduleEvent(row)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, nil
}

func toScheduleException(x sqlcgen.ScheduleException) domain.ScheduleException {
	return domain.ScheduleException{
		EventID: x.EventID, Date: x.OriginalDate, Kind: domain.ScheduleExceptionKind(x.Kind),
		StartsAt: x.StartsAt, EndsAt: x.EndsAt, Location: x.Location, Teacher: x.Teacher, Note: x.Note,
		UpdatedBy: x.UpdatedBy, UpdatedAt: x.UpdatedAt,
	}
}

func rrule(e *domain.ScheduleEvent) *string {
	if e.Repeat == nil {
		return nil
	}
	v := e.Repeat.RRule()
	return &v
}

func (r *scheduleRepo) Create(ctx context.Context, e domain.ScheduleEvent) (*domain.ScheduleEvent, error) {
	row, err := r.s.queries(ctx).CreateScheduleEvent(ctx, sqlcgen.CreateScheduleEventParams{
		ID: e.ID, GroupID: e.GroupID, SubjectID: e.SubjectID, ClientID: e.ClientID, Title: e.Title,
		Kind: string(e.Kind), StartsAt: e.StartsAt, EndsAt: e.EndsAt, Timezone: e.Timezone,
		Location: e.Location, Teacher: e.Teacher, Note: e.Note, Rrule: rrule(&e), RruleUntil: e.Until,
		CreatedBy: e.CreatedBy,
	})
	if err != nil {
		return nil, mapErr(err, "schedule event")
	}
	return toScheduleEvent(row)
}

func (r *scheduleRepo) Get(ctx context.Context, id uuid.UUID) (*domain.ScheduleEvent, error) {
	row, err := r.s.queries(ctx).GetScheduleEvent(ctx, id)
	if err != nil {
		return nil, mapErr(err, "schedule event")
	}
	return toScheduleEvent(row)
}

func (r *scheduleRepo) GetByClientID(ctx context.Context, groupID, clientID uuid.UUID) (*domain.ScheduleEvent, error) {
	row, err := r.s.queries(ctx).GetScheduleEventByClientID(ctx, sqlcgen.GetScheduleEventByClientIDParams{
		GroupID: groupID, ClientID: &clientID,
	})
	if err != nil {
		return nil, mapErr(err, "schedule event")
	}
	return toScheduleEvent(row)
}

func (r *scheduleRepo) ListInWindow(ctx context.Context, groupID uuid.UUID, from, to time.Time) ([]domain.ScheduleEvent, error) {
	rows, err := r.s.queries(ctx).ListScheduleEventsInWindow(ctx, sqlcgen.ListScheduleEventsInWindowParams{
		GroupID: groupID, WindowFrom: from, WindowTo: to,
	})
	if err != nil {
		return nil, mapErr(err, "schedule events")
	}
	return toScheduleEvents(rows)
}

func (r *scheduleRepo) ListAll(ctx context.Context, groupID uuid.UUID) ([]domain.ScheduleEvent, error) {
	rows, err := r.s.queries(ctx).ListGroupScheduleEvents(ctx, groupID)
	if err != nil {
		return nil, mapErr(err, "schedule events")
	}
	return toScheduleEvents(rows)
}

// staleOrMissing tells a lost race from a missing event after a versioned
// update touched nothing.
func (r *scheduleRepo) staleOrMissing(ctx context.Context, id uuid.UUID) error {
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	return domain.Conflict("the class was changed by someone else: reload it and try again")
}

func (r *scheduleRepo) Update(ctx context.Context, e domain.ScheduleEvent) (*domain.ScheduleEvent, error) {
	row, err := r.s.queries(ctx).UpdateScheduleEvent(ctx, sqlcgen.UpdateScheduleEventParams{
		ID: e.ID, Version: e.Version, SubjectID: e.SubjectID, Title: e.Title, Kind: string(e.Kind),
		StartsAt: e.StartsAt, EndsAt: e.EndsAt, Timezone: e.Timezone, Location: e.Location, Teacher: e.Teacher,
		Note: e.Note, Rrule: rrule(&e), RruleUntil: e.Until, UpdatedBy: e.UpdatedBy,
	})
	if errors.Is(mapErr(err, ""), domain.ErrNotFound) {
		return nil, r.staleOrMissing(ctx, e.ID)
	}
	if err != nil {
		return nil, mapErr(err, "schedule event")
	}
	return toScheduleEvent(row)
}

func (r *scheduleRepo) EndSeries(ctx context.Context, id uuid.UUID, version int32, until time.Time, by uuid.UUID) (*domain.ScheduleEvent, error) {
	row, err := r.s.queries(ctx).EndScheduleSeries(ctx, sqlcgen.EndScheduleSeriesParams{
		ID: id, Version: version, Until: until, UpdatedBy: &by,
	})
	if errors.Is(mapErr(err, ""), domain.ErrNotFound) {
		return nil, r.staleOrMissing(ctx, id)
	}
	if err != nil {
		return nil, mapErr(err, "schedule event")
	}
	return toScheduleEvent(row)
}

func (r *scheduleRepo) SoftDelete(ctx context.Context, id uuid.UUID, version int32, at time.Time, by uuid.UUID) error {
	n, err := r.s.queries(ctx).SoftDeleteScheduleEvent(ctx, sqlcgen.SoftDeleteScheduleEventParams{
		ID: id, Version: version, DeletedAt: &at, UpdatedBy: &by,
	})
	if err != nil {
		return mapErr(err, "schedule event")
	}
	if n == 0 {
		return r.staleOrMissing(ctx, id)
	}
	return nil
}

func (r *scheduleRepo) ListExceptions(ctx context.Context, eventIDs []uuid.UUID) ([]domain.ScheduleException, error) {
	if len(eventIDs) == 0 {
		return nil, nil
	}
	rows, err := r.s.queries(ctx).ListScheduleExceptions(ctx, eventIDs)
	if err != nil {
		return nil, mapErr(err, "schedule exceptions")
	}
	out := make([]domain.ScheduleException, len(rows))
	for i, row := range rows {
		out[i] = toScheduleException(row)
	}
	return out, nil
}

func (r *scheduleRepo) GetException(ctx context.Context, eventID uuid.UUID, date time.Time) (*domain.ScheduleException, error) {
	row, err := r.s.queries(ctx).GetScheduleException(ctx, sqlcgen.GetScheduleExceptionParams{
		EventID: eventID, OriginalDate: date,
	})
	if err != nil {
		return nil, mapErr(err, "schedule exception")
	}
	x := toScheduleException(row)
	return &x, nil
}

func (r *scheduleRepo) PutException(ctx context.Context, x domain.ScheduleException) (*domain.ScheduleException, error) {
	row, err := r.s.queries(ctx).UpsertScheduleException(ctx, sqlcgen.UpsertScheduleExceptionParams{
		EventID: x.EventID, OriginalDate: x.Date, Kind: string(x.Kind), StartsAt: x.StartsAt, EndsAt: x.EndsAt,
		Location: x.Location, Teacher: x.Teacher, Note: x.Note, UpdatedBy: x.UpdatedBy,
	})
	if err != nil {
		return nil, mapErr(err, "schedule exception")
	}
	out := toScheduleException(row)
	return &out, nil
}

func (r *scheduleRepo) DeleteException(ctx context.Context, eventID uuid.UUID, date time.Time) error {
	n, err := r.s.queries(ctx).DeleteScheduleException(ctx, sqlcgen.DeleteScheduleExceptionParams{
		EventID: eventID, OriginalDate: date,
	})
	if err != nil {
		return mapErr(err, "schedule exception")
	}
	if n == 0 {
		return domain.NotFound("schedule exception")
	}
	return nil
}

func (r *scheduleRepo) MoveExceptions(ctx context.Context, fromEventID, toEventID uuid.UUID, fromDate time.Time) error {
	return mapErr(r.s.queries(ctx).MoveScheduleExceptions(ctx, sqlcgen.MoveScheduleExceptionsParams{
		FromEventID: fromEventID, ToEventID: toEventID, FromDate: fromDate,
	}), "schedule exceptions")
}

func (r *scheduleRepo) DeleteExceptionsFrom(ctx context.Context, eventID uuid.UUID, fromDate time.Time) error {
	return mapErr(r.s.queries(ctx).DeleteScheduleExceptionsFrom(ctx, sqlcgen.DeleteScheduleExceptionsFromParams{
		EventID: eventID, FromDate: fromDate,
	}), "schedule exceptions")
}
