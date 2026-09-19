package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type driveRepo struct{ s *Store }

func toConnection(c sqlcgen.DriveConnection) *domain.DriveConnection {
	return &domain.DriveConnection{
		ID:               c.ID,
		GroupID:          c.GroupID,
		RootFolderID:     c.RootFolderID,
		RootFolderName:   c.RootFolderName,
		DriveID:          c.DriveID,
		Status:           domain.DriveConnectionStatus(c.Status),
		LastError:        c.LastError,
		LastErrorCode:    c.LastErrorCode,
		SyncStartedAt:    c.SyncStartedAt,
		LastSyncAt:       c.LastSyncAt,
		LastFullScanAt:   c.LastFullScanAt,
		ChangesPageToken: c.ChangesPageToken,
		SyncIntervalSec:  c.SyncIntervalSec,
		Writable:         c.Writable,
		CreatedBy:        c.CreatedBy,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

func toDriveItem(i sqlcgen.DriveItem) *domain.DriveItem {
	return &domain.DriveItem{
		ID:             i.ID,
		ConnectionID:   i.ConnectionID,
		DriveFileID:    i.DriveFileID,
		ParentID:       i.ParentID,
		PathCache:      i.PathCache,
		Name:           i.Name,
		Mime:           i.Mime,
		IsFolder:       i.IsFolder,
		MD5:            i.Md5,
		SizeBytes:      i.SizeBytes,
		ModifiedTime:   i.ModifiedTime,
		WebViewLink:    i.WebViewLink,
		MaterialID:     i.MaterialID,
		State:          domain.DriveItemState(i.State),
		Classification: domain.ParseClassification(i.Classification),
		LastError:      i.LastError,
		SeenAt:         i.SeenAt,
		CreatedAt:      i.CreatedAt,
		UpdatedAt:      i.UpdatedAt,
	}
}

func toDriveItems(rows []sqlcgen.DriveItem) []domain.DriveItem {
	out := make([]domain.DriveItem, len(rows))
	for i, row := range rows {
		out[i] = *toDriveItem(row)
	}
	return out
}

func (r *driveRepo) UpsertConnection(ctx context.Context, c domain.DriveConnection) (*domain.DriveConnection, error) {
	row, err := r.s.queries(ctx).UpsertDriveConnection(ctx, sqlcgen.UpsertDriveConnectionParams{
		ID:              c.ID,
		GroupID:         c.GroupID,
		RootFolderID:    c.RootFolderID,
		RootFolderName:  c.RootFolderName,
		DriveID:         c.DriveID,
		SyncIntervalSec: c.SyncIntervalSec,
		Writable:        c.Writable,
		CreatedBy:       c.CreatedBy,
	})
	if err != nil {
		return nil, mapErr(err, "drive connection")
	}
	return toConnection(row), nil
}

func (r *driveRepo) GetConnection(ctx context.Context, id uuid.UUID) (*domain.DriveConnection, error) {
	row, err := r.s.queries(ctx).GetDriveConnection(ctx, id)
	if err != nil {
		return nil, mapErr(err, "drive connection")
	}
	return toConnection(row), nil
}

func (r *driveRepo) GetConnectionByGroup(ctx context.Context, groupID uuid.UUID) (*domain.DriveConnection, error) {
	row, err := r.s.queries(ctx).GetDriveConnectionByGroup(ctx, groupID)
	if err != nil {
		return nil, mapErr(err, "drive connection")
	}
	return toConnection(row), nil
}

func (r *driveRepo) DeleteConnection(ctx context.Context, id uuid.UUID) error {
	return mapErr(r.s.queries(ctx).DeleteDriveConnection(ctx, id), "drive connection")
}

func (r *driveRepo) ListDue(ctx context.Context, now time.Time) ([]domain.DriveConnection, error) {
	rows, err := r.s.queries(ctx).ListDueDriveConnections(ctx, now)
	if err != nil {
		return nil, mapErr(err, "drive connection")
	}
	out := make([]domain.DriveConnection, len(rows))
	for i, row := range rows {
		out[i] = *toConnection(row)
	}
	return out, nil
}

func (r *driveRepo) BeginSync(ctx context.Context, id uuid.UUID, now, staleBefore time.Time) (bool, error) {
	n, err := r.s.queries(ctx).BeginDriveSync(ctx, sqlcgen.BeginDriveSyncParams{ID: id, Now: now, StaleBefore: staleBefore})
	return n == 1, mapErr(err, "drive connection")
}

func (r *driveRepo) FinishSync(ctx context.Context, p domain.FinishSyncParams) error {
	return mapErr(r.s.queries(ctx).FinishDriveSync(ctx, sqlcgen.FinishDriveSyncParams{
		ID:            p.ID,
		Status:        string(p.Status),
		LastError:     p.LastError,
		LastErrorCode: p.LastErrorCode,
		PageToken:     p.PageToken,
		CompletedAt:   p.CompletedAt,
		FullScan:      p.FullScan,
		Succeeded:     p.Succeeded,
	}), "drive connection")
}

func (r *driveRepo) GetItem(ctx context.Context, connectionID uuid.UUID, fileID string) (*domain.DriveItem, error) {
	row, err := r.s.queries(ctx).GetDriveItem(ctx, sqlcgen.GetDriveItemParams{ConnectionID: connectionID, DriveFileID: fileID})
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return toDriveItem(row), nil
}

func (r *driveRepo) GetItemByMaterial(ctx context.Context, materialID uuid.UUID) (*domain.DriveItem, error) {
	row, err := r.s.queries(ctx).GetDriveItemByMaterial(ctx, &materialID)
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return toDriveItem(row), nil
}

func (r *driveRepo) UpsertItem(ctx context.Context, item domain.DriveItem) (*domain.DriveItem, error) {
	state := item.State
	if state == "" {
		state = domain.DriveItemNew
	}
	row, err := r.s.queries(ctx).UpsertDriveItem(ctx, sqlcgen.UpsertDriveItemParams{
		ID:             item.ID,
		ConnectionID:   item.ConnectionID,
		DriveFileID:    item.DriveFileID,
		ParentID:       item.ParentID,
		PathCache:      item.PathCache,
		Name:           item.Name,
		Mime:           item.Mime,
		IsFolder:       item.IsFolder,
		Md5:            item.MD5,
		SizeBytes:      item.SizeBytes,
		ModifiedTime:   item.ModifiedTime,
		WebViewLink:    item.WebViewLink,
		MaterialID:     item.MaterialID,
		State:          string(state),
		Classification: item.Classification.JSON(),
		SeenAt:         item.SeenAt,
	})
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return toDriveItem(row), nil
}

func (r *driveRepo) UpdateItemState(ctx context.Context, id uuid.UUID, state domain.DriveItemState, materialID *uuid.UUID, c domain.Classification, lastError *string) error {
	return mapErr(r.s.queries(ctx).UpdateDriveItemState(ctx, sqlcgen.UpdateDriveItemStateParams{
		ID:             id,
		State:          string(state),
		MaterialID:     materialID,
		Classification: c.JSON(),
		LastError:      lastError,
	}), "drive item")
}

func (r *driveRepo) ListItems(ctx context.Context, connectionID uuid.UUID, state *domain.DriveItemState, limit, offset int32) ([]domain.DriveItem, error) {
	var st *string
	if state != nil {
		s := string(*state)
		st = &s
	}
	rows, err := r.s.queries(ctx).ListDriveItems(ctx, sqlcgen.ListDriveItemsParams{ConnectionID: connectionID, State: st, Limit: limit, Offset: offset})
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return toDriveItems(rows), nil
}

func (r *driveRepo) ListFolders(ctx context.Context, connectionID uuid.UUID) ([]domain.DriveItem, error) {
	rows, err := r.s.queries(ctx).ListDriveFolders(ctx, connectionID)
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return toDriveItems(rows), nil
}

func (r *driveRepo) ListReclassifyCandidates(ctx context.Context, groupID uuid.UUID) ([]domain.DriveItem, error) {
	rows, err := r.s.queries(ctx).ListReclassifyCandidates(ctx, groupID)
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return toDriveItems(rows), nil
}

func (r *driveRepo) ListUnseen(ctx context.Context, connectionID uuid.UUID, before time.Time) ([]domain.DriveItem, error) {
	rows, err := r.s.queries(ctx).ListUnseenDriveItems(ctx, sqlcgen.ListUnseenDriveItemsParams{ConnectionID: connectionID, SeenAt: before})
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return toDriveItems(rows), nil
}

func (r *driveRepo) DeleteItems(ctx context.Context, connectionID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).DeleteDriveItems(ctx, connectionID), "drive item")
}

func (r *driveRepo) Stats(ctx context.Context, connectionID uuid.UUID) (*domain.DriveStats, error) {
	row, err := r.s.queries(ctx).DriveItemStats(ctx, connectionID)
	if err != nil {
		return nil, mapErr(err, "drive item")
	}
	return &domain.DriveStats{
		Files: row.Files, Folders: row.Folders, Linked: row.Linked, Skipped: row.Skipped, Deleted: row.Deleted, Errors: row.Errors,
	}, nil
}

func marshalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return raw, nil
}

func parseUploadMeta(raw []byte) domain.UploadMeta {
	var m domain.UploadMeta
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func toPublisher(r sqlcgen.DrivePublisher) *domain.DrivePublisher {
	return &domain.DrivePublisher{
		GroupID: r.GroupID, Email: r.GoogleEmail, RefreshTokenEnc: r.RefreshTokenEnc, Scopes: r.Scopes,
		LastError: r.LastError, ConnectedBy: r.ConnectedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (r *driveRepo) UpsertPublisher(ctx context.Context, p domain.DrivePublisher) (*domain.DrivePublisher, error) {
	row, err := r.s.queries(ctx).UpsertDrivePublisher(ctx, sqlcgen.UpsertDrivePublisherParams{
		GroupID: p.GroupID, GoogleEmail: p.Email, RefreshTokenEnc: p.RefreshTokenEnc, Scopes: p.Scopes, ConnectedBy: p.ConnectedBy,
	})
	if err != nil {
		return nil, mapErr(err, "drive publisher")
	}
	return toPublisher(row), nil
}

func (r *driveRepo) GetPublisher(ctx context.Context, groupID uuid.UUID) (*domain.DrivePublisher, error) {
	row, err := r.s.queries(ctx).GetDrivePublisher(ctx, groupID)
	if err != nil {
		return nil, mapErr(err, "drive publisher")
	}
	return toPublisher(row), nil
}

func (r *driveRepo) DeletePublisher(ctx context.Context, groupID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).DeleteDrivePublisher(ctx, groupID), "drive publisher")
}

func (r *driveRepo) SetPublisherError(ctx context.Context, groupID uuid.UUID, msg *string) error {
	return mapErr(r.s.queries(ctx).SetDrivePublisherError(ctx, sqlcgen.SetDrivePublisherErrorParams{GroupID: groupID, LastError: msg}), "drive publisher")
}
