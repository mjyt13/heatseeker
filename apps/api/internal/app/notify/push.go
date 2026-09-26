package notify

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/ids"
)

// Deliver puts already stored notifications on their owners' devices. The
// in-app copy is written either way, so a failure here only costs a push.
func (s *Service) Deliver(ctx context.Context, notificationIDs []uuid.UUID) (int, error) {
	if s.Push == nil || len(notificationIDs) == 0 {
		return 0, nil
	}
	notes, err := s.Notify.Notifications(ctx, notificationIDs)
	if err != nil || len(notes) == 0 {
		return 0, err
	}
	users := make([]uuid.UUID, 0, len(notes))
	seen := map[uuid.UUID]bool{}
	for _, n := range notes {
		if !seen[n.UserID] {
			seen[n.UserID] = true
			users = append(users, n.UserID)
		}
	}
	targets, err := s.Notify.PushTargets(ctx, users)
	if err != nil {
		return 0, err
	}
	byUser := make(map[uuid.UUID][]domain.PushTarget, len(users))
	for _, t := range targets {
		byUser[t.UserID] = append(byUser[t.UserID], t)
	}
	now := s.Clock.Now()
	sent := 0
	for i := range notes {
		n := notes[i]
		ready, skipped := splitQuiet(byUser[n.UserID], now, urgent(n))
		for _, t := range skipped {
			s.record(ctx, n.ID, &t.DeviceID, domain.DeliverySkipped, "quiet hours or push off", nil)
		}
		if len(ready) == 0 {
			continue
		}
		results, err := s.Push.Push(ctx, n, ready)
		if err != nil {
			return sent, err
		}
		for _, r := range results {
			status, at := domain.DeliveryFailed, (*time.Time)(nil)
			if r.Sent {
				status, at = domain.DeliverySent, &now
				sent++
			}
			id := r.DeviceID
			s.record(ctx, n.ID, &id, status, r.Error, at)
			if r.Dead {
				if err := s.Notify.DisableDevicePush(ctx, r.DeviceID); err != nil {
					s.Log.Error("disable dead device", "device", r.DeviceID, "err", err)
				}
			}
		}
	}
	return sent, nil
}

// urgent reports whether an announcement was marked urgent by the headman.
func urgent(n domain.Notification) bool {
	if n.Type != domain.NotifyAnnouncement {
		return false
	}
	var data struct {
		Urgent bool `json:"urgent"`
	}
	return json.Unmarshal(n.Data, &data) == nil && data.Urgent
}

// splitQuiet separates devices that may be disturbed right now from the ones
// whose owner switched push off or is inside their quiet hours. Quiet hours
// hold everything back, including an urgent announcement, unless the member
// allowed exactly that (`urgent_in_quiet`, off by default — D51).
func splitQuiet(targets []domain.PushTarget, now time.Time, urgent bool) (ready, skipped []domain.PushTarget) {
	for _, t := range targets {
		allowedAtNight := urgent && t.Settings.UrgentInQuiet
		quiet := t.Settings.Quiet(now, location(t.Timezone)) && !allowedAtNight
		if t.Settings.PushEnabled && !quiet {
			ready = append(ready, t)
			continue
		}
		skipped = append(skipped, t)
	}
	return ready, skipped
}

// record keeps the delivery log; it never fails the push.
func (s *Service) record(ctx context.Context, notificationID uuid.UUID, deviceID *uuid.UUID, status domain.DeliveryStatus, msg string, at *time.Time) {
	err := s.Notify.RecordDelivery(ctx, domain.NotificationDelivery{
		ID: ids.New(), NotificationID: notificationID, Channel: domain.ChannelPush,
		DeviceID: deviceID, Status: status, Error: msg, SentAt: at,
	})
	if err != nil {
		s.Log.Error("record delivery", "notification", notificationID, "err", err)
	}
}
