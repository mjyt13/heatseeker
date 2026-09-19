// Package materials implements the material library of a group: the feed with
// filters and search, editing, moderation (archive/delete/Inbox), uploads to
// our storage and opening files from wherever they live (docs/PLAN.md §5).
package materials

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/app/classify"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/signed"
)

// Settings come from configuration.
type Settings struct {
	APIBaseURL         string // public origin + "/api/v1", used for signed links
	PresignTTL         time.Duration
	UploadTTL          time.Duration
	MaxUploadBytes     int64
	AllowedExt         []string
	HardDeleteAfter    time.Duration
	ProxyEnabled       bool
	StreamTTL          time.Duration
	DriveUploadEnabled bool
	// DrivePublisherOAuth: the server can connect a publishing Google account (D34).
	DrivePublisherOAuth bool
	HashMaxBytes        int64
	// PreviewMaxBytes bounds office files converted to PDF previews.
	PreviewMaxBytes int64
	MinConfidence   float64
	SigningSecret   string
}

// Deps are the collaborators of the service.
type Deps struct {
	Materials domain.MaterialRepo
	Uploads   domain.UploadRepo
	Subjects  domain.SubjectRepo
	Tags      domain.TagRepo
	Drive     domain.DriveRepo
	Store     domain.MediaStore
	Client    domain.DriveClient // nil when Drive is not configured
	// Converter makes PDF previews of office files; nil disables them.
	Converter domain.DocumentConverter
	Access    *access.Service
	Events    *events.Publisher
	Tx        domain.TxManager
	Queue     domain.JobQueue
	Clock     clock.Clock
	Log       *slog.Logger
}

// Service is the materials use-case layer.
type Service struct {
	Deps
	cfg     Settings
	links   *signed.Signer // local storage links
	streams *signed.Signer // Drive proxy links
}

// NewService wires the service.
func NewService(d Deps, cfg Settings) *Service {
	cfg.APIBaseURL = strings.TrimRight(cfg.APIBaseURL, "/")
	return &Service{
		Deps:    d,
		cfg:     cfg,
		links:   signed.New(cfg.SigningSecret, "media-local"),
		streams: signed.New(cfg.SigningSecret, "media-stream"),
	}
}

// Limits describes upload constraints for clients.
type Limits struct {
	MaxUploadBytes     int64
	AllowedExt         []string
	DriveUploadEnabled bool
	ProxyEnabled       bool
}

// Limits returns the upload constraints.
func (s *Service) Limits() Limits {
	return Limits{
		MaxUploadBytes:     s.cfg.MaxUploadBytes,
		AllowedExt:         s.cfg.AllowedExt,
		DriveUploadEnabled: s.cfg.DriveUploadEnabled && s.Client != nil,
		ProxyEnabled:       s.cfg.ProxyEnabled && s.Client != nil,
	}
}

// ListInput filters the feed.
type ListInput struct {
	SubjectID *uuid.UUID
	NoSubject bool
	TagIDs    []uuid.UUID
	Kind      *domain.MaterialKind
	FileType  *domain.FileType
	Mine      bool
	Inbox     bool
	Archived  bool
	Query     string
	Cursor    string
	Limit     int32
}

// Page is one page of the feed.
type Page struct {
	Items      []domain.MaterialView
	NextCursor string
	InboxCount int64
}

const (
	defaultPageSize = 30
	maxPageSize     = 100
)

// List returns a page of materials.
func (s *Service) List(ctx context.Context, actorID, groupID uuid.UUID, in ListInput) (*Page, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	f := domain.MaterialFilter{
		GroupID:   groupID,
		Status:    domain.MaterialActive,
		SubjectID: in.SubjectID,
		NoSubject: in.NoSubject,
		Kind:      in.Kind,
		FileType:  in.FileType,
		Inbox:     in.Inbox,
		Query:     strings.TrimSpace(in.Query),
		Limit:     in.Limit,
	}
	if in.Archived {
		f.Status = domain.MaterialArchived
	}
	if in.Mine {
		f.UploaderID = &actorID
	}
	if f.FileType != nil && !slices.Contains(domain.AllFileTypes, *f.FileType) {
		return nil, domain.Invalid("file_type", "unknown file type")
	}
	if f.Kind != nil && !slices.Contains(domain.AllMaterialKinds, *f.Kind) {
		return nil, domain.Invalid("kind", "unknown material kind")
	}
	if utf8.RuneCountInString(f.Query) > 200 {
		return nil, domain.Invalid("q", "query is too long")
	}
	if f.Limit <= 0 {
		f.Limit = defaultPageSize
	}
	f.Limit = min(f.Limit, maxPageSize)
	if in.Cursor != "" {
		c, err := decodeCursor(in.Cursor)
		if err != nil {
			return nil, err
		}
		f.After = c
	}
	if len(in.TagIDs) > 0 {
		// A subject tag is the same filter as the subject itself.
		tags, err := s.Tags.List(ctx, groupID)
		if err != nil {
			return nil, err
		}
		byID := make(map[uuid.UUID]domain.Tag, len(tags))
		for _, t := range tags {
			byID[t.ID] = t
		}
		for _, id := range in.TagIDs {
			t, ok := byID[id]
			if !ok {
				return nil, domain.Invalid("tag", "unknown tag")
			}
			if t.SubjectID != nil {
				if f.SubjectID != nil && *f.SubjectID != *t.SubjectID {
					return &Page{Items: []domain.MaterialView{}}, nil
				}
				f.SubjectID = t.SubjectID
				continue
			}
			f.TagIDs = append(f.TagIDs, id)
		}
	}
	f.Limit++
	items, err := s.Materials.List(ctx, f)
	if err != nil {
		return nil, err
	}
	page := &Page{Items: items}
	if int32(len(items)) == f.Limit {
		page.Items = items[:f.Limit-1]
		last := page.Items[len(page.Items)-1].Material
		page.NextCursor = encodeCursor(domain.MaterialCursor{SortAt: last.SortAt, ID: last.ID})
	}
	if actor.Can(authz.MaterialModerate) || in.Inbox {
		if page.InboxCount, err = s.Materials.CountInbox(ctx, groupID); err != nil {
			return nil, err
		}
	}
	return page, nil
}

func encodeCursor(c domain.MaterialCursor) string {
	buf := make([]byte, 8, 24)
	binary.BigEndian.PutUint64(buf, uint64(c.SortAt.UnixMicro()))
	buf = append(buf, c.ID[:]...)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func decodeCursor(s string) (*domain.MaterialCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) != 24 {
		return nil, domain.Invalid("cursor", "malformed cursor")
	}
	id, err := uuid.FromBytes(raw[8:])
	if err != nil {
		return nil, domain.Invalid("cursor", "malformed cursor")
	}
	micros := int64(binary.BigEndian.Uint64(raw[:8])) //nolint:gosec // round-trips encodeCursor
	return &domain.MaterialCursor{SortAt: time.UnixMicro(micros).UTC(), ID: id}, nil
}

// Details is a material with its history and origin.
type Details struct {
	View      domain.MaterialView
	Versions  []domain.MaterialVersion
	DrivePath []string // folders on Drive, when indexed from there
	CanEdit   bool
	CanDelete bool
	Moderator bool
}

// Get returns one material.
func (s *Service) Get(ctx context.Context, actorID, materialID uuid.UUID) (*Details, error) {
	view, actor, err := s.loadView(ctx, actorID, materialID)
	if err != nil {
		return nil, err
	}
	versions, err := s.Materials.ListVersions(ctx, materialID)
	if err != nil {
		return nil, err
	}
	d := &Details{
		View:      *view,
		Versions:  versions,
		CanEdit:   s.canEdit(actor, &view.Material),
		CanDelete: s.canDelete(actor, &view.Material) == nil,
		Moderator: actor.Can(authz.MaterialModerate),
	}
	if view.Material.Source == domain.SourceDrive {
		item, err := s.Drive.GetItemByMaterial(ctx, materialID)
		switch {
		case err == nil:
			d.DrivePath = item.PathSegments()
		case !errors.Is(err, domain.ErrNotFound):
			return nil, err
		}
	}
	return d, nil
}

// loadView loads a material the actor may see.
func (s *Service) loadView(ctx context.Context, actorID, materialID uuid.UUID) (*domain.MaterialView, *access.Actor, error) {
	view, err := s.Materials.GetView(ctx, materialID)
	if err != nil {
		return nil, nil, err
	}
	actor, err := s.Access.Actor(ctx, actorID, view.Material.GroupID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			// Do not reveal materials of other groups.
			return nil, nil, domain.NotFound("material")
		}
		return nil, nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, nil, err
	}
	if view.Material.Status == domain.MaterialDeleted && !actor.Can(authz.MaterialModerate) {
		return nil, nil, domain.NotFound("material")
	}
	return view, actor, nil
}

func (s *Service) isOwner(actor *access.Actor, m *domain.Material) bool {
	return m.UploaderID != nil && *m.UploaderID == actor.User.ID && actor.Can(authz.MaterialEditOwn)
}

func (s *Service) canEdit(actor *access.Actor, m *domain.Material) bool {
	return actor.Can(authz.MaterialModerate) || s.isOwner(actor, m)
}

// canDelete: moderators always; owners only until someone else opened the file.
func (s *Service) canDelete(actor *access.Actor, m *domain.Material) error {
	if actor.Can(authz.MaterialModerate) {
		return nil
	}
	if !s.isOwner(actor, m) {
		return domain.Forbidden("only the uploader or a moderator may delete a material")
	}
	if m.DownloadCount > 0 {
		return domain.Forbidden("the material was already opened by others; archive it instead")
	}
	return nil
}

// UpdateInput edits a material. Nil fields are left unchanged; SetSubject
// distinguishes "clear the subject" from "leave it".
type UpdateInput struct {
	Title       *string
	Description *string
	SetSubject  bool
	SubjectID   *uuid.UUID
	Kind        *domain.MaterialKind
	TagIDs      *[]uuid.UUID
}

// Update edits title, description, subject, kind and tags.
func (s *Service) Update(ctx context.Context, actorID, materialID uuid.UUID, in UpdateInput) (*domain.MaterialView, error) {
	view, actor, err := s.loadView(ctx, actorID, materialID)
	if err != nil {
		return nil, err
	}
	m := view.Material
	if !s.canEdit(actor, &m) {
		return nil, domain.Forbidden("only the uploader or a moderator may edit a material")
	}
	if m.Status == domain.MaterialDeleted {
		return nil, domain.Conflict("material is deleted")
	}
	p := domain.UpdateMaterialParams{
		ID: m.ID, Title: m.Title, Description: m.Description, SubjectID: m.SubjectID, Kind: m.Kind,
		Classification: m.Classification, NeedsReview: m.NeedsReview, ReviewReason: m.ReviewReason,
	}
	if in.Title != nil {
		if p.Title, err = normalizeTitle(*in.Title); err != nil {
			return nil, err
		}
	}
	if in.Description != nil {
		if p.Description, err = normalizeDescription(*in.Description); err != nil {
			return nil, err
		}
	}
	reclassified := false
	if in.SetSubject && !sameID(in.SubjectID, m.SubjectID) {
		if err := s.checkSubject(ctx, m.GroupID, in.SubjectID); err != nil {
			return nil, err
		}
		p.SubjectID = in.SubjectID
		reclassified = true
	}
	if in.Kind != nil && *in.Kind != m.Kind {
		if !slices.Contains(domain.AllMaterialKinds, *in.Kind) {
			return nil, domain.Invalid("kind", "unknown material kind")
		}
		p.Kind = *in.Kind
		reclassified = true
	}
	if reclassified {
		markManual(&p)
	}
	var tagIDs []uuid.UUID
	if in.TagIDs != nil {
		if tagIDs, err = s.checkTags(ctx, m.GroupID, *in.TagIDs); err != nil {
			return nil, err
		}
	}
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := s.Materials.Update(ctx, p); err != nil {
			return err
		}
		if in.TagIDs != nil {
			if err := s.Materials.SetTags(ctx, m.ID, tagIDs); err != nil {
				return err
			}
		}
		return s.emit(ctx, m.GroupID, domain.EventMaterialUpdated, &actorID, m.ID, map[string]any{
			"title": p.Title, "subject_id": p.SubjectID, "kind": p.Kind,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Materials.GetView(ctx, m.ID)
}

// markManual records a human decision and takes the material out of the
// Inbox when it was there only because the classifier was unsure.
func markManual(p *domain.UpdateMaterialParams) {
	p.Classification.Method = domain.ClassifyManual
	p.Classification.SubjectID = p.SubjectID
	p.Classification.Kind = p.Kind
	p.Classification.Confidence = 1
	if p.ReviewReason != nil && *p.ReviewReason == domain.ReviewLowConfidence {
		p.NeedsReview = false
		p.ReviewReason = nil
	}
}

// ClassifyInput is a moderator's decision from the Inbox.
type ClassifyInput struct {
	SubjectID *uuid.UUID
	Kind      domain.MaterialKind
	// LearnAlias adds the Drive folder name to the subject's aliases so similar
	// files are recognised next time.
	LearnAlias bool
}

// ClassifyResult is the classified material and the alias learned from its
// Drive folder, if any.
type ClassifyResult struct {
	View         *domain.MaterialView
	LearnedAlias string
}

// Classify confirms subject and kind and clears the review flag.
func (s *Service) Classify(ctx context.Context, actorID, materialID uuid.UUID, in ClassifyInput) (*ClassifyResult, error) {
	view, actor, err := s.loadView(ctx, actorID, materialID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.MaterialModerate); err != nil {
		return nil, err
	}
	m := view.Material
	if in.Kind == "" {
		in.Kind = m.Kind
	}
	if !slices.Contains(domain.AllMaterialKinds, in.Kind) {
		return nil, domain.Invalid("kind", "unknown material kind")
	}
	if err := s.checkSubject(ctx, m.GroupID, in.SubjectID); err != nil {
		return nil, err
	}
	p := domain.UpdateMaterialParams{
		ID: m.ID, Title: m.Title, Description: m.Description, SubjectID: in.SubjectID, Kind: in.Kind,
		Classification: m.Classification,
	}
	p.ReviewReason = m.ReviewReason
	p.NeedsReview = m.NeedsReview
	markManual(&p)
	p.NeedsReview = false
	p.ReviewReason = nil

	var alias string
	if in.LearnAlias && in.SubjectID != nil && m.Source == domain.SourceDrive {
		if alias, err = s.aliasCandidate(ctx, m.GroupID, m.ID, *in.SubjectID); err != nil {
			return nil, err
		}
	}
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := s.Materials.Update(ctx, p); err != nil {
			return err
		}
		if alias != "" {
			if err := s.Subjects.AddAliases(ctx, *in.SubjectID, m.GroupID, []string{alias}); err != nil {
				return err
			}
		}
		return s.emit(ctx, m.GroupID, domain.EventMaterialClassified, &actorID, m.ID, map[string]any{
			"title": m.Title, "subject_id": in.SubjectID, "kind": in.Kind, "learned_alias": alias,
		})
	})
	if err != nil {
		return nil, err
	}
	fresh, err := s.Materials.GetView(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	return &ClassifyResult{View: fresh, LearnedAlias: alias}, nil
}

// MaxBulkClassify caps one ClassifyMany call.
const MaxBulkClassify = 200

// BulkClassifyInput assigns one subject (and optionally one kind) to many
// Inbox materials at once.
type BulkClassifyInput struct {
	MaterialIDs []uuid.UUID
	SubjectID   *uuid.UUID
	// Kind nil keeps each material's own kind.
	Kind *domain.MaterialKind
}

// ClassifyMany is the Inbox bulk action. Materials of other groups or already
// deleted ones are skipped; it returns how many were classified.
func (s *Service) ClassifyMany(ctx context.Context, actorID, groupID uuid.UUID, in BulkClassifyInput) (int, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return 0, err
	}
	if err := actor.Require(authz.MaterialModerate); err != nil {
		return 0, err
	}
	if len(in.MaterialIDs) == 0 || len(in.MaterialIDs) > MaxBulkClassify {
		return 0, domain.Invalid("material_ids", fmt.Sprintf("must contain 1–%d ids", MaxBulkClassify))
	}
	if in.Kind != nil && !slices.Contains(domain.AllMaterialKinds, *in.Kind) {
		return 0, domain.Invalid("kind", "unknown material kind")
	}
	if err := s.checkSubject(ctx, groupID, in.SubjectID); err != nil {
		return 0, err
	}
	done := 0
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		seen := map[uuid.UUID]bool{}
		for _, id := range in.MaterialIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			m, err := s.Materials.Get(ctx, id)
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if m.GroupID != groupID || m.Status == domain.MaterialDeleted {
				continue
			}
			p := domain.UpdateMaterialParams{
				ID: m.ID, Title: m.Title, Description: m.Description, SubjectID: in.SubjectID, Kind: m.Kind,
				Classification: m.Classification,
			}
			if in.Kind != nil {
				p.Kind = *in.Kind
			}
			markManual(&p)
			p.NeedsReview, p.ReviewReason = false, nil
			if _, err := s.Materials.Update(ctx, p); err != nil {
				return err
			}
			e, err := domain.NewEvent(groupID, domain.EventMaterialClassified, &actorID, "material", &m.ID, map[string]any{
				"title": m.Title, "subject_id": in.SubjectID, "kind": p.Kind, "bulk": true,
			})
			e.Audit = false // the summary below is what the activity feed shows
			if err := s.Events.Emit(ctx, e, err); err != nil {
				return err
			}
			done++
		}
		if done == 0 {
			return nil
		}
		e, err := domain.NewEvent(groupID, domain.EventMaterialsBulkClassified, &actorID, "group", &groupID, map[string]any{
			"count": done, "subject_id": in.SubjectID, "kind": in.Kind,
		})
		return s.Events.Emit(ctx, e, err)
	})
	if err != nil {
		return 0, err
	}
	return done, nil
}

// aliasCandidate picks the deepest Drive folder that names neither a kind
// ("Лекции") nor already the subject.
func (s *Service) aliasCandidate(ctx context.Context, groupID, materialID, subjectID uuid.UUID) (string, error) {
	item, err := s.Drive.GetItemByMaterial(ctx, materialID)
	if errors.Is(err, domain.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	subject, err := s.Subjects.Get(ctx, subjectID, groupID)
	if err != nil {
		return "", err
	}
	target := classify.FromDomain([]domain.Subject{*subject})[0]
	segments := item.PathSegments()
	for i := len(segments) - 1; i >= 0; i-- {
		folder := strings.TrimSpace(segments[i])
		if folder == "" || classify.DetectKind("", []string{folder}) != domain.KindOther {
			continue
		}
		if classify.MatchScore(folder, target) >= 0.75 {
			return "", nil // the path already names the subject
		}
		if len(classify.Tokens(folder)) == 0 || utf8.RuneCountInString(folder) > 80 {
			continue
		}
		return strings.ToLower(folder), nil
	}
	return "", nil
}

// Archive hides a material from the feed.
func (s *Service) Archive(ctx context.Context, actorID, materialID uuid.UUID) error {
	return s.transition(ctx, actorID, materialID, domain.MaterialArchived)
}

// Restore returns an archived (or, for moderators, deleted) material.
func (s *Service) Restore(ctx context.Context, actorID, materialID uuid.UUID) error {
	return s.transition(ctx, actorID, materialID, domain.MaterialActive)
}

// Delete soft-deletes a material; files are purged after the grace period.
func (s *Service) Delete(ctx context.Context, actorID, materialID uuid.UUID) error {
	return s.transition(ctx, actorID, materialID, domain.MaterialDeleted)
}

func (s *Service) transition(ctx context.Context, actorID, materialID uuid.UUID, to domain.MaterialStatus) error {
	view, actor, err := s.loadView(ctx, actorID, materialID)
	if err != nil {
		return err
	}
	m := view.Material
	if m.Status == to {
		return nil
	}
	var kind domain.EventKind
	switch to {
	case domain.MaterialArchived:
		if m.Status != domain.MaterialActive {
			return domain.Conflict("only active materials can be archived")
		}
		if !s.canEdit(actor, &m) {
			return domain.Forbidden("only the uploader or a moderator may archive a material")
		}
		kind = domain.EventMaterialArchived
	case domain.MaterialActive:
		if m.Status == domain.MaterialDeleted {
			if err := actor.Require(authz.MaterialModerate); err != nil {
				return err
			}
		} else if !s.canEdit(actor, &m) {
			return domain.Forbidden("only the uploader or a moderator may restore a material")
		}
		kind = domain.EventMaterialRestored
	case domain.MaterialDeleted:
		if err := s.canDelete(actor, &m); err != nil {
			return err
		}
		kind = domain.EventMaterialDeleted
	default:
		return domain.Invalid("status", "unsupported transition")
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := s.Materials.SetStatus(ctx, m.ID, to, &actorID); err != nil {
			return err
		}
		return s.emit(ctx, m.GroupID, kind, &actorID, m.ID, map[string]any{"title": m.Title})
	})
}

func (s *Service) checkSubject(ctx context.Context, groupID uuid.UUID, subjectID *uuid.UUID) error {
	if subjectID == nil {
		return nil
	}
	subject, err := s.Subjects.Get(ctx, *subjectID, groupID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Invalid("subject_id", "unknown subject")
	}
	if err != nil {
		return err
	}
	if subject.ArchivedAt != nil {
		return domain.Invalid("subject_id", "subject is archived")
	}
	return nil
}

// checkTags validates tag ids; subject tags are dropped because the subject
// is stored on the material itself.
func (s *Service) checkTags(ctx context.Context, groupID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return []uuid.UUID{}, nil
	}
	if len(ids) > 20 {
		return nil, domain.Invalid("tag_ids", "too many tags")
	}
	tags, err := s.Tags.List(ctx, groupID)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]domain.Tag, len(tags))
	for _, t := range tags {
		byID[t.ID] = t
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		t, ok := byID[id]
		if !ok {
			return nil, domain.Invalid("tag_ids", "unknown tag")
		}
		if t.Kind == domain.TagKindSubject || t.Kind == domain.TagKindSystem || slices.Contains(out, id) {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

func (s *Service) emit(ctx context.Context, groupID uuid.UUID, kind domain.EventKind, actor *uuid.UUID, materialID uuid.UUID, payload any) error {
	e, err := domain.NewEvent(groupID, kind, actor, "material", &materialID, payload)
	return s.Events.Emit(ctx, e, err)
}

func normalizeTitle(v string) (string, error) {
	v = strings.Join(strings.Fields(v), " ")
	if n := utf8.RuneCountInString(v); n < 1 || n > 200 {
		return "", domain.Invalid("title", "must be 1–200 characters")
	}
	return v, nil
}

func normalizeDescription(v string) (string, error) {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > 10000 {
		return "", domain.Invalid("description", "must be at most 10000 characters")
	}
	return v, nil
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
