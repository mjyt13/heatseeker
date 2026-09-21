package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type materialRepo struct{ s *Store }

// materialRow mirrors the column list shared by the material queries; sqlc
// emits a distinct row type per query, all convertible to this one.
type materialRow struct {
	ID               uuid.UUID
	GroupID          uuid.UUID
	SubjectID        *uuid.UUID
	UploaderID       *uuid.UUID
	Title            string
	Description      string
	Kind             string
	Source           string
	Status           string
	CurrentVersionID *uuid.UUID
	Classification   []byte
	NeedsReview      bool
	ReviewReason     *string
	DownloadCount    int32
	SortAt           time.Time
	ArchivedBy       *uuid.UUID
	ArchivedAt       *time.Time
	DeletedBy        *uuid.UUID
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	TaskID           *uuid.UUID
}

func toMaterial(r materialRow) *domain.Material {
	m := &domain.Material{
		ID:               r.ID,
		GroupID:          r.GroupID,
		SubjectID:        r.SubjectID,
		UploaderID:       r.UploaderID,
		Title:            r.Title,
		Description:      r.Description,
		Kind:             domain.MaterialKind(r.Kind),
		Source:           domain.MaterialSource(r.Source),
		Status:           domain.MaterialStatus(r.Status),
		CurrentVersionID: r.CurrentVersionID,
		Classification:   domain.ParseClassification(r.Classification),
		NeedsReview:      r.NeedsReview,
		DownloadCount:    r.DownloadCount,
		SortAt:           r.SortAt,
		ArchivedBy:       r.ArchivedBy,
		ArchivedAt:       r.ArchivedAt,
		DeletedBy:        r.DeletedBy,
		DeletedAt:        r.DeletedAt,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		TaskID:           r.TaskID,
	}
	if r.ReviewReason != nil {
		reason := domain.ReviewReason(*r.ReviewReason)
		m.ReviewReason = &reason
	}
	return m
}

func toVersion(v sqlcgen.MaterialVersion) *domain.MaterialVersion {
	out := &domain.MaterialVersion{
		ID:                v.ID,
		MaterialID:        v.MaterialID,
		VersionNo:         v.VersionNo,
		Storage:           domain.StorageKind(v.Storage),
		StorageKey:        v.StorageKey,
		CacheExpiresAt:    v.CacheExpiresAt,
		DriveFileID:       v.DriveFileID,
		DriveWebViewLink:  v.DriveWebViewLink,
		DriveMD5:          v.DriveMd5,
		DriveModifiedTime: v.DriveModifiedTime,
		DriveRevisionID:   v.DriveRevisionID,
		DriveUploadError:  v.DriveUploadError,
		OriginalName:      v.OriginalName,
		Mime:              v.Mime,
		SizeBytes:         v.SizeBytes,
		SHA256:            v.Sha256,
		ScanStatus:        domain.ScanStatus(v.ScanStatus),
		PreviewKey:        v.PreviewKey,
		PreviewError:      v.PreviewError,
		UploadedBy:        v.UploadedBy,
		CreatedAt:         v.CreatedAt,
	}
	if v.PreviewStatus != nil {
		ps := domain.PreviewStatus(*v.PreviewStatus)
		out.PreviewStatus = &ps
	}
	if v.DriveUploadStatus != nil {
		st := domain.DriveUploadStatus(*v.DriveUploadStatus)
		out.DriveUploadStatus = &st
	}
	return out
}

func reasonPtr(r *domain.ReviewReason) *string {
	if r == nil {
		return nil
	}
	s := string(*r)
	return &s
}

func (r *materialRepo) Create(ctx context.Context, m domain.Material) (*domain.Material, error) {
	row, err := r.s.queries(ctx).CreateMaterial(ctx, sqlcgen.CreateMaterialParams{
		ID:             m.ID,
		GroupID:        m.GroupID,
		SubjectID:      m.SubjectID,
		UploaderID:     m.UploaderID,
		Title:          m.Title,
		Description:    m.Description,
		Kind:           string(m.Kind),
		Source:         string(m.Source),
		Status:         string(m.Status),
		Classification: m.Classification.JSON(),
		NeedsReview:    m.NeedsReview,
		ReviewReason:   reasonPtr(m.ReviewReason),
		SortAt:         m.SortAt,
		TaskID:         m.TaskID,
	})
	if err != nil {
		return nil, mapErr(err, "material")
	}
	return toMaterial(materialRow(row)), nil
}

func (r *materialRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Material, error) {
	row, err := r.s.queries(ctx).GetMaterial(ctx, id)
	if err != nil {
		return nil, mapErr(err, "material")
	}
	return toMaterial(materialRow(row)), nil
}

func (r *materialRepo) GetView(ctx context.Context, id uuid.UUID) (*domain.MaterialView, error) {
	q := r.s.queries(ctx)
	row, err := q.GetMaterialView(ctx, id)
	if err != nil {
		return nil, mapErr(err, "material")
	}
	view := &domain.MaterialView{
		Material: *toMaterial(materialRow{
			ID: row.ID, GroupID: row.GroupID, SubjectID: row.SubjectID, UploaderID: row.UploaderID, Title: row.Title,
			Description: row.Description, Kind: row.Kind, Source: row.Source, Status: row.Status,
			CurrentVersionID: row.CurrentVersionID, Classification: row.Classification, NeedsReview: row.NeedsReview,
			ReviewReason: row.ReviewReason, DownloadCount: row.DownloadCount, SortAt: row.SortAt,
			ArchivedBy: row.ArchivedBy, ArchivedAt: row.ArchivedAt, DeletedBy: row.DeletedBy, DeletedAt: row.DeletedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TaskID: row.TaskID,
		}),
		Version: *toVersion(row.MaterialVersion),
		TagIDs:  []uuid.UUID{},
	}
	tags, err := q.ListMaterialTags(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, mapErr(err, "material tags")
	}
	for _, t := range tags {
		view.TagIDs = append(view.TagIDs, t.TagID)
	}
	return view, nil
}

func (r *materialRepo) List(ctx context.Context, f domain.MaterialFilter) ([]domain.MaterialView, error) {
	q := r.s.queries(ctx)
	params := sqlcgen.ListMaterialsParams{
		GroupID:    f.GroupID,
		Status:     string(f.Status),
		SubjectID:  f.SubjectID,
		NoSubject:  f.NoSubject,
		UploaderID: f.UploaderID,
		Inbox:      f.Inbox,
		TagIds:     f.TagIDs,
		MaxRows:    f.Limit,
	}
	if params.TagIds == nil {
		params.TagIds = []uuid.UUID{}
	}
	params.MimePatterns = []string{}
	if f.FileType != nil {
		params.MimePatterns, params.MimeExclude = f.FileType.MimePatterns()
	}
	if f.Kind != nil {
		k := string(*f.Kind)
		params.Kind = &k
	}
	if query := strings.TrimSpace(f.Query); query != "" {
		pattern := likeEscaper.Replace(query)
		params.Query = &query
		params.LikePattern = &pattern
	}
	if f.After != nil {
		params.AfterSortAt = &f.After.SortAt
		params.AfterID = &f.After.ID
	}
	rows, err := q.ListMaterials(ctx, params)
	if err != nil {
		return nil, mapErr(err, "material")
	}
	out := make([]domain.MaterialView, len(rows))
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, row := range rows {
		out[i] = domain.MaterialView{
			Material: *toMaterial(materialRow{
				ID: row.ID, GroupID: row.GroupID, SubjectID: row.SubjectID, UploaderID: row.UploaderID, Title: row.Title,
				Description: row.Description, Kind: row.Kind, Source: row.Source, Status: row.Status,
				CurrentVersionID: row.CurrentVersionID, Classification: row.Classification, NeedsReview: row.NeedsReview,
				ReviewReason: row.ReviewReason, DownloadCount: row.DownloadCount, SortAt: row.SortAt,
				ArchivedBy: row.ArchivedBy, ArchivedAt: row.ArchivedAt, DeletedBy: row.DeletedBy, DeletedAt: row.DeletedAt,
				CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TaskID: row.TaskID,
			}),
			Version: *toVersion(row.MaterialVersion),
			TagIDs:  []uuid.UUID{},
		}
		ids[i] = row.ID
		index[row.ID] = i
	}
	if len(ids) == 0 {
		return out, nil
	}
	tags, err := q.ListMaterialTags(ctx, ids)
	if err != nil {
		return nil, mapErr(err, "material tags")
	}
	for _, t := range tags {
		if i, ok := index[t.MaterialID]; ok {
			out[i].TagIDs = append(out[i].TagIDs, t.TagID)
		}
	}
	return out, nil
}

// likeEscaper escapes LIKE metacharacters (the query uses ESCAPE '\').
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (r *materialRepo) CountInbox(ctx context.Context, groupID uuid.UUID) (int64, error) {
	n, err := r.s.queries(ctx).CountInbox(ctx, groupID)
	return n, mapErr(err, "material")
}

func (r *materialRepo) Update(ctx context.Context, p domain.UpdateMaterialParams) (*domain.Material, error) {
	row, err := r.s.queries(ctx).UpdateMaterial(ctx, sqlcgen.UpdateMaterialParams{
		ID:             p.ID,
		Title:          p.Title,
		Description:    p.Description,
		SubjectID:      p.SubjectID,
		Kind:           string(p.Kind),
		Classification: p.Classification.JSON(),
		NeedsReview:    p.NeedsReview,
		ReviewReason:   reasonPtr(p.ReviewReason),
	})
	if err != nil {
		return nil, mapErr(err, "material")
	}
	return toMaterial(materialRow(row)), nil
}

func (r *materialRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.MaterialStatus, actor *uuid.UUID) (*domain.Material, error) {
	row, err := r.s.queries(ctx).SetMaterialStatus(ctx, sqlcgen.SetMaterialStatusParams{ID: id, Status: string(status), Actor: actor})
	if err != nil {
		return nil, mapErr(err, "material")
	}
	return toMaterial(materialRow(row)), nil
}

func (r *materialRepo) Share(ctx context.Context, id uuid.UUID) (*domain.Material, error) {
	row, err := r.s.queries(ctx).ShareMaterial(ctx, id)
	if err != nil {
		return nil, mapErr(err, "material")
	}
	return toMaterial(materialRow(row)), nil
}

func (r *materialRepo) IncrementDownloads(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.s.queries(ctx).IncrementMaterialDownloads(ctx, id), "material")
}

func (r *materialRepo) SetTags(ctx context.Context, id uuid.UUID, tagIDs []uuid.UUID) error {
	q := r.s.queries(ctx)
	if err := q.DeleteMaterialTags(ctx, id); err != nil {
		return mapErr(err, "material tags")
	}
	if len(tagIDs) == 0 {
		return nil
	}
	return mapErr(q.InsertMaterialTags(ctx, sqlcgen.InsertMaterialTagsParams{MaterialID: id, TagIds: tagIDs}), "material tags")
}

func (r *materialRepo) ListPurgeable(ctx context.Context, deletedBefore time.Time, limit int32) ([]domain.Material, error) {
	rows, err := r.s.queries(ctx).ListPurgeableMaterials(ctx, sqlcgen.ListPurgeableMaterialsParams{DeletedAt: &deletedBefore, Limit: limit})
	if err != nil {
		return nil, mapErr(err, "material")
	}
	out := make([]domain.Material, len(rows))
	for i, row := range rows {
		out[i] = *toMaterial(materialRow(row))
	}
	return out, nil
}

func (r *materialRepo) HardDelete(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.s.queries(ctx).HardDeleteMaterial(ctx, id), "material")
}

func (r *materialRepo) CreateVersion(ctx context.Context, v domain.MaterialVersion) (*domain.MaterialVersion, error) {
	var uploadStatus *string
	if v.DriveUploadStatus != nil {
		s := string(*v.DriveUploadStatus)
		uploadStatus = &s
	}
	row, err := r.s.queries(ctx).CreateMaterialVersion(ctx, sqlcgen.CreateMaterialVersionParams{
		ID:                v.ID,
		MaterialID:        v.MaterialID,
		Storage:           string(v.Storage),
		StorageKey:        v.StorageKey,
		DriveFileID:       v.DriveFileID,
		DriveWebViewLink:  v.DriveWebViewLink,
		DriveMd5:          v.DriveMD5,
		DriveModifiedTime: v.DriveModifiedTime,
		DriveRevisionID:   v.DriveRevisionID,
		DriveUploadStatus: uploadStatus,
		OriginalName:      v.OriginalName,
		Mime:              v.Mime,
		SizeBytes:         v.SizeBytes,
		Sha256:            v.SHA256,
		ScanStatus:        string(v.ScanStatus),
		UploadedBy:        v.UploadedBy,
	})
	if err != nil {
		return nil, mapErr(err, "material version")
	}
	return toVersion(row), nil
}

func (r *materialRepo) SetCurrentVersion(ctx context.Context, materialID, versionID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).SetCurrentVersion(ctx, sqlcgen.SetCurrentVersionParams{ID: materialID, CurrentVersionID: &versionID}), "material")
}

func (r *materialRepo) GetVersion(ctx context.Context, id uuid.UUID) (*domain.MaterialVersion, error) {
	row, err := r.s.queries(ctx).GetMaterialVersion(ctx, id)
	if err != nil {
		return nil, mapErr(err, "material version")
	}
	return toVersion(row), nil
}

func (r *materialRepo) ListVersions(ctx context.Context, materialID uuid.UUID) ([]domain.MaterialVersion, error) {
	rows, err := r.s.queries(ctx).ListMaterialVersions(ctx, materialID)
	if err != nil {
		return nil, mapErr(err, "material version")
	}
	out := make([]domain.MaterialVersion, len(rows))
	for i, row := range rows {
		out[i] = *toVersion(row)
	}
	return out, nil
}

func (r *materialRepo) UpdateVersionFile(ctx context.Context, v domain.MaterialVersion) error {
	return mapErr(r.s.queries(ctx).UpdateVersionFile(ctx, sqlcgen.UpdateVersionFileParams{
		ID:                v.ID,
		OriginalName:      v.OriginalName,
		Mime:              v.Mime,
		SizeBytes:         v.SizeBytes,
		DriveWebViewLink:  v.DriveWebViewLink,
		DriveMd5:          v.DriveMD5,
		DriveModifiedTime: v.DriveModifiedTime,
		DriveRevisionID:   v.DriveRevisionID,
	}), "material version")
}

func (r *materialRepo) ClaimVersionPreview(ctx context.Context, versionID uuid.UUID) (bool, error) {
	_, err := r.s.queries(ctx).ClaimVersionPreview(ctx, versionID)
	switch err = mapErr(err, "material version"); {
	case err == nil:
		return true, nil
	case errors.Is(err, domain.ErrNotFound): // already requested or tried
		return false, nil
	}
	return false, err
}

func (r *materialRepo) SetVersionPreview(ctx context.Context, versionID uuid.UUID, status domain.PreviewStatus, key, errMsg *string) error {
	st := string(status)
	return mapErr(r.s.queries(ctx).SetVersionPreview(ctx, sqlcgen.SetVersionPreviewParams{
		ID: versionID, PreviewStatus: &st, PreviewKey: key, PreviewError: errMsg,
	}), "material version")
}

func (r *materialRepo) SetVersionDriveRevision(ctx context.Context, versionID uuid.UUID, revisionID string) error {
	return mapErr(r.s.queries(ctx).SetVersionDriveRevision(ctx, sqlcgen.SetVersionDriveRevisionParams{
		ID: versionID, DriveRevisionID: &revisionID,
	}), "material version")
}

func (r *materialRepo) SetVersionDriveUpload(ctx context.Context, versionID uuid.UUID, status domain.DriveUploadStatus, errMsg, fileID, webViewLink *string) error {
	st := string(status)
	return mapErr(r.s.queries(ctx).SetVersionDriveUpload(ctx, sqlcgen.SetVersionDriveUploadParams{
		ID:                versionID,
		DriveUploadStatus: &st,
		DriveUploadError:  errMsg,
		FileID:            fileID,
		WebViewLink:       webViewLink,
	}), "material version")
}

func (r *materialRepo) SetVersionHash(ctx context.Context, versionID uuid.UUID, sha256 string) error {
	return mapErr(r.s.queries(ctx).SetVersionHash(ctx, sqlcgen.SetVersionHashParams{ID: versionID, Sha256: &sha256}), "material version")
}

// --- uploads ---

type uploadRepo struct{ s *Store }

func toUpload(u sqlcgen.Upload) *domain.Upload {
	return &domain.Upload{
		ID:          u.ID,
		GroupID:     u.GroupID,
		UserID:      u.UserID,
		Storage:     domain.StorageKind(u.Storage),
		StorageKey:  u.StorageKey,
		FileName:    u.FileName,
		Mime:        u.Mime,
		SizeBytes:   u.SizeBytes,
		Meta:        parseUploadMeta(u.Meta),
		Status:      domain.UploadStatus(u.Status),
		MaterialID:  u.MaterialID,
		ExpiresAt:   u.ExpiresAt,
		CompletedAt: u.CompletedAt,
		CreatedAt:   u.CreatedAt,
	}
}

func (r *uploadRepo) Create(ctx context.Context, u domain.Upload) (*domain.Upload, error) {
	meta, err := marshalJSON(u.Meta)
	if err != nil {
		return nil, err
	}
	row, err := r.s.queries(ctx).CreateUpload(ctx, sqlcgen.CreateUploadParams{
		ID:         u.ID,
		GroupID:    u.GroupID,
		UserID:     u.UserID,
		Storage:    string(u.Storage),
		StorageKey: u.StorageKey,
		FileName:   u.FileName,
		Mime:       u.Mime,
		SizeBytes:  u.SizeBytes,
		Meta:       meta,
		ExpiresAt:  u.ExpiresAt,
	})
	if err != nil {
		return nil, mapErr(err, "upload")
	}
	return toUpload(row), nil
}

func (r *uploadRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Upload, error) {
	row, err := r.s.queries(ctx).GetUpload(ctx, id)
	if err != nil {
		return nil, mapErr(err, "upload")
	}
	return toUpload(row), nil
}

func (r *uploadRepo) Complete(ctx context.Context, id, materialID uuid.UUID) (bool, error) {
	n, err := r.s.queries(ctx).CompleteUpload(ctx, sqlcgen.CompleteUploadParams{ID: id, MaterialID: &materialID})
	return n == 1, mapErr(err, "upload")
}

func (r *uploadRepo) ListExpired(ctx context.Context, now time.Time, limit int32) ([]domain.Upload, error) {
	rows, err := r.s.queries(ctx).ListExpiredUploads(ctx, sqlcgen.ListExpiredUploadsParams{ExpiresAt: now, Limit: limit})
	if err != nil {
		return nil, mapErr(err, "upload")
	}
	out := make([]domain.Upload, len(rows))
	for i, row := range rows {
		out[i] = *toUpload(row)
	}
	return out, nil
}

func (r *uploadRepo) MarkExpired(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.s.queries(ctx).MarkUploadExpired(ctx, id), "upload")
}
