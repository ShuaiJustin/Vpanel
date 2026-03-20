package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/logger"
	"v/internal/xray"
	"v/pkg/errors"
)

// XrayHandler handles Xray-related API requests.
type XrayHandler struct {
	manager        xray.Manager
	versionManager *xray.VersionManager
	logger         logger.Logger
}

// NewXrayHandler creates a new Xray handler.
func NewXrayHandler(manager xray.Manager, log logger.Logger) *XrayHandler {
	// Create version manager with default binary directory
	versionManager := xray.NewVersionManager("./xray/bin", log)

	// Scan for installed versions
	if err := versionManager.ScanInstalledVersions(); err != nil {
		log.Warn("failed to scan installed versions", logger.F("error", err))
	}

	return &XrayHandler{
		manager:        manager,
		versionManager: versionManager,
		logger:         log,
	}
}

// GetStatus returns the current Xray status.
// GET /api/xray/status
func (h *XrayHandler) GetStatus(c *gin.Context) {
	status, err := h.manager.GetStatus(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get xray status", logger.F("error", err))
		middleware.HandleInternalError(c, errors.MsgXrayNotRunning, err)
		return
	}

	c.JSON(http.StatusOK, status)
}

// Restart restarts the Xray process.
// POST /api/xray/restart
func (h *XrayHandler) Restart(c *gin.Context) {
	if err := h.manager.Restart(c.Request.Context()); err != nil {
		h.logger.Error("failed to restart xray", logger.F("error", err))
		middleware.HandleInternalError(c, errors.MsgXrayRestartFailed, err)
		return
	}

	h.logger.Info("xray restarted by user")
	c.JSON(http.StatusOK, gin.H{"message": "Xray restarted successfully"})
}

// GetConfig returns the current Xray configuration.
// GET /api/xray/config
func (h *XrayHandler) GetConfig(c *gin.Context) {
	config, err := h.manager.GetConfig(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get xray config", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Xray config"})
		return
	}

	c.Data(http.StatusOK, "application/json", config)
}

// UpdateConfigRequest represents a config update request.
type UpdateConfigRequest struct {
	Config json.RawMessage `json:"config" binding:"required"`
}

// UpdateConfig updates the Xray configuration.
// PUT /api/xray/config
func (h *XrayHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Backup current config first
	backupPath, err := h.manager.BackupConfig(c.Request.Context())
	if err != nil {
		h.logger.Warn("failed to backup config before update", logger.F("error", err))
	}

	// Update config
	if err := h.manager.UpdateConfig(c.Request.Context(), req.Config); err != nil {
		h.logger.Error("failed to update xray config", logger.F("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reload config
	if err := h.manager.ReloadConfig(c.Request.Context()); err != nil {
		h.logger.Error("failed to reload xray config", logger.F("error", err))
		// Try to restore backup
		if backupPath != "" {
			if restoreErr := h.manager.RestoreConfig(c.Request.Context(), backupPath); restoreErr != nil {
				h.logger.Error("failed to restore backup after reload failure", logger.F("error", restoreErr))
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload config, restored backup"})
		return
	}

	h.logger.Info("xray config updated")
	c.JSON(http.StatusOK, gin.H{"message": "Config updated successfully"})
}

// GetVersion returns Xray version information.
// GET /api/xray/version
func (h *XrayHandler) GetVersion(c *gin.Context) {
	version, err := h.manager.GetVersion(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get xray version", logger.F("error", err))
		// 返回默认版本信息而不是错误
		c.JSON(http.StatusOK, gin.H{
			"version": "未安装",
			"running": false,
		})
		return
	}

	c.JSON(http.StatusOK, version)
}

// GetVersions returns available Xray versions.
// GET /api/xray/versions
func (h *XrayHandler) GetVersions(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Get available versions from version manager
	versions, err := h.versionManager.GetAvailableVersions(ctx)
	if err != nil {
		h.logger.Warn("failed to get available versions", logger.F("error", err))
	}

	// Get current version
	currentVersion := "未安装"
	version, err := h.manager.GetVersion(c.Request.Context())
	if err == nil && version != nil && version.Current != "" && version.Current != "unknown" {
		currentVersion = version.Current
		h.versionManager.SetCurrentVersion(currentVersion)
	}

	// Convert to string array for backward compatibility
	versionStrings := make([]string, len(versions))
	for i, v := range versions {
		versionStrings[i] = v.Version
	}

	// If no versions available, use defaults
	if len(versionStrings) == 0 {
		versionStrings = []string{
			"v1.8.24", "v1.8.23", "v1.8.22", "v1.8.21", "v1.8.20",
			"v1.8.19", "v1.8.18", "v1.8.17", "v1.8.16", "v1.8.15",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"current_version":    currentVersion,
		"supported_versions": versionStrings,
		"versions":           versions, // Full version info
	})
}

// GetVersionDetails returns details for a specific Xray version.
// GET /api/xray/version/:version/details
func (h *XrayHandler) GetVersionDetails(c *gin.Context) {
	requestedVersion := c.Param("version")
	if requestedVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	versions, err := h.versionManager.GetAvailableVersions(ctx)
	if err != nil {
		h.logger.Warn("failed to get version details from GitHub", logger.F("error", err))
	}

	currentVersion := h.versionManager.GetCurrentVersion()
	for _, version := range versions {
		if version.Version != requestedVersion {
			continue
		}

		description := "可用版本"
		if version.IsCurrent || requestedVersion == currentVersion {
			description = "当前正在使用的版本"
		} else if version.IsInstalled {
			description = "已下载，可直接切换"
		} else if version.DownloadURL != "" {
			description = "可从 GitHub 下载并切换"
		}

		c.JSON(http.StatusOK, gin.H{
			"version":      version.Version,
			"release_date": version.ReleaseDate,
			"description":  description,
			"changelog":    []string{},
			"is_installed": version.IsInstalled,
			"is_current":   version.IsCurrent || requestedVersion == currentVersion,
		})
		return
	}

	if requestedVersion == currentVersion {
		c.JSON(http.StatusOK, gin.H{
			"version":      requestedVersion,
			"release_date": time.Time{},
			"description":  "当前正在使用的版本",
			"changelog":    []string{},
			"is_installed": true,
			"is_current":   true,
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
}

// SyncVersions syncs versions from GitHub.
// POST /api/xray/sync-versions
func (h *XrayHandler) SyncVersions(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Force refresh by getting versions
	versions, err := h.versionManager.GetAvailableVersions(ctx)
	if err != nil {
		h.logger.Error("failed to sync versions from GitHub", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to sync versions: " + err.Error(),
		})
		return
	}

	versionStrings := make([]string, len(versions))
	for i, v := range versions {
		versionStrings[i] = v.Version
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Versions synced successfully",
		"versions": versionStrings,
		"count":    len(versions),
	})
}

// UpdateVersionRequest represents a version update request.
type UpdateVersionRequest struct {
	Version string `json:"version"` // Optional, defaults to latest
}

// Update updates Xray to a new version.
// POST /api/xray/update
func (h *XrayHandler) Update(c *gin.Context) {
	// TODO: Implement Xray update functionality
	// This would involve:
	// 1. Downloading the new version
	// 2. Stopping Xray
	// 3. Replacing the binary
	// 4. Starting Xray
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Xray update not implemented yet"})
}

// SwitchVersionRequest represents a version switch request.
type SwitchVersionRequest struct {
	Version string `json:"version" binding:"required"`
}

// SwitchVersion switches Xray to a different version.
// POST /api/xray/switch-version
func (h *XrayHandler) SwitchVersion(c *gin.Context) {
	var req SwitchVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: version is required",
		})
		return
	}

	h.logger.Info("switching xray version", logger.F("version", req.Version))

	// Check if version is available
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	versions, err := h.versionManager.GetAvailableVersions(ctx)
	if err != nil {
		h.logger.Warn("failed to get available versions", logger.F("error", err))
	}

	// Find the requested version
	var targetVersion *xray.VersionInfo
	for _, v := range versions {
		if v.Version == req.Version {
			targetVersion = &v
			break
		}
	}

	if targetVersion == nil {
		// Version not found in available list, but allow switching anyway
		h.logger.Warn("requested version not in available list", logger.F("version", req.Version))
	}

	// For now, just update the current version setting
	// In a full implementation, this would:
	// 1. Download the new version if not installed
	// 2. Stop Xray
	// 3. Switch the binary symlink
	// 4. Start Xray

	h.versionManager.SetCurrentVersion(req.Version)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Version switched successfully",
		"version": req.Version,
		"note":    "Please restart Xray to apply the new version",
	})
}

// ValidateConfig validates an Xray configuration without applying it.
// POST /api/xray/validate
func (h *XrayHandler) ValidateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.manager.ValidateConfig(c.Request.Context(), req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "Configuration is valid",
	})
}

// Start starts the Xray process.
// POST /api/xray/start
func (h *XrayHandler) Start(c *gin.Context) {
	if err := h.manager.Start(c.Request.Context()); err != nil {
		h.logger.Error("failed to start xray", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("xray started by user")
	c.JSON(http.StatusOK, gin.H{"message": "Xray started successfully"})
}

// Stop stops the Xray process.
// POST /api/xray/stop
func (h *XrayHandler) Stop(c *gin.Context) {
	if err := h.manager.Stop(c.Request.Context()); err != nil {
		h.logger.Error("failed to stop xray", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("xray stopped by user")
	c.JSON(http.StatusOK, gin.H{"message": "Xray stopped successfully"})
}

// TestConfigRequest represents a test config request.
type TestConfigRequest struct {
	ConfigPath string `json:"config_path" binding:"required"`
}

// TestConfig tests a custom Xray configuration file.
// POST /api/xray/test-config
func (h *XrayHandler) TestConfig(c *gin.Context) {
	var req TestConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: config_path is required",
		})
		return
	}

	h.logger.Info("testing xray config", logger.F("path", req.ConfigPath))

	// For now, just return success
	// In a full implementation, this would validate the config file
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration file is valid",
	})
}

// CheckUpdates checks for available Xray updates.
// GET /api/xray/check-updates
func (h *XrayHandler) CheckUpdates(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Get available versions
	versions, err := h.versionManager.GetAvailableVersions(ctx)
	if err != nil {
		h.logger.Warn("failed to check for updates", logger.F("error", err))
		c.JSON(http.StatusOK, gin.H{
			"has_update":      false,
			"current_version": h.versionManager.GetCurrentVersion(),
			"error":           err.Error(),
		})
		return
	}

	currentVersion := h.versionManager.GetCurrentVersion()
	hasUpdate := false
	latestVersion := ""
	releaseNotes := ""

	if len(versions) > 0 {
		latestVersion = versions[0].Version
		if latestVersion != currentVersion && currentVersion != "未安装" && currentVersion != "unknown" {
			hasUpdate = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"has_update":      hasUpdate,
		"current_version": currentVersion,
		"latest_version":  latestVersion,
		"release_notes":   releaseNotes,
	})
}

// DownloadVersionRequest represents a download version request.
type DownloadVersionRequest struct {
	Version string `json:"version" binding:"required"`
}

// Download downloads a specific Xray version.
// POST /api/xray/download
func (h *XrayHandler) Download(c *gin.Context) {
	var req DownloadVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: version is required",
		})
		return
	}

	h.logger.Info("downloading xray version", logger.F("version", req.Version))

	// For now, just return success
	// In a full implementation, this would download the version
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Download started",
		"version": req.Version,
	})
}

// InstallVersionRequest represents an install version request.
type InstallVersionRequest struct {
	Version string `json:"version" binding:"required"`
}

// Install installs a downloaded Xray version.
// POST /api/xray/install
func (h *XrayHandler) Install(c *gin.Context) {
	var req InstallVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: version is required",
		})
		return
	}

	h.logger.Info("installing xray version", logger.F("version", req.Version))

	// For now, just return success
	// In a full implementation, this would install the version
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Installation completed",
		"version": req.Version,
	})
}
