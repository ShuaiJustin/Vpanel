// Package repository provides data access implementations.
package repository

import (
	"context"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// proxyRepository implements ProxyRepository.
type proxyRepository struct {
	db *gorm.DB
}

// NewProxyRepository creates a new proxy repository.
func NewProxyRepository(db *gorm.DB) ProxyRepository {
	return &proxyRepository{db: db}
}

func (r *proxyRepository) validProxyQuery(ctx context.Context) *gorm.DB {
	query := r.db.WithContext(ctx).Model(&Proxy{})
	if !r.db.Migrator().HasTable((&User{}).TableName()) {
		return query
	}
	return query.Joins("JOIN users ON users.id = proxies.user_id")
}

// Create creates a new proxy.
func (r *proxyRepository) Create(ctx context.Context, proxy *Proxy) error {
	result := r.db.WithContext(ctx).Create(proxy)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create proxy", result.Error)
	}
	return nil
}

// GetByID retrieves a proxy by ID.
func (r *proxyRepository) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	var proxy Proxy
	result := r.db.WithContext(ctx).First(&proxy, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("proxy", id)
		}
		return nil, errors.NewDatabaseError("failed to get proxy", result.Error)
	}
	return &proxy, nil
}

// Update updates a proxy.
func (r *proxyRepository) Update(ctx context.Context, proxy *Proxy) error {
	result := r.db.WithContext(ctx).Save(proxy)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update proxy", result.Error)
	}
	return nil
}

// Delete deletes a proxy by ID.
func (r *proxyRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&Proxy{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete proxy", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("proxy", id)
	}
	return nil
}

// List retrieves proxies with pagination.
func (r *proxyRepository) List(ctx context.Context, limit, offset int) ([]*Proxy, error) {
	var proxies []*Proxy
	result := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&proxies)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to list proxies", result.Error)
	}
	return proxies, nil
}

// GetByProtocol retrieves proxies by protocol.
func (r *proxyRepository) GetByProtocol(ctx context.Context, protocol string) ([]*Proxy, error) {
	var proxies []*Proxy
	result := r.db.WithContext(ctx).Where("protocol = ?", protocol).Find(&proxies)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get proxies by protocol", result.Error)
	}
	return proxies, nil
}

// GetEnabled retrieves all enabled proxies.
func (r *proxyRepository) GetEnabled(ctx context.Context) ([]*Proxy, error) {
	var proxies []*Proxy
	result := r.validProxyQuery(ctx).Where("proxies.enabled = ?", true).Find(&proxies)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get enabled proxies", result.Error)
	}
	return proxies, nil
}

// GetByUserID retrieves proxies by user ID with pagination.
func (r *proxyRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*Proxy, error) {
	var proxies []*Proxy
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Limit(limit).Offset(offset).Find(&proxies)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get proxies by user ID", result.Error)
	}
	return proxies, nil
}

// CountByUserID counts proxies by user ID.
func (r *proxyRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&Proxy{}).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count proxies by user ID", result.Error)
	}
	return count, nil
}

// GetByPort retrieves a proxy by port.
func (r *proxyRepository) GetByPort(ctx context.Context, port int) (*Proxy, error) {
	var proxy Proxy
	result := r.validProxyQuery(ctx).Where("proxies.port = ?", port).First(&proxy)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // No conflict
		}
		return nil, errors.NewDatabaseError("failed to get proxy by port", result.Error)
	}
	return &proxy, nil
}

// EnableByUserID enables all proxies for a user.
func (r *proxyRepository) EnableByUserID(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).Model(&Proxy{}).Where("user_id = ?", userID).Update("enabled", true)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to enable proxies by user ID", result.Error)
	}
	return nil
}

// DisableByUserID disables all proxies for a user.
func (r *proxyRepository) DisableByUserID(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).Model(&Proxy{}).Where("user_id = ?", userID).Update("enabled", false)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to disable proxies by user ID", result.Error)
	}
	return nil
}

// DeleteByIDs deletes proxies by IDs.
func (r *proxyRepository) DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).Delete(&Proxy{}, ids)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete proxies by IDs", result.Error)
	}
	return nil
}

// Count returns the total number of proxies.
func (r *proxyRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&Proxy{}).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count proxies", result.Error)
	}
	return count, nil
}

// CountEnabled returns the number of enabled proxies.
func (r *proxyRepository) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&Proxy{}).Where("enabled = ?", true).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count enabled proxies", result.Error)
	}
	return count, nil
}

// CountByProtocol returns proxy counts grouped by protocol.
func (r *proxyRepository) CountByProtocol(ctx context.Context) ([]*ProtocolCount, error) {
	var results []*ProtocolCount
	err := r.db.WithContext(ctx).Model(&Proxy{}).
		Select("protocol, COUNT(*) as count").
		Group("protocol").
		Scan(&results).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to count proxies by protocol", err)
	}
	return results, nil
}

// GetByNodeID retrieves all enabled proxies for a specific node.
func (r *proxyRepository) GetByNodeID(ctx context.Context, nodeID int64) ([]*Proxy, error) {
	var proxies []*Proxy

	result := r.validProxyQuery(ctx).
		Where("proxies.node_id = ? AND proxies.enabled = ?", nodeID, true).
		Find(&proxies)

	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get proxies by node ID", result.Error)
	}

	return proxies, nil
}
