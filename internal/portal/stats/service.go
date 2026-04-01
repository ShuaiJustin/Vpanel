// Package stats provides statistics services for the user portal.
package stats

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"gorm.io/gorm"

	"v/internal/database/repository"
)

const unknownProtocolName = "unknown"

// Service provides statistics operations for the user portal.
type Service struct {
	db              *gorm.DB
	trafficRepo     repository.TrafficRepository
	nodeTrafficRepo repository.NodeTrafficRepository
	userRepo        repository.UserRepository
}

// NewService creates a new stats service.
func NewService(
	db *gorm.DB,
	trafficRepo repository.TrafficRepository,
	nodeTrafficRepo repository.NodeTrafficRepository,
	userRepo repository.UserRepository,
) *Service {
	return &Service{
		db:              db,
		trafficRepo:     trafficRepo,
		nodeTrafficRepo: nodeTrafficRepo,
		userRepo:        userRepo,
	}
}

// TrafficSummary represents a summary of traffic usage.
type TrafficSummary struct {
	Upload      int64  `json:"upload"`
	Download    int64  `json:"download"`
	Total       int64  `json:"total"`
	UploadStr   string `json:"upload_str"`
	DownloadStr string `json:"download_str"`
	TotalStr    string `json:"total_str"`
}

// DailyTraffic represents traffic for a single day.
type DailyTraffic struct {
	Date     string `json:"date"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
	Total    int64  `json:"total"`
}

// TrafficStats represents traffic statistics for a period.
type TrafficStats struct {
	Summary   *TrafficSummary `json:"summary"`
	Daily     []*DailyTraffic `json:"daily"`
	StartDate string          `json:"start_date"`
	EndDate   string          `json:"end_date"`
	Period    string          `json:"period"` // day, week, month, year, custom
}

// NodeUsage represents user traffic grouped by node.
type NodeUsage struct {
	NodeID     int64   `json:"node_id"`
	NodeName   string  `json:"node_name"`
	Upload     int64   `json:"upload"`
	Download   int64   `json:"download"`
	Traffic    int64   `json:"traffic"`
	Percentage float64 `json:"percentage"`
}

// ProtocolUsage represents user traffic grouped by protocol.
type ProtocolUsage struct {
	Protocol   string  `json:"protocol"`
	Count      int64   `json:"count"`
	Upload     int64   `json:"upload"`
	Download   int64   `json:"download"`
	Traffic    int64   `json:"traffic"`
	Percentage float64 `json:"percentage"`
}

// GetTrafficSummary retrieves total traffic for a user.
func (s *Service) GetTrafficSummary(ctx context.Context, userID int64) (*TrafficSummary, error) {
	upload, download, err := s.trafficRepo.GetTotalByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return newTrafficSummary(upload, download), nil
}

// GetTrafficSummaryInRange retrieves traffic for a user within a time range.
func (s *Service) GetTrafficSummaryInRange(ctx context.Context, userID int64, start, end time.Time) (*TrafficSummary, error) {
	if s.db == nil {
		daily, err := s.GetDailyTrafficInRange(ctx, userID, start, end)
		if err != nil {
			return nil, err
		}
		return AggregateDaily(daily), nil
	}

	var result struct {
		Upload   int64
		Download int64
	}
	dialect := s.db.Dialector.Name()
	rangeArgs := repository.BuildTimeRangeArgs(dialect, start, end)
	if err := s.db.WithContext(ctx).
		Table("traffic").
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("user_id = ? AND "+repository.BuildTimeRangeCondition(dialect, "recorded_at"), append([]any{userID}, rangeArgs...)...).
		Scan(&result).Error; err != nil {
		return nil, err
	}
	return newTrafficSummary(result.Upload, result.Download), nil
}

// GetDailyTraffic retrieves daily traffic for a user within a date range.
func (s *Service) GetDailyTraffic(ctx context.Context, userID int64, days int) ([]*DailyTraffic, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	end := time.Now()
	start := end.AddDate(0, 0, -days)
	return s.GetDailyTrafficInRange(ctx, userID, start, end)
}

// GetDailyTrafficInRange retrieves daily traffic for a user within a time range.
func (s *Service) GetDailyTrafficInRange(ctx context.Context, userID int64, start, end time.Time) ([]*DailyTraffic, error) {
	timeline, err := s.trafficRepo.GetTrafficTimelineByUser(ctx, userID, start, end, "day")
	if err != nil {
		return nil, err
	}

	daily := make([]*DailyTraffic, len(timeline))
	for i, point := range timeline {
		daily[i] = &DailyTraffic{
			Date:     point.Time.Format("2006-01-02"),
			Upload:   point.Upload,
			Download: point.Download,
			Total:    point.Upload + point.Download,
		}
	}

	return daily, nil
}

// GetTrafficStats retrieves traffic statistics for a period.
func (s *Service) GetTrafficStats(ctx context.Context, userID int64, period string) (*TrafficStats, error) {
	resolvedPeriod, start, end, err := ResolveRange(period, "", "")
	if err != nil {
		return nil, err
	}
	return s.GetTrafficStatsInRange(ctx, userID, resolvedPeriod, start, end)
}

// GetTrafficStatsInRange retrieves traffic statistics within an explicit time range.
func (s *Service) GetTrafficStatsInRange(ctx context.Context, userID int64, period string, start, end time.Time) (*TrafficStats, error) {
	summary, err := s.GetTrafficSummaryInRange(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	daily, err := s.GetDailyTrafficInRange(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	return &TrafficStats{
		Summary:   summary,
		Daily:     daily,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		Period:    period,
	}, nil
}

// GetUsageStats retrieves usage grouped by node and protocol for a time range.
func (s *Service) GetUsageStats(ctx context.Context, userID int64, period, startDate, endDate string) (*TrafficSummary, []*NodeUsage, []*ProtocolUsage, error) {
	resolvedPeriod, start, end, err := ResolveRange(period, startDate, endDate)
	if err != nil {
		return nil, nil, nil, err
	}

	summary, err := s.GetTrafficSummaryInRange(ctx, userID, start, end)
	if err != nil {
		return nil, nil, nil, err
	}

	byNode, err := s.getNodeUsage(ctx, userID, start, end, summary.Total)
	if err != nil {
		return nil, nil, nil, err
	}

	byProtocol, err := s.getProtocolUsage(ctx, userID, start, end, summary.Total)
	if err != nil {
		return nil, nil, nil, err
	}

	if resolvedPeriod == "" {
		resolvedPeriod = "month"
	}

	return summary, byNode, byProtocol, nil
}

func (s *Service) getNodeUsage(ctx context.Context, userID int64, start, end time.Time, total int64) ([]*NodeUsage, error) {
	if s.db == nil {
		return []*NodeUsage{}, nil
	}

	type nodeUsageRow struct {
		NodeID   int64
		NodeName string
		Upload   int64
		Download int64
	}

	var rows []nodeUsageRow
	dialect := s.db.Dialector.Name()
	rangeArgs := repository.BuildTimeRangeArgs(dialect, start, end)
	if err := s.db.WithContext(ctx).
		Table("node_traffic nt").
		Select("nt.node_id, COALESCE(n.name, '') as node_name, COALESCE(SUM(nt.upload), 0) as upload, COALESCE(SUM(nt.download), 0) as download").
		Joins("LEFT JOIN nodes n ON n.id = nt.node_id").
		Where("nt.user_id = ? AND "+repository.BuildTimeRangeCondition(dialect, "nt.recorded_at"), append([]any{userID}, rangeArgs...)...).
		Group("nt.node_id, COALESCE(n.name, '')").
		Order("(COALESCE(SUM(nt.upload), 0) + COALESCE(SUM(nt.download), 0)) DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]*NodeUsage, 0, len(rows))
	for _, row := range rows {
		traffic := row.Upload + row.Download
		percentage := float64(0)
		if total > 0 {
			percentage = float64(traffic) / float64(total) * 100
		}
		result = append(result, &NodeUsage{
			NodeID:     row.NodeID,
			NodeName:   normalizeNodeUsageName(row.NodeID, row.NodeName),
			Upload:     row.Upload,
			Download:   row.Download,
			Traffic:    traffic,
			Percentage: percentage,
		})
	}

	return result, nil
}

func (s *Service) getProtocolUsage(ctx context.Context, userID int64, start, end time.Time, total int64) ([]*ProtocolUsage, error) {
	if s.db == nil {
		return []*ProtocolUsage{}, nil
	}

	type protocolUsageRow struct {
		Protocol string
		Count    int64
		Upload   int64
		Download int64
	}

	var rows []protocolUsageRow
	dialect := s.db.Dialector.Name()
	rangeArgs := repository.BuildTimeRangeArgs(dialect, start, end)
	if err := s.db.WithContext(ctx).
		Table("traffic t").
		Select("COALESCE(p.protocol, ?) as protocol, COUNT(DISTINCT CASE WHEN p.id IS NOT NULL THEN t.proxy_id END) as count, COALESCE(SUM(t.upload), 0) as upload, COALESCE(SUM(t.download), 0) as download", unknownProtocolName).
		Joins("LEFT JOIN proxies p ON p.id = t.proxy_id").
		Where("t.user_id = ? AND "+repository.BuildTimeRangeCondition(dialect, "t.recorded_at"), append([]any{userID}, rangeArgs...)...).
		Group("COALESCE(p.protocol, 'unknown')").
		Order("(COALESCE(SUM(t.upload), 0) + COALESCE(SUM(t.download), 0)) DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]*ProtocolUsage, 0, len(rows))
	for _, row := range rows {
		traffic := row.Upload + row.Download
		percentage := float64(0)
		if total > 0 {
			percentage = float64(traffic) / float64(total) * 100
		}
		result = append(result, &ProtocolUsage{
			Protocol:   row.Protocol,
			Count:      row.Count,
			Upload:     row.Upload,
			Download:   row.Download,
			Traffic:    traffic,
			Percentage: percentage,
		})
	}

	return result, nil
}

func normalizeNodeUsageName(nodeID int64, nodeName string) string {
	if nodeName != "" {
		return nodeName
	}
	return fmt.Sprintf("deleted-node-%d", nodeID)
}

// ExportTrafficCSV exports traffic data as CSV.
func (s *Service) ExportTrafficCSV(ctx context.Context, userID int64, days int) ([]byte, error) {
	daily, err := s.GetDailyTraffic(ctx, userID, days)
	if err != nil {
		return nil, err
	}

	return buildTrafficCSV(daily)
}

// ExportTrafficCSVInRange exports traffic data as CSV for an explicit range.
func (s *Service) ExportTrafficCSVInRange(ctx context.Context, userID int64, start, end time.Time) ([]byte, error) {
	daily, err := s.GetDailyTrafficInRange(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	return buildTrafficCSV(daily)
}

func buildTrafficCSV(daily []*DailyTraffic) ([]byte, error) {

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	if err := writer.Write([]string{"Date", "Upload (bytes)", "Download (bytes)", "Total (bytes)", "Upload", "Download", "Total"}); err != nil {
		return nil, err
	}

	for _, d := range daily {
		record := []string{
			d.Date,
			fmt.Sprintf("%d", d.Upload),
			fmt.Sprintf("%d", d.Download),
			fmt.Sprintf("%d", d.Total),
			FormatBytes(d.Upload),
			FormatBytes(d.Download),
			FormatBytes(d.Total),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// AggregateDaily aggregates a list of daily traffic records.
func AggregateDaily(daily []*DailyTraffic) *TrafficSummary {
	var upload, download int64
	for _, d := range daily {
		upload += d.Upload
		download += d.Download
	}
	return newTrafficSummary(upload, download)
}

func newTrafficSummary(upload, download int64) *TrafficSummary {
	total := upload + download
	return &TrafficSummary{
		Upload:      upload,
		Download:    download,
		Total:       total,
		UploadStr:   FormatBytes(upload),
		DownloadStr: FormatBytes(download),
		TotalStr:    FormatBytes(total),
	}
}

// ResolveRange validates and resolves a period or explicit date range.
func ResolveRange(period, startDate, endDate string) (string, time.Time, time.Time, error) {
	return resolveRangeAt(time.Now(), period, startDate, endDate)
}

func resolveRangeAt(now time.Time, period, startDate, endDate string) (string, time.Time, time.Time, error) {
	if startDate != "" || endDate != "" {
		period = "custom"
	}
	period = ValidatePeriod(period)

	switch period {
	case "custom":
		if startDate == "" {
			return "", time.Time{}, time.Time{}, fmt.Errorf("invalid start date")
		}
		if endDate == "" {
			return "", time.Time{}, time.Time{}, fmt.Errorf("invalid end date")
		}
		start, err := time.ParseInLocation("2006-01-02", startDate, now.Location())
		if err != nil {
			return "", time.Time{}, time.Time{}, fmt.Errorf("invalid start date")
		}
		end, err := time.ParseInLocation("2006-01-02", endDate, now.Location())
		if err != nil {
			return "", time.Time{}, time.Time{}, fmt.Errorf("invalid end date")
		}
		if end.Before(start) {
			return "", time.Time{}, time.Time{}, fmt.Errorf("end date must not be before start date")
		}
		end = end.Add(24*time.Hour - time.Nanosecond)
		return period, start, end, nil
	case "day":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return period, start, now, nil
	case "week":
		return period, portalStartOfWeek(now), now, nil
	case "month":
		return period, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), now, nil
	case "year":
		return period, time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()), now, nil
	default:
		return "month", time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), now, nil
	}
}

func portalStartOfWeek(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	date := now.AddDate(0, 0, -(weekday - 1))
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

// FormatBytes formats bytes into human-readable string.
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ValidatePeriod validates and normalizes a period string.
func ValidatePeriod(period string) string {
	switch period {
	case "day", "week", "month", "year", "custom":
		return period
	default:
		return "month"
	}
}

// GetPeriodDays returns the number of days for a period.
func GetPeriodDays(period string) int {
	switch period {
	case "day":
		return 1
	case "week":
		return 7
	case "month":
		return 30
	case "year":
		return 365
	default:
		return 30
	}
}
