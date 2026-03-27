// Package node provides node management functionality for multi-server management.
package node

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
	apperrors "v/pkg/errors"
)

// TrafficStats represents aggregated traffic statistics.
type TrafficStats struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// NodeTrafficStats represents traffic statistics for a specific node.
type NodeTrafficStats struct {
	NodeID   int64 `json:"node_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// UserTrafficStats represents traffic statistics for a specific user.
type UserTrafficStats struct {
	UserID   int64 `json:"user_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// UserNodeTrafficStats represents traffic statistics for a user on a specific node.
type UserNodeTrafficStats struct {
	UserID   int64 `json:"user_id"`
	NodeID   int64 `json:"node_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// GroupTrafficStats represents traffic statistics for a node group.
type GroupTrafficStats struct {
	GroupID  int64 `json:"group_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// ProxyTrafficStats represents traffic statistics for a specific proxy.
type ProxyTrafficStats struct {
	ProxyID  int64 `json:"proxy_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// TrafficRecord represents a single traffic record for recording.
type TrafficRecord struct {
	NodeID   int64  `json:"node_id"`
	UserID   int64  `json:"user_id"`
	ProxyID  *int64 `json:"proxy_id,omitempty"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// TrafficFilter defines filter options for querying traffic.
type TrafficFilter struct {
	NodeID  *int64
	UserID  *int64
	GroupID *int64
	ProxyID *int64
	Start   time.Time
	End     time.Time
}

// NodeTrafficAlert represents a node traffic threshold or hard-limit alert.
type NodeTrafficAlert struct {
	NodeID           int64
	NodeName         string
	Level            string
	TrafficTotal     int64
	TrafficLimit     int64
	UsagePercent     float64
	ThresholdPercent float64
	TriggeredAt      time.Time
}

// TrafficService provides traffic statistics aggregation operations.
type TrafficService struct {
	db                           *gorm.DB
	nodeTrafficRepo              repository.NodeTrafficRepository
	trafficRepo                  repository.TrafficRepository
	proxyRepo                    repository.ProxyRepository
	userRepo                     repository.UserRepository
	nodeRepo                     repository.NodeRepository
	groupRepo                    repository.NodeGroupRepository
	userAccessCheck              func(context.Context, int64) error
	accessRevokedHook            func(context.Context, int64, []int64, string)
	nodeTrafficLimitExceededHook func(context.Context, int64, string)
	nodeTrafficAlertHook         func(context.Context, *NodeTrafficAlert)
	logger                       logger.Logger
}

// NewTrafficService creates a new traffic service.
func NewTrafficService(
	db *gorm.DB,
	nodeTrafficRepo repository.NodeTrafficRepository,
	trafficRepo repository.TrafficRepository,
	proxyRepo repository.ProxyRepository,
	userRepo repository.UserRepository,
	nodeRepo repository.NodeRepository,
	groupRepo repository.NodeGroupRepository,
	log logger.Logger,
) *TrafficService {
	return &TrafficService{
		db:              db,
		nodeTrafficRepo: nodeTrafficRepo,
		trafficRepo:     trafficRepo,
		proxyRepo:       proxyRepo,
		userRepo:        userRepo,
		nodeRepo:        nodeRepo,
		groupRepo:       groupRepo,
		logger:          log,
	}
}

// WithUserAccessCheck registers a post-update access checker used to detect revoked users.
func (s *TrafficService) WithUserAccessCheck(check func(context.Context, int64) error) *TrafficService {
	s.userAccessCheck = check
	return s
}

// WithAccessRevokedHook registers a callback triggered after traffic updates revoke access.
func (s *TrafficService) WithAccessRevokedHook(hook func(context.Context, int64, []int64, string)) *TrafficService {
	s.accessRevokedHook = hook
	return s
}

// WithNodeTrafficLimitExceededHook registers a callback triggered when a node exceeds its traffic limit.
func (s *TrafficService) WithNodeTrafficLimitExceededHook(hook func(context.Context, int64, string)) *TrafficService {
	s.nodeTrafficLimitExceededHook = hook
	return s
}

// WithNodeTrafficAlertHook registers a callback triggered when node traffic crosses an alert threshold.
func (s *TrafficService) WithNodeTrafficAlertHook(hook func(context.Context, *NodeTrafficAlert)) *TrafficService {
	s.nodeTrafficAlertHook = hook
	return s
}

// RecordTraffic records a traffic entry for a node.
func (s *TrafficService) RecordTraffic(ctx context.Context, record *TrafficRecord) error {
	if record == nil {
		return nil
	}
	return s.RecordTrafficBatch(ctx, []*TrafficRecord{record})
}

// RecordTrafficBatch records multiple traffic entries.
func (s *TrafficService) RecordTrafficBatch(ctx context.Context, records []*TrafficRecord) error {
	if len(records) == 0 {
		return nil
	}

	normalized := normalizeTrafficRecords(records)
	if len(normalized) == 0 {
		return nil
	}

	// Check node traffic limits before recording
	if s.nodeRepo != nil {
		nodeIDs := make(map[int64]struct{})
		for _, r := range normalized {
			nodeIDs[r.NodeID] = struct{}{}
		}
		for nodeID := range nodeIDs {
			nodeData, err := s.nodeRepo.GetByID(ctx, nodeID)
			if err != nil {
				continue
			}
			if nodeData.TrafficLimit > 0 && nodeData.TrafficTotal >= nodeData.TrafficLimit {
				s.logger.Warn("Node traffic limit exceeded, dropping traffic records",
					logger.F("node_id", nodeID),
					logger.F("traffic_total", nodeData.TrafficTotal),
					logger.F("traffic_limit", nodeData.TrafficLimit))
				// Filter out records for this node
				filtered := make([]*TrafficRecord, 0, len(normalized))
				for _, r := range normalized {
					if r.NodeID != nodeID {
						filtered = append(filtered, r)
					}
				}
				normalized = filtered
			}
		}
		if len(normalized) == 0 {
			return nil
		}
	}

	now := time.Now()
	batchNodesByUser := collectBatchNodesByUser(normalized)
	beforeNodes := s.snapshotNodesByID(ctx, collectRecordNodeIDs(normalized))

	if s.db == nil {
		traffic := make([]*repository.NodeTraffic, len(normalized))
		for i, r := range normalized {
			traffic[i] = &repository.NodeTraffic{
				NodeID:     r.NodeID,
				UserID:     r.UserID,
				ProxyID:    r.ProxyID,
				Upload:     r.Upload,
				Download:   r.Download,
				RecordedAt: now,
			}
		}

		if err := s.nodeTrafficRepo.CreateBatch(ctx, traffic); err != nil {
			s.logger.Error("Failed to record traffic batch", logger.Err(err))
			return err
		}
		return nil
	}

	userTotals := collectUserTrafficTotals(normalized)

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nodeTrafficRecords := make([]*repository.NodeTraffic, 0, len(normalized))
		globalTrafficRecords := make([]*repository.Traffic, 0, len(normalized))
		nodeUploads := make(map[int64]int64)
		nodeDownloads := make(map[int64]int64)

		for _, record := range normalized {
			nodeTrafficRecords = append(nodeTrafficRecords, &repository.NodeTraffic{
				NodeID:     record.NodeID,
				UserID:     record.UserID,
				ProxyID:    record.ProxyID,
				Upload:     record.Upload,
				Download:   record.Download,
				RecordedAt: now,
			})

			if record.ProxyID != nil && *record.ProxyID > 0 {
				globalTrafficRecords = append(globalTrafficRecords, &repository.Traffic{
					UserID:     record.UserID,
					ProxyID:    *record.ProxyID,
					Upload:     record.Upload,
					Download:   record.Download,
					RecordedAt: now,
				})
			}

			nodeUploads[record.NodeID] += record.Upload
			nodeDownloads[record.NodeID] += record.Download
		}

		if len(nodeTrafficRecords) > 0 {
			if err := tx.Create(&nodeTrafficRecords).Error; err != nil {
				return err
			}
		}

		if len(globalTrafficRecords) > 0 {
			if err := tx.Create(&globalTrafficRecords).Error; err != nil {
				return err
			}
		}

		for userID, total := range userTotals {
			if total <= 0 {
				continue
			}
			if err := tx.Model(&repository.User{}).
				Where("id = ?", userID).
				Update("traffic_used", gorm.Expr("traffic_used + ?", total)).Error; err != nil {
				return err
			}
			if err := tx.Model(&repository.Trial{}).
				Where("user_id = ? AND status = ? AND expire_at > ?", userID, "active", now).
				Update("traffic_used", gorm.Expr("traffic_used + ?", total)).Error; err != nil {
				return err
			}
		}

		for nodeID, upload := range nodeUploads {
			download := nodeDownloads[nodeID]
			total := upload + download
			if total <= 0 {
				continue
			}
			if err := tx.Model(&repository.Node{}).
				Where("id = ?", nodeID).
				Updates(map[string]interface{}{
					"traffic_up":    gorm.Expr("traffic_up + ?", upload),
					"traffic_down":  gorm.Expr("traffic_down + ?", download),
					"traffic_total": gorm.Expr("traffic_total + ?", total),
				}).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		s.logger.Error("Failed to record traffic batch", logger.Err(err))
		return err
	}

	s.notifyNodeTrafficAlerts(ctx, beforeNodes, normalized, now)
	s.handleExceededNodeTrafficLimits(ctx, normalized)
	s.notifyRevokedUsers(ctx, batchNodesByUser, userTotals)

	return nil
}

func collectUserTrafficTotals(records []*TrafficRecord) map[int64]int64 {
	userTotals := make(map[int64]int64)
	for _, record := range records {
		if record == nil {
			continue
		}
		total := record.Upload + record.Download
		if total <= 0 {
			continue
		}
		userTotals[record.UserID] += total
	}
	return userTotals
}

func collectBatchNodesByUser(records []*TrafficRecord) map[int64]map[int64]struct{} {
	batchNodesByUser := make(map[int64]map[int64]struct{})
	for _, record := range records {
		if record == nil || record.UserID <= 0 || record.NodeID <= 0 {
			continue
		}
		nodeIDs := batchNodesByUser[record.UserID]
		if nodeIDs == nil {
			nodeIDs = make(map[int64]struct{})
			batchNodesByUser[record.UserID] = nodeIDs
		}
		nodeIDs[record.NodeID] = struct{}{}
	}
	return batchNodesByUser
}

func (s *TrafficService) notifyRevokedUsers(ctx context.Context, batchNodesByUser map[int64]map[int64]struct{}, userTotals map[int64]int64) {
	if s.userAccessCheck == nil || s.accessRevokedHook == nil || len(userTotals) == 0 {
		return
	}

	for _, userID := range sortedUserIDs(userTotals) {
		if userTotals[userID] <= 0 {
			continue
		}

		accessErr := s.userAccessCheck(ctx, userID)
		if accessErr == nil {
			continue
		}
		if !apperrors.IsForbidden(accessErr) {
			s.logger.Warn("failed to verify user access after traffic update",
				logger.Err(accessErr),
				logger.UserID(userID),
			)
			continue
		}

		nodeIDs, err := s.resolveAffectedNodeIDs(ctx, userID, batchNodesByUser[userID])
		if err != nil {
			s.logger.Warn("failed to resolve affected nodes for revoked user",
				logger.Err(err),
				logger.UserID(userID),
			)
			continue
		}
		if len(nodeIDs) == 0 {
			continue
		}

		reason := accessRevocationReason(accessErr)
		s.logger.Info("user access revoked after traffic update",
			logger.UserID(userID),
			logger.F("node_ids", nodeIDs),
			logger.F("reason", reason),
		)
		s.accessRevokedHook(ctx, userID, nodeIDs, reason)
	}
}

func (s *TrafficService) resolveAffectedNodeIDs(ctx context.Context, userID int64, batchNodeIDs map[int64]struct{}) ([]int64, error) {
	unique := make(map[int64]struct{})
	for nodeID := range batchNodeIDs {
		if nodeID > 0 {
			unique[nodeID] = struct{}{}
		}
	}

	if s.proxyRepo != nil {
		proxies, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
		if err != nil {
			return nil, err
		}
		for _, proxy := range proxies {
			if proxy != nil && proxy.NodeID != nil && *proxy.NodeID > 0 {
				unique[*proxy.NodeID] = struct{}{}
			}
		}
	}

	if len(unique) == 0 {
		return nil, nil
	}

	nodeIDs := make([]int64, 0, len(unique))
	for nodeID := range unique {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Slice(nodeIDs, func(i, j int) bool { return nodeIDs[i] < nodeIDs[j] })
	return nodeIDs, nil
}

func sortedUserIDs(userTotals map[int64]int64) []int64 {
	userIDs := make([]int64, 0, len(userTotals))
	for userID := range userTotals {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	return userIDs
}

func collectRecordNodeIDs(records []*TrafficRecord) map[int64]struct{} {
	nodeIDs := make(map[int64]struct{})
	for _, record := range records {
		if record == nil || record.NodeID <= 0 {
			continue
		}
		nodeIDs[record.NodeID] = struct{}{}
	}
	return nodeIDs
}

func (s *TrafficService) snapshotNodesByID(ctx context.Context, nodeIDs map[int64]struct{}) map[int64]*repository.Node {
	if s == nil || s.nodeRepo == nil || len(nodeIDs) == 0 {
		return nil
	}

	snapshots := make(map[int64]*repository.Node, len(nodeIDs))
	for _, nodeID := range sortedNodeIDs(nodeIDs) {
		nodeData, err := s.nodeRepo.GetByID(ctx, nodeID)
		if err != nil || nodeData == nil {
			continue
		}
		snapshots[nodeID] = cloneNodeSnapshot(nodeData)
	}
	return snapshots
}

func cloneNodeSnapshot(nodeData *repository.Node) *repository.Node {
	if nodeData == nil {
		return nil
	}
	clone := *nodeData
	return &clone
}

func (s *TrafficService) notifyNodeTrafficAlerts(ctx context.Context, beforeNodes map[int64]*repository.Node, records []*TrafficRecord, triggeredAt time.Time) {
	if s == nil || s.nodeRepo == nil || s.nodeTrafficAlertHook == nil || len(records) == 0 {
		return
	}

	nodeIDs := collectRecordNodeIDs(records)
	for _, nodeID := range sortedNodeIDs(nodeIDs) {
		afterNode, err := s.nodeRepo.GetByID(ctx, nodeID)
		if err != nil || afterNode == nil {
			continue
		}
		for _, alert := range buildNodeTrafficAlerts(beforeNodes[nodeID], afterNode, triggeredAt) {
			s.nodeTrafficAlertHook(ctx, alert)
		}
	}
}

func buildNodeTrafficAlerts(beforeNode, afterNode *repository.Node, triggeredAt time.Time) []*NodeTrafficAlert {
	if afterNode == nil || afterNode.TrafficLimit <= 0 {
		return nil
	}

	alerts := make([]*NodeTrafficAlert, 0, 2)
	beforeUsage := nodeTrafficUsagePercent(beforeNode)
	afterUsage := nodeTrafficUsagePercent(afterNode)
	threshold := normalizedTrafficAlertThreshold(afterNode)

	if threshold > 0 && beforeUsage < threshold && afterUsage >= threshold {
		alerts = append(alerts, newNodeTrafficAlert(afterNode, "threshold", threshold, triggeredAt))
	}
	if beforeUsage < 100 && afterUsage >= 100 {
		alerts = append(alerts, newNodeTrafficAlert(afterNode, "limit", 100, triggeredAt))
	}

	return alerts
}

func normalizedTrafficAlertThreshold(nodeData *repository.Node) float64 {
	if nodeData == nil || nodeData.TrafficLimit <= 0 {
		return 0
	}
	threshold := nodeData.AlertTrafficThreshold
	if threshold <= 0 {
		return 0
	}
	if threshold > 100 {
		threshold = 100
	}
	return threshold
}

func nodeTrafficUsagePercent(nodeData *repository.Node) float64 {
	if nodeData == nil || nodeData.TrafficLimit <= 0 {
		return 0
	}
	return float64(nodeData.TrafficTotal) * 100 / float64(nodeData.TrafficLimit)
}

func newNodeTrafficAlert(nodeData *repository.Node, level string, threshold float64, triggeredAt time.Time) *NodeTrafficAlert {
	if nodeData == nil {
		return nil
	}
	return &NodeTrafficAlert{
		NodeID:           nodeData.ID,
		NodeName:         nodeData.Name,
		Level:            level,
		TrafficTotal:     nodeData.TrafficTotal,
		TrafficLimit:     nodeData.TrafficLimit,
		UsagePercent:     nodeTrafficUsagePercent(nodeData),
		ThresholdPercent: threshold,
		TriggeredAt:      triggeredAt,
	}
}

func (s *TrafficService) handleExceededNodeTrafficLimits(ctx context.Context, records []*TrafficRecord) {
	if s == nil || s.nodeRepo == nil || len(records) == 0 {
		return
	}

	nodeIDs := make(map[int64]struct{})
	for _, record := range records {
		if record == nil || record.NodeID <= 0 {
			continue
		}
		nodeIDs[record.NodeID] = struct{}{}
	}
	if len(nodeIDs) == 0 {
		return
	}

	for _, nodeID := range sortedNodeIDs(nodeIDs) {
		nodeData, err := s.nodeRepo.GetByID(ctx, nodeID)
		if err != nil || !nodeTrafficLimitExceeded(nodeData) {
			continue
		}
		if nodeData.Status != repository.NodeStatusUnhealthy {
			if err := s.nodeRepo.UpdateStatus(ctx, nodeID, repository.NodeStatusUnhealthy); err != nil {
				s.logger.Warn("failed to mark node unhealthy after traffic limit exceeded",
					logger.Err(err),
					logger.F("node_id", nodeID),
				)
				continue
			}
			s.logger.Warn("node traffic limit exceeded; node marked unhealthy",
				logger.F("node_id", nodeID),
				logger.F("traffic_total", nodeData.TrafficTotal),
				logger.F("traffic_limit", nodeData.TrafficLimit),
			)
		}
		if s.nodeTrafficLimitExceededHook != nil {
			reason := "node traffic limit exceeded"
			s.nodeTrafficLimitExceededHook(ctx, nodeID, reason)
		}
	}
}

func sortedNodeIDs(nodeIDs map[int64]struct{}) []int64 {
	ids := make([]int64, 0, len(nodeIDs))
	for nodeID := range nodeIDs {
		ids = append(ids, nodeID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func nodeTrafficLimitExceeded(nodeData *repository.Node) bool {
	return nodeData != nil && nodeData.TrafficLimit > 0 && nodeData.TrafficTotal >= nodeData.TrafficLimit
}

func accessRevocationReason(err error) string {
	if err == nil {
		return ""
	}
	if appErr, ok := apperrors.AsAppError(err); ok && appErr.Message != "" {
		return appErr.Message
	}
	return err.Error()
}

func normalizeTrafficRecords(records []*TrafficRecord) []*TrafficRecord {
	normalized := make([]*TrafficRecord, 0, len(records))
	for _, record := range records {
		if record == nil || record.UserID <= 0 || record.NodeID <= 0 {
			continue
		}

		upload := record.Upload
		download := record.Download
		if upload < 0 {
			upload = 0
		}
		if download < 0 {
			download = 0
		}
		if upload == 0 && download == 0 {
			continue
		}

		normalizedRecord := &TrafficRecord{
			NodeID:   record.NodeID,
			UserID:   record.UserID,
			Upload:   upload,
			Download: download,
		}
		if record.ProxyID != nil && *record.ProxyID > 0 {
			proxyID := *record.ProxyID
			normalizedRecord.ProxyID = &proxyID
		}
		normalized = append(normalized, normalizedRecord)
	}
	return normalized
}

// GetTotalTraffic returns total traffic across all nodes within a time range.
func (s *TrafficService) GetTotalTraffic(ctx context.Context, start, end time.Time) (*TrafficStats, error) {
	upload, download, err := s.nodeTrafficRepo.GetTotalTraffic(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get total traffic", logger.Err(err))
		return nil, err
	}

	return &TrafficStats{
		Upload:   upload,
		Download: download,
		Total:    upload + download,
	}, nil
}

// GetTrafficByNode returns traffic statistics for a specific node.
func (s *TrafficService) GetTrafficByNode(ctx context.Context, nodeID int64, start, end time.Time) (*NodeTrafficStats, error) {
	upload, download, err := s.nodeTrafficRepo.GetTotalByNodeInRange(ctx, nodeID, start, end)
	if err != nil {
		s.logger.Error("Failed to get traffic by node",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return nil, err
	}

	return &NodeTrafficStats{
		NodeID:   nodeID,
		Upload:   upload,
		Download: download,
		Total:    upload + download,
	}, nil
}

// GetTrafficByUser returns traffic statistics for a specific user across all nodes.
func (s *TrafficService) GetTrafficByUser(ctx context.Context, userID int64, start, end time.Time) (*UserTrafficStats, error) {
	upload, download, err := s.nodeTrafficRepo.GetTotalByUserInRange(ctx, userID, start, end)
	if err != nil {
		s.logger.Error("Failed to get traffic by user",
			logger.Err(err),
			logger.F("user_id", userID))
		return nil, err
	}

	return &UserTrafficStats{
		UserID:   userID,
		Upload:   upload,
		Download: download,
		Total:    upload + download,
	}, nil
}

// GetTrafficByUserOnNode returns traffic statistics for a user on a specific node.
func (s *TrafficService) GetTrafficByUserOnNode(ctx context.Context, userID, nodeID int64) (*UserNodeTrafficStats, error) {
	upload, download, err := s.nodeTrafficRepo.GetTotalByUserOnNode(ctx, userID, nodeID)
	if err != nil {
		s.logger.Error("Failed to get traffic by user on node",
			logger.Err(err),
			logger.F("user_id", userID),
			logger.F("node_id", nodeID))
		return nil, err
	}

	return &UserNodeTrafficStats{
		UserID:   userID,
		NodeID:   nodeID,
		Upload:   upload,
		Download: download,
		Total:    upload + download,
	}, nil
}

// GetTrafficStatsByNode returns traffic statistics grouped by node.
func (s *TrafficService) GetTrafficStatsByNode(ctx context.Context, start, end time.Time) ([]*NodeTrafficStats, error) {
	repoStats, err := s.nodeTrafficRepo.GetStatsByNode(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get traffic stats by node", logger.Err(err))
		return nil, err
	}

	stats := make([]*NodeTrafficStats, len(repoStats))
	for i, rs := range repoStats {
		stats[i] = &NodeTrafficStats{
			NodeID:   rs.NodeID,
			Upload:   rs.Upload,
			Download: rs.Download,
			Total:    rs.Total,
		}
	}

	return stats, nil
}

// GetTrafficStatsByGroup returns traffic statistics grouped by node group.
func (s *TrafficService) GetTrafficStatsByGroup(ctx context.Context, start, end time.Time) ([]*GroupTrafficStats, error) {
	repoStats, err := s.nodeTrafficRepo.GetStatsByGroup(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get traffic stats by group", logger.Err(err))
		return nil, err
	}

	stats := make([]*GroupTrafficStats, len(repoStats))
	for i, rs := range repoStats {
		stats[i] = &GroupTrafficStats{
			GroupID:  rs.GroupID,
			Upload:   rs.Upload,
			Download: rs.Download,
			Total:    rs.Upload + rs.Download,
		}
	}

	return stats, nil
}

// GetTrafficByGroup returns traffic statistics for a specific group.
func (s *TrafficService) GetTrafficByGroup(ctx context.Context, groupID int64, start, end time.Time) (*GroupTrafficStats, error) {
	// Get all node IDs in the group
	nodeIDs, err := s.groupRepo.GetNodeIDs(ctx, groupID)
	if err != nil {
		s.logger.Error("Failed to get node IDs for group",
			logger.Err(err),
			logger.F("group_id", groupID))
		return nil, err
	}

	if len(nodeIDs) == 0 {
		return &GroupTrafficStats{
			GroupID:  groupID,
			Upload:   0,
			Download: 0,
			Total:    0,
		}, nil
	}

	// Aggregate traffic for all nodes in the group in a single query
	type result struct {
		Upload   int64
		Download int64
	}
	var res result
	rangeArgs := repository.BuildTimeRangeArgs(s.db.Dialector.Name(), start, end)
	err = s.db.WithContext(ctx).
		Model(&repository.NodeTraffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("node_id IN ? AND "+repository.BuildTimeRangeCondition(s.db.Dialector.Name(), "recorded_at"), append([]any{nodeIDs}, rangeArgs...)...).
		Scan(&res).Error
	if err != nil {
		s.logger.Error("Failed to get traffic for group",
			logger.Err(err),
			logger.F("group_id", groupID))
		return nil, err
	}

	return &GroupTrafficStats{
		GroupID:  groupID,
		Upload:   res.Upload,
		Download: res.Download,
		Total:    res.Upload + res.Download,
	}, nil
}

// GetUserTrafficBreakdownByNode returns traffic breakdown by node for a specific user.
func (s *TrafficService) GetUserTrafficBreakdownByNode(ctx context.Context, userID int64, start, end time.Time) ([]*UserNodeTrafficStats, error) {
	// Get all traffic records for the user in the time range
	records, err := s.nodeTrafficRepo.GetByUserAndDateRange(ctx, userID, start, end)
	if err != nil {
		s.logger.Error("Failed to get user traffic records",
			logger.Err(err),
			logger.F("user_id", userID))
		return nil, err
	}

	// Aggregate by node
	nodeTraffic := make(map[int64]*UserNodeTrafficStats)
	for _, r := range records {
		if _, exists := nodeTraffic[r.NodeID]; !exists {
			nodeTraffic[r.NodeID] = &UserNodeTrafficStats{
				UserID: userID,
				NodeID: r.NodeID,
			}
		}
		nodeTraffic[r.NodeID].Upload += r.Upload
		nodeTraffic[r.NodeID].Download += r.Download
	}

	// Convert to slice and calculate totals
	stats := make([]*UserNodeTrafficStats, 0, len(nodeTraffic))
	for _, s := range nodeTraffic {
		s.Total = s.Upload + s.Download
		stats = append(stats, s)
	}

	return stats, nil
}

// GetTopUsersByTraffic returns top users by traffic on a specific node.
func (s *TrafficService) GetTopUsersByTraffic(ctx context.Context, nodeID int64, start, end time.Time, limit int) ([]*UserNodeTrafficStats, error) {
	repoStats, err := s.nodeTrafficRepo.GetStatsByUser(ctx, nodeID, start, end, limit)
	if err != nil {
		s.logger.Error("Failed to get top users by traffic",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return nil, err
	}

	stats := make([]*UserNodeTrafficStats, len(repoStats))
	for i, rs := range repoStats {
		stats[i] = &UserNodeTrafficStats{
			UserID:   rs.UserID,
			NodeID:   rs.NodeID,
			Upload:   rs.Upload,
			Download: rs.Download,
			Total:    rs.Upload + rs.Download,
		}
	}

	return stats, nil
}

// CleanupOldRecords deletes traffic records older than the specified duration.
func (s *TrafficService) CleanupOldRecords(ctx context.Context, retention time.Duration) (int64, error) {
	before := time.Now().Add(-retention)
	deleted, err := s.nodeTrafficRepo.DeleteOlderThan(ctx, before)
	if err != nil {
		s.logger.Error("Failed to cleanup old traffic records",
			logger.Err(err),
			logger.F("before", before))
		return 0, err
	}

	s.logger.Info("Cleaned up old traffic records",
		logger.F("deleted", deleted),
		logger.F("before", before))
	return deleted, nil
}

// DeleteByNode deletes all traffic records for a specific node.
func (s *TrafficService) DeleteByNode(ctx context.Context, nodeID int64) error {
	if err := s.nodeTrafficRepo.DeleteByNodeID(ctx, nodeID); err != nil {
		s.logger.Error("Failed to delete traffic by node",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return err
	}

	s.logger.Info("Deleted traffic records for node", logger.F("node_id", nodeID))
	return nil
}

// ResetNodeTraffic resets a node's accumulated traffic counters to zero.
func (s *TrafficService) ResetNodeTraffic(ctx context.Context, nodeID int64) error {
	return s.resetNodeTrafficAt(ctx, nodeID, time.Now())
}

func (s *TrafficService) resetNodeTrafficAt(ctx context.Context, nodeID int64, resetAt time.Time) error {
	if s.db == nil {
		return nil
	}
	if resetAt.IsZero() {
		resetAt = time.Now()
	}

	err := s.db.WithContext(ctx).
		Model(&repository.Node{}).
		Where("id = ?", nodeID).
		Updates(map[string]interface{}{
			"traffic_up":       0,
			"traffic_down":     0,
			"traffic_total":    0,
			"traffic_reset_at": resetAt,
		}).Error
	if err != nil {
		s.logger.Error("Failed to reset node traffic",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return err
	}

	s.logger.Info("Node traffic counters reset",
		logger.F("node_id", nodeID),
		logger.F("reset_at", resetAt))
	return nil
}

// ProcessMonthlyTrafficResets initializes missing reset anchors and resets nodes whose monthly cycle is due.
func (s *TrafficService) ProcessMonthlyTrafficResets(ctx context.Context, now time.Time) ([]int64, error) {
	if s == nil || s.nodeRepo == nil {
		return nil, nil
	}

	nodes, err := s.nodeRepo.List(ctx, &repository.NodeFilter{Limit: 10000})
	if err != nil {
		return nil, err
	}

	resetNodeIDs := make([]int64, 0)
	for _, nodeData := range nodes {
		if !nodeRequiresTrafficCycle(nodeData) {
			continue
		}
		if nodeData.TrafficResetAt == nil {
			if initErr := s.initializeNodeTrafficResetAt(ctx, nodeData.ID, now); initErr != nil {
				s.logger.Warn("failed to initialize node traffic reset anchor",
					logger.Err(initErr),
					logger.F("node_id", nodeData.ID),
				)
			}
			continue
		}
		if !nodeTrafficResetDue(nodeData, now) {
			continue
		}
		if resetErr := s.resetNodeTrafficAt(ctx, nodeData.ID, now); resetErr != nil {
			s.logger.Warn("failed to process monthly node traffic reset",
				logger.Err(resetErr),
				logger.F("node_id", nodeData.ID),
			)
			continue
		}
		resetNodeIDs = append(resetNodeIDs, nodeData.ID)
	}

	return resetNodeIDs, nil
}

func (s *TrafficService) initializeNodeTrafficResetAt(ctx context.Context, nodeID int64, resetAt time.Time) error {
	if s == nil || s.db == nil || nodeID <= 0 {
		return nil
	}
	if resetAt.IsZero() {
		resetAt = time.Now()
	}
	if err := s.db.WithContext(ctx).
		Model(&repository.Node{}).
		Where("id = ? AND traffic_reset_at IS NULL", nodeID).
		Update("traffic_reset_at", resetAt).Error; err != nil {
		return err
	}

	s.logger.Info("initialized node traffic reset anchor",
		logger.F("node_id", nodeID),
		logger.F("traffic_reset_at", resetAt),
	)
	return nil
}

func nodeRequiresTrafficCycle(nodeData *repository.Node) bool {
	return nodeData != nil && nodeData.ID > 0 && nodeData.TrafficLimit > 0
}

func nodeTrafficResetDue(nodeData *repository.Node, now time.Time) bool {
	if !nodeRequiresTrafficCycle(nodeData) || nodeData.TrafficResetAt == nil {
		return false
	}
	return !now.Before(nodeData.TrafficResetAt.AddDate(0, 1, 0))
}

// AggregatedTrafficStats represents comprehensive aggregated traffic statistics.
type AggregatedTrafficStats struct {
	TotalUpload   int64                `json:"total_upload"`
	TotalDownload int64                `json:"total_download"`
	Total         int64                `json:"total"`
	ByNode        []*NodeTrafficStats  `json:"by_node,omitempty"`
	ByGroup       []*GroupTrafficStats `json:"by_group,omitempty"`
}

// GetAggregatedStats returns comprehensive aggregated traffic statistics.
// This aggregates traffic by user, proxy, node, and group as specified in Requirements 8.2.
func (s *TrafficService) GetAggregatedStats(ctx context.Context, start, end time.Time) (*AggregatedTrafficStats, error) {
	// Get total traffic
	totalUpload, totalDownload, err := s.nodeTrafficRepo.GetTotalTraffic(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get total traffic for aggregation", logger.Err(err))
		return nil, err
	}

	// Get traffic by node
	nodeStats, err := s.GetTrafficStatsByNode(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get node stats for aggregation", logger.Err(err))
		return nil, err
	}

	// Get traffic by group
	groupStats, err := s.GetTrafficStatsByGroup(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get group stats for aggregation", logger.Err(err))
		return nil, err
	}

	return &AggregatedTrafficStats{
		TotalUpload:   totalUpload,
		TotalDownload: totalDownload,
		Total:         totalUpload + totalDownload,
		ByNode:        nodeStats,
		ByGroup:       groupStats,
	}, nil
}

// VerifyAggregationConsistency verifies that the sum of per-node traffic equals total traffic.
// This is used to validate Property 19: Traffic Aggregation Consistency.
func (s *TrafficService) VerifyAggregationConsistency(ctx context.Context, start, end time.Time) (bool, error) {
	// Get total traffic
	totalUpload, totalDownload, err := s.nodeTrafficRepo.GetTotalTraffic(ctx, start, end)
	if err != nil {
		return false, err
	}

	// Get traffic by node
	nodeStats, err := s.nodeTrafficRepo.GetStatsByNode(ctx, start, end)
	if err != nil {
		return false, err
	}

	// Sum up per-node traffic
	var sumUpload, sumDownload int64
	for _, ns := range nodeStats {
		sumUpload += ns.Upload
		sumDownload += ns.Download
	}

	// Verify consistency
	return sumUpload == totalUpload && sumDownload == totalDownload, nil
}

// VerifyUserTrafficConsistency verifies that the sum of per-node traffic for a user equals total user traffic.
// This is used to validate Property 19: Traffic Aggregation Consistency for user-level aggregation.
func (s *TrafficService) VerifyUserTrafficConsistency(ctx context.Context, userID int64, start, end time.Time) (bool, error) {
	// Get total traffic for user
	totalUpload, totalDownload, err := s.nodeTrafficRepo.GetTotalByUserInRange(ctx, userID, start, end)
	if err != nil {
		return false, err
	}

	// Get traffic breakdown by node for user
	breakdown, err := s.GetUserTrafficBreakdownByNode(ctx, userID, start, end)
	if err != nil {
		return false, err
	}

	// Sum up per-node traffic
	var sumUpload, sumDownload int64
	for _, b := range breakdown {
		sumUpload += b.Upload
		sumDownload += b.Download
	}

	// Verify consistency
	return sumUpload == totalUpload && sumDownload == totalDownload, nil
}

// AggregateTrafficByProxy aggregates traffic statistics by proxy.
func (s *TrafficService) AggregateTrafficByProxy(ctx context.Context, start, end time.Time) ([]*ProxyTrafficStats, error) {
	// Get all traffic records in the time range
	records, err := s.nodeTrafficRepo.GetByDateRange(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get traffic records for proxy aggregation", logger.Err(err))
		return nil, err
	}

	// Aggregate by proxy
	proxyTraffic := make(map[int64]*ProxyTrafficStats)
	for _, r := range records {
		if r.ProxyID == nil {
			continue
		}
		proxyID := *r.ProxyID
		if _, exists := proxyTraffic[proxyID]; !exists {
			proxyTraffic[proxyID] = &ProxyTrafficStats{
				ProxyID: proxyID,
			}
		}
		proxyTraffic[proxyID].Upload += r.Upload
		proxyTraffic[proxyID].Download += r.Download
	}

	// Convert to slice and calculate totals
	stats := make([]*ProxyTrafficStats, 0, len(proxyTraffic))
	for _, s := range proxyTraffic {
		s.Total = s.Upload + s.Download
		stats = append(stats, s)
	}

	return stats, nil
}

// AggregateTrafficByUserAndProxy aggregates traffic by user and proxy combination.
type UserProxyTrafficStats struct {
	UserID   int64 `json:"user_id"`
	ProxyID  int64 `json:"proxy_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// AggregateTrafficByUserAndProxy returns traffic aggregated by user and proxy.
func (s *TrafficService) AggregateTrafficByUserAndProxy(ctx context.Context, start, end time.Time) ([]*UserProxyTrafficStats, error) {
	// Get all traffic records in the time range
	records, err := s.nodeTrafficRepo.GetByDateRange(ctx, start, end)
	if err != nil {
		s.logger.Error("Failed to get traffic records for user-proxy aggregation", logger.Err(err))
		return nil, err
	}

	// Aggregate by user and proxy
	type key struct {
		userID  int64
		proxyID int64
	}
	traffic := make(map[key]*UserProxyTrafficStats)
	for _, r := range records {
		if r.ProxyID == nil {
			continue
		}
		k := key{userID: r.UserID, proxyID: *r.ProxyID}
		if _, exists := traffic[k]; !exists {
			traffic[k] = &UserProxyTrafficStats{
				UserID:  r.UserID,
				ProxyID: *r.ProxyID,
			}
		}
		traffic[k].Upload += r.Upload
		traffic[k].Download += r.Download
	}

	// Convert to slice and calculate totals
	stats := make([]*UserProxyTrafficStats, 0, len(traffic))
	for _, s := range traffic {
		s.Total = s.Upload + s.Download
		stats = append(stats, s)
	}

	return stats, nil
}
