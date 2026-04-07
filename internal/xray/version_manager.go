// Package xray provides Xray-core version management.
package xray

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"v/internal/logger"
)

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// VersionInfo represents version information.
type VersionInfo struct {
	Version     string    `json:"version"`
	ReleaseDate time.Time `json:"release_date"`
	IsInstalled bool      `json:"is_installed"`
	IsCurrent   bool      `json:"is_current"`
	DownloadURL string    `json:"download_url,omitempty"`
}

// VersionManager manages Xray versions.
type VersionManager struct {
	mu                sync.RWMutex
	logger            logger.Logger
	binaryDir         string
	currentVersion    string
	cachedVersions    []VersionInfo
	lastFetchTime     time.Time
	cacheDuration     time.Duration
	httpClient        *http.Client
	githubAPIURL      string
	installedVersions map[string]string // version -> binary path
}

// NewVersionManager creates a new version manager.
func NewVersionManager(binaryDir string, log logger.Logger) *VersionManager {
	return &VersionManager{
		logger:        log,
		binaryDir:     binaryDir,
		cacheDuration: 30 * time.Minute,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		githubAPIURL:      "https://api.github.com/repos/XTLS/Xray-core/releases",
		installedVersions: make(map[string]string),
	}
}

// GetAvailableVersions fetches available versions from GitHub.
func (vm *VersionManager) GetAvailableVersions(ctx context.Context) ([]VersionInfo, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Check cache first (valid for cacheDuration)
	if len(vm.cachedVersions) > 0 && time.Since(vm.lastFetchTime) < vm.cacheDuration {
		return vm.cachedVersions, nil
	}

	// Try to fetch from GitHub
	releases, err := vm.fetchGitHubReleases(ctx)
	if err != nil {
		vm.logger.Warn("failed to fetch GitHub releases", logger.F("error", err))
		// Return cached versions if available
		if len(vm.cachedVersions) > 0 {
			return vm.cachedVersions, nil
		}
		// Return default versions if no cache (no error, just use defaults)
		defaultVersions := vm.getDefaultVersionsUnlocked()
		vm.cachedVersions = defaultVersions
		vm.lastFetchTime = time.Now()
		return defaultVersions, nil
	}

	// Convert to VersionInfo
	versions := make([]VersionInfo, 0, len(releases))
	for _, release := range releases {
		if release.Prerelease {
			continue // Skip prereleases
		}

		downloadURL := vm.getDownloadURL(release)
		isInstalled := false
		if _, ok := vm.installedVersions[release.TagName]; ok {
			isInstalled = true
		}
		versions = append(versions, VersionInfo{
			Version:     release.TagName,
			ReleaseDate: release.PublishedAt,
			IsInstalled: isInstalled,
			IsCurrent:   release.TagName == vm.currentVersion,
			DownloadURL: downloadURL,
		})
	}

	// Sort by version (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	// Limit to 20 versions
	if len(versions) > 20 {
		versions = versions[:20]
	}

	// Update cache (already holding lock)
	vm.cachedVersions = versions
	vm.lastFetchTime = time.Now()

	return versions, nil
}

// fetchGitHubReleases fetches releases from GitHub API.
func (vm *VersionManager) fetchGitHubReleases(ctx context.Context) ([]GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", vm.githubAPIURL+"?per_page=30", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "V-Panel/1.0")

	resp, err := vm.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	return releases, nil
}

// getDownloadURL returns the download URL for the current platform.
func (vm *VersionManager) getDownloadURL(release GitHubRelease) string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch to Xray naming
	archMap := map[string]string{
		"amd64": "64",
		"386":   "32",
		"arm64": "arm64-v8a",
		"arm":   "arm32-v7a",
	}

	xrayArch, ok := archMap[arch]
	if !ok {
		xrayArch = arch
	}

	// Build expected filename pattern
	var pattern string
	switch osName {
	case "darwin":
		pattern = fmt.Sprintf("Xray-macos-%s.zip", xrayArch)
	case "linux":
		pattern = fmt.Sprintf("Xray-linux-%s.zip", xrayArch)
	case "windows":
		pattern = fmt.Sprintf("Xray-windows-%s.zip", xrayArch)
	default:
		pattern = fmt.Sprintf("Xray-%s-%s.zip", osName, xrayArch)
	}

	for _, asset := range release.Assets {
		if strings.EqualFold(asset.Name, pattern) {
			return asset.BrowserDownloadURL
		}
	}

	// Try alternative patterns
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, strings.ToLower(osName)) && strings.Contains(name, xrayArch) {
			return asset.BrowserDownloadURL
		}
	}

	return ""
}

// isVersionInstalled checks if a version is installed.
func (vm *VersionManager) isVersionInstalled(version string) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	_, ok := vm.installedVersions[version]
	return ok
}

// getDefaultVersionsUnlocked returns default version list when GitHub is unavailable.
// Must be called with lock held.
func (vm *VersionManager) getDefaultVersionsUnlocked() []VersionInfo {
	defaultVersions := []string{
		"v1.8.24", "v1.8.23", "v1.8.22", "v1.8.21", "v1.8.20",
		"v1.8.19", "v1.8.18", "v1.8.17", "v1.8.16", "v1.8.15",
	}

	versions := make([]VersionInfo, len(defaultVersions))
	for i, v := range defaultVersions {
		isInstalled := false
		if _, ok := vm.installedVersions[v]; ok {
			isInstalled = true
		}
		versions[i] = VersionInfo{
			Version:     v,
			IsInstalled: isInstalled,
			IsCurrent:   v == vm.currentVersion,
		}
	}
	return versions
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func platformBinaryName() string {
	if runtime.GOOS == "windows" {
		return "xray.exe"
	}
	return "xray"
}

func (vm *VersionManager) archivePath(version string) string {
	return filepath.Join(vm.binaryDir, "downloads", fmt.Sprintf("%s.zip", normalizeVersion(version)))
}

// GetInstalledBinaryPath returns the installed binary path for a version.
func (vm *VersionManager) GetInstalledBinaryPath(version string) (string, bool) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	path, ok := vm.installedVersions[normalizeVersion(version)]
	return path, ok
}

// DownloadVersion downloads the release archive for a version into the downloads directory.
func (vm *VersionManager) DownloadVersion(ctx context.Context, version string) (string, error) {
	version = normalizeVersion(version)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}

	versions, err := vm.GetAvailableVersions(ctx)
	if err != nil {
		return "", err
	}

	var downloadURL string
	for _, item := range versions {
		if item.Version == version {
			downloadURL = item.DownloadURL
			break
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("download URL not found for version %s", version)
	}

	archivePath := vm.archivePath(version)
	if _, err := os.Stat(archivePath); err == nil {
		return archivePath, nil
	}

	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "V-Panel/1.0")

	resp, err := vm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}

	file, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to create archive file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save downloaded archive: %w", err)
	}

	return archivePath, nil
}

// InstallVersion installs a downloaded Xray version into the binary directory.
func (vm *VersionManager) InstallVersion(ctx context.Context, version string) (string, error) {
	version = normalizeVersion(version)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}

	if installedPath, ok := vm.GetInstalledBinaryPath(version); ok {
		if _, err := os.Stat(installedPath); err == nil {
			return installedPath, nil
		}
	}

	archivePath := vm.archivePath(version)
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		var downloadErr error
		archivePath, downloadErr = vm.DownloadVersion(ctx, version)
		if downloadErr != nil {
			return "", downloadErr
		}
	}

	if err := os.MkdirAll(vm.binaryDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create binary directory: %w", err)
	}

	installedPath := filepath.Join(vm.binaryDir, fmt.Sprintf("xray-%s", version))
	if runtime.GOOS == "windows" {
		installedPath += ".exe"
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer reader.Close()

	binaryName := platformBinaryName()
	found := false
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(file.Name) != binaryName {
			continue
		}

		src, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open archive entry: %w", err)
		}

		dst, err := os.Create(installedPath)
		if err != nil {
			src.Close()
			return "", fmt.Errorf("failed to create installed binary: %w", err)
		}

		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			return "", fmt.Errorf("failed to extract binary: %w", err)
		}
		dst.Close()
		src.Close()

		if runtime.GOOS != "windows" {
			if err := os.Chmod(installedPath, 0o755); err != nil {
				return "", fmt.Errorf("failed to set executable permission: %w", err)
			}
		}
		found = true
		break
	}

	if !found {
		return "", fmt.Errorf("xray executable not found in archive")
	}

	vm.mu.Lock()
	vm.installedVersions[version] = installedPath
	vm.mu.Unlock()

	return installedPath, nil
}

// getDefaultVersions returns default version list when GitHub is unavailable.
func (vm *VersionManager) getDefaultVersions() []VersionInfo {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.getDefaultVersionsUnlocked()
}

// compareVersions compares two version strings.
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}
		if n1 != n2 {
			return n1 - n2
		}
	}
	return 0
}

// SetCurrentVersion sets the current version.
func (vm *VersionManager) SetCurrentVersion(version string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.currentVersion = version
}

// GetCurrentVersion returns the current version.
func (vm *VersionManager) GetCurrentVersion() string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.currentVersion
}

// ScanInstalledVersions scans the binary directory for installed versions.
func (vm *VersionManager) ScanInstalledVersions() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.installedVersions = make(map[string]string)

	if vm.binaryDir == "" {
		return nil
	}

	// Check if directory exists
	if _, err := os.Stat(vm.binaryDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(vm.binaryDir)
	if err != nil {
		return fmt.Errorf("failed to read binary directory: %w", err)
	}

	versionRegex := regexp.MustCompile(`xray[-_]?(v?\d+\.\d+\.\d+)`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		matches := versionRegex.FindStringSubmatch(name)
		if len(matches) > 1 {
			version := matches[1]
			if !strings.HasPrefix(version, "v") {
				version = "v" + version
			}
			vm.installedVersions[version] = filepath.Join(vm.binaryDir, name)
		}
	}

	return nil
}

// GetInstalledVersions returns list of installed versions.
func (vm *VersionManager) GetInstalledVersions() []string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := make([]string, 0, len(vm.installedVersions))
	for v := range vm.installedVersions {
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	return versions
}
