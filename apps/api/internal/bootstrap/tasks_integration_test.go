//go:build integration

package bootstrap_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"
)

type taskDTO struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	MyStatus      string     `json:"my_status"`
	Kind          string     `json:"kind"`
	Priority      string     `json:"priority"`
	AssignMode    string     `json:"assign_mode"`
	Visibility    string     `json:"visibility"`
	DueAt         *time.Time `json:"due_at"`
	Overdue       bool       `json:"overdue"`
	PinnedAt      *time.Time `json:"pinned_at"`
	SubjectID     *string    `json:"subject_id"`
	AssigneeIDs   []string   `json:"assignee_ids"`
	MaterialIDs   []string   `json:"material_ids"`
	DoneCount     int32      `json:"done_count"`
	AssignedCount int32      `json:"assigned_count"`
}

type taskListDTO struct {
	Items []taskDTO `json:"items"`
}

type boardDTO struct {
	ByStatus map[string]int32 `json:"by_status"`
	Open     int32            `json:"open"`
	Overdue  int32            `json:"overdue"`
	DueSoon  int32            `json:"due_soon"`
	Mine     int32            `json:"mine"`
}

// Tasks end to end: who may create, edit, pin and close; personal progress;
// private tasks; the board counters and the deadline scanner.
func TestTasksFlow(t *testing.T) {
	e := setup(t)
	c := e.c

	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Задачи ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)
	joinCode := created.Group["join_code"].(string)

	var student session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Студент", "invite_code": joinCode}, 201, &student)

	var subject struct {
		ID string `json:"id"`
	}
	c.do("POST", "/groups/"+groupID+"/subjects", owner.AccessToken, map[string]any{"name": "Базы данных"}, 201, &subject)

	// --- anyone may create a task; client_id makes it idempotent ---
	due := time.Now().UTC().Add(12 * time.Hour).Truncate(time.Minute)
	clientID := "11111111-1111-4111-8111-111111111111"
	body := map[string]any{
		"client_id": clientID, "title": "  Лабораторная   №3 ", "description": "Сдать отчёт",
		"subject_id": subject.ID, "kind": "TEACHER", "priority": "HIGH", "due_at": due.Format(time.RFC3339),
	}
	var task taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", student.AccessToken, body, 201, &task)
	if task.Title != "Лабораторная №3" || task.Status != "TODO" || task.Kind != "TEACHER" || task.Priority != "HIGH" {
		t.Fatalf("created task = %+v", task)
	}
	if task.DueAt == nil || !task.DueAt.Equal(due) || task.Overdue {
		t.Fatalf("due = %v (want %v)", task.DueAt, due)
	}
	var again taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", student.AccessToken, body, 201, &again)
	if again.ID != task.ID {
		t.Fatalf("repeat with the same client_id created another task: %s != %s", again.ID, task.ID)
	}

	// --- a private task is invisible to everyone else ---
	var private taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{
		"title": "Позвонить преподавателю", "kind": "PERSONAL", "assign_mode": "SELF", "visibility": "PRIVATE",
	}, 201, &private)
	c.do("GET", "/tasks/"+private.ID, student.AccessToken, nil, 404, nil)
	var studentList taskListDTO
	c.do("GET", "/groups/"+groupID+"/tasks", student.AccessToken, nil, 200, &studentList)
	if len(studentList.Items) != 1 || studentList.Items[0].ID != task.ID {
		t.Fatalf("student sees %d tasks: %+v", len(studentList.Items), studentList.Items)
	}
	// a private task cannot be pinned for the group
	c.do("POST", "/tasks/"+private.ID+"/pin", owner.AccessToken, nil, 422, nil)

	// --- assignees must be members of the group ---
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{
		"title": "Разделить доклад", "assign_mode": "SELECTED",
		"assignee_ids": []string{"22222222-2222-4222-8222-222222222222"},
	}, 422, nil)
	var shared taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{
		"title": "Разделить доклад", "assign_mode": "SELECTED", "assignee_ids": []string{student.User.ID},
	}, 201, &shared)
	if len(shared.AssigneeIDs) != 1 || shared.AssigneeIDs[0] != student.User.ID || shared.AssignedCount != 1 {
		t.Fatalf("assignees = %+v", shared)
	}

	// --- personal progress does not move the group status ---
	c.do("PATCH", "/tasks/"+shared.ID+"/me/status", student.AccessToken, map[string]any{"status": "DONE"}, 200, &shared)
	if shared.MyStatus != "DONE" || shared.Status != "TODO" || shared.DoneCount != 1 {
		t.Fatalf("after my status: %+v", shared)
	}

	// --- pinning is for the headman; the student is refused ---
	c.do("POST", "/tasks/"+task.ID+"/pin", student.AccessToken, nil, 403, nil)
	var pinned taskDTO
	c.do("POST", "/tasks/"+task.ID+"/pin", owner.AccessToken, nil, 200, &pinned)
	if pinned.PinnedAt == nil {
		t.Fatal("task was not pinned")
	}
	var ownerList taskListDTO
	c.do("GET", "/groups/"+groupID+"/tasks", owner.AccessToken, nil, 200, &ownerList)
	if len(ownerList.Items) != 3 || ownerList.Items[0].ID != task.ID {
		t.Fatalf("pinned task is not first: %+v", ownerList.Items)
	}

	// --- the author edits their own task, a stranger does not ---
	c.do("PATCH", "/tasks/"+shared.ID, student.AccessToken, map[string]any{"title": "Чужая задача"}, 403, nil)
	var edited taskDTO
	c.do("PATCH", "/tasks/"+task.ID, student.AccessToken, map[string]any{
		"title": "Лабораторная №3 (обновлено)", "subject_id": subject.ID, "kind": "TEACHER",
		"priority": "NORMAL", "due_at": due.Add(24 * time.Hour).Format(time.RFC3339),
	}, 200, &edited)
	if edited.Title != "Лабораторная №3 (обновлено)" || edited.Priority != "NORMAL" {
		t.Fatalf("edited = %+v", edited)
	}

	// --- the group status is for the author and the headman ---
	c.do("PATCH", "/tasks/"+shared.ID+"/status", student.AccessToken, map[string]any{"status": "DONE"}, 403, nil)
	var closed taskDTO
	c.do("PATCH", "/tasks/"+shared.ID+"/status", owner.AccessToken, map[string]any{"status": "DONE"}, 200, &closed)
	if closed.Status != "DONE" {
		t.Fatalf("status = %s", closed.Status)
	}

	// --- filters and the board ---
	var open taskListDTO
	c.do("GET", "/groups/"+groupID+"/tasks?open=true", owner.AccessToken, nil, 200, &open)
	if len(open.Items) != 2 {
		t.Fatalf("open tasks = %d: %+v", len(open.Items), open.Items)
	}
	var mine taskListDTO
	c.do("GET", "/groups/"+groupID+"/tasks?mine=true&open=true", student.AccessToken, nil, 200, &mine)
	if len(mine.Items) != 1 || mine.Items[0].ID != task.ID {
		t.Fatalf("student's open tasks = %+v", mine.Items)
	}
	var board boardDTO
	c.do("GET", "/groups/"+groupID+"/tasks/board", owner.AccessToken, nil, 200, &board)
	if board.Open != 2 || board.ByStatus["DONE"] != 1 || board.Overdue != 0 || board.DueSoon != 1 {
		t.Fatalf("board = %+v", board)
	}
	// the student closed the shared task for themselves, so it is not in "mine"
	c.do("GET", "/groups/"+groupID+"/tasks/board", student.AccessToken, nil, 200, &board)
	if board.Mine != 1 {
		t.Fatalf("student's board = %+v", board)
	}

	// --- the deadline scanner announces a task once ---
	soon := time.Now().UTC().Add(20 * time.Hour).Truncate(time.Minute)
	var upcoming taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{
		"title": "Курсовая", "due_at": soon.Format(time.RFC3339),
	}, 201, &upcoming)
	ctx := context.Background()
	n, err := e.svc.Tasks.ScanDeadlines(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("the scanner announced nothing")
	}
	repeat, err := e.svc.Tasks.ScanDeadlines(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repeat != 0 {
		t.Fatalf("the scanner announced %d tasks again", repeat)
	}
	kinds := eventKinds(t, c, groupID, owner.AccessToken)
	if kinds["task.created"] != 4 || kinds["task.status_changed"] != 1 || kinds["task.pinned"] != 1 || kinds["task.updated"] != 1 {
		t.Fatalf("task events = %v", kinds)
	}
	if kinds["task.due_soon"] == 0 {
		t.Fatalf("no deadline reminder among %v", kinds)
	}

	// --- delete ---
	c.do("DELETE", "/tasks/"+task.ID, owner.AccessToken, nil, 204, nil)
	c.do("GET", "/tasks/"+task.ID, owner.AccessToken, nil, 404, nil)
}

// eventKinds counts the kinds in the group log.
func eventKinds(t *testing.T, c *client, groupID, token string) map[string]int {
	t.Helper()
	var sync struct {
		Events []struct {
			Kind string `json:"kind"`
		} `json:"events"`
	}
	c.do("GET", fmt.Sprintf("/groups/%s/sync?since=0", groupID), token, nil, 200, &sync)
	kinds := map[string]int{}
	for _, e := range sync.Events {
		kinds[e.Kind]++
	}
	return kinds
}

// Attachments: set as a whole by whoever may edit the task, kept by edits
// that do not mention them, and limited to the group's live materials.
func TestTaskAttachments(t *testing.T) {
	e := setup(t)
	c := e.c

	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Вложения ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)
	var student session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Студент", "invite_code": created.Group["join_code"].(string)}, 201, &student)

	uploadPDF := func(token, name string, extra ...map[string]any) string {
		t.Helper()
		content := []byte("%PDF-1.4 " + name)
		body := map[string]any{"file_name": name, "size_bytes": len(content)}
		for _, e := range extra {
			for k, v := range e {
				body[k] = v
			}
		}
		var tk struct {
			UploadID string            `json:"upload_id"`
			Method   string            `json:"method"`
			URL      string            `json:"url"`
			Headers  map[string]string `json:"headers"`
		}
		c.do("POST", "/groups/"+groupID+"/materials/uploads", token, body, 201, &tk)
		if code, _, resp := c.raw(tk.Method, tk.URL, bytes.NewReader(content), tk.Headers); code != 204 {
			t.Fatalf("PUT %s: %d %s", name, code, resp)
		}
		var m struct {
			ID string `json:"id"`
		}
		c.do("POST", "/uploads/"+tk.UploadID+"/complete", token, nil, 200, &m)
		return m.ID
	}
	first := uploadPDF(student.AccessToken, "task1.pdf")
	second := uploadPDF(owner.AccessToken, "task2.pdf")

	var task taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", student.AccessToken, map[string]any{"title": "Реферат"}, 201, &task)
	if len(task.MaterialIDs) != 0 {
		t.Fatalf("new task materials = %v", task.MaterialIDs)
	}

	var attached taskDTO
	c.do("PUT", "/tasks/"+task.ID+"/materials", student.AccessToken, map[string]any{"material_ids": []string{first, second}}, 200, &attached)
	if len(attached.MaterialIDs) != 2 || attached.MaterialIDs[0] != first {
		t.Fatalf("attached = %v", attached.MaterialIDs)
	}

	// Editing the task without material_ids keeps the files.
	var edited taskDTO
	c.do("PATCH", "/tasks/"+task.ID, student.AccessToken, map[string]any{"title": "Реферат по БД"}, 200, &edited)
	if len(edited.MaterialIDs) != 2 {
		t.Fatalf("edit dropped attachments: %v", edited.MaterialIDs)
	}

	// Someone else's task: a student cannot attach.
	var other taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{"title": "Чужая"}, 201, &other)
	c.do("PUT", "/tasks/"+other.ID+"/materials", student.AccessToken, map[string]any{"material_ids": []string{first}}, 403, nil)

	// Unknown ids are skipped; an empty list detaches everything.
	var skipped taskDTO
	c.do("PUT", "/tasks/"+task.ID+"/materials", student.AccessToken, map[string]any{"material_ids": []string{first, "01a0c222-0000-7000-8000-000000000000"}}, 200, &skipped)
	if len(skipped.MaterialIDs) != 1 || skipped.MaterialIDs[0] != first {
		t.Fatalf("after unknown id = %v", skipped.MaterialIDs)
	}
	var cleared taskDTO
	c.do("PUT", "/tasks/"+task.ID+"/materials", student.AccessToken, map[string]any{"material_ids": []string{}}, 200, &cleared)
	if len(cleared.MaterialIDs) != 0 {
		t.Fatalf("after clearing = %v", cleared.MaterialIDs)
	}
	// --- a file kept in the task (D43) ---
	feed := func(token string) []string {
		t.Helper()
		var page struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		c.do("GET", "/groups/"+groupID+"/materials", token, nil, 200, &page)
		out := []string{}
		for _, it := range page.Items {
			out = append(out, it.ID)
		}
		return out
	}
	kept := uploadPDF(student.AccessToken, "draft.pdf", map[string]any{"task_id": task.ID, "task_only": true})
	c.do("GET", "/tasks/"+task.ID, student.AccessToken, nil, 200, &edited)
	if !contains(edited.MaterialIDs, kept) {
		t.Fatalf("uploaded file is not attached: %v", edited.MaterialIDs)
	}
	if contains(feed(owner.AccessToken), kept) || contains(feed(student.AccessToken), kept) {
		t.Fatal("a file kept in a task shows up in the feed")
	}
	var keptDTO struct {
		TaskID *string `json:"task_id"`
	}
	// The task is the group's, so every member reaches the file through it.
	c.do("GET", "/materials/"+kept, owner.AccessToken, nil, 200, &keptDTO)
	if keptDTO.TaskID == nil || *keptDTO.TaskID != task.ID {
		t.Fatalf("task_id = %v", keptDTO.TaskID)
	}
	// It cannot be carried into another task.
	var carried taskDTO
	c.do("PUT", "/tasks/"+other.ID+"/materials", owner.AccessToken, map[string]any{"material_ids": []string{kept, second}}, 200, &carried)
	if contains(carried.MaterialIDs, kept) || !contains(carried.MaterialIDs, second) {
		t.Fatalf("other task materials = %v", carried.MaterialIDs)
	}
	// Nor discussed on its own: its title must not surface in the list.
	c.do("POST", fmt.Sprintf("/groups/%s/discussions/MATERIAL/%s/messages", groupID, kept), student.AccessToken,
		map[string]any{"client_id": "33333333-3333-4333-8333-333333333333", "body": "?"}, 422, nil)

	// A private task's file is the author's alone.
	var private taskDTO
	c.do("POST", "/groups/"+groupID+"/tasks", owner.AccessToken, map[string]any{
		"title": "Личное", "kind": "PERSONAL", "assign_mode": "SELF", "visibility": "PRIVATE",
	}, 201, &private)
	secret := uploadPDF(owner.AccessToken, "secret.pdf", map[string]any{"task_id": private.ID, "task_only": true})
	c.do("GET", "/materials/"+secret, student.AccessToken, nil, 404, nil)
	c.do("GET", "/materials/"+secret+"/open", student.AccessToken, nil, 404, nil)

	// Upload rules: only whoever manages the task; task_only needs a task and no Drive.
	c.do("POST", "/groups/"+groupID+"/materials/uploads", student.AccessToken,
		map[string]any{"file_name": "x.pdf", "size_bytes": 10, "task_id": other.ID}, 403, nil)
	c.do("POST", "/groups/"+groupID+"/materials/uploads", student.AccessToken,
		map[string]any{"file_name": "x.pdf", "size_bytes": 10, "task_only": true}, 422, nil)
	c.do("POST", "/groups/"+groupID+"/materials/uploads", student.AccessToken,
		map[string]any{"file_name": "x.pdf", "size_bytes": 10, "task_id": task.ID, "task_only": true, "to_drive": true}, 422, nil)

	// Sharing: the owner of the file opens it to the group; the file stays attached.
	c.do("POST", "/materials/"+secret+"/share", student.AccessToken, nil, 404, nil)
	var shared struct {
		TaskID *string `json:"task_id"`
	}
	c.do("POST", "/materials/"+kept+"/share", student.AccessToken, nil, 200, &shared)
	if shared.TaskID != nil || !contains(feed(owner.AccessToken), kept) {
		t.Fatalf("shared file: task_id=%v, in feed=%v", shared.TaskID, contains(feed(owner.AccessToken), kept))
	}
	c.do("GET", "/tasks/"+task.ID, student.AccessToken, nil, 200, &edited)
	if !contains(edited.MaterialIDs, kept) {
		t.Fatal("sharing detached the file from its task")
	}
}
