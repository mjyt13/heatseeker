// Package subjects manages courses and their mirrored SUBJECT tags.
package subjects

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

// Service is the subjects use-case layer.
type Service struct {
	subjects domain.SubjectRepo
	tags     domain.TagRepo
	access   *access.Service
	events   *events.Publisher
	tx       domain.TxManager
}

// NewService wires the service.
func NewService(subjects domain.SubjectRepo, tags domain.TagRepo, acc *access.Service, pub *events.Publisher, tx domain.TxManager) *Service {
	return &Service{subjects: subjects, tags: tags, access: acc, events: pub, tx: tx}
}

// Input is the subject form. Nil pointers mean "not provided".
type Input struct {
	Name           string
	ShortName      *string
	Teacher        *string
	TeacherContact *string
	Color          *string
	Semester       *string
	Aliases        []string
	SortOrder      *int32
}

func (in Input) validate() (Input, error) {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 120 {
		return in, domain.Invalid("name", "must be 1–120 characters")
	}
	in.ShortName = trimPtr(in.ShortName, 30, "short_name")
	in.Teacher = trimPtr(in.Teacher, 120, "teacher")
	in.TeacherContact = trimPtr(in.TeacherContact, 200, "teacher_contact")
	in.Color = trimPtr(in.Color, 16, "color")
	in.Semester = trimPtr(in.Semester, 30, "semester")
	in.Aliases = normalizeAliases(in.Aliases)
	return in, nil
}

// Create adds a subject and its SUBJECT tag atomically.
func (s *Service) Create(ctx context.Context, actorID, groupID uuid.UUID, in Input) (*domain.Subject, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.SubjectManage); err != nil {
		return nil, err
	}
	in, err = in.validate()
	if err != nil {
		return nil, err
	}
	var created *domain.Subject
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		var sortOrder int32
		if in.SortOrder != nil {
			sortOrder = *in.SortOrder
		} else {
			existing, err := s.subjects.List(ctx, groupID, false)
			if err != nil {
				return err
			}
			sortOrder = int32(len(existing))
		}
		created, err = s.subjects.Create(ctx, domain.Subject{
			ID: ids.New(), GroupID: groupID, Name: in.Name, ShortName: in.ShortName, Teacher: in.Teacher,
			TeacherContact: in.TeacherContact, Color: in.Color, Semester: in.Semester, Aliases: in.Aliases,
			SortOrder: sortOrder, CreatedBy: &actorID,
		})
		if err != nil {
			return err
		}
		tagSlug, err := s.uniqueTagSlug(ctx, groupID, in.Name)
		if err != nil {
			return err
		}
		if _, err := s.tags.Create(ctx, domain.Tag{
			ID: ids.New(), GroupID: groupID, Name: in.Name, Slug: tagSlug, Color: in.Color, Kind: domain.TagKindSubject, SubjectID: &created.ID,
		}); err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventSubjectCreated, &actorID, &created.ID, map[string]any{"name": created.Name})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// List returns subjects visible to the actor.
func (s *Service) List(ctx context.Context, actorID, groupID uuid.UUID, includeArchived bool) ([]domain.Subject, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	return s.subjects.List(ctx, groupID, includeArchived)
}

// Get returns one subject.
func (s *Service) Get(ctx context.Context, actorID, groupID, subjectID uuid.UUID) (*domain.Subject, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	return s.subjects.Get(ctx, subjectID, groupID)
}

// Update edits a subject and keeps its tag in sync.
func (s *Service) Update(ctx context.Context, actorID, groupID, subjectID uuid.UUID, in Input) (*domain.Subject, error) {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.SubjectManage); err != nil {
		return nil, err
	}
	in, err = in.validate()
	if err != nil {
		return nil, err
	}
	var updated *domain.Subject
	err = s.tx.RunInTx(ctx, func(ctx context.Context) error {
		current, err := s.subjects.Get(ctx, subjectID, groupID)
		if err != nil {
			return err
		}
		sortOrder := current.SortOrder
		if in.SortOrder != nil {
			sortOrder = *in.SortOrder
		}
		updated, err = s.subjects.Update(ctx, domain.Subject{
			ID: subjectID, GroupID: groupID, Name: in.Name, ShortName: in.ShortName, Teacher: in.Teacher,
			TeacherContact: in.TeacherContact, Color: in.Color, Semester: in.Semester, Aliases: in.Aliases, SortOrder: sortOrder,
		})
		if err != nil {
			return err
		}
		if tag, err := s.tags.GetBySubject(ctx, subjectID); err == nil {
			tagSlug := tag.Slug
			if tag.Name != updated.Name {
				if tagSlug, err = s.uniqueTagSlug(ctx, groupID, updated.Name); err != nil {
					return err
				}
			}
			if _, err := s.tags.Update(ctx, tag.ID, groupID, updated.Name, tagSlug, updated.Color); err != nil {
				return err
			}
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		return s.emit(ctx, groupID, domain.EventSubjectUpdated, &actorID, &subjectID, map[string]any{"name": updated.Name})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Archive hides a subject without deleting its data.
func (s *Service) Archive(ctx context.Context, actorID, groupID, subjectID uuid.UUID) error {
	return s.setArchived(ctx, actorID, groupID, subjectID, true)
}

// Restore un-archives a subject.
func (s *Service) Restore(ctx context.Context, actorID, groupID, subjectID uuid.UUID) error {
	return s.setArchived(ctx, actorID, groupID, subjectID, false)
}

func (s *Service) setArchived(ctx context.Context, actorID, groupID, subjectID uuid.UUID, archived bool) error {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.SubjectManage); err != nil {
		return err
	}
	return s.tx.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := s.subjects.Get(ctx, subjectID, groupID); err != nil {
			return err
		}
		kind := domain.EventSubjectRestored
		if archived {
			kind = domain.EventSubjectArchived
			if err := s.subjects.Archive(ctx, subjectID, groupID); err != nil {
				return err
			}
		} else if err := s.subjects.Restore(ctx, subjectID, groupID); err != nil {
			return err
		}
		return s.emit(ctx, groupID, kind, &actorID, &subjectID, nil)
	})
}

// AddAliases appends classifier aliases (called when a user corrects a
// mis-classified material).
func (s *Service) AddAliases(ctx context.Context, actorID, groupID, subjectID uuid.UUID, aliases []string) error {
	actor, err := s.access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.SubjectManage); err != nil {
		return err
	}
	aliases = normalizeAliases(aliases)
	if len(aliases) == 0 {
		return nil
	}
	return s.subjects.AddAliases(ctx, subjectID, groupID, aliases)
}

func (s *Service) uniqueTagSlug(ctx context.Context, groupID uuid.UUID, name string) (string, error) {
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
	e, err := domain.NewEvent(groupID, kind, actor, "subject", entityID, payload)
	return s.events.Emit(ctx, e, err)
}

func trimPtr(p *string, max int, _ string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	if utf8.RuneCountInString(v) > max {
		v = string([]rune(v)[:max])
	}
	return &v
}

func normalizeAliases(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, a := range in {
		a = strings.ToLower(strings.Join(strings.Fields(a), " "))
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}
