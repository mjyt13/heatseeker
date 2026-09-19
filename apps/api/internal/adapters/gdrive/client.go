// Package gdrive adapts the Google Drive v3 API to domain.DriveClient using a
// service account (docs/PLAN.md §6). Folder owners share their folder with the
// service account's e-mail; no end-user OAuth is involved.
package gdrive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"heatseeker/api/internal/domain"
)

const fileFields = "id,name,mimeType,md5Checksum,size,createdTime,modifiedTime,parents,description,webViewLink,trashed,driveId,appProperties,headRevisionId,capabilities/canAddChildren"

// Client implements domain.DriveClient.
type Client struct {
	svc   *drive.Service
	email string
}

// New builds a client from a base64-encoded service account JSON key.
func New(ctx context.Context, keyBase64 string) (*Client, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(keyBase64))
	if err != nil {
		return nil, fmt.Errorf("GDRIVE_SERVICE_ACCOUNT_JSON_BASE64: not valid base64: %w", err)
	}
	var key struct {
		Type        string `json:"type"`
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(raw, &key); err != nil {
		return nil, fmt.Errorf("GDRIVE_SERVICE_ACCOUNT_JSON_BASE64: not a JSON key: %w", err)
	}
	if key.Type != "service_account" || key.ClientEmail == "" {
		return nil, errors.New("GDRIVE_SERVICE_ACCOUNT_JSON_BASE64: expected a service_account key with client_email")
	}
	svc, err := drive.NewService(ctx,
		option.WithAuthCredentialsJSON(option.ServiceAccount, raw),
		option.WithScopes(drive.DriveScope),
	)
	if err != nil {
		return nil, fmt.Errorf("drive client: %w", err)
	}
	return &Client{svc: svc, email: key.ClientEmail}, nil
}

// Ping checks that the key works and the Drive API is enabled in its project.
func (c *Client) Ping(ctx context.Context) error {
	if _, err := c.svc.About.Get().Fields("user(emailAddress)").Context(ctx).Do(); err != nil {
		return mapErr(err, "drive about")
	}
	return nil
}

// ServiceAccountEmail implements domain.DriveClient.
func (c *Client) ServiceAccountEmail() string { return c.email }

// GetFile implements domain.DriveClient.
func (c *Client) GetFile(ctx context.Context, fileID string) (*domain.DriveFile, error) {
	f, err := c.svc.Files.Get(fileID).SupportsAllDrives(true).Fields(googleapi.Field(fileFields)).Context(ctx).Do()
	if err != nil {
		return nil, mapErr(err, "drive file")
	}
	return toFile(f), nil
}

// ListChildren implements domain.DriveClient.
func (c *Client) ListChildren(ctx context.Context, folderID, pageToken string) ([]domain.DriveFile, string, error) {
	call := c.svc.Files.List().
		Q(fmt.Sprintf("'%s' in parents and trashed = false", escapeQuery(folderID))).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Corpora("allDrives").
		PageSize(1000).
		Fields(googleapi.Field("nextPageToken,files(" + fileFields + ")")).
		Context(ctx)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	res, err := call.Do()
	if err != nil {
		return nil, "", mapErr(err, "drive folder")
	}
	out := make([]domain.DriveFile, 0, len(res.Files))
	for _, f := range res.Files {
		out = append(out, *toFile(f))
	}
	return out, res.NextPageToken, nil
}

// StartPageToken implements domain.DriveClient.
func (c *Client) StartPageToken(ctx context.Context, driveID string) (string, error) {
	call := c.svc.Changes.GetStartPageToken().SupportsAllDrives(true).Context(ctx)
	if driveID != "" {
		call = call.DriveId(driveID)
	}
	res, err := call.Do()
	if err != nil {
		return "", mapErr(err, "drive changes")
	}
	return res.StartPageToken, nil
}

// ListChanges implements domain.DriveClient.
func (c *Client) ListChanges(ctx context.Context, driveID, pageToken string) (*domain.DriveChangesPage, error) {
	call := c.svc.Changes.List(pageToken).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		IncludeRemoved(true).
		PageSize(1000).
		Fields(googleapi.Field("nextPageToken,newStartPageToken,changes(fileId,removed,file(" + fileFields + "))")).
		Context(ctx)
	if driveID != "" {
		call = call.DriveId(driveID)
	}
	res, err := call.Do()
	if err != nil {
		return nil, mapErr(err, "drive changes")
	}
	page := &domain.DriveChangesPage{NextPageToken: res.NextPageToken, NewStartPageToken: res.NewStartPageToken}
	for _, ch := range res.Changes {
		change := domain.DriveChange{FileID: ch.FileId, Removed: ch.Removed}
		if ch.File != nil {
			change.File = toFile(ch.File)
		}
		page.Changes = append(page.Changes, change)
	}
	return page, nil
}

// Download implements domain.DriveClient.
func (c *Client) Download(ctx context.Context, fileID, rangeHeader string) (*domain.DriveContent, error) {
	call := c.svc.Files.Get(fileID).SupportsAllDrives(true).AcknowledgeAbuse(false).Context(ctx)
	if rangeHeader != "" {
		call.Header().Set("Range", rangeHeader)
	}
	res, err := call.Download()
	if err != nil {
		return nil, mapErr(err, "drive file")
	}
	return toContent(res), nil
}

// Export implements domain.DriveClient.
func (c *Client) Export(ctx context.Context, fileID, mime string) (*domain.DriveContent, error) {
	if mime == "" {
		mime = "application/pdf"
	}
	res, err := c.svc.Files.Export(fileID, mime).Context(ctx).Download()
	if err != nil {
		return nil, mapErr(err, "drive file")
	}
	return toContent(res), nil
}

// ListRevisions implements domain.DriveClient.
func (c *Client) ListRevisions(ctx context.Context, fileID string) ([]domain.DriveRevision, error) {
	var out []domain.DriveRevision
	pageToken := ""
	for {
		call := c.svc.Revisions.List(fileID).
			PageSize(1000).
			Fields("nextPageToken,revisions(id,md5Checksum,size,modifiedTime)").
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return nil, mapErr(err, "drive revisions")
		}
		for _, r := range res.Revisions {
			rev := domain.DriveRevision{ID: r.Id, MD5: r.Md5Checksum, Size: r.Size}
			rev.ModifiedTime, _ = time.Parse(time.RFC3339, r.ModifiedTime)
			out = append(out, rev)
		}
		if res.NextPageToken == "" {
			return out, nil
		}
		pageToken = res.NextPageToken
	}
}

// DownloadRevision implements domain.DriveClient.
func (c *Client) DownloadRevision(ctx context.Context, fileID, revisionID, rangeHeader string) (*domain.DriveContent, error) {
	call := c.svc.Revisions.Get(fileID, revisionID).AcknowledgeAbuse(false).Context(ctx)
	if rangeHeader != "" {
		call.Header().Set("Range", rangeHeader)
	}
	res, err := call.Download()
	if err != nil {
		return nil, mapErr(err, "drive revision")
	}
	return toContent(res), nil
}

// Upload implements domain.DriveClient.
func (c *Client) Upload(ctx context.Context, u domain.DriveUpload) (*domain.DriveFile, error) {
	meta := &drive.File{
		Name:          u.Name,
		MimeType:      u.MimeType,
		Parents:       []string{u.ParentID},
		Description:   u.Description,
		AppProperties: u.AppProperties,
	}
	f, err := c.svc.Files.Create(meta).
		SupportsAllDrives(true).
		Media(u.Body, googleapi.ContentType(u.MimeType)).
		Fields(googleapi.Field(fileFields)).
		Context(ctx).
		Do()
	if err != nil {
		return nil, mapErr(err, "drive upload")
	}
	return toFile(f), nil
}

func toFile(f *drive.File) *domain.DriveFile {
	out := &domain.DriveFile{
		ID:             f.Id,
		Name:           f.Name,
		MimeType:       f.MimeType,
		MD5:            f.Md5Checksum,
		Size:           f.Size,
		Parents:        f.Parents,
		Description:    f.Description,
		WebViewLink:    f.WebViewLink,
		Trashed:        f.Trashed,
		DriveID:        f.DriveId,
		AppProperties:  f.AppProperties,
		HeadRevisionID: f.HeadRevisionId,
	}
	out.CreatedTime, _ = time.Parse(time.RFC3339, f.CreatedTime)
	out.ModifiedTime, _ = time.Parse(time.RFC3339, f.ModifiedTime)
	if f.Capabilities != nil {
		out.CanAddChild = f.Capabilities.CanAddChildren
	}
	return out
}

func toContent(res *http.Response) *domain.DriveContent {
	return &domain.DriveContent{
		Body:          res.Body,
		ContentType:   res.Header.Get("Content-Type"),
		ContentLength: res.ContentLength,
		ContentRange:  res.Header.Get("Content-Range"),
		Partial:       res.StatusCode == http.StatusPartialContent,
	}
}

func escapeQuery(s string) string {
	return strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s)
}

// apiDisabled reports Google's "API not enabled for this project" answer
// (403 accessNotConfigured, ErrorInfo reason SERVICE_DISABLED).
func apiDisabled(gerr *googleapi.Error) bool {
	if gerr.Code != http.StatusForbidden {
		return false
	}
	for _, item := range gerr.Errors {
		if item.Reason == "accessNotConfigured" {
			return true
		}
	}
	for _, d := range gerr.Details {
		if m, ok := d.(map[string]any); ok && m["reason"] == "SERVICE_DISABLED" {
			return true
		}
	}
	return false
}

// mapErr converts Google API errors into domain errors.
func mapErr(err error, entity string) error {
	// A client acting as the publishing account refreshes its token first.
	if terr := asTokenErr(err, false); terr != nil {
		return terr
	}
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return fmt.Errorf("%s: %w", entity, err)
	}
	for _, item := range gerr.Errors {
		if item.Reason == "storageQuotaExceeded" {
			return fmt.Errorf("%w: %s", domain.ErrDriveQuota, gerr.Message)
		}
	}
	if apiDisabled(gerr) {
		// A project-level problem, not a missing share: say so explicitly.
		return domain.WithCode(domain.CodeDriveAPIDisabled,
			fmt.Errorf("%w: Google Drive API is disabled in the service account's Cloud project: %s", domain.ErrUnavailable, gerr.Message))
	}
	switch gerr.Code {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s (not shared with the service account?)", domain.ErrNotFound, entity)
	case http.StatusUnauthorized:
		return domain.WithCode(domain.CodeDriveAuthFailed,
			fmt.Errorf("%w: service account rejected: %s", domain.ErrUnavailable, gerr.Message))
	case http.StatusForbidden:
		for _, item := range gerr.Errors {
			if strings.Contains(item.Reason, "RateLimit") || item.Reason == "userRateLimitExceeded" {
				return fmt.Errorf("%w: %s", domain.ErrRateLimited, gerr.Message)
			}
		}
		return fmt.Errorf("%w: %s: %s", domain.ErrForbidden, entity, gerr.Message)
	case http.StatusRequestedRangeNotSatisfiable:
		return fmt.Errorf("%w: range not satisfiable", domain.ErrInvalid)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", domain.ErrRateLimited, gerr.Message)
	}
	return fmt.Errorf("%s: %w", entity, err)
}
