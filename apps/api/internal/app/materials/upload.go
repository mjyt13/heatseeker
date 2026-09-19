package materials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/classify"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/filenames"
	"heatseeker/api/internal/platform/ids"
)

// UploadInput starts an upload: file facts plus the material form.
type UploadInput struct {
	FileName    string
	SizeBytes   int64
	Mime        string
	Title       string
	Description string
	SubjectID   *uuid.UUID
	Kind        *domain.MaterialKind
	TagIDs      []uuid.UUID
	ToDrive     bool
}

// UploadTicket tells the client where to send the bytes.
type UploadTicket struct {
	Upload  domain.Upload
	Request domain.PresignedRequest
	MaxSize int64
}

// CreateUpload validates the form and returns a direct upload URL.
func (s *Service) CreateUpload(ctx context.Context, actorID, groupID uuid.UUID, in UploadInput) (*UploadTicket, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.MaterialUpload); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.FileName)
	if name == "" || len([]rune(name)) > 255 {
		return nil, domain.Invalid("file_name", "must be 1–255 characters")
	}
	ext := Extension(name)
	if !slices.Contains(s.cfg.AllowedExt, ext) {
		return nil, domain.Invalid("file_name", fmt.Sprintf("files of type .%s are not allowed", ext))
	}
	if in.SizeBytes <= 0 {
		return nil, domain.Invalid("size_bytes", "must be positive")
	}
	if in.SizeBytes > s.cfg.MaxUploadBytes {
		return nil, fmt.Errorf("%w: file exceeds %d MB", domain.ErrTooLarge, s.cfg.MaxUploadBytes>>20)
	}
	meta := domain.UploadMeta{Description: in.Description, SubjectID: in.SubjectID, ToDrive: in.ToDrive}
	if meta.Title, err = normalizeTitle(firstNonEmpty(in.Title, classify.TitleFromFileName(name))); err != nil {
		return nil, err
	}
	if meta.Description, err = normalizeDescription(in.Description); err != nil {
		return nil, err
	}
	if in.Kind != nil {
		if !slices.Contains(domain.AllMaterialKinds, *in.Kind) {
			return nil, domain.Invalid("kind", "unknown material kind")
		}
		meta.Kind = *in.Kind
	}
	if err := s.checkSubject(ctx, groupID, in.SubjectID); err != nil {
		return nil, err
	}
	if meta.TagIDs, err = s.checkTags(ctx, groupID, in.TagIDs); err != nil {
		return nil, err
	}
	if in.ToDrive {
		if err := s.checkDriveUpload(ctx, actor.Can(authz.DriveUpload), actor.User.Secured(), groupID); err != nil {
			return nil, err
		}
	}

	uploadID := ids.New()
	mime := MimeFor(ext, in.Mime)
	u := domain.Upload{
		ID: uploadID, GroupID: groupID, UserID: actorID, Storage: s.Store.Kind(),
		StorageKey: fmt.Sprintf("tmp/uploads/%s/%s", uploadID, filenames.SafeName(name)),
		FileName:   name, Mime: mime, SizeBytes: in.SizeBytes, Meta: meta,
		ExpiresAt: s.Clock.Now().Add(s.cfg.UploadTTL),
	}
	saved, err := s.Uploads.Create(ctx, u)
	if err != nil {
		return nil, err
	}
	req, err := s.Store.PresignPut(ctx, u.StorageKey, mime, s.cfg.PresignTTL)
	if err != nil {
		return nil, err
	}
	if req == nil {
		exp := s.Clock.Now().Add(s.cfg.PresignTTL)
		token, err := s.links.Sign(localClaims{Op: opPut, Key: u.StorageKey, Mime: mime, Max: s.cfg.MaxUploadBytes}, exp)
		if err != nil {
			return nil, err
		}
		req = &domain.PresignedRequest{
			Method: "PUT", URL: fmt.Sprintf("%s/media/%s", s.cfg.APIBaseURL, url.PathEscape(token)),
			Headers: map[string]string{"Content-Type": mime}, ExpiresAt: exp,
		}
	}
	return &UploadTicket{Upload: *saved, Request: *req, MaxSize: s.cfg.MaxUploadBytes}, nil
}

// checkDriveUpload explains precisely why a copy to Drive is impossible.
func (s *Service) checkDriveUpload(ctx context.Context, allowed, secured bool, groupID uuid.UUID) error {
	if !s.cfg.DriveUploadEnabled || s.Client == nil {
		return domain.Unavailable("uploading to Google Drive is turned off on this server")
	}
	if !allowed {
		return domain.Forbidden(string(authz.DriveUpload))
	}
	if !secured {
		return domain.Forbidden("secure your account to upload to Google Drive")
	}
	conn, err := s.Drive.GetConnectionByGroup(ctx, groupID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Unavailable("the group has no Google Drive folder connected")
	}
	if err != nil {
		return err
	}
	var pub *domain.DrivePublisher
	if s.cfg.DrivePublisherOAuth {
		pub, err = s.Drive.GetPublisher(ctx, groupID)
		if errors.Is(err, domain.ErrNotFound) {
			pub = nil
		} else if err != nil {
			return err
		}
	}
	if domain.CanPublishToDrive(conn, pub) {
		return nil
	}
	switch {
	case conn.DriveID != nil:
		return domain.Unavailable("the service account can only read the group's Drive folder")
	case pub != nil:
		return domain.WithCode(domain.CodeDrivePublisherRevoked,
			domain.Unavailable(fmt.Sprintf("Google rejected the publishing account %s: reconnect it on the Drive screen", pub.Email)))
	}
	return domain.WithCode(domain.CodeDrivePublisherRequired,
		domain.Unavailable(`publishing to "My Drive" needs the head's Google account connected on the Drive screen`))
}

// CompleteUpload checks the stored file and turns the upload into a
// material. Repeating the call returns the same material.
func (s *Service) CompleteUpload(ctx context.Context, actorID, uploadID uuid.UUID) (*domain.MaterialView, error) {
	u, err := s.Uploads.Get(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if u.UserID != actorID {
		return nil, domain.NotFound("upload")
	}
	switch u.Status {
	case domain.UploadCompleted:
		if u.MaterialID == nil {
			return nil, domain.NotFound("material")
		}
		return s.Materials.GetView(ctx, *u.MaterialID)
	case domain.UploadExpired:
		return nil, fmt.Errorf("%w: upload expired", domain.ErrGone)
	}
	if !s.Clock.Now().Before(u.ExpiresAt) {
		return nil, fmt.Errorf("%w: upload expired", domain.ErrGone)
	}
	actor, err := s.Access.Actor(ctx, actorID, u.GroupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.MaterialUpload); err != nil {
		return nil, err
	}

	info, err := s.Store.Stat(ctx, u.StorageKey)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.Invalid("file", "the file has not been uploaded yet")
	}
	if err != nil {
		return nil, err
	}
	if info.Size > s.cfg.MaxUploadBytes {
		_ = s.Store.Delete(ctx, u.StorageKey)
		return nil, fmt.Errorf("%w: file exceeds %d MB", domain.ErrTooLarge, s.cfg.MaxUploadBytes>>20)
	}
	if err := s.sniff(ctx, u); err != nil {
		_ = s.Store.Delete(ctx, u.StorageKey)
		return nil, err
	}

	meta := u.Meta
	toDrive := meta.ToDrive
	if toDrive {
		if err := s.checkDriveUpload(ctx, actor.Can(authz.DriveUpload), actor.User.Secured(), u.GroupID); err != nil {
			return nil, err
		}
	}
	class := domain.Classification{Method: domain.ClassifyUpload, Kind: meta.Kind, SubjectID: meta.SubjectID, Confidence: 1}
	if meta.SubjectID == nil || meta.Kind == "" {
		// Fill what the uploader left out from the file name.
		subjects, err := s.Subjects.List(ctx, u.GroupID, false)
		if err != nil {
			return nil, err
		}
		guess := classify.Classify(classify.Input{FileName: u.FileName, Description: meta.Description}, classify.FromDomain(subjects))
		if meta.Kind == "" {
			class.Kind = guess.Kind
		}
		if meta.SubjectID == nil && guess.SubjectID != nil && guess.Confidence >= s.cfg.MinConfidence {
			class.SubjectID = guess.SubjectID
			class.Method = domain.ClassifyAuto
			class.Confidence = guess.Confidence
			class.Signals = guess.Signals
		}
		class.Topic, class.Semester = guess.Topic, guess.Semester
	}

	materialID := ids.New()
	versionID := ids.New()
	finalKey := fmt.Sprintf("groups/%s/materials/%s/v1/%s", u.GroupID, materialID, filenames.SafeName(u.FileName))
	if err := s.Store.Move(ctx, u.StorageKey, finalKey); err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		m, err := s.Materials.Create(ctx, domain.Material{
			ID: materialID, GroupID: u.GroupID, SubjectID: class.SubjectID, UploaderID: &actorID,
			Title: meta.Title, Description: meta.Description, Kind: class.Kind, Source: domain.SourceUpload,
			Status: domain.MaterialActive, Classification: class, SortAt: now,
		})
		if err != nil {
			return err
		}
		v := domain.MaterialVersion{
			ID: versionID, MaterialID: m.ID, Storage: s.Store.Kind(), StorageKey: &finalKey,
			OriginalName: u.FileName, Mime: u.Mime, SizeBytes: info.Size,
			ScanStatus: domain.ScanSkipped, UploadedBy: &actorID,
		}
		if toDrive {
			pending := domain.DriveUploadPending
			v.DriveUploadStatus = &pending
		}
		if _, err := s.Materials.CreateVersion(ctx, v); err != nil {
			return err
		}
		if err := s.Materials.SetCurrentVersion(ctx, m.ID, versionID); err != nil {
			return err
		}
		// Office files are converted to a PDF preview right away.
		previewing := s.wantsPreview(v.Mime, v.SizeBytes)
		if previewing {
			if _, err := s.Materials.ClaimVersionPreview(ctx, versionID); err != nil {
				return err
			}
		}
		if len(meta.TagIDs) > 0 {
			if err := s.Materials.SetTags(ctx, m.ID, meta.TagIDs); err != nil {
				return err
			}
		}
		ok, err := s.Uploads.Complete(ctx, u.ID, m.ID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.Conflict("upload was completed concurrently")
		}
		payload := domain.MaterialVersionPayload{MaterialID: m.ID.String(), VersionID: versionID.String()}
		s.Tx.AfterCommit(ctx, func() {
			s.enqueue(domain.Job{Type: domain.JobMaterialHash, Payload: payload, Queue: "low", MaxRetry: 3})
			if toDrive {
				s.enqueue(domain.Job{Type: domain.JobDriveUpload, Payload: payload, Queue: "default", MaxRetry: 5})
			}
			if previewing {
				s.enqueuePreview(m.ID, versionID)
			}
		})
		return s.emit(ctx, u.GroupID, domain.EventMaterialAdded, &actorID, m.ID, map[string]any{
			"title": m.Title, "subject_id": m.SubjectID, "kind": m.Kind, "source": m.Source,
		})
	})
	if err != nil {
		// Put the file back so the client can retry completing the upload.
		if mvErr := s.Store.Move(ctx, finalKey, u.StorageKey); mvErr != nil {
			s.Log.Error("restore upload after failed completion", "upload", u.ID, "err", mvErr)
		}
		if errors.Is(err, domain.ErrConflict) {
			if again, getErr := s.Uploads.Get(ctx, u.ID); getErr == nil && again.MaterialID != nil {
				return s.Materials.GetView(ctx, *again.MaterialID)
			}
		}
		return nil, err
	}
	return s.Materials.GetView(ctx, materialID)
}

func (s *Service) sniff(ctx context.Context, u *domain.Upload) error {
	r, _, err := s.Store.Open(ctx, u.StorageKey, 0, sniffLen)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	head, err := io.ReadAll(io.LimitReader(r, sniffLen))
	if err != nil {
		return fmt.Errorf("read upload head: %w", err)
	}
	return CheckContent(Extension(u.FileName), head)
}

// enqueue schedules background work after commit; failures are logged, the
// periodic jobs catch up.
func (s *Service) enqueue(job domain.Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Queue.Enqueue(ctx, job); err != nil {
		s.Log.Error("enqueue job", "type", job.Type, "err", err)
	}
}

// HashVersion computes the SHA-256 of a stored version (dedup, integrity).
func (s *Service) HashVersion(ctx context.Context, versionID uuid.UUID) error {
	v, err := s.Materials.GetVersion(ctx, versionID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if v.SHA256 != nil || v.StorageKey == nil || v.Storage == domain.StorageDrive {
		return nil
	}
	if s.cfg.HashMaxBytes > 0 && v.SizeBytes > s.cfg.HashMaxBytes {
		return nil
	}
	r, _, err := s.Store.Open(ctx, *v.StorageKey, 0, -1)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return fmt.Errorf("hash version: %w", err)
	}
	return s.Materials.SetVersionHash(ctx, v.ID, hex.EncodeToString(h.Sum(nil)))
}

// CleanupUploads removes files of abandoned uploads.
func (s *Service) CleanupUploads(ctx context.Context) (int, error) {
	expired, err := s.Uploads.ListExpired(ctx, s.Clock.Now(), 500)
	if err != nil {
		return 0, err
	}
	for _, u := range expired {
		if err := s.Store.Delete(ctx, u.StorageKey); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return 0, err
		}
		if err := s.Uploads.MarkExpired(ctx, u.ID); err != nil {
			return 0, err
		}
	}
	return len(expired), nil
}

// PurgeDeleted removes deleted materials past the grace period together with
// their files in our storage. Files on Google Drive are never touched; their
// index entries are marked SKIPPED so the next sync does not bring them back.
func (s *Service) PurgeDeleted(ctx context.Context) (int, error) {
	list, err := s.Materials.ListPurgeable(ctx, s.Clock.Now().Add(-s.cfg.HardDeleteAfter), 200)
	if err != nil {
		return 0, err
	}
	for _, m := range list {
		versions, err := s.Materials.ListVersions(ctx, m.ID)
		if err != nil {
			return 0, err
		}
		for _, v := range versions {
			if v.StorageKey != nil && v.Storage != domain.StorageDrive {
				if err := s.Store.Delete(ctx, *v.StorageKey); err != nil && !errors.Is(err, domain.ErrNotFound) {
					return 0, err
				}
			}
			if v.PreviewKey != nil {
				if err := s.Store.Delete(ctx, *v.PreviewKey); err != nil && !errors.Is(err, domain.ErrNotFound) {
					return 0, err
				}
			}
		}
		err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
			item, err := s.Drive.GetItemByMaterial(ctx, m.ID)
			switch {
			case err == nil:
				reason := "deleted in the app"
				if err := s.Drive.UpdateItemState(ctx, item.ID, domain.DriveItemSkipped, nil, item.Classification, &reason); err != nil {
					return err
				}
			case !errors.Is(err, domain.ErrNotFound):
				return err
			}
			return s.Materials.HardDelete(ctx, m.ID)
		})
		if err != nil {
			return 0, err
		}
	}
	return len(list), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
