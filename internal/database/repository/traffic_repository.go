// Package repository provides data access implementations.
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

const unknownTrafficProtocol = "unknown"

// trafficRepository implements TrafficRepository.
type trafficRepository struct {
	db *gorm.DB
}

// NewTrafficRepository creates a new traffic repository.
func NewTrafficRepository(db *gorm.DB) TrafficRepository {
	return &trafficRepository{db: db}
}

func (r *trafficRepository) timelineGroupingClause(interval string) string {
	return BuildTimeGroupingClause(r.db.Dialector.Name(), "recorded_at", interval)
}

// Create creates a new traffic record.
func (r *trafficRepository) Create(ctx context.Context, traffic *Traffic) error {
	result := r.db.WithContext(ctx).Create(traffic)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create traffic record", result.Error)
	}
	return nil
}

// GetByUserID retrieves traffic records by user ID.
func (r *trafficRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*Traffic, error) {
	var records []*Traffic
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("recorded_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	result := query.Find(&records)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by user", result.Error)
	}
	return records, nil
}

// GetByProxyID retrieves traffic records by proxy ID.
func (r *trafficRepository) GetByProxyID(ctx context.Context, proxyID int64, limit, offset int) ([]*Traffic, error) {
	var records []*Traffic
	query := r.db.WithContext(ctx).
		Where("proxy_id = ?", proxyID).
		Order("recorded_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	result := query.Find(&records)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by proxy", result.Error)
	}
	return records, nil
}

// GetByDateRange retrieves traffic records within a date range.
func (r *trafficRepository) GetByDateRange(ctx context.Context, start, end time.Time) ([]*Traffic, error) {
	var records []*Traffic
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	result := r.db.WithContext(ctx).
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), rangeArgs...).
		Order("recorded_at DESC").
		Find(&records)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by date range", result.Error)
	}
	return records, nil
}

// DeleteOlderThan deletes traffic records older than the specified time.
func (r *trafficRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("recorded_at < ?", before).
		Delete(&Traffic{})
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to delete old traffic records", result.Error)
	}
	return result.RowsAffected, nil
}

// GetTotalByUser retrieves total upload and download for a user.
func (r *trafficRepository) GetTotalByUser(ctx context.Context, userID int64) (upload, download int64, err error) {
	var result struct {
		Upload   int64
		Download int64
	}
	err = r.db.WithContext(ctx).
		Model(&Traffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("user_id = ?", userID).
		Scan(&result).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic", err)
	}
	return result.Upload, result.Download, nil
}

// GetTotalByProxy retrieves total upload and download for a proxy.
func (r *trafficRepository) GetTotalByProxy(ctx context.Context, proxyID int64) (upload, download int64, err error) {
	var result struct {
		Upload   int64
		Download int64
	}
	err = r.db.WithContext(ctx).
		Model(&Traffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("proxy_id = ?", proxyID).
		Scan(&result).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic by proxy", err)
	}
	return result.Upload, result.Download, nil
}

// GetTotalTraffic retrieves total upload and download for all traffic.
func (r *trafficRepository) GetTotalTraffic(ctx context.Context) (upload, download int64, err error) {
	var result struct {
		Upload   int64
		Download int64
	}
	err = r.db.WithContext(ctx).
		Model(&Traffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Scan(&result).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic", err)
	}
	return result.Upload, result.Download, nil
}

// GetTotalTrafficByPeriod retrieves total upload and download within a time period.
func (r *trafficRepository) GetTotalTrafficByPeriod(ctx context.Context, start, end time.Time) (upload, download int64, err error) {
	var result struct {
		Upload   int64
		Download int64
	}
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	err = r.db.WithContext(ctx).
		Model(&Traffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), rangeArgs...).
		Scan(&result).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic by period", err)
	}
	return result.Upload, result.Download, nil
}

// GetTrafficByProtocol retrieves traffic statistics grouped by protocol.
func (r *trafficRepository) GetTrafficByProtocol(ctx context.Context, start, end time.Time) ([]*ProtocolTrafficStats, error) {
	var results []*ProtocolTrafficStats
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	err := r.db.WithContext(ctx).
		Table("traffic t").
		Select("COALESCE(p.protocol, ?) as protocol, COUNT(DISTINCT CASE WHEN p.id IS NOT NULL THEN p.id END) as count, COALESCE(SUM(t.upload), 0) as upload, COALESCE(SUM(t.download), 0) as download", unknownTrafficProtocol).
		Joins("LEFT JOIN proxies p ON t.proxy_id = p.id").
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "t.recorded_at"), rangeArgs...).
		Group("COALESCE(p.protocol, 'unknown')").
		Scan(&results).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by protocol", err)
	}
	return results, nil
}

// GetTrafficByUser retrieves traffic statistics grouped by user.
func (r *trafficRepository) GetTrafficByUser(ctx context.Context, start, end time.Time, limit int) ([]*UserTrafficStats, error) {
	dialect := r.db.Dialector.Name()
	rangeCondition := BuildTimeRangeCondition(dialect, "t.recorded_at")
	rangeArgs := BuildTimeRangeArgs(dialect, start, end)

	if dialect == "sqlite" {
		type sqliteUserTrafficStats struct {
			UserID       int64
			Username     string
			Email        string
			Upload       int64
			Download     int64
			ProxyCount   int64
			TrafficLimit int64
			LastActive   string `gorm:"column:last_active"`
		}

		var rows []*sqliteUserTrafficStats
		selectClause := fmt.Sprintf(
			"t.user_id, COALESCE(u.username, '') as username, COALESCE(u.email, '') as email, COALESCE(u.traffic_limit, 0) as traffic_limit, COALESCE(SUM(t.upload), 0) as upload, COALESCE(SUM(t.download), 0) as download, COUNT(DISTINCT CASE WHEN t.proxy_id > 0 THEN t.proxy_id END) as proxy_count, %s as last_active",
			BuildTimeMaxExpr(dialect, "t.recorded_at"),
		)
		query := r.db.WithContext(ctx).
			Table("traffic t").
			Select(selectClause).
			Joins("LEFT JOIN users u ON t.user_id = u.id").
			Where("t.user_id > 0 AND "+rangeCondition, rangeArgs...).
			Group("t.user_id, COALESCE(u.username, ''), COALESCE(u.email, ''), COALESCE(u.traffic_limit, 0)").
			Order("(COALESCE(SUM(t.upload), 0) + COALESCE(SUM(t.download), 0)) DESC")

		if limit > 0 {
			query = query.Limit(limit)
		}

		if err := query.Scan(&rows).Error; err != nil {
			return nil, errors.NewDatabaseError("failed to get traffic by user", err)
		}

		results := make([]*UserTrafficStats, 0, len(rows))
		for _, row := range rows {
			lastActive, err := ParseAggregatedTime(dialect, row.LastActive, time.Local)
			if err != nil {
				return nil, errors.NewDatabaseError("failed to parse last activity time", err)
			}

			results = append(results, &UserTrafficStats{
				UserID:       row.UserID,
				Username:     normalizeTrafficUsername(row.UserID, row.Username),
				Email:        row.Email,
				Upload:       row.Upload,
				Download:     row.Download,
				ProxyCount:   row.ProxyCount,
				TrafficLimit: row.TrafficLimit,
				LastActive:   lastActive,
			})
		}

		return results, nil
	}

	var results []*UserTrafficStats
	query := r.db.WithContext(ctx).
		Table("traffic t").
		Select("t.user_id, COALESCE(u.username, '') as username, COALESCE(u.email, '') as email, COALESCE(u.traffic_limit, 0) as traffic_limit, COALESCE(SUM(t.upload), 0) as upload, COALESCE(SUM(t.download), 0) as download, COUNT(DISTINCT CASE WHEN t.proxy_id > 0 THEN t.proxy_id END) as proxy_count, MAX(t.recorded_at) as last_active").
		Joins("LEFT JOIN users u ON t.user_id = u.id").
		Where("t.user_id > 0 AND "+rangeCondition, rangeArgs...).
		Group("t.user_id, COALESCE(u.username, ''), COALESCE(u.email, ''), COALESCE(u.traffic_limit, 0)").
		Order("(COALESCE(SUM(t.upload), 0) + COALESCE(SUM(t.download), 0)) DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(&results).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by user", err)
	}
	for _, result := range results {
		if result == nil {
			continue
		}
		result.Username = normalizeTrafficUsername(result.UserID, result.Username)
	}
	return results, nil
}

func normalizeTrafficUsername(userID int64, username string) string {
	if username != "" {
		return username
	}
	return fmt.Sprintf("deleted-user-%d", userID)
}

// GetTrafficTimeline retrieves traffic data points for timeline charts.
func (r *trafficRepository) GetTrafficTimeline(ctx context.Context, start, end time.Time, interval string) ([]*TrafficTimelinePoint, error) {
	// Use a temporary struct to scan string time values
	type tempResult struct {
		TimeStr  string `gorm:"column:time"`
		Upload   int64
		Download int64
	}

	var tempResults []*tempResult

	groupClause := r.timelineGroupingClause(interval)
	selectClause := groupClause + " as time, COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download"
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)

	err := r.db.WithContext(ctx).
		Table("traffic").
		Select(selectClause).
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), rangeArgs...).
		Group(groupClause).
		Order("time ASC").
		Scan(&tempResults).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get traffic timeline", err)
	}

	// Convert string times to time.Time
	results := make([]*TrafficTimelinePoint, len(tempResults))
	for i, temp := range tempResults {
		parsedTime := parseTimelineBucket(temp.TimeStr, interval)

		results[i] = &TrafficTimelinePoint{
			Time:     parsedTime,
			Upload:   temp.Upload,
			Download: temp.Download,
		}
	}

	return fillTrafficTimelineGaps(start, end, interval, results), nil
}

// GetTrafficTimelineByUser retrieves traffic data points for timeline charts filtered by user ID.
func (r *trafficRepository) GetTrafficTimelineByUser(ctx context.Context, userID int64, start, end time.Time, interval string) ([]*TrafficTimelinePoint, error) {
	// Use a temporary struct to scan string time values
	type tempResult struct {
		TimeStr  string `gorm:"column:time"`
		Upload   int64
		Download int64
	}

	var tempResults []*tempResult

	groupClause := r.timelineGroupingClause(interval)
	selectClause := groupClause + " as time, COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download"
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)

	err := r.db.WithContext(ctx).
		Table("traffic").
		Select(selectClause).
		Where("user_id = ? AND "+BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), append([]any{userID}, rangeArgs...)...).
		Group(groupClause).
		Order("time ASC").
		Scan(&tempResults).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get traffic timeline by user", err)
	}

	// Convert string times to time.Time
	results := make([]*TrafficTimelinePoint, len(tempResults))
	for i, temp := range tempResults {
		parsedTime := parseTimelineBucket(temp.TimeStr, interval)

		results[i] = &TrafficTimelinePoint{
			Time:     parsedTime,
			Upload:   temp.Upload,
			Download: temp.Download,
		}
	}

	return fillTrafficTimelineGaps(start, end, interval, results), nil
}

func fillTrafficTimelineGaps(start, end time.Time, interval string, points []*TrafficTimelinePoint) []*TrafficTimelinePoint {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return points
	}

	location := start.Location()
	if location == nil {
		location = time.Local
	}

	startBucket := alignTimelineBucket(start.In(location), interval)
	endBucket := alignTimelineBucket(end.In(location), interval)
	if endBucket.Before(startBucket) {
		return points
	}

	pointsByBucket := make(map[string]*TrafficTimelinePoint, len(points))
	for _, point := range points {
		if point == nil {
			continue
		}
		bucketTime := alignTimelineBucket(point.Time.In(location), interval)
		pointsByBucket[timelineBucketKey(bucketTime, interval)] = &TrafficTimelinePoint{
			Time:     bucketTime,
			Upload:   point.Upload,
			Download: point.Download,
		}
	}

	filled := make([]*TrafficTimelinePoint, 0)
	for current := startBucket; !current.After(endBucket); current = advanceTimelineBucket(current, interval) {
		key := timelineBucketKey(current, interval)
		if point, exists := pointsByBucket[key]; exists {
			filled = append(filled, point)
			continue
		}

		filled = append(filled, &TrafficTimelinePoint{
			Time:     current,
			Upload:   0,
			Download: 0,
		})
	}

	return filled
}

func alignTimelineBucket(ts time.Time, interval string) time.Time {
	location := ts.Location()
	if location == nil {
		location = time.Local
	}

	switch interval {
	case "day":
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, location)
	case "month":
		return time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, location)
	case "hour":
		fallthrough
	default:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, location)
	}
}

func advanceTimelineBucket(ts time.Time, interval string) time.Time {
	switch interval {
	case "day":
		return ts.AddDate(0, 0, 1)
	case "month":
		return ts.AddDate(0, 1, 0)
	case "hour":
		fallthrough
	default:
		return ts.Add(time.Hour)
	}
}

func timelineBucketKey(ts time.Time, interval string) string {
	switch interval {
	case "day":
		return ts.Format("2006-01-02")
	case "month":
		return ts.Format("2006-01")
	case "hour":
		fallthrough
	default:
		return ts.Format("2006-01-02 15:00:00")
	}
}

func parseTimelineBucket(raw, interval string) time.Time {
	switch interval {
	case "hour":
		if parsed, err := time.ParseInLocation("2006-01-02 15:00:00", raw, time.Local); err == nil {
			return parsed
		}
	case "day":
		if parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
			return parsed
		}
	case "month":
		if parsed, err := time.ParseInLocation("2006-01", raw, time.Local); err == nil {
			return parsed
		}
	default:
		if parsed, err := time.ParseInLocation("2006-01-02 15:00:00", raw, time.Local); err == nil {
			return parsed
		}
	}

	return time.Time{}
}
