package node

import (
	"fmt"
	"os"
	"strings"

	pkgcrypto "v/pkg/crypto"
)

const sshSecretPrefix = "enc:"
const defaultSSHEncryptionSecret = "development-secret-change-in-production"

func sshEncryptionKey() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("V_JWT_SECRET"))
	if len(secret) < 32 || secret == defaultSSHEncryptionSecret {
		return nil, fmt.Errorf("SSH password storage requires a strong V_JWT_SECRET")
	}
	return pkgcrypto.DeriveKey(secret), nil
}

// EncryptSSHPassword encrypts an SSH password for storage.
func EncryptSSHPassword(plaintext string) (string, error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, sshSecretPrefix) {
		return plaintext, nil
	}
	key, err := sshEncryptionKey()
	if err != nil {
		return "", err
	}
	ciphertext, err := pkgcrypto.Encrypt(plaintext, key)
	if err != nil {
		return "", fmt.Errorf("encrypt ssh password: %w", err)
	}
	return sshSecretPrefix + ciphertext, nil
}

// DecryptSSHPassword decrypts a stored SSH password.
func DecryptSSHPassword(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, sshSecretPrefix) {
		return trimmed, nil
	}
	key, err := sshEncryptionKey()
	if err != nil {
		return "", err
	}
	plaintext, err := pkgcrypto.Decrypt(strings.TrimPrefix(trimmed, sshSecretPrefix), key)
	if err != nil {
		return "", fmt.Errorf("decrypt ssh password: %w", err)
	}
	return plaintext, nil
}
