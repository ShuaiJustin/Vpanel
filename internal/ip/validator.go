package ip

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Validator provides IP validation functionality including whitelist and blacklist checks.
type Validator struct {
	db          *gorm.DB
	cidrMatcher *CIDRMatcher
}

// NewValidator creates a new Validator instance.
func NewValidator(db *gorm.DB) *Validator {
	return &Validator{
		db:          db,
		cidrMatcher: NewCIDRMatcher(),
	}
}

// IsWhitelisted checks if an IP is in the whitelist.
// It checks both global whitelist (userID = nil) and user-specific whitelist.
func (v *Validator) IsWhitelisted(ctx context.Context, ip string, userID *uint) bool {
	var entries []IPWhitelist

	query := v.db.WithContext(ctx).Where("user_id IS NULL")
	if userID != nil {
		query = v.db.WithContext(ctx).Where("user_id IS NULL OR user_id = ?", *userID)
	}

	if err := query.Find(&entries).Error; err != nil {
		return false
	}

	for _, entry := range entries {
		// Check exact IP match
		if v.cidrMatcher.MatchesIP(ip, entry.IP) {
			return true
		}
		// Check CIDR match
		if entry.CIDR != "" && v.cidrMatcher.MatchesCIDR(ip, entry.CIDR) {
			return true
		}
	}

	return false
}

// IsBlacklisted checks if an IP is in the blacklist.
// It checks both global blacklist (userID = nil) and user-specific blacklist.
// Returns the blacklist entry if found and not expired.
func (v *Validator) IsBlacklisted(ctx context.Context, ip string, userID *uint) (*BlacklistEntry, bool) {
	var entries []IPBlacklist

	query := v.db.WithContext(ctx).Where("user_id IS NULL")
	if userID != nil {
		query = v.db.WithContext(ctx).Where("user_id IS NULL OR user_id = ?", *userID)
	}

	if err := query.Find(&entries).Error; err != nil {
		return nil, false
	}

	now := time.Now()
	for _, entry := range entries {
		// Skip expired entries
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			continue
		}

		// Check exact IP match
		if v.cidrMatcher.MatchesIP(ip, entry.IP) {
			return &BlacklistEntry{
				ID:        entry.ID,
				IP:        entry.IP,
				CIDR:      entry.CIDR,
				Reason:    entry.Reason,
				ExpiresAt: entry.ExpiresAt,
			}, true
		}

		// Check CIDR match
		if entry.CIDR != "" && v.cidrMatcher.MatchesCIDR(ip, entry.CIDR) {
			return &BlacklistEntry{
				ID:        entry.ID,
				IP:        entry.IP,
				CIDR:      entry.CIDR,
				Reason:    entry.Reason,
				ExpiresAt: entry.ExpiresAt,
			}, true
		}
	}

	return nil, false
}


// AddToWhitelist adds an IP or CIDR to the whitelist.
func (v *Validator) AddToWhitelist(ctx context.Context, entry *IPWhitelist) error {
	return v.db.WithContext(ctx).Create(entry).Error
}

// RemoveFromWhitelist removes an entry from the whitelist.
func (v *Validator) RemoveFromWhitelist(ctx context.Context, id uint) error {
	return v.db.WithContext(ctx).Delete(&IPWhitelist{}, id).Error
}

// GetWhitelist returns all whitelist entries, optionally filtered by user.
func (v *Validator) GetWhitelist(ctx context.Context, userID *uint) ([]IPWhitelist, error) {
	var entries []IPWhitelist
	query := v.db.WithContext(ctx)
	if userID != nil {
		query = query.Where("user_id IS NULL OR user_id = ?", *userID)
	}
	err := query.Order("created_at DESC").Find(&entries).Error
	return entries, err
}

// AddToBlacklist adds an IP or CIDR to the blacklist.
func (v *Validator) AddToBlacklist(ctx context.Context, entry *IPBlacklist) error {
	return v.db.WithContext(ctx).Create(entry).Error
}

// RemoveFromBlacklist removes an entry from the blacklist.
func (v *Validator) RemoveFromBlacklist(ctx context.Context, id uint) error {
	return v.db.WithContext(ctx).Delete(&IPBlacklist{}, id).Error
}

// GetBlacklist returns all blacklist entries, optionally filtered by user.
func (v *Validator) GetBlacklist(ctx context.Context, userID *uint) ([]IPBlacklist, error) {
	var entries []IPBlacklist
	query := v.db.WithContext(ctx)
	if userID != nil {
		query = query.Where("user_id IS NULL OR user_id = ?", *userID)
	}
	err := query.Order("created_at DESC").Find(&entries).Error
	return entries, err
}

// CleanupExpiredBlacklist removes expired blacklist entries.
func (v *Validator) CleanupExpiredBlacklist(ctx context.Context) (int64, error) {
	result := v.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).
		Delete(&IPBlacklist{})
	return result.RowsAffected, result.Error
}

// ImportWhitelist imports multiple IPs to the whitelist.
func (v *Validator) ImportWhitelist(ctx context.Context, ips []string, userID *uint, description string, createdBy uint) error {
	entries := make([]IPWhitelist, 0, len(ips))
	for _, ip := range ips {
		entry := IPWhitelist{
			IP:          ip,
			UserID:      userID,
			Description: description,
			CreatedBy:   createdBy,
		}
		// Check if it's a CIDR notation
		if v.cidrMatcher.IsValidCIDR(ip) {
			entry.CIDR = ip
			entry.IP = ""
		}
		entries = append(entries, entry)
	}
	return v.db.WithContext(ctx).Create(&entries).Error
}
