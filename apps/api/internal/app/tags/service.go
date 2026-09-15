// Package tags manages custom tags and builds the home-screen quick tags.
package tags

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/ids"
	"heatseeker/api/internal/platform/slug"
)

// Service is the tags use-case layer.
type Service struct {
	tags     domain.TagRepo
	subjects domain.SubjectRepo
	access   *access.Service
	events   *events.Publisher
	tx       domain.TxManager
}

// NewService wires the service.
func NewService(tags domain.TagRepo, subjects domain.SubjectRepo, acc *access.Service, pub *events.Publisher, tx domain.TxManager) *Service {
	return &Service{tags: tags, subjects: subjects, access: acc, events: pub, tx: tx}
}

// Input is the tag form.
type Input struct {
	Name  string
	Color *string
	Kind  domain.TagKind
}

// List returns all tags of a group.
func (s *Service) List(ctx context.Context, actorID, groupID uuid.UUID) ([]domain.Tag, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	return s.tags.List(ctx, groupID)
}

// Create adds a custom tag (TOPIC, TYPE or CUSTOM).
func (s *Service) Create(ctx context.Context, actorID, groupID uuid.UUID, in Input) (*domain.Tag, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.TagManage); err != nil {
		return nil, err
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return nil, err
	}
	kind := in.Kind
	if kind == "" {
		kind = domain.TagKindCustom
	}
	if kind == domain.TagKindSubject || kind == domain.TagKindSystem {
		return nil, domain.Invalid("kind", "subject and system tags are managed automatically")
	}
	var created *domain.Tag
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		tagSlug, err := s.uniqueSlug(ctx, groupID, name)
		if err != nil {
			return err
		}
		created, err = s.tags.Create(ctx, domain.Tag{ID: ids.New(), GroupID: groupID, Name: name, Slug: tagSlug, Color: in.Color, Kind: kind})
		if err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventTagCreated, &actorID, &created.ID, map[string]any{"name": name})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Update renames or recolours a custom tag.
func (s *Service) Update(ctx context.Context, actorID, groupID, tagID uuid.UUID, in Input) (*domain.Tag, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.TagManage); err != nil {
		return nil, err
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return nil, err
	}
	var updated *domain.Tag
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		current, err := s.tags.Get(ctx, tagID, groupID)
		if err != nil {
			return err
		}
		if current.Kind == domain.TagKindSubject {
			return domain.Invalid("id", "rename the subject instead of its tag")
		}
		tagSlug := current.Slug
		if current.Name != name {
			if tagSlug, err = s.uniqueSlug(ctx, groupID, name); err != nil {
				return err
			}
		}
		updated, err = s.tags.Update(ctx, tagID, groupID, name, tagSlug, in.Color)
		if err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventTagUpdated, &actorID, &tagID, map[string]any{"name": name})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes a custom tag.
func (s *Service) Delete(ctx context.Context, actorID, groupID, tagID uuid.UUID) error {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.TagManage); err != nil {
		return err
	}
	return s.tx.RunInTx(ctx, func(ctx context.Context) error {
		current, err := s.tags.Get(ctx, tagID, groupID)
		if err != nil {
			return err
		}
		if current.Kind == domain.TagKindSubject {
			return domain.Invalid("id", "subject tags are removed with their subject")
		}
		if err := s.tags.Delete(ctx, tagID, groupID); err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventTagDeleted, &actorID, &tagID, map[string]any{"name": current.Name})
	})
}

// System quick tags shown before subjects. Labels are i18n keys.
var systemQuickTags = []domain.QuickTag{
	{Key: "system:mine", Label: "quick_tags.mine", Kind: domain.TagKindSystem, Order: 0},
	{Key: "system:saved", Label: "quick_tags.saved", Kind: domain.TagKindSystem, Order: 1},
	{Key: "system:unread", Label: "quick_tags.unread", Kind: domain.TagKindSystem, Order: 2},
}

// QuickTags builds the chips for the home screen: system filters first, then
// active subjects in their sort order.
func (s *Service) QuickTags(ctx context.Context, actorID, groupID uuid.UUID) ([]domain.QuickTag, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	subjects, err := s.subjects.List(ctx, groupID, false)
	if err != nil {
		return nil, err
	}
	tags, err := s.tags.List(ctx, groupID)
	if err != nil {
		return nil, err
	}
	tagBySubject := map[uuid.UUID]domain.Tag{}
	for _, t := range tags {
		if t.SubjectID != nil {
			tagBySubject[*t.SubjectID] = t
		}
	}
	out := make([]domain.QuickTag, 0, len(systemQuickTags)+len(subjects))
	out = append(out, systemQuickTags...)
	for i, sub := range subjects {
		q := domain.QuickTag{
			Key:     "subject:" + sub.ID.String(),
			Label:   sub.Name,
			Color:   sub.Color,
			Kind:    domain.TagKindSubject,
			Subject: &sub.ID,
			Order:   len(systemQuickTags) + i,
		}
		if sub.ShortName != nil && *sub.ShortName != "" {
			q.Label = *sub.ShortName
		}
		if t, ok := tagBySubject[sub.ID]; ok {
			id := t.ID
			q.TagID = &id
		}
		out = append(out, q)
	}
	return out, nil
}

func (s *Service) uniqueSlug(ctx context.Context, groupID uuid.UUID, name string) (string, error) {
	base := slug.Make(name)
	candidate := base
	for i := 0; i < 5; i++ {
		_, err := s.tags.GetBySlug(ctx, groupID, candidate)
		if errors.Is(err, domain.ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = base + "-" + ids.Code(4)
	}
	return base + "-" + ids.Code(8), nil
}

func (s *Service) emit(ctx context.Context, groupID uuid.UUID, kind domain.EventKind, actor *uuid.UUID, entityID *uuid.UUID, payload any) error {
	e, err := domain.NewEvent(groupID, kind, actor, "tag", entityID, payload)
	return s.events.Emit(ctx, e, err)
}

func normalizeName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		return "", domain.Invalid("name", "must be 1–60 characters")
	}
	return name, nil
}
