package drive

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/classify"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/ids"
)

// ErrPermanent marks failures that retrying will not fix.
var ErrPermanent = errors.New("permanent failure")

// UploadVersion publishes an uploaded file to the group's Drive folder: on
// behalf of the group's publishing Google account when one is connected (D34),
// otherwise by the service account, which can only write into shared drives.
func (s *Service) UploadVersion(ctx context.Context, materialID, versionID uuid.UUID) error {
	v, err := s.Materials.GetVersion(ctx, versionID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if v.MaterialID != materialID || v.DriveFileID != nil {
		return nil // mismatch or already published
	}
	fail := func(msg string) error {
		if err := s.Materials.SetVersionDriveUpload(ctx, v.ID, domain.DriveUploadFailed, &msg, nil, nil); err != nil {
			return err
		}
		return fmt.Errorf("%w: %s", ErrPermanent, msg)
	}
	if !s.cfg.UploadEnabled || s.Client == nil {
		return fail("uploading to Google Drive is turned off")
	}
	if v.StorageKey == nil || v.Storage == domain.StorageDrive {
		return fail("nothing to upload")
	}
	m, err := s.Materials.Get(ctx, materialID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if m.Status == domain.MaterialDeleted {
		return fail("the material was deleted")
	}
	conn, err := s.Repo.GetConnectionByGroup(ctx, m.GroupID)
	if errors.Is(err, domain.ErrNotFound) {
		return fail("the group has no Google Drive folder connected")
	}
	if err != nil {
		return err
	}
	pub, client, err := s.publisher(ctx, m.GroupID)
	if err != nil {
		return fail("cannot use the publishing Google account: " + err.Error())
	}
	if !pub.Usable() {
		client = s.Client
		switch {
		case !conn.Writable:
			return fail("the service account can only read the group's Drive folder")
		case conn.DriveID != nil:
			// A shared drive: the service account uploads by itself.
		case pub != nil:
			return fail(fmt.Sprintf("Google rejected the publishing account %s: reconnect it on the Drive screen", pub.Email))
		default:
			return fail("publishing to \"My Drive\" needs the head's Google account: connect it on the Drive screen")
		}
		pub = nil
	}
	parentID, path, err := s.uploadFolder(ctx, conn, m.SubjectID)
	if err != nil {
		return err
	}
	uploader := "участник группы"
	if v.UploadedBy != nil {
		if u, err := s.Users.GetByID(ctx, *v.UploadedBy); err == nil {
			uploader = u.Name
		}
	}
	body, _, err := s.Store.Open(ctx, *v.StorageKey, 0, -1)
	if errors.Is(err, domain.ErrNotFound) {
		return fail("the file is missing in storage")
	}
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()

	file, err := client.Upload(ctx, domain.DriveUpload{
		Name:        v.OriginalName,
		MimeType:    v.Mime,
		ParentID:    parentID,
		Description: fmt.Sprintf("Загрузил: %s (через Heatseeker)", uploader),
		AppProperties: map[string]string{
			PropMaterialID: m.ID.String(),
			PropGroupID:    m.GroupID.String(),
		},
		Body: body,
	})
	switch {
	case pub != nil && domain.ErrorCode(err) == domain.CodeDrivePublisherRevoked:
		msg := err.Error()
		if serr := s.Repo.SetPublisherError(ctx, m.GroupID, &msg); serr != nil {
			return serr
		}
		return fail(fmt.Sprintf("Google rejected the publishing account %s: reconnect it on the Drive screen", pub.Email))
	case pub != nil && errors.Is(err, domain.ErrDriveQuota):
		return fail(fmt.Sprintf("the Google Drive of %s is full", pub.Email))
	case pub != nil && (errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound)):
		return fail(fmt.Sprintf("%s cannot write to the group's Drive folder", pub.Email))
	case errors.Is(err, domain.ErrDriveQuota):
		return fail("Google Drive refused the file: the service account has no storage quota. Connect the head's Google account or use a shared drive")
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrNotFound):
		return fail("the service account cannot write to the group's Drive folder")
	case err != nil:
		msg := err.Error()
		_ = s.Materials.SetVersionDriveUpload(ctx, v.ID, domain.DriveUploadPending, &msg, nil, nil)
		return err
	}

	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		link := strPtr(file.WebViewLink)
		if err := s.Materials.SetVersionDriveUpload(ctx, v.ID, domain.DriveUploadDone, nil, &file.ID, link); err != nil {
			return err
		}
		item, err := s.Repo.UpsertItem(ctx, domain.DriveItem{
			ID: ids.New(), ConnectionID: conn.ID, DriveFileID: file.ID, ParentID: &parentID, PathCache: joinPath(path),
			Name: file.Name, Mime: file.MimeType, MD5: strPtr(file.MD5), SizeBytes: &file.Size,
			ModifiedTime: timePtr(file.ModifiedTime), WebViewLink: link, MaterialID: &m.ID,
			State: domain.DriveItemLinked, Classification: m.Classification, SeenAt: s.Clock.Now(),
		})
		if err != nil {
			return err
		}
		if err := s.Repo.UpdateItemState(ctx, item.ID, domain.DriveItemLinked, &m.ID, m.Classification, nil); err != nil {
			return err
		}
		return s.emit(ctx, m.GroupID, domain.EventMaterialUpdated, v.UploadedBy, m.ID, map[string]any{
			"title": m.Title, "reason": "copied_to_drive",
		}, true)
	})
}

// uploadFolder picks the top-level folder named after the subject, or the
// root when there is none.
func (s *Service) uploadFolder(ctx context.Context, conn *domain.DriveConnection, subjectID *uuid.UUID) (string, []string, error) {
	if subjectID == nil {
		return conn.RootFolderID, nil, nil
	}
	subject, err := s.Subjects.Get(ctx, *subjectID, conn.GroupID)
	if errors.Is(err, domain.ErrNotFound) {
		return conn.RootFolderID, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	folders, err := s.Repo.ListFolders(ctx, conn.ID)
	if err != nil {
		return "", nil, err
	}
	target := classify.FromDomain([]domain.Subject{*subject})[0]
	best, bestScore := (*domain.DriveItem)(nil), 0.0
	for i := range folders {
		f := &folders[i]
		if ptrValue(f.ParentID) != conn.RootFolderID {
			continue
		}
		score := classify.MatchScore(f.Name, target)
		if score > bestScore || (score == bestScore && best != nil && strings.Compare(f.Name, best.Name) < 0) {
			best, bestScore = f, score
		}
	}
	if best == nil || bestScore < 0.75 {
		return conn.RootFolderID, nil, nil
	}
	return best.DriveFileID, []string{best.Name}, nil
}
