// Package announcements serves the headman's one-way messages to the whole
// group: everybody gets them, an urgent one even at night (docs/PLAN.md §8.2).
package announcements

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/ids"
)

// MaxList bounds one page of announcements.
const MaxList = 100

// Deps are the collaborators the service needs.
type Deps struct {
	Repo   domain.AnnouncementRepo
	Access *access.Service
	Events *events.Publisher
	Tx     domain.TxManager
	Log    *slog.Logger
}

// Service is the announcements use case.
type Service struct{ Deps }

// NewService wires the service.
func NewService(d Deps) *Service { return &Service{d} }

// Input is the announcement form.
type Input struct {
	Title  string
	Body   string
	Urgent bool
	Pinned bool
}

// List returns the group's announcements, pinned first.
func (s *Service) List(ctx context.Context, actorID, groupID uuid.UUID, limit int32) ([]domain.AnnouncementView, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > MaxList {
		limit = MaxList
	}
	return s.Repo.List(ctx, groupID, limit)
}

// Create publishes an announcement (authz.AnnouncementSend, L2).
func (s *Service) Create(ctx context.Context, actorID, groupID uuid.UUID, in Input) (*domain.Announcement, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.AnnouncementSend); err != nil {
		return nil, err
	}
	title, body, err := normalize(in)
	if err != nil {
		return nil, err
	}
	var saved *domain.Announcement
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		saved, err = s.Repo.Create(ctx, domain.Announcement{
			ID: ids.New(), GroupID: groupID, AuthorID: &actorID, Title: title, Body: body,
			Urgent: in.Urgent, Pinned: in.Pinned,
		})
		if err != nil {
			return err
		}
		return s.emit(ctx, saved, domain.EventAnnouncementCreated, actorID)
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// Update edits an announcement. It does not announce it again: the group has
// already heard it.
func (s *Service) Update(ctx context.Context, actorID, id uuid.UUID, in Input) (*domain.Announcement, error) {
	current, actor, err := s.load(ctx, actorID, id)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.AnnouncementSend); err != nil {
		return nil, err
	}
	title, body, err := normalize(in)
	if err != nil {
		return nil, err
	}
	var saved *domain.Announcement
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		saved, err = s.Repo.Update(ctx, domain.Announcement{
			ID: current.ID, Title: title, Body: body, Urgent: in.Urgent, Pinned: in.Pinned,
		})
		if err != nil {
			return err
		}
		saved.GroupID = current.GroupID
		return s.emit(ctx, saved, domain.EventAnnouncementUpdated, actorID)
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// Delete takes an announcement down.
func (s *Service) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	current, actor, err := s.load(ctx, actorID, id)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.AnnouncementSend); err != nil {
		return err
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.SoftDelete(ctx, id); err != nil {
			return err
		}
		return s.emit(ctx, current, domain.EventAnnouncementDeleted, actorID)
	})
}

func (s *Service) load(ctx context.Context, actorID, id uuid.UUID) (*domain.Announcement, *access.Actor, error) {
	a, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	actor, err := s.Access.Actor(ctx, actorID, a.GroupID)
	if err != nil {
		return nil, nil, err
	}
	return a, actor, nil
}

func (s *Service) emit(ctx context.Context, a *domain.Announcement, kind domain.EventKind, actorID uuid.UUID) error {
	e, err := domain.NewEvent(a.GroupID, kind, &actorID, "announcement", &a.ID, map[string]any{
		"title": a.Title, "body": a.Body, "urgent": a.Urgent,
	})
	return s.Events.Emit(ctx, e, err)
}

func normalize(in Input) (title, body string, err error) {
	title = strings.Join(strings.Fields(in.Title), " ")
	if n := utf8.RuneCountInString(title); n < 1 || n > 200 {
		return "", "", domain.Invalid("title", "must be 1–200 characters")
	}
	body = strings.TrimSpace(in.Body)
	if utf8.RuneCountInString(body) > 5000 {
		return "", "", domain.Invalid("body", "must be at most 5000 characters")
	}
	return title, body, nil
}
