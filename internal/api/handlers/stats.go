// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/pkg/errors"
)

// StatsHandler handles statistics-related requests.
type StatsHandler struct {
	logger logger.Logger
	repos  *repository.Repositories
	cache  cache.Cache
}

// Cache keys and TTLs for statistics
const (
	statsCacheTTL           = 30 * time.Second // Short TTL for real-time stats
	dashboardStatsCacheKey  = "stats:dashboard"
	protocolStatsCacheKey   = "stats:protocol"
	trafficStatsCachePrefix = "stats:traffic:"
	userStatsCachePrefix    = "stats:user:"
)

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(log logger.Logger, repos *repository.Repositories, c cache.Cache) *StatsHandler {
	return &StatsHandler{
		logger: log,
		repos:  repos,
		cache:  c,
	}
}

// getRequestID extracts request ID from context.
func getRequestID(c *gin.Context) string {
	if id, exists := c.Get("request_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

func parseStatsRange(c *gin.Context) (string, time.Time, time.Time, bool, *errors.AppError) {
	period := strings.TrimSpace(c.DefaultQuery("period", "today"))
	startStr := strings.TrimSpace(c.Query("start"))
	endStr := strings.TrimSpace(c.Query("end"))

	if period == "custom" || startStr != "" || endStr != "" {
		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			return "", time.Time{}, time.Time{}, false, errors.NewValidationError("invalid start date", err)
		}
		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return "", time.Time{}, time.Time{}, false, errors.NewValidationError("invalid end date", err)
		}
		if end.Before(start) {
			return "", time.Time{}, time.Time{}, false, errors.NewValidationError("end date must be after start date", nil)
		}
		return "custom", start, end, false, nil
	}

	start, end := getPeriodRange(period)
	return period, start, end, true, nil
}

// DashboardStats represents dashboard statistics.
type DashboardStats struct {
	TotalUsers      int64 `json:"total_users"`
	ActiveUsers     int64 `json:"active_users"`
	TotalProxies    int64 `json:"total_proxies"`
	ActiveProxies   int64 `json:"active_proxies"`
	TotalTraffic    int64 `json:"total_traffic"`
	UploadTraffic   int64 `json:"upload_traffic"`
	DownloadTraffic int64 `json:"download_traffic"`
	OnlineCount     int   `json:"online_count"`
}

// GetDashboardStats returns dashboard statistics.
func (h *StatsHandler) GetDashboardStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Try to get from cache first
	if h.cache != nil {
		if cached, err := h.cache.Get(ctx, dashboardStatsCacheKey); err == nil && cached != nil {
			var stats DashboardStats
			if err := json.Unmarshal(cached, &stats); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    stats,
				})
				return
			}
		}
	}

	stats := DashboardStats{}

	// Get total users
	totalUsers, err := h.repos.User.Count(ctx)
	if err != nil {
		h.logger.Error("failed to count users", logger.F("error", err))
	} else {
		stats.TotalUsers = totalUsers
	}

	// Get active users
	activeUsers, err := h.repos.User.CountActive(ctx)
	if err != nil {
		h.logger.Error("failed to count active users", logger.F("error", err))
	} else {
		stats.ActiveUsers = activeUsers
	}

	// Get total proxies
	totalProxies, err := h.repos.Proxy.Count(ctx)
	if err != nil {
		h.logger.Error("failed to count proxies", logger.F("error", err))
	} else {
		stats.TotalProxies = totalProxies
	}

	// Get active proxies
	activeProxies, err := h.repos.Proxy.CountEnabled(ctx)
	if err != nil {
		h.logger.Error("failed to count enabled proxies", logger.F("error", err))
	} else {
		stats.ActiveProxies = activeProxies
	}

	// Get total traffic
	upload, download, err := h.repos.Traffic.GetTotalTraffic(ctx)
	if err != nil {
		h.logger.Error("failed to get total traffic", logger.F("error", err))
	} else {
		stats.UploadTraffic = upload
		stats.DownloadTraffic = download
		stats.TotalTraffic = upload + download
	}

	if h.repos != nil && h.repos.Node != nil {
		statusCounts, err := h.repos.Node.CountByStatus(ctx)
		if err != nil {
			h.logger.Warn("failed to count nodes by status", logger.F("error", err))
		} else {
			stats.OnlineCount = int(statusCounts[repository.NodeStatusOnline])
		}
	}

	// Cache the result
	if h.cache != nil {
		if data, err := json.Marshal(stats); err == nil {
			if err := h.cache.Set(ctx, dashboardStatsCacheKey, data, statsCacheTTL); err != nil {
				h.logger.Warn("failed to cache dashboard stats", logger.F("error", err))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// ProtocolStats represents protocol statistics.
type ProtocolStats struct {
	Protocol string `json:"protocol"`
	Count    int64  `json:"count"`
	Traffic  int64  `json:"traffic"`
	Status   string `json:"status"`
}

func buildProtocolStats(protocolCounts []*repository.ProtocolCount, trafficStats []*repository.ProtocolTrafficStats) []ProtocolStats {
	statsByProtocol := make(map[string]ProtocolStats, len(protocolCounts)+len(trafficStats))

	for _, pc := range protocolCounts {
		if pc == nil {
			continue
		}
		statsByProtocol[pc.Protocol] = ProtocolStats{
			Protocol: pc.Protocol,
			Count:    pc.Count,
			Status:   "active",
		}
	}

	for _, ts := range trafficStats {
		if ts == nil {
			continue
		}
		ps := statsByProtocol[ts.Protocol]
		if ps.Protocol == "" {
			ps.Protocol = ts.Protocol
			ps.Status = "active"
		}
		ps.Traffic = ts.Upload + ts.Download
		statsByProtocol[ts.Protocol] = ps
	}

	defaultProtocols := []string{"vmess", "vless", "trojan", "shadowsocks"}
	for _, protocol := range defaultProtocols {
		if _, exists := statsByProtocol[protocol]; !exists {
			statsByProtocol[protocol] = ProtocolStats{
				Protocol: protocol,
				Count:    0,
				Traffic:  0,
				Status:   "active",
			}
		}
	}

	protocolOrder := make([]string, 0, len(statsByProtocol))
	seen := make(map[string]struct{}, len(statsByProtocol))
	for _, protocol := range defaultProtocols {
		if _, exists := statsByProtocol[protocol]; exists {
			protocolOrder = append(protocolOrder, protocol)
			seen[protocol] = struct{}{}
		}
	}
	for _, pc := range protocolCounts {
		if pc == nil {
			continue
		}
		if _, exists := seen[pc.Protocol]; exists {
			continue
		}
		protocolOrder = append(protocolOrder, pc.Protocol)
		seen[pc.Protocol] = struct{}{}
	}
	for _, ts := range trafficStats {
		if ts == nil {
			continue
		}
		if _, exists := seen[ts.Protocol]; exists {
			continue
		}
		protocolOrder = append(protocolOrder, ts.Protocol)
		seen[ts.Protocol] = struct{}{}
	}

	stats := make([]ProtocolStats, 0, len(protocolOrder))
	for _, protocol := range protocolOrder {
		stats = append(stats, statsByProtocol[protocol])
	}
	return stats
}

// GetProtocolStats returns protocol statistics.
func (h *StatsHandler) GetProtocolStats(c *gin.Context) {
	ctx := c.Request.Context()
	period, start, end, cacheable, rangeErr := parseStatsRange(c)
	if rangeErr != nil {
		c.JSON(http.StatusBadRequest, rangeErr.ToResponse(getRequestID(c)))
		return
	}

	// Try to get from cache first
	cacheKey := ""
	if cacheable {
		cacheKey = fmt.Sprintf("%s:%s", protocolStatsCacheKey, period)
	}
	if cacheKey != "" && h.cache != nil {
		if cached, err := h.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			var stats []ProtocolStats
			if err := json.Unmarshal(cached, &stats); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    stats,
				})
				return
			}
		}
	}

	// Get proxy counts by protocol
	protocolCounts, err := h.repos.Proxy.CountByProtocol(ctx)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get protocol counts", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, errors.NewInternalError("failed to get protocol stats", err).ToResponse(getRequestID(c)))
		return
	}

	// Get traffic by protocol
	trafficStats, err := h.repos.Traffic.GetTrafficByProtocol(ctx, start, end)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get traffic by protocol", logger.F("error", err))
		trafficStats = []*repository.ProtocolTrafficStats{}
	}

	stats := buildProtocolStats(protocolCounts, trafficStats)

	// Cache the result
	if cacheKey != "" && h.cache != nil {
		if data, err := json.Marshal(stats); err == nil {
			if err := h.cache.Set(ctx, cacheKey, data, statsCacheTTL); err != nil {
				h.logger.Warn("failed to cache protocol stats", logger.F("error", err))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// TrafficStats represents traffic statistics.
type TrafficStats struct {
	Total      int64   `json:"total"`
	Upload     int64   `json:"up"`
	Download   int64   `json:"down"`
	Limit      int64   `json:"limit"`
	UserLimit  int64   `json:"user_limit"`
	NodeLimit  int64   `json:"node_limit"`
	Percentage float64 `json:"percentage"`
}

func effectiveTrafficLimit(userLimit, nodeLimit int64) int64 {
	switch {
	case userLimit > 0 && nodeLimit > 0:
		if userLimit < nodeLimit {
			return userLimit
		}
		return nodeLimit
	case userLimit > 0:
		return userLimit
	case nodeLimit > 0:
		return nodeLimit
	default:
		return 0
	}
}

func (h *StatsHandler) getTrafficLimitSummary(ctx context.Context) (userLimit int64, nodeLimit int64, err error) {
	if h == nil || h.repos == nil || h.repos.DB() == nil {
		return 0, 0, nil
	}

	db := h.repos.DB().WithContext(ctx)
	now := time.Now()

	if err := db.Model(&repository.User{}).
		Select("COALESCE(SUM(traffic_limit), 0)").
		Where("enabled = ?", true).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Where("traffic_limit > 0").
		Scan(&userLimit).Error; err != nil {
		return 0, 0, err
	}

	if err := db.Model(&repository.Node{}).
		Select("COALESCE(SUM(traffic_limit), 0)").
		Where("status = ?", repository.NodeStatusOnline).
		Where("traffic_limit > 0").
		Scan(&nodeLimit).Error; err != nil {
		return 0, 0, err
	}

	return userLimit, nodeLimit, nil
}

// GetTrafficStats returns traffic statistics.
func (h *StatsHandler) GetTrafficStats(c *gin.Context) {
	ctx := c.Request.Context()
	period, start, end, cacheable, rangeErr := parseStatsRange(c)
	if rangeErr != nil {
		c.JSON(http.StatusBadRequest, rangeErr.ToResponse(getRequestID(c)))
		return
	}
	cacheKey := ""
	if cacheable {
		cacheKey = fmt.Sprintf("%s%s", trafficStatsCachePrefix, period)
	}

	// Try to get from cache first (only for non-custom periods)
	if cacheKey != "" && h.cache != nil {
		if cached, err := h.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			var stats TrafficStats
			if err := json.Unmarshal(cached, &stats); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    stats,
				})
				return
			}
		}
	}

	upload, download, err := h.repos.Traffic.GetTotalTrafficByPeriod(ctx, start, end)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get traffic stats", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, errors.NewInternalError("failed to get traffic stats", err).ToResponse(getRequestID(c)))
		return
	}

	total := upload + download
	userLimit, nodeLimit, limitErr := h.getTrafficLimitSummary(ctx)
	if limitErr != nil {
		h.logger.Warn("failed to get traffic limit summary", logger.F("error", limitErr))
	}
	limit := effectiveTrafficLimit(userLimit, nodeLimit)
	percentage := float64(0)
	if limit > 0 {
		percentage = float64(total) / float64(limit) * 100
	}

	stats := TrafficStats{
		Total:      total,
		Upload:     upload,
		Download:   download,
		Limit:      limit,
		UserLimit:  userLimit,
		NodeLimit:  nodeLimit,
		Percentage: percentage,
	}

	// Cache the result (only for non-custom periods)
	if cacheKey != "" && h.cache != nil {
		if data, err := json.Marshal(stats); err == nil {
			if err := h.cache.Set(ctx, cacheKey, data, statsCacheTTL); err != nil {
				h.logger.Warn("failed to cache traffic stats", logger.F("error", err))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// UserStats represents user statistics.
type UserStats struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Upload       int64  `json:"upload"`
	Download     int64  `json:"download"`
	Total        int64  `json:"total"`
	ProxyCount   int64  `json:"proxy_count"`
	TrafficLimit int64  `json:"traffic_limit"`
	LastActive   string `json:"last_active"`
}

// GetUserStats returns user statistics.
func (h *StatsHandler) GetUserStats(c *gin.Context) {
	ctx := c.Request.Context()
	limit := 10 // Default limit
	period, start, end, cacheable, rangeErr := parseStatsRange(c)
	if rangeErr != nil {
		c.JSON(http.StatusBadRequest, rangeErr.ToResponse(getRequestID(c)))
		return
	}

	// Try to get from cache first
	cacheKey := ""
	if cacheable {
		cacheKey = fmt.Sprintf("%s%s", userStatsCachePrefix, period)
	}
	if cacheKey != "" && h.cache != nil {
		if cached, err := h.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			var stats []UserStats
			if err := json.Unmarshal(cached, &stats); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    stats,
				})
				return
			}
		}
	}

	trafficStats, err := h.repos.Traffic.GetTrafficByUser(ctx, start, end, limit)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get user stats", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, errors.NewInternalError("failed to get user stats", err).ToResponse(getRequestID(c)))
		return
	}

	stats := make([]UserStats, 0, len(trafficStats))
	for _, ts := range trafficStats {
		lastActive := ""
		if ts.LastActive != nil {
			lastActive = ts.LastActive.Format(time.RFC3339)
		}
		stats = append(stats, UserStats{
			UserID:       ts.UserID,
			Username:     ts.Username,
			Email:        ts.Email,
			Upload:       ts.Upload,
			Download:     ts.Download,
			Total:        ts.Upload + ts.Download,
			ProxyCount:   ts.ProxyCount,
			TrafficLimit: ts.TrafficLimit,
			LastActive:   lastActive,
		})
	}

	// Cache the result
	if cacheKey != "" && h.cache != nil {
		if data, err := json.Marshal(stats); err == nil {
			if err := h.cache.Set(ctx, cacheKey, data, statsCacheTTL); err != nil {
				h.logger.Warn("failed to cache user stats", logger.F("error", err))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// TimelinePoint represents a point in the timeline.
type TimelinePoint struct {
	Time     string `json:"time"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// DetailedStats represents detailed statistics.
type DetailedStats struct {
	Period       string          `json:"period"`
	TotalTraffic int64           `json:"total_traffic"`
	Upload       int64           `json:"upload"`
	Download     int64           `json:"download"`
	ByProtocol   []ProtocolStats `json:"by_protocol"`
	ByUser       []UserStats     `json:"by_user"`
	Timeline     []TimelinePoint `json:"timeline"`
}

// GetDetailedStats returns detailed statistics.
func (h *StatsHandler) GetDetailedStats(c *gin.Context) {
	ctx := c.Request.Context()
	period, start, end, _, rangeErr := parseStatsRange(c)
	if rangeErr != nil {
		c.JSON(http.StatusBadRequest, rangeErr.ToResponse(getRequestID(c)))
		return
	}

	// Get total traffic
	upload, download, err := h.repos.Traffic.GetTotalTrafficByPeriod(ctx, start, end)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get total traffic", logger.F("error", err))
	}

	stats := DetailedStats{
		Period:       period,
		TotalTraffic: upload + download,
		Upload:       upload,
		Download:     download,
		ByProtocol:   []ProtocolStats{},
		ByUser:       []UserStats{},
		Timeline:     []TimelinePoint{},
	}

	// Get protocol stats
	protocolCounts, err := h.repos.Proxy.CountByProtocol(ctx)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get protocol counts", logger.F("error", err))
	} else {
		trafficByProtocol, trafficErr := h.repos.Traffic.GetTrafficByProtocol(ctx, start, end)
		if trafficErr != nil {
			if handleRequestContextError(c, trafficErr) {
				return
			}
			h.logger.Error("failed to get traffic by protocol", logger.F("error", trafficErr))
		} else {
			for _, protocolStat := range buildProtocolStats(protocolCounts, trafficByProtocol) {
				stats.ByProtocol = append(stats.ByProtocol, protocolStat)
			}
		}
	}

	// Get user stats
	userTraffic, err := h.repos.Traffic.GetTrafficByUser(ctx, start, end, 10)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get user stats", logger.F("error", err))
	} else {
		for _, ut := range userTraffic {
			lastActive := ""
			if ut.LastActive != nil {
				lastActive = ut.LastActive.Format(time.RFC3339)
			}
			stats.ByUser = append(stats.ByUser, UserStats{
				UserID:       ut.UserID,
				Username:     ut.Username,
				Email:        ut.Email,
				Upload:       ut.Upload,
				Download:     ut.Download,
				Total:        ut.Upload + ut.Download,
				ProxyCount:   ut.ProxyCount,
				TrafficLimit: ut.TrafficLimit,
				LastActive:   lastActive,
			})
		}
	}

	// Get timeline data
	interval := getIntervalForPeriod(period)
	timeline, err := h.repos.Traffic.GetTrafficTimeline(ctx, start, end, interval)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get traffic timeline", logger.F("error", err))
	} else {
		for _, tp := range timeline {
			stats.Timeline = append(stats.Timeline, TimelinePoint{
				Time:     tp.Time.Format(time.RFC3339),
				Upload:   tp.Upload,
				Download: tp.Download,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// getPeriodRange returns the start and end time for a given period.
func getPeriodRange(period string) (start, end time.Time) {
	return getPeriodRangeAt(time.Now(), period)
}

func getPeriodRangeAt(now time.Time, period string) (start, end time.Time) {
	end = now

	switch period {
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		start = startOfWeek(now)
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "year":
		start = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location())
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	return start, end
}

func startOfWeek(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	date := now.AddDate(0, 0, -(weekday - 1))
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

// getIntervalForPeriod returns the appropriate interval for timeline data.
func getIntervalForPeriod(period string) string {
	switch period {
	case "today":
		return "hour"
	case "week":
		return "day"
	case "month":
		return "day"
	case "year":
		return "month"
	default:
		return "hour"
	}
}
