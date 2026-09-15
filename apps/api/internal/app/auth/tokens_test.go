package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/clock"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	clk := &clock.Fixed{T: now}
	tk := NewTokens("0123456789abcdef0123456789abcdef", 15*time.Minute, 30, "pepper-pepper-pepper", clk)

	userID, deviceID := uuid.New(), uuid.New()
	raw, exp, err := tk.IssueAccess(userID, deviceID)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("exp = %v", exp)
	}
	claims, err := tk.ParseAccess(raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != userID.String() || claims.DeviceID != deviceID.String() {
		t.Fatalf("claims = %+v", claims)
	}

	// Expired.
	clk.T = now.Add(16 * time.Minute)
	if _, err := tk.ParseAccess(raw); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for expired token, got %v", err)
	}

	// Wrong secret.
	other := NewTokens("ffffffffffffffffffffffffffffffff", 15*time.Minute, 30, "pepper-pepper-pepper", &clock.Fixed{T: now})
	if _, err := other.ParseAccess(raw); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for wrong secret, got %v", err)
	}
}

func TestRefreshHashIsPeppered(t *testing.T) {
	a := NewTokens("0123456789abcdef0123456789abcdef", time.Minute, 1, "pepper-a", clock.Real{})
	b := NewTokens("0123456789abcdef0123456789abcdef", time.Minute, 1, "pepper-b", clock.Real{})
	raw, hash, _, err := a.NewRefresh()
	if err != nil {
		t.Fatal(err)
	}
	if a.HashRefresh(raw) != hash {
		t.Fatal("hash must be deterministic")
	}
	if b.HashRefresh(raw) == hash {
		t.Fatal("different pepper must produce a different hash")
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	ok, err = VerifyPassword(hash, "wrong")
	if err != nil || ok {
		t.Fatalf("wrong password verify = %v, %v", ok, err)
	}
	if _, err := VerifyPassword("not-a-hash", "x"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}
