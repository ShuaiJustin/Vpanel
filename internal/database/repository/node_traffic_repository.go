// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// NodeTraffic represents traffic statistics for a node.
type NodeTraffic struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	NodeID     int64     `gorm:"index;not null"`
	UserID     int64     `gorm:"index;not null"`
	ProxyID    *int64    `gorm:"index"`
	Upload     int64     `gorm:"default:0"` // bytes
	Download   int64     `gorm:"default:0"` // bytes
	RecordedAt time.Time `gorm:"index"`

	Node  *Node  `gorm:"foreignKey:NodeID"`
	User  *User  `gorm:"foreignKey:UserID"`
	Proxy *Proxy `gorm:"foreignKey:ProxyID"`
}

// TableName returns the table name for NodeTraffic.
func (NodeTraffic) TableName() string {
	return "node_traffic"
}

// NodeTrafficStats represents aggregated traffic statistics.
type NodeTrafficStats struct {
	NodeID   int64
	Upload   int64
	Download int64
	Total    int64
}

// UserNodeTrafficStats represents traffic statistics for a user on a node.
type UserNodeTrafficStats struct {
	UserID   int64
	NodeID   int64
	Upload   int64
	Download int64
}

// GroupTrafficStats represents traffic statistics for a node group.
type GroupTrafficStats struct {
	GroupID  int64
	Upload   int64
	Download int64
}

// NodeTrafficRepository defines the interface for node traffic data access.
type NodeTrafficRepository interface {
	// CRUD operations
	Create(ctx context.Context, traffic *NodeTraffic) error
	CreateBatch(ctx context.Context, traffic []*NodeTraffic) error
	GetByID(ctx context.Context, id int64) (*NodeTraffic, error)
	Delete(ctx context.Context, id int64) error

	// Query operations
	GetByNodeID(ctx context.Context, nodeID int64, limit, offset int) ([]*NodeTraffic, error)
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*NodeTraffic, error)
	GetByDateRange(ctx context.Context, start, end time.Time) ([]*NodeTraffic, error)
	GetByNodeAndDateRange(ctx context.Context, nodeID int64, start, end time.Time) ([]*NodeTraffic, error)
	GetByUserAndDateRange(ctx context.Context, userID int64, start, end time.Time) ([]*NodeTraffic, error)

	// Aggregation operations
	GetTotalByNode(ctx context.Context, nodeID int64) (upload, download int64, err error)
	GetTotalByUser(ctx context.Context, userID int64) (upload, download int64, err error)
	GetTotalByUserOnNode(ctx context.Context, userID, nodeID int64) (upload, download int64, err error)
	GetTotalByNodeInRange(ctx context.Context, nodeID int64, start, end time.Time) (upload, download int64, err error)
	GetTotalByUserInRange(ctx context.Context, userID int64, start, end time.Time) (upload, download int64, err error)

	// Statistics
	GetStatsByNode(ctx context.Context, start, end time.Time) ([]*NodeTrafficStats, error)
	GetStatsByUser(ctx context.Context, nodeID int64, start, end time.Time, limit int) ([]*UserNodeTrafficStats, error)
	GetStatsByGroup(ctx context.Context, start, end time.Time) ([]*GroupTrafficStats, error)
	GetTotalTraffic(ctx context.Context, start, end time.Time) (upload, download int64, err error)

	// Cleanup
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
	DeleteByNodeID(ctx context.Context, nodeID int64) error
}

// nodeTrafficRepository implements NodeTrafficRepository.
type nodeTrafficRepository struct {
	db *gorm.DB
}

// NewNodeTrafficRepository creates a new node traffic repository.
func NewNodeTrafficRepository(db *gorm.DB) NodeTrafficRepository {
	return &nodeTrafficRepository{db: db}
}

// Create creates a new node traffic record.
func (r *nodeTrafficRepository) Create(ctx context.Context, traffic *NodeTraffic) error {
	result := r.db.WithContext(ctx).Create(traffic)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create node traffic", result.Error)
	}
	return nil
}

// CreateBatch creates multiple node traffic records.
func (r *nodeTrafficRepository) CreateBatch(ctx context.Context, traffic []*NodeTraffic) error {
	if len(traffic) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).Create(&traffic)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create node traffic batch", result.Error)
	}
	return nil
}

// GetByID retrieves a node traffic record by ID.
func (r *nodeTrafficRepository) GetByID(ctx context.Context, id int64) (*NodeTraffic, error) {
	var traffic NodeTraffic
	result := r.db.WithContext(ctx).First(&traffic, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("node traffic", id)
		}
		return nil, errors.NewDatabaseError("failed to get node traffic", result.Error)
	}
	return &traffic, nil
}

// Delete deletes a node traffic record by ID.
func (r *nodeTrafficRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&NodeTraffic{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete node traffic", result.Error)
	}
	return nil
}

// GetByNodeID retrieves traffic records for a node with pagination.
func (r *nodeTrafficRepository) GetByNodeID(ctx context.Context, nodeID int64, limit, offset int) ([]*NodeTraffic, error) {
	var traffic []*NodeTraffic
	query := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("recorded_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	result := query.Find(&traffic)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by node ID", result.Error)
	}
	return traffic, nil
}

// GetByUserID retrieves traffic records for a user with pagination.
func (r *nodeTrafficRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*NodeTraffic, error) {
	var traffic []*NodeTraffic
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("recorded_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	result := query.Find(&traffic)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by user ID", result.Error)
	}
	return traffic, nil
}

// GetByDateRange retrieves traffic records within a date range.
func (r *nodeTrafficRepository) GetByDateRange(ctx context.Context, start, end time.Time) ([]*NodeTraffic, error) {
	var traffic []*NodeTraffic
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	result := r.db.WithContext(ctx).
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), rangeArgs...).
		Order("recorded_at DESC").
		Find(&traffic)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by date range", result.Error)
	}
	return traffic, nil
}

// GetByNodeAndDateRange retrieves traffic records for a node within a date range.
func (r *nodeTrafficRepository) GetByNodeAndDateRange(ctx context.Context, nodeID int64, start, end time.Time) ([]*NodeTraffic, error) {
	var traffic []*NodeTraffic
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	result := r.db.WithContext(ctx).
		Where("node_id = ? AND "+BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), append([]any{nodeID}, rangeArgs...)...).
		Order("recorded_at DESC").
		Find(&traffic)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by node and date range", result.Error)
	}
	return traffic, nil
}

// GetByUserAndDateRange retrieves traffic records for a user within a date range.
func (r *nodeTrafficRepository) GetByUserAndDateRange(ctx context.Context, userID int64, start, end time.Time) ([]*NodeTraffic, error) {
	var traffic []*NodeTraffic
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND "+BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), append([]any{userID}, rangeArgs...)...).
		Order("recorded_at DESC").
		Find(&traffic)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get traffic by user and date range", result.Error)
	}
	return traffic, nil
}

// GetTotalByNode returns total upload and download for a node.
func (r *nodeTrafficRepository) GetTotalByNode(ctx context.Context, nodeID int64) (upload, download int64, err error) {
	type result struct {
		Upload   int64
		Download int64
	}
	var res result
	err = r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("node_id = ?", nodeID).
		Scan(&res).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic by node", err)
	}
	return res.Upload, res.Download, nil
}

// GetTotalByUser returns total upload and download for a user across all nodes.
func (r *nodeTrafficRepository) GetTotalByUser(ctx context.Context, userID int64) (upload, download int64, err error) {
	type result struct {
		Upload   int64
		Download int64
	}
	var res result
	err = r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("user_id = ?", userID).
		Scan(&res).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic by user", err)
	}
	return res.Upload, res.Download, nil
}

// GetTotalByUserOnNode returns total upload and download for a user on a specific node.
func (r *nodeTrafficRepository) GetTotalByUserOnNode(ctx context.Context, userID, nodeID int64) (upload, download int64, err error) {
	type result struct {
		Upload   int64
		Download int64
	}
	var res result
	err = r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("user_id = ? AND node_id = ?", userID, nodeID).
		Scan(&res).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic by user on node", err)
	}
	return res.Upload, res.Download, nil
}

// GetTotalByNodeInRange returns total upload and download for a node within a date range.
func (r *nodeTrafficRepository) GetTotalByNodeInRange(ctx context.Context, nodeID int64, start, end time.Time) (upload, download int64, err error) {
	type result struct {
		Upload   int64
		Download int64
	}
	var res result
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	err = r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("node_id = ? AND "+BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), append([]any{nodeID}, rangeArgs...)...).
		Scan(&res).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic by node in range", err)
	}
	return res.Upload, res.Download, nil
}

// GetTotalByUserInRange returns total upload and download for a user within a date range.
func (r *nodeTrafficRepository) GetTotalByUserInRange(ctx context.Context, userID int64, start, end time.Time) (upload, download int64, err error) {
	type result struct {
		Upload   int64
		Download int64
	}
	var res result
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	err = r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("user_id = ? AND "+BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), append([]any{userID}, rangeArgs...)...).
		Scan(&res).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic by user in range", err)
	}
	return res.Upload, res.Download, nil
}

// GetStatsByNode returns traffic statistics grouped by node.
func (r *nodeTrafficRepository) GetStatsByNode(ctx context.Context, start, end time.Time) ([]*NodeTrafficStats, error) {
	var stats []*NodeTrafficStats
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	err := r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("node_id, COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download, COALESCE(SUM(upload + download), 0) as total").
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), rangeArgs...).
		Group("node_id").
		Scan(&stats).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get stats by node", err)
	}
	return stats, nil
}

// GetStatsByUser returns traffic statistics for users on a specific node.
func (r *nodeTrafficRepository) GetStatsByUser(ctx context.Context, nodeID int64, start, end time.Time, limit int) ([]*UserNodeTrafficStats, error) {
	var stats []*UserNodeTrafficStats
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	query := r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("user_id, node_id, COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where("node_id = ? AND user_id > 0 AND "+BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), append([]any{nodeID}, rangeArgs...)...).
		Group("user_id, node_id").
		Order("(upload + download) DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Scan(&stats).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get stats by user", err)
	}
	return stats, nil
}

// GetStatsByGroup returns traffic statistics grouped by node group.
func (r *nodeTrafficRepository) GetStatsByGroup(ctx context.Context, start, end time.Time) ([]*GroupTrafficStats, error) {
	var stats []*GroupTrafficStats
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	err := r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("node_group_members.group_id, COALESCE(SUM(node_traffic.upload), 0) as upload, COALESCE(SUM(node_traffic.download), 0) as download").
		Joins("JOIN node_group_members ON node_group_members.node_id = node_traffic.node_id").
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "node_traffic.recorded_at"), rangeArgs...).
		Group("node_group_members.group_id").
		Scan(&stats).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get stats by group", err)
	}
	return stats, nil
}

// GetTotalTraffic returns total upload and download across all nodes within a date range.
func (r *nodeTrafficRepository) GetTotalTraffic(ctx context.Context, start, end time.Time) (upload, download int64, err error) {
	type result struct {
		Upload   int64
		Download int64
	}
	var res result
	rangeArgs := BuildTimeRangeArgs(r.db.Dialector.Name(), start, end)
	err = r.db.WithContext(ctx).
		Model(&NodeTraffic{}).
		Select("COALESCE(SUM(upload), 0) as upload, COALESCE(SUM(download), 0) as download").
		Where(BuildTimeRangeCondition(r.db.Dialector.Name(), "recorded_at"), rangeArgs...).
		Scan(&res).Error
	if err != nil {
		return 0, 0, errors.NewDatabaseError("failed to get total traffic", err)
	}
	return res.Upload, res.Download, nil
}

// DeleteOlderThan deletes traffic records older than the specified time.
func (r *nodeTrafficRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("recorded_at < ?", before).
		Delete(&NodeTraffic{})
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to delete old traffic records", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteByNodeID deletes all traffic records for a node.
func (r *nodeTrafficRepository) DeleteByNodeID(ctx context.Context, nodeID int64) error {
	result := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Delete(&NodeTraffic{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete traffic by node ID", result.Error)
	}
	return nil
}
