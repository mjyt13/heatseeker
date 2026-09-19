package domain

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrDriveQuota is returned when Drive refuses to store a file because the
// owner (the service account) has no storage quota left or none at all.
var ErrDriveQuota = errors.New("drive storage quota exceeded")

// FolderMime is the MIME type of Drive folders.
const FolderMime = "application/vnd.google-apps.folder"

// GoogleAppsMimePrefix marks native Google documents that have no binary
// content and must be exported to be downloaded.
const GoogleAppsMimePrefix = "application/vnd.google-apps."

// DriveFile is the subset of Drive file metadata the core uses.
type DriveFile struct {
	ID            string
	Name          string
	MimeType      string
	MD5           string
	Size          int64
	CreatedTime   time.Time
	ModifiedTime  time.Time
	Parents       []string
	Description   string
	WebViewLink   string
	Trashed       bool
	DriveID       string
	CanAddChild   bool
	AppProperties map[string]string
	// HeadRevisionID is the current content revision (binary files only).
	HeadRevisionID string
}

// DriveRevision is a stored revision of a binary Drive file.
type DriveRevision struct {
	ID           string
	MD5          string
	Size         int64
	ModifiedTime time.Time
}

// IsFolder reports whether the file is a folder.
func (f *DriveFile) IsFolder() bool { return f.MimeType == FolderMime }

// IsGoogleDoc reports whether the file is a native Google document
// (Docs/Sheets/Slides/…), which has no md5 and is downloaded via export.
func (f *DriveFile) IsGoogleDoc() bool {
	return strings.HasPrefix(f.MimeType, GoogleAppsMimePrefix) && !f.IsFolder()
}

// Parent returns the first parent id (Drive files have exactly one since 2020).
func (f *DriveFile) Parent() string {
	if len(f.Parents) == 0 {
		return ""
	}
	return f.Parents[0]
}

// DriveChange is one entry of the Drive changes feed.
type DriveChange struct {
	FileID  string
	Removed bool
	File    *DriveFile
}

// DriveChangesPage is a page of the changes feed. Exactly one of the tokens is
// set: NextPageToken while more pages follow, NewStartPageToken at the end.
type DriveChangesPage struct {
	Changes           []DriveChange
	NextPageToken     string
	NewStartPageToken string
}

// DriveUpload describes a file to create on Drive.
type DriveUpload struct {
	Name          string
	MimeType      string
	ParentID      string
	Description   string
	AppProperties map[string]string
	Body          io.Reader
}

// DriveContent is a (possibly partial) download.
type DriveContent struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64  // -1 when unknown
	ContentRange  string // set for 206 responses
	Partial       bool
}

// DriveClient is the Drive API as seen by the core (Adapter). A nil client
// means Drive is not configured.
type DriveClient interface {
	// ServiceAccountEmail is the address folder owners share their folder with.
	ServiceAccountEmail() string
	GetFile(ctx context.Context, fileID string) (*DriveFile, error)
	// ListChildren returns one page of non-trashed children of a folder.
	ListChildren(ctx context.Context, folderID, pageToken string) (files []DriveFile, next string, err error)
	StartPageToken(ctx context.Context, driveID string) (string, error)
	ListChanges(ctx context.Context, driveID, pageToken string) (*DriveChangesPage, error)
	// Download streams binary content; rangeHeader is passed through ("" for all).
	Download(ctx context.Context, fileID, rangeHeader string) (*DriveContent, error)
	// Export converts a native Google document (to PDF by default).
	Export(ctx context.Context, fileID, mime string) (*DriveContent, error)
	// ListRevisions returns the revisions Google still keeps for a binary file.
	ListRevisions(ctx context.Context, fileID string) ([]DriveRevision, error)
	// DownloadRevision streams the content of one revision (Range passed through).
	DownloadRevision(ctx context.Context, fileID, revisionID, rangeHeader string) (*DriveContent, error)
	Upload(ctx context.Context, u DriveUpload) (*DriveFile, error)
}

// DrivePublisher is the Google account that publishes uploads to the group's
// Drive folder (D34). The refresh token is sealed; only the Drive service
// opens it.
type DrivePublisher struct {
	GroupID         uuid.UUID
	Email           string
	RefreshTokenEnc []byte
	Scopes          string
	LastError       *string
	ConnectedBy     *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Usable reports whether uploads can go through the account: Google has not
// rejected its token.
func (p *DrivePublisher) Usable() bool { return p != nil && p.LastError == nil }

// CanPublishToDrive reports whether uploads can be copied to the connected
// folder: by the publishing account, or by the service account inside a
// shared drive (it has no storage quota in "My Drive"). pub is nil when no
// account is connected or the server cannot use one.
func CanPublishToDrive(conn *DriveConnection, pub *DrivePublisher) bool {
	if conn == nil {
		return false
	}
	return pub.Usable() || (conn.Writable && conn.DriveID != nil)
}

// DriveGrant is the result of a completed Google sign-in for publishing.
type DriveGrant struct {
	RefreshToken string
	Email        string
	Scopes       string
}

// DriveAuthorizer runs the OAuth flow of the publishing account and builds a
// Drive client acting on its behalf.
type DriveAuthorizer interface {
	// AuthURL is the Google consent page; state comes back to the callback.
	AuthURL(state string) string
	// Exchange trades the callback's code for a refresh token. It fails with
	// CodeDriveScopeMissing when the user did not grant Drive access.
	Exchange(ctx context.Context, code string) (*DriveGrant, error)
	// Client acts as the account; a revoked token surfaces as
	// CodeDrivePublisherRevoked on the first call.
	Client(ctx context.Context, refreshToken string) (DriveClient, error)
	// Revoke invalidates the refresh token at Google.
	Revoke(ctx context.Context, refreshToken string) error
}

// DriveConnectionStatus is the sync state of a connection.
type DriveConnectionStatus string

// Connection statuses.
const (
	DrivePending DriveConnectionStatus = "PENDING"
	DriveSyncing DriveConnectionStatus = "SYNCING"
	DriveOK      DriveConnectionStatus = "OK"
	DriveError   DriveConnectionStatus = "ERROR"
)

// DriveConnection links a group to a shared Drive folder.
type DriveConnection struct {
	ID               uuid.UUID
	GroupID          uuid.UUID
	RootFolderID     string
	RootFolderName   string
	DriveID          *string
	Status           DriveConnectionStatus
	LastError        *string
	LastErrorCode    *string
	SyncStartedAt    *time.Time
	LastSyncAt       *time.Time
	LastFullScanAt   *time.Time
	ChangesPageToken *string
	SyncIntervalSec  int32
	Writable         bool
	CreatedBy        *uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// DriveItemState is what the indexer did with a Drive file.
type DriveItemState string

// Drive item states.
const (
	DriveItemNew      DriveItemState = "NEW"
	DriveItemLinked   DriveItemState = "LINKED"
	DriveItemImported DriveItemState = "IMPORTED"
	DriveItemSkipped  DriveItemState = "SKIPPED"
	DriveItemError    DriveItemState = "ERROR"
	DriveItemDeleted  DriveItemState = "DELETED"
)

// DriveItem is an indexed file or folder under a connection's root.
type DriveItem struct {
	ID             uuid.UUID
	ConnectionID   uuid.UUID
	DriveFileID    string
	ParentID       *string
	PathCache      string
	Name           string
	Mime           string
	IsFolder       bool
	MD5            *string
	SizeBytes      *int64
	ModifiedTime   *time.Time
	WebViewLink    *string
	MaterialID     *uuid.UUID
	State          DriveItemState
	Classification Classification
	LastError      *string
	SeenAt         time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PathSegments splits PathCache into folder names.
func (i *DriveItem) PathSegments() []string {
	if i.PathCache == "" {
		return nil
	}
	return strings.Split(i.PathCache, "/")
}

// DriveStats summarises a connection's index.
type DriveStats struct {
	Files     int64
	Folders   int64
	Linked    int64
	Skipped   int64
	Deleted   int64
	Errors    int64
	InboxSize int64
}
