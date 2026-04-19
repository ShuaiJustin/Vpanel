// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Order represents an order in the database.
type Order struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	OrderNo        string `gorm:"uniqueIndex;size:64;not null"`
	UserID         int64  `gorm:"index;not null"`
	PlanID         int64  `gorm:"index;not null"`
	CouponID       *int64 `gorm:"index"`
	OriginalAmount int64  `gorm:"not null"`
	DiscountAmount int64  `gorm:"default:0"`
	BalanceUsed    int64  `gorm:"default:0"`
	PayAmount      int64  `gorm:"not null"`
	Status         string `gorm:"size:32;default:pending;index"`
	PaymentMethod  string `gorm:"size:32"`
	PaymentNo      string `gorm:"size:128;index"`
	PaidAt         *time.Time
	ExpiredAt      time.Time `gorm:"index;not null"`
	Notes          string    `gorm:"type:text"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`

	User   *User           `gorm:"foreignKey:UserID"`
	Plan   *CommercialPlan `gorm:"foreignKey:PlanID"`
	Coupon *Coupon         `gorm:"foreignKey:CouponID"`
}

// TableName returns the table name for Order.
func (Order) TableName() string {
	return "orders"
}

// Order status constants
const (
	OrderStatusPending   = "pending"
	OrderStatusPaid      = "paid"
	OrderStatusCompleted = "completed"
	OrderStatusCancelled = "cancelled"
	OrderStatusRefunded  = "refunded"
)

// OrderFilter defines filter options for listing orders.
type OrderFilter struct {
	UserID        *int64
	Search        string
	Status        string
	PaymentMethod string
	StartDate     *time.Time
	EndDate       *time.Time
	MinAmount     *int64
	MaxAmount     *int64
}

// OrderRepository defines the interface for order data access.
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id int64) (*Order, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*Order, error)
	GetByPaymentNo(ctx context.Context, paymentNo string) (*Order, error)
	Update(ctx context.Context, order *Order) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter OrderFilter, limit, offset int) ([]*Order, int64, error)
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Order, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
	MarkPaid(ctx context.Context, id int64, paymentNo string, paidAt time.Time) error
	GetExpiredPending(ctx context.Context) ([]*Order, error)
	CancelExpired(ctx context.Context) (int64, error)
	Count(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	GetRevenueByDateRange(ctx context.Context, start, end time.Time) (int64, error)
	GetOrderCountByDateRange(ctx context.Context, start, end time.Time) (int64, error)
}

// orderRepository implements OrderRepository.
type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository creates a new order repository.
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

// Create creates a new order.
func (r *orderRepository) Create(ctx context.Context, order *Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// GetByID retrieves an order by ID.
func (r *orderRepository) GetByID(ctx context.Context, id int64) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).Preload("User").Preload("Plan").Preload("Coupon").First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetByOrderNo retrieves an order by order number.
func (r *orderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).Preload("User").Preload("Plan").Preload("Coupon").
		Where("order_no = ?", orderNo).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetByPaymentNo retrieves an order by payment number.
func (r *orderRepository) GetByPaymentNo(ctx context.Context, paymentNo string) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).Preload("User").Preload("Plan").
		Where("payment_no = ?", paymentNo).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Update updates an order.
func (r *orderRepository) Update(ctx context.Context, order *Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

// Delete deletes an order by ID.
func (r *orderRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Order{}, id).Error
}

// List lists orders with filter and pagination.
func (r *orderRepository) List(ctx context.Context, filter OrderFilter, limit, offset int) ([]*Order, int64, error) {
	var orders []*Order
	var total int64

	query := r.db.WithContext(ctx).Model(&Order{})

	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if searchQuery := strings.TrimSpace(filter.Search); searchQuery != "" {
		if numericID, err := strconv.ParseInt(searchQuery, 10, 64); err == nil {
			// Numeric search should match exact identifiers instead of fuzzy order numbers.
			query = query.Where("(user_id = ? OR id = ?)", numericID, numericID)
		} else {
			searchLike := "%" + searchQuery + "%"
			query = query.Where("(order_no LIKE ? OR payment_no LIKE ?)", searchLike, searchLike)
		}
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.PaymentMethod != "" {
		query = query.Where("payment_method = ?", filter.PaymentMethod)
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", *filter.EndDate)
	}
	if filter.MinAmount != nil {
		query = query.Where("pay_amount >= ?", *filter.MinAmount)
	}
	if filter.MaxAmount != nil {
		query = query.Where("pay_amount <= ?", *filter.MaxAmount)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("User").Preload("Plan").
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&orders).Error
	return orders, total, err
}

// ListByUser lists orders for a specific user.
func (r *orderRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Order, int64, error) {
	var orders []*Order
	var total int64

	query := r.db.WithContext(ctx).Model(&Order{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Plan").
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&orders).Error
	return orders, total, err
}

// UpdateStatus updates the status of an order.
func (r *orderRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).
		Model(&Order{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// MarkPaid marks an order as paid.
// Uses a status guard so concurrent payment callbacks cannot both flip the
// order to "paid" and trigger the post-payment side effects twice. Returns
// gorm.ErrRecordNotFound if the order is no longer pending (already paid,
// cancelled, or expired), letting callers skip the duplicate side effects.
func (r *orderRepository) MarkPaid(ctx context.Context, id int64, paymentNo string, paidAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&Order{}).
		Where("id = ? AND status = ?", id, OrderStatusPending).
		Updates(map[string]interface{}{
			"status":     OrderStatusPaid,
			"payment_no": paymentNo,
			"paid_at":    paidAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetExpiredPending retrieves all expired pending orders.
func (r *orderRepository) GetExpiredPending(ctx context.Context) ([]*Order, error) {
	var orders []*Order
	err := r.db.WithContext(ctx).
		Where("status = ? AND expired_at < ?", OrderStatusPending, time.Now()).
		Find(&orders).Error
	return orders, err
}

// CancelExpired cancels all expired pending orders.
func (r *orderRepository) CancelExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&Order{}).
		Where("status = ? AND expired_at < ?", OrderStatusPending, time.Now()).
		Update("status", OrderStatusCancelled)
	return result.RowsAffected, result.Error
}

// Count returns the total number of orders.
func (r *orderRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Order{}).Count(&count).Error
	return count, err
}

// CountByStatus returns the number of orders with a specific status.
func (r *orderRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Order{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// GetRevenueByDateRange returns total revenue for a date range.
func (r *orderRepository) GetRevenueByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&Order{}).
		Where("status IN (?, ?) AND paid_at >= ? AND paid_at <= ?",
			OrderStatusPaid, OrderStatusCompleted, start, end).
		Select("COALESCE(SUM(pay_amount), 0)").Scan(&total).Error
	return total, err
}

// GetOrderCountByDateRange returns order count for a date range.
func (r *orderRepository) GetOrderCountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Order{}).
		Where("status IN (?, ?) AND paid_at >= ? AND paid_at <= ?",
			OrderStatusPaid, OrderStatusCompleted, start, end).
		Count(&count).Error
	return count, err
}
