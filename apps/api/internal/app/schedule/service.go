// Package schedule implements the group timetable: one-off classes and weekly
// series, cancellations and changes of single classes, and a calendar feed
// (docs/PLAN.md §9, stage 3). The headman and admins edit it directly (D22);
// everybody reads it.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
	"heatseeker/api/internal/platform/signed"
)

// Settings come from configuration.
type Settings struct {
	// APIBaseURL is the public origin + "/api/v1", for calendar links.
	APIBaseURL    string
	SigningSecret string
	// CalendarLinkTTL is how long a calendar subscription link works.
	CalendarLinkTTL time.Duration
}

// Deps are the collaborators of the service.
type Deps struct {
	Schedule domain.ScheduleRepo
	Subjects domain.SubjectRepo
	Access   *access.Service
	Events   *events.Publisher
	Tx       domain.TxManager
	Clock    clock.Clock
	Log      *slog.Logger
}

// Service is the schedule use-case layer.
type Service struct {
	Deps
	cfg       Settings
	calendars *signed.Signer
}

// NewService wires the service.
func NewService(d Deps, cfg Settings) *Service {
	cfg.APIBaseURL = strings.TrimRight(cfg.APIBaseURL, "/")
	if cfg.CalendarLinkTTL <= 0 {
		cfg.CalendarLinkTTL = 365 * 24 * time.Hour
	}
	return &Service{Deps: d, cfg: cfg, calendars: signed.New(cfg.SigningSecret, "schedule-calendar")}
}

const (
	// MaxWindow bounds one schedule request: a term fits.
	MaxWindow = 190 * 24 * time.Hour
	// maxDuration bounds one class.
	maxDuration = 24 * time.Hour
	// maxSeriesYears bounds how far a series may be planned.
	maxSeriesYears = 2
)

// Input is the class form. It fully replaces the stored values on update.
type Input struct {
	// ClientID makes creation idempotent when the phone repeats the request.
	ClientID  *uuid.UUID
	SubjectID *uuid.UUID
	Title     string
	Kind      *domain.ScheduleKind
	// StartsAt and EndsAt are the first class of a series.
	StartsAt time.Time
	EndsAt   time.Time
	// Timezone is the IANA zone whose wall clock the series keeps.
	Timezone string
	Location string
	Teacher  string
	Note     string
	Repeat   *domain.Recurrence
	// Until is the last date of a series (civil), nil — no end.
	Until *time.Time
}

// Scope says which classes of a series an edit touches.
type Scope string

// Edit scopes. A single class is changed with SetOccurrence.
const (
	ScopeAll       Scope = "ALL"
	ScopeFollowing Scope = "FOLLOWING"
)

// UpdateInput edits a class or a series.
type UpdateInput struct {
	Input
	// Version is the one the editor loaded.
	Version int32
	Scope   Scope
	// From is the first date the FOLLOWING scope changes (civil).
	From *time.Time
}

// OccurrenceInput cancels or changes one class. Nil overrides keep the
// series values.
type OccurrenceInput struct {
	Cancelled bool
	StartsAt  *time.Time
	EndsAt    *time.Time
	Location  *string
	Teacher   *string
	Note      string
}

// Details is a class with the changes of its single occurrences.
type Details struct {
	Event      domain.ScheduleEvent
	Exceptions []domain.ScheduleException
}

// Window returns the classes overlapping [from, to), cancelled ones
// included, ordered by start.
func (s *Service) Window(ctx context.Context, actorID, groupID uuid.UUID, from, to time.Time) ([]domain.Occurrence, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.ScheduleRead); err != nil {
		return nil, err
	}
	if !to.After(from) {
		return nil, domain.Invalid("to", "must be after from")
	}
	if to.Sub(from) > MaxWindow {
		return nil, domain.Invalid("to", fmt.Sprintf("the window is at most %d days", int(MaxWindow.Hours()/24)))
	}
	list, err := s.Schedule.ListInWindow(ctx, groupID, from, to)
	if err != nil {
		return nil, err
	}
	return s.expand(ctx, list, from, to)
}

// expand turns events into their classes inside [from, to).
func (s *Service) expand(ctx context.Context, list []domain.ScheduleEvent, from, to time.Time) ([]domain.Occurrence, error) {
	byEvent, err := s.exceptionsOf(ctx, list)
	if err != nil {
		return nil, err
	}
	var out []domain.Occurrence
	for i := range list {
		occ, err := list[i].Occurrences(from, to, byEvent[list[i].ID])
		if err != nil {
			s.Log.Warn("skip schedule event", "event", list[i].ID, "err", err)
			continue
		}
		out = append(out, occ...)
	}
	domain.SortOccurrences(out)
	return out, nil
}

func (s *Service) exceptionsOf(ctx context.Context, list []domain.ScheduleEvent) (map[uuid.UUID][]domain.ScheduleException, error) {
	eventIDs := make([]uuid.UUID, len(list))
	for i := range list {
		eventIDs[i] = list[i].ID
	}
	all, err := s.Schedule.ListExceptions(ctx, eventIDs)
	if err != nil {
		return nil, err
	}
	byEvent := map[uuid.UUID][]domain.ScheduleException{}
	for _, x := range all {
		byEvent[x.EventID] = append(byEvent[x.EventID], x)
	}
	return byEvent, nil
}

// Get returns a class with its exceptions.
func (s *Service) Get(ctx context.Context, actorID, eventID uuid.UUID) (*Details, error) {
	e, _, err := s.load(ctx, actorID, eventID)
	if err != nil {
		return nil, err
	}
	return s.details(ctx, e)
}

func (s *Service) details(ctx context.Context, e *domain.ScheduleEvent) (*Details, error) {
	exceptions, err := s.Schedule.ListExceptions(ctx, []uuid.UUID{e.ID})
	if err != nil {
		return nil, err
	}
	return &Details{Event: *e, Exceptions: exceptions}, nil
}

// load returns an event the actor may read.
func (s *Service) load(ctx context.Context, actorID, eventID uuid.UUID) (*domain.ScheduleEvent, *access.Actor, error) {
	e, err := s.Schedule.Get(ctx, eventID)
	if err != nil {
		return nil, nil, err
	}
	actor, err := s.Access.Actor(ctx, actorID, e.GroupID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return nil, nil, domain.NotFound("schedule event")
		}
		return nil, nil, err
	}
	if err := actor.Require(authz.ScheduleRead); err != nil {
		return nil, nil, err
	}
	return e, actor, nil
}

// Create adds a class or a series (authz.ScheduleEdit).
func (s *Service) Create(ctx context.Context, actorID, groupID uuid.UUID, in Input) (*Details, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.ScheduleEdit); err != nil {
		return nil, err
	}
	if in.ClientID != nil {
		switch existing, err := s.Schedule.GetByClientID(ctx, groupID, *in.ClientID); {
		case err == nil:
			return s.details(ctx, existing)
		case !errors.Is(err, domain.ErrNotFound):
			return nil, err
		}
	}
	e, err := s.build(ctx, groupID, in)
	if err != nil {
		return nil, err
	}
	e.ID, e.ClientID, e.CreatedBy = ids.New(), in.ClientID, &actorID
	var created *domain.ScheduleEvent
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if created, err = s.Schedule.Create(ctx, e); err != nil {
			return err
		}
		return s.emit(ctx, created, domain.EventScheduleCreated, actorID, nil)
	})
	if err != nil {
		return nil, err
	}
	return &Details{Event: *created}, nil
}

// Update edits a whole series, or its classes from a date on: the series
// then ends the day before and a new one takes over, with the changes of
// single classes from that date.
func (s *Service) Update(ctx context.Context, actorID, eventID uuid.UUID, in UpdateInput) (*Details, error) {
	old, actor, err := s.load(ctx, actorID, eventID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.ScheduleEdit); err != nil {
		return nil, err
	}
	if err := checkVersion(old, in.Version); err != nil {
		return nil, err
	}
	next, err := s.build(ctx, old.GroupID, in.Input)
	if err != nil {
		return nil, err
	}
	split, err := s.splitDate(old, in.Scope, in.From)
	if err != nil {
		return nil, err
	}
	var saved *domain.ScheduleEvent
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if split == nil {
			next.ID, next.Version, next.UpdatedBy = old.ID, in.Version, &actorID
			if saved, err = s.Schedule.Update(ctx, next); err != nil {
				return err
			}
			return s.emit(ctx, saved, domain.EventScheduleUpdated, actorID, nil)
		}
		if _, err := s.Schedule.EndSeries(ctx, old.ID, in.Version, split.AddDate(0, 0, -1), actorID); err != nil {
			return err
		}
		next.ID, next.CreatedBy = ids.New(), &actorID
		if saved, err = s.Schedule.Create(ctx, next); err != nil {
			return err
		}
		if err := s.Schedule.MoveExceptions(ctx, old.ID, saved.ID, *split); err != nil {
			return err
		}
		return s.emit(ctx, saved, domain.EventScheduleUpdated, actorID, map[string]any{
			"split_from": old.ID.String(), "from": split.Format(domain.DateLayout),
		})
	})
	if err != nil {
		return nil, err
	}
	return s.details(ctx, saved)
}

// Delete removes a whole series, or its classes from a date on.
func (s *Service) Delete(ctx context.Context, actorID, eventID uuid.UUID, version int32, scope Scope, from *time.Time) error {
	e, actor, err := s.load(ctx, actorID, eventID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.ScheduleEdit); err != nil {
		return err
	}
	if err := checkVersion(e, version); err != nil {
		return err
	}
	split, err := s.splitDate(e, scope, from)
	if err != nil {
		return err
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if split == nil {
			if err := s.Schedule.SoftDelete(ctx, e.ID, version, s.Clock.Now(), actorID); err != nil {
				return err
			}
			return s.emit(ctx, e, domain.EventScheduleDeleted, actorID, nil)
		}
		if _, err := s.Schedule.EndSeries(ctx, e.ID, version, split.AddDate(0, 0, -1), actorID); err != nil {
			return err
		}
		if err := s.Schedule.DeleteExceptionsFrom(ctx, e.ID, *split); err != nil {
			return err
		}
		return s.emit(ctx, e, domain.EventScheduleDeleted, actorID, map[string]any{"from": split.Format(domain.DateLayout)})
	})
}

// checkVersion reports an edit started from an outdated copy before looking
// at its dates (the repository re-checks it against concurrent writers).
func checkVersion(e *domain.ScheduleEvent, version int32) error {
	if version != e.Version {
		return domain.Conflict("the class was changed by someone else: reload it and try again")
	}
	return nil
}

// splitDate is the date a FOLLOWING edit starts at, or nil when the edit
// touches the whole series (ALL, a one-off class, or the first date).
func (s *Service) splitDate(e *domain.ScheduleEvent, scope Scope, from *time.Time) (*time.Time, error) {
	switch scope {
	case ScopeAll, "":
		return nil, nil
	case ScopeFollowing:
	default:
		return nil, domain.Invalid("scope", "unknown scope "+string(scope))
	}
	if from == nil {
		return nil, domain.Invalid("from", "name the first date to change")
	}
	if e.Repeat == nil {
		return nil, nil
	}
	loc, err := e.Loc()
	if err != nil {
		return nil, err
	}
	date := domain.CivilDate(*from)
	if !date.After(e.FirstDate(loc)) {
		return nil, nil
	}
	if !e.OccursOn(date, loc) {
		return nil, domain.Invalid("from", "there is no class of this series on "+date.Format(domain.DateLayout))
	}
	return &date, nil
}

// GetOccurrence returns the class planned on date, with its change applied.
func (s *Service) GetOccurrence(ctx context.Context, actorID, eventID uuid.UUID, date time.Time) (*domain.Occurrence, error) {
	e, _, err := s.load(ctx, actorID, eventID)
	if err != nil {
		return nil, err
	}
	loc, err := e.Loc()
	if err != nil {
		return nil, err
	}
	day := domain.CivilDate(date)
	if !e.OccursOn(day, loc) {
		return nil, domain.NotFound("class")
	}
	x, err := s.Schedule.GetException(ctx, e.ID, day)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		x = nil
	case err != nil:
		return nil, err
	}
	o := e.Occurrence(day, loc, x)
	return &o, nil
}

// SetOccurrence cancels or changes one class (authz.ScheduleEdit).
func (s *Service) SetOccurrence(ctx context.Context, actorID, eventID uuid.UUID, date time.Time, in OccurrenceInput) (*domain.Occurrence, error) {
	e, loc, err := s.editableClass(ctx, actorID, eventID, date)
	if err != nil {
		return nil, err
	}
	x := domain.ScheduleException{EventID: e.ID, Date: domain.CivilDate(date), UpdatedBy: &actorID}
	if x.Note, err = normalizeText("note", in.Note, 500); err != nil {
		return nil, err
	}
	kind := domain.EventScheduleCancelled
	if in.Cancelled {
		x.Kind = domain.ExceptionCancelled
	} else {
		kind, x.Kind = domain.EventScheduleChanged, domain.ExceptionChanged
		if (in.StartsAt == nil) != (in.EndsAt == nil) {
			return nil, domain.Invalid("ends_at", "set both the start and the end, or neither")
		}
		if in.StartsAt != nil {
			start, end, err := checkTimes(*in.StartsAt, *in.EndsAt)
			if err != nil {
				return nil, err
			}
			x.StartsAt, x.EndsAt = &start, &end
		}
		if x.Location, err = optionalText("location", in.Location, 200); err != nil {
			return nil, err
		}
		if x.Teacher, err = optionalText("teacher", in.Teacher, 200); err != nil {
			return nil, err
		}
		if x.StartsAt == nil && x.Location == nil && x.Teacher == nil && x.Note == "" {
			return nil, domain.Invalid("occurrence", "nothing to change: set a time, a room, a teacher or a note")
		}
	}
	var saved *domain.ScheduleException
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if saved, err = s.Schedule.PutException(ctx, x); err != nil {
			return err
		}
		o := e.Occurrence(x.Date, loc, saved)
		return s.emitOccurrence(ctx, &o, kind, actorID)
	})
	if err != nil {
		return nil, err
	}
	o := e.Occurrence(x.Date, loc, saved)
	return &o, nil
}

// ResetOccurrence puts one class back as the series plans it.
func (s *Service) ResetOccurrence(ctx context.Context, actorID, eventID uuid.UUID, date time.Time) (*domain.Occurrence, error) {
	e, loc, err := s.editableClass(ctx, actorID, eventID, date)
	if err != nil {
		return nil, err
	}
	day := domain.CivilDate(date)
	o := e.Occurrence(day, loc, nil)
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.Schedule.DeleteException(ctx, e.ID, day); err != nil {
			return err
		}
		return s.emitOccurrence(ctx, &o, domain.EventScheduleReset, actorID)
	})
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// editableClass loads an event the actor may edit and checks that it has a
// class on date.
func (s *Service) editableClass(ctx context.Context, actorID, eventID uuid.UUID, date time.Time) (*domain.ScheduleEvent, *time.Location, error) {
	e, actor, err := s.load(ctx, actorID, eventID)
	if err != nil {
		return nil, nil, err
	}
	if err := actor.Require(authz.ScheduleEdit); err != nil {
		return nil, nil, err
	}
	loc, err := e.Loc()
	if err != nil {
		return nil, nil, err
	}
	if !e.OccursOn(domain.CivilDate(date), loc) {
		return nil, nil, domain.Invalid("date", "there is no class of this series on "+date.Format(domain.DateLayout))
	}
	return e, loc, nil
}

// build validates the form into an event (without identity and authorship).
func (s *Service) build(ctx context.Context, groupID uuid.UUID, in Input) (domain.ScheduleEvent, error) {
	e := domain.ScheduleEvent{GroupID: groupID, SubjectID: in.SubjectID, Kind: domain.ScheduleLecture}
	var err error
	if e.Title, err = normalizeText("title", strings.Join(strings.Fields(in.Title), " "), 200); err != nil {
		return e, err
	}
	if e.Title == "" && in.SubjectID == nil {
		return e, domain.Invalid("title", "name the class or pick its subject")
	}
	if in.Kind != nil {
		if !slices.Contains(domain.AllScheduleKinds, *in.Kind) {
			return e, domain.Invalid("kind", "unknown kind "+string(*in.Kind))
		}
		e.Kind = *in.Kind
	}
	if in.Timezone == "" {
		return e, domain.Invalid("timezone", "name the time zone, e.g. Europe/Moscow")
	}
	loc, err := time.LoadLocation(in.Timezone)
	if err != nil {
		return e, domain.Invalid("timezone", "unknown time zone "+in.Timezone)
	}
	e.Timezone = loc.String()
	if e.StartsAt, e.EndsAt, err = checkTimes(in.StartsAt, in.EndsAt); err != nil {
		return e, err
	}
	if e.Location, err = normalizeText("location", in.Location, 200); err != nil {
		return e, err
	}
	if e.Teacher, err = normalizeText("teacher", in.Teacher, 200); err != nil {
		return e, err
	}
	if e.Note, err = normalizeText("note", in.Note, 2000); err != nil {
		return e, err
	}
	if err := s.checkSubject(ctx, groupID, in.SubjectID); err != nil {
		return e, err
	}
	if in.Repeat == nil {
		return e, nil
	}
	r := domain.Recurrence{IntervalWeeks: in.Repeat.IntervalWeeks, Weekdays: slices.Clone(in.Repeat.Weekdays)}
	first := e.FirstDate(loc)
	// The first class always counts, as in RFC 5545.
	if len(r.Weekdays) > 0 && !slices.Contains(r.Weekdays, first.Weekday()) {
		r.Weekdays = append(r.Weekdays, first.Weekday())
	}
	if err := r.Normalize(); err != nil {
		return e, err
	}
	e.Repeat = &r
	if in.Until != nil {
		until := domain.CivilDate(*in.Until)
		if until.Before(first) {
			return e, domain.Invalid("until", "the series cannot end before its first class")
		}
		if until.After(first.AddDate(maxSeriesYears, 0, 0)) {
			return e, domain.Invalid("until", fmt.Sprintf("a series lasts at most %d years", maxSeriesYears))
		}
		e.Until = &until
	}
	return e, nil
}

func checkTimes(start, end time.Time) (time.Time, time.Time, error) {
	start, end = start.UTC().Truncate(time.Minute), end.UTC().Truncate(time.Minute)
	if !end.After(start) {
		return start, end, domain.Invalid("ends_at", "the class must end after it starts")
	}
	if end.Sub(start) > maxDuration {
		return start, end, domain.Invalid("ends_at", "a class lasts at most 24 hours")
	}
	return start, end, nil
}

func normalizeText(field, v string, limit int) (string, error) {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > limit {
		return "", domain.Invalid(field, fmt.Sprintf("must be at most %d characters", limit))
	}
	return v, nil
}

func optionalText(field string, v *string, limit int) (*string, error) {
	if v == nil {
		return nil, nil
	}
	out, err := normalizeText(field, *v, limit)
	return &out, err
}

// checkSubject makes sure the subject belongs to the group.
func (s *Service) checkSubject(ctx context.Context, groupID uuid.UUID, subjectID *uuid.UUID) error {
	if subjectID == nil {
		return nil
	}
	if _, err := s.Subjects.Get(ctx, *subjectID, groupID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Invalid("subject_id", "no such subject in this group")
		}
		return err
	}
	return nil
}

func (s *Service) emit(ctx context.Context, e *domain.ScheduleEvent, kind domain.EventKind, actorID uuid.UUID, extra map[string]any) error {
	payload := map[string]any{
		"title": e.Title, "kind": string(e.Kind), "starts_at": e.StartsAt.UTC().Format(time.RFC3339),
		"recurring": e.Repeat != nil,
	}
	if e.SubjectID != nil {
		payload["subject_id"] = e.SubjectID.String()
	}
	for k, v := range extra {
		payload[k] = v
	}
	ev, err := domain.NewEvent(e.GroupID, kind, &actorID, "schedule_event", &e.ID, payload)
	return s.Events.Emit(ctx, ev, err)
}

func (s *Service) emitOccurrence(ctx context.Context, o *domain.Occurrence, kind domain.EventKind, actorID uuid.UUID) error {
	return s.emit(ctx, o.Event, kind, actorID, map[string]any{
		"date": o.Date.Format(domain.DateLayout), "starts_at": o.StartsAt.UTC().Format(time.RFC3339),
	})
}

type calendarClaims struct {
	User  uuid.UUID `json:"u"`
	Group uuid.UUID `json:"g"`
}

// CalendarLink is a personal link to subscribe to the schedule from a
// calendar app. It works while the member stays in the group.
func (s *Service) CalendarLink(ctx context.Context, actorID, groupID uuid.UUID) (string, time.Time, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := actor.Require(authz.ScheduleRead); err != nil {
		return "", time.Time{}, err
	}
	exp := s.Clock.Now().Add(s.cfg.CalendarLinkTTL)
	token, err := s.calendars.Sign(calendarClaims{User: actorID, Group: groupID}, exp)
	if err != nil {
		return "", time.Time{}, err
	}
	return fmt.Sprintf("%s/groups/%s/schedule.ics?token=%s", s.cfg.APIBaseURL, groupID, url.QueryEscape(token)), exp, nil
}

// Calendar renders the whole schedule as iCalendar for a subscription link.
func (s *Service) Calendar(ctx context.Context, groupID uuid.UUID, token string) ([]byte, error) {
	var c calendarClaims
	if err := s.calendars.Verify(token, s.Clock.Now(), &c); err != nil {
		return nil, err
	}
	if c.Group != groupID {
		return nil, fmt.Errorf("%w: link does not match the group", domain.ErrUnauthorized)
	}
	actor, err := s.Access.Actor(ctx, c.User, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.ScheduleRead); err != nil {
		return nil, err
	}
	list, err := s.Schedule.ListAll(ctx, groupID)
	if err != nil {
		return nil, err
	}
	byEvent, err := s.exceptionsOf(ctx, list)
	if err != nil {
		return nil, err
	}
	subjects, err := s.Subjects.List(ctx, groupID, true)
	if err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]string, len(subjects))
	for _, sub := range subjects {
		names[sub.ID] = sub.Name
	}
	return buildICS(actor.Group.Name, list, byEvent, names, s.Clock.Now()), nil
}
