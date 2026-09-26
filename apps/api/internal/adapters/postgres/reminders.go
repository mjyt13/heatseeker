package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

type announcementRepo struct{ s *Store }

func toAnnouncement(a sqlcgen.Announcement) domain.Announcement {
	return domain.Announcement{
		ID: a.ID, GroupID: a.GroupID, AuthorID: a.AuthorID, Title: a.Title, Body: a.Body,
		Urgent: a.Urgent, Pinned: a.Pinned, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		DeletedAt: a.DeletedAt,
	}
}

func (r *announcementRepo) Create(ctx context.Context, a domain.Announcement) (*domain.Announcement, error) {
	row, err := r.s.queries(ctx).CreateAnnouncement(ctx, sqlcgen.CreateAnnouncementParams{
		ID: a.ID, GroupID: a.GroupID, AuthorID: a.AuthorID, Title: a.Title, Body: a.Body,
		Urgent: a.Urgent, Pinned: a.Pinned,
	})
	if err != nil {
		return nil, mapErr(err, "announcement")
	}
	out := toAnnouncement(row)
	return &out, nil
}

func (r *announcementRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Announcement, error) {
	row, err := r.s.queries(ctx).GetAnnouncement(ctx, id)
	if err != nil {
		return nil, mapErr(err, "announcement")
	}
	out := toAnnouncement(row)
	return &out, nil
}

func (r *announcementRepo) List(ctx context.Context, groupID uuid.UUID, limit int32) ([]domain.AnnouncementView, error) {
	rows, err := r.s.queries(ctx).ListAnnouncements(ctx, sqlcgen.ListAnnouncementsParams{
		GroupID: groupID, Lim: limit,
	})
	if err != nil {
		return nil, mapErr(err, "announcements")
	}
	out := make([]domain.AnnouncementView, len(rows))
	for i, row := range rows {
		out[i] = domain.AnnouncementView{
			Announcement: toAnnouncement(row.Announcement), AuthorName: row.AuthorName,
		}
	}
	return out, nil
}

func (r *announcementRepo) Update(ctx context.Context, a domain.Announcement) (*domain.Announcement, error) {
	row, err := r.s.queries(ctx).UpdateAnnouncement(ctx, sqlcgen.UpdateAnnouncementParams{
		ID: a.ID, Title: a.Title, Body: a.Body, Urgent: a.Urgent, Pinned: a.Pinned,
	})
	if err != nil {
		return nil, mapErr(err, "announcement")
	}
	out := toAnnouncement(row)
	return &out, nil
}

func (r *announcementRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	n, err := r.s.queries(ctx).DeleteAnnouncement(ctx, id)
	if err != nil {
		return mapErr(err, "announcement")
	}
	if n == 0 {
		return domain.NotFound("announcement")
	}
	return nil
}

type reminderRepo struct{ s *Store }

func toReminder(r sqlcgen.Reminder) domain.Reminder {
	out := domain.Reminder{
		ID: r.ID, GroupID: r.GroupID, UserID: r.UserID, Title: r.Title, Note: r.Note,
		RemindAt: r.RemindAt, Repeat: domain.ReminderRepeat(r.Repeat), TargetID: r.TargetID,
		Status: domain.ReminderStatus(r.Status), LastFired: r.LastFiredAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.TargetType != nil {
		t := domain.ReminderTarget(*r.TargetType)
		out.TargetType = &t
	}
	return out
}

func targetText(t *domain.ReminderTarget) *string {
	if t == nil {
		return nil
	}
	s := string(*t)
	return &s
}

func (r *reminderRepo) Create(ctx context.Context, rem domain.Reminder) (*domain.Reminder, error) {
	row, err := r.s.queries(ctx).CreateReminder(ctx, sqlcgen.CreateReminderParams{
		ID: rem.ID, GroupID: rem.GroupID, UserID: rem.UserID, Title: rem.Title, Note: rem.Note,
		RemindAt: rem.RemindAt, Repeat: string(rem.Repeat),
		TargetType: targetText(rem.TargetType), TargetID: rem.TargetID,
	})
	if err != nil {
		return nil, mapErr(err, "reminder")
	}
	out := toReminder(row)
	return &out, nil
}

func (r *reminderRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Reminder, error) {
	row, err := r.s.queries(ctx).GetReminder(ctx, id)
	if err != nil {
		return nil, mapErr(err, "reminder")
	}
	out := toReminder(row)
	return &out, nil
}

func (r *reminderRepo) List(ctx context.Context, userID, groupID uuid.UUID, openOnly bool, limit int32) ([]domain.Reminder, error) {
	rows, err := r.s.queries(ctx).ListReminders(ctx, sqlcgen.ListRemindersParams{
		UserID: userID, GroupID: groupID, OpenOnly: openOnly, Lim: limit,
	})
	if err != nil {
		return nil, mapErr(err, "reminders")
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toReminder(row)
	}
	return out, nil
}

func (r *reminderRepo) Update(ctx context.Context, rem domain.Reminder) (*domain.Reminder, error) {
	row, err := r.s.queries(ctx).UpdateReminder(ctx, sqlcgen.UpdateReminderParams{
		ID: rem.ID, UserID: rem.UserID, Title: rem.Title, Note: rem.Note, RemindAt: rem.RemindAt,
		Repeat: string(rem.Repeat), TargetType: targetText(rem.TargetType), TargetID: rem.TargetID,
		Status: string(rem.Status), LastFiredAt: rem.LastFired,
	})
	if err != nil {
		return nil, mapErr(err, "reminder")
	}
	out := toReminder(row)
	return &out, nil
}

func (r *reminderRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	n, err := r.s.queries(ctx).DeleteReminder(ctx, sqlcgen.DeleteReminderParams{ID: id, UserID: userID})
	if err != nil {
		return mapErr(err, "reminder")
	}
	if n == 0 {
		return domain.NotFound("reminder")
	}
	return nil
}

func (r *reminderRepo) Due(ctx context.Context, now time.Time, limit int32) ([]domain.Reminder, error) {
	rows, err := r.s.queries(ctx).ListDueReminders(ctx, sqlcgen.ListDueRemindersParams{Now: now, Lim: limit})
	if err != nil {
		return nil, mapErr(err, "reminders")
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toReminder(row)
	}
	return out, nil
}
