// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/subscription"
	"v/pkg/errors"
)

// SubscriptionHandler handles subscription-related requests.
type SubscriptionHandler struct {
	service                    *subscription.Service
	logger                     logger.Logger
	profileUpdateIntervalHours int
}

// NewSubscriptionHandler creates a new SubscriptionHandler.
func NewSubscriptionHandler(service *subscription.Service, log logger.Logger, profileUpdateIntervalHours ...int) *SubscriptionHandler {
	interval := 24
	if len(profileUpdateIntervalHours) > 0 && profileUpdateIntervalHours[0] > 0 {
		interval = profileUpdateIntervalHours[0]
	}

	return &SubscriptionHandler{
		service:                    service,
		logger:                     log,
		profileUpdateIntervalHours: interval,
	}
}

// SubscriptionLinkResponse represents the response for getting subscription link.
type SubscriptionLinkResponse struct {
	Link      string                    `json:"link"`
	ShortLink string                    `json:"short_link,omitempty"`
	Token     string                    `json:"token"`
	ShortCode string                    `json:"short_code,omitempty"`
	Formats   []subscription.FormatInfo `json:"formats"`
	Stats     *AccessStats              `json:"stats,omitempty"`
}

// AccessStats represents subscription access statistics.
type AccessStats struct {
	TotalAccess  int64  `json:"total_access"`
	LastAccessAt string `json:"last_access_at,omitempty"`
	LastIP       string `json:"last_ip,omitempty"`
}

// GetLink returns the subscription link for the current user.
// GET /api/subscription/link
func (h *SubscriptionHandler) GetLink(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warn("subscription link request without user_id in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	if err := h.service.CheckUserAccess(c.Request.Context(), userID.(int64)); err != nil {
		if errors.IsForbidden(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to check subscription access", logger.F("error", err), logger.UserID(userID.(int64)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check subscription access"})
		return
	}

	info, err := h.service.GetSubscriptionInfoWithBaseURL(c.Request.Context(), userID.(int64), subscriptionBaseURLFromRequest(c))
	if err != nil {
		h.logger.Error("failed to get subscription info",
			logger.F("error", err),
			logger.F("error_type", fmt.Sprintf("%T", err)),
			logger.UserID(userID.(int64)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription info"})
		return
	}

	response := SubscriptionLinkResponse{
		Link:      info.Link,
		ShortLink: info.ShortLink,
		Token:     info.Token,
		ShortCode: info.ShortCode,
		Formats:   info.Formats,
		Stats: &AccessStats{
			TotalAccess: info.AccessCount,
		},
	}

	if info.LastAccessAt != nil {
		response.Stats.LastAccessAt = info.LastAccessAt.Format("2006-01-02T15:04:05Z")
	}

	c.JSON(http.StatusOK, response)
}

// GetInfo returns detailed subscription information for the current user.
// GET /api/subscription/info
func (h *SubscriptionHandler) GetInfo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	if err := h.service.CheckUserAccess(c.Request.Context(), userID.(int64)); err != nil {
		if errors.IsForbidden(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to check subscription access", logger.F("error", err), logger.UserID(userID.(int64)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check subscription access"})
		return
	}

	info, err := h.service.GetSubscriptionInfoWithBaseURL(c.Request.Context(), userID.(int64), subscriptionBaseURLFromRequest(c))
	if err != nil {
		h.logger.Error("failed to get subscription info", logger.F("error", err), logger.UserID(userID.(int64)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription info"})
		return
	}

	c.JSON(http.StatusOK, info)
}

// Regenerate regenerates the subscription token for the current user.
// POST /api/subscription/regenerate
func (h *SubscriptionHandler) Regenerate(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	if err := h.service.CheckUserAccess(c.Request.Context(), userID.(int64)); err != nil {
		if errors.IsForbidden(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to check subscription access", logger.F("error", err), logger.UserID(userID.(int64)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check subscription access"})
		return
	}

	sub, err := h.service.RegenerateToken(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("failed to regenerate token", logger.F("error", err), logger.UserID(userID.(int64)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to regenerate token"})
		return
	}

	h.logger.Info("subscription token regenerated", logger.UserID(userID.(int64)))

	// Get full subscription info
	info, err := h.service.GetSubscriptionInfoWithBaseURL(c.Request.Context(), userID.(int64), subscriptionBaseURLFromRequest(c))
	if err != nil {
		// Return basic info if we can't get full info
		c.JSON(http.StatusOK, gin.H{
			"token":      sub.Token,
			"short_code": sub.ShortCode,
			"message":    "Token regenerated successfully",
		})
		return
	}

	c.JSON(http.StatusOK, info)
}

func subscriptionBaseURLFromRequest(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}

	scheme := "http"
	if xfProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfProto != "" {
		scheme = strings.Split(xfProto, ",")[0]
	} else if c.Request.TLS != nil {
		scheme = "https"
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	host = strings.Split(host, ",")[0]
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

func subscriptionRequestHostname(c *gin.Context) string {
	baseURL := subscriptionBaseURLFromRequest(c)
	if baseURL == "" {
		return ""
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	return parsed.Hostname()
}

func subscriptionFormatDisplayName(format subscription.ClientFormat) string {
	switch format {
	case subscription.FormatClash:
		return "Clash"
	case subscription.FormatClashMeta:
		return "Clash Meta"
	case subscription.FormatShadowrocket:
		return "Shadowrocket"
	case subscription.FormatSurge:
		return "Surge"
	case subscription.FormatQuantumultX:
		return "Quantumult X"
	case subscription.FormatSingbox:
		return "Sing-box"
	case subscription.FormatV2rayN:
		return "V2RayN"
	default:
		return "Subscription"
	}
}

func sanitizeSubscriptionFilenameComponent(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '.' || r == '-' || r == '_' || unicode.IsSpace(r):
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}

func buildSubscriptionPresentation(c *gin.Context, format subscription.ClientFormat, fileExt string) (string, string) {
	hostname := subscriptionRequestHostname(c)
	formatName := subscriptionFormatDisplayName(format)

	titleBase := hostname
	if titleBase == "" {
		titleBase = "V Panel"
	}
	title := fmt.Sprintf("%s - %s", titleBase, formatName)

	fileBase := sanitizeSubscriptionFilenameComponent(hostname)
	if fileBase == "" {
		fileBase = "v-panel"
	}
	formatBase := sanitizeSubscriptionFilenameComponent(string(format))
	if formatBase == "" {
		formatBase = "subscription"
	}

	return title, fmt.Sprintf("%s-%s.%s", fileBase, formatBase, fileExt)
}

func buildSubscriptionResourceURLs(c *gin.Context) (string, string) {
	baseURL := subscriptionBaseURLFromRequest(c)
	if baseURL == "" {
		return "", ""
	}

	return baseURL + "/user/subscription", baseURL + "/user/help"
}

// GetContent returns subscription content by token.
// GET /api/subscription/:token
func (h *SubscriptionHandler) GetContent(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	// Validate token and get subscription
	sub, err := h.service.ValidateToken(c.Request.Context(), token)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}
		h.logger.Error("failed to validate token", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate token"})
		return
	}

	h.serveSubscriptionContent(c, sub)
}

// GetShortContent returns subscription content by short code.
// GET /s/:code
func (h *SubscriptionHandler) GetShortContent(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Short code is required"})
		return
	}

	// Validate short code and get subscription
	sub, err := h.service.ValidateShortCode(c.Request.Context(), code)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}
		h.logger.Error("failed to validate short code", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate short code"})
		return
	}

	h.serveSubscriptionContent(c, sub)
}

// serveSubscriptionContent serves subscription content for a validated subscription.
func (h *SubscriptionHandler) serveSubscriptionContent(c *gin.Context, sub *repository.Subscription) {
	ctx := c.Request.Context()

	// Check user access (disabled, expired, traffic exceeded)
	if err := h.service.CheckUserAccess(ctx, sub.UserID); err != nil {
		if errors.IsForbidden(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to check user access", logger.F("error", err), logger.UserID(sub.UserID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check access"})
		return
	}

	// Detect format from query param or User-Agent
	format := h.detectFormat(c)

	// Parse content options from query params
	options := h.parseContentOptions(c)

	// Generate content
	content, contentType, fileExt, err := h.service.GenerateContent(ctx, sub.UserID, format, options)
	if err != nil {
		h.logger.Error("failed to generate content", logger.F("error", err), logger.UserID(sub.UserID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate subscription content"})
		return
	}

	// Update access stats
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()
	if err := h.service.UpdateAccessStats(ctx, sub.ID, clientIP, userAgent); err != nil {
		h.logger.Warn("failed to update access stats", logger.F("error", err), logger.F("subscription_id", sub.ID))
	}

	// Set response headers
	h.setSubscriptionHeaders(c, ctx, sub, format, fileExt)

	// Return content
	c.Data(http.StatusOK, contentType, content)
}

// detectFormat detects the subscription format from query param or User-Agent.
func (h *SubscriptionHandler) detectFormat(c *gin.Context) subscription.ClientFormat {
	// Check explicit format parameter first
	if formatParam := c.Query("format"); formatParam != "" {
		switch strings.ToLower(formatParam) {
		case "v2ray", "v2rayn", "v2rayng", "base64", "raw":
			return subscription.FormatV2rayN
		case "clash":
			return subscription.FormatClash
		case "clashmeta", "clash.meta", "mihomo":
			return subscription.FormatClashMeta
		case "shadowrocket":
			return subscription.FormatShadowrocket
		case "surge":
			return subscription.FormatSurge
		case "quantumultx", "quantumult":
			return subscription.FormatQuantumultX
		case "singbox", "sing-box":
			return subscription.FormatSingbox
		}
	}

	// Fall back to User-Agent detection
	return h.service.DetectClientFormat(c.Request.UserAgent())
}

// parseContentOptions parses content options from query parameters.
func (h *SubscriptionHandler) parseContentOptions(c *gin.Context) *subscription.ContentOptions {
	options := &subscription.ContentOptions{}

	// Parse protocols filter
	if protocols := c.Query("protocols"); protocols != "" {
		options.Protocols = strings.Split(protocols, ",")
	}

	// Parse include filter
	if include := c.Query("include"); include != "" {
		for _, idStr := range strings.Split(include, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				options.Include = append(options.Include, id)
			}
		}
	}

	// Parse exclude filter
	if exclude := c.Query("exclude"); exclude != "" {
		for _, idStr := range strings.Split(exclude, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				options.Exclude = append(options.Exclude, id)
			}
		}
	}

	// Parse rename template
	if rename := c.Query("rename"); rename != "" {
		options.RenameTemplate = rename
	}

	return options
}

// setSubscriptionHeaders sets the standard subscription response headers.
func (h *SubscriptionHandler) setSubscriptionHeaders(c *gin.Context, ctx context.Context, sub *repository.Subscription, format subscription.ClientFormat, fileExt string) {
	title, filename := buildSubscriptionPresentation(c, format, fileExt)
	profileWebPageURL, supportURL := buildSubscriptionResourceURLs(c)

	// Content-Disposition for download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Profile-Update-Interval (in hours)
	c.Header("Profile-Update-Interval", strconv.Itoa(h.profileUpdateIntervalHours))

	userinfoHeader := "upload=0; download=0; total=0; expire=0"
	userinfo, err := h.service.GetSubscriptionUserinfo(ctx, sub.UserID)
	if err != nil {
		h.logger.Warn("failed to load subscription userinfo", logger.F("error", err), logger.UserID(sub.UserID))
	} else {
		userinfoHeader = fmt.Sprintf(
			"upload=%d; download=%d; total=%d; expire=%d",
			userinfo.Upload,
			userinfo.Download,
			userinfo.Total,
			userinfo.Expire,
		)
	}
	c.Header("Subscription-Userinfo", userinfoHeader)

	// Profile metadata
	c.Header("Profile-Title", title)
	if profileWebPageURL != "" {
		c.Header("Profile-Web-Page-URL", profileWebPageURL)
	}
	if supportURL != "" {
		c.Header("Support-URL", supportURL)
	}

	// Cache control
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

// AdminSubscriptionItem represents a subscription item in admin list.
type AdminSubscriptionItem struct {
	ID           int64  `json:"id"`
	UserID       int64  `json:"user_id"`
	Username     string `json:"username,omitempty"`
	Token        string `json:"token"`
	ShortCode    string `json:"short_code,omitempty"`
	CreatedAt    string `json:"created_at"`
	LastAccessAt string `json:"last_access_at,omitempty"`
	AccessCount  int64  `json:"access_count"`
	LastIP       string `json:"last_ip,omitempty"`
}

// AdminSubscriptionListResponse represents admin subscription list response.
type AdminSubscriptionListResponse struct {
	Subscriptions []AdminSubscriptionItem `json:"subscriptions"`
	Total         int64                   `json:"total"`
	Page          int                     `json:"page"`
	PageSize      int                     `json:"page_size"`
}

// AdminList returns all subscriptions (admin only).
// GET /api/admin/subscriptions
func (h *SubscriptionHandler) AdminList(c *gin.Context) {
	// Parse pagination params
	page := 1
	pageSize := 20

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	// Build filter
	filter := &repository.SubscriptionFilter{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}

	// Parse optional user_id filter
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			filter.UserID = &userID
		}
	}
	if minAccessCountStr := c.Query("min_access_count"); minAccessCountStr != "" {
		if minAccessCount, err := strconv.ParseInt(minAccessCountStr, 10, 64); err == nil {
			filter.MinAccessCount = &minAccessCount
		}
	}
	if maxAccessCountStr := c.Query("max_access_count"); maxAccessCountStr != "" {
		if maxAccessCount, err := strconv.ParseInt(maxAccessCountStr, 10, 64); err == nil {
			filter.MaxAccessCount = &maxAccessCount
		}
	}

	// Get subscriptions
	subscriptions, total, err := h.service.ListAllSubscriptions(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list subscriptions", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list subscriptions"})
		return
	}

	// Build response with user information
	items := make([]AdminSubscriptionItem, len(subscriptions))
	for i, sub := range subscriptions {
		items[i] = AdminSubscriptionItem{
			ID:          sub.ID,
			UserID:      sub.UserID,
			Token:       sub.Token,
			ShortCode:   sub.ShortCode,
			CreatedAt:   sub.CreatedAt.Format("2006-01-02T15:04:05Z"),
			AccessCount: sub.AccessCount,
			LastIP:      sub.LastIP,
		}

		// Add username if User relation is loaded
		if sub.User != nil {
			items[i].Username = sub.User.Username
		}

		if sub.LastAccessAt != nil {
			items[i].LastAccessAt = sub.LastAccessAt.Format("2006-01-02T15:04:05Z")
		}
	}

	c.JSON(http.StatusOK, AdminSubscriptionListResponse{
		Subscriptions: items,
		Total:         total,
		Page:          page,
		PageSize:      pageSize,
	})
}

// AdminRevoke revokes a user's subscription (admin only).
// DELETE /api/admin/subscriptions/:user_id
func (h *SubscriptionHandler) AdminRevoke(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.service.RevokeSubscription(c.Request.Context(), userID); err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}
		h.logger.Error("failed to revoke subscription", logger.F("error", err), logger.UserID(userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke subscription"})
		return
	}

	h.logger.Info("subscription revoked by admin", logger.UserID(userID))
	c.JSON(http.StatusOK, gin.H{"message": "Subscription revoked successfully"})
}

// AdminResetStats resets access statistics for a subscription (admin only).
// POST /api/admin/subscriptions/:user_id/reset-stats
func (h *SubscriptionHandler) AdminResetStats(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get subscription by user ID first
	sub, err := h.service.GetOrCreateSubscription(c.Request.Context(), userID)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}
		h.logger.Error("failed to get subscription", logger.F("error", err), logger.UserID(userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription"})
		return
	}

	if err := h.service.ResetAccessStats(c.Request.Context(), sub.ID); err != nil {
		h.logger.Error("failed to reset stats", logger.F("error", err), logger.UserID(userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset statistics"})
		return
	}

	h.logger.Info("subscription stats reset by admin", logger.UserID(userID))
	c.JSON(http.StatusOK, gin.H{"message": "Statistics reset successfully"})
}
