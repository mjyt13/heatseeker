package domain

import (
	"context"
	"io"
	"time"
)

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ModTime     time.Time
}

// PresignedRequest is what a client needs to send or fetch an object
// directly, bypassing the API process.
type PresignedRequest struct {
	Method    string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

// MediaStore stores material bytes (Strategy: s3 | local). Use cases never talk
// to S3 or the filesystem directly.
//
// PresignPut and PresignGet return (nil, nil) when the store cannot hand out
// direct URLs (local filesystem); callers then route bytes through the API.
type MediaStore interface {
	Kind() StorageKind
	// PresignPut lets a client upload key directly.
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (*PresignedRequest, error)
	// PresignGet lets a client download key directly. fileName and inline
	// control Content-Disposition.
	PresignGet(ctx context.Context, key, fileName string, inline bool, ttl time.Duration) (*PresignedRequest, error)
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Open reads length bytes from offset (length < 0 — until the end).
	Open(ctx context.Context, key string, offset, length int64) (io.ReadCloser, *ObjectInfo, error)
	Stat(ctx context.Context, key string) (*ObjectInfo, error)
	Move(ctx context.Context, src, dst string) error
	Delete(ctx context.Context, key string) error
}

// Job is a background task handed to the queue (Command). Payload is encoded
// as JSON.
type Job struct {
	Type      string
	Payload   any
	Queue     string
	UniqueFor time.Duration // drop duplicates enqueued within this window
	MaxRetry  int           // 0 — library default
	Delay     time.Duration
}

// JobQueue enqueues background work.
type JobQueue interface {
	Enqueue(ctx context.Context, job Job) error
}

// Background job types. Handlers live in package jobs.
const (
	JobAuthCleanupRefreshTokens = "auth:cleanup_refresh_tokens"
	JobEventsPrune              = "events:prune"
	JobDriveSyncDue             = "drive:sync_due"
	JobDriveSync                = "drive:sync"
	JobDriveUpload              = "drive:upload"
	JobUploadsCleanup           = "uploads:cleanup"
	JobMaterialsPurge           = "materials:purge"
	JobMaterialHash             = "materials:hash"
	JobMaterialPreview          = "materials:preview"
	JobMaterialsReclassify      = "materials:reclassify"
)

// DriveSyncPayload is the payload of JobDriveSync.
type DriveSyncPayload struct {
	ConnectionID string `json:"connection_id"`
	Full         bool   `json:"full,omitempty"`
}

// GroupPayload targets a whole group (JobMaterialsReclassify).
type GroupPayload struct {
	GroupID string `json:"group_id"`
}

// MaterialVersionPayload targets one version (JobDriveUpload, JobMaterialHash).
type MaterialVersionPayload struct {
	MaterialID string `json:"material_id"`
	VersionID  string `json:"version_id"`
}
