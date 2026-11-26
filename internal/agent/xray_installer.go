// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"v/internal/logger"
)

// XrayInstaller handles Xray installation and verification.
type XrayInstaller struct {
	logger logger.Logger
}

// NewXrayInstaller creates a new Xray installer.
func NewXrayInstaller(log logger.Logger) *XrayInstaller {
	return &XrayInstaller{
		logger: log,
	}
}

// CheckXrayInstalled checks if Xray is installed.
func (i *XrayInstaller) CheckXrayInstalled() bool {
	// Check if xray command exists
	_, err := exec.LookPath("xray")
	if err == nil {
		i.logger.Info("Xray is already installed")
		return true
	}

	// Check common installation paths
	commonPaths := []string{
		"/usr/local/bin/xray",
		"/usr/bin/xray",
		"/opt/xray/xray",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			i.logger.Info("Xray found at", logger.F("path", path))
			return true
		}
	}

	i.logger.Info("Xray is not installed")
	return false
}

// GetXrayVersion returns the installed Xray version.
func (i *XrayInstaller) GetXrayVersion() (string, error) {
	cmd := exec.Command("xray", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get xray version: %w", err)
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Xray") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}

	return "unknown", nil
}

// InstallXray installs Xray using the official installation script.
func (i *XrayInstaller) InstallXray(ctx context.Context) error {
	i.logger.Info("Starting Xray installation...")

	// Check if already installed
	if i.CheckXrayInstalled() {
		version, _ := i.GetXrayVersion()
		i.logger.Info("Xray is already installed", logger.F("version", version))
		return nil
	}

	// Determine installation method based on OS
	switch runtime.GOOS {
	case "linux":
		return i.installXrayLinux(ctx)
	case "darwin":
		return i.installXrayMacOS(ctx)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// SwitchXrayVersion stages and validates a release before replacing the configured binary.
// An empty version selects GitHub's latest release.
func (i *XrayInstaller) SwitchXrayVersion(ctx context.Context, version, binaryPath, configPath string, restart func(context.Context, string) error) (string, error) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")

	arch := runtime.GOARCH
	archMap := map[string]string{
		"amd64": "64",
		"386":   "32",
		"arm64": "arm64-v8a",
		"arm":   "arm32-v7a",
	}
	xrayArch, ok := archMap[arch]
	if !ok {
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}

	osLabel := "linux"
	if runtime.GOOS == "darwin" {
		osLabel = "macos"
	} else if runtime.GOOS != "linux" {
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	var downloadURL string
	if version == "" {
		downloadURL = fmt.Sprintf(
			"https://github.com/XTLS/Xray-core/releases/latest/download/Xray-%s-%s.zip",
			osLabel, xrayArch,
		)
		i.logger.Info("Downloading latest Xray", logger.F("url", downloadURL))
	} else {
		downloadURL = fmt.Sprintf(
			"https://github.com/XTLS/Xray-core/releases/download/v%s/Xray-%s-%s.zip",
			version, osLabel, xrayArch,
		)
		i.logger.Info("Downloading Xray version", logger.F("version", version), logger.F("url", downloadURL))
	}

	tmpDir, err := os.MkdirTemp("", "xray-install-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	zipFile := filepath.Join(tmpDir, "xray.zip")
	if output, err := exec.CommandContext(ctx, "curl", "-fsSL", "--max-time", "300", "-o", zipFile, downloadURL).CombinedOutput(); err != nil {
		return "", fmt.Errorf("download Xray %q: %w: %s", version, err, strings.TrimSpace(string(output)))
	}
	return activateXrayRelease(ctx, zipFile, version, binaryPath, configPath, restart)
}

func activateXrayRelease(ctx context.Context, archivePath, requestedVersion, binaryPath, configPath string, restart func(context.Context, string) error) (string, error) {
	path, err := exec.LookPath(binaryPath)
	if err != nil {
		return "", fmt.Errorf("find configured Xray binary: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve configured Xray binary: %w", err)
	}

	staging, err := os.MkdirTemp(filepath.Dir(path), ".xray-switch-")
	if err != nil {
		return "", binaryWriteError("Xray", path, err)
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(staging)
		}
	}()
	candidate := filepath.Join(staging, "xray")
	if err := extractXrayReleaseBinary(archivePath, candidate); err != nil {
		return "", err
	}
	output, err := exec.CommandContext(ctx, candidate, "version").Output()
	fields := strings.Fields(string(output))
	if err != nil || len(fields) < 2 || fields[0] != "Xray" {
		return "", fmt.Errorf("cannot identify version of downloaded Xray")
	}
	version := strings.TrimPrefix(fields[1], "v")
	if requestedVersion != "" && version != requestedVersion {
		return "", fmt.Errorf("downloaded Xray version %s does not match %s", version, requestedVersion)
	}
	if output, err := exec.CommandContext(ctx, candidate, "run", "-test", "-c", configPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("Xray %s cannot load the current configuration: %w: %s", version, err, strings.TrimSpace(string(output)))
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	backup := filepath.Join(staging, "previous")
	if err := os.Link(path, backup); err != nil {
		return "", binaryWriteError("Xray", path, err)
	}
	if err := os.Rename(candidate, path); err != nil {
		return "", binaryWriteError("Xray", path, err)
	}
	if err := restart(ctx, path); err != nil {
		if restoreErr := os.Rename(backup, path); restoreErr != nil {
			keepBackup = true
			return "", fmt.Errorf("start Xray %s: %v; restore previous binary from %s: %w", version, err, backup, restoreErr)
		}
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if rollbackErr := restart(rollbackCtx, path); rollbackErr != nil {
			return "", fmt.Errorf("start Xray %s: %v; restart restored Xray: %w", version, err, rollbackErr)
		}
		return "", fmt.Errorf("start Xray %s: %w; previous version restored", version, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func extractXrayReleaseBinary(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open Xray archive: %w", err)
	}
	defer archive.Close()
	src, err := archive.Open("xray")
	if err != nil {
		return fmt.Errorf("read Xray binary from archive: %w", err)
	}
	defer src.Close()
	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("stage Xray binary: %w", err)
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		return fmt.Errorf("extract Xray binary: %w", copyErr)
	}
	return closeErr
}

// installXrayLinux installs Xray on Linux using the official script.
func (i *XrayInstaller) installXrayLinux(ctx context.Context) error {
	i.logger.Info("Installing Xray on Linux...")

	// Download and execute the official installation script
	script := `bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install`

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install xray: %w", err)
	}

	// Verify installation
	if !i.CheckXrayInstalled() {
		return fmt.Errorf("xray installation verification failed")
	}

	version, _ := i.GetXrayVersion()
	i.logger.Info("Xray installed successfully", logger.F("version", version))

	return nil
}

// installXrayMacOS installs Xray on macOS.
func (i *XrayInstaller) installXrayMacOS(ctx context.Context) error {
	i.logger.Info("Installing Xray on macOS...")

	// Check if Homebrew is available
	if _, err := exec.LookPath("brew"); err == nil {
		// Try to install via Homebrew
		cmd := exec.CommandContext(ctx, "brew", "install", "xray")
		if err := cmd.Run(); err == nil {
			i.logger.Info("Xray installed via Homebrew")
			return nil
		}
	}

	// Fallback to manual installation
	return i.installXrayManual(ctx)
}

// installXrayManual manually downloads and installs Xray.
func (i *XrayInstaller) installXrayManual(ctx context.Context) error {
	i.logger.Info("Installing Xray manually...")

	// Determine architecture
	arch := runtime.GOARCH
	archMap := map[string]string{
		"amd64": "64",
		"386":   "32",
		"arm64": "arm64-v8a",
		"arm":   "arm32-v7a",
	}

	xrayArch, ok := archMap[arch]
	if !ok {
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Download URL
	downloadURL := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-%s.zip", xrayArch)
	if runtime.GOOS == "darwin" {
		downloadURL = fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/latest/download/Xray-macos-%s.zip", xrayArch)
	}

	i.logger.Info("Downloading Xray", logger.F("url", downloadURL))

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "xray-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download
	zipFile := filepath.Join(tmpDir, "xray.zip")
	cmd := exec.CommandContext(ctx, "curl", "-L", "-o", zipFile, downloadURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download xray: %w", err)
	}

	// Extract
	cmd = exec.CommandContext(ctx, "unzip", "-o", zipFile, "-d", tmpDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract xray: %w", err)
	}

	// Install to /usr/local/bin
	xrayBinary := filepath.Join(tmpDir, "xray")
	installPath := "/usr/local/bin/xray"

	cmd = exec.CommandContext(ctx, "install", "-m", "755", xrayBinary, installPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install xray binary: %w", err)
	}

	i.logger.Info("Xray installed successfully", logger.F("path", installPath))

	return nil
}

// SetupXrayConfig creates initial Xray configuration if it doesn't exist.
func (i *XrayInstaller) SetupXrayConfig(configPath string) error {
	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		i.logger.Info("Xray config already exists", logger.F("path", configPath))
		return nil
	}

	// Create config directory
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create initial config
	initialConfig := `{
  "log": {
    "loglevel": "warning",
    "access": "",
    "error": ""
  },
  "inbounds": [],
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "direct"
    }
  ]
}`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	i.logger.Info("Created initial Xray config", logger.F("path", configPath))

	return nil
}

// EnsureXrayInstalled ensures Xray is installed and configured.
func (i *XrayInstaller) EnsureXrayInstalled(ctx context.Context, configPath string) error {
	// Check if installed
	if !i.CheckXrayInstalled() {
		i.logger.Info("Xray not found, installing...")
		if err := i.InstallXray(ctx); err != nil {
			return fmt.Errorf("failed to install xray: %w", err)
		}
	}

	// Setup config
	if err := i.SetupXrayConfig(configPath); err != nil {
		return fmt.Errorf("failed to setup xray config: %w", err)
	}

	// Verify installation
	version, err := i.GetXrayVersion()
	if err != nil {
		return fmt.Errorf("failed to verify xray installation: %w", err)
	}

	i.logger.Info("Xray is ready", logger.F("version", version))

	return nil
}
