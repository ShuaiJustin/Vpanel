package ip

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// NotificationSender interface for sending notifications
type NotificationSender interface {
	NotifyNewDevice(data NotificationData) error
	NotifyIPLimitReached(data NotificationData) error
	NotifySuspiciousActivity(data NotificationData) error
	NotifyDeviceKicked(data NotificationData) error
	NotifyAutoBlacklisted(data NotificationData) error
}

// NotificationData contains data for IP-related notifications
type NotificationData struct {
	UserID       uint
	Username     string
	Email        string
	IP           string
	Country      string
	City         string
	DeviceInfo   string
	Reason       string
	CurrentCount int
	MaxCount     int
	Timestamp    time.Time
}

// ProxySessionActivity represents a recently active proxy session reported by a node.
type ProxySessionActivity struct {
	UserID     uint
	ProxyID    int64
	IP         string
	LastSeen   time.Time
	DeviceInfo string
}

// Error codes for IP restriction.
const (
	ErrCodeIPLimitExceeded     = "IP_LIMIT_EXCEEDED"
	ErrCodeIPBlacklisted       = "IP_BLACKLISTED"
	ErrCodeGeoRestricted       = "GEO_RESTRICTED"
	ErrCodeSubscriptionIPLimit = "SUBSCRIPTION_IP_LIMIT"
	ErrCodeIPKickFailed        = "IP_KICK_FAILED"
	ErrCodeInvalidCIDR         = "INVALID_CIDR"
	ErrCodeGeolocationFailed   = "GEOLOCATION_FAILED"
)

// Service provides the main IP restriction functionality.
type Service struct {
	db         *gorm.DB
	validator  *Validator
	tracker    *Tracker
	geoService *GeolocationService
	settings   *IPRestrictionSettings
	notifier   NotificationSender

	geoWarmupMu sync.Mutex
	geoWarmups  map[string]struct{}
}

// ServiceConfig holds configuration for the IP restriction service.
type ServiceConfig struct {
	GeoConfig *GeolocationConfig
	Notifier  NotificationSender
}

// NewService creates a new IP restriction service.
func NewService(db *gorm.DB, config *ServiceConfig) (*Service, error) {
	var geoConfig *GeolocationConfig
	var notifier NotificationSender

	if config != nil {
		geoConfig = config.GeoConfig
		notifier = config.Notifier
	}

	// Create geolocation service - it will work even without GeoIP database
	geoService, err := NewGeolocationService(db, geoConfig)
	if err != nil {
		// Log warning but continue - geolocation is optional
		geoService = nil
	}

	return &Service{
		db:         db,
		validator:  NewValidator(db),
		tracker:    NewTracker(db),
		geoService: geoService,
		settings:   DefaultIPRestrictionSettings(),
		notifier:   notifier,
		geoWarmups: make(map[string]struct{}),
	}, nil
}

// Close closes the service and releases resources.
func (s *Service) Close() error {
	if s.geoService != nil {
		return s.geoService.Close()
	}
	return nil
}

// LoadSettings loads IP restriction settings from the database.
func (s *Service) LoadSettings(ctx context.Context) error {
	var setting struct {
		Value string
	}
	err := s.db.WithContext(ctx).
		Table("settings").
		Where("`key` = ?", "ip_restriction").
		Select("value").
		First(&setting).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Use default settings
			return nil
		}
		return err
	}

	var settings IPRestrictionSettings
	if err := json.Unmarshal([]byte(setting.Value), &settings); err != nil {
		return err
	}

	s.settings = &settings
	return nil
}

// SaveSettings saves IP restriction settings to the database.
func (s *Service) SaveSettings(ctx context.Context, settings *IPRestrictionSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Exec(
		"INSERT OR REPLACE INTO settings (`key`, value, updated_at) VALUES (?, ?, ?)",
		"ip_restriction", string(data), time.Now(),
	).Error
}

// GetSettings returns the current settings.
func (s *Service) GetSettings() *IPRestrictionSettings {
	return s.settings
}

// SetNotifier sets the notification sender.
func (s *Service) SetNotifier(notifier NotificationSender) {
	s.notifier = notifier
}

// CheckAccess checks if an IP is allowed to access for a user.
func (s *Service) CheckAccess(ctx context.Context, userID uint, ip string, accessType AccessType, maxConcurrentIPs int) (*AccessResult, error) {
	if !s.settings.Enabled {
		return &AccessResult{Allowed: true}, nil
	}

	// Check whitelist first - whitelisted IPs bypass all checks
	if s.validator.IsWhitelisted(ctx, ip, &userID) {
		return &AccessResult{Allowed: true, Reason: "whitelisted"}, nil
	}

	// Check blacklist
	if entry, blocked := s.validator.IsBlacklisted(ctx, ip, &userID); blocked {
		return &AccessResult{
			Allowed: false,
			Code:    ErrCodeIPBlacklisted,
			Reason:  fmt.Sprintf("IP is blacklisted: %s", entry.Reason),
		}, nil
	}

	// Check geo restriction
	if s.settings.GeoRestrictionEnabled {
		geoResult, err := s.geoService.CheckGeoRestriction(ctx, ip, s.settings.AllowedCountries, s.settings.BlockedCountries)
		if err == nil && !geoResult.Allowed {
			return &AccessResult{
				Allowed: false,
				Code:    ErrCodeGeoRestricted,
				Reason:  fmt.Sprintf("Access from %s is not allowed: %s", geoResult.Country, geoResult.Reason),
			}, nil
		}
	}

	// Web/API access should not consume proxy device slots. Keep blacklist and
	// geo checks above, then let authenticated users manage their devices even
	// when the proxy device limit has been reached.
	if accessType == AccessTypeAPI {
		if err := s.tracker.RemoveActiveIPUnlessDeviceType(ctx, userID, ip, "proxy"); err != nil {
			return nil, err
		}
		return &AccessResult{Allowed: true, Reason: "api access"}, nil
	}

	// Check concurrent IP limit
	// Use provided maxConcurrentIPs, or fall back to default
	limit := maxConcurrentIPs
	if limit < 0 {
		limit = s.settings.DefaultMaxConcurrentIPs
	}

	// 0 means unlimited
	if limit == 0 {
		return &AccessResult{Allowed: true, Reason: "unlimited"}, nil
	}

	// Clean up inactive IPs first
	timeout := time.Duration(s.settings.InactiveTimeout) * time.Minute
	_, _ = s.tracker.CleanupInactiveIPsForUser(ctx, userID, timeout)

	// Check if IP is already active
	isActive, err := s.tracker.IsIPActive(ctx, userID, ip)
	if err != nil {
		return nil, err
	}

	if isActive {
		// Update last active time
		_ = s.tracker.UpdateLastActive(ctx, userID, ip)
		return &AccessResult{Allowed: true, Reason: "existing session"}, nil
	}

	// Check current active IP count
	count, err := s.tracker.GetActiveIPCount(ctx, userID)
	if err != nil {
		return nil, err
	}

	if count >= limit {
		// Get online IPs for error response
		onlineIPs, _ := s.tracker.GetOnlineIPs(ctx, userID)
		ips := make([]string, len(onlineIPs))
		for i, oip := range onlineIPs {
			ips[i] = oip.IP
		}

		// Send notification for IP limit reached
		if s.notifier != nil {
			var country, city string
			if s.geoService != nil {
				geoInfo, _ := s.geoService.Lookup(ctx, ip)
				if geoInfo != nil {
					country = geoInfo.Country
					city = geoInfo.City
				}
			}
			_ = s.notifier.NotifyIPLimitReached(NotificationData{
				UserID:       userID,
				IP:           ip,
				Country:      country,
				City:         city,
				CurrentCount: count,
				MaxCount:     limit,
				Timestamp:    time.Now(),
			})
		}

		return &AccessResult{
			Allowed:        false,
			Code:           ErrCodeIPLimitExceeded,
			Reason:         fmt.Sprintf("Maximum device limit (%d) reached", limit),
			RemainingSlots: 0,
			OnlineIPs:      ips,
		}, nil
	}

	return &AccessResult{
		Allowed:        true,
		RemainingSlots: limit - count - 1,
	}, nil
}

// RecordActivity records IP activity and adds to active IPs.
func (s *Service) RecordActivity(ctx context.Context, userID uint, ip, userAgent string, accessType AccessType) error {
	// Get geolocation info
	var country, city string
	if s.geoService != nil {
		// Activity recording runs for almost every authenticated request, so keep it local/cache-only.
		geoInfo, err := s.geoService.LookupLocal(ctx, ip)
		if err == nil && geoInfo != nil {
			country = geoInfo.Country
			city = geoInfo.City
		}
		if err != nil || !hasGeolocationDetails(geoInfo) {
			s.warmGeolocationAsync(ip)
		}
	}

	timeout := time.Duration(s.settings.InactiveTimeout) * time.Minute
	if timeout > 0 {
		_, _ = s.tracker.CleanupInactiveIPsForUser(ctx, userID, timeout)
	}

	if accessType == AccessTypeAPI {
		// Browser/API visits are audit history, not online proxy devices. If an
		// older build already inserted this browser IP as an active device, remove
		// it unless it has since been refreshed by the proxy session reporter.
		if err := s.tracker.RemoveActiveIPUnlessDeviceType(ctx, userID, ip, "proxy"); err != nil {
			return err
		}
		return s.tracker.RecordIPHistory(ctx, &IPHistory{
			UserID:     userID,
			IP:         ip,
			UserAgent:  userAgent,
			AccessType: accessType,
			Country:    country,
			City:       city,
			CreatedAt:  time.Now(),
		})
	}

	// Detect device type from user agent
	deviceType := detectDeviceType(userAgent)

	// Check if this is a new device
	isNewDevice := false
	isActive, _ := s.tracker.IsIPActive(ctx, userID, ip)
	if !isActive {
		isNewDevice = true
	}

	// Add to active IPs
	if err := s.tracker.AddActiveIP(ctx, userID, ip, userAgent, deviceType, country, city); err != nil {
		return err
	}

	// Record in history
	record := &IPHistory{
		UserID:     userID,
		IP:         ip,
		UserAgent:  userAgent,
		AccessType: accessType,
		Country:    country,
		City:       city,
		CreatedAt:  time.Now(),
	}

	// Check for suspicious activity
	isSuspicious := s.isSuspiciousActivity(ctx, userID, country)
	if isSuspicious {
		record.IsSuspicious = true
		// Send suspicious activity notification
		if s.notifier != nil {
			_ = s.notifier.NotifySuspiciousActivity(NotificationData{
				UserID:     userID,
				IP:         ip,
				Country:    country,
				City:       city,
				DeviceInfo: userAgent,
				Reason:     "Multiple countries detected in short time window",
				Timestamp:  time.Now(),
			})
		}
	}

	// Send new device notification
	if isNewDevice && s.notifier != nil {
		_ = s.notifier.NotifyNewDevice(NotificationData{
			UserID:     userID,
			IP:         ip,
			Country:    country,
			City:       city,
			DeviceInfo: userAgent,
			Timestamp:  time.Now(),
		})
	}

	return s.tracker.RecordIPHistory(ctx, record)
}

// RecordProxySessions refreshes active proxy sessions reported by a node heartbeat.
// Existing active IPs are touched, while new IPs are added to active_ips and ip_history.
func (s *Service) RecordProxySessions(ctx context.Context, sessions []ProxySessionActivity) error {
	if len(sessions) == 0 {
		return nil
	}

	uniqueSessions := make(map[uint]map[string]ProxySessionActivity)
	for _, session := range sessions {
		if session.UserID == 0 || strings.TrimSpace(session.IP) == "" {
			continue
		}

		userSessions := uniqueSessions[session.UserID]
		if userSessions == nil {
			userSessions = make(map[string]ProxySessionActivity)
			uniqueSessions[session.UserID] = userSessions
		}

		current, exists := userSessions[session.IP]
		if !exists || session.LastSeen.After(current.LastSeen) {
			userSessions[session.IP] = session
		}
	}

	timeout := time.Duration(s.settings.InactiveTimeout) * time.Minute
	now := time.Now()
	for userID, userSessions := range uniqueSessions {
		if timeout > 0 {
			_, _ = s.tracker.CleanupInactiveIPsForUser(ctx, userID, timeout)
		}

		for _, session := range userSessions {
			deviceInfo := strings.TrimSpace(session.DeviceInfo)
			if deviceInfo == "" {
				if session.ProxyID > 0 {
					deviceInfo = fmt.Sprintf("Proxy #%d connection", session.ProxyID)
				} else {
					deviceInfo = "Proxy connection"
				}
			}

			country, city := "", ""
			if s.geoService != nil {
				geoInfo, err := s.geoService.LookupLocal(ctx, session.IP)
				if err == nil && geoInfo != nil {
					country = geoInfo.Country
					city = geoInfo.City
				}
				if err != nil || !hasGeolocationDetails(geoInfo) {
					s.warmGeolocationAsync(session.IP)
				}
			}

			isActive, err := s.tracker.IsIPActive(ctx, userID, session.IP)
			if err != nil {
				return err
			}
			if isActive {
				if err := s.tracker.AddActiveIP(ctx, userID, session.IP, deviceInfo, "proxy", country, city); err != nil {
					return err
				}
				continue
			}

			if err := s.tracker.AddActiveIP(ctx, userID, session.IP, deviceInfo, "proxy", country, city); err != nil {
				return err
			}

			recordedAt := session.LastSeen
			if recordedAt.IsZero() {
				recordedAt = now
			}
			if err := s.tracker.RecordIPHistory(ctx, &IPHistory{
				UserID:     userID,
				IP:         session.IP,
				UserAgent:  deviceInfo,
				AccessType: AccessTypeProxy,
				Country:    country,
				City:       city,
				CreatedAt:  recordedAt,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// isSuspiciousActivity checks if the activity is suspicious.
func (s *Service) isSuspiciousActivity(ctx context.Context, userID uint, currentCountry string) bool {
	if currentCountry == "" {
		return false
	}

	// Get recent countries (last 30 minutes)
	countries, err := s.tracker.GetRecentCountries(ctx, userID, 30)
	if err != nil {
		return false
	}

	// If more than 3 different countries in 30 minutes, it's suspicious
	uniqueCountries := make(map[string]bool)
	uniqueCountries[currentCountry] = true
	for _, c := range countries {
		uniqueCountries[c] = true
	}

	return len(uniqueCountries) > 3
}

// detectDeviceType detects device type from user agent.
func detectDeviceType(userAgent string) string {
	// Simple detection - can be enhanced with a proper library
	ua := userAgent
	if ua == "" {
		return "unknown"
	}

	// Check for mobile indicators
	mobileKeywords := []string{"Mobile", "Android", "iPhone", "iPad", "iPod"}
	for _, keyword := range mobileKeywords {
		if contains(ua, keyword) {
			if contains(ua, "iPad") || contains(ua, "Tablet") {
				return "tablet"
			}
			return "mobile"
		}
	}

	return "desktop"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetOnlineIPs returns online IPs for a user.
func (s *Service) GetOnlineIPs(ctx context.Context, userID uint) ([]OnlineIP, error) {
	onlineIPs, err := s.tracker.GetOnlineIPs(ctx, userID)
	if err != nil {
		return nil, err
	}

	for i := range onlineIPs {
		s.enrichOnlineIP(ctx, userID, &onlineIPs[i])
	}

	return onlineIPs, nil
}

// GetAggregatedIPHistory returns grouped IP history and fills missing geolocation details when available.
func (s *Service) GetAggregatedIPHistory(ctx context.Context, userID uint, limit, offset int) ([]IPHistorySummary, int64, error) {
	summaries, total, err := s.tracker.GetAggregatedIPHistory(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	for i := range summaries {
		s.enrichIPHistorySummary(ctx, userID, &summaries[i])
	}

	return summaries, total, nil
}

func (s *Service) enrichOnlineIP(ctx context.Context, _ uint, onlineIP *OnlineIP) {
	if onlineIP == nil {
		return
	}

	country, city, countryCode, changed := s.resolveMissingGeoFast(ctx, onlineIP.IP, onlineIP.Country, onlineIP.City, onlineIP.CountryCode)
	if !changed {
		return
	}

	onlineIP.Country = country
	onlineIP.City = city
	onlineIP.CountryCode = countryCode
}

func (s *Service) enrichIPHistorySummary(ctx context.Context, _ uint, summary *IPHistorySummary) {
	if summary == nil {
		return
	}

	country, city, countryCode, changed := s.resolveMissingGeoFast(ctx, summary.IP, summary.Country, summary.City, summary.CountryCode)
	if !changed {
		return
	}

	summary.Country = country
	summary.City = city
	summary.CountryCode = countryCode
}

func (s *Service) resolveMissingGeoFast(ctx context.Context, ip, country, city, countryCode string) (string, string, string, bool) {
	if s.geoService == nil || strings.TrimSpace(ip) == "" {
		return country, city, countryCode, false
	}

	needsCountry := strings.TrimSpace(country) == ""
	needsCity := strings.TrimSpace(city) == ""
	needsCountryCode := strings.TrimSpace(countryCode) == ""
	if !needsCountry && !needsCity && !needsCountryCode {
		return country, city, countryCode, false
	}

	geoInfo, err := s.geoService.LookupFast(ctx, ip)
	if err != nil || !hasGeolocationDetails(geoInfo) {
		return country, city, countryCode, false
	}

	changed := false
	if needsCountry && strings.TrimSpace(geoInfo.Country) != "" {
		country = geoInfo.Country
		changed = true
	}
	if needsCity {
		resolvedCity := strings.TrimSpace(geoInfo.City)
		if resolvedCity == "" {
			resolvedCity = strings.TrimSpace(geoInfo.Region)
		}
		if resolvedCity != "" {
			city = resolvedCity
			changed = true
		}
	}
	if needsCountryCode && strings.TrimSpace(geoInfo.CountryCode) != "" {
		countryCode = strings.ToUpper(strings.TrimSpace(geoInfo.CountryCode))
		changed = true
	}

	return country, city, countryCode, changed
}

func (s *Service) resolveMissingGeo(ctx context.Context, ip, country, city, countryCode string) (string, string, string, bool) {
	if s.geoService == nil || strings.TrimSpace(ip) == "" {
		return country, city, countryCode, false
	}

	needsCountry := strings.TrimSpace(country) == ""
	needsCity := strings.TrimSpace(city) == ""
	needsCountryCode := strings.TrimSpace(countryCode) == ""
	if !needsCountry && !needsCity && !needsCountryCode {
		return country, city, countryCode, false
	}

	geoInfo, err := s.geoService.Lookup(ctx, ip)
	if err != nil || !hasGeolocationDetails(geoInfo) {
		return country, city, countryCode, false
	}

	changed := false
	if needsCountry && strings.TrimSpace(geoInfo.Country) != "" {
		country = geoInfo.Country
		changed = true
	}
	if needsCity {
		resolvedCity := strings.TrimSpace(geoInfo.City)
		if resolvedCity == "" {
			resolvedCity = strings.TrimSpace(geoInfo.Region)
		}
		if resolvedCity != "" {
			city = resolvedCity
			changed = true
		}
	}
	if needsCountryCode && strings.TrimSpace(geoInfo.CountryCode) != "" {
		countryCode = strings.ToUpper(strings.TrimSpace(geoInfo.CountryCode))
		changed = true
	}

	return country, city, countryCode, changed
}

func (s *Service) warmGeolocationAsync(ip string) {
	if s.geoService == nil || !isExternalLookupCandidate(ip) {
		return
	}

	s.geoWarmupMu.Lock()
	if _, exists := s.geoWarmups[ip]; exists {
		s.geoWarmupMu.Unlock()
		return
	}
	s.geoWarmups[ip] = struct{}{}
	s.geoWarmupMu.Unlock()

	go func(targetIP string) {
		defer func() {
			s.geoWarmupMu.Lock()
			delete(s.geoWarmups, targetIP)
			s.geoWarmupMu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, _ = s.geoService.Lookup(ctx, targetIP)
	}(ip)
}

// EnrichActiveIPRecords fills missing geolocation fields for active IP records.
func (s *Service) EnrichActiveIPRecords(ctx context.Context, records []ActiveIP) {
	for i := range records {
		country, city, countryCode, changed := s.resolveMissingGeo(ctx, records[i].IP, records[i].Country, records[i].City, records[i].CountryCode)
		records[i].Country = country
		records[i].City = city
		records[i].CountryCode = countryCode
		if !changed || records[i].UserID == 0 {
			continue
		}
		if strings.TrimSpace(country) == "" && strings.TrimSpace(city) == "" {
			continue
		}
		_ = s.db.WithContext(ctx).
			Model(&ActiveIP{}).
			Where("user_id = ? AND ip = ? AND (country = '' OR country IS NULL OR city = '' OR city IS NULL)", records[i].UserID, records[i].IP).
			Updates(map[string]any{"country": country, "city": city}).Error
	}
}

// EnrichIPHistoryRecords fills missing geolocation fields for raw IP history records.
func (s *Service) EnrichIPHistoryRecords(ctx context.Context, records []IPHistory) {
	for i := range records {
		country, city, countryCode, changed := s.resolveMissingGeo(ctx, records[i].IP, records[i].Country, records[i].City, records[i].CountryCode)
		records[i].Country = country
		records[i].City = city
		records[i].CountryCode = countryCode
		if !changed {
			continue
		}
		if strings.TrimSpace(country) == "" && strings.TrimSpace(city) == "" {
			continue
		}
		_ = s.db.WithContext(ctx).
			Model(&IPHistory{}).
			Where("user_id = ? AND ip = ? AND (country = '' OR country IS NULL OR city = '' OR city IS NULL)", records[i].UserID, records[i].IP).
			Updates(map[string]any{"country": country, "city": city}).Error
	}
}

// KickIP removes an IP from active IPs and optionally adds to temporary blacklist.
func (s *Service) KickIP(ctx context.Context, userID uint, ip string, addToBlacklist bool, blockDuration time.Duration) error {
	// Get geolocation info for notification
	var country, city string
	if s.geoService != nil {
		geoInfo, _ := s.geoService.Lookup(ctx, ip)
		if geoInfo != nil {
			country = geoInfo.Country
			city = geoInfo.City
		}
	}

	// Remove from active IPs
	if err := s.tracker.RemoveActiveIP(ctx, userID, ip); err != nil {
		return err
	}

	// Send device kicked notification
	if s.notifier != nil {
		_ = s.notifier.NotifyDeviceKicked(NotificationData{
			UserID:    userID,
			IP:        ip,
			Country:   country,
			City:      city,
			Reason:    "Device kicked by user or admin",
			Timestamp: time.Now(),
		})
	}

	// Optionally add to temporary blacklist
	if addToBlacklist && blockDuration > 0 {
		expiresAt := time.Now().Add(blockDuration)
		entry := &IPBlacklist{
			IP:          ip,
			UserID:      &userID,
			Reason:      "kicked by user",
			ExpiresAt:   &expiresAt,
			IsAutomatic: false,
		}
		return s.validator.AddToBlacklist(ctx, entry)
	}

	return nil
}

// GetIPStats returns IP statistics for a user.
func (s *Service) GetIPStats(ctx context.Context, userID uint, maxConcurrentIPs int) (*IPStats, error) {
	// Clean up inactive IPs first
	timeout := time.Duration(s.settings.InactiveTimeout) * time.Minute
	_, _ = s.tracker.CleanupInactiveIPsForUser(ctx, userID, timeout)

	// Get active IP count
	activeCount, err := s.tracker.GetActiveIPCount(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get unique IP count (last 30 days)
	startTime := time.Now().AddDate(0, 0, -30)
	endTime := time.Now()
	uniqueCount, err := s.tracker.GetUniqueIPCount(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// Get IPs by country
	ipsByCountry, err := s.tracker.GetIPsByCountry(ctx, userID)
	if err != nil {
		ipsByCountry = make(map[string]int)
	}

	// Get recent IPs
	recentIPs, err := s.GetOnlineIPs(ctx, userID)
	if err != nil {
		recentIPs = []OnlineIP{}
	}

	// Calculate limit
	limit := maxConcurrentIPs
	if limit < 0 {
		limit = s.settings.DefaultMaxConcurrentIPs
	}

	remaining := limit - activeCount
	if remaining < 0 || limit == 0 {
		remaining = 0
	}

	// Check for suspicious activity
	countries, _ := s.tracker.GetRecentCountries(ctx, userID, 30)
	suspicious := len(countries) > 3

	return &IPStats{
		TotalUniqueIPs:     uniqueCount,
		CurrentActiveIPs:   activeCount,
		MaxConcurrentIPs:   limit,
		RemainingSlots:     remaining,
		IPsByCountry:       ipsByCountry,
		RecentIPs:          recentIPs,
		SuspiciousActivity: suspicious,
	}, nil
}

// RecordFailedAttempt records a failed access attempt.
func (s *Service) RecordFailedAttempt(ctx context.Context, ip, reason string) error {
	attempt := &FailedAttempt{
		IP:        ip,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	return s.db.WithContext(ctx).Create(attempt).Error
}

// CheckAutoBlacklist checks if an IP should be auto-blacklisted.
func (s *Service) CheckAutoBlacklist(ctx context.Context, ip string) (bool, error) {
	if !s.settings.AutoBlacklistEnabled {
		return false, nil
	}

	// Count failed attempts in the window
	windowStart := time.Now().Add(-time.Duration(s.settings.FailedAttemptWindow) * time.Minute)
	var count int64
	err := s.db.WithContext(ctx).
		Model(&FailedAttempt{}).
		Where("ip = ? AND created_at >= ?", ip, windowStart).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	if int(count) >= s.settings.MaxFailedAttempts {
		// Add to blacklist
		expiresAt := time.Now().Add(time.Duration(s.settings.AutoBlacklistDuration) * time.Minute)
		entry := &IPBlacklist{
			IP:          ip,
			Reason:      fmt.Sprintf("auto-blacklisted: %d failed attempts", count),
			ExpiresAt:   &expiresAt,
			IsAutomatic: true,
		}
		if err := s.validator.AddToBlacklist(ctx, entry); err != nil {
			return false, err
		}

		// Send auto-blacklist notification
		if s.notifier != nil {
			var country, city string
			if s.geoService != nil {
				geoInfo, _ := s.geoService.Lookup(ctx, ip)
				if geoInfo != nil {
					country = geoInfo.Country
					city = geoInfo.City
				}
			}
			_ = s.notifier.NotifyAutoBlacklisted(NotificationData{
				IP:        ip,
				Country:   country,
				City:      city,
				Reason:    fmt.Sprintf("Auto-blacklisted after %d failed attempts", count),
				Timestamp: time.Now(),
			})
		}

		return true, nil
	}

	return false, nil
}

// CleanupFailedAttempts removes old failed attempt records.
func (s *Service) CleanupFailedAttempts(ctx context.Context) (int64, error) {
	// Keep only records from the last window period
	cutoff := time.Now().Add(-time.Duration(s.settings.FailedAttemptWindow*2) * time.Minute)
	result := s.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&FailedAttempt{})
	return result.RowsAffected, result.Error
}

// Validator returns the IP validator.
func (s *Service) Validator() *Validator {
	return s.validator
}

// Tracker returns the IP tracker.
func (s *Service) Tracker() *Tracker {
	return s.tracker
}

// GeoService returns the geolocation service.
func (s *Service) GeoService() *GeolocationService {
	return s.geoService
}
