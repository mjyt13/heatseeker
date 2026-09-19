package materials

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

// PreviewNone is reported for an office file whose preview was not requested.
const PreviewNone = "NONE"

// PreviewsEnabled reports whether office files get PDF previews.
func (s *Service) PreviewsEnabled() bool { return s.Converter != nil }

// PreviewState is what clients see about a version's preview: "" for files
// shown as they are (or when previews are off), PreviewNone when not yet
// requested, otherwise the stored status.
func (s *Service) PreviewState(v *domain.MaterialVersion) string {
	if !s.PreviewsEnabled() || !domain.NeedsPDFPreview(v.Mime) {
		return ""
	}
	if v.PreviewStatus == nil {
		return PreviewNone
	}
	return string(*v.PreviewStatus)
}

// RequestPreview asks for the PDF preview of a version (the current one when
// versionID is nil) and returns its state. Converting takes a few seconds;
// clients poll the material until the state is READY.
func (s *Service) RequestPreview(ctx context.Context, actorID, materialID uuid.UUID, versionID *uuid.UUID) (string, error) {
	view, _, err := s.loadView(ctx, actorID, materialID)
	if err != nil {
		return "", err
	}
	v := view.Version
	if versionID != nil && *versionID != v.ID {
		other, err := s.Materials.GetVersion(ctx, *versionID)
		if err != nil {
			return "", err
		}
		if other.MaterialID != materialID {
			return "", domain.NotFound("material version")
		}
		v = *other
	}
	if !s.PreviewsEnabled() {
		return "", domain.Unavailable("previews of office files are turned off on this server")
	}
	if !domain.NeedsPDFPreview(v.Mime) {
		return "", domain.Invalid("file", "this file is shown as is, without a preview")
	}
	if st := s.PreviewState(&v); st == string(domain.PreviewPending) || st == string(domain.PreviewReady) || st == string(domain.PreviewSkipped) {
		return st, nil
	}
	if err := s.requestPreview(ctx, materialID, v.ID); err != nil {
		return "", err
	}
	return string(domain.PreviewPending), nil
}

// requestPreview claims the version and enqueues the conversion.
func (s *Service) requestPreview(ctx context.Context, materialID, versionID uuid.UUID) error {
	claimed, err := s.Materials.ClaimVersionPreview(ctx, versionID)
	if err != nil || !claimed {
		return err
	}
	s.enqueuePreview(materialID, versionID)
	return nil
}

func (s *Service) enqueuePreview(materialID, versionID uuid.UUID) {
	s.enqueue(domain.Job{
		Type:    domain.JobMaterialPreview,
		Payload: domain.MaterialVersionPayload{MaterialID: materialID.String(), VersionID: versionID.String()},
		Queue:   "low", MaxRetry: 3,
	})
}

// wantsPreview reports whether a new upload is converted right away.
func (s *Service) wantsPreview(mime string, size int64) bool {
	return s.PreviewsEnabled() && domain.NeedsPDFPreview(mime) && size <= s.cfg.PreviewMaxBytes
}

func previewKey(groupID, versionID uuid.UUID) string {
	return fmt.Sprintf("groups/%s/previews/%s.pdf", groupID, versionID)
}

// previewName is the file name the browser shows for a preview.
func previewName(original string) string {
	return strings.TrimSuffix(original, path.Ext(original)) + ".pdf"
}

// BuildPreview converts a version to PDF and stores the result (worker job).
// Files the converter rejects are marked FAILED and nil is returned; other
// errors are returned for a retry.
func (s *Service) BuildPreview(ctx context.Context, versionID uuid.UUID) error {
	v, err := s.Materials.GetVersion(ctx, versionID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if v.PreviewStatus != nil && *v.PreviewStatus == domain.PreviewReady {
		return nil
	}
	fail := func(status domain.PreviewStatus, msg string) error {
		return s.Materials.SetVersionPreview(ctx, v.ID, status, nil, &msg)
	}
	if !s.PreviewsEnabled() || !domain.NeedsPDFPreview(v.Mime) {
		return fail(domain.PreviewSkipped, "no preview for this file type")
	}
	if v.SizeBytes > s.cfg.PreviewMaxBytes {
		return fail(domain.PreviewSkipped, fmt.Sprintf("the file is larger than %d MB", s.cfg.PreviewMaxBytes>>20))
	}
	m, err := s.Materials.Get(ctx, v.MaterialID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	src, err := s.openSource(ctx, m, v)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrGone) {
		return fail(domain.PreviewFailed, "the file is no longer available")
	}
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	pdf, err := s.Converter.ConvertToPDF(ctx, v.OriginalName, src)
	if errors.Is(err, domain.ErrInvalid) {
		s.Log.Warn("office preview rejected", "version", v.ID, "err", err)
		return fail(domain.PreviewFailed, err.Error())
	}
	if err != nil {
		return err
	}
	defer func() { _ = pdf.Close() }()
	key := previewKey(m.GroupID, v.ID)
	if err := s.Store.Put(ctx, key, pdf, -1, "application/pdf"); err != nil {
		return err
	}
	return s.Materials.SetVersionPreview(ctx, v.ID, domain.PreviewReady, &key, nil)
}

// PreviewGaveUp marks a preview FAILED after the last retry.
func (s *Service) PreviewGaveUp(ctx context.Context, versionID uuid.UUID, cause error) error {
	msg := cause.Error()
	if runes := []rune(msg); len(runes) > 500 {
		msg = string(runes[:500])
	}
	return s.Materials.SetVersionPreview(ctx, versionID, domain.PreviewFailed, nil, &msg)
}

// openSource reads a version's bytes wherever they live.
func (s *Service) openSource(ctx context.Context, m *domain.Material, v *domain.MaterialVersion) (io.ReadCloser, error) {
	switch v.Storage {
	case domain.StorageS3, domain.StorageLocal:
		if v.StorageKey == nil {
			return nil, domain.NotFound("file")
		}
		r, _, err := s.Store.Open(ctx, *v.StorageKey, 0, -1)
		return r, err
	case domain.StorageDrive:
		if s.Client == nil || v.DriveFileID == nil {
			return nil, domain.NotFound("file")
		}
		var dc *domain.DriveContent
		var err error
		if m.CurrentVersionID != nil && *m.CurrentVersionID == v.ID {
			dc, err = s.Client.Download(ctx, *v.DriveFileID, "")
		} else {
			var revisionID string
			if revisionID, err = s.revisionOf(ctx, v, false); err == nil {
				dc, err = s.Client.DownloadRevision(ctx, *v.DriveFileID, revisionID, "")
			}
		}
		if err != nil {
			return nil, err
		}
		return dc.Body, nil
	}
	return nil, fmt.Errorf("unknown storage %q", v.Storage)
}
