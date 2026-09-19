package materials

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/filenames"
)

// OpenResult tells the client how to show a file.
type OpenResult struct {
	MaterialID       uuid.UUID
	VersionID        uuid.UUID
	Mode             domain.MediaMode
	Storage          domain.StorageKind
	FileName         string
	Mime             string
	SizeBytes        int64
	DriveWebViewLink *string
	StreamURL        *string // Drive proxy through the API (Range supported)
	DownloadURL      *string // direct link to our storage (S3 presigned or signed API link)
	// PreviewURL is the PDF preview of an office file, once converted;
	// PreviewState is its state (see Service.PreviewState).
	PreviewURL   *string
	PreviewState string
	ExpiresAt    *time.Time
}

// OpenInput selects a version and presentation.
type OpenInput struct {
	VersionID *uuid.UUID
	Download  bool // force Content-Disposition: attachment
}

// Open returns links for viewing or downloading a material and counts the
// download when someone other than the uploader opens it.
func (s *Service) Open(ctx context.Context, actorID, materialID uuid.UUID, in OpenInput) (*OpenResult, error) {
	view, actor, err := s.loadView(ctx, actorID, materialID)
	if err != nil {
		return nil, err
	}
	m := view.Material
	v := view.Version
	current := v.ID
	if in.VersionID != nil && *in.VersionID != v.ID {
		other, err := s.Materials.GetVersion(ctx, *in.VersionID)
		if err != nil {
			return nil, err
		}
		if other.MaterialID != m.ID {
			return nil, domain.NotFound("material version")
		}
		v = *other
	}
	switch v.ScanStatus {
	case domain.ScanPending:
		return nil, domain.Conflict("the file is still being checked")
	case domain.ScanInfected:
		return nil, domain.Forbidden("the file is quarantined")
	}

	res := &OpenResult{
		MaterialID: m.ID, VersionID: v.ID, Mode: actor.Group.MediaMode, Storage: v.Storage,
		FileName: v.OriginalName, Mime: v.Mime, SizeBytes: v.SizeBytes, DriveWebViewLink: v.DriveWebViewLink,
	}
	now := s.Clock.Now()
	inline := !in.Download && filenames.Inline(v.Mime)
	switch v.Storage {
	case domain.StorageDrive:
		// Stage 1 serves every mode as LINK; CACHE/IMPORT copies arrive in stage 5.
		if v.ID != current {
			// The Drive link shows the file as it is now, so an older version
			// is only served from its revision, and only while Google keeps it.
			res.DriveWebViewLink = nil
			if !s.cfg.ProxyEnabled || s.Client == nil || v.DriveFileID == nil {
				return nil, versionUnavailable()
			}
			if _, err := s.revisionOf(ctx, &v, true); err != nil {
				return nil, err
			}
		}
		if s.cfg.ProxyEnabled && s.Client != nil && v.DriveFileID != nil {
			exp := now.Add(s.cfg.StreamTTL)
			token, err := s.streams.Sign(streamClaims{Material: m.ID, Version: v.ID, User: actorID, Download: in.Download}, exp)
			if err != nil {
				return nil, err
			}
			u := fmt.Sprintf("%s/materials/%s/stream?token=%s", s.cfg.APIBaseURL, m.ID, url.QueryEscape(token))
			res.StreamURL, res.ExpiresAt = &u, &exp
		}
		if v.DriveWebViewLink == nil && res.StreamURL == nil {
			return nil, domain.Unavailable("the file is on Google Drive, but no link is available")
		}
	case domain.StorageS3, domain.StorageLocal:
		if v.StorageKey == nil {
			return nil, domain.NotFound("file")
		}
		req, err := s.fileLink(ctx, *v.StorageKey, v.OriginalName, v.Mime, inline)
		if err != nil {
			return nil, err
		}
		res.DownloadURL, res.ExpiresAt = &req.URL, &req.ExpiresAt
	default:
		return nil, fmt.Errorf("unknown storage %q", v.Storage)
	}

	res.PreviewState = s.PreviewState(&v)
	if !in.Download && res.PreviewState == string(domain.PreviewReady) && v.PreviewKey != nil {
		req, err := s.fileLink(ctx, *v.PreviewKey, previewName(v.OriginalName), "application/pdf", true)
		if err != nil {
			return nil, err
		}
		res.PreviewURL = &req.URL
		if res.ExpiresAt == nil || req.ExpiresAt.Before(*res.ExpiresAt) {
			res.ExpiresAt = &req.ExpiresAt
		}
	}

	if m.UploaderID == nil || *m.UploaderID != actorID {
		if err := s.Materials.IncrementDownloads(ctx, m.ID); err != nil {
			s.Log.Warn("count download", "material", m.ID, "err", err)
		}
	}
	return res, nil
}

// fileLink is a download link for an object of our storage: presigned by S3,
// or signed by the API for the local store.
func (s *Service) fileLink(ctx context.Context, key, name, mime string, inline bool) (*domain.PresignedRequest, error) {
	req, err := s.Store.PresignGet(ctx, key, name, inline, s.cfg.PresignTTL)
	if err != nil || req != nil {
		return req, err
	}
	exp := s.Clock.Now().Add(s.cfg.PresignTTL)
	token, err := s.links.Sign(localClaims{Op: opGet, Key: key, Name: name, Mime: mime, Inline: inline}, exp)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/media/%s", s.cfg.APIBaseURL, url.PathEscape(token))
	return &domain.PresignedRequest{URL: u, ExpiresAt: exp}, nil
}

type streamClaims struct {
	Material uuid.UUID `json:"m"`
	Version  uuid.UUID `json:"v"`
	User     uuid.UUID `json:"u"`
	Download bool      `json:"d,omitempty"`
}

const (
	opGet = "get"
	opPut = "put"
)

type localClaims struct {
	Op     string `json:"o"`
	Key    string `json:"k"`
	Name   string `json:"n,omitempty"`
	Mime   string `json:"t,omitempty"`
	Inline bool   `json:"i,omitempty"`
	Max    int64  `json:"x,omitempty"`
}

// Content is a (partial) file body for the HTTP layer.
type Content struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64  // -1 when unknown
	ContentRange  string // set for partial content
	Partial       bool
	FileName      string
	Inline        bool
}

// Stream proxies a Drive file through the service account.
func (s *Service) Stream(ctx context.Context, materialID uuid.UUID, token, rangeHeader string) (*Content, error) {
	var c streamClaims
	if err := s.streams.Verify(token, s.Clock.Now(), &c); err != nil {
		return nil, err
	}
	if c.Material != materialID {
		return nil, fmt.Errorf("%w: link does not match the material", domain.ErrUnauthorized)
	}
	if s.Client == nil || !s.cfg.ProxyEnabled {
		return nil, domain.Unavailable("drive proxy is disabled")
	}
	// Membership may have ended since the link was issued.
	view, _, err := s.loadView(ctx, c.User, materialID)
	if err != nil {
		return nil, err
	}
	v, err := s.Materials.GetVersion(ctx, c.Version)
	if err != nil {
		return nil, err
	}
	if v.MaterialID != materialID || v.Storage != domain.StorageDrive || v.DriveFileID == nil {
		return nil, domain.NotFound("file")
	}
	inline := !c.Download
	if strings.HasPrefix(v.Mime, domain.GoogleAppsMimePrefix) {
		dc, err := s.Client.Export(ctx, *v.DriveFileID, "application/pdf")
		if err != nil {
			return nil, err
		}
		return &Content{
			Body: dc.Body, ContentType: "application/pdf", ContentLength: dc.ContentLength,
			FileName: v.OriginalName + ".pdf", Inline: inline,
		}, nil
	}
	if rangeHeader != "" && !strings.HasPrefix(rangeHeader, "bytes=") {
		rangeHeader = ""
	}
	var dc *domain.DriveContent
	if v.ID == view.Version.ID {
		dc, err = s.Client.Download(ctx, *v.DriveFileID, rangeHeader)
	} else {
		var revisionID string
		if revisionID, err = s.revisionOf(ctx, v, false); err == nil {
			dc, err = s.Client.DownloadRevision(ctx, *v.DriveFileID, revisionID, rangeHeader)
			if errors.Is(err, domain.ErrNotFound) {
				err = versionUnavailable()
			}
		}
	}
	if err != nil {
		return nil, err
	}
	contentType := v.Mime
	if contentType == "" {
		contentType = dc.ContentType
	}
	return &Content{
		Body: dc.Body, ContentType: contentType, ContentLength: dc.ContentLength, ContentRange: dc.ContentRange,
		Partial: dc.Partial, FileName: v.OriginalName, Inline: inline && filenames.Inline(v.Mime),
	}, nil
}

// revisionOf returns the Drive revision holding an older version's content.
// Versions indexed before revisions were recorded are matched by md5, and the
// match is remembered. verify re-checks a known revision (streams skip it: a
// player sends many range requests). A revision Google has already dropped is
// reported as CodeVersionUnavailable.
func (s *Service) revisionOf(ctx context.Context, v *domain.MaterialVersion, verify bool) (string, error) {
	known := v.DriveRevisionID
	if known != nil && !verify {
		return *known, nil
	}
	if v.DriveFileID == nil || (known == nil && v.DriveMD5 == nil) {
		return "", versionUnavailable() // native Google documents have no stored revisions
	}
	revisions, err := s.Client.ListRevisions(ctx, *v.DriveFileID)
	if err != nil {
		return "", err
	}
	for i := len(revisions) - 1; i >= 0; i-- {
		r := revisions[i]
		if known != nil {
			if r.ID == *known {
				return r.ID, nil
			}
			continue
		}
		if r.MD5 == *v.DriveMD5 {
			id := r.ID
			if err := s.Materials.SetVersionDriveRevision(ctx, v.ID, id); err != nil {
				s.Log.Warn("remember drive revision", "version", v.ID, "err", err)
			}
			v.DriveRevisionID = &id
			return id, nil
		}
	}
	return "", versionUnavailable()
}

func versionUnavailable() error {
	return domain.WithCode(domain.CodeVersionUnavailable,
		fmt.Errorf("%w: Google Drive no longer keeps this version of the file", domain.ErrGone))
}

// LocalGet serves an object of the local store by a signed link.
func (s *Service) LocalGet(ctx context.Context, token, rangeHeader string) (*Content, error) {
	var c localClaims
	if err := s.links.Verify(token, s.Clock.Now(), &c); err != nil {
		return nil, err
	}
	if c.Op != opGet {
		return nil, fmt.Errorf("%w: wrong link type", domain.ErrUnauthorized)
	}
	info, err := s.Store.Stat(ctx, c.Key)
	if err != nil {
		return nil, err
	}
	offset, length, partial, err := parseRange(rangeHeader, info.Size)
	if err != nil {
		return nil, err
	}
	body, _, err := s.Store.Open(ctx, c.Key, offset, length)
	if err != nil {
		return nil, err
	}
	out := &Content{Body: body, ContentType: c.Mime, ContentLength: length, FileName: c.Name, Inline: c.Inline, Partial: partial}
	if partial {
		out.ContentRange = fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, info.Size)
	}
	return out, nil
}

// LocalPut receives an upload for the local store by a signed link.
func (s *Service) LocalPut(ctx context.Context, token string, body io.Reader) error {
	var c localClaims
	if err := s.links.Verify(token, s.Clock.Now(), &c); err != nil {
		return err
	}
	if c.Op != opPut {
		return fmt.Errorf("%w: wrong link type", domain.ErrUnauthorized)
	}
	limited := &countingReader{r: io.LimitReader(body, c.Max+1)}
	if err := s.Store.Put(ctx, c.Key, limited, -1, c.Mime); err != nil {
		return err
	}
	if limited.n > c.Max {
		_ = s.Store.Delete(ctx, c.Key)
		return fmt.Errorf("%w: file exceeds %d bytes", domain.ErrTooLarge, c.Max)
	}
	return nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// parseRange understands a single "bytes=" range (RFC 9110 §14.1.2).
func parseRange(header string, size int64) (offset, length int64, partial bool, err error) {
	if header == "" {
		return 0, size, false, nil
	}
	spec, ok := strings.CutPrefix(header, "bytes=")
	if !ok || strings.Contains(spec, ",") {
		return 0, size, false, nil // unsupported: serve everything
	}
	from, to, ok := strings.Cut(strings.TrimSpace(spec), "-")
	if !ok {
		return 0, size, false, nil
	}
	notSatisfiable := fmt.Errorf("%w: range not satisfiable", errRange)
	if from == "" { // suffix: last N bytes
		n, perr := strconv.ParseInt(to, 10, 64)
		if perr != nil || n <= 0 {
			return 0, 0, false, notSatisfiable
		}
		n = min(n, size)
		return size - n, n, true, nil
	}
	start, perr := strconv.ParseInt(from, 10, 64)
	if perr != nil || start < 0 || start >= size {
		return 0, 0, false, notSatisfiable
	}
	end := size - 1
	if to != "" {
		e, perr := strconv.ParseInt(to, 10, 64)
		if perr != nil || e < start {
			return 0, 0, false, notSatisfiable
		}
		end = min(e, size-1)
	}
	return start, end - start + 1, true, nil
}

// errRange marks an unsatisfiable Range header (HTTP 416).
var errRange = errors.New("range")

// IsRangeError reports whether err came from an unsatisfiable Range header.
func IsRangeError(err error) bool { return errors.Is(err, errRange) }
