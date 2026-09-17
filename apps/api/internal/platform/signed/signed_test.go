package signed

import (
	"errors"
	"strings"
	"testing"
	"time"

	"heatseeker/api/internal/domain"
)

type claims struct {
	Key string `json:"k"`
}

func TestSignVerify(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	s := New("secret-secret-secret-secret-secret", "media")
	token, err := s.Sign(claims{Key: "groups/1/a.pdf"}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	var got claims
	if err := s.Verify(token, now, &got); err != nil || got.Key != "groups/1/a.pdf" {
		t.Fatalf("verify: %v %+v", err, got)
	}
	if err := s.Verify(token, now.Add(time.Minute), &got); !errors.Is(err, domain.ErrGone) {
		t.Fatalf("expired token: %v", err)
	}
	other := New("secret-secret-secret-secret-secret", "stream")
	if err := other.Verify(token, now, &got); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("token accepted for another purpose: %v", err)
	}
	payload, sig, _ := strings.Cut(token, ".")
	tampered := payload[:len(payload)-2] + "AA." + sig
	if err := s.Verify(tampered, now, &got); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("tampered token: %v", err)
	}
	for _, bad := range []string{"", "abc", "a.b", "..."} {
		if err := s.Verify(bad, now, &got); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("Verify(%q) = %v", bad, err)
		}
	}
}
