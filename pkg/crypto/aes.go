// Package crypto provides AES-GCM encryption and decryption utilities
// for protecting sensitive data such as SSH passwords.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

var (
	ErrEmptyPlaintext  = errors.New("plaintext must not be empty")
	ErrEmptyCiphertext = errors.New("ciphertext must not be empty")
	ErrInvalidKey      = errors.New("key must be 16, 24, or 32 bytes")
	ErrCiphertextShort = errors.New("ciphertext too short")
	ErrDecodeBase64    = errors.New("failed to decode base64 ciphertext")
)

// DeriveKey creates a 32-byte AES-256 key from an arbitrary secret
// string by computing its SHA-256 hash.
func DeriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// Encrypt encrypts plaintext using AES-GCM with the provided key.
// A random nonce is generated and prepended to the ciphertext.
// The result is returned as a base64-encoded string.
// Key must be 16, 24, or 32 bytes long.
func Encrypt(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", ErrEmptyPlaintext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal appends the encrypted+authenticated ciphertext after the nonce.
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded ciphertext that was produced by Encrypt.
// It expects the nonce to be prepended to the ciphertext.
// Key must be 16, 24, or 32 bytes long.
func Decrypt(ciphertext string, key []byte) (string, error) {
	if ciphertext == "" {
		return "", ErrEmptyCiphertext
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecodeBase64, err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrCiphertextShort
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong key or corrupted data): %w", err)
	}

	return string(plaintext), nil
}
