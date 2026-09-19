// Package secretbox encrypts small secrets stored in the database (Google
// refresh tokens) with AES-256-GCM under APP_ENCRYPTION_KEY.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// Box seals and opens secrets. The associated data binds a ciphertext to its
// owner (for example a group id), so a sealed value copied to another row
// does not open.
type Box struct {
	aead cipher.AEAD
}

// New derives the AES key from the configured secret (SHA-256 of it).
func New(secret string) (*Box, error) {
	if len(secret) < 32 {
		return nil, errors.New("secretbox: the key must be at least 32 characters")
	}
	key := sha256.Sum256([]byte("heatseeker/secretbox/" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal returns nonce || ciphertext.
func (b *Box) Seal(plaintext, associated []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize(), b.aead.NonceSize()+len(plaintext)+b.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secretbox: nonce: %w", err)
	}
	return b.aead.Seal(nonce, nonce, plaintext, associated), nil
}

// Open reverses Seal. It fails when the key, the data or the associated data
// differ.
func (b *Box) Open(sealed, associated []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n+b.aead.Overhead() {
		return nil, errors.New("secretbox: sealed value is too short")
	}
	out, err := b.aead.Open(nil, sealed[:n], sealed[n:], associated)
	if err != nil {
		return nil, errors.New("secretbox: cannot decrypt (wrong APP_ENCRYPTION_KEY or corrupted value)")
	}
	return out, nil
}
