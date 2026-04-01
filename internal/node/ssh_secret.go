package node

import (
	"fmt"
	"os"
	"strings"

	pkgcrypto "v/pkg/crypto"
)

const sshSecretPrefix = "enc:"
const defaultSSHEncryptionSecret = "development-secret-change-in-production"

func sshEncryptionSecret() string {
	secret := strings.TrimSpace(os.Getenv("V_JWT_SECRET"))
	if len(secret) < 32 {
		return defaultSSHEncryptionSecret
	}
	return secret
}

func sshEncryptionKey() []byte {
	return pkgcrypto.DeriveKey(sshEncryptionSecret())
}

// EncryptSSHPassword encrypts an SSH password for storage.
func EncryptSSHPassword(plaintext string) (string, error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, sshSecretPrefix) {
		return plaintext, nil
	}
	ciphertext, err := pkgcrypto.Encrypt(plaintext, sshEncryptionKey())
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
	plaintext, err := pkgcrypto.Decrypt(strings.TrimPrefix(trimmed, sshSecretPrefix), sshEncryptionKey())
	if err != nil {
		return "", fmt.Errorf("decrypt ssh password: %w", err)
	}
	return plaintext, nil
}
