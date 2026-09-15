package postgres

import (
	"context"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type subjectRepo struct{ s *Store }

func toSubject(s sqlcgen.Subject) *domain.Subject {
	aliases := s.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	return &domain.Subject{
		ID:             s.ID,
		GroupID:        s.GroupID,
		Name:           s.Name,
		ShortName:      s.ShortName,
		Teacher:        s.Teacher,
		TeacherContact: s.TeacherContact,
		Color:          s.Color,
		Semester:       s.Semester,
		Aliases:        aliases,
		SortOrder:      s.SortOrder,
		CreatedBy:      s.CreatedBy,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		ArchivedAt:     s.ArchivedAt,
	}
}

func (r *subjectRepo) Create(ctx context.Context, s domain.Subject) (*domain.Subject, error) {
	row, err := r.s.queries(ctx).CreateSubject(ctx, sqlcgen.CreateSubjectParams{
		ID:             s.ID,
		GroupID:        s.GroupID,
		Name:           s.Name,
		ShortName:      s.ShortName,
		Teacher:        s.Teacher,
		TeacherContact: s.TeacherContact,
		Color:          s.Color,
		Semester:       s.Semester,
		Aliases:        nonNil(s.Aliases),
		SortOrder:      s.SortOrder,
		CreatedBy:      s.CreatedBy,
	})
	if err != nil {
		return nil, mapErr(err, "subject")
	}
	return toSubject(row), nil
}

func (r *subjectRepo) Get(ctx context.Context, id, groupID uuid.UUID) (*domain.Subject, error) {
	row, err := r.s.queries(ctx).GetSubject(ctx, sqlcgen.GetSubjectParams{ID: id, GroupID: groupID})
	if err != nil {
		return nil, mapErr(err, "subject")
	}
	return toSubject(row), nil
}

func (r *subjectRepo) List(ctx context.Context, groupID uuid.UUID, includeArchived bool) ([]domain.Subject, error) {
	rows, err := r.s.queries(ctx).ListSubjects(ctx, sqlcgen.ListSubjectsParams{GroupID: groupID, IncludeArchived: includeArchived})
	if err != nil {
		return nil, mapErr(err, "subject")
	}
	out := make([]domain.Subject, len(rows))
	for i, row := range rows {
		out[i] = *toSubject(row)
	}
	return out, nil
}

func (r *subjectRepo) Update(ctx context.Context, s domain.Subject) (*domain.Subject, error) {
	row, err := r.s.queries(ctx).UpdateSubject(ctx, sqlcgen.UpdateSubjectParams{
		ID:             s.ID,
		GroupID:        s.GroupID,
		Name:           s.Name,
		ShortName:      s.ShortName,
		Teacher:        s.Teacher,
		TeacherContact: s.TeacherContact,
		Color:          s.Color,
		Semester:       s.Semester,
		Aliases:        nonNil(s.Aliases),
		SortOrder:      s.SortOrder,
	})
	if err != nil {
		return nil, mapErr(err, "subject")
	}
	return toSubject(row), nil
}

func (r *subjectRepo) Archive(ctx context.Context, id, groupID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).ArchiveSubject(ctx, sqlcgen.ArchiveSubjectParams{ID: id, GroupID: groupID}), "subject")
}

func (r *subjectRepo) Restore(ctx context.Context, id, groupID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).RestoreSubject(ctx, sqlcgen.RestoreSubjectParams{ID: id, GroupID: groupID}), "subject")
}

func (r *subjectRepo) AddAliases(ctx context.Context, id, groupID uuid.UUID, aliases []string) error {
	return mapErr(r.s.queries(ctx).AddSubjectAliases(ctx, sqlcgen.AddSubjectAliasesParams{ID: id, GroupID: groupID, NewAliases: nonNil(aliases)}), "subject")
}

// --- tags ---

type tagRepo struct{ s *Store }

func toTag(t sqlcgen.Tag) *domain.Tag {
	return &domain.Tag{
		ID:        t.ID,
		GroupID:   t.GroupID,
		Name:      t.Name,
		Slug:      t.Slug,
		Color:     t.Color,
		Kind:      domain.TagKind(t.Kind),
		SubjectID: t.SubjectID,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func (r *tagRepo) Create(ctx context.Context, t domain.Tag) (*domain.Tag, error) {
	row, err := r.s.queries(ctx).CreateTag(ctx, sqlcgen.CreateTagParams{
		ID:        t.ID,
		GroupID:   t.GroupID,
		Name:      t.Name,
		Slug:      t.Slug,
		Color:     t.Color,
		Kind:      string(t.Kind),
		SubjectID: t.SubjectID,
	})
	if err != nil {
		return nil, mapErr(err, "tag")
	}
	return toTag(row), nil
}

func (r *tagRepo) Get(ctx context.Context, id, groupID uuid.UUID) (*domain.Tag, error) {
	row, err := r.s.queries(ctx).GetTag(ctx, sqlcgen.GetTagParams{ID: id, GroupID: groupID})
	if err != nil {
		return nil, mapErr(err, "tag")
	}
	return toTag(row), nil
}

func (r *tagRepo) GetBySlug(ctx context.Context, groupID uuid.UUID, slug string) (*domain.Tag, error) {
	row, err := r.s.queries(ctx).GetTagBySlug(ctx, sqlcgen.GetTagBySlugParams{GroupID: groupID, Slug: slug})
	if err != nil {
		return nil, mapErr(err, "tag")
	}
	return toTag(row), nil
}

func (r *tagRepo) GetBySubject(ctx context.Context, subjectID uuid.UUID) (*domain.Tag, error) {
	row, err := r.s.queries(ctx).GetTagBySubject(ctx, &subjectID)
	if err != nil {
		return nil, mapErr(err, "tag")
	}
	return toTag(row), nil
}

func (r *tagRepo) List(ctx context.Context, groupID uuid.UUID) ([]domain.Tag, error) {
	rows, err := r.s.queries(ctx).ListTags(ctx, groupID)
	if err != nil {
		return nil, mapErr(err, "tag")
	}
	out := make([]domain.Tag, len(rows))
	for i, row := range rows {
		out[i] = *toTag(row)
	}
	return out, nil
}

func (r *tagRepo) Update(ctx context.Context, id, groupID uuid.UUID, name, slug string, color *string) (*domain.Tag, error) {
	row, err := r.s.queries(ctx).UpdateTag(ctx, sqlcgen.UpdateTagParams{ID: id, GroupID: groupID, Name: name, Slug: slug, Color: color})
	if err != nil {
		return nil, mapErr(err, "tag")
	}
	return toTag(row), nil
}

func (r *tagRepo) Delete(ctx context.Context, id, groupID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).DeleteTag(ctx, sqlcgen.DeleteTagParams{ID: id, GroupID: groupID}), "tag")
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
