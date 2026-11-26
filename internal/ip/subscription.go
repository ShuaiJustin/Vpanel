package ip

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubscriptionIPService handles IP restrictions for subscription links.
type SubscriptionIPService struct {
	db         *gorm.DB
	geoService *GeolocationService
}

// NewSubscriptionIPService creates a new SubscriptionIPService.
func NewSubscriptionIPService(db *gorm.DB, geoService *GeolocationService) *SubscriptionIPService {
	return &SubscriptionIPService{
		db:         db,
		geoService: geoService,
	}
}

// RecordAccess records an IP access to a subscription link.
func (s *SubscriptionIPService) RecordAccess(ctx context.Context, subscriptionID uint, ip, userAgent string) error {
	// Get geolocation info
	var country string
	if s.geoService != nil {
		geoInfo, err := s.geoService.Lookup(ctx, ip)
		if err == nil && geoInfo != nil {
			country = geoInfo.Country
		}
	}

	now := time.Now()
	access := SubscriptionIPAccess{
		SubscriptionID: subscriptionID,
		IP:             ip,
		UserAgent:      userAgent,
		Country:        country,
		AccessCount:    1,
		FirstAccess:    now,
		LastAccess:     now,
	}

	// Upsert: update access count and last access if exists
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "subscription_id"}, {Name: "ip"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"access_count": gorm.Expr("access_count + 1"),
			"last_access":  now,
			"user_agent":   userAgent,
		}),
	}).Create(&access).Error
}

// GetUniqueIPCount returns the count of unique IPs that have accessed a subscription.
func (s *SubscriptionIPService) GetUniqueIPCount(ctx context.Context, subscriptionID uint) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&SubscriptionIPAccess{}).
		Where("subscription_id = ?", subscriptionID).
		Count(&count).Error
	return int(count), err
}

// CheckIPLimit checks if a new IP can access the subscription.
func (s *SubscriptionIPService) CheckIPLimit(ctx context.Context, subscriptionID uint, ip string, limit int) (*AccessResult, error) {
	// If limit is 0 or negative, no limit
	if limit <= 0 {
		return &AccessResult{Allowed: true}, nil
	}

	// Check if IP already has access
	var existing SubscriptionIPAccess
	err := s.db.WithContext(ctx).
		Where("subscription_id = ? AND ip = ?", subscriptionID, ip).
		First(&existing).Error

	if err == nil {
		// IP already has access
		return &AccessResult{Allowed: true, Reason: "existing access"}, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Check current unique IP count
	count, err := s.GetUniqueIPCount(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	if count >= limit {
		return &AccessResult{
			Allowed: false,
			Code:    ErrCodeSubscriptionIPLimit,
			Reason:  "subscription IP limit reached",
		}, nil
	}

	return &AccessResult{
		Allowed:        true,
		RemainingSlots: limit - count - 1,
	}, nil
}

// GetAccessList returns all IPs that have accessed a subscription.
func (s *SubscriptionIPService) GetAccessList(ctx context.Context, subscriptionID uint) ([]SubscriptionIPAccess, error) {
	var accesses []SubscriptionIPAccess
	err := s.db.WithContext(ctx).
		Where("subscription_id = ?", subscriptionID).
		Order("last_access DESC").
		Find(&accesses).Error
	return accesses, err
}

// ClearAccessList clears all IP access records for a subscription.
func (s *SubscriptionIPService) ClearAccessList(ctx context.Context, subscriptionID uint) error {
	return s.db.WithContext(ctx).
		Where("subscription_id = ?", subscriptionID).
		Delete(&SubscriptionIPAccess{}).Error
}

// RemoveIP removes a specific IP from the access list.
func (s *SubscriptionIPService) RemoveIP(ctx context.Context, subscriptionID uint, ip string) error {
	return s.db.WithContext(ctx).
		Where("subscription_id = ? AND ip = ?", subscriptionID, ip).
		Delete(&SubscriptionIPAccess{}).Error
}

// GetAccessStats returns access statistics for a subscription.
func (s *SubscriptionIPService) GetAccessStats(ctx context.Context, subscriptionID uint) (*SubscriptionIPStats, error) {
	accesses, err := s.GetAccessList(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	stats := &SubscriptionIPStats{
		UniqueIPs:    len(accesses),
		TotalAccess:  0,
		IPsByCountry: make(map[string]int),
		RecentIPs:    make([]SubscriptionIPInfo, 0),
	}

	for _, access := range accesses {
		stats.TotalAccess += access.AccessCount
		if access.Country != "" {
			stats.IPsByCountry[access.Country]++
		}
		stats.RecentIPs = append(stats.RecentIPs, SubscriptionIPInfo{
			IP:          access.IP,
			Country:     access.Country,
			AccessCount: access.AccessCount,
			FirstAccess: access.FirstAccess,
			LastAccess:  access.LastAccess,
		})
	}

	return stats, nil
}

// SubscriptionIPStats holds statistics for subscription IP access.
type SubscriptionIPStats struct {
	UniqueIPs    int                  `json:"unique_ips"`
	TotalAccess  int                  `json:"total_access"`
	IPsByCountry map[string]int       `json:"ips_by_country"`
	RecentIPs    []SubscriptionIPInfo `json:"recent_ips"`
}

// SubscriptionIPInfo holds information about a subscription IP access.
type SubscriptionIPInfo struct {
	IP          string    `json:"ip"`
	Country     string    `json:"country"`
	AccessCount int       `json:"access_count"`
	FirstAccess time.Time `json:"first_access"`
	LastAccess  time.Time `json:"last_access"`
}
