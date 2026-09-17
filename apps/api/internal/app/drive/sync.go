package drive

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/classify"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/ids"
)

// Keys of Drive appProperties written when the app publishes a file.
const (
	PropMaterialID = "heatseeker_material_id"
	PropGroupID    = "heatseeker_group_id"
)

// SyncResult counts what a run changed.
type SyncResult struct {
	Full    bool
	Added   int
	Updated int
	Removed int
	Skipped int
}

func (r SyncResult) changed() bool { return r.Added+r.Updated+r.Removed > 0 }

// Sync brings the index of one connection up to date. It is idempotent and
// safe to run concurrently: only one run holds a connection at a time.
func (s *Service) Sync(ctx context.Context, connID uuid.UUID, forceFull bool) (*SyncResult, error) {
	if s.Client == nil {
		return nil, domain.Unavailable("Google Drive is not configured on this server")
	}
	conn, err := s.Repo.GetConnection(ctx, connID)
	if errors.Is(err, domain.ErrNotFound) {
		return &SyncResult{}, nil // disconnected meanwhile
	}
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	ok, err := s.Repo.BeginSync(ctx, connID, now, now.Add(-s.cfg.StaleAfter))
	if err != nil {
		return nil, err
	}
	if !ok {
		s.Log.Info("drive sync already running", "connection", connID)
		return &SyncResult{}, nil
	}

	subjects, err := s.Subjects.List(ctx, conn.GroupID, false)
	if err != nil {
		return nil, s.fail(ctx, conn, false, err)
	}
	run := &syncRun{
		svc:      s,
		conn:     conn,
		subjects: classify.FromDomain(subjects),
		initial:  conn.LastFullScanAt == nil,
		folders:  map[string]*domain.DriveItem{},
	}
	full := forceFull || conn.ChangesPageToken == nil || conn.LastFullScanAt == nil ||
		now.Sub(*conn.LastFullScanAt) >= s.cfg.FullRescanInterval
	var token string
	if !full {
		var needFull bool
		token, needFull, err = run.incremental(ctx, *conn.ChangesPageToken)
		if errors.Is(err, domain.ErrInvalid) {
			// The page token is no longer accepted: rebuild from scratch.
			s.Log.Warn("drive changes token rejected, rescanning", "connection", connID, "err", err)
			needFull, err = true, nil
		}
		full = err == nil && needFull
	}
	if err == nil && full {
		token, err = run.fullScan(ctx)
	}
	run.result.Full = full
	if err != nil {
		return nil, s.fail(ctx, conn, full, err)
	}
	err = s.Repo.FinishSync(ctx, domain.FinishSyncParams{
		ID: connID, Status: domain.DriveOK, PageToken: &token, FullScan: full, Succeeded: true, CompletedAt: s.Clock.Now(),
	})
	if err != nil {
		return nil, err
	}
	res := run.result
	if res.changed() {
		err := s.emit(ctx, conn.GroupID, domain.EventDriveSynced, nil, conn.ID, map[string]any{
			"added": res.Added, "updated": res.Updated, "removed": res.Removed, "full": res.Full,
		}, true)
		if err != nil {
			return nil, err
		}
	}
	s.Log.Info("drive sync finished", "connection", connID, "full", res.Full,
		"added", res.Added, "updated", res.Updated, "removed", res.Removed, "skipped", res.Skipped)
	return &res, nil
}

func (s *Service) fail(ctx context.Context, conn *domain.DriveConnection, full bool, cause error) error {
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	err := s.Repo.FinishSync(context.WithoutCancel(ctx), domain.FinishSyncParams{
		ID: conn.ID, Status: domain.DriveError, LastError: &msg, FullScan: full, CompletedAt: s.Clock.Now(),
	})
	if err != nil {
		s.Log.Error("record drive sync failure", "connection", conn.ID, "err", err)
	}
	return fmt.Errorf("drive sync %s: %w", conn.ID, cause)
}

type syncRun struct {
	svc      *Service
	conn     *domain.DriveConnection
	subjects []classify.Subject
	initial  bool
	folders  map[string]*domain.DriveItem // drive id → folder item
	result   SyncResult
}

// fullScan walks the whole tree, then marks everything not seen as removed.
// It returns the changes token taken before the walk so that edits made
// during the scan are picked up next time.
func (r *syncRun) fullScan(ctx context.Context) (string, error) {
	s := r.svc
	token, err := s.Client.StartPageToken(ctx, r.driveID())
	if err != nil {
		return "", err
	}
	root, err := s.Client.GetFile(ctx, r.conn.RootFolderID)
	if err != nil {
		return "", fmt.Errorf("root folder: %w", err)
	}
	if root.Trashed {
		return "", domain.Invalid("folder", "the connected folder was moved to the trash")
	}
	scanStart := s.Clock.Now()
	type node struct {
		id   string
		path []string
	}
	queue := []node{{id: r.conn.RootFolderID}}
	visited := map[string]bool{r.conn.RootFolderID: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		pageToken := ""
		for {
			children, next, err := s.Client.ListChildren(ctx, cur.id, pageToken)
			if err != nil {
				return "", err
			}
			for i := range children {
				f := &children[i]
				if f.IsFolder() {
					if visited[f.ID] {
						continue
					}
					visited[f.ID] = true
					if _, err := r.upsertFolder(ctx, f, cur.path); err != nil {
						return "", err
					}
					queue = append(queue, node{id: f.ID, path: appendPath(cur.path, f.Name)})
					continue
				}
				if err := r.processFile(ctx, f, cur.path); err != nil {
					return "", err
				}
			}
			if next == "" {
				break
			}
			pageToken = next
		}
	}
	unseen, err := s.Repo.ListUnseen(ctx, r.conn.ID, scanStart)
	if err != nil {
		return "", err
	}
	for i := range unseen {
		if err := r.removed(ctx, &unseen[i]); err != nil {
			return "", err
		}
	}
	return token, nil
}

// incremental applies the changes feed. needFull asks for a full scan when a
// folder appeared, moved or was renamed (descendant paths change).
func (r *syncRun) incremental(ctx context.Context, token string) (newToken string, needFull bool, err error) {
	s := r.svc
	folders, err := s.Repo.ListFolders(ctx, r.conn.ID)
	if err != nil {
		return "", false, err
	}
	for i := range folders {
		r.folders[folders[i].DriveFileID] = &folders[i]
	}
	for {
		page, err := s.Client.ListChanges(ctx, r.driveID(), token)
		if err != nil {
			return "", false, err
		}
		for _, ch := range page.Changes {
			known, err := s.Repo.GetItem(ctx, r.conn.ID, ch.FileID)
			if errors.Is(err, domain.ErrNotFound) {
				known = nil
			} else if err != nil {
				return "", false, err
			}
			gone := ch.Removed || ch.File == nil || ch.File.Trashed
			var path []string
			inTree := false
			if !gone {
				path, inTree = r.pathOf(ch.File.Parent())
			}
			if gone || !inTree {
				if known != nil && known.State != domain.DriveItemDeleted {
					if known.IsFolder {
						needFull = true // its files vanished too
						continue
					}
					if err := r.removed(ctx, known); err != nil {
						return "", false, err
					}
				}
				continue
			}
			f := ch.File
			if f.IsFolder() {
				if known == nil || known.Name != f.Name || ptrValue(known.ParentID) != f.Parent() || known.State == domain.DriveItemDeleted {
					needFull = true
				}
				item, err := r.upsertFolder(ctx, f, path)
				if err != nil {
					return "", false, err
				}
				r.folders[f.ID] = item
				continue
			}
			if err := r.processFile(ctx, f, path); err != nil {
				return "", false, err
			}
		}
		if page.NextPageToken == "" {
			return page.NewStartPageToken, needFull, nil
		}
		token = page.NextPageToken
	}
}

// pathOf returns the folder names from the root to parentID (inclusive)
// when parentID lies inside the connected tree.
func (r *syncRun) pathOf(parentID string) ([]string, bool) {
	if parentID == r.conn.RootFolderID {
		return nil, true
	}
	folder, ok := r.folders[parentID]
	if !ok {
		return nil, false
	}
	return appendPath(folder.PathSegments(), folder.Name), true
}

func (r *syncRun) driveID() string {
	if r.conn.DriveID == nil {
		return ""
	}
	return *r.conn.DriveID
}

func (r *syncRun) upsertFolder(ctx context.Context, f *domain.DriveFile, path []string) (*domain.DriveItem, error) {
	s := r.svc
	parent := f.Parent()
	item, err := s.Repo.UpsertItem(ctx, domain.DriveItem{
		ID: ids.New(), ConnectionID: r.conn.ID, DriveFileID: f.ID, ParentID: &parent, PathCache: joinPath(path),
		Name: f.Name, Mime: f.MimeType, IsFolder: true, ModifiedTime: timePtr(f.ModifiedTime),
		WebViewLink: strPtr(f.WebViewLink), State: domain.DriveItemNew, SeenAt: s.Clock.Now(),
	})
	if err != nil {
		return nil, err
	}
	if item.State == domain.DriveItemDeleted {
		if err := s.Repo.UpdateItemState(ctx, item.ID, domain.DriveItemNew, nil, item.Classification, nil); err != nil {
			return nil, err
		}
		item.State = domain.DriveItemNew
	}
	return item, nil
}

// Native Google types that cannot be opened as documents.
var unsupportedGoogleTypes = map[string]bool{
	"application/vnd.google-apps.form":        true,
	"application/vnd.google-apps.shortcut":    true,
	"application/vnd.google-apps.map":         true,
	"application/vnd.google-apps.site":        true,
	"application/vnd.google-apps.script":      true,
	"application/vnd.google-apps.drive-sdk":   true,
	"application/vnd.google-apps.jam":         true,
	"application/vnd.google-apps.fusiontable": true,
}

func (r *syncRun) processFile(ctx context.Context, f *domain.DriveFile, path []string) error {
	s := r.svc
	known, err := s.Repo.GetItem(ctx, r.conn.ID, f.ID)
	if errors.Is(err, domain.ErrNotFound) {
		known = nil
	} else if err != nil {
		return err
	}
	parent := f.Parent()
	entry := domain.DriveItem{
		ID: ids.New(), ConnectionID: r.conn.ID, DriveFileID: f.ID, ParentID: &parent, PathCache: joinPath(path),
		Name: f.Name, Mime: f.MimeType, MD5: strPtr(f.MD5), ModifiedTime: timePtr(f.ModifiedTime),
		WebViewLink: strPtr(f.WebViewLink), State: domain.DriveItemNew, SeenAt: s.Clock.Now(),
	}
	if !f.IsGoogleDoc() {
		size := f.Size
		entry.SizeBytes = &size
	}
	if unsupportedGoogleTypes[f.MimeType] {
		entry.State = domain.DriveItemSkipped
		saved, err := s.Repo.UpsertItem(ctx, entry)
		if err != nil {
			return err
		}
		if saved.State != domain.DriveItemSkipped {
			reason := "unsupported Google type"
			return s.Repo.UpdateItemState(ctx, saved.ID, domain.DriveItemSkipped, saved.MaterialID, saved.Classification, &reason)
		}
		r.result.Skipped++
		return nil
	}

	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		saved, err := s.Repo.UpsertItem(ctx, entry)
		if err != nil {
			return err
		}
		switch {
		case saved.MaterialID != nil:
			return r.refresh(ctx, known, saved, f, path)
		case saved.State == domain.DriveItemSkipped:
			r.result.Skipped++ // deleted in the app or unsupported: leave it alone
			return nil
		}
		if linked, err := r.linkPublished(ctx, saved, f); linked || err != nil {
			return err
		}
		return r.create(ctx, saved, f, path)
	})
}

// linkPublished attaches a file the app itself uploaded (appProperties) to its
// existing material instead of creating a duplicate.
func (r *syncRun) linkPublished(ctx context.Context, item *domain.DriveItem, f *domain.DriveFile) (bool, error) {
	s := r.svc
	raw := f.AppProperties[PropMaterialID]
	if raw == "" {
		return false, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return false, nil
	}
	m, err := s.Materials.Get(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if m.GroupID != r.conn.GroupID {
		return false, nil
	}
	return true, s.Repo.UpdateItemState(ctx, item.ID, domain.DriveItemLinked, &m.ID, m.Classification, nil)
}

func (r *syncRun) classify(f *domain.DriveFile, path []string) (domain.Classification, bool) {
	c := classify.Classify(classify.Input{FileName: f.Name, Folders: path, Description: f.Description}, r.subjects)
	confident := c.SubjectID != nil && c.Confidence >= r.svc.cfg.MinConfidence
	return c, confident
}

func (r *syncRun) create(ctx context.Context, item *domain.DriveItem, f *domain.DriveFile, path []string) error {
	s := r.svc
	c, confident := r.classify(f, path)
	m := domain.Material{
		ID: ids.New(), GroupID: r.conn.GroupID, Title: classify.TitleFromFileName(f.Name),
		Description: "", Kind: c.Kind, Source: domain.SourceDrive, Status: domain.MaterialActive,
		Classification: c, SortAt: f.CreatedTime,
	}
	if m.SortAt.IsZero() {
		m.SortAt = s.Clock.Now()
	}
	if confident {
		m.SubjectID = c.SubjectID
	} else {
		reason := domain.ReviewLowConfidence
		m.NeedsReview, m.ReviewReason = true, &reason
	}
	created, err := s.Materials.Create(ctx, m)
	if err != nil {
		return err
	}
	v := driveVersion(f)
	v.ID, v.MaterialID = ids.New(), created.ID
	if _, err := s.Materials.CreateVersion(ctx, v); err != nil {
		return err
	}
	if err := s.Materials.SetCurrentVersion(ctx, created.ID, v.ID); err != nil {
		return err
	}
	if err := s.Repo.UpdateItemState(ctx, item.ID, domain.DriveItemLinked, &created.ID, c, nil); err != nil {
		return err
	}
	r.result.Added++
	// The first scan of a folder may add hundreds of files: keep them out of
	// the activity feed (drive.synced summarises them) but in the sync log.
	return s.emit(ctx, r.conn.GroupID, domain.EventMaterialAdded, nil, created.ID, map[string]any{
		"title": created.Title, "subject_id": created.SubjectID, "kind": created.Kind,
		"source": created.Source, "needs_review": created.NeedsReview,
	}, !r.initial)
}

// refresh updates a linked material after its Drive file changed.
func (r *syncRun) refresh(ctx context.Context, known, item *domain.DriveItem, f *domain.DriveFile, path []string) error {
	s := r.svc
	m, err := s.Materials.Get(ctx, *item.MaterialID)
	if err != nil {
		return err
	}
	if m.CurrentVersionID == nil {
		return nil
	}
	v, err := s.Materials.GetVersion(ctx, *m.CurrentVersionID)
	if err != nil {
		return err
	}
	if known == nil {
		known = item
	}
	wasRemoved := known.State == domain.DriveItemDeleted
	contentChanged := f.MD5 != "" && ptrValue(v.DriveMD5) != "" && ptrValue(v.DriveMD5) != f.MD5
	renamed := known.Name != f.Name
	moved := known.PathCache != joinPath(path)
	metaChanged := v.DriveFileID != nil && *v.DriveFileID == f.ID &&
		(v.OriginalName != f.Name || !timeEqual(v.DriveModifiedTime, f.ModifiedTime) || ptrValue(v.DriveWebViewLink) != f.WebViewLink)
	if wasRemoved {
		if err := s.Repo.UpdateItemState(ctx, item.ID, domain.DriveItemLinked, item.MaterialID, item.Classification, nil); err != nil {
			return err
		}
	}
	if m.Status == domain.MaterialDeleted || m.Source != domain.SourceDrive {
		// Uploads published to Drive keep their own metadata and copy.
		return nil
	}
	if !contentChanged && !renamed && !moved && !wasRemoved && !metaChanged {
		return nil
	}

	reason := "metadata"
	switch {
	case contentChanged:
		nv := driveVersion(f)
		nv.ID, nv.MaterialID = ids.New(), m.ID
		if _, err := s.Materials.CreateVersion(ctx, nv); err != nil {
			return err
		}
		if err := s.Materials.SetCurrentVersion(ctx, m.ID, nv.ID); err != nil {
			return err
		}
		reason = "new_version"
	case v.DriveFileID != nil && *v.DriveFileID == f.ID:
		fresh := driveVersion(f)
		fresh.ID = v.ID
		if err := s.Materials.UpdateVersionFile(ctx, fresh); err != nil {
			return err
		}
	}

	p := domain.UpdateMaterialParams{
		ID: m.ID, Title: m.Title, Description: m.Description, SubjectID: m.SubjectID, Kind: m.Kind,
		Classification: m.Classification, NeedsReview: m.NeedsReview, ReviewReason: m.ReviewReason,
	}
	if renamed && m.Title == classify.TitleFromFileName(known.Name) {
		p.Title = classify.TitleFromFileName(f.Name)
		reason = "renamed"
	}
	if (renamed || moved) && m.Classification.Method == domain.ClassifyAuto {
		c, confident := r.classify(f, path)
		p.Classification, p.Kind = c, c.Kind
		if confident {
			p.SubjectID = c.SubjectID
			if p.ReviewReason != nil && *p.ReviewReason == domain.ReviewLowConfidence {
				p.NeedsReview, p.ReviewReason = false, nil
			}
		} else if !p.NeedsReview {
			low := domain.ReviewLowConfidence
			p.SubjectID, p.NeedsReview, p.ReviewReason = nil, true, &low
		}
		if moved {
			reason = "moved"
		}
		if err := s.Repo.UpdateItemState(ctx, item.ID, domain.DriveItemLinked, item.MaterialID, c, nil); err != nil {
			return err
		}
	}
	if wasRemoved {
		reason = "restored_on_drive"
		if p.ReviewReason != nil && *p.ReviewReason == domain.ReviewRemovedFromDrive {
			p.NeedsReview, p.ReviewReason = false, nil
			if p.SubjectID == nil && m.Classification.Method == domain.ClassifyAuto {
				low := domain.ReviewLowConfidence
				p.NeedsReview, p.ReviewReason = true, &low
			}
		}
		if m.Status == domain.MaterialArchived && m.ArchivedBy == nil {
			// Archived automatically by GDRIVE_DELETE_POLICY=archive.
			if _, err := s.Materials.SetStatus(ctx, m.ID, domain.MaterialActive, nil); err != nil {
				return err
			}
		}
	}
	if _, err := s.Materials.Update(ctx, p); err != nil {
		return err
	}
	r.result.Updated++
	return s.emit(ctx, r.conn.GroupID, domain.EventMaterialUpdated, nil, m.ID, map[string]any{
		"title": p.Title, "subject_id": p.SubjectID, "kind": p.Kind, "reason": reason,
	}, reason != "metadata")
}

// removed handles a file that disappeared from the folder (deleted, trashed
// or moved out). Materials are never deleted automatically.
func (r *syncRun) removed(ctx context.Context, item *domain.DriveItem) error {
	s := r.svc
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.UpdateItemState(ctx, item.ID, domain.DriveItemDeleted, item.MaterialID, item.Classification, nil); err != nil {
			return err
		}
		if item.IsFolder || item.MaterialID == nil {
			return nil
		}
		m, err := s.Materials.Get(ctx, *item.MaterialID)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if m.Status != domain.MaterialActive || m.Source != domain.SourceDrive {
			// Uploads published to Drive keep their own copy.
			return nil
		}
		r.result.Removed++
		payload := map[string]any{"title": m.Title, "reason": "removed_from_drive"}
		if s.cfg.DeletePolicy == "archive" {
			if _, err := s.Materials.SetStatus(ctx, m.ID, domain.MaterialArchived, nil); err != nil {
				return err
			}
			return s.emit(ctx, r.conn.GroupID, domain.EventMaterialArchived, nil, m.ID, payload, true)
		}
		reason := domain.ReviewRemovedFromDrive
		_, err = s.Materials.Update(ctx, domain.UpdateMaterialParams{
			ID: m.ID, Title: m.Title, Description: m.Description, SubjectID: m.SubjectID, Kind: m.Kind,
			Classification: m.Classification, NeedsReview: true, ReviewReason: &reason,
		})
		if err != nil {
			return err
		}
		return s.emit(ctx, r.conn.GroupID, domain.EventMaterialUpdated, nil, m.ID, payload, true)
	})
}

func driveVersion(f *domain.DriveFile) domain.MaterialVersion {
	v := domain.MaterialVersion{
		Storage: domain.StorageDrive, DriveFileID: &f.ID, DriveWebViewLink: strPtr(f.WebViewLink),
		DriveMD5: strPtr(f.MD5), DriveModifiedTime: timePtr(f.ModifiedTime),
		OriginalName: f.Name, Mime: f.MimeType, ScanStatus: domain.ScanSkipped,
	}
	if !f.IsGoogleDoc() {
		v.SizeBytes = f.Size
	}
	return v
}

// Path segments are joined with "/", so slashes inside names are replaced.
func appendPath(path []string, name string) []string {
	out := make([]string, 0, len(path)+1)
	out = append(out, path...)
	return append(out, strings.ReplaceAll(name, "/", "∕"))
}

func joinPath(path []string) string { return strings.Join(path, "/") }

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	t = t.UTC()
	return &t
}

func timeEqual(a *time.Time, b time.Time) bool {
	if a == nil {
		return b.IsZero()
	}
	return a.Equal(b)
}
