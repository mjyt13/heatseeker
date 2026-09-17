package gdrive

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // mirrors Drive's md5Checksum, not used for security
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"heatseeker/api/internal/domain"
)

// Fake is an in-memory Drive for tests and local development without a
// service account. It records a changes feed like the real API.
type Fake struct {
	mu      sync.Mutex
	files   map[string]*fakeFile
	changes []domain.DriveChange
	nextID  int
	now     time.Time
	// QuotaExceeded makes Upload fail like a service account without storage.
	QuotaExceeded bool
}

type fakeFile struct {
	meta    domain.DriveFile
	content []byte
}

// NewFake returns an empty fake Drive.
func NewFake() *Fake {
	return &Fake{files: map[string]*fakeFile{}, now: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)}
}

// ServiceAccountEmail implements domain.DriveClient.
func (f *Fake) ServiceAccountEmail() string { return "heatseeker@test.iam.gserviceaccount.com" }

func (f *Fake) tick() time.Time {
	f.now = f.now.Add(time.Minute)
	return f.now
}

func (f *Fake) newID() string {
	f.nextID++
	// Real ids are long; the API rejects short folder references.
	return fmt.Sprintf("fakeDriveId%06d", f.nextID)
}

func (f *Fake) record(id string, removed bool) {
	f.changes = append(f.changes, domain.DriveChange{FileID: id, Removed: removed})
}

// AddFolder creates a folder and returns its id. parent "" makes a root.
func (f *Fake) AddFolder(parent, name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	now := f.tick()
	meta := domain.DriveFile{ID: id, Name: name, MimeType: domain.FolderMime, CreatedTime: now, ModifiedTime: now, CanAddChild: true,
		WebViewLink: "https://drive.google.com/drive/folders/" + id}
	if parent != "" {
		meta.Parents = []string{parent}
	}
	f.files[id] = &fakeFile{meta: meta}
	f.record(id, false)
	return id
}

// AddFile creates a binary file with content and returns its id.
func (f *Fake) AddFile(parent, name, mime, content string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.put(id, parent, name, mime, []byte(content), nil)
	return id
}

// AddGoogleDoc creates a native Google document.
func (f *Fake) AddGoogleDoc(parent, name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	now := f.tick()
	f.files[id] = &fakeFile{meta: domain.DriveFile{
		ID: id, Name: name, MimeType: "application/vnd.google-apps.document", Parents: []string{parent},
		CreatedTime: now, ModifiedTime: now, WebViewLink: "https://docs.google.com/document/d/" + id,
	}, content: []byte("%PDF-export of " + name)}
	f.record(id, false)
	return id
}

func (f *Fake) put(id, parent, name, mime string, content []byte, props map[string]string) *domain.DriveFile {
	now := f.tick()
	sum := md5.Sum(content) //nolint:gosec // see import
	created := now
	if old, ok := f.files[id]; ok {
		created = old.meta.CreatedTime
	}
	f.files[id] = &fakeFile{meta: domain.DriveFile{
		ID: id, Name: name, MimeType: mime, MD5: hex.EncodeToString(sum[:]), Size: int64(len(content)),
		Parents: []string{parent}, CreatedTime: created, ModifiedTime: now, AppProperties: props,
		WebViewLink: "https://drive.google.com/file/d/" + id + "/view",
	}, content: content}
	f.record(id, false)
	meta := f.files[id].meta
	return &meta
}

// UpdateContent replaces a file's content (new md5).
func (f *Fake) UpdateContent(id, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file := f.files[id]
	f.put(id, file.meta.Parent(), file.meta.Name, file.meta.MimeType, []byte(content), file.meta.AppProperties)
}

// Rename changes a file's name.
func (f *Fake) Rename(id, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[id].meta.Name = name
	f.files[id].meta.ModifiedTime = f.tick()
	f.record(id, false)
}

// Move reparents a file.
func (f *Fake) Move(id, parent string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[id].meta.Parents = []string{parent}
	f.record(id, false)
}

// Trash marks a file as trashed.
func (f *Fake) Trash(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[id].meta.Trashed = true
	f.record(id, false)
}

// Delete removes a file permanently.
func (f *Fake) Delete(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.files, id)
	f.record(id, true)
}

// Content returns the stored bytes of a file.
func (f *Fake) Content(id string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[id]
	if !ok {
		return "", false
	}
	return string(file.content), true
}

// FindByName returns the id of the first live file with that name.
func (f *Fake) FindByName(name string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.files))
	for id := range f.files {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if file := f.files[id]; file.meta.Name == name && !file.meta.Trashed {
			return id, true
		}
	}
	return "", false
}

// GetFile implements domain.DriveClient.
func (f *Fake) GetFile(_ context.Context, fileID string) (*domain.DriveFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: drive file", domain.ErrNotFound)
	}
	meta := file.meta
	return &meta, nil
}

// ListChildren implements domain.DriveClient (pages of two to exercise paging).
func (f *Fake) ListChildren(_ context.Context, folderID, pageToken string) ([]domain.DriveFile, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []domain.DriveFile
	for _, file := range f.files {
		if file.meta.Parent() == folderID && !file.meta.Trashed {
			all = append(all, file.meta)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	start, _ := strconv.Atoi(pageToken)
	end := min(start+2, len(all))
	next := ""
	if end < len(all) {
		next = strconv.Itoa(end)
	}
	return all[start:end], next, nil
}

// StartPageToken implements domain.DriveClient.
func (f *Fake) StartPageToken(context.Context, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strconv.Itoa(len(f.changes)), nil
}

// ListChanges implements domain.DriveClient (pages of three).
func (f *Fake) ListChanges(_ context.Context, _ string, pageToken string) (*domain.DriveChangesPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	start, err := strconv.Atoi(pageToken)
	if err != nil || start > len(f.changes) {
		return nil, fmt.Errorf("%w: bad page token", domain.ErrInvalid)
	}
	end := min(start+3, len(f.changes))
	page := &domain.DriveChangesPage{}
	for _, ch := range f.changes[start:end] {
		// Like the real API, a change carries the file's current state.
		if file, ok := f.files[ch.FileID]; ok && !ch.Removed {
			meta := file.meta
			ch.File = &meta
		} else {
			ch.Removed = true
		}
		page.Changes = append(page.Changes, ch)
	}
	if end < len(f.changes) {
		page.NextPageToken = strconv.Itoa(end)
	} else {
		page.NewStartPageToken = strconv.Itoa(end)
	}
	return page, nil
}

// Download implements domain.DriveClient. Supports "bytes=a-b" and "bytes=a-".
func (f *Fake) Download(_ context.Context, fileID, rangeHeader string) (*domain.DriveContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: drive file", domain.ErrNotFound)
	}
	if file.meta.IsGoogleDoc() {
		return nil, fmt.Errorf("%w: only files with binary content can be downloaded", domain.ErrForbidden)
	}
	data := file.content
	out := &domain.DriveContent{ContentType: file.meta.MimeType, ContentLength: int64(len(data))}
	if spec, ok := strings.CutPrefix(rangeHeader, "bytes="); ok {
		from, to, _ := strings.Cut(spec, "-")
		a, _ := strconv.Atoi(from)
		b := len(data) - 1
		if to != "" {
			b, _ = strconv.Atoi(to)
		}
		b = min(b, len(data)-1)
		if a > b {
			return nil, fmt.Errorf("%w: range not satisfiable", domain.ErrInvalid)
		}
		out.ContentRange = fmt.Sprintf("bytes %d-%d/%d", a, b, len(data))
		out.Partial = true
		data = data[a : b+1]
		out.ContentLength = int64(len(data))
	}
	out.Body = io.NopCloser(bytes.NewReader(data))
	return out, nil
}

// Export implements domain.DriveClient.
func (f *Fake) Export(_ context.Context, fileID, mime string) (*domain.DriveContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[fileID]
	if !ok {
		return nil, fmt.Errorf("%w: drive file", domain.ErrNotFound)
	}
	if mime == "" {
		mime = "application/pdf"
	}
	return &domain.DriveContent{Body: io.NopCloser(bytes.NewReader(file.content)), ContentType: mime, ContentLength: -1}, nil
}

// Upload implements domain.DriveClient.
func (f *Fake) Upload(_ context.Context, u domain.DriveUpload) (*domain.DriveFile, error) {
	content, err := io.ReadAll(u.Body)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.QuotaExceeded {
		return nil, fmt.Errorf("%w: Service Accounts do not have storage quota", domain.ErrDriveQuota)
	}
	parent, ok := f.files[u.ParentID]
	if !ok || !parent.meta.IsFolder() {
		return nil, fmt.Errorf("%w: parent folder", domain.ErrNotFound)
	}
	meta := f.put(f.newID(), u.ParentID, u.Name, u.MimeType, content, u.AppProperties)
	f.files[meta.ID].meta.Description = u.Description
	meta.Description = u.Description
	return meta, nil
}

var _ domain.DriveClient = (*Fake)(nil)
var _ domain.DriveClient = (*Client)(nil)
