package notify

import (
	"context"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/ids"
)

// MaxMute is the longest a member can silence something: a mute always ends
// (docs/PLAN.md §8.2), and a year of silence is a mistake, not a wish.
const MaxMute = 365 * 24 * time.Hour

const (
	defaultPageSize = 30
	maxPageSize     = 100
)

// Page is a page of the member's own notifications.
type Page struct {
	Items []domain.Notification
	// Unread counts everything unread in the filtered scope, not only this page.
	Unread int
	// Next is the cursor for the following page; empty at the end.
	Next string
}

// List returns the member's notifications, newest first.
func (s *Service) List(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID, unreadOnly bool, cursor string, limit int32) (*Page, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	f := domain.NotificationFilter{UserID: userID, GroupID: groupID, UnreadOnly: unreadOnly, Limit: limit + 1}
	if cursor != "" {
		at, id, err := parseCursor(cursor)
		if err != nil {
			return nil, err
		}
		f.Before, f.BeforeID = &at, &id
	}
	items, err := s.Notify.List(ctx, f)
	if err != nil {
		return nil, err
	}
	page := &Page{Items: items}
	if len(items) > int(limit) {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.Next = makeCursor(last.CreatedAt, last.ID)
	}
	if page.Unread, err = s.Notify.Unread(ctx, userID, groupID); err != nil {
		return nil, err
	}
	return page, nil
}

// Unread counts what the member has not seen yet.
func (s *Service) Unread(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID) (int, error) {
	return s.Notify.Unread(ctx, userID, groupID)
}

// MarkRead marks the listed notifications as read, or every unread one in the
// scope when all is set.
func (s *Service) MarkRead(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID, notificationIDs []uuid.UUID, all bool) (int, error) {
	if !all && len(notificationIDs) == 0 {
		return 0, domain.Invalid("ids", "list what to mark, or ask for all")
	}
	return s.Notify.MarkRead(ctx, userID, groupID, notificationIDs, all)
}

// Pref is one type as the member sees it, with the default already applied.
type Pref struct {
	Type    domain.NotificationType
	Enabled bool
	// Custom tells the client the member has set this one themselves.
	Custom bool
}

// Prefs returns every type the reader can produce, in a fixed order.
func (s *Service) Prefs(ctx context.Context, actorID, groupID uuid.UUID) ([]Pref, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	saved, err := s.Notify.Prefs(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	custom := make(map[domain.NotificationType]bool, len(saved))
	for _, p := range saved {
		custom[p.Type] = p.Enabled
	}
	out := make([]Pref, 0, len(domain.NotificationTypesInUse))
	for _, t := range domain.NotificationTypesInUse {
		enabled, ok := custom[t]
		if !ok {
			enabled = t.DefaultOn(actor.Group.Kind, s.set.DPOSilent)
		}
		out = append(out, Pref{Type: t, Enabled: enabled, Custom: ok})
	}
	return out, nil
}

// SetPrefs switches types on and off in one group.
func (s *Service) SetPrefs(ctx context.Context, actorID, groupID uuid.UUID, prefs map[domain.NotificationType]bool) ([]Pref, error) {
	if _, err := s.Access.Actor(ctx, actorID, groupID); err != nil {
		return nil, err
	}
	err := s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		for t, enabled := range prefs {
			if !t.Valid() {
				return domain.Invalid("type", "unknown notification type "+string(t))
			}
			if err := s.Notify.SetPref(ctx, domain.NotificationPref{
				UserID: actorID, GroupID: groupID, Type: t, Enabled: enabled,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Prefs(ctx, actorID, groupID)
}

// Settings returns the member's delivery settings, defaults included.
func (s *Service) Settings(ctx context.Context, userID uuid.UUID) (domain.NotificationSettings, error) {
	saved, err := s.Notify.Settings(ctx, userID)
	if err != nil {
		return domain.NotificationSettings{}, err
	}
	if saved == nil {
		return s.defaultSettings(userID), nil
	}
	return *saved, nil
}

func (s *Service) defaultSettings(userID uuid.UUID) domain.NotificationSettings {
	return domain.NotificationSettings{
		UserID: userID, PushEnabled: true, QuietFrom: s.set.QuietFrom, QuietTo: s.set.QuietTo,
	}
}

// SettingsInput is what a member may change about delivery.
type SettingsInput struct {
	PushEnabled bool
	// QuietFrom and QuietTo are minutes from midnight in the member's own
	// timezone; both go together, nil means no quiet hours.
	QuietFrom *int16
	QuietTo   *int16
	// UrgentInQuiet lets urgent announcements through the quiet hours.
	UrgentInQuiet bool
}

// SaveSettings stores push, quiet hours and whether urgent announcements may
// break through them.
func (s *Service) SaveSettings(ctx context.Context, userID uuid.UUID, in SettingsInput) (domain.NotificationSettings, error) {
	if (in.QuietFrom == nil) != (in.QuietTo == nil) {
		return domain.NotificationSettings{}, domain.Invalid("quiet_to", "set both ends of the quiet hours, or neither")
	}
	for _, v := range []*int16{in.QuietFrom, in.QuietTo} {
		if v != nil && (*v < 0 || *v > 1439) {
			return domain.NotificationSettings{}, domain.Invalid("quiet_from", "must be minutes from midnight, 0–1439")
		}
	}
	saved, err := s.Notify.SaveSettings(ctx, domain.NotificationSettings{
		UserID: userID, PushEnabled: in.PushEnabled, QuietFrom: in.QuietFrom, QuietTo: in.QuietTo,
		UrgentInQuiet: in.UrgentInQuiet,
	})
	if err != nil {
		return domain.NotificationSettings{}, err
	}
	return *saved, nil
}

// Mutes lists what is silenced right now.
func (s *Service) Mutes(ctx context.Context, actorID uuid.UUID, groupID *uuid.UUID) ([]domain.NotificationMute, error) {
	if groupID != nil {
		if _, err := s.Access.Actor(ctx, actorID, *groupID); err != nil {
			return nil, err
		}
	}
	return s.Notify.Mutes(ctx, actorID, groupID)
}

// Mute silences a subject, a discussion, one type or the whole group until a
// moment in time. Muting again simply moves the end.
func (s *Service) Mute(ctx context.Context, actorID, groupID uuid.UUID, scope domain.MuteScope, scopeID string, until time.Time) (*domain.NotificationMute, error) {
	if _, err := s.Access.Actor(ctx, actorID, groupID); err != nil {
		return nil, err
	}
	if !scope.Valid() {
		return nil, domain.Invalid("scope_type", "unknown mute scope "+string(scope))
	}
	now := s.Clock.Now()
	if !until.After(now) {
		return nil, domain.Invalid("until", "must be in the future")
	}
	if until.After(now.Add(MaxMute)) {
		return nil, domain.Invalid("until", "must be within a year")
	}
	switch scope {
	case domain.MuteGroup:
		scopeID = groupID.String()
	case domain.MuteType:
		if !domain.NotificationType(scopeID).Valid() {
			return nil, domain.Invalid("scope_id", "unknown notification type "+scopeID)
		}
	default:
		id, err := uuid.Parse(scopeID)
		if err != nil {
			return nil, domain.Invalid("scope_id", "must be an id")
		}
		scopeID = id.String()
	}
	return s.Notify.SetMute(ctx, domain.NotificationMute{
		ID: ids.New(), UserID: actorID, GroupID: groupID, Scope: scope, ScopeID: scopeID, Until: until.UTC(),
	})
}

// Unmute brings a scope back.
func (s *Service) Unmute(ctx context.Context, actorID, groupID uuid.UUID, scope domain.MuteScope, scopeID string) error {
	if _, err := s.Access.Actor(ctx, actorID, groupID); err != nil {
		return err
	}
	if scope == domain.MuteGroup {
		scopeID = groupID.String()
	}
	return s.Notify.DeleteMute(ctx, actorID, groupID, scope, scopeID)
}

// Cleanup drops expired mutes and notifications nobody will open again.
func (s *Service) Cleanup(ctx context.Context) (int64, error) {
	now := s.Clock.Now()
	if _, err := s.Notify.DeleteExpiredMutes(ctx, now); err != nil {
		return 0, err
	}
	if s.set.Retention <= 0 {
		return 0, nil
	}
	return s.Notify.DeleteOlderThan(ctx, now.Add(-s.set.Retention))
}
