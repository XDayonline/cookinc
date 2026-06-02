// Package crypto provides authenticated encryption for cookinc sync payloads.
//
// Uses AES-256-GCM with a pair-derived or shared secret key.
// Key derivation: if pairing, X25519 + HKDF-SHA256;
// if shared secret, SHA-256 of the secret (32-byte minimum).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// Encrypt seals plaintext with AES-256-GCM using the given key.
// Returns nonce || ciphertext || tag.
// Key must be exactly 32 bytes.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes (got %d)", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens an AES-256-GCM sealed payload.
// Input must be nonce (12 bytes) || ciphertext || tag (16 bytes).
// Key must be exactly 32 bytes.
func Decrypt(key, sealed []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes (got %d)", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// DeriveKeyFromSecret hashes a shared secret to a 32-byte AES key.
// The secret MUST be at least 32 bytes.
func DeriveKeyFromSecret(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}
