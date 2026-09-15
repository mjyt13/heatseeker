// Package google verifies Google Sign-In ID tokens.
package google

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/api/idtoken"

	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/domain"
)

// Verifier validates ID tokens against a set of accepted audiences (web,
// Android and iOS client IDs).
type Verifier struct {
	audiences []string
}

// NewVerifier builds a verifier. With no audiences, Verify always fails so
// that a misconfigured deployment cannot accept arbitrary tokens.
func NewVerifier(audiences []string) *Verifier {
	return &Verifier{audiences: audiences}
}

// Verify implements auth.GoogleVerifier.
func (v *Verifier) Verify(ctx context.Context, raw string) (*auth.GoogleClaims, error) {
	if len(v.audiences) == 0 {
		return nil, fmt.Errorf("%w: google sign-in is not configured", domain.ErrInvalid)
	}
	var lastErr error
	for _, aud := range v.audiences {
		payload, err := idtoken.Validate(ctx, raw, aud)
		if err != nil {
			lastErr = err
			continue
		}
		return &auth.GoogleClaims{
			Subject:       payload.Subject,
			Email:         claimString(payload.Claims, "email"),
			EmailVerified: claimBool(payload.Claims, "email_verified"),
			Name:          claimString(payload.Claims, "name"),
			Picture:       claimString(payload.Claims, "picture"),
		}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no audience matched")
	}
	return nil, fmt.Errorf("%w: invalid google token: %v", domain.ErrUnauthorized, lastErr)
}

func claimString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func claimBool(m map[string]any, key string) bool {
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}
