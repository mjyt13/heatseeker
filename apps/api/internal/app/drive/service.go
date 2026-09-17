// Package drive connects a group to a shared Google Drive folder through the
// service account, keeps the index up to date and publishes uploaded files to
// the folder (docs/PLAN.md §6).
package drive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/access"
	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/events"
	"heatseeker/api/internal/platform/clock"
	"heatseeker/api/internal/platform/ids"
)

// Settings come from configuration.
type Settings struct {
	SyncInterval       time.Duration
	FullRescanInterval time.Duration
	// StaleAfter lets a new run take over a SYNCING connection whose worker died.
	StaleAfter    time.Duration
	MinConfidence float64
	DeletePolicy  string // flag | archive
	UploadEnabled bool
}

// Deps are the collaborators of the service.
type Deps struct {
	Repo      domain.DriveRepo
	Materials domain.MaterialRepo
	Subjects  domain.SubjectRepo
	Users     domain.UserRepo
	Store     domain.MediaStore
	Client    domain.DriveClient // nil when no service account is configured
	Access    *access.Service
	Events    *events.Publisher
	Tx        domain.TxManager
	Queue     domain.JobQueue
	Clock     clock.Clock
	Log       *slog.Logger
}

// Service is the Drive use-case layer.
type Service struct {
	Deps
	cfg Settings
}

// NewService wires the service.
func NewService(d Deps, cfg Settings) *Service {
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = 30 * time.Minute
	}
	return &Service{Deps: d, cfg: cfg}
}

// Status is what a group sees about its Drive integration.
type Status struct {
	Configured          bool
	ServiceAccountEmail string
	UploadEnabled       bool
	Connection          *domain.DriveConnection
	Stats               *domain.DriveStats
}

// Status describes the group's connection (readable by every member so
// anyone can see where files come from).
func (s *Service) Status(ctx context.Context, actorID, groupID uuid.UUID) (*Status, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.GroupRead); err != nil {
		return nil, err
	}
	st := &Status{Configured: s.Client != nil, UploadEnabled: s.cfg.UploadEnabled && s.Client != nil}
	if s.Client != nil {
		st.ServiceAccountEmail = s.Client.ServiceAccountEmail()
	}
	conn, err := s.Repo.GetConnectionByGroup(ctx, groupID)
	if errors.Is(err, domain.ErrNotFound) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	st.Connection = conn
	if st.Stats, err = s.Repo.Stats(ctx, conn.ID); err != nil {
		return nil, err
	}
	if st.Stats.InboxSize, err = s.Materials.CountInbox(ctx, groupID); err != nil {
		return nil, err
	}
	if !actor.Can(authz.DriveManage) {
		st.Connection.LastError = nil
	}
	return st, nil
}

var (
	folderURLRe = regexp.MustCompile(`/folders/([A-Za-z0-9_-]{10,})`)
	idParamRe   = regexp.MustCompile(`[?&]id=([A-Za-z0-9_-]{10,})`)
	bareIDRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{10,}$`)
)

// ParseFolderRef extracts a folder id from a Drive link or returns a bare id.
func ParseFolderRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if m := folderURLRe.FindStringSubmatch(ref); m != nil {
		return m[1], nil
	}
	if m := idParamRe.FindStringSubmatch(ref); m != nil {
		return m[1], nil
	}
	if bareIDRe.MatchString(ref) {
		return ref, nil
	}
	return "", domain.Invalid("folder", "paste a link to a Google Drive folder")
}

// Connect links the group to a folder shared with the service account and
// schedules the first full scan.
func (s *Service) Connect(ctx context.Context, actorID, groupID uuid.UUID, folderRef string) (*domain.DriveConnection, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.DriveManage); err != nil {
		return nil, err
	}
	if s.Client == nil {
		return nil, domain.Unavailable("Google Drive is not configured on this server")
	}
	folderID, err := ParseFolderRef(folderRef)
	if err != nil {
		return nil, err
	}
	folder, err := s.Client.GetFile(ctx, folderID)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
		return nil, domain.Invalid("folder", fmt.Sprintf("the folder is not shared with %s", s.Client.ServiceAccountEmail()))
	}
	if err != nil {
		return nil, err
	}
	if !folder.IsFolder() {
		return nil, domain.Invalid("folder", "the link points to a file, not a folder")
	}
	if folder.Trashed {
		return nil, domain.Invalid("folder", "the folder is in the trash")
	}
	var driveID *string
	if folder.DriveID != "" {
		driveID = &folder.DriveID
	}
	var conn *domain.DriveConnection
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		previous, err := s.Repo.GetConnectionByGroup(ctx, groupID)
		switch {
		case err == nil && previous.RootFolderID != folderID:
			// A different folder: forget the old index, keep the materials.
			if err := s.Repo.DeleteItems(ctx, previous.ID); err != nil {
				return err
			}
		case err != nil && !errors.Is(err, domain.ErrNotFound):
			return err
		}
		conn, err = s.Repo.UpsertConnection(ctx, domain.DriveConnection{
			ID: ids.New(), GroupID: groupID, RootFolderID: folderID, RootFolderName: folder.Name, DriveID: driveID,
			SyncIntervalSec: int32(s.cfg.SyncInterval / time.Second), Writable: folder.CanAddChild, CreatedBy: &actorID,
		})
		if err != nil {
			return err
		}
		connID := conn.ID
		s.Tx.AfterCommit(ctx, func() { s.enqueueSync(connID, true) })
		return s.emit(ctx, groupID, domain.EventDriveConnected, &actorID, conn.ID, map[string]any{
			"folder_name": folder.Name, "writable": folder.CanAddChild,
		}, true)
	})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// Disconnect removes the connection and its index; materials stay.
func (s *Service) Disconnect(ctx context.Context, actorID, groupID uuid.UUID) error {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.DriveManage); err != nil {
		return err
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		conn, err := s.Repo.GetConnectionByGroup(ctx, groupID)
		if err != nil {
			return err
		}
		if err := s.Repo.DeleteConnection(ctx, conn.ID); err != nil {
			return err
		}
		return s.emit(ctx, groupID, domain.EventDriveDisconnected, &actorID, conn.ID, map[string]any{"folder_name": conn.RootFolderName}, true)
	})
}

// RequestSync schedules a sync now (full rescan when full is set).
func (s *Service) RequestSync(ctx context.Context, actorID, groupID uuid.UUID, full bool) error {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.DriveManage); err != nil {
		return err
	}
	if s.Client == nil {
		return domain.Unavailable("Google Drive is not configured on this server")
	}
	conn, err := s.Repo.GetConnectionByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	return s.Queue.Enqueue(ctx, syncJob(conn.ID, full))
}

// ListItems pages through indexed files (for admins troubleshooting).
func (s *Service) ListItems(ctx context.Context, actorID, groupID uuid.UUID, state *domain.DriveItemState, limit, offset int32) ([]domain.DriveItem, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.DriveManage); err != nil {
		return nil, err
	}
	conn, err := s.Repo.GetConnectionByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.Repo.ListItems(ctx, conn.ID, state, limit, max(offset, 0))
}

// EnqueueDue schedules incremental syncs for connections whose interval has
// passed. Called every minute by the scheduler.
func (s *Service) EnqueueDue(ctx context.Context) (int, error) {
	if s.Client == nil {
		return 0, nil
	}
	due, err := s.Repo.ListDue(ctx, s.Clock.Now())
	if err != nil {
		return 0, err
	}
	for _, c := range due {
		if err := s.Queue.Enqueue(ctx, syncJob(c.ID, false)); err != nil {
			return 0, err
		}
	}
	return len(due), nil
}

func syncJob(connID uuid.UUID, full bool) domain.Job {
	return domain.Job{
		Type:      domain.JobDriveSync,
		Payload:   domain.DriveSyncPayload{ConnectionID: connID.String(), Full: full},
		Queue:     "default",
		UniqueFor: 2 * time.Minute,
		MaxRetry:  3,
	}
}

func (s *Service) enqueueSync(connID uuid.UUID, full bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Queue.Enqueue(ctx, syncJob(connID, full)); err != nil {
		s.Log.Error("enqueue drive sync", "connection", connID, "err", err)
	}
}

func (s *Service) emit(ctx context.Context, groupID uuid.UUID, kind domain.EventKind, actor *uuid.UUID, entityID uuid.UUID, payload any, audit bool) error {
	entity := "material"
	if strings.HasPrefix(string(kind), "drive.") {
		entity = "drive_connection"
	}
	e, err := domain.NewEvent(groupID, kind, actor, entity, &entityID, payload)
	e.Audit = audit
	return s.Events.Emit(ctx, e, err)
}
