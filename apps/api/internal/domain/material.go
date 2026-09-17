package domain

import (
	"encoding/json"
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
	DriveUploadStatus *DriveUploadStatus
	DriveUploadError  *string
	OriginalName      string
	Mime              string
	SizeBytes         int64
	SHA256            *string
	ScanStatus        ScanStatus
	UploadedBy        *uuid.UUID
	CreatedAt         time.Time
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
	TagIDs     []uuid.UUID
	Query      string
	After      *MaterialCursor
	Limit      int32
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
}
