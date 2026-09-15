package auth

import "context"

// GoogleClaims are the fields of a verified Google ID token that we use.
type GoogleClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// GoogleVerifier validates Google ID tokens (implemented in adapters/google).
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (*GoogleClaims, error)
}
