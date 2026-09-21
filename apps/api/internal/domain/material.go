package domain

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/google/uuid"
)

// MaterialKind is the type of a study material.
type MaterialKind string

// Material kinds.
const (
	KindLecture    MaterialKind = "LECTURE"
	KindNotes      MaterialKind = "NOTES"
	KindReport     MaterialKind = "REPORT"
	KindCalc       MaterialKind = "CALC"
	KindAssignment MaterialKind = "ASSIGNMENT"
	KindOther      MaterialKind = "OTHER"
)

// AllMaterialKinds lists kinds in display order.
var AllMaterialKinds = []MaterialKind{KindLecture, KindNotes, KindReport, KindCalc, KindAssignment, KindOther}

// MaterialSource tells where a material came from.
type MaterialSource string

// Material sources.
const (
	SourceUpload MaterialSource = "UPLOAD"
	SourceDrive  MaterialSource = "GDRIVE"
)

// MaterialStatus is the lifecycle state of a material.
type MaterialStatus string

// Material statuses. DELETED is a soft delete; files are purged later.
const (
	MaterialActive   MaterialStatus = "ACTIVE"
	MaterialArchived MaterialStatus = "ARCHIVED"
	MaterialDeleted  MaterialStatus = "DELETED"
)

// ReviewReason explains why a material sits in the moderators' Inbox.
type ReviewReason string

// Review reasons.
const (
	ReviewLowConfidence    ReviewReason = "LOW_CONFIDENCE"
	ReviewRemovedFromDrive ReviewReason = "REMOVED_FROM_DRIVE"
)

// StorageKind says where the bytes of a version live.
type StorageKind string

// Storage kinds.
const (
	StorageDrive StorageKind = "DRIVE"
	StorageS3    StorageKind = "S3"
	StorageLocal StorageKind = "LOCAL"
)

// ScanStatus is the antivirus verdict of a version.
type ScanStatus string

// Scan statuses.
const (
	ScanPending  ScanStatus = "PENDING"
	ScanClean    ScanStatus = "CLEAN"
	ScanInfected ScanStatus = "INFECTED"
	ScanSkipped  ScanStatus = "SKIPPED"
)

// PreviewStatus tracks the PDF preview of an office file.
type PreviewStatus string

// Preview statuses.
const (
	PreviewPending PreviewStatus = "PENDING"
	PreviewReady   PreviewStatus = "READY"
	PreviewFailed  PreviewStatus = "FAILED"
	PreviewSkipped PreviewStatus = "SKIPPED" // too large to convert
)

// officeMimes are the formats browsers cannot show but LibreOffice converts
// to PDF. Native Google documents are exported to PDF by Drive instead.
var officeMimes = map[string]bool{
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"application/vnd.ms-excel":                                                  true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.oasis.opendocument.text":                                   true,
	"application/vnd.oasis.opendocument.presentation":                           true,
	"application/vnd.oasis.opendocument.spreadsheet":                            true,
	"application/rtf": true,
	"text/rtf":        true,
}

// NeedsPDFPreview reports whether a file is shown through a PDF preview.
func NeedsPDFPreview(mime string) bool { return officeMimes[mime] }

// DocumentConverter turns office documents into PDF (Adapter: Gotenberg).
// fileName carries the extension the converter relies on. A file it cannot
// convert yields ErrInvalid; other errors are worth retrying.
type DocumentConverter interface {
	ConvertToPDF(ctx context.Context, fileName string, r io.Reader) (io.ReadCloser, error)
}

// DriveUploadStatus tracks a copy of an uploaded file to Google Drive.
type DriveUploadStatus string

// Drive upload statuses.
const (
	DriveUploadPending DriveUploadStatus = "PENDING"
	DriveUploadDone    DriveUploadStatus = "DONE"
	DriveUploadFailed  DriveUploadStatus = "FAILED"
)

// ClassifyMethod names how the subject/kind of a material was decided.
type ClassifyMethod string

// Classification methods.
const (
	ClassifyAuto   ClassifyMethod = "auto"
	ClassifyManual ClassifyMethod = "manual"
	ClassifyUpload ClassifyMethod = "upload" // chosen by the uploader
)

// Classification is the stored outcome of the classifier or a human decision.
type Classification struct {
	SubjectID  *uuid.UUID     `json:"subject_id,omitempty"`
	Kind       MaterialKind   `json:"kind,omitempty"`
	Confidence float64        `json:"confidence"`
	Method     ClassifyMethod `json:"method,omitempty"`
	Signals    []string       `json:"signals,omitempty"`
	Topic      *int           `json:"topic,omitempty"`
	Semester   *int           `json:"semester,omitempty"`
}

// JSON encodes the classification for storage.
func (c Classification) JSON() json.RawMessage {
	raw, err := json.Marshal(c)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

// ParseClassification decodes a stored classification, tolerating empty input.
func ParseClassification(raw json.RawMessage) Classification {
	var c Classification
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c)
	}
	return c
}

// Material is a file (or Google document) shared inside a group.
type Material struct {
	ID               uuid.UUID
	GroupID          uuid.UUID
	SubjectID        *uuid.UUID
	UploaderID       *uuid.UUID
	Title            string
	Description      string
	Kind             MaterialKind
	Source           MaterialSource
	Status           MaterialStatus
	CurrentVersionID *uuid.UUID
	Classification   Classification
	NeedsReview      bool
	ReviewReason     *ReviewReason
	DownloadCount    int32
	SortAt           time.Time
	ArchivedBy       *uuid.UUID
	ArchivedAt       *time.Time
	DeletedBy        *uuid.UUID
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// TaskID: the file was uploaded into this task and stays with it — not
	// in the feed, seen by those who see the task (D43). Nil — group material.
	TaskID *uuid.UUID
}

// MaterialVersion is one revision of a material's content.
type MaterialVersion struct {
	ID                uuid.UUID
	MaterialID        uuid.UUID
	VersionNo         int32
	Storage           StorageKind
	StorageKey        *string
	CacheExpiresAt    *time.Time
	DriveFileID       *string
	DriveWebViewLink  *string
	DriveMD5          *string
	DriveModifiedTime *time.Time
	// DriveRevisionID is the Drive revision the version was indexed from.
	DriveRevisionID   *string
	DriveUploadStatus *DriveUploadStatus
	DriveUploadError  *string
	OriginalName      string
	Mime              string
	SizeBytes         int64
	SHA256            *string
	ScanStatus        ScanStatus
	// PDF preview of an office file (nil status: not requested).
	PreviewStatus *PreviewStatus
	PreviewKey    *string
	PreviewError  *string
	UploadedBy    *uuid.UUID
	CreatedAt     time.Time
}

// MaterialView is a material with its current version and tags, as listed.
type MaterialView struct {
	Material Material
	Version  MaterialVersion
	TagIDs   []uuid.UUID
}

// MaterialFilter selects materials for the feed.
type MaterialFilter struct {
	GroupID    uuid.UUID
	Status     MaterialStatus
	SubjectID  *uuid.UUID
	NoSubject  bool
	Kind       *MaterialKind
	UploaderID *uuid.UUID
	Inbox      bool
	FileType   *FileType
	TagIDs     []uuid.UUID
	Query      string
	After      *MaterialCursor
	Limit      int32
}

// FileType is a coarse file category for the feed filter (D35).
type FileType string

// File categories.
const (
	FileDocument FileType = "DOCUMENT"
	FileImage    FileType = "IMAGE"
	FileAudio    FileType = "AUDIO"
	FileVideo    FileType = "VIDEO"
	FileArchive  FileType = "ARCHIVE"
	FileOther    FileType = "OTHER"
)

// AllFileTypes lists the categories, exported to packages/shared.
var AllFileTypes = []FileType{FileDocument, FileImage, FileAudio, FileVideo, FileArchive, FileOther}

// fileTypeMimes maps each category to SQL LIKE patterns over the MIME type.
var fileTypeMimes = map[FileType][]string{
	FileDocument: {
		"application/pdf", "text/%", "application/rtf", "application/msword",
		"application/vnd.openxmlformats-officedocument.%", "application/vnd.ms-%",
		"application/vnd.oasis.opendocument.%", "application/vnd.google-apps.%",
	},
	FileImage: {"image/%"},
	FileAudio: {"audio/%"},
	FileVideo: {"video/%"},
	FileArchive: {
		"application/zip", "application/x-zip-compressed", "application/x-7z-compressed",
		"application/x-rar-compressed", "application/vnd.rar", "application/gzip", "application/x-tar",
	},
}

// MimePatterns returns LIKE patterns for the category and whether matching
// ones must be excluded (OTHER is "none of the known categories").
func (t FileType) MimePatterns() (patterns []string, exclude bool) {
	if t != FileOther {
		return fileTypeMimes[t], false
	}
	for _, known := range AllFileTypes {
		patterns = append(patterns, fileTypeMimes[known]...)
	}
	return patterns, true
}

// MaterialCursor is the keyset position in the feed (sort_at DESC, id DESC).
type MaterialCursor struct {
	SortAt time.Time
	ID     uuid.UUID
}

// Upload is a pending direct upload that becomes a material once completed.
type Upload struct {
	ID          uuid.UUID
	GroupID     uuid.UUID
	UserID      uuid.UUID
	Storage     StorageKind
	StorageKey  string
	FileName    string
	Mime        string
	SizeBytes   int64
	Meta        UploadMeta
	Status      UploadStatus
	MaterialID  *uuid.UUID
	ExpiresAt   time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
}

// UploadStatus is the state of a pending upload.
type UploadStatus string

// Upload statuses.
const (
	UploadPending   UploadStatus = "PENDING"
	UploadCompleted UploadStatus = "COMPLETED"
	UploadExpired   UploadStatus = "EXPIRED"
)

// UploadMeta is the material form captured when the upload starts.
type UploadMeta struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	SubjectID   *uuid.UUID   `json:"subject_id,omitempty"`
	Kind        MaterialKind `json:"kind,omitempty"`
	TagIDs      []uuid.UUID  `json:"tag_ids,omitempty"`
	ToDrive     bool         `json:"to_drive,omitempty"`
	// TaskID attaches the new material to this task; TaskOnly keeps it there (D43).
	TaskID   *uuid.UUID `json:"task_id,omitempty"`
	TaskOnly bool       `json:"task_only,omitempty"`
}
