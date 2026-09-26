//go:build integration

package bootstrap_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/platform/ids"
)

type notificationDTO struct {
	ID        string  `json:"id"`
	GroupID   string  `json:"group_id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	ReadAt    *string `json:"read_at"`
	CreatedAt string  `json:"created_at"`
}

type notificationsDTO struct {
	Items      []notificationDTO `json:"items"`
	Unread     int               `json:"unread"`
	NextCursor string            `json:"next_cursor"`
}

type prefDTO struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	Custom  bool   `json:"custom"`
}

// fanout runs the log reader the way the worker would.
func (e *env) fanout(t *testing.T, groupID string) int {
	t.Helper()
	id, err := uuid.Parse(groupID)
	if err != nil {
		t.Fatal(err)
	}
	n, err := e.svc.Notify.Fanout(context.Background(), id)
	if err != nil {
		t.Fatalf("fanout: %v", err)
	}
	return n
}

// hasType reports whether the list holds a notification of that type.
func hasType(items []notificationDTO, kind string) *notificationDTO {
	for i := range items {
		if items[i].Type == kind {
			return &items[i]
		}
	}
	return nil
}

// Notifications end to end: who hears about a message, switching a type off,
// muting a subject for a while, deadlines skipping the people who are done,
// and what actually reaches a device.
func TestNotificationsFlow(t *testing.T) {
	e := setup(t)
	c := e.c

	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Уведомления ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)
	joinCode := created.Group["join_code"].(string)

	var student session
	c.do("POST", "/auth/register", "", map[string]any{
		"name": "Студент", "invite_code": joinCode, "platform": "ANDROID",
	}, 201, &student)
	var quiet session
	c.do("POST", "/auth/register", "", map[string]any{
		"name": "Соня", "invite_code": joinCode, "platform": "ANDROID",
	}, 201, &quiet)

	var subject struct {
		ID string `json:"id"`
	}
	c.do("POST", "/groups/"+groupID+"/subjects", owner.AccessToken, map[string]any{"name": "Базы данных"}, 201, &subject)
	// The first pass starts at the head: nothing that happened before the
	// reader existed is replayed.
	if n := e.fanout(t, groupID); n != 0 {
		t.Fatalf("first fan-out replayed %d events", n)
	}
	c.do("POST", "/me/notifications/read", student.AccessToken, map[string]any{"all": true}, 200, nil)
	c.do("POST", "/me/notifications/read", quiet.AccessToken, map[string]any{"all": true}, 200, nil)
	e.push.drain()

	subjectPath := fmt.Sprintf("/groups/%s/discussions/SUBJECT/%s", groupID, subject.ID)

	// --- a message in a subject discussion reaches everybody but its author (D48) ---
	var first messageDTO
	c.do("POST", subjectPath+"/messages", student.AccessToken, map[string]any{
		"client_id": ids.New().String(), "body": "Когда лабораторная?",
	}, 201, &first)
	if n := e.fanout(t, groupID); n != 2 {
		t.Fatalf("message fan-out wrote %d notifications, want 2", n)
	}
	var mine notificationsDTO
	c.do("GET", "/me/notifications?group_id="+groupID, owner.AccessToken, nil, 200, &mine)
	msg := hasType(mine.Items, "MESSAGE_NEW")
	if msg == nil || msg.Title != "Базы данных" || msg.Body != "Студент: Когда лабораторная?" {
		t.Fatalf("owner notifications = %+v", mine.Items)
	}
	if mine.Unread != 1 {
		t.Fatalf("owner unread = %d, want 1", mine.Unread)
	}
	var author notificationsDTO
	c.do("GET", "/me/notifications?group_id="+groupID, student.AccessToken, nil, 200, &author)
	if len(author.Items) != 0 {
		t.Fatalf("author notified about their own message: %+v", author.Items)
	}

	// --- a second pass over the log changes nothing ---
	if n := e.fanout(t, groupID); n != 0 {
		t.Fatalf("second fan-out wrote %d notifications, want 0", n)
	}

	// --- marking read empties the badge ---
	var read struct {
		Marked int `json:"marked"`
		Unread int `json:"unread"`
	}
	c.do("POST", "/me/notifications/read", owner.AccessToken, map[string]any{"ids": []string{msg.ID}}, 200, &read)
	if read.Marked != 1 || read.Unread != 0 {
		t.Fatalf("mark read = %+v", read)
	}

	// --- a reply reaches the author of the message it answers ---
	c.do("POST", subjectPath+"/messages", owner.AccessToken, map[string]any{
		"client_id": ids.New().String(), "body": "В четверг", "reply_to_id": first.ID,
	}, 201, nil)
	e.fanout(t, groupID)
	c.do("GET", "/me/notifications?group_id="+groupID, student.AccessToken, nil, 200, &author)
	reply := hasType(author.Items, "MESSAGE_REPLY")
	if reply == nil || reply.Title != "Ответ от Староста" || reply.Body != "В четверг" {
		t.Fatalf("reply notification = %+v", author.Items)
	}
	// The reply took the place of the ordinary "new message" line.
	if hasType(author.Items, "MESSAGE_NEW") != nil {
		t.Fatalf("author got both a reply and a message line: %+v", author.Items)
	}

	// --- switching a type off silences it ---
	var prefs struct {
		Items []prefDTO `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/notification-prefs", quiet.AccessToken, nil, 200, &prefs)
	if len(prefs.Items) == 0 {
		t.Fatal("no preferences offered")
	}
	for _, p := range prefs.Items {
		if p.Type == "MESSAGE_NEW" && (!p.Enabled || p.Custom) {
			t.Fatalf("MESSAGE_NEW default = %+v, want enabled and not custom", p)
		}
		if p.Type == "MEMBER_JOINED" && p.Enabled {
			t.Fatalf("MEMBER_JOINED is on by default: %+v", p)
		}
	}
	c.do("PUT", "/groups/"+groupID+"/notification-prefs", quiet.AccessToken, map[string]any{
		"items": []map[string]any{{"type": "MESSAGE_NEW", "enabled": false}},
	}, 200, &prefs)
	// Clear what arrived before the switch, so only new lines count.
	c.do("POST", "/me/notifications/read", quiet.AccessToken, map[string]any{"all": true}, 200, nil)
	c.do("POST", subjectPath+"/messages", student.AccessToken, map[string]any{
		"client_id": ids.New().String(), "body": "Понял, спасибо",
	}, 201, nil)
	e.fanout(t, groupID)
	var sleeper notificationsDTO
	c.do("GET", "/me/notifications?group_id="+groupID+"&unread_only=true", quiet.AccessToken, nil, 200, &sleeper)
	if hasType(sleeper.Items, "MESSAGE_NEW") != nil {
		t.Fatalf("a switched-off type still arrived: %+v", sleeper.Items)
	}

	// --- a mute silences a subject for a while ---
	c.do("PUT", "/groups/"+groupID+"/notification-prefs", quiet.AccessToken, map[string]any{
		"items": []map[string]any{{"type": "MESSAGE_NEW", "enabled": true}},
	}, 200, &prefs)
	var mute struct {
		ScopeType string `json:"scope_type"`
		ScopeID   string `json:"scope_id"`
	}
	c.do("POST", "/groups/"+groupID+"/notification-mutes", quiet.AccessToken, map[string]any{
		"scope_type": "SUBJECT", "scope_id": subject.ID,
		"until": time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339),
	}, 200, &mute)
	if mute.ScopeType != "SUBJECT" || mute.ScopeID != subject.ID {
		t.Fatalf("mute = %+v", mute)
	}
	c.do("POST", "/me/notifications/read", quiet.AccessToken, map[string]any{"all": true}, 200, nil)
	c.do("POST", "/me/notifications/read", owner.AccessToken, map[string]any{"all": true}, 200, nil)
	// A mute always ends: the past and "forever" are both refused.
	c.do("POST", "/groups/"+groupID+"/notification-mutes", quiet.AccessToken, map[string]any{
		"scope_type": "SUBJECT", "scope_id": subject.ID,
		"until": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
	}, 422, nil)
	c.do("POST", subjectPath+"/messages", student.AccessToken, map[string]any{
		"client_id": ids.New().String(), "body": "Ещё вопрос",
	}, 201, nil)
	e.fanout(t, groupID)
	c.do("GET", "/me/notifications?group_id="+groupID+"&unread_only=true", quiet.AccessToken, nil, 200, &sleeper)
	if hasType(sleeper.Items, "MESSAGE_NEW") != nil {
		t.Fatalf("a muted subject still arrived: %+v", sleeper.Items)
	}
	// The owner, who muted nothing, did hear about it.
	c.do("GET", "/me/notifications?group_id="+groupID+"&unread_only=true", owner.AccessToken, nil, 200, &mine)
	if hasType(mine.Items, "MESSAGE_NEW") == nil {
		t.Fatalf("owner missed the message: %+v", mine.Items)
	}
	c.do("DELETE", fmt.Sprintf("/groups/%s/notification-mutes?scope_type=SUBJECT&scope_id=%s", groupID, subject.ID),
		quiet.AccessToken, nil, 204, nil)
	c.do("DELETE", fmt.Sprintf("/groups/%s/notification-mutes?scope_type=SUBJECT&scope_id=%s", groupID, subject.ID),
		quiet.AccessToken, nil, 404, nil)

	// --- a task for everybody reaches everybody but its author ---
	var task struct {
		ID string `json:"id"`
	}
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{
		"client_id": ids.New().String(), "title": "Сдать отчёт", "kind": "TEACHER",
		"assign_mode": "ALL", "due_at": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	}, 201, &task)
	e.fanout(t, groupID)
	c.do("GET", "/me/notifications?group_id="+groupID+"&unread_only=true", student.AccessToken, nil, 200, &author)
	created0 := hasType(author.Items, "TASK_CREATED")
	if created0 == nil || created0.Title != "Новая задача" {
		t.Fatalf("task notification = %+v", author.Items)
	}

	// --- push: a device with a token is reached, quiet hours are not ---
	var devices struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	c.do("GET", "/me/devices", student.AccessToken, nil, 200, &devices)
	if len(devices.Items) == 0 {
		t.Fatal("no device registered at sign-up")
	}
	c.do("PUT", "/me/devices/"+devices.Items[0].ID+"/push", student.AccessToken, map[string]any{
		"provider": "EXPO", "token": "ExponentPushToken[student]",
	}, 200, nil)
	c.do("GET", "/me/devices", quiet.AccessToken, nil, 200, &devices)
	c.do("PUT", "/me/devices/"+devices.Items[0].ID+"/push", quiet.AccessToken, map[string]any{
		"provider": "EXPO", "token": "ExponentPushToken[quiet]",
	}, 200, nil)
	// Quiet hours around the clock: nothing may disturb this member.
	c.do("PUT", "/me/notification-settings", quiet.AccessToken, map[string]any{
		"push_enabled": true, "quiet_from": 0, "quiet_to": 1439,
	}, 200, nil)
	e.push.drain()
	c.do("POST", "/me/notifications/read", student.AccessToken, map[string]any{"all": true}, 200, nil)
	c.do("POST", "/me/notifications/read", quiet.AccessToken, map[string]any{"all": true}, 200, nil)

	c.do("POST", subjectPath+"/messages", owner.AccessToken, map[string]any{
		"client_id": ids.New().String(), "body": "Лабораторную перенесли",
	}, 201, nil)
	e.fanout(t, groupID)
	ids2 := notificationIDs(t, e, student.AccessToken)
	if _, err := e.svc.Notify.Deliver(context.Background(), ids2); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	sent := e.push.drain()
	if len(sent) != 1 || sent[0].Body != "Староста: Лабораторную перенесли" {
		t.Fatalf("push = %+v", sent)
	}
	quietIDs := notificationIDs(t, e, quiet.AccessToken)
	if _, err := e.svc.Notify.Deliver(context.Background(), quietIDs); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if sent := e.push.drain(); len(sent) != 0 {
		t.Fatalf("push during quiet hours: %+v", sent)
	}
}

// notificationIDs collects the unread notifications of one member.
func notificationIDs(t *testing.T, e *env, token string) []uuid.UUID {
	t.Helper()
	var page notificationsDTO
	e.c.do("GET", "/me/notifications?unread_only=true", token, nil, 200, &page)
	out := make([]uuid.UUID, 0, len(page.Items))
	for _, item := range page.Items {
		id, err := uuid.Parse(item.ID)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}
