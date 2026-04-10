// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/ip"
	"v/internal/logger"
	"v/pkg/errors"
)

// IPRestrictionHandler handles IP restriction related requests.
type IPRestrictionHandler struct {
	logger    logger.Logger
	ipService *ip.Service
	schema    concurrentIPSchema
}

type ipCountryStat struct {
	Country string `json:"country" gorm:"column:country"`
	Count   int64  `json:"count" gorm:"column:count"`
}

type concurrentIPSchema struct {
	userHasMaxConcurrentIPs bool
	userHasPlanID           bool
	hasPlansTable           bool
	hasCommercialPlansTable bool
}

// NewIPRestrictionHandler creates a new IPRestrictionHandler.
func NewIPRestrictionHandler(log logger.Logger, ipService *ip.Service) *IPRestrictionHandler {
	schema := concurrentIPSchema{}
	if ipService != nil && ipService.Tracker() != nil && ipService.Tracker().GetDB() != nil {
		db := ipService.Tracker().GetDB()
		migrator := db.Migrator()
		schema = concurrentIPSchema{
			userHasMaxConcurrentIPs: migrator.HasColumn("users", "max_concurrent_ips"),
			userHasPlanID:           migrator.HasColumn("users", "plan_id"),
			hasPlansTable:           migrator.HasTable("plans"),
			hasCommercialPlansTable: migrator.HasTable("commercial_plans"),
		}
	}

	return &IPRestrictionHandler{
		logger:    log,
		ipService: ipService,
		schema:    schema,
	}
}

// ResolveUserMaxConcurrentIPs resolves effective device limit for a user.
// Priority: user override > plan default > global default.
// Return semantics: -1 use global default, 0 unlimited, >0 explicit limit.
func (h *IPRestrictionHandler) ResolveUserMaxConcurrentIPs(userID int64) int {
	return h.resolveUserMaxConcurrentIPsWithContext(context.Background(), userID)
}

func (h *IPRestrictionHandler) resolveUserMaxConcurrentIPsWithContext(ctx context.Context, userID int64) int {
	if h.ipService == nil || h.ipService.Tracker() == nil || h.ipService.Tracker().GetDB() == nil {
		return -1
	}

	db := h.ipService.Tracker().GetDB().WithContext(ctx)

	type userRow struct {
		Role             string `gorm:"column:role"`
		MaxConcurrentIPs *int   `gorm:"column:max_concurrent_ips"`
		PlanID           *int64 `gorm:"column:plan_id"`
	}

	var result userRow
	selectFields := []string{"role"}
	if h.schema.userHasMaxConcurrentIPs {
		selectFields = append(selectFields, "max_concurrent_ips")
	}
	if h.schema.userHasPlanID {
		selectFields = append(selectFields, "plan_id")
	}

	err := db.Table("users").Select(strings.Join(selectFields, ", ")).Where("id = ?", userID).Take(&result).Error
	if err != nil {
		h.logger.Warn("Failed to resolve user max concurrent IPs", logger.F("user_id", userID), logger.F("error", err))
		return -1
	}

	if strings.EqualFold(result.Role, "admin") {
		return 0
	}

	if h.schema.userHasMaxConcurrentIPs && result.MaxConcurrentIPs != nil && *result.MaxConcurrentIPs >= 0 {
		return *result.MaxConcurrentIPs
	}

	if h.schema.userHasPlanID && result.PlanID != nil && *result.PlanID > 0 {
		if planDefault, ok := h.lookupPlanDefaultMaxConcurrentIPs(ctx, *result.PlanID); ok {
			return planDefault
		}
	}

	return -1
}

func (h *IPRestrictionHandler) lookupPlanDefaultMaxConcurrentIPs(ctx context.Context, planID int64) (int, bool) {
	if h.ipService == nil || h.ipService.Tracker() == nil || h.ipService.Tracker().GetDB() == nil {
		return 0, false
	}

	db := h.ipService.Tracker().GetDB().WithContext(ctx)

	type planRow struct {
		Value *int `gorm:"column:value"`
	}

	if h.schema.hasPlansTable {
		var plan planRow
		if err := db.Table("plans").Select("default_max_concurrent_ips AS value").Where("id = ?", planID).Take(&plan).Error; err == nil {
			if plan.Value != nil && *plan.Value >= 0 {
				return *plan.Value, true
			}
			return 0, false
		}
	}

	if h.schema.hasCommercialPlansTable {
		var plan planRow
		if err := db.Table("commercial_plans").Select("ip_limit AS value").Where("id = ?", planID).Take(&plan).Error; err == nil {
			if plan.Value != nil && *plan.Value >= 0 {
				return *plan.Value, true
			}
			return 0, false
		}
	}

	return 0, false
}

// GetStats returns IP restriction statistics.
// GET /api/admin/ip-restrictions/stats
func (h *IPRestrictionHandler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Check if IP service is available
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	// Additional safety check for tracker
	tracker := h.ipService.Tracker()
	if tracker == nil {
		h.logger.Error("IP tracker is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Tracker initialization failed",
		})
		return
	}

	// Get global statistics
	var totalActiveIPs int64
	var totalBlacklisted int64
	var totalWhitelisted int64
	var blockedToday int64
	var suspiciousCount int64
	countryStats := make([]ipCountryStat, 0)

	db := tracker.GetDB()
	if db == nil {
		h.logger.Error("Database connection is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Database service is not available",
			"error":   "Database connection failed",
		})
		return
	}

	timeout := time.Duration(h.ipService.GetSettings().InactiveTimeout) * time.Minute
	if timeout > 0 {
		if _, err := tracker.CleanupInactiveIPs(ctx, timeout); err != nil {
			h.logger.Warn("Failed to cleanup inactive IPs before stats", logger.Err(err))
		}
	}

	if validator := h.ipService.Validator(); validator != nil {
		if _, err := validator.CleanupExpiredBlacklist(ctx); err != nil {
			h.logger.Warn("Failed to cleanup expired blacklist entries before stats", logger.Err(err))
		}
	}

	if err := db.WithContext(ctx).Model(&ip.ActiveIP{}).Count(&totalActiveIPs).Error; err != nil {
		h.logger.Error("Failed to count active IPs", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve IP statistics",
			"error":   "Database query failed",
		})
		return
	}

	if err := db.WithContext(ctx).Model(&ip.IPBlacklist{}).Count(&totalBlacklisted).Error; err != nil {
		h.logger.Error("Failed to count blacklisted IPs", logger.Err(err))
		// Don't return error, just log it and continue with 0
		totalBlacklisted = 0
	}

	if err := db.WithContext(ctx).Model(&ip.IPWhitelist{}).Count(&totalWhitelisted).Error; err != nil {
		h.logger.Error("Failed to count whitelisted IPs", logger.Err(err))
		// Don't return error, just log it and continue with 0
		totalWhitelisted = 0
	}

	// Count unique active users
	var activeUsers int64
	if err := db.WithContext(ctx).Model(&ip.ActiveIP{}).Distinct("user_id").Count(&activeUsers).Error; err != nil {
		h.logger.Error("Failed to count active users", logger.Err(err))
		// Don't return error, just log it and continue with 0
		activeUsers = 0
	}

	startOfDay := time.Now().In(time.Local)
	startOfDay = time.Date(startOfDay.Year(), startOfDay.Month(), startOfDay.Day(), 0, 0, 0, 0, startOfDay.Location())

	if err := db.WithContext(ctx).
		Model(&ip.FailedAttempt{}).
		Where("created_at >= ?", startOfDay).
		Count(&blockedToday).Error; err != nil {
		h.logger.Error("Failed to count blocked attempts", logger.Err(err))
		blockedToday = 0
	}

	if err := db.WithContext(ctx).
		Model(&ip.IPHistory{}).
		Where("is_suspicious = ?", true).
		Count(&suspiciousCount).Error; err != nil {
		h.logger.Error("Failed to count suspicious activities", logger.Err(err))
		suspiciousCount = 0
	}

	if err := db.WithContext(ctx).
		Model(&ip.ActiveIP{}).
		Select("country, COUNT(*) AS count").
		Where("country <> ''").
		Group("country").
		Order("COUNT(*) DESC").
		Scan(&countryStats).Error; err != nil {
		h.logger.Error("Failed to aggregate active IP countries", logger.Err(err))
		countryStats = make([]ipCountryStat, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"total_active_ips":  totalActiveIPs,
			"total_blacklisted": totalBlacklisted,
			"total_whitelisted": totalWhitelisted,
			"active_users":      activeUsers,
			"blocked_today":     blockedToday,
			"suspicious_count":  suspiciousCount,
			"country_stats":     countryStats,
			"settings":          h.ipService.GetSettings(),
		},
	})
}

// GetAllOnlineIPs returns all online IPs across all users (admin only).
// GET /api/admin/ip-restrictions/online
func (h *IPRestrictionHandler) GetAllOnlineIPs(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	// Get all active IPs from database
	tracker := h.ipService.Tracker()
	if tracker == nil {
		h.logger.Error("IP tracker is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP tracker is not available",
		})
		return
	}

	db := tracker.GetDB()
	if db == nil {
		h.logger.Error("Database connection is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Database service is not available",
		})
		return
	}

	timeout := time.Duration(h.ipService.GetSettings().InactiveTimeout) * time.Minute
	if timeout > 0 {
		if _, err := tracker.CleanupInactiveIPs(ctx, timeout); err != nil {
			h.logger.Warn("Failed to cleanup inactive IPs before listing online IPs", logger.Err(err))
		}
	}

	var activeIPs []ip.ActiveIP
	if err := db.WithContext(ctx).Find(&activeIPs).Error; err != nil {
		h.logger.Error("Failed to get active IPs", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve online IPs",
			"error":   "Database query failed",
		})
		return
	}

	h.ipService.EnrichActiveIPRecords(ctx, activeIPs)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    activeIPs,
	})
}

// GetUserOnlineIPs returns online IPs for a specific user.
// GET /api/admin/users/:id/online-ips
func (h *IPRestrictionHandler) GetUserOnlineIPs(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid user ID", nil))
		return
	}

	onlineIPs, err := h.ipService.GetOnlineIPs(ctx, uint(userID))
	if err != nil {
		h.logger.Error("Failed to get online IPs", logger.F("error", err), logger.F("user_id", userID))
		middleware.RespondWithError(c, errors.NewDatabaseError("get online IPs", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    onlineIPs,
	})
}

// KickIPRequest represents a request to kick an IP.
type KickIPRequest struct {
	IP             string `json:"ip" binding:"required"`
	AddToBlacklist bool   `json:"add_to_blacklist"`
	BlockDuration  int    `json:"block_duration"` // minutes
}

// KickUserIP kicks a specific IP for a user.
// POST /api/admin/users/:id/kick-ip
func (h *IPRestrictionHandler) KickUserIP(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid user ID", nil))
		return
	}

	var req KickIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	blockDuration := time.Duration(req.BlockDuration) * time.Minute
	if err := h.ipService.KickIP(ctx, uint(userID), req.IP, req.AddToBlacklist, blockDuration); err != nil {
		h.logger.Error("Failed to kick IP", logger.F("error", err), logger.F("user_id", userID), logger.F("ip", req.IP))
		middleware.RespondWithError(c, errors.NewDatabaseError("kick IP", err))
		return
	}

	h.logger.Info("IP kicked", logger.F("user_id", userID), logger.F("ip", req.IP))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "IP kicked successfully",
	})
}

// WhitelistEntry represents a whitelist entry request.
type WhitelistEntry struct {
	IP          string `json:"ip"`
	CIDR        string `json:"cidr"`
	UserID      *uint  `json:"user_id"`
	Description string `json:"description"`
}

// GetWhitelist returns the IP whitelist.
// GET /api/admin/ip-whitelist
func (h *IPRestrictionHandler) GetWhitelist(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	var userID *uint
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseUint(userIDStr, 10, 64)
		if err == nil {
			uid := uint(id)
			userID = &uid
		}
	}

	entries, err := h.ipService.Validator().GetWhitelist(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get whitelist", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get whitelist", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    entries,
	})
}

// AddWhitelistRequest represents a request to add to whitelist.
type AddWhitelistRequest struct {
	IP          string `json:"ip"`
	CIDR        string `json:"cidr"`
	UserID      *uint  `json:"user_id"`
	Description string `json:"description"`
}

// AddWhitelist adds an IP to the whitelist.
// POST /api/admin/ip-whitelist
func (h *IPRestrictionHandler) AddWhitelist(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	var req AddWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	if req.IP == "" && req.CIDR == "" {
		middleware.RespondWithError(c, errors.NewValidationError("IP or CIDR is required", nil))
		return
	}

	// Get current user ID from context
	currentUserID := middleware.GetUserID(c)

	entry := &ip.IPWhitelist{
		IP:          req.IP,
		CIDR:        req.CIDR,
		UserID:      req.UserID,
		Description: req.Description,
		CreatedBy:   uint(currentUserID),
	}

	if err := h.ipService.Validator().AddToWhitelist(ctx, entry); err != nil {
		h.logger.Error("Failed to add to whitelist", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("add to whitelist", err))
		return
	}

	h.logger.Info("IP added to whitelist", logger.F("ip", req.IP), logger.F("cidr", req.CIDR))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "added to whitelist",
		"data":    entry,
	})
}

// DeleteWhitelist removes an IP from the whitelist.
// DELETE /api/admin/ip-whitelist/:id
func (h *IPRestrictionHandler) DeleteWhitelist(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid ID", nil))
		return
	}

	if err := h.ipService.Validator().RemoveFromWhitelist(ctx, uint(id)); err != nil {
		h.logger.Error("Failed to remove from whitelist", logger.F("error", err), logger.F("id", id))
		middleware.RespondWithError(c, errors.NewDatabaseError("remove from whitelist", err))
		return
	}

	h.logger.Info("IP removed from whitelist", logger.F("id", id))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "removed from whitelist",
	})
}

// ImportWhitelistRequest represents a request to import whitelist.
type ImportWhitelistRequest struct {
	IPs         []string `json:"ips" binding:"required"`
	UserID      *uint    `json:"user_id"`
	Description string   `json:"description"`
}

// ImportWhitelist imports multiple IPs to the whitelist.
// POST /api/admin/ip-whitelist/import
func (h *IPRestrictionHandler) ImportWhitelist(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	var req ImportWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	currentUserID := middleware.GetUserID(c)

	if err := h.ipService.Validator().ImportWhitelist(ctx, req.IPs, req.UserID, req.Description, uint(currentUserID)); err != nil {
		h.logger.Error("Failed to import whitelist", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("import whitelist", err))
		return
	}

	h.logger.Info("Whitelist imported", logger.F("count", len(req.IPs)))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "whitelist imported",
		"data": gin.H{
			"imported": len(req.IPs),
		},
	})
}

// GetBlacklist returns the IP blacklist.
// GET /api/admin/ip-blacklist
func (h *IPRestrictionHandler) GetBlacklist(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	var userID *uint
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseUint(userIDStr, 10, 64)
		if err == nil {
			uid := uint(id)
			userID = &uid
		}
	}

	entries, err := h.ipService.Validator().GetBlacklist(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get blacklist", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get blacklist", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    entries,
	})
}

// AddBlacklistRequest represents a request to add to blacklist.
type AddBlacklistRequest struct {
	IP        string `json:"ip"`
	CIDR      string `json:"cidr"`
	UserID    *uint  `json:"user_id"`
	Reason    string `json:"reason"`
	ExpiresIn int    `json:"expires_in"` // minutes, 0 for permanent
}

// AddBlacklist adds an IP to the blacklist.
// POST /api/admin/ip-blacklist
func (h *IPRestrictionHandler) AddBlacklist(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	var req AddBlacklistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	if req.IP == "" && req.CIDR == "" {
		middleware.RespondWithError(c, errors.NewValidationError("IP or CIDR is required", nil))
		return
	}

	currentUserID := middleware.GetUserID(c)
	createdBy := uint(currentUserID)

	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Minute)
		expiresAt = &t
	}

	entry := &ip.IPBlacklist{
		IP:          req.IP,
		CIDR:        req.CIDR,
		UserID:      req.UserID,
		Reason:      req.Reason,
		ExpiresAt:   expiresAt,
		IsAutomatic: false,
		CreatedBy:   &createdBy,
	}

	if err := h.ipService.Validator().AddToBlacklist(ctx, entry); err != nil {
		h.logger.Error("Failed to add to blacklist", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("add to blacklist", err))
		return
	}

	h.logger.Info("IP added to blacklist", logger.F("ip", req.IP), logger.F("cidr", req.CIDR))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "added to blacklist",
		"data":    entry,
	})
}

// DeleteBlacklist removes an IP from the blacklist.
// DELETE /api/admin/ip-blacklist/:id
func (h *IPRestrictionHandler) DeleteBlacklist(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid ID", nil))
		return
	}

	if err := h.ipService.Validator().RemoveFromBlacklist(ctx, uint(id)); err != nil {
		h.logger.Error("Failed to remove from blacklist", logger.F("error", err), logger.F("id", id))
		middleware.RespondWithError(c, errors.NewDatabaseError("remove from blacklist", err))
		return
	}

	h.logger.Info("IP removed from blacklist", logger.F("id", id))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "removed from blacklist",
	})
}

// GetIPRestrictionSettings returns IP restriction settings.
// GET /api/admin/settings/ip-restriction
func (h *IPRestrictionHandler) GetIPRestrictionSettings(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	settings := h.ipService.GetSettings()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    settings,
	})
}

// UpdateIPRestrictionSettings updates IP restriction settings.
// PUT /api/admin/settings/ip-restriction
func (h *IPRestrictionHandler) UpdateIPRestrictionSettings(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	var settings ip.IPRestrictionSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	if err := h.ipService.SaveSettings(ctx, &settings); err != nil {
		h.logger.Error("Failed to save IP restriction settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("save settings", err))
		return
	}

	h.logger.Info("IP restriction settings updated")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "settings updated",
		"data":    settings,
	})
}

// ===== User API Endpoints =====

// GetUserDevices returns the current user's online devices.
// GET /api/user/devices
func (h *IPRestrictionHandler) GetUserDevices(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.RespondWithError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	onlineIPs, err := h.ipService.GetOnlineIPs(ctx, uint(userID))
	if err != nil {
		h.logger.Error("Failed to get user devices", logger.F("error", err), logger.F("user_id", userID))
		middleware.RespondWithError(c, errors.NewDatabaseError("get devices", err))
		return
	}

	currentIP := c.ClientIP()
	devices := make([]gin.H, len(onlineIPs))
	for i, onlineIP := range onlineIPs {
		devices[i] = gin.H{
			"ip":           onlineIP.IP,
			"user_agent":   onlineIP.UserAgent,
			"device_type":  onlineIP.DeviceType,
			"country":      onlineIP.Country,
			"country_code": onlineIP.CountryCode,
			"city":         onlineIP.City,
			"last_active":  onlineIP.LastActive,
			"created_at":   onlineIP.CreatedAt,
			"is_current":   onlineIP.IP == currentIP,
		}
	}

	maxDevices := h.resolveUserMaxConcurrentIPsWithContext(ctx, userID)
	if maxDevices < 0 {
		maxDevices = h.ipService.GetSettings().DefaultMaxConcurrentIPs
	}
	remainingSlots := 0
	if maxDevices > 0 {
		remainingSlots = maxDevices - len(onlineIPs)
		if remainingSlots < 0 {
			remainingSlots = 0
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"devices":         devices,
			"max_devices":     maxDevices,
			"current_count":   len(onlineIPs),
			"current_ip":      currentIP,
			"remaining_slots": remainingSlots,
		},
	})
}

// UserKickDeviceRequest represents a request to kick a device.
type UserKickDeviceRequest struct {
	AddToBlacklist bool `json:"add_to_blacklist"`
	BlockDuration  int  `json:"block_duration"` // minutes
}

// KickUserDevice kicks a specific device for the current user.
// POST /api/user/devices/:ip/kick
func (h *IPRestrictionHandler) KickUserDevice(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.RespondWithError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	ipAddr := c.Param("ip")
	if ipAddr == "" {
		middleware.RespondWithError(c, errors.NewValidationError("IP address is required", nil))
		return
	}
	if ipAddr == c.ClientIP() {
		middleware.RespondWithError(c, errors.NewValidationError("current device cannot be kicked", nil))
		return
	}

	var req UserKickDeviceRequest
	// Bind JSON if provided, otherwise use defaults
	_ = c.ShouldBindJSON(&req)

	blockDuration := time.Duration(req.BlockDuration) * time.Minute
	if err := h.ipService.KickIP(ctx, uint(userID), ipAddr, req.AddToBlacklist, blockDuration); err != nil {
		h.logger.Error("Failed to kick device", logger.F("error", err), logger.F("user_id", userID), logger.F("ip", ipAddr))
		middleware.RespondWithError(c, errors.NewDatabaseError("kick device", err))
		return
	}

	h.logger.Info("User kicked device", logger.F("user_id", userID), logger.F("ip", ipAddr))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "device kicked successfully",
	})
}

// GetUserIPStats returns IP statistics for the current user.
// GET /api/user/ip-stats
func (h *IPRestrictionHandler) GetUserIPStats(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.RespondWithError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	maxConcurrentIPs := h.resolveUserMaxConcurrentIPsWithContext(ctx, userID)
	if maxConcurrentIPs < 0 {
		maxConcurrentIPs = h.ipService.GetSettings().DefaultMaxConcurrentIPs
	}

	stats, err := h.ipService.GetIPStats(ctx, uint(userID), maxConcurrentIPs)
	if err != nil {
		h.logger.Error("Failed to get IP stats", logger.F("error", err), logger.F("user_id", userID))
		middleware.RespondWithError(c, errors.NewDatabaseError("get IP stats", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// GetUserIPHistory returns IP history for the current user.
// GET /api/user/ip-history
func (h *IPRestrictionHandler) GetUserIPHistory(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.RespondWithError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	// Parse query parameters
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	filter := &ip.IPHistoryFilter{
		Limit:  limit,
		Offset: offset,
	}

	history, total, err := h.ipService.GetAggregatedIPHistory(ctx, uint(userID), filter.Limit, filter.Offset)
	if err != nil {
		h.logger.Error("Failed to get IP history", logger.F("error", err), logger.F("user_id", userID))
		middleware.RespondWithError(c, errors.NewDatabaseError("get IP history", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"list":  history,
			"total": total,
		},
	})
}

// GetAllIPHistory returns IP history for all users (admin only).
// GET /api/admin/ip-restrictions/history
func (h *IPRestrictionHandler) GetAllIPHistory(c *gin.Context) {
	if h.ipService == nil {
		h.logger.Error("IP service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP restriction service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	ctx := c.Request.Context()

	// Parse query parameters
	var userID *uint
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
			uid := uint(id)
			userID = &uid
		}
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	tracker := h.ipService.Tracker()
	if tracker == nil {
		h.logger.Error("IP tracker is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "IP tracker is not available",
		})
		return
	}

	// If user_id is specified, get history for that user
	if userID != nil {
		filter := &ip.IPHistoryFilter{
			Limit:  limit,
			Offset: offset,
		}

		history, err := tracker.GetIPHistory(ctx, *userID, filter)
		if err != nil {
			h.logger.Error("Failed to get IP history", logger.Err(err), logger.F("user_id", *userID))
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Failed to retrieve IP history",
				"error":   "Database query failed",
			})
			return
		}

		h.ipService.EnrichIPHistoryRecords(ctx, history)

		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    history,
		})
		return
	}

	// Otherwise, return all IP history (this might be expensive, so we limit it)
	db := tracker.GetDB()
	if db == nil {
		h.logger.Error("Database connection is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Database service is not available",
		})
		return
	}

	var history []ip.IPHistory
	if err := db.WithContext(ctx).Limit(limit).Offset(offset).Order("created_at DESC").Find(&history).Error; err != nil {
		h.logger.Error("Failed to get all IP history", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve IP history",
			"error":   "Database query failed",
		})
		return
	}

	h.ipService.EnrichIPHistoryRecords(ctx, history)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    history,
	})
}
