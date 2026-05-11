package ip

import (
	"context"
	"time"

	"v/internal/database/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Tracker provides IP tracking functionality.
type Tracker struct {
	db *gorm.DB
}

// NewTracker creates a new Tracker instance.
func NewTracker(db *gorm.DB) *Tracker {
	return &Tracker{db: db}
}

// AddActiveIP adds or updates an active IP for a user.
func (t *Tracker) AddActiveIP(ctx context.Context, userID uint, ip, userAgent, deviceType, country, city string) error {
	activeIP := ActiveIP{
		UserID:     userID,
		IP:         ip,
		UserAgent:  userAgent,
		DeviceType: deviceType,
		Country:    country,
		City:       city,
		LastActive: time.Now(),
	}

	// Upsert: update if exists, insert if not
	return t.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "ip"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_agent", "device_type", "country", "city", "last_active"}),
	}).Create(&activeIP).Error
}

// RemoveActiveIP removes an active IP for a user.
func (t *Tracker) RemoveActiveIP(ctx context.Context, userID uint, ip string) error {
	return t.db.WithContext(ctx).
		Where("user_id = ? AND ip = ?", userID, ip).
		Delete(&ActiveIP{}).Error
}

// RemoveActiveIPUnlessDeviceType removes a stale active IP unless it represents
// a preserved device type such as a proxy session.
func (t *Tracker) RemoveActiveIPUnlessDeviceType(ctx context.Context, userID uint, ip, preservedDeviceType string) error {
	return t.db.WithContext(ctx).
		Where("user_id = ? AND ip = ? AND (device_type IS NULL OR device_type <> ?)", userID, ip, preservedDeviceType).
		Delete(&ActiveIP{}).Error
}

// GetActiveIPCount returns the count of active IPs for a user.
func (t *Tracker) GetActiveIPCount(ctx context.Context, userID uint) (int, error) {
	var count int64
	err := t.db.WithContext(ctx).
		Model(&ActiveIP{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return int(count), err
}

// GetActiveIPs returns all active IPs for a user.
func (t *Tracker) GetActiveIPs(ctx context.Context, userID uint) ([]ActiveIP, error) {
	var ips []ActiveIP
	err := t.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_active DESC").
		Find(&ips).Error
	return ips, err
}

// GetOnlineIPs returns online IP information for a user.
func (t *Tracker) GetOnlineIPs(ctx context.Context, userID uint) ([]OnlineIP, error) {
	activeIPs, err := t.GetActiveIPs(ctx, userID)
	if err != nil {
		return nil, err
	}

	onlineIPs := make([]OnlineIP, len(activeIPs))
	for i, ip := range activeIPs {
		onlineIPs[i] = OnlineIP{
			IP:         ip.IP,
			UserAgent:  ip.UserAgent,
			DeviceType: ip.DeviceType,
			Country:    ip.Country,
			City:       ip.City,
			LastActive: ip.LastActive,
			CreatedAt:  ip.CreatedAt,
		}
	}
	return onlineIPs, nil
}

// UpdateLastActive updates the last active timestamp for an IP.
func (t *Tracker) UpdateLastActive(ctx context.Context, userID uint, ip string) error {
	return t.db.WithContext(ctx).
		Model(&ActiveIP{}).
		Where("user_id = ? AND ip = ?", userID, ip).
		Update("last_active", time.Now()).Error
}

// IsIPActive checks if an IP is currently active for a user.
func (t *Tracker) IsIPActive(ctx context.Context, userID uint, ip string) (bool, error) {
	var count int64
	err := t.db.WithContext(ctx).
		Model(&ActiveIP{}).
		Where("user_id = ? AND ip = ?", userID, ip).
		Count(&count).Error
	return count > 0, err
}

// CleanupInactiveIPs removes IPs that have been inactive for longer than the timeout.
func (t *Tracker) CleanupInactiveIPs(ctx context.Context, timeout time.Duration) (int, error) {
	cutoff := time.Now().Add(-timeout)
	result := t.db.WithContext(ctx).
		Where("last_active < ?", cutoff).
		Delete(&ActiveIP{})
	return int(result.RowsAffected), result.Error
}

// CleanupInactiveIPsForUser removes inactive IPs for a specific user.
func (t *Tracker) CleanupInactiveIPsForUser(ctx context.Context, userID uint, timeout time.Duration) (int, error) {
	cutoff := time.Now().Add(-timeout)
	result := t.db.WithContext(ctx).
		Where("user_id = ? AND last_active < ?", userID, cutoff).
		Delete(&ActiveIP{})
	return int(result.RowsAffected), result.Error
}

// RemoveAllActiveIPs removes all active IPs for a user.
func (t *Tracker) RemoveAllActiveIPs(ctx context.Context, userID uint) error {
	return t.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&ActiveIP{}).Error
}

// RecordIPHistory records an IP access in the history.
func (t *Tracker) RecordIPHistory(ctx context.Context, record *IPHistory) error {
	return t.db.WithContext(ctx).Create(record).Error
}

// GetIPHistory returns IP history for a user with optional filters.
func (t *Tracker) GetIPHistory(ctx context.Context, userID uint, filter *IPHistoryFilter) ([]IPHistory, error) {
	var records []IPHistory
	query := t.db.WithContext(ctx).Where("user_id = ?", userID)

	if filter != nil {
		if filter.StartTime != nil {
			query = query.Where("created_at >= ?", *filter.StartTime)
		}
		if filter.EndTime != nil {
			query = query.Where("created_at <= ?", *filter.EndTime)
		}
		if filter.AccessType != nil {
			query = query.Where("access_type = ?", *filter.AccessType)
		}
		if filter.IP != "" {
			query = query.Where("ip = ?", filter.IP)
		}
		if filter.Country != "" {
			query = query.Where("country = ?", filter.Country)
		}
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	err := query.Order("created_at DESC").Find(&records).Error
	return records, err
}

// GetAggregatedIPHistory returns grouped IP history suitable for the user portal device view.
func (t *Tracker) GetAggregatedIPHistory(ctx context.Context, userID uint, limit, offset int) ([]IPHistorySummary, int64, error) {
	dialect := t.db.Dialector.Name()

	var total int64
	if err := t.db.WithContext(ctx).
		Model(&IPHistory{}).
		Where("user_id = ?", userID).
		Distinct("ip").
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var summaries []IPHistorySummary
	if dialect == "sqlite" {
		type sqliteIPHistorySummary struct {
			IP          string `gorm:"column:ip"`
			FirstSeen   string `gorm:"column:first_seen"`
			LastSeen    string `gorm:"column:last_seen"`
			AccessCount int64  `gorm:"column:access_count"`
		}

		var rows []sqliteIPHistorySummary
		query := t.db.WithContext(ctx).
			Model(&IPHistory{}).
			Select("ip, MIN(created_at) AS first_seen, MAX(created_at) AS last_seen, COUNT(*) AS access_count").
			Where("user_id = ?", userID).
			Group("ip").
			Order("MAX(created_at) DESC")

		if limit > 0 {
			query = query.Limit(limit)
		}
		if offset > 0 {
			query = query.Offset(offset)
		}

		if err := query.Scan(&rows).Error; err != nil {
			return nil, 0, err
		}

		summaries = make([]IPHistorySummary, 0, len(rows))
		for _, row := range rows {
			firstSeen, err := repository.ParseAggregatedTime(dialect, row.FirstSeen, time.Local)
			if err != nil {
				return nil, 0, err
			}
			lastSeen, err := repository.ParseAggregatedTime(dialect, row.LastSeen, time.Local)
			if err != nil {
				return nil, 0, err
			}

			summary := IPHistorySummary{
				IP:          row.IP,
				AccessCount: row.AccessCount,
			}
			if firstSeen != nil {
				summary.FirstSeen = *firstSeen
			}
			if lastSeen != nil {
				summary.LastSeen = *lastSeen
			}

			summaries = append(summaries, summary)
		}
	} else {
		query := t.db.WithContext(ctx).
			Model(&IPHistory{}).
			Select("ip, MIN(created_at) AS first_seen, MAX(created_at) AS last_seen, COUNT(*) AS access_count").
			Where("user_id = ?", userID).
			Group("ip").
			Order("MAX(created_at) DESC")

		if limit > 0 {
			query = query.Limit(limit)
		}
		if offset > 0 {
			query = query.Offset(offset)
		}

		if err := query.Scan(&summaries).Error; err != nil {
			return nil, 0, err
		}
	}

	if len(summaries) == 0 {
		return summaries, total, nil
	}

	ips := make([]string, 0, len(summaries))
	summaryByIP := make(map[string]*IPHistorySummary, len(summaries))
	for i := range summaries {
		ips = append(ips, summaries[i].IP)
		summaryByIP[summaries[i].IP] = &summaries[i]
	}

	type latestLocationRow struct {
		ID        uint      `gorm:"column:id"`
		IP        string    `gorm:"column:ip"`
		Country   string    `gorm:"column:country"`
		City      string    `gorm:"column:city"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}

	latestByIP := t.db.WithContext(ctx).
		Model(&IPHistory{}).
		Select("ip, MAX(created_at) AS last_seen").
		Where("user_id = ? AND ip IN ?", userID, ips).
		Group("ip")

	var latestRows []latestLocationRow
	if err := t.db.WithContext(ctx).
		Table("ip_history AS history").
		Select("history.id, history.ip, history.country, history.city, history.created_at").
		Joins("JOIN (?) AS latest ON latest.ip = history.ip AND latest.last_seen = history.created_at", latestByIP).
		Where("history.user_id = ? AND history.ip IN ?", userID, ips).
		Order("history.ip ASC, history.id DESC").
		Scan(&latestRows).Error; err != nil {
		return nil, 0, err
	}

	appliedIPs := make(map[string]struct{}, len(latestRows))
	for _, row := range latestRows {
		if _, exists := appliedIPs[row.IP]; exists {
			continue
		}
		appliedIPs[row.IP] = struct{}{}
		if summary, exists := summaryByIP[row.IP]; exists {
			summary.Country = row.Country
			summary.City = row.City
		}
	}

	return summaries, total, nil
}

// GetUniqueIPCount returns the count of unique IPs for a user within a time range.
func (t *Tracker) GetUniqueIPCount(ctx context.Context, userID uint, startTime, endTime time.Time) (int, error) {
	var count int64
	err := t.db.WithContext(ctx).
		Model(&IPHistory{}).
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, startTime, endTime).
		Distinct("ip").
		Count(&count).Error
	return int(count), err
}

// GetIPsByCountry returns IP counts grouped by country for a user.
func (t *Tracker) GetIPsByCountry(ctx context.Context, userID uint) (map[string]int, error) {
	type result struct {
		Country string
		Count   int
	}
	var results []result

	err := t.db.WithContext(ctx).
		Model(&IPHistory{}).
		Select("country, COUNT(DISTINCT ip) as count").
		Where("user_id = ?", userID).
		Group("country").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	countryMap := make(map[string]int)
	for _, r := range results {
		countryMap[r.Country] = r.Count
	}
	return countryMap, nil
}

// CleanupOldHistory removes IP history records older than the retention period.
func (t *Tracker) CleanupOldHistory(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := t.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&IPHistory{})
	return result.RowsAffected, result.Error
}

// MarkSuspicious marks an IP history record as suspicious.
func (t *Tracker) MarkSuspicious(ctx context.Context, id uint) error {
	return t.db.WithContext(ctx).
		Model(&IPHistory{}).
		Where("id = ?", id).
		Update("is_suspicious", true).Error
}

// GetRecentCountries returns the countries accessed by a user in the last N minutes.
func (t *Tracker) GetRecentCountries(ctx context.Context, userID uint, minutes int) ([]string, error) {
	var countries []string
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)

	err := t.db.WithContext(ctx).
		Model(&IPHistory{}).
		Where("user_id = ? AND created_at >= ?", userID, cutoff).
		Distinct("country").
		Pluck("country", &countries).Error

	return countries, err
}

// GetDB returns the database connection.
func (t *Tracker) GetDB() *gorm.DB {
	return t.db
}
