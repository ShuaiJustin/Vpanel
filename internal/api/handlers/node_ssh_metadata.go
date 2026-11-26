package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"v/internal/node"
)

const invalidNodeSSHPrivateKeyMessage = "SSH 私钥无效，请提供有效的未加密私钥"

func validNodeSSHPrivateKey(privateKey string) bool {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey == "" {
		return true
	}
	_, err := ssh.ParsePrivateKey([]byte(privateKey))
	return err == nil
}

type nodeSSHMetadataInput struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
}

func persistNodeSSHMetadata(ctx context.Context, nodeService *node.Service, nodeID int64, input nodeSSHMetadataInput) error {
	if nodeService == nil {
		return fmt.Errorf("node service is unavailable")
	}

	keyPath, err := persistNodeSSHPrivateKey(nodeID, input.PrivateKey)
	if err != nil {
		return err
	}

	previousKeyPath, err := nodeService.UpdateSSHConfig(ctx, nodeID, input.Host, input.Port, input.Username, input.Password, keyPath)
	if err != nil {
		if keyPath != "" {
			_ = os.Remove(keyPath)
		}
		return err
	}

	// Remove only keys managed by the panel for this node, including legacy filenames.
	if keyPath != "" && filepath.Dir(previousKeyPath) == filepath.Dir(keyPath) {
		legacyName := fmt.Sprintf("node-%d.key", nodeID)
		name := filepath.Base(previousKeyPath)
		if name == legacyName || (strings.HasPrefix(name, fmt.Sprintf("node-%d-", nodeID)) && strings.HasSuffix(name, ".key")) {
			_ = os.Remove(previousKeyPath)
		}
	}
	return nil
}

func persistNodeSSHPrivateKey(nodeID int64, privateKey string) (string, error) {
	trimmed := strings.TrimSpace(privateKey)
	if trimmed == "" {
		return "", nil
	}

	dataDir := strings.TrimSpace(os.Getenv("VPANEL_DATA_DIR"))
	if dataDir == "" {
		dataDir = "/app/data"
	}

	keyDir := filepath.Join(dataDir, "node-ssh-keys")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return "", fmt.Errorf("create node ssh key dir: %w", err)
	}

	// Each save writes a separate 0600 file before changing the database reference.
	file, err := os.CreateTemp(keyDir, fmt.Sprintf("node-%d-*.key", nodeID))
	if err != nil {
		return "", fmt.Errorf("create node ssh private key: %w", err)
	}
	_, writeErr := file.WriteString(trimmed + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("write node ssh private key: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("close node ssh private key: %w", closeErr)
	}

	return file.Name(), nil
}
