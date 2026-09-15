// Package ids generates identifiers. All primary keys are UUIDv7 so they sort
// by creation time and stay index-friendly.
package ids

import (
	"crypto/rand"
	"encoding/base32"

	"github.com/google/uuid"
)

// New returns a fresh UUIDv7.
func New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// Code returns a random, URL-safe, human-typeable code of n characters
// (base32 without padding, lower-cased). Used for invite and join codes.
func Code(n int) string {
	if n <= 0 {
		n = 10
	}
	buf := make([]byte, (n*5+7)/8+1)
	if _, err := rand.Read(buf); err != nil {
		panic("ids: crypto/rand unavailable: " + err.Error())
	}
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	return toLower(s[:n])
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
