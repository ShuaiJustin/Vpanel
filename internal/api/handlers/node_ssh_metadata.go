package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"v/internal/node"
)

type nodeSSHMetadataInput struct {
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey string
}

func persistNodeSSHMetadata(ctx context.Context, nodeService *node.Service, nodeID int64, input nodeSSHMetadataInput) error {
	if nodeService == nil {
		return fmt.Errorf("node service is unavailable")
	}

	keyPath, err := persistNodeSSHPrivateKey(nodeID, input.PrivateKey)
	if err != nil {
		return err
	}

	return nodeService.UpdateSSHConfig(ctx, nodeID, input.Host, input.Port, input.Username, input.Password, keyPath)
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

	keyPath := filepath.Join(keyDir, fmt.Sprintf("node-%d.key", nodeID))
	tempPath := keyPath + ".tmp"
	keyData := []byte(trimmed + "\n")

	if err := os.WriteFile(tempPath, keyData, 0o600); err != nil {
		return "", fmt.Errorf("write node ssh private key: %w", err)
	}
	if err := os.Rename(tempPath, keyPath); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("install node ssh private key: %w", err)
	}
	if err := os.Chmod(keyPath, 0o600); err != nil {
		return "", fmt.Errorf("chmod node ssh private key: %w", err)
	}

	return keyPath, nil
}
