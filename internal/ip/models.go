// Package ip provides IP restriction functionality for the V Panel application.
package ip

import (
	"time"
)

// AccessType represents the type of access being made.
type AccessType string

const (
	AccessTypeSubscription AccessType = "subscription"
	AccessTypeProxy        AccessType = "proxy"
	AccessTypeAPI          AccessType = "api"
)

// IPWhitelist represents an IP address or CIDR range that is allowed to bypass restrictions.
type IPWhitelist struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	IP          string    `json:"ip" gorm:"column:ip;size:45;not null"`
	CIDR        string    `json:"cidr" gorm:"column:cidr;size:50"`
	UserID      *uint     `json:"user_id" gorm:"column:user_id;index"`
	Description string    `json:"description" gorm:"column:description;size:255"`
	CreatedBy   uint      `json:"created_by" gorm:"column:created_by"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName returns the table name for IPWhitelist.
func (IPWhitelist) TableName() string {
	return "ip_whitelist"
}

// IPBlacklist represents an IP address or CIDR range that is blocked from access.
type IPBlacklist struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	IP          string     `json:"ip" gorm:"column:ip;size:45;not null"`
	CIDR        string     `json:"cidr" gorm:"column:cidr;size:50"`
	UserID      *uint      `json:"user_id" gorm:"column:user_id;index"`
	Reason      string     `json:"reason" gorm:"column:reason;size:255"`
	ExpiresAt   *time.Time `json:"expires_at" gorm:"column:expires_at"`
	IsAutomatic bool       `json:"is_automatic" gorm:"column:is_automatic"`
	CreatedBy   *uint      `json:"created_by" gorm:"column:created_by"`
	CreatedAt   time.Time  `json:"created_at" gorm:"column:created_at"`
}

// TableName returns the table name for IPBlacklist.
func (IPBlacklist) TableName() string {
	return "ip_blacklist"
}

// IsExpired checks if the blacklist entry has expired.
func (b *IPBlacklist) IsExpired() bool {
	if b.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*b.ExpiresAt)
}

// ActiveIP represents a currently active IP address for a user.
type ActiveIP struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	UserID     uint      `json:"user_id" gorm:"index:idx_active_ip_user_ip,unique"`
	IP         string    `json:"ip" gorm:"size:45;index:idx_active_ip_user_ip,unique"`
	UserAgent  string    `json:"user_agent" gorm:"size:500"`
	DeviceType string    `json:"device_type" gorm:"size:50"` // desktop, mobile, tablet
	Country    string    `json:"country" gorm:"size:100"`
	City       string    `json:"city" gorm:"size:100"`
	LastActive time.Time `json:"last_active" gorm:"index"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName returns the table name for ActiveIP.
func (ActiveIP) TableName() string {
	return "active_ips"
}

// IsInactive checks if the IP has been inactive for the given duration.
func (a *ActiveIP) IsInactive(timeout time.Duration) bool {
	return time.Since(a.LastActive) > timeout
}

// IPHistory represents a historical record of IP access.
type IPHistory struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	UserID       uint       `json:"user_id" gorm:"index:idx_ip_history_user_time"`
	IP           string     `json:"ip" gorm:"size:45;index"`
	UserAgent    string     `json:"user_agent" gorm:"size:500"`
	AccessType   AccessType `json:"access_type" gorm:"size:20"`
	Country      string     `json:"country" gorm:"size:100"`
	City         string     `json:"city" gorm:"size:100"`
	IsSuspicious bool       `json:"is_suspicious"`
	CreatedAt    time.Time  `json:"created_at" gorm:"index:idx_ip_history_user_time"`
}

// TableName returns the table name for IPHistory.
func (IPHistory) TableName() string {
	return "ip_history"
}

// IPHistorySummary represents aggregated portal-facing IP history grouped by IP.
type IPHistorySummary struct {
	IP          string    `json:"ip" gorm:"column:ip"`
	Country     string    `json:"country" gorm:"column:country"`
	City        string    `json:"city" gorm:"column:city"`
	FirstSeen   time.Time `json:"first_seen" gorm:"column:first_seen"`
	LastSeen    time.Time `json:"last_seen" gorm:"column:last_seen"`
	AccessCount int64     `json:"access_count" gorm:"column:access_count"`
}

// SubscriptionIPAccess tracks unique IPs that have accessed a subscription link.
type SubscriptionIPAccess struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	SubscriptionID uint      `json:"subscription_id" gorm:"index:idx_sub_ip_access,unique"`
	IP             string    `json:"ip" gorm:"size:45;index:idx_sub_ip_access,unique"`
	UserAgent      string    `json:"user_agent" gorm:"size:500"`
	Country        string    `json:"country" gorm:"size:100"`
	AccessCount    int       `json:"access_count"`
	FirstAccess    time.Time `json:"first_access"`
	LastAccess     time.Time `json:"last_access"`
}

// TableName returns the table name for SubscriptionIPAccess.
func (SubscriptionIPAccess) TableName() string {
	return "subscription_ip_access"
}

// GeoCache caches geolocation lookup results.
type GeoCache struct {
	IP          string    `json:"ip" gorm:"primaryKey;size:45"`
	Country     string    `json:"country" gorm:"size:100"`
	CountryCode string    `json:"country_code" gorm:"size:2"`
	Region      string    `json:"region" gorm:"size:100"`
	City        string    `json:"city" gorm:"size:100"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	ISP         string    `json:"isp" gorm:"size:200"`
	CachedAt    time.Time `json:"cached_at" gorm:"index"`
}

// TableName returns the table name for GeoCache.
func (GeoCache) TableName() string {
	return "geo_cache"
}

// IsCacheValid checks if the cache entry is still valid.
func (g *GeoCache) IsCacheValid(ttl time.Duration) bool {
	return time.Since(g.CachedAt) < ttl
}

// FailedAttempt tracks failed access attempts for auto-blacklisting.
type FailedAttempt struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	IP        string    `json:"ip" gorm:"size:45;index"`
	Reason    string    `json:"reason" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName returns the table name for FailedAttempt.
func (FailedAttempt) TableName() string {
	return "failed_attempts"
}

// IPRestrictionSettings holds the configuration for IP restriction features.
type IPRestrictionSettings struct {
	// Global settings
	Enabled                 bool `json:"enabled"`
	DefaultMaxConcurrentIPs int  `json:"default_max_concurrent_ips"`
	InactiveTimeout         int  `json:"inactive_timeout"` // minutes

	// Subscription link IP limit
	SubscriptionIPLimitEnabled bool `json:"subscription_ip_limit_enabled"`
	DefaultSubscriptionIPLimit int  `json:"default_subscription_ip_limit"`

	// Geo restriction
	GeoRestrictionEnabled bool     `json:"geo_restriction_enabled"`
	AllowedCountries      []string `json:"allowed_countries"`
	BlockedCountries      []string `json:"blocked_countries"`

	// Auto blacklist rules
	AutoBlacklistEnabled  bool `json:"auto_blacklist_enabled"`
	MaxFailedAttempts     int  `json:"max_failed_attempts"`
	FailedAttemptWindow   int  `json:"failed_attempt_window"`   // minutes
	AutoBlacklistDuration int  `json:"auto_blacklist_duration"` // minutes
}

// DefaultIPRestrictionSettings returns the default settings.
func DefaultIPRestrictionSettings() *IPRestrictionSettings {
	return &IPRestrictionSettings{
		Enabled:                    true,
		DefaultMaxConcurrentIPs:    3,
		InactiveTimeout:            10,
		SubscriptionIPLimitEnabled: false,
		DefaultSubscriptionIPLimit: 5,
		GeoRestrictionEnabled:      false,
		AllowedCountries:           []string{},
		BlockedCountries:           []string{},
		AutoBlacklistEnabled:       true,
		MaxFailedAttempts:          10,
		FailedAttemptWindow:        15,
		AutoBlacklistDuration:      60,
	}
}

// AccessResult represents the result of an IP access check.
type AccessResult struct {
	Allowed        bool     `json:"allowed"`
	Reason         string   `json:"reason,omitempty"`
	Code           string   `json:"code,omitempty"`
	RemainingSlots int      `json:"remaining_slots,omitempty"`
	OnlineIPs      []string `json:"online_ips,omitempty"`
}

// OnlineIP represents an online IP with its details.
type OnlineIP struct {
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	DeviceType string    `json:"device_type"`
	Country    string    `json:"country"`
	City       string    `json:"city"`
	LastActive time.Time `json:"last_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// IPStats represents IP statistics for a user.
type IPStats struct {
	TotalUniqueIPs     int            `json:"total_unique_ips"`
	CurrentActiveIPs   int            `json:"current_active_ips"`
	MaxConcurrentIPs   int            `json:"max_concurrent_ips"`
	RemainingSlots     int            `json:"remaining_slots"`
	IPsByCountry       map[string]int `json:"ips_by_country"`
	RecentIPs          []OnlineIP     `json:"recent_ips"`
	SuspiciousActivity bool           `json:"suspicious_activity"`
}

// GeoInfo represents geolocation information for an IP.
type GeoInfo struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ISP         string  `json:"isp"`
}

// GeoCheckResult represents the result of a geo restriction check.
type GeoCheckResult struct {
	Allowed     bool   `json:"allowed"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
	Reason      string `json:"reason,omitempty"`
}

// BlacklistEntry represents a blacklist match result.
type BlacklistEntry struct {
	ID        uint       `json:"id"`
	IP        string     `json:"ip"`
	CIDR      string     `json:"cidr"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// IPHistoryFilter represents filter options for IP history queries.
type IPHistoryFilter struct {
	StartTime  *time.Time  `json:"start_time"`
	EndTime    *time.Time  `json:"end_time"`
	AccessType *AccessType `json:"access_type"`
	IP         string      `json:"ip"`
	Country    string      `json:"country"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
}
