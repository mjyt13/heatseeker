package http

import (
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/drive"
	"heatseeker/api/internal/app/materials"
	"heatseeker/api/internal/domain"
)

// ClassificationDTO is the classifier's (or a human's) decision.
type ClassificationDTO struct {
	SubjectID  *uuid.UUID `json:"subject_id,omitempty" doc:"Предложенный/подтверждённый предмет."`
	Kind       string     `json:"kind,omitempty"`
	Confidence float64    `json:"confidence"`
	Method     string     `json:"method,omitempty" enum:"auto,manual,upload"`
	Signals    []string   `json:"signals,omitempty" doc:"Чем руководствовался классификатор: path:…, name:…, conflict."`
	Topic      *int       `json:"topic,omitempty"`
	Semester   *int       `json:"semester,omitempty"`
}

func toClassificationDTO(c domain.Classification) ClassificationDTO {
	return ClassificationDTO{
		SubjectID: c.SubjectID, Kind: string(c.Kind), Confidence: c.Confidence, Method: string(c.Method),
		Signals: c.Signals, Topic: c.Topic, Semester: c.Semester,
	}
}

// MaterialVersionDTO is one revision of a file.
type MaterialVersionDTO struct {
	ID                uuid.UUID  `json:"id"`
	VersionNo         int32      `json:"version_no"`
	Storage           string     `json:"storage" enum:"DRIVE,S3,LOCAL"`
	OriginalName      string     `json:"original_name"`
	Mime              string     `json:"mime"`
	SizeBytes         int64      `json:"size_bytes"`
	ScanStatus        string     `json:"scan_status" enum:"PENDING,CLEAN,INFECTED,SKIPPED"`
	DriveWebViewLink  *string    `json:"drive_web_view_link,omitempty"`
	DriveModifiedTime *time.Time `json:"drive_modified_time,omitempty"`
	DriveUploadStatus *string    `json:"drive_upload_status,omitempty" enum:"PENDING,DONE,FAILED"`
	DriveUploadError  *string    `json:"drive_upload_error,omitempty"`
	UploadedBy        *uuid.UUID `json:"uploaded_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func toVersionDTO(v *domain.MaterialVersion) MaterialVersionDTO {
	dto := MaterialVersionDTO{
		ID: v.ID, VersionNo: v.VersionNo, Storage: string(v.Storage), OriginalName: v.OriginalName, Mime: v.Mime,
		SizeBytes: v.SizeBytes, ScanStatus: string(v.ScanStatus), DriveWebViewLink: v.DriveWebViewLink,
		DriveModifiedTime: v.DriveModifiedTime, DriveUploadError: v.DriveUploadError, UploadedBy: v.UploadedBy,
		CreatedAt: v.CreatedAt,
	}
	if v.DriveUploadStatus != nil {
		s := string(*v.DriveUploadStatus)
		dto.DriveUploadStatus = &s
	}
	return dto
}

// MaterialDTO is a material as listed in the feed.
type MaterialDTO struct {
	ID             uuid.UUID          `json:"id"`
	GroupID        uuid.UUID          `json:"group_id"`
	SubjectID      *uuid.UUID         `json:"subject_id,omitempty"`
	UploaderID     *uuid.UUID         `json:"uploader_id,omitempty" doc:"Нет у файлов, найденных на Диске."`
	Title          string             `json:"title"`
	Description    string             `json:"description" doc:"Markdown."`
	Kind           string             `json:"kind" enum:"LECTURE,NOTES,REPORT,CALC,ASSIGNMENT,OTHER"`
	Source         string             `json:"source" enum:"UPLOAD,GDRIVE"`
	Status         string             `json:"status" enum:"ACTIVE,ARCHIVED,DELETED"`
	TagIDs         []uuid.UUID        `json:"tag_ids"`
	Classification ClassificationDTO  `json:"classification"`
	NeedsReview    bool               `json:"needs_review" doc:"Лежит во «Входящих» модератора."`
	ReviewReason   *string            `json:"review_reason,omitempty" enum:"LOW_CONFIDENCE,REMOVED_FROM_DRIVE"`
	DownloadCount  int32              `json:"download_count"`
	SortAt         time.Time          `json:"sort_at"`
	File           MaterialVersionDTO `json:"file" doc:"Текущая версия файла."`
	ArchivedAt     *time.Time         `json:"archived_at,omitempty"`
	DeletedAt      *time.Time         `json:"deleted_at,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

func toMaterialDTO(v *domain.MaterialView) MaterialDTO {
	m := v.Material
	dto := MaterialDTO{
		ID: m.ID, GroupID: m.GroupID, SubjectID: m.SubjectID, UploaderID: m.UploaderID, Title: m.Title,
		Description: m.Description, Kind: string(m.Kind), Source: string(m.Source), Status: string(m.Status),
		TagIDs: v.TagIDs, Classification: toClassificationDTO(m.Classification), NeedsReview: m.NeedsReview,
		DownloadCount: m.DownloadCount, SortAt: m.SortAt, File: toVersionDTO(&v.Version),
		ArchivedAt: m.ArchivedAt, DeletedAt: m.DeletedAt, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if dto.TagIDs == nil {
		dto.TagIDs = []uuid.UUID{}
	}
	if m.ReviewReason != nil {
		r := string(*m.ReviewReason)
		dto.ReviewReason = &r
	}
	return dto
}

// MaterialDetailsDTO adds history, origin and what the caller may do.
type MaterialDetailsDTO struct {
	MaterialDTO
	Versions  []MaterialVersionDTO `json:"versions"`
	DrivePath []string             `json:"drive_path,omitempty" doc:"Папки на Диске от корня группы."`
	CanEdit   bool                 `json:"can_edit"`
	CanDelete bool                 `json:"can_delete"`
	Moderator bool                 `json:"moderator"`
}

func toMaterialDetailsDTO(d *materials.Details) MaterialDetailsDTO {
	out := MaterialDetailsDTO{
		MaterialDTO: toMaterialDTO(&d.View), DrivePath: d.DrivePath,
		CanEdit: d.CanEdit, CanDelete: d.CanDelete, Moderator: d.Moderator,
		Versions: make([]MaterialVersionDTO, len(d.Versions)),
	}
	for i := range d.Versions {
		out.Versions[i] = toVersionDTO(&d.Versions[i])
	}
	return out
}

// OpenDTO tells the client how to show a file.
type OpenDTO struct {
	MaterialID       uuid.UUID  `json:"material_id"`
	VersionID        uuid.UUID  `json:"version_id"`
	Mode             string     `json:"mode" enum:"LINK,CACHE,IMPORT"`
	Storage          string     `json:"storage" enum:"DRIVE,S3,LOCAL"`
	FileName         string     `json:"file_name"`
	Mime             string     `json:"mime"`
	SizeBytes        int64      `json:"size_bytes"`
	DriveWebViewLink *string    `json:"drive_web_view_link,omitempty" doc:"Открыть в Google Диске (нужен доступ к папке)."`
	StreamURL        *string    `json:"stream_url,omitempty" doc:"Файл с Диска через сервер (поддерживает Range)."`
	DownloadURL      *string    `json:"download_url,omitempty" doc:"Прямая ссылка на файл в хранилище."`
	ExpiresAt        *time.Time `json:"expires_at,omitempty" doc:"Когда ссылки перестанут работать."`
}

func toOpenDTO(o *materials.OpenResult) OpenDTO {
	return OpenDTO{
		MaterialID: o.MaterialID, VersionID: o.VersionID, Mode: string(o.Mode), Storage: string(o.Storage),
		FileName: o.FileName, Mime: o.Mime, SizeBytes: o.SizeBytes, DriveWebViewLink: o.DriveWebViewLink,
		StreamURL: o.StreamURL, DownloadURL: o.DownloadURL, ExpiresAt: o.ExpiresAt,
	}
}

// UploadTicketDTO tells the client where to send the file.
type UploadTicketDTO struct {
	UploadID  uuid.UUID         `json:"upload_id"`
	Method    string            `json:"method" enum:"PUT"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
	MaxBytes  int64             `json:"max_bytes"`
	FileName  string            `json:"file_name"`
	Mime      string            `json:"mime"`
}

func toUploadTicketDTO(t *materials.UploadTicket) UploadTicketDTO {
	headers := t.Request.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	method := t.Request.Method
	if method == "" {
		method = "PUT"
	}
	return UploadTicketDTO{
		UploadID: t.Upload.ID, Method: method, URL: t.Request.URL, Headers: headers, ExpiresAt: t.Request.ExpiresAt,
		MaxBytes: t.MaxSize, FileName: t.Upload.FileName, Mime: t.Upload.Mime,
	}
}

// DriveConnectionDTO describes the group's Drive folder.
type DriveConnectionDTO struct {
	ID              uuid.UUID  `json:"id"`
	RootFolderID    string     `json:"root_folder_id"`
	RootFolderName  string     `json:"root_folder_name"`
	FolderURL       string     `json:"folder_url"`
	SharedDrive     bool       `json:"shared_drive" doc:"Папка лежит в общем диске (Google Workspace)."`
	Status          string     `json:"status" enum:"PENDING,SYNCING,OK,ERROR"`
	LastError       *string    `json:"last_error,omitempty" doc:"Только для управляющих Диском."`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	LastFullScanAt  *time.Time `json:"last_full_scan_at,omitempty"`
	SyncIntervalSec int32      `json:"sync_interval_sec"`
	Writable        bool       `json:"writable" doc:"Сервисный аккаунт может добавлять файлы в папку."`
	CreatedAt       time.Time  `json:"created_at"`
}

// DriveStatsDTO summarises the index.
type DriveStatsDTO struct {
	Files     int64 `json:"files"`
	Folders   int64 `json:"folders"`
	Linked    int64 `json:"linked"`
	Skipped   int64 `json:"skipped"`
	Deleted   int64 `json:"deleted"`
	Errors    int64 `json:"errors"`
	InboxSize int64 `json:"inbox_size"`
}

// DriveStatusDTO is the Drive integration as seen by a member.
type DriveStatusDTO struct {
	Configured          bool                `json:"configured" doc:"На сервере настроен сервисный аккаунт."`
	ServiceAccountEmail string              `json:"service_account_email,omitempty" doc:"Этому адресу нужно выдать доступ к папке."`
	UploadEnabled       bool                `json:"upload_enabled"`
	Connection          *DriveConnectionDTO `json:"connection,omitempty"`
	Stats               *DriveStatsDTO      `json:"stats,omitempty"`
}

func toDriveConnectionDTO(c *domain.DriveConnection) *DriveConnectionDTO {
	return &DriveConnectionDTO{
		ID: c.ID, RootFolderID: c.RootFolderID, RootFolderName: c.RootFolderName,
		FolderURL:   "https://drive.google.com/drive/folders/" + c.RootFolderID,
		SharedDrive: c.DriveID != nil, Status: string(c.Status), LastError: c.LastError, LastSyncAt: c.LastSyncAt,
		LastFullScanAt: c.LastFullScanAt, SyncIntervalSec: c.SyncIntervalSec, Writable: c.Writable, CreatedAt: c.CreatedAt,
	}
}

func toDriveStatusDTO(s *drive.Status) DriveStatusDTO {
	dto := DriveStatusDTO{Configured: s.Configured, ServiceAccountEmail: s.ServiceAccountEmail, UploadEnabled: s.UploadEnabled}
	if s.Connection != nil {
		dto.Connection = toDriveConnectionDTO(s.Connection)
	}
	if s.Stats != nil {
		st := s.Stats
		dto.Stats = &DriveStatsDTO{
			Files: st.Files, Folders: st.Folders, Linked: st.Linked, Skipped: st.Skipped,
			Deleted: st.Deleted, Errors: st.Errors, InboxSize: st.InboxSize,
		}
	}
	return dto
}

// DriveItemDTO is an indexed Drive file.
type DriveItemDTO struct {
	ID             uuid.UUID         `json:"id"`
	DriveFileID    string            `json:"drive_file_id"`
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	Mime           string            `json:"mime"`
	SizeBytes      *int64            `json:"size_bytes,omitempty"`
	ModifiedTime   *time.Time        `json:"modified_time,omitempty"`
	WebViewLink    *string           `json:"web_view_link,omitempty"`
	MaterialID     *uuid.UUID        `json:"material_id,omitempty"`
	State          string            `json:"state" enum:"NEW,LINKED,IMPORTED,SKIPPED,ERROR,DELETED"`
	Classification ClassificationDTO `json:"classification"`
	LastError      *string           `json:"last_error,omitempty"`
	SeenAt         time.Time         `json:"seen_at"`
}

func toDriveItemDTO(i *domain.DriveItem) DriveItemDTO {
	return DriveItemDTO{
		ID: i.ID, DriveFileID: i.DriveFileID, Path: i.PathCache, Name: i.Name, Mime: i.Mime, SizeBytes: i.SizeBytes,
		ModifiedTime: i.ModifiedTime, WebViewLink: i.WebViewLink, MaterialID: i.MaterialID, State: string(i.State),
		Classification: toClassificationDTO(i.Classification), LastError: i.LastError, SeenAt: i.SeenAt,
	}
}
