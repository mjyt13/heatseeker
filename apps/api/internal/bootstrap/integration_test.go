//go:build integration

package bootstrap_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"heatseeker/api/internal/platform/ids"
)

// End-to-end flow over real Postgres: register → create group → subjects/tags
// → second user joins by code → permissions → sync → token rotation → login.
//
// Run with: TEST_DATABASE_URL=... go test -tags integration ./internal/bootstrap/
func TestCoreFlow(t *testing.T) {
	e := setup(t)
	c := e.c

	// The dev database is reused between runs: emails must be unique per run.
	runID := ids.Code(6)
	email := "maria-" + runID + "@example.com"

	// --- register first user (name only) ---
	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "  Мария  Иванова ", "platform": "ANDROID"}, 201, &owner)
	if owner.User.Name != "Мария Иванова" || owner.User.Secured {
		t.Fatalf("unexpected user: %+v", owner.User)
	}
	if owner.AccessToken == "" || owner.RefreshToken == "" {
		t.Fatal("tokens missing")
	}

	// --- create group ---
	var created struct {
		Group      map[string]any `json:"group"`
		Membership struct {
			Roles       []string `json:"roles"`
			Permissions []string `json:"permissions"`
		} `json:"membership"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Магистратура ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)
	joinCode := created.Group["join_code"].(string)
	if !contains(created.Membership.Roles, "OWNER") || !contains(created.Membership.Roles, "STUDENT") {
		t.Fatalf("creator roles = %v", created.Membership.Roles)
	}
	if !contains(created.Membership.Permissions, "member.manage") {
		t.Fatalf("owner permissions = %v", created.Membership.Permissions)
	}

	// --- my groups ---
	var mine struct {
		Items []json.RawMessage `json:"items"`
	}
	c.do("GET", "/me/groups", owner.AccessToken, nil, 200, &mine)
	if len(mine.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(mine.Items))
	}

	// --- subject + auto tag ---
	var subject struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	c.do("POST", "/groups/"+groupID+"/subjects", owner.AccessToken, map[string]any{
		"name": "Математический анализ", "short_name": "Матан", "aliases": []string{"матан", "matan"},
	}, 201, &subject)
	var tagsRes struct {
		Items []struct {
			Kind      string  `json:"kind"`
			SubjectID *string `json:"subject_id"`
			Slug      string  `json:"slug"`
		} `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/tags", owner.AccessToken, nil, 200, &tagsRes)
	if len(tagsRes.Items) != 1 || tagsRes.Items[0].Kind != "SUBJECT" || tagsRes.Items[0].SubjectID == nil || *tagsRes.Items[0].SubjectID != subject.ID {
		t.Fatalf("subject tag not created: %+v", tagsRes.Items)
	}
	if tagsRes.Items[0].Slug != "matematicheskiy-analiz" {
		t.Fatalf("slug = %s", tagsRes.Items[0].Slug)
	}
	var quick struct {
		Items []struct {
			Key   string `json:"key"`
			Label string `json:"label"`
		} `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/quick-tags", owner.AccessToken, nil, 200, &quick)
	if len(quick.Items) != 4 || quick.Items[0].Key != "system:mine" || quick.Items[3].Label != "Матан" {
		t.Fatalf("quick tags = %+v", quick.Items)
	}

	// --- second user joins by group code during registration ---
	var student session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Пётр", "invite_code": joinCode}, 201, &student)
	if student.Joined == nil || student.Joined.Group.ID != groupID {
		t.Fatalf("student did not join: %+v", student.Joined)
	}
	if !contains(student.Joined.Membership.Roles, "STUDENT") || contains(student.Joined.Membership.Roles, "OWNER") {
		t.Fatalf("student roles = %v", student.Joined.Membership.Roles)
	}
	// joining again is idempotent
	c.do("POST", "/groups/join/"+joinCode, student.AccessToken, nil, 200, nil)

	var members struct {
		Items    []json.RawMessage `json:"items"`
		Extended bool              `json:"extended"`
	}
	c.do("GET", "/groups/"+groupID+"/members", student.AccessToken, nil, 200, &members)
	if len(members.Items) != 2 || members.Extended {
		t.Fatalf("members as student: n=%d extended=%v", len(members.Items), members.Extended)
	}

	// --- permissions: student cannot manage subjects, owner (light account) cannot manage members ---
	c.do("POST", "/groups/"+groupID+"/subjects", student.AccessToken, map[string]any{"name": "Физика"}, 403, nil)
	c.do("PUT", "/groups/"+groupID+"/members/"+student.User.ID+"/roles", owner.AccessToken, map[string]any{"roles": []string{"HEADMAN", "STUDENT"}}, 403, nil)

	// --- secure the owner account → member management unlocks ---
	var secured user
	c.do("POST", "/me/credentials", owner.AccessToken, map[string]any{"email": "Maria-" + runID + "@Example.com", "password": "correct horse"}, 200, &secured)
	if !secured.Secured || secured.Email == nil || *secured.Email != email {
		t.Fatalf("secured user = %+v", secured)
	}
	c.do("PUT", "/groups/"+groupID+"/members/"+student.User.ID+"/roles", owner.AccessToken, map[string]any{"roles": []string{"HEADMAN", "STUDENT"}}, 200, nil)
	// headman may now manage subjects
	c.do("POST", "/groups/"+groupID+"/subjects", student.AccessToken, map[string]any{"name": "Физика"}, 201, nil)

	// --- invite with roles, preview without auth ---
	var invite struct {
		Code string `json:"code"`
	}
	c.do("POST", "/groups/"+groupID+"/invites", owner.AccessToken, map[string]any{"roles": []string{"GUEST"}, "max_uses": 1}, 201, &invite)
	var preview struct {
		Roles       []string `json:"roles"`
		MemberCount int64    `json:"member_count"`
		ViaInvite   bool     `json:"via_invite"`
	}
	c.do("GET", "/groups/join/"+invite.Code, "", nil, 200, &preview)
	if !preview.ViaInvite || preview.MemberCount != 2 || !contains(preview.Roles, "GUEST") {
		t.Fatalf("preview = %+v", preview)
	}
	var guest session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Гость", "invite_code": invite.Code}, 201, &guest)
	// invite exhausted
	c.do("POST", "/auth/register", "", map[string]any{"name": "Ещё гость", "invite_code": invite.Code}, 410, nil)
	// guest cannot write
	c.do("POST", "/groups/"+groupID+"/tags", guest.AccessToken, map[string]any{"name": "x"}, 403, nil)

	// --- sync: the log contains everything that happened ---
	var sync struct {
		Events []struct {
			Kind string `json:"kind"`
			Seq  int64  `json:"seq"`
		} `json:"events"`
		NextSeq int64 `json:"next_seq"`
		Latest  int64 `json:"latest"`
	}
	c.do("GET", "/groups/"+groupID+"/sync?since=0", student.AccessToken, nil, 200, &sync)
	kinds := map[string]int{}
	for i, e := range sync.Events {
		kinds[e.Kind]++
		if e.Seq != int64(i+1) {
			t.Fatalf("seq gap at %d: %+v", i, sync.Events)
		}
	}
	if kinds["group.created"] != 1 || kinds["member.joined"] != 3 || kinds["subject.created"] != 2 || kinds["member.roles_changed"] != 1 {
		t.Fatalf("event kinds = %v", kinds)
	}
	if sync.NextSeq != sync.Latest || sync.Latest != int64(len(sync.Events)) {
		t.Fatalf("cursors: next=%d latest=%d n=%d", sync.NextSeq, sync.Latest, len(sync.Events))
	}
	var delta struct {
		Events []json.RawMessage `json:"events"`
	}
	c.do("GET", fmt.Sprintf("/groups/%s/sync?since=%d", groupID, sync.NextSeq), student.AccessToken, nil, 200, &delta)
	if len(delta.Events) != 0 {
		t.Fatalf("expected empty delta, got %d", len(delta.Events))
	}

	// --- refresh rotation and reuse detection ---
	var rotated session
	c.do("POST", "/auth/refresh", "", map[string]any{"refresh_token": student.RefreshToken}, 200, &rotated)
	if rotated.RefreshToken == student.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	c.do("GET", "/me", rotated.AccessToken, nil, 200, nil)
	c.do("POST", "/auth/refresh", "", map[string]any{"refresh_token": student.RefreshToken}, 401, nil) // reuse → all sessions revoked
	c.do("POST", "/auth/refresh", "", map[string]any{"refresh_token": rotated.RefreshToken}, 401, nil)

	// --- login with the new credentials ---
	var login session
	c.do("POST", "/auth/login", "", map[string]any{"email": email, "password": "correct horse"}, 200, &login)
	if login.User.ID != owner.User.ID {
		t.Fatal("login returned a different user")
	}
	c.do("POST", "/auth/login", "", map[string]any{"email": email, "password": "wrong"}, 401, nil)

	// --- search by name and join directly (D33) ---
	var stream struct {
		Group struct {
			ID string `json:"id"`
		} `json:"group"`
	}
	streamName := "Поток Поиска " + runID
	c.do("POST", "/groups", login.AccessToken, map[string]any{"name": streamName}, 201, &stream)
	var anna session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Анна"}, 201, &anna)
	type searchHit struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		MemberCount int64  `json:"member_count"`
		IsMember    bool   `json:"is_member"`
	}
	var found struct {
		Items []searchHit `json:"items"`
	}
	search := func(q string) []searchHit {
		found.Items = nil
		c.do("GET", "/groups/search?q="+url.QueryEscape(q), anna.AccessToken, nil, 200, &found)
		return found.Items
	}
	hits := search("  ПОИСКА " + runID + " ")
	if len(hits) != 1 || hits[0].ID != stream.Group.ID || hits[0].Name != streamName || hits[0].MemberCount != 1 || hits[0].IsMember {
		t.Fatalf("search hits = %+v", hits)
	}
	if hits := search("%" + runID); len(hits) != 0 {
		t.Fatalf("LIKE metacharacters must be literal, got %+v", hits)
	}
	c.do("POST", "/groups/"+stream.Group.ID+"/join", anna.AccessToken, nil, 200, nil)
	c.do("POST", "/groups/"+stream.Group.ID+"/join", anna.AccessToken, nil, 200, nil) // idempotent
	if hits := search(runID); len(hits) != 1 || !hits[0].IsMember || hits[0].MemberCount != 2 {
		t.Fatalf("after join = %+v", hits)
	}
	// invitation-only groups are neither listed nor joinable
	c.do("PATCH", "/groups/"+stream.Group.ID, login.AccessToken, map[string]any{"join_policy": "INVITE"}, 200, nil)
	if hits := search(runID); len(hits) != 0 {
		t.Fatalf("invite-only group listed: %+v", hits)
	}
	var bob session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Боб"}, 201, &bob)
	c.do("POST", "/groups/"+stream.Group.ID+"/join", bob.AccessToken, nil, 403, nil)
	c.do("GET", "/groups/search?q="+runID, "", nil, 401, nil)

	// --- unauthenticated access is rejected ---
	c.do("GET", "/me", "", nil, 401, nil)
	c.do("GET", "/groups/"+groupID, "", nil, 401, nil)
}

type user struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Email   *string `json:"email"`
	Secured bool    `json:"secured"`
}

type session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         user   `json:"user"`
	Joined       *struct {
		Group struct {
			ID string `json:"id"`
		} `json:"group"`
		Membership struct {
			Roles []string `json:"roles"`
		} `json:"membership"`
	} `json:"joined"`
}
