// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

// InstallXrayVersion downloads and installs a specific Xray version, replacing
// any existing binary. An empty version means "latest release".
// Supports Linux and macOS.
func (i *XrayInstaller) InstallXrayVersion(ctx context.Context, version string) error {
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
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	osLabel := "linux"
	if runtime.GOOS == "darwin" {
		osLabel = "macos"
	} else if runtime.GOOS != "linux" {
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
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
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	zipFile := filepath.Join(tmpDir, "xray.zip")
	if err := exec.CommandContext(ctx, "curl", "-fL", "--max-time", "300", "-o", zipFile, downloadURL).Run(); err != nil {
		return fmt.Errorf("failed to download xray %q: %w", version, err)
	}

	if err := exec.CommandContext(ctx, "unzip", "-o", zipFile, "-d", tmpDir).Run(); err != nil {
		return fmt.Errorf("failed to extract xray: %w", err)
	}

	xrayBinary := filepath.Join(tmpDir, "xray")
	if _, err := os.Stat(xrayBinary); err != nil {
		return fmt.Errorf("xray binary not found in release archive: %w", err)
	}

	installPath := i.resolveInstallPath()
	if err := exec.CommandContext(ctx, "install", "-m", "755", xrayBinary, installPath).Run(); err != nil {
		return fmt.Errorf("failed to install xray binary to %s: %w", installPath, err)
	}

	installedVersion, _ := i.GetXrayVersion()
	i.logger.Info("Xray installed",
		logger.F("path", installPath),
		logger.F("requested_version", version),
		logger.F("installed_version", installedVersion))

	return nil
}

// resolveInstallPath returns the existing xray binary path or a sensible default.
func (i *XrayInstaller) resolveInstallPath() string {
	if path, err := exec.LookPath("xray"); err == nil {
		return path
	}
	for _, p := range []string{"/usr/local/bin/xray", "/usr/bin/xray", "/opt/xray/xray"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/usr/local/bin/xray"
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
