// Package repository provides data access implementations.
package repository

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// Subscription represents a user's subscription record in the database.
type Subscription struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	UserID       int64      `gorm:"uniqueIndex;not null"`
	Token        string     `gorm:"uniqueIndex;size:64;not null"`
	ShortCode    string     `gorm:"uniqueIndex;size:16"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	LastAccessAt *time.Time `gorm:""`
	AccessCount  int64      `gorm:"default:0"`
	LastIP       string     `gorm:"size:45"`
	LastUA       string     `gorm:"size:256"`

	// Relations
	User *User `gorm:"foreignKey:UserID"`
}

// TableName returns the table name for Subscription.
func (Subscription) TableName() string {
	return "subscriptions"
}

// SubscriptionFilter represents filter options for listing subscriptions.
type SubscriptionFilter struct {
	UserID           *int64
	MinAccessCount   *int64
	MaxAccessCount   *int64
	LastAccessAfter  *time.Time
	LastAccessBefore *time.Time
	Keyword          string
	Activity         string
	Sort             string
	Limit            int
	Offset           int
}

// SubscriptionRepository defines the interface for subscription data access.
type SubscriptionRepository interface {
	// Create creates a new subscription record.
	Create(ctx context.Context, subscription *Subscription) error

	// GetByID retrieves a subscription by its ID.
	GetByID(ctx context.Context, id int64) (*Subscription, error)

	// GetByToken retrieves a subscription by its token.
	GetByToken(ctx context.Context, token string) (*Subscription, error)

	// GetByShortCode retrieves a subscription by its short code.
	GetByShortCode(ctx context.Context, shortCode string) (*Subscription, error)

	// GetByUserID retrieves a subscription by user ID.
	GetByUserID(ctx context.Context, userID int64) (*Subscription, error)

	// Update updates an existing subscription.
	Update(ctx context.Context, subscription *Subscription) error

	// Delete deletes a subscription by ID.
	Delete(ctx context.Context, id int64) error

	// DeleteByUserID deletes a subscription by user ID.
	DeleteByUserID(ctx context.Context, userID int64) error

	// UpdateAccessStats updates the access statistics for a subscription.
	UpdateAccessStats(ctx context.Context, id int64, ip string, userAgent string) error

	// ListAll retrieves all subscriptions with optional filtering.
	ListAll(ctx context.Context, filter *SubscriptionFilter) ([]*Subscription, int64, error)

	// ResetAccessStats resets the access statistics for a subscription.
	ResetAccessStats(ctx context.Context, id int64) error
}

// subscriptionRepository implements SubscriptionRepository.
type subscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository creates a new subscription repository.
func NewSubscriptionRepository(db *gorm.DB) SubscriptionRepository {
	return &subscriptionRepository{db: db}
}

// Create creates a new subscription record.
func (r *subscriptionRepository) Create(ctx context.Context, subscription *Subscription) error {
	result := r.db.WithContext(ctx).Create(subscription)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create subscription", result.Error)
	}
	return nil
}

// GetByID retrieves a subscription by its ID.
func (r *subscriptionRepository) GetByID(ctx context.Context, id int64) (*Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).First(&subscription, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("subscription", id)
		}
		return nil, errors.NewDatabaseError("failed to get subscription", result.Error)
	}
	return &subscription, nil
}

// GetByToken retrieves a subscription by its token.
func (r *subscriptionRepository) GetByToken(ctx context.Context, token string) (*Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).Where("token = ?", token).First(&subscription)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("subscription", token)
		}
		return nil, errors.NewDatabaseError("failed to get subscription by token", result.Error)
	}
	return &subscription, nil
}

// GetByShortCode retrieves a subscription by its short code.
func (r *subscriptionRepository) GetByShortCode(ctx context.Context, shortCode string) (*Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).Where("short_code = ?", shortCode).First(&subscription)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("subscription", shortCode)
		}
		return nil, errors.NewDatabaseError("failed to get subscription by short code", result.Error)
	}
	return &subscription, nil
}

// GetByUserID retrieves a subscription by user ID.
func (r *subscriptionRepository) GetByUserID(ctx context.Context, userID int64) (*Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&subscription)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("subscription", userID)
		}
		return nil, errors.NewDatabaseError("failed to get subscription by user ID", result.Error)
	}
	return &subscription, nil
}

// Update updates an existing subscription.
func (r *subscriptionRepository) Update(ctx context.Context, subscription *Subscription) error {
	result := r.db.WithContext(ctx).Save(subscription)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update subscription", result.Error)
	}
	return nil
}

// Delete deletes a subscription by ID.
func (r *subscriptionRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&Subscription{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete subscription", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("subscription", id)
	}
	return nil
}

// DeleteByUserID deletes a subscription by user ID.
func (r *subscriptionRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&Subscription{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete subscription by user ID", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("subscription", userID)
	}
	return nil
}

// UpdateAccessStats updates the access statistics for a subscription.
func (r *subscriptionRepository) UpdateAccessStats(ctx context.Context, id int64, ip string, userAgent string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&Subscription{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_count":   gorm.Expr("access_count + 1"),
			"last_access_at": now,
			"last_ip":        ip,
			"last_ua":        userAgent,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update access stats", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("subscription", id)
	}
	return nil
}

// ListAll retrieves all subscriptions with optional filtering.
func (r *subscriptionRepository) ListAll(ctx context.Context, filter *SubscriptionFilter) ([]*Subscription, int64, error) {
	var subscriptions []*Subscription
	var total int64

	query := r.db.WithContext(ctx).Model(&Subscription{})

	// Apply filters
	if filter != nil {
		if filter.UserID != nil {
			query = query.Where("user_id = ?", *filter.UserID)
		}
		if filter.MinAccessCount != nil {
			query = query.Where("access_count >= ?", *filter.MinAccessCount)
		}
		if filter.MaxAccessCount != nil {
			query = query.Where("access_count <= ?", *filter.MaxAccessCount)
		}
		if filter.LastAccessAfter != nil {
			query = query.Where("last_access_at >= ?", *filter.LastAccessAfter)
		}
		if filter.LastAccessBefore != nil {
			query = query.Where("last_access_at <= ?", *filter.LastAccessBefore)
		}
		if filter.Activity != "" {
			switch filter.Activity {
			case "never":
				query = query.Where("access_count = 0")
			case "recent":
				query = query.Where("access_count > 0").Where("last_access_at >= ?", time.Now().Add(-7*24*time.Hour))
			case "stale":
				query = query.Where("access_count > 0").Where("last_access_at IS NULL OR last_access_at < ?", time.Now().Add(-30*24*time.Hour))
			}
		}
		if keyword := strings.ToLower(strings.TrimSpace(filter.Keyword)); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Joins("LEFT JOIN users ON users.id = subscriptions.user_id").
				Where(
					"LOWER(subscriptions.token) LIKE ? OR LOWER(COALESCE(subscriptions.short_code, '')) LIKE ? OR LOWER(COALESCE(subscriptions.last_ip, '')) LIKE ? OR LOWER(COALESCE(users.username, '')) LIKE ?",
					like, like, like, like,
				)

			if numericID, err := strconv.ParseInt(keyword, 10, 64); err == nil {
				query = query.Or("subscriptions.user_id = ? OR subscriptions.id = ?", numericID, numericID)
			}
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count subscriptions", err)
	}

	if filter != nil {
		switch filter.Sort {
		case "access_desc":
			query = query.Order("access_count DESC").Order("last_access_at DESC").Order("created_at DESC")
		case "created_desc":
			query = query.Order("created_at DESC")
		case "user_desc":
			query = query.Order("user_id DESC")
		default:
			query = query.Order("CASE WHEN last_access_at IS NULL THEN 1 ELSE 0 END").Order("last_access_at DESC").Order("created_at DESC")
		}
	}

	// Apply pagination
	if filter != nil {
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	// Preload User relation and fetch results
	if err := query.Preload("User").Find(&subscriptions).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to list subscriptions", err)
	}

	return subscriptions, total, nil
}

// ResetAccessStats resets the access statistics for a subscription.
func (r *subscriptionRepository) ResetAccessStats(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Model(&Subscription{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_count":   0,
			"last_access_at": nil,
			"last_ip":        "",
			"last_ua":        "",
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to reset access stats", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("subscription", id)
	}
	return nil
}
