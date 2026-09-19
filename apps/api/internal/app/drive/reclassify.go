package drive

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/classify"
	"heatseeker/api/internal/domain"
)

// reclassifyDelay lets a burst of Inbox decisions settle into one run.
const reclassifyDelay = 2 * time.Second

// OnEvent is an event-bus subscriber: when subjects or their aliases change,
// Inbox files may now be recognisable, so a reclassification is scheduled.
func (s *Service) OnEvent(e domain.Event) {
	switch e.Kind {
	case domain.EventSubjectCreated, domain.EventSubjectUpdated, domain.EventSubjectRestored:
	case domain.EventMaterialClassified:
		var p struct {
			LearnedAlias string `json:"learned_alias"`
		}
		if json.Unmarshal(e.Payload, &p) != nil || p.LearnedAlias == "" {
			return
		}
	default:
		return
	}
	if s.Queue == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Only Drive files are reclassified: skip groups without a connection.
	if _, err := s.Repo.GetConnectionByGroup(ctx, e.GroupID); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			s.Log.Error("reclassify: load drive connection", "group", e.GroupID, "err", err)
		}
		return
	}
	// No uniqueness: a run already in progress may have read the old aliases,
	// and repeated runs are cheap and idempotent.
	err := s.Queue.Enqueue(ctx, domain.Job{
		Type:     domain.JobMaterialsReclassify,
		Payload:  domain.GroupPayload{GroupID: e.GroupID.String()},
		Queue:    "default",
		MaxRetry: 3,
		Delay:    reclassifyDelay,
	})
	if err != nil {
		s.Log.Error("enqueue reclassify", "group", e.GroupID, "err", err)
	}
}

// Reclassify re-runs automatic classification for the group's Drive files that
// wait in the Inbox because the classifier was unsure, and files the ones it is
// now confident about. It returns how many left the Inbox.
func (s *Service) Reclassify(ctx context.Context, groupID uuid.UUID) (int, error) {
	candidates, err := s.Repo.ListReclassifyCandidates(ctx, groupID)
	if err != nil || len(candidates) == 0 {
		return 0, err
	}
	subjects, err := s.Subjects.List(ctx, groupID, false)
	if err != nil {
		return 0, err
	}
	targets := classify.FromDomain(subjects)
	if len(targets) == 0 {
		return 0, nil
	}
	resolved := 0
	for _, item := range candidates {
		c := classify.Classify(classify.Input{FileName: item.Name, Folders: item.PathSegments()}, targets)
		if c.SubjectID == nil || c.Confidence < s.cfg.MinConfidence {
			continue
		}
		done, err := s.fileResolved(ctx, groupID, item, c)
		if err != nil {
			return resolved, err
		}
		if done {
			resolved++
		}
	}
	if resolved > 0 {
		conn, err := s.Repo.GetConnectionByGroup(ctx, groupID)
		switch {
		case err == nil:
			err = s.emit(ctx, groupID, domain.EventDriveReclassified, nil, conn.ID, map[string]any{"count": resolved}, true)
			if err != nil {
				return resolved, err
			}
		case !errors.Is(err, domain.ErrNotFound):
			return resolved, err
		}
	}
	s.Log.Info("inbox reclassified", "group", groupID, "candidates", len(candidates), "resolved", resolved)
	return resolved, nil
}

// fileResolved moves one material out of the Inbox, re-checking its state in
// the transaction: a moderator may have decided in the meantime.
func (s *Service) fileResolved(ctx context.Context, groupID uuid.UUID, item domain.DriveItem, c domain.Classification) (bool, error) {
	done := false
	err := s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		m, err := s.Materials.Get(ctx, *item.MaterialID)
		if err != nil {
			return err
		}
		if m.GroupID != groupID || !m.NeedsReview || m.ReviewReason == nil ||
			*m.ReviewReason != domain.ReviewLowConfidence || m.Classification.Method == domain.ClassifyManual {
			return nil
		}
		_, err = s.Materials.Update(ctx, domain.UpdateMaterialParams{
			ID: m.ID, Title: m.Title, Description: m.Description, SubjectID: c.SubjectID, Kind: c.Kind,
			Classification: c, NeedsReview: false, ReviewReason: nil,
		})
		if err != nil {
			return err
		}
		if err := s.Repo.UpdateItemState(ctx, item.ID, item.State, item.MaterialID, c, nil); err != nil {
			return err
		}
		done = true
		// Per-file events keep offline clients in sync; the summary event is
		// what the activity feed shows.
		return s.emit(ctx, groupID, domain.EventMaterialClassified, nil, m.ID, map[string]any{
			"title": m.Title, "subject_id": c.SubjectID, "kind": c.Kind, "method": c.Method,
		}, false)
	})
	return done, err
}
