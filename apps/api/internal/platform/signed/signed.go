// Package signed issues short-lived HMAC tokens for URLs that clients open
// without an Authorization header: media streams, local-storage uploads and
// downloads. Tokens are bound to a purpose so one kind cannot be replayed as
// another.
package signed

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"heatseeker/api/internal/domain"
)

// Signer signs and verifies tokens for one purpose.
type Signer struct {
	key []byte
}

// New derives a purpose-specific key from secret.
func New(secret, purpose string) *Signer {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("heatseeker/signed/" + purpose))
	return &Signer{key: mac.Sum(nil)}
}

type envelope struct {
	Data json.RawMessage `json:"d"`
	Exp  int64           `json:"e"`
}

var enc = base64.RawURLEncoding

// Sign encodes claims with an expiry.
func (s *Signer) Sign(claims any, expires time.Time) (string, error) {
	data, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	body, err := json.Marshal(envelope{Data: data, Exp: expires.Unix()})
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	payload := enc.EncodeToString(body)
	return payload + "." + enc.EncodeToString(s.mac(payload)), nil
}

// Verify checks the signature and expiry and decodes claims into out.
func (s *Signer) Verify(token string, now time.Time, out any) error {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return fmt.Errorf("%w: malformed link token", domain.ErrUnauthorized)
	}
	got, err := enc.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac(payload)) {
		return fmt.Errorf("%w: invalid link token", domain.ErrUnauthorized)
	}
	raw, err := enc.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("%w: malformed link token", domain.ErrUnauthorized)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%w: malformed link token", domain.ErrUnauthorized)
	}
	if now.Unix() >= env.Exp {
		return fmt.Errorf("%w: link expired", domain.ErrGone)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("%w: malformed link token", domain.ErrUnauthorized)
	}
	return nil
}

func (s *Signer) mac(payload string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(payload))
	return m.Sum(nil)
}
