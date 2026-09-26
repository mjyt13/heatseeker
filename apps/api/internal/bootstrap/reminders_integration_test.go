//go:build integration

package bootstrap_test

import (
	"context"
	"testing"
	"time"

	"heatseeker/api/internal/platform/ids"
)

type announcementDTO struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Urgent     bool   `json:"urgent"`
	Pinned     bool   `json:"pinned"`
	AuthorName string `json:"author_name"`
}

type reminderDTO struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	RemindAt string `json:"remind_at"`
	Repeat   string `json:"repeat"`
	Status   string `json:"status"`
}

// Announcements reach the whole group and, when urgent, the phone even during
// quiet hours; reminders are personal and fire on their own schedule.
func TestAnnouncementsAndReminders(t *testing.T) {
	e := setup(t)
	c := e.c
	ctx := context.Background()

	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Объявления ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)
	joinCode := created.Group["join_code"].(string)
	// Announcing needs a secured account (authz.SecuredActions).
	c.do("POST", "/me/credentials", owner.AccessToken, map[string]any{
		"email": "headman-" + ids.Code(8) + "@example.com", "password": "very-secret-pass",
	}, 200, nil)

	var student session
	c.do("POST", "/auth/register", "", map[string]any{
		"name": "Студент", "invite_code": joinCode, "platform": "ANDROID",
	}, 201, &student)
	e.fanout(t, groupID)

	// --- a student may read announcements but not make them ---
	c.do("POST", "/groups/"+groupID+"/announcements", student.AccessToken, map[string]any{
		"title": "Пара отменена",
	}, 403, nil)

	var announcement announcementDTO
	c.do("POST", "/groups/"+groupID+"/announcements", owner.AccessToken, map[string]any{
		"title": "Завтра пары нет", "body": "Преподаватель заболел", "urgent": true, "pinned": true,
	}, 201, &announcement)
	if !announcement.Urgent || announcement.Title != "Завтра пары нет" {
		t.Fatalf("announcement = %+v", announcement)
	}
	var list struct {
		Items []announcementDTO `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/announcements", student.AccessToken, nil, 200, &list)
	if len(list.Items) != 1 || list.Items[0].AuthorName != "Староста" {
		t.Fatalf("announcements = %+v", list.Items)
	}

	// --- everybody but the author hears about it ---
	e.fanout(t, groupID)
	var mine notificationsDTO
	c.do("GET", "/me/notifications?group_id="+groupID, student.AccessToken, nil, 200, &mine)
	heard := hasType(mine.Items, "ANNOUNCEMENT")
	if heard == nil || heard.Title != "Срочное объявление" || heard.Body != "Завтра пары нет: Преподаватель заболел" {
		t.Fatalf("announcement notification = %+v", mine.Items)
	}
	var authors notificationsDTO
	c.do("GET", "/me/notifications?group_id="+groupID, owner.AccessToken, nil, 200, &authors)
	if hasType(authors.Items, "ANNOUNCEMENT") != nil {
		t.Fatalf("author was told about their own announcement: %+v", authors.Items)
	}

	// --- quiet hours hold back the push, urgent or not (D51) ---
	var devices struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	c.do("GET", "/me/devices", student.AccessToken, nil, 200, &devices)
	c.do("PUT", "/me/devices/"+devices.Items[0].ID+"/push", student.AccessToken, map[string]any{
		"provider": "EXPO", "token": "ExponentPushToken[student]",
	}, 200, nil)
	c.do("PUT", "/me/notification-settings", student.AccessToken, map[string]any{
		"push_enabled": true, "quiet_from": 0, "quiet_to": 1439,
	}, 200, nil)
	e.push.drain()
	if _, err := e.svc.Notify.Deliver(ctx, notificationIDs(t, e, student.AccessToken)); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if sent := e.push.drain(); len(sent) != 0 {
		t.Fatalf("urgent announcement disturbed quiet hours: %+v", sent)
	}
	// The list still has it: only the push was held back.
	c.do("GET", "/me/notifications?group_id="+groupID+"&unread_only=true", student.AccessToken, nil, 200, &mine)
	if hasType(mine.Items, "ANNOUNCEMENT") == nil {
		t.Fatalf("announcement missing from the list: %+v", mine.Items)
	}
	// Allowed explicitly, an urgent announcement does come through the night.
	c.do("PUT", "/me/notification-settings", student.AccessToken, map[string]any{
		"push_enabled": true, "quiet_from": 0, "quiet_to": 1439, "urgent_in_quiet": true,
	}, 200, nil)
	if _, err := e.svc.Notify.Deliver(ctx, notificationIDs(t, e, student.AccessToken)); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if sent := e.push.drain(); len(sent) != 1 || sent[0].Title != "Срочное объявление" {
		t.Fatalf("urgent push with urgent_in_quiet = %+v", sent)
	}
	// An ordinary announcement still waits for the morning.
	c.do("POST", "/me/notifications/read", student.AccessToken, map[string]any{"all": true}, 200, nil)
	c.do("POST", "/groups/"+groupID+"/announcements", owner.AccessToken, map[string]any{
		"title": "Не срочно", "urgent": false,
	}, 201, nil)
	e.fanout(t, groupID)
	if _, err := e.svc.Notify.Deliver(ctx, notificationIDs(t, e, student.AccessToken)); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if sent := e.push.drain(); len(sent) != 0 {
		t.Fatalf("ordinary announcement disturbed quiet hours: %+v", sent)
	}
	c.do("PUT", "/me/notification-settings", student.AccessToken, map[string]any{"push_enabled": true}, 200, nil)
	c.do("POST", "/me/notifications/read", student.AccessToken, map[string]any{"all": true}, 200, nil)

	// --- a personal reminder fires for its owner and nobody else ---
	var reminder reminderDTO
	c.do("POST", "/groups/"+groupID+"/reminders", student.AccessToken, map[string]any{
		"title": "Скинуть материалы преподавателю",
		// Already due: the scanner takes it on its next run.
		"remind_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}, 201, &reminder)
	if reminder.Status != "SCHEDULED" || reminder.Repeat != "NONE" {
		t.Fatalf("reminder = %+v", reminder)
	}
	n, err := e.svc.Reminders.Scan(ctx)
	if err != nil || n != 1 {
		t.Fatalf("scan = %d, %v", n, err)
	}
	c.do("GET", "/me/notifications?group_id="+groupID+"&unread_only=true", student.AccessToken, nil, 200, &mine)
	fired := hasType(mine.Items, "REMINDER")
	if fired == nil || fired.Body != "Скинуть материалы преподавателю" {
		t.Fatalf("reminder notification = %+v", mine.Items)
	}
	c.do("GET", "/me/notifications?group_id="+groupID+"&unread_only=true", owner.AccessToken, nil, 200, &authors)
	if hasType(authors.Items, "REMINDER") != nil {
		t.Fatalf("somebody else's reminder leaked: %+v", authors.Items)
	}
	// A second run does not repeat it.
	if n, err := e.svc.Reminders.Scan(ctx); err != nil || n != 0 {
		t.Fatalf("second scan = %d, %v", n, err)
	}

	// --- snooze puts it back on the schedule, "done" closes it ---
	var after reminderDTO
	c.do("POST", "/reminders/"+reminder.ID+"/snooze", student.AccessToken, map[string]any{"minutes": 120}, 200, &after)
	if after.Status != "SCHEDULED" {
		t.Fatalf("snoozed = %+v", after)
	}
	when, err := time.Parse(time.RFC3339, after.RemindAt)
	if err != nil || time.Until(when) < time.Hour {
		t.Fatalf("snooze moved to %v (%v)", after.RemindAt, err)
	}
	c.do("POST", "/reminders/"+reminder.ID+"/done", student.AccessToken, map[string]any{"done": true}, 200, &after)
	if after.Status != "DONE" {
		t.Fatalf("done = %+v", after)
	}

	// --- a repeating reminder comes back tomorrow ---
	var daily reminderDTO
	c.do("POST", "/groups/"+groupID+"/reminders", student.AccessToken, map[string]any{
		"title": "Проверить расписание", "repeat": "DAILY",
		"remind_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}, 201, &daily)
	if _, err := e.svc.Reminders.Scan(ctx); err != nil {
		t.Fatalf("scan: %v", err)
	}
	var open struct {
		Items []reminderDTO `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/reminders?open_only=true", student.AccessToken, nil, 200, &open)
	var repeated *reminderDTO
	for i := range open.Items {
		if open.Items[i].ID == daily.ID {
			repeated = &open.Items[i]
		}
	}
	if repeated == nil || repeated.Status != "SCHEDULED" {
		t.Fatalf("repeating reminder = %+v", open.Items)
	}
	next, err := time.Parse(time.RFC3339, repeated.RemindAt)
	if err != nil || time.Until(next) < 23*time.Hour {
		t.Fatalf("daily reminder moved to %v (%v)", repeated.RemindAt, err)
	}

	// --- somebody else's reminder is not theirs to touch ---
	c.do("POST", "/reminders/"+daily.ID+"/done", owner.AccessToken, map[string]any{"done": true}, 404, nil)
	c.do("DELETE", "/reminders/"+daily.ID, student.AccessToken, nil, 204, nil)
}
