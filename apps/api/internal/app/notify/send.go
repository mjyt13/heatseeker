package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/ids"
)

// Send writes one notification addressed to a single member and puts it on
// their devices. It is for facts that are not group news — a reminder the
// member set for themselves — so it asks no preferences and no mutes: an
// alarm one has set should ring. Quiet hours still hold back the push.
func (s *Service) Send(ctx context.Context, n domain.Notification) error {
	if n.ID == uuid.Nil {
		n.ID = ids.New()
	}
	if n.Data == nil {
		n.Data = json.RawMessage("{}")
	}
	saved, err := s.Notify.Insert(ctx, n)
	if err != nil {
		return err
	}
	if saved == nil || s.Queue == nil {
		return nil // already delivered once
	}
	job := domain.Job{
		Type: domain.JobNotifyPush, Payload: domain.NotifyPushPayload{IDs: []string{saved.ID.String()}},
		Queue: "default", MaxRetry: 3,
	}
	if err := s.Queue.Enqueue(ctx, job); err != nil {
		s.Log.Error("schedule push", "notification", saved.ID, "err", err)
	}
	return nil
}

// Data encodes a deep link for Send.
func Data(v map[string]any) (json.RawMessage, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("notification data: %w", err)
	}
	return raw, nil
}
