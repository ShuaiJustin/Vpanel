// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// Node represents a remote Xray node server in the database.
type Node struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	Name              string     `gorm:"size:128;not null"`
	Address           string     `gorm:"size:256;not null"` // IP or domain
	Port              int        `gorm:"default:18443"`     // Agent port
	PanelURL          string     `gorm:"size:256"`          // Panel server URL
	Token             string     `gorm:"size:64;uniqueIndex"`
	Status            string     `gorm:"size:32;default:offline;index"` // online, offline, unhealthy
	Tags              string     `gorm:"type:text"`                     // JSON array
	Region            string     `gorm:"size:64;index"`
	Weight            int        `gorm:"default:1"`
	MaxUsers          int        `gorm:"default:0"` // 0 = unlimited
	CurrentUsers      int        `gorm:"default:0"`
	Latency           int        `gorm:"default:0"` // milliseconds
	LastSeenAt        *time.Time `gorm:""`
	SyncStatus        string     `gorm:"size:32;default:pending"` // synced, pending, failed
	SyncedAt          *time.Time `gorm:""`
	InstallStatus     string     `gorm:"size:32;default:idle;index"` // idle, queued, running, success, failed
	InstallMessage    string     `gorm:"size:255"`
	InstallSteps      string     `gorm:"type:text"`
	InstallLogs       string     `gorm:"type:text"`
	InstallStartedAt  *time.Time `gorm:""`
	InstallFinishedAt *time.Time `gorm:""`
	InstallUpdatedAt  *time.Time `gorm:""`
	IPWhitelist       string     `gorm:"type:text"` // JSON array of allowed IPs

	// 流量统计
	TrafficUp      int64      `gorm:"default:0"` // 上传流量 (bytes)
	TrafficDown    int64      `gorm:"default:0"` // 下载流量 (bytes)
	TrafficTotal   int64      `gorm:"default:0"` // 总流量 (bytes)
	TrafficLimit   int64      `gorm:"default:0"` // 流量限制 (bytes), 0 = 无限制
	TrafficResetAt *time.Time `gorm:""`          // 流量重置时间

	// 负载信息
	CPUUsage    float64 `gorm:"default:0"` // CPU 使用率 (0-100)
	MemoryUsage float64 `gorm:"default:0"` // 内存使用率 (0-100)
	DiskUsage   float64 `gorm:"default:0"` // 磁盘使用率 (0-100)
	NetSpeed    int64   `gorm:"default:0"` // 当前网速 (bytes/s)

	// 速率限制
	SpeedLimit int64 `gorm:"default:0"` // 速率限制 (bytes/s), 0 = 无限制

	// 协议支持
	Protocols string `gorm:"type:text"` // JSON array: ["vless", "vmess", "trojan", "shadowsocks"]

	// TLS 配置
	TLSEnabled  bool   `gorm:"default:false"`
	TLSDomain   string `gorm:"size:256"`
	TLSCertPath string `gorm:"size:512"`
	TLSKeyPath  string `gorm:"size:512"`

	// 节点分组
	GroupID *int64 `gorm:"index"` // 节点组 ID

	// 排序和优先级
	Priority int `gorm:"default:0"` // 优先级，数字越大优先级越高
	Sort     int `gorm:"default:0"` // 排序序号

	// 告警配置
	AlertTrafficThreshold float64 `gorm:"default:80"` // 流量告警阈值 (%)
	AlertCPUThreshold     float64 `gorm:"default:80"` // CPU 告警阈值 (%)
	AlertMemoryThreshold  float64 `gorm:"default:80"` // 内存告警阈值 (%)

	// 备注和描述
	Description string `gorm:"type:text"` // 节点描述
	Remarks     string `gorm:"type:text"` // 管理员备注

	// 网络优化配置
	NetworkOptimizationSettings string `gorm:"type:text"` // JSON encoded per-node optimization settings

	// Xray 状态
	XrayRunning bool   `gorm:"default:false"` // Xray 是否运行
	XrayVersion string `gorm:"size:64"`       // Xray 版本

	// 证书关联
	CertificateID *int64 `gorm:"index"` // 关联的证书 ID

	// SSH 配置（用于证书部署和远程管理）
	SSHHost     string `gorm:"size:256"`             // SSH 主机地址（默认使用 Address）
	SSHPort     int    `gorm:"default:22"`           // SSH 端口
	SSHUser     string `gorm:"size:64;default:root"` // SSH 用户名
	SSHPassword string `gorm:"size:256"`             // SSH 密码（加密存储）
	SSHKeyPath  string `gorm:"size:512"`             // SSH 私钥路径

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName returns the table name for Node.
func (Node) TableName() string {
	return "nodes"
}

// NodeStatus constants
const (
	NodeStatusOnline    = "online"
	NodeStatusOffline   = "offline"
	NodeStatusUnhealthy = "unhealthy"
)

// NodeSyncStatus constants
const (
	NodeSyncStatusSynced  = "synced"
	NodeSyncStatusPending = "pending"
	NodeSyncStatusFailed  = "failed"
)

// NodeInstallStatus constants
const (
	NodeInstallStatusIdle    = "idle"
	NodeInstallStatusQueued  = "queued"
	NodeInstallStatusRunning = "running"
	NodeInstallStatusSuccess = "success"
	NodeInstallStatusFailed  = "failed"
)

// NodeFilter defines filter options for listing nodes.
type NodeFilter struct {
	Status  string
	Region  string
	Tags    []string
	GroupID *int64
	Limit   int
	Offset  int
}

// NodeRepository defines the interface for node data access.
type NodeRepository interface {
	// CRUD operations
	Create(ctx context.Context, node *Node) error
	GetByID(ctx context.Context, id int64) (*Node, error)
	Update(ctx context.Context, node *Node) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter *NodeFilter) ([]*Node, error)
	Count(ctx context.Context, filter *NodeFilter) (int64, error)

	// Token operations
	GetByToken(ctx context.Context, token string) (*Node, error)
	UpdateToken(ctx context.Context, id int64, token string) error

	// Status operations
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error
	UpdateMetrics(ctx context.Context, id int64, latency int, currentUsers int) error
	UpdateLoadMetrics(ctx context.Context, id int64, cpuUsage, memoryUsage, diskUsage float64) error
	UpdateXrayStatus(ctx context.Context, id int64, xrayRunning bool, xrayVersion string) error
	UpdateInstallStatus(ctx context.Context, id int64, status, message, steps, logs string, startedAt, finishedAt, updatedAt *time.Time) error

	// Sync operations
	UpdateSyncStatus(ctx context.Context, id int64, status string, syncedAt *time.Time) error
	GetPendingSync(ctx context.Context) ([]*Node, error)

	// Query operations
	GetByStatus(ctx context.Context, status string) ([]*Node, error)
	GetByRegion(ctx context.Context, region string) ([]*Node, error)
	GetHealthy(ctx context.Context) ([]*Node, error)
	GetOnline(ctx context.Context) ([]*Node, error)
	GetAvailable(ctx context.Context) ([]*Node, error) // online and not at capacity

	// Statistics
	CountByStatus(ctx context.Context) (map[string]int64, error)
	GetTotalUsers(ctx context.Context) (int64, error)

	// Transaction operations
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
	DeleteInTx(ctx context.Context, id int64) error
	GetAvailableInTx(ctx context.Context) ([]*Node, error)

	// Batch update for heartbeat (atomic)
	UpdateHeartbeatBatch(ctx context.Context, id int64, status string, lastSeen time.Time, latency int, currentUsers int, cpuUsage, memoryUsage, diskUsage float64, xrayRunning bool, xrayVersion string) error
}

// nodeRepository implements NodeRepository.
type nodeRepository struct {
	db *gorm.DB
}

// NewNodeRepository creates a new node repository.
func NewNodeRepository(db *gorm.DB) NodeRepository {
	return &nodeRepository{db: db}
}

// Create creates a new node.
func (r *nodeRepository) Create(ctx context.Context, node *Node) error {
	result := r.db.WithContext(ctx).Create(node)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create node", result.Error)
	}
	return nil
}

// GetByID retrieves a node by ID.
func (r *nodeRepository) GetByID(ctx context.Context, id int64) (*Node, error) {
	var node Node
	result := r.db.WithContext(ctx).First(&node, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("node", id)
		}
		return nil, errors.NewDatabaseError("failed to get node", result.Error)
	}
	return &node, nil
}

// Update updates a node.
func (r *nodeRepository) Update(ctx context.Context, node *Node) error {
	result := r.db.WithContext(ctx).Save(node)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node", result.Error)
	}
	return nil
}

// Delete deletes a node by ID.
func (r *nodeRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&Node{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete node", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// List retrieves nodes with filtering and pagination.
func (r *nodeRepository) List(ctx context.Context, filter *NodeFilter) ([]*Node, error) {
	var nodes []*Node
	query := r.db.WithContext(ctx).Model(&Node{})

	if filter != nil {
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.Region != "" {
			query = query.Where("region = ?", filter.Region)
		}
		if filter.GroupID != nil {
			query = query.Joins("JOIN node_group_members ON node_group_members.node_id = nodes.id").
				Where("node_group_members.group_id = ?", *filter.GroupID)
		}
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	result := query.Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to list nodes", result.Error)
	}
	return nodes, nil
}

// Count counts nodes with filtering.
func (r *nodeRepository) Count(ctx context.Context, filter *NodeFilter) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&Node{})

	if filter != nil {
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.Region != "" {
			query = query.Where("region = ?", filter.Region)
		}
		if filter.GroupID != nil {
			query = query.Joins("JOIN node_group_members ON node_group_members.node_id = nodes.id").
				Where("node_group_members.group_id = ?", *filter.GroupID)
		}
	}

	result := query.Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count nodes", result.Error)
	}
	return count, nil
}

// GetByToken retrieves a node by its authentication token.
func (r *nodeRepository) GetByToken(ctx context.Context, token string) (*Node, error) {
	var node Node
	result := r.db.WithContext(ctx).Where("token = ?", token).First(&node)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("node", token)
		}
		return nil, errors.NewDatabaseError("failed to get node by token", result.Error)
	}
	return &node, nil
}

// UpdateToken updates a node's authentication token.
func (r *nodeRepository) UpdateToken(ctx context.Context, id int64, token string) error {
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Update("token", token)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node token", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// UpdateStatus updates a node's status.
func (r *nodeRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node status", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// UpdateLastSeen updates a node's last seen timestamp.
func (r *nodeRepository) UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Update("last_seen_at", lastSeen)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node last seen", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// UpdateMetrics updates a node's metrics (latency and current users).
func (r *nodeRepository) UpdateMetrics(ctx context.Context, id int64, latency int, currentUsers int) error {
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Updates(map[string]interface{}{
		"latency":       latency,
		"current_users": currentUsers,
	})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node metrics", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// UpdateLoadMetrics updates a node's load metrics (CPU, memory, disk usage).
func (r *nodeRepository) UpdateLoadMetrics(ctx context.Context, id int64, cpuUsage, memoryUsage, diskUsage float64) error {
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Updates(map[string]interface{}{
		"cpu_usage":    cpuUsage,
		"memory_usage": memoryUsage,
		"disk_usage":   diskUsage,
	})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node load metrics", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// UpdateXrayStatus updates a node's Xray status and version.
func (r *nodeRepository) UpdateXrayStatus(ctx context.Context, id int64, xrayRunning bool, xrayVersion string) error {
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Updates(map[string]interface{}{
		"xray_running": xrayRunning,
		"xray_version": xrayVersion,
	})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node xray status", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// UpdateInstallStatus updates a node's install status, progress and logs.
func (r *nodeRepository) UpdateInstallStatus(ctx context.Context, id int64, status, message, steps, logs string, startedAt, finishedAt, updatedAt *time.Time) error {
	updates := map[string]interface{}{
		"install_status":      status,
		"install_message":     message,
		"install_steps":       steps,
		"install_logs":        logs,
		"install_started_at":  startedAt,
		"install_finished_at": finishedAt,
		"install_updated_at":  updatedAt,
	}
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node install status", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// UpdateSyncStatus updates a node's sync status.
func (r *nodeRepository) UpdateSyncStatus(ctx context.Context, id int64, status string, syncedAt *time.Time) error {
	updates := map[string]interface{}{
		"sync_status": status,
	}
	if syncedAt != nil {
		updates["synced_at"] = syncedAt
	}
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node sync status", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// GetPendingSync retrieves nodes with pending sync status.
func (r *nodeRepository) GetPendingSync(ctx context.Context) ([]*Node, error) {
	var nodes []*Node
	result := r.db.WithContext(ctx).Where("sync_status = ?", NodeSyncStatusPending).Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get pending sync nodes", result.Error)
	}
	return nodes, nil
}

// GetByStatus retrieves nodes by status.
func (r *nodeRepository) GetByStatus(ctx context.Context, status string) ([]*Node, error) {
	var nodes []*Node
	result := r.db.WithContext(ctx).Where("status = ?", status).Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get nodes by status", result.Error)
	}
	return nodes, nil
}

// GetByRegion retrieves nodes by region.
func (r *nodeRepository) GetByRegion(ctx context.Context, region string) ([]*Node, error) {
	var nodes []*Node
	result := r.db.WithContext(ctx).Where("region = ?", region).Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get nodes by region", result.Error)
	}
	return nodes, nil
}

// GetHealthy retrieves all healthy nodes (online status).
func (r *nodeRepository) GetHealthy(ctx context.Context) ([]*Node, error) {
	var nodes []*Node
	result := r.db.WithContext(ctx).Where("status = ?", NodeStatusOnline).Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get healthy nodes", result.Error)
	}
	return nodes, nil
}

// GetOnline retrieves all online nodes.
func (r *nodeRepository) GetOnline(ctx context.Context) ([]*Node, error) {
	var nodes []*Node
	result := r.db.WithContext(ctx).Where("status = ?", NodeStatusOnline).Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get online nodes", result.Error)
	}
	return nodes, nil
}

// GetAvailable retrieves nodes that are online and not at capacity.
func (r *nodeRepository) GetAvailable(ctx context.Context) ([]*Node, error) {
	var nodes []*Node
	result := r.db.WithContext(ctx).
		Where("status = ?", NodeStatusOnline).
		Where("max_users = 0 OR current_users < max_users").
		Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get available nodes", result.Error)
	}
	return nodes, nil
}

// CountByStatus returns node counts grouped by status.
func (r *nodeRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	var results []statusCount
	err := r.db.WithContext(ctx).Model(&Node{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&results).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to count nodes by status", err)
	}

	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

// GetTotalUsers returns the total number of users across all nodes.
func (r *nodeRepository) GetTotalUsers(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&Node{}).
		Select("COALESCE(SUM(current_users), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, errors.NewDatabaseError("failed to get total users", err)
	}
	return total, nil
}

// Transaction executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
func (r *nodeRepository) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create a new context with the transaction
		txCtx := context.WithValue(ctx, "tx", tx)
		return fn(txCtx)
	})
}

// DeleteInTx deletes a node within a transaction.
func (r *nodeRepository) DeleteInTx(ctx context.Context, id int64) error {
	db := r.getDB(ctx)
	result := db.Delete(&Node{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete node in transaction", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}

// GetAvailableInTx retrieves nodes that are online and not at capacity within a transaction.
func (r *nodeRepository) GetAvailableInTx(ctx context.Context) ([]*Node, error) {
	db := r.getDB(ctx)
	var nodes []*Node
	result := db.
		Where("status = ?", NodeStatusOnline).
		Where("max_users = 0 OR current_users < max_users").
		Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get available nodes in transaction", result.Error)
	}
	return nodes, nil
}

// getDB returns the appropriate database connection (transaction or regular).
func (r *nodeRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value("tx").(*gorm.DB); ok {
		return tx
	}
	return r.db.WithContext(ctx)
}

// UpdateHeartbeatBatch atomically updates all heartbeat-related fields in a single query.
func (r *nodeRepository) UpdateHeartbeatBatch(ctx context.Context, id int64, status string, lastSeen time.Time, latency int, currentUsers int, cpuUsage, memoryUsage, diskUsage float64, xrayRunning bool, xrayVersion string) error {
	updates := map[string]interface{}{
		"status":        status,
		"last_seen_at":  lastSeen,
		"latency":       latency,
		"current_users": currentUsers,
		"cpu_usage":     cpuUsage,
		"memory_usage":  memoryUsage,
		"disk_usage":    diskUsage,
		"xray_running":  xrayRunning,
		"xray_version":  xrayVersion,
	}
	result := r.db.WithContext(ctx).Model(&Node{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update heartbeat batch", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node", id)
	}
	return nil
}
