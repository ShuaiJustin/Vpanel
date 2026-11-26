// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Coupon represents a coupon in the database.
type Coupon struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	Code           string    `gorm:"uniqueIndex;size:32;not null"`
	Name           string    `gorm:"size:128;not null"`
	Type           string    `gorm:"size:16;not null"`
	Value          int64     `gorm:"not null"`
	MinOrderAmount int64     `gorm:"default:0"`
	MaxDiscount    int64     `gorm:"default:0"`
	TotalLimit     int       `gorm:"default:0"`
	PerUserLimit   int       `gorm:"default:1"`
	UsedCount      int       `gorm:"default:0"`
	PlanIDs        string    `gorm:"type:text"`
	StartAt        time.Time `gorm:"not null"`
	ExpireAt       time.Time `gorm:"not null;index"`
	IsActive       bool      `gorm:"default:true;index"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

// TableName returns the table name for Coupon.
func (Coupon) TableName() string {
	return "coupons"
}

// Coupon type constants
const (
	CouponTypeFixed      = "fixed"
	CouponTypePercentage = "percentage"
)

// CouponUsage represents a coupon usage record in the database.
type CouponUsage struct {
	ID       int64     `gorm:"primaryKey;autoIncrement"`
	CouponID int64     `gorm:"index;not null"`
	UserID   int64     `gorm:"index;not null"`
	OrderID  int64     `gorm:"index;not null"`
	Discount int64     `gorm:"not null"`
	UsedAt   time.Time `gorm:"not null"`

	Coupon *Coupon `gorm:"foreignKey:CouponID"`
	User   *User   `gorm:"foreignKey:UserID"`
	Order  *Order  `gorm:"foreignKey:OrderID"`
}

// TableName returns the table name for CouponUsage.
func (CouponUsage) TableName() string {
	return "coupon_usages"
}

// CouponFilter defines filter options for listing coupons.
type CouponFilter struct {
	IsActive  *bool
	Type      string
	StartDate *time.Time
	EndDate   *time.Time
}

// CouponRepository defines the interface for coupon data access.
type CouponRepository interface {
	// Coupon operations
	Create(ctx context.Context, coupon *Coupon) error
	GetByID(ctx context.Context, id int64) (*Coupon, error)
	GetByCode(ctx context.Context, code string) (*Coupon, error)
	Update(ctx context.Context, coupon *Coupon) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter CouponFilter, limit, offset int) ([]*Coupon, int64, error)
	IncrementUsedCount(ctx context.Context, id int64) error
	SetActive(ctx context.Context, id int64, active bool) error

	// Usage operations
	CreateUsage(ctx context.Context, usage *CouponUsage) error
	GetUsageCount(ctx context.Context, couponID int64) (int, error)
	GetUserUsageCount(ctx context.Context, couponID, userID int64) (int, error)
	ListUsages(ctx context.Context, couponID int64, limit, offset int) ([]*CouponUsage, int64, error)

	// Statistics
	Count(ctx context.Context) (int64, error)
	CountActive(ctx context.Context) (int64, error)
	GetTotalDiscountAmount(ctx context.Context, couponID int64) (int64, error)
}

// couponRepository implements CouponRepository.
type couponRepository struct {
	db *gorm.DB
}

// NewCouponRepository creates a new coupon repository.
func NewCouponRepository(db *gorm.DB) CouponRepository {
	return &couponRepository{db: db}
}

// Create creates a new coupon.
func (r *couponRepository) Create(ctx context.Context, coupon *Coupon) error {
	active, perUserLimit := coupon.IsActive, coupon.PerUserLimit
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(coupon).Error; err != nil {
			return err
		}
		// GORM fills zero values from the schema defaults on create.
		if coupon.IsActive == active && coupon.PerUserLimit == perUserLimit {
			return nil
		}
		coupon.IsActive, coupon.PerUserLimit = active, perUserLimit
		return tx.Model(coupon).UpdateColumns(map[string]any{
			"is_active": active, "per_user_limit": perUserLimit,
		}).Error
	})
}

// GetByID retrieves a coupon by ID.
func (r *couponRepository) GetByID(ctx context.Context, id int64) (*Coupon, error) {
	var coupon Coupon
	err := r.db.WithContext(ctx).First(&coupon, id).Error
	if err != nil {
		return nil, err
	}
	return &coupon, nil
}

// GetByCode retrieves a coupon by code.
func (r *couponRepository) GetByCode(ctx context.Context, code string) (*Coupon, error) {
	var coupon Coupon
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&coupon).Error
	if err != nil {
		return nil, err
	}
	return &coupon, nil
}

// Update updates a coupon.
func (r *couponRepository) Update(ctx context.Context, coupon *Coupon) error {
	return r.db.WithContext(ctx).Model(coupon).Select("*").Omit("used_count").Updates(coupon).Error
}

// Delete deletes a coupon by ID.
func (r *couponRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Coupon{}, id).Error
}

// List lists coupons with filter and pagination.
func (r *couponRepository) List(ctx context.Context, filter CouponFilter, limit, offset int) ([]*Coupon, int64, error) {
	var coupons []*Coupon
	var total int64

	query := r.db.WithContext(ctx).Model(&Coupon{})

	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.StartDate != nil {
		query = query.Where("start_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("expire_at <= ?", *filter.EndDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&coupons).Error
	return coupons, total, err
}

// IncrementUsedCount increments the used count of a coupon.
func (r *couponRepository) IncrementUsedCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&Coupon{}).
		Where("id = ?", id).
		Update("used_count", gorm.Expr("used_count + 1")).Error
}

// SetActive sets the active status of a coupon.
func (r *couponRepository) SetActive(ctx context.Context, id int64, active bool) error {
	return r.db.WithContext(ctx).
		Model(&Coupon{}).
		Where("id = ?", id).
		Update("is_active", active).Error
}

// CreateUsage creates a new coupon usage record.
func (r *couponRepository) CreateUsage(ctx context.Context, usage *CouponUsage) error {
	return r.db.WithContext(ctx).Create(usage).Error
}

// GetUsageCount returns the total usage count for a coupon.
func (r *couponRepository) GetUsageCount(ctx context.Context, couponID int64) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&CouponUsage{}).Where("coupon_id = ?", couponID).Count(&count).Error
	return int(count), err
}

// GetUserUsageCount returns the usage count for a specific user and coupon.
func (r *couponRepository) GetUserUsageCount(ctx context.Context, couponID, userID int64) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&CouponUsage{}).
		Where("coupon_id = ? AND user_id = ?", couponID, userID).
		Count(&count).Error
	return int(count), err
}

// ListUsages lists usage records for a coupon.
func (r *couponRepository) ListUsages(ctx context.Context, couponID int64, limit, offset int) ([]*CouponUsage, int64, error) {
	var usages []*CouponUsage
	var total int64

	query := r.db.WithContext(ctx).Model(&CouponUsage{}).Where("coupon_id = ?", couponID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("User").Preload("Order").
		Order("used_at DESC").Limit(limit).Offset(offset).Find(&usages).Error
	return usages, total, err
}

// Count returns the total number of coupons.
func (r *couponRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Coupon{}).Count(&count).Error
	return count, err
}

// CountActive returns the number of active coupons.
func (r *couponRepository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	now := time.Now()
	err := r.db.WithContext(ctx).Model(&Coupon{}).
		Where("is_active = ? AND start_at <= ? AND expire_at >= ?", true, now, now).
		Count(&count).Error
	return count, err
}

// GetTotalDiscountAmount returns the total discount amount for a coupon.
func (r *couponRepository) GetTotalDiscountAmount(ctx context.Context, couponID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&CouponUsage{}).
		Where("coupon_id = ?", couponID).
		Select("COALESCE(SUM(discount), 0)").Scan(&total).Error
	return total, err
}
