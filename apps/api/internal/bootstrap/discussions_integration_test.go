//go:build integration

package bootstrap_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"heatseeker/api/internal/platform/ids"
)

type messageDTO struct {
	ID             string  `json:"id"`
	ThreadID       string  `json:"thread_id"`
	Seq            int64   `json:"seq"`
	AuthorName     string  `json:"author_name"`
	Body           string  `json:"body"`
	Mine           bool    `json:"mine"`
	Deleted        bool    `json:"deleted"`
	HiddenForAll   bool    `json:"hidden_for_all"`
	HiddenForAllBy *string `json:"hidden_for_all_by"`
	HiddenByMe     bool    `json:"hidden_by_me"`
	EditedAt       *string `json:"edited_at"`
	ReplyTo        *struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	} `json:"reply_to"`
	Thread *struct {
		TargetType string `json:"target_type"`
	} `json:"thread"`
}

type messagePageDTO struct {
	Thread *struct {
		ID string `json:"id"`
	} `json:"thread"`
	Items       []messageDTO `json:"items"`
	HasMore     bool         `json:"has_more"`
	LastReadSeq int64        `json:"last_read_seq"`
	LastSeq     int64        `json:"last_seq"`
}

type discussionDTO struct {
	TargetType   string  `json:"target_type"`
	TargetID     string  `json:"target_id"`
	SubjectID    *string `json:"subject_id"`
	Title        *string `json:"title"`
	ThreadID     *string `json:"thread_id"`
	MessageCount int32   `json:"message_count"`
	Unread       int32   `json:"unread"`
	NestedUnread int32   `json:"nested_unread"`
	LastMessage  *struct {
		AuthorName string `json:"author_name"`
		Body       string `json:"body"`
	} `json:"last_message"`
}

type discussionsDTO struct {
	General  discussionDTO   `json:"general"`
	Subjects []discussionDTO `json:"subjects"`
	Others   []discussionDTO `json:"others"`
	Unread   int32           `json:"unread"`
}

// Discussions end to end: lazy threads, idempotent sending, replies, unread
// counts, editing and deleting, hiding for oneself and moderation, and what
// the event log keeps.
func TestDiscussionsFlow(t *testing.T) {
	e := setup(t)
	c := e.c
	runID := ids.Code(6)

	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Обсуждения ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)
	joinCode := created.Group["join_code"].(string)

	var student session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Студент", "invite_code": joinCode}, 201, &student)

	var invite struct {
		Code string `json:"code"`
	}
	c.do("POST", "/groups/"+groupID+"/invites", owner.AccessToken, map[string]any{"roles": []string{"GUEST"}, "max_uses": 1}, 201, &invite)
	var guest session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Гость", "invite_code": invite.Code}, 201, &guest)

	var subject struct {
		ID string `json:"id"`
	}
	c.do("POST", "/groups/"+groupID+"/subjects", owner.AccessToken, map[string]any{"name": "Базы данных"}, 201, &subject)

	subjectPath := fmt.Sprintf("/groups/%s/discussions/SUBJECT/%s", groupID, subject.ID)
	generalPath := fmt.Sprintf("/groups/%s/discussions/GENERAL/%s", groupID, groupID)

	// --- an empty discussion reads fine and writes nothing ---
	var page messagePageDTO
	c.do("GET", subjectPath+"/messages", student.AccessToken, nil, 200, &page)
	if page.Thread != nil || len(page.Items) != 0 {
		t.Fatalf("empty discussion = %+v", page)
	}
	c.do("POST", subjectPath+"/read", student.AccessToken, map[string]any{}, 204, nil)
	var overview discussionsDTO
	c.do("GET", "/groups/"+groupID+"/discussions", student.AccessToken, nil, 200, &overview)
	if overview.General.TargetID != groupID || overview.General.ThreadID != nil || len(overview.Subjects) != 1 ||
		overview.Subjects[0].TargetID != subject.ID || overview.Subjects[0].ThreadID != nil {
		t.Fatalf("overview before messages = %+v", overview)
	}

	// --- the first message creates the thread; client_id makes it idempotent ---
	clientID := "22222222-2222-4222-8222-222222222222"
	var first messageDTO
	c.do("POST", subjectPath+"/messages", student.AccessToken, map[string]any{
		"client_id": clientID, "body": "  Кто понял **третью** нормальную форму?\r\n",
	}, 201, &first)
	if first.Body != "Кто понял **третью** нормальную форму?" || !first.Mine || first.AuthorName != "Студент" || first.Seq == 0 {
		t.Fatalf("first message = %+v", first)
	}
	var again messageDTO
	c.do("POST", subjectPath+"/messages", student.AccessToken, map[string]any{"client_id": clientID, "body": "другой текст"}, 201, &again)
	if again.ID != first.ID || again.Body != first.Body {
		t.Fatalf("repeat created another message: %+v", again)
	}

	// --- guests read but do not write; bad targets are rejected ---
	c.do("POST", subjectPath+"/messages", guest.AccessToken, map[string]any{"client_id": ids.New().String(), "body": "привет"}, 403, nil)
	c.do("GET", subjectPath+"/messages", guest.AccessToken, nil, 200, nil)
	c.do("GET", fmt.Sprintf("/groups/%s/discussions/GENERAL/%s/messages", groupID, subject.ID), student.AccessToken, nil, 404, nil)
	c.do("GET", fmt.Sprintf("/groups/%s/discussions/LESSON/%s/messages", groupID, subject.ID), student.AccessToken, nil, 422, nil)
	c.do("POST", subjectPath+"/messages", student.AccessToken, map[string]any{"client_id": ids.New().String(), "body": "   "}, 422, nil)

	// --- a reply from the owner; the student now has one unread message ---
	var reply messageDTO
	c.do("POST", subjectPath+"/messages", owner.AccessToken, map[string]any{
		"client_id": ids.New().String(), "body": "Я объясню на паре", "reply_to_id": first.ID,
	}, 201, &reply)
	if reply.ReplyTo == nil || reply.ReplyTo.ID != first.ID || reply.ReplyTo.Body != first.Body {
		t.Fatalf("reply = %+v", reply)
	}
	c.do("POST", generalPath+"/messages", owner.AccessToken, map[string]any{"client_id": ids.New().String(), "body": "Всем привет"}, 201, nil)
	// A reply must point into the same thread.
	c.do("POST", generalPath+"/messages", owner.AccessToken, map[string]any{
		"client_id": ids.New().String(), "body": "не туда", "reply_to_id": first.ID,
	}, 422, nil)

	c.do("GET", "/groups/"+groupID+"/discussions", student.AccessToken, nil, 200, &overview)
	if overview.Unread != 2 || overview.General.Unread != 1 || overview.Subjects[0].Unread != 1 ||
		overview.Subjects[0].MessageCount != 2 || overview.Subjects[0].LastMessage == nil ||
		overview.Subjects[0].LastMessage.Body != "Я объясню на паре" {
		t.Fatalf("student overview = %+v", overview)
	}
	c.do("GET", subjectPath+"/messages", student.AccessToken, nil, 200, &page)
	if len(page.Items) != 2 || page.Items[0].ID != first.ID || page.Items[1].ID != reply.ID ||
		page.LastReadSeq != first.Seq || page.LastSeq != reply.Seq {
		t.Fatalf("thread page = %+v", page)
	}
	c.do("POST", subjectPath+"/read", student.AccessToken, map[string]any{}, 204, nil)
	c.do("GET", "/groups/"+groupID+"/discussions", student.AccessToken, nil, 200, &overview)
	if overview.Unread != 1 || overview.Subjects[0].Unread != 0 {
		t.Fatalf("after reading = %+v", overview)
	}

	// --- paging by seq ---
	for i := range 3 {
		c.do("POST", subjectPath+"/messages", student.AccessToken, map[string]any{
			"client_id": ids.New().String(), "body": fmt.Sprintf("сообщение %d", i),
		}, 201, nil)
	}
	c.do("GET", subjectPath+"/messages?limit=2", student.AccessToken, nil, 200, &page)
	if len(page.Items) != 2 || !page.HasMore || page.Items[1].Body != "сообщение 2" {
		t.Fatalf("newest page = %+v", page)
	}
	var older messagePageDTO
	c.do("GET", fmt.Sprintf("%s/messages?limit=2&before_seq=%d", subjectPath, page.Items[0].Seq), student.AccessToken, nil, 200, &older)
	if len(older.Items) != 2 || !older.HasMore || older.Items[1].Body != "сообщение 0" {
		t.Fatalf("older page = %+v", older)
	}
	var newer messagePageDTO
	c.do("GET", fmt.Sprintf("%s/messages?after_seq=%d", subjectPath, reply.Seq), student.AccessToken, nil, 200, &newer)
	if len(newer.Items) != 3 || newer.HasMore || newer.Items[0].Body != "сообщение 0" {
		t.Fatalf("newer page = %+v", newer)
	}
	c.do("GET", fmt.Sprintf("%s/messages?after_seq=1&before_seq=2", subjectPath), student.AccessToken, nil, 422, nil)

	// --- edit and delete: only the author ---
	var edited messageDTO
	c.do("PATCH", "/messages/"+first.ID, student.AccessToken, map[string]any{"body": "Кто понял 3НФ?"}, 200, &edited)
	if edited.Body != "Кто понял 3НФ?" || edited.EditedAt == nil {
		t.Fatalf("edited = %+v", edited)
	}
	c.do("PATCH", "/messages/"+first.ID, owner.AccessToken, map[string]any{"body": "чужое"}, 403, nil)
	c.do("DELETE", "/messages/"+reply.ID, student.AccessToken, nil, 403, nil)
	lastID := newer.Items[2].ID
	c.do("DELETE", "/messages/"+lastID, student.AccessToken, nil, 204, nil)
	c.do("DELETE", "/messages/"+lastID, student.AccessToken, nil, 204, nil)
	var deleted messageDTO
	for _, m := range fetchMessages(t, c, subjectPath, student.AccessToken, "") {
		if m.ID == lastID {
			deleted = m
		}
	}
	if !deleted.Deleted || deleted.Body != "" {
		t.Fatalf("deleted message = %+v", deleted)
	}
	// Only the author brings it back.
	c.do("POST", "/messages/"+lastID+"/restore", owner.AccessToken, nil, 403, nil)
	var restored messageDTO
	c.do("POST", "/messages/"+lastID+"/restore", student.AccessToken, nil, 200, &restored)
	if restored.Deleted || restored.Body != "сообщение 2" {
		t.Fatalf("restored message = %+v", restored)
	}
	c.do("DELETE", "/messages/"+lastID, student.AccessToken, nil, 204, nil)

	// --- hiding for oneself folds the text away for that reader only ---
	var hidden messageDTO
	c.do("POST", "/messages/"+reply.ID+"/hide", student.AccessToken, nil, 200, &hidden)
	if !hidden.HiddenByMe || hidden.Body == "" {
		t.Fatalf("hide response = %+v", hidden)
	}
	if m := findMessage(t, fetchMessages(t, c, subjectPath, student.AccessToken, ""), reply.ID); !m.HiddenByMe || m.Body != "" {
		t.Fatalf("hidden message without include_hidden = %+v", m)
	}
	if m := findMessage(t, fetchMessages(t, c, subjectPath, student.AccessToken, "include_hidden=true"), reply.ID); m.Body == "" {
		t.Fatalf("hidden message with include_hidden = %+v", m)
	}
	if m := findMessage(t, fetchMessages(t, c, subjectPath, owner.AccessToken, ""), reply.ID); m.HiddenByMe || m.Body == "" {
		t.Fatalf("owner sees the message hidden by someone else = %+v", m)
	}
	var hiddenList struct {
		Items []messageDTO `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/messages/hidden", student.AccessToken, nil, 200, &hiddenList)
	if len(hiddenList.Items) != 1 || hiddenList.Items[0].ID != reply.ID || hiddenList.Items[0].Body == "" ||
		hiddenList.Items[0].Thread == nil || hiddenList.Items[0].Thread.TargetType != "SUBJECT" {
		t.Fatalf("hidden by me = %+v", hiddenList.Items)
	}
	c.do("DELETE", "/messages/"+reply.ID+"/hide", student.AccessToken, nil, 200, nil)
	c.do("GET", "/groups/"+groupID+"/messages/hidden", student.AccessToken, nil, 200, &hiddenList)
	if len(hiddenList.Items) != 0 {
		t.Fatalf("hidden list after unhide = %+v", hiddenList.Items)
	}

	// --- moderation needs the role and a secured account ---
	c.do("POST", "/messages/"+first.ID+"/moderate", student.AccessToken, nil, 403, nil)
	c.do("POST", "/messages/"+first.ID+"/moderate", owner.AccessToken, nil, 403, nil)
	c.do("POST", "/me/credentials", owner.AccessToken, map[string]any{"email": "mod-" + runID + "@example.com", "password": "correct horse"}, 200, nil)
	var moderated messageDTO
	c.do("POST", "/messages/"+first.ID+"/moderate", owner.AccessToken, nil, 200, &moderated)
	if !moderated.HiddenForAll || moderated.HiddenForAllBy == nil || moderated.Body == "" {
		t.Fatalf("moderator's view = %+v", moderated)
	}
	// The author still sees the text, without knowing who hid it.
	if m := findMessage(t, fetchMessages(t, c, subjectPath, student.AccessToken, ""), first.ID); !m.HiddenForAll || m.Body == "" || m.HiddenForAllBy != nil {
		t.Fatalf("author's view of a moderated message = %+v", m)
	}
	// Everyone else sees a stub, also in quotes.
	for _, m := range fetchMessages(t, c, subjectPath, guest.AccessToken, "") {
		if m.ID == first.ID && (m.Body != "" || !m.HiddenForAll) {
			t.Fatalf("guest's view of a moderated message = %+v", m)
		}
		if m.ID == reply.ID && (m.ReplyTo == nil || m.ReplyTo.Body != "") {
			t.Fatalf("guest's view of a quote of a moderated message = %+v", m)
		}
	}
	c.do("PATCH", "/messages/"+first.ID, student.AccessToken, map[string]any{"body": "исправлю"}, 409, nil)
	c.do("DELETE", "/messages/"+first.ID+"/moderate", owner.AccessToken, nil, 200, nil)
	if m := findMessage(t, fetchMessages(t, c, subjectPath, guest.AccessToken, ""), first.ID); m.HiddenForAll || m.Body == "" {
		t.Fatalf("restored message = %+v", m)
	}

	// --- material and task discussions count towards their subject ---
	var task struct {
		ID string `json:"id"`
	}
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{"title": "Лабораторная №1", "subject_id": subject.ID}, 201, &task)
	taskPath := fmt.Sprintf("/groups/%s/discussions/TASK/%s", groupID, task.ID)
	c.do("POST", taskPath+"/messages", owner.AccessToken, map[string]any{"client_id": ids.New().String(), "body": "Срок перенесли"}, 201, nil)
	c.do("GET", "/groups/"+groupID+"/discussions", student.AccessToken, nil, 200, &overview)
	if overview.Subjects[0].NestedUnread != 1 || len(overview.Others) != 1 || overview.Others[0].TargetType != "TASK" ||
		overview.Others[0].Title == nil || *overview.Others[0].Title != "Лабораторная №1" {
		t.Fatalf("overview with a task discussion = %+v", overview)
	}
	var private struct {
		ID string `json:"id"`
	}
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{
		"title": "Личное", "kind": "PERSONAL", "assign_mode": "SELF", "visibility": "PRIVATE",
	}, 201, &private)
	privatePath := fmt.Sprintf("/groups/%s/discussions/TASK/%s/messages", groupID, private.ID)
	c.do("GET", privatePath, student.AccessToken, nil, 404, nil)
	c.do("POST", privatePath, owner.AccessToken, map[string]any{"client_id": ids.New().String(), "body": "себе"}, 422, nil)

	// --- the log names messages but never keeps their text (D40) ---
	var sync struct {
		Events []struct {
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		} `json:"events"`
	}
	c.do("GET", fmt.Sprintf("/groups/%s/sync?since=0", groupID), student.AccessToken, nil, 200, &sync)
	kinds := map[string]int{}
	for _, ev := range sync.Events {
		kinds[ev.Kind]++
		if strings.HasPrefix(ev.Kind, "message.") && strings.Contains(string(ev.Payload), "3НФ") {
			t.Fatalf("event %s carries the text: %s", ev.Kind, ev.Payload)
		}
	}
	// 2 subject + 1 general + 3 paging + 1 task message; repeats create nothing.
	if kinds["message.created"] != 7 || kinds["message.updated"] != 1 || kinds["message.deleted"] != 2 || kinds["message.undeleted"] != 1 ||
		kinds["message.hidden"] != 1 || kinds["message.restored"] != 1 {
		t.Fatalf("message events = %v", kinds)
	}
	var activity struct {
		Items []struct {
			Kind string `json:"kind"`
		} `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/activity", owner.AccessToken, nil, 200, &activity)
	for _, it := range activity.Items {
		if it.Kind == "message.created" || it.Kind == "message.updated" || it.Kind == "message.deleted" {
			t.Fatalf("activity feed shows %s", it.Kind)
		}
	}
}

func fetchMessages(t *testing.T, c *client, path, token, query string) []messageDTO {
	t.Helper()
	var page messagePageDTO
	c.do("GET", path+"/messages?limit=200&"+query, token, nil, 200, &page)
	return page.Items
}

func findMessage(t *testing.T, items []messageDTO, id string) messageDTO {
	t.Helper()
	for _, m := range items {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("message %s not in %+v", id, items)
	return messageDTO{}
}
