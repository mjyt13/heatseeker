//go:build integration

package bootstrap_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// Sign-up with a client_id is idempotent: retries and double taps return the
// account created first instead of making new ones.
func TestRegisterIdempotent(t *testing.T) {
	e := setup(t)
	c := e.c

	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var group struct {
		Group struct {
			ID       string `json:"id"`
			JoinCode string `json:"join_code"`
		} `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Повторы " + uuid.NewString()[:8]}, 201, &group)

	clientID := uuid.NewString()
	body := map[string]any{"name": "Вася", "client_id": clientID}

	// Concurrent double taps: every response carries the same user.
	const taps = 4
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		status int
		body   []byte
	}
	results := make(chan result, taps)
	var wg sync.WaitGroup
	for range taps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := http.Post(c.base+"/auth/register", "application/json", bytes.NewReader(raw))
			if err != nil {
				results <- result{status: -1, body: []byte(err.Error())}
				return
			}
			defer func() { _ = res.Body.Close() }()
			data, _ := io.ReadAll(res.Body)
			results <- result{status: res.StatusCode, body: data}
		}()
	}
	wg.Wait()
	close(results)
	userIDs := map[string]bool{}
	for r := range results {
		if r.status != http.StatusCreated {
			t.Fatalf("concurrent register: status %d: %s", r.status, r.body)
		}
		var s session
		if err := json.Unmarshal(r.body, &s); err != nil {
			t.Fatal(err)
		}
		userIDs[s.User.ID] = true
	}
	if len(userIDs) != 1 {
		t.Fatalf("double taps created %d users", len(userIDs))
	}

	// A later retry (lost response) with an invite code: same user, joins the group.
	var retry session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Вася", "client_id": clientID, "invite_code": group.Group.JoinCode}, 201, &retry)
	if !userIDs[retry.User.ID] {
		t.Fatalf("retry returned another user %s", retry.User.ID)
	}
	if retry.Joined == nil || retry.Joined.Group.ID != group.Group.ID {
		t.Fatalf("retry did not join: %+v", retry.Joined)
	}
	c.do("GET", "/me", retry.AccessToken, nil, 200, nil)

	// Without client_id (or with another one) a new account is created.
	var other session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Вася", "client_id": uuid.NewString()}, 201, &other)
	if userIDs[other.User.ID] {
		t.Fatal("a different client_id reused the account")
	}

	// Once the account is secured, the client_id no longer signs in.
	c.do("POST", "/me/credentials", retry.AccessToken, map[string]any{"password": "correct horse"}, 200, nil)
	c.do("POST", "/auth/register", "", body, 409, nil)

	c.do("POST", "/auth/register", "", map[string]any{"name": "Вася", "client_id": "not-a-uuid"}, 422, nil)
}
