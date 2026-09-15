package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/clock"
)

// AccessClaims are the JWT claims of an access token.
type AccessClaims struct {
	jwt.RegisteredClaims
	DeviceID string `json:"dev,omitempty"`
}

// Tokens issues and verifies access and refresh tokens.
type Tokens struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	pepper     string
	clock      clock.Clock
}

// NewTokens builds a token issuer.
func NewTokens(secret string, accessTTL time.Duration, refreshTTLDays int, pepper string, clk clock.Clock) *Tokens {
	return &Tokens{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: time.Duration(refreshTTLDays) * 24 * time.Hour,
		pepper:     pepper,
		clock:      clk,
	}
}

// IssueAccess signs a short-lived HS256 access token.
func (t *Tokens) IssueAccess(userID, deviceID uuid.UUID) (string, time.Time, error) {
	now := t.clock.Now()
	exp := now.Add(t.accessTTL)
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
		DeviceID: deviceID.String(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, exp, nil
}

// ParseAccess verifies an access token and returns its claims.
func (t *Tokens) ParseAccess(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(t.clock.Now))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUnauthorized, err)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject", domain.ErrUnauthorized)
	}
	return claims, nil
}

// NewRefresh generates an opaque refresh token and its storage hash.
func (t *Tokens) NewRefresh() (raw, hash string, expiresAt time.Time, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, t.HashRefresh(raw), t.clock.Now().Add(t.refreshTTL), nil
}

// HashRefresh hashes a raw refresh token with the server pepper.
func (t *Tokens) HashRefresh(raw string) string {
	sum := sha256.Sum256([]byte(t.pepper + ":" + raw))
	return hex.EncodeToString(sum[:])
}
