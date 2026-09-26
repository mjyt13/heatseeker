package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type notifyRepo struct{ s *Store }

func toNotification(n sqlcgen.Notification) domain.Notification {
	return domain.Notification{
		ID: n.ID, UserID: n.UserID, GroupID: n.GroupID, Type: domain.NotificationType(n.Type),
		Title: n.Title, Body: n.Body, Data: rawJSON(n.Data), Seq: n.Seq, DedupeKey: n.DedupeKey,
		ReadAt: n.ReadAt, CreatedAt: n.CreatedAt,
	}
}

func toNotifications(rows []sqlcgen.Notification) []domain.Notification {
	out := make([]domain.Notification, len(rows))
	for i, row := range rows {
		out[i] = toNotification(row)
	}
	return out
}

func toMute(m sqlcgen.NotificationMute) domain.NotificationMute {
	return domain.NotificationMute{
		ID: m.ID, UserID: m.UserID, GroupID: m.GroupID, Scope: domain.MuteScope(m.ScopeType),
		ScopeID: m.ScopeID, Until: m.Until, CreatedAt: m.CreatedAt,
	}
}

func toSettings(s sqlcgen.NotificationSetting) domain.NotificationSettings {
	return domain.NotificationSettings{
		UserID: s.UserID, PushEnabled: s.PushEnabled, QuietFrom: s.QuietFrom, QuietTo: s.QuietTo,
		UrgentInQuiet: s.UrgentInQuiet,
	}
}

func (r *notifyRepo) Cursor(ctx context.Context, groupID uuid.UUID) (int64, bool, error) {
	seq, err := r.s.queries(ctx).GetNotifierCursor(ctx, groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, mapErr(err, "notifier cursor")
	}
	return seq, true, nil
}

func (r *notifyRepo) SetCursor(ctx context.Context, groupID uuid.UUID, seq int64) error {
	err := r.s.queries(ctx).SetNotifierCursor(ctx, sqlcgen.SetNotifierCursorParams{GroupID: groupID, LastSeq: seq})
	return mapErr(err, "notifier cursor")
}

func (r *notifyRepo) GroupsWithPending(ctx context.Context, limit int32) ([]uuid.UUID, error) {
	ids, err := r.s.queries(ctx).ListGroupsWithPendingEvents(ctx, limit)
	return ids, mapErr(err, "groups")
}

func (r *notifyRepo) Recipients(ctx context.Context, q domain.NotifyRecipients) ([]domain.Recipient, error) {
	p := sqlcgen.ListNotifyRecipientsParams{
		GroupID: q.GroupID, Type: string(q.Type), Exclude: q.Exclude, DefaultEnabled: q.Default,
		Now: q.Now, SubjectID: idText(q.SubjectID), ThreadID: idText(q.ThreadID), Seq: q.Seq,
	}
	if p.Exclude == nil {
		p.Exclude = []uuid.UUID{}
	}
	rows, err := r.s.queries(ctx).ListNotifyRecipients(ctx, p)
	if err != nil {
		return nil, mapErr(err, "recipients")
	}
	out := make([]domain.Recipient, len(rows))
	for i, row := range rows {
		out[i] = domain.Recipient{UserID: row.UserID, Timezone: row.Timezone}
	}
	return out, nil
}

func (r *notifyRepo) ThreadParticipants(ctx context.Context, threadID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.s.queries(ctx).ListThreadParticipants(ctx, threadID)
	if err != nil {
		return nil, mapErr(err, "thread participants")
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, id := range rows {
		if id != nil {
			out = append(out, *id)
		}
	}
	return out, nil
}

func (r *notifyRepo) TaskAssignees(ctx context.Context, taskID uuid.UUID) ([]domain.TaskAssignee, error) {
	rows, err := r.s.queries(ctx).ListTaskAssignees(ctx, taskID)
	if err != nil {
		return nil, mapErr(err, "task assignees")
	}
	out := make([]domain.TaskAssignee, len(rows))
	for i, row := range rows {
		out[i] = domain.TaskAssignee{UserID: row.UserID, Status: domain.TaskStatus(row.Status)}
	}
	return out, nil
}

func (r *notifyRepo) Insert(ctx context.Context, n domain.Notification) (*domain.Notification, error) {
	rows, err := r.s.queries(ctx).InsertNotification(ctx, sqlcgen.InsertNotificationParams{
		ID: n.ID, UserID: n.UserID, GroupID: n.GroupID, Type: string(n.Type), Title: n.Title,
		Body: n.Body, Data: rawJSON(n.Data), Seq: n.Seq, DedupeKey: n.DedupeKey,
	})
	if err != nil {
		return nil, mapErr(err, "notification")
	}
	if len(rows) == 0 {
		return nil, nil // already delivered
	}
	out := toNotification(rows[0])
	return &out, nil
}

func (r *notifyRepo) List(ctx context.Context, f domain.NotificationFilter) ([]domain.Notification, error) {
	rows, err := r.s.queries(ctx).ListNotifications(ctx, sqlcgen.ListNotificationsParams{
		UserID: f.UserID, GroupID: f.GroupID, UnreadOnly: f.UnreadOnly,
		Before: f.Before, BeforeID: f.BeforeID, Lim: f.Limit,
	})
	if err != nil {
		return nil, mapErr(err, "notifications")
	}
	return toNotifications(rows), nil
}

func (r *notifyRepo) Unread(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID) (int, error) {
	n, err := r.s.queries(ctx).CountUnreadNotifications(ctx, sqlcgen.CountUnreadNotificationsParams{
		UserID: userID, GroupID: groupID,
	})
	return int(n), mapErr(err, "notifications")
}

func (r *notifyRepo) MarkRead(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID, ids []uuid.UUID, all bool) (int, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	n, err := r.s.queries(ctx).MarkNotificationsRead(ctx, sqlcgen.MarkNotificationsReadParams{
		UserID: userID, GroupID: groupID, Ids: ids, AllOfThem: all,
	})
	return int(n), mapErr(err, "notifications")
}

func (r *notifyRepo) DeleteOlderThan(ctx context.Context, t time.Time) (int64, error) {
	n, err := r.s.queries(ctx).DeleteOldNotifications(ctx, t)
	return n, mapErr(err, "notifications")
}

func (r *notifyRepo) Prefs(ctx context.Context, userID, groupID uuid.UUID) ([]domain.NotificationPref, error) {
	rows, err := r.s.queries(ctx).ListNotificationPrefs(ctx, sqlcgen.ListNotificationPrefsParams{
		UserID: userID, GroupID: groupID,
	})
	if err != nil {
		return nil, mapErr(err, "notification prefs")
	}
	out := make([]domain.NotificationPref, len(rows))
	for i, row := range rows {
		out[i] = domain.NotificationPref{
			UserID: row.UserID, GroupID: row.GroupID, Type: domain.NotificationType(row.Type), Enabled: row.Enabled,
		}
	}
	return out, nil
}

func (r *notifyRepo) SetPref(ctx context.Context, p domain.NotificationPref) error {
	err := r.s.queries(ctx).SetNotificationPref(ctx, sqlcgen.SetNotificationPrefParams{
		UserID: p.UserID, GroupID: p.GroupID, Type: string(p.Type), Enabled: p.Enabled,
	})
	return mapErr(err, "notification pref")
}

func (r *notifyRepo) Settings(ctx context.Context, userID uuid.UUID) (*domain.NotificationSettings, error) {
	row, err := r.s.queries(ctx).GetNotificationSettings(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // defaults
	}
	if err != nil {
		return nil, mapErr(err, "notification settings")
	}
	out := toSettings(row)
	return &out, nil
}

func (r *notifyRepo) SaveSettings(ctx context.Context, s domain.NotificationSettings) (*domain.NotificationSettings, error) {
	row, err := r.s.queries(ctx).SaveNotificationSettings(ctx, sqlcgen.SaveNotificationSettingsParams{
		UserID: s.UserID, PushEnabled: s.PushEnabled, QuietFrom: s.QuietFrom, QuietTo: s.QuietTo,
		UrgentInQuiet: s.UrgentInQuiet,
	})
	if err != nil {
		return nil, mapErr(err, "notification settings")
	}
	out := toSettings(row)
	return &out, nil
}

func (r *notifyRepo) Mutes(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID) ([]domain.NotificationMute, error) {
	rows, err := r.s.queries(ctx).ListNotificationMutes(ctx, sqlcgen.ListNotificationMutesParams{
		UserID: userID, GroupID: groupID,
	})
	if err != nil {
		return nil, mapErr(err, "notification mutes")
	}
	out := make([]domain.NotificationMute, len(rows))
	for i, row := range rows {
		out[i] = toMute(row)
	}
	return out, nil
}

func (r *notifyRepo) SetMute(ctx context.Context, m domain.NotificationMute) (*domain.NotificationMute, error) {
	row, err := r.s.queries(ctx).SetNotificationMute(ctx, sqlcgen.SetNotificationMuteParams{
		ID: m.ID, UserID: m.UserID, GroupID: m.GroupID, ScopeType: string(m.Scope), ScopeID: m.ScopeID, Until: m.Until,
	})
	if err != nil {
		return nil, mapErr(err, "notification mute")
	}
	out := toMute(row)
	return &out, nil
}

func (r *notifyRepo) DeleteMute(ctx context.Context, userID, groupID uuid.UUID, scope domain.MuteScope, scopeID string) error {
	n, err := r.s.queries(ctx).DeleteNotificationMute(ctx, sqlcgen.DeleteNotificationMuteParams{
		UserID: userID, GroupID: groupID, ScopeType: string(scope), ScopeID: scopeID,
	})
	if err != nil {
		return mapErr(err, "notification mute")
	}
	if n == 0 {
		return domain.NotFound("notification mute")
	}
	return nil
}

func (r *notifyRepo) DeleteExpiredMutes(ctx context.Context, now time.Time) (int64, error) {
	n, err := r.s.queries(ctx).DeleteExpiredNotificationMutes(ctx, now)
	return n, mapErr(err, "notification mutes")
}

func (r *notifyRepo) Message(ctx context.Context, id uuid.UUID) (*domain.NotifyMessage, error) {
	row, err := r.s.queries(ctx).GetNotifyMessage(ctx, id)
	if err != nil {
		return nil, mapErr(err, "message")
	}
	out := &domain.NotifyMessage{
		ID: row.ID, ThreadID: row.ThreadID, Seq: row.Seq, AuthorID: row.AuthorID, AuthorName: row.AuthorName,
		Body: row.Body, ReplyToID: row.ReplyToID, ReplyAuthorID: row.ReplyAuthorID,
		Deleted:    row.DeletedAt != nil || row.HiddenForAllAt != nil,
		TargetType: domain.ThreadTarget(row.TargetType), TargetID: row.TargetID,
		SubjectID: row.SubjectID, SubjectName: row.SubjectName,
	}
	if row.Title != nil {
		out.Title = *row.Title
	}
	return out, nil
}

func (r *notifyRepo) Task(ctx context.Context, id uuid.UUID) (*domain.NotifyTask, error) {
	row, err := r.s.queries(ctx).GetNotifyTask(ctx, id)
	if err != nil {
		return nil, mapErr(err, "task")
	}
	return &domain.NotifyTask{
		ID: row.ID, GroupID: row.GroupID, SubjectID: row.SubjectID, CreatedBy: row.CreatedBy,
		Title: row.Title, AssignMode: domain.TaskAssignMode(row.AssignMode),
		Visibility: domain.TaskVisibility(row.Visibility), DueAt: row.DueAt,
	}, nil
}

func (r *notifyRepo) UserName(ctx context.Context, id uuid.UUID) (string, error) {
	name, err := r.s.queries(ctx).GetNotifyUserName(ctx, id)
	return name, mapErr(err, "user")
}

func (r *notifyRepo) SubjectName(ctx context.Context, id uuid.UUID) (string, error) {
	name, err := r.s.queries(ctx).GetNotifySubjectName(ctx, id)
	return name, mapErr(err, "subject")
}

func (r *notifyRepo) MaterialOwner(ctx context.Context, id uuid.UUID) (*uuid.UUID, error) {
	owner, err := r.s.queries(ctx).GetNotifyMaterialOwner(ctx, id)
	return owner, mapErr(err, "material")
}

func (r *notifyRepo) PushTargets(ctx context.Context, userIDs []uuid.UUID) ([]domain.PushTarget, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := r.s.queries(ctx).ListPushTargets(ctx, userIDs)
	if err != nil {
		return nil, mapErr(err, "push targets")
	}
	out := make([]domain.PushTarget, 0, len(rows))
	for _, row := range rows {
		if row.PushToken == nil {
			continue
		}
		out = append(out, domain.PushTarget{
			DeviceID: row.DeviceID, UserID: row.UserID, Platform: row.Platform, Token: *row.PushToken,
			Timezone: row.Timezone,
			Settings: domain.NotificationSettings{
				UserID: row.UserID, PushEnabled: row.PushEnabled, QuietFrom: row.QuietFrom, QuietTo: row.QuietTo,
				UrgentInQuiet: row.UrgentInQuiet,
			},
		})
	}
	return out, nil
}

func (r *notifyRepo) Notifications(ctx context.Context, ids []uuid.UUID) ([]domain.Notification, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.s.queries(ctx).ListNotificationsForPush(ctx, ids)
	if err != nil {
		return nil, mapErr(err, "notifications")
	}
	return toNotifications(rows), nil
}

func (r *notifyRepo) RecordDelivery(ctx context.Context, d domain.NotificationDelivery) error {
	var msg *string
	if d.Error != "" {
		msg = &d.Error
	}
	err := r.s.queries(ctx).InsertNotificationDelivery(ctx, sqlcgen.InsertNotificationDeliveryParams{
		ID: d.ID, NotificationID: d.NotificationID, Channel: string(d.Channel), DeviceID: d.DeviceID,
		Status: string(d.Status), Error: msg, SentAt: d.SentAt,
	})
	return mapErr(err, "notification delivery")
}

func (r *notifyRepo) DisableDevicePush(ctx context.Context, deviceID uuid.UUID) error {
	return mapErr(r.s.queries(ctx).DisableDevicePush(ctx, deviceID), "device")
}

// idText renders an optional id the way mute scopes store it.
func idText(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}
