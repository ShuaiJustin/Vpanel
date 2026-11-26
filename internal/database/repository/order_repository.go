// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Order represents an order in the database.
type Order struct {
	ID                 int64  `gorm:"primaryKey;autoIncrement"`
	OrderNo            string `gorm:"uniqueIndex;size:64;not null"`
	UserID             int64  `gorm:"index;not null"`
	PlanID             int64  `gorm:"index;not null"`
	PlanSnapshot       string `gorm:"type:text"`
	CouponID           *int64 `gorm:"index"`
	OriginalAmount     int64  `gorm:"not null"`
	DiscountAmount     int64  `gorm:"default:0"`
	BalanceUsed        int64  `gorm:"default:0"`
	PayAmount          int64  `gorm:"not null"`
	Status             string `gorm:"size:32;default:pending;index"`
	PaymentMethod      string `gorm:"size:32"`
	PaymentNo          string `gorm:"size:128;index"`
	PaidAt             *time.Time
	FulfilledAt        *time.Time
	EntitlementStartAt *time.Time
	EntitlementEndAt   *time.Time
	FulfillmentPending bool `gorm:"default:false;index"`
	PaymentCheckedAt   *time.Time
	RefundedAmount     int64     `gorm:"default:0"`
	ExpiredAt          time.Time `gorm:"index;not null"`
	Notes              string    `gorm:"type:text"`
	CreatedAt          time.Time `gorm:"autoCreateTime"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime"`

	User   *User           `gorm:"foreignKey:UserID"`
	Plan   *CommercialPlan `gorm:"foreignKey:PlanID"`
	Coupon *Coupon         `gorm:"foreignKey:CouponID"`
}

// TableName returns the table name for Order.
func (Order) TableName() string {
	return "orders"
}

// PurchasedPlan returns immutable terms for new orders. Legacy orders have no
// snapshot until their plan is edited, and retain the existing catalog fallback.
func (o *Order) PurchasedPlan() (*CommercialPlan, error) {
	if o.PlanSnapshot == "" {
		return nil, nil
	}
	var plan CommercialPlan
	if err := json.Unmarshal([]byte(o.PlanSnapshot), &plan); err != nil {
		return nil, err
	}
	if plan.ID != o.PlanID || plan.Duration <= 0 || plan.TrafficLimit < 0 || plan.Price < 0 {
		return nil, gorm.ErrInvalidData
	}
	return &plan, nil
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
	UpdatePendingPaymentMethod(ctx context.Context, id int64, method string) error
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

type CouponOrderRepository interface {
	CreateWithCoupon(context.Context, *Order, string, func(CouponRepository, *Coupon) (int64, error)) error
	ReleaseExpiredCouponOrders(context.Context) error
}

// NewOrderRepository creates a new order repository.
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

// Create creates a new order.
func (r *orderRepository) Create(ctx context.Context, order *Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *orderRepository) CreateWithCoupon(ctx context.Context, order *Order, code string, validate func(CouponRepository, *Coupon) (int64, error)) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Acquire the coupon write lock before reading usage. The increment
		// rolls back if eligibility or any subsequent write fails.
		claim := tx.Model(&Coupon{}).Where("code = ?", code).
			UpdateColumn("used_count", gorm.Expr("used_count + 1"))
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		coupons := NewCouponRepository(tx)
		record, err := coupons.GetByCode(ctx, code)
		if err != nil {
			return err
		}
		record.UsedCount--
		discount, err := validate(coupons, record)
		if err != nil {
			return err
		}
		if discount <= 0 || discount > order.OriginalAmount {
			return gorm.ErrInvalidData
		}
		order.CouponID = &record.ID
		order.DiscountAmount = discount
		order.PayAmount = order.OriginalAmount - discount
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		return coupons.CreateUsage(ctx, &CouponUsage{
			CouponID: record.ID, UserID: order.UserID, OrderID: order.ID,
			Discount: discount, UsedAt: time.Now(),
		})
	})
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

func (r *orderRepository) UpdatePendingPaymentMethod(ctx context.Context, id int64, method string) error {
	result := r.db.WithContext(ctx).Model(&Order{}).
		Where("id = ? AND status = ? AND expired_at > ?", id, OrderStatusPending, time.Now()).
		Update("payment_method", method)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCommercialState
	}
	return nil
}

func (r *orderRepository) ListPaymentsToReconcile(ctx context.Context, methods []string, now time.Time, limit int) ([]*Order, error) {
	var orders []*Order
	err := r.db.WithContext(ctx).
		Where("status = ? AND payment_method IN ? AND pay_amount > 0 AND expired_at > ?", OrderStatusPending, methods, now.AddDate(0, 0, -7)).
		Where("payment_checked_at IS NULL OR payment_checked_at < ?", now.Add(-time.Minute)).
		Order("CASE WHEN payment_checked_at IS NULL THEN 0 ELSE 1 END, payment_checked_at, id").
		Limit(limit).Find(&orders).Error
	return orders, err
}

func (r *orderRepository) MarkPaymentChecked(ctx context.Context, id int64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Order{}).
		Where("id = ? AND status = ?", id, OrderStatusPending).
		UpdateColumn("payment_checked_at", now).Error
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
	switch filter.Status {
	case OrderStatusPending:
		query = query.Where("status = ? AND expired_at > ?", OrderStatusPending, time.Now())
	case "expired":
		query = query.Where("status = ? AND expired_at <= ?", OrderStatusPending, time.Now())
	case "":
	default:
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
	if status == OrderStatusCancelled {
		return r.cancelOrder(ctx, id, false, time.Now())
	}
	allowed := map[string][]string{
		OrderStatusPaid:      {OrderStatusPending},
		OrderStatusCompleted: {OrderStatusPaid},
		OrderStatusRefunded:  {OrderStatusPaid, OrderStatusCompleted},
	}
	from, ok := allowed[status]
	if !ok {
		return ErrCommercialState
	}
	query := r.db.WithContext(ctx).
		Model(&Order{}).
		Where("id = ? AND status IN ?", id, from)
	result := query.Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCommercialState
	}
	return nil
}

const checkoutPaymentGracePeriod = 15 * time.Minute

func (r *orderRepository) cancelOrder(ctx context.Context, id int64, expired bool, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&Order{}).Where("id = ? AND status = ?", id, OrderStatusPending)
		if expired {
			graceCutoff := now.Add(-checkoutPaymentGracePeriod)
			query = query.Where(
				"coupon_id IS NOT NULL AND ((COALESCE(payment_method, '') = '' AND expired_at <= ?) OR (COALESCE(payment_method, '') != '' AND expired_at <= ?))",
				now, graceCutoff,
			)
		} else {
			// A payment attempt can settle later. Keep its reservation while
			// the gateway may still deliver an on-time payment.
			query = query.Where("expired_at > ? AND (coupon_id IS NULL OR COALESCE(payment_method, '') = '')", now)
		}
		result := query.Update("status", OrderStatusCancelled)
		if err := result.Error; err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return ErrCommercialState
		}

		var order Order
		if err := tx.Select("coupon_id").First(&order, id).Error; err != nil || order.CouponID == nil {
			return err
		}
		removed := tx.Where("order_id = ? AND coupon_id = ?", id, *order.CouponID).Delete(&CouponUsage{})
		if err := removed.Error; err != nil {
			return err
		}
		if removed.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&Coupon{}).Where("id = ? AND used_count >= ?", *order.CouponID, removed.RowsAffected).
			UpdateColumn("used_count", gorm.Expr("used_count - ?", removed.RowsAffected)).Error
	})
}

func (r *orderRepository) ReleaseExpiredCouponOrders(ctx context.Context) error {
	now := time.Now()
	graceCutoff := now.Add(-checkoutPaymentGracePeriod)
	var ids []int64
	// The commercial scheduler runs once a minute. Bound each sweep to
	// 100 indexed candidates so checkout and gateway callbacks stay responsive.
	err := r.db.WithContext(ctx).Model(&Order{}).
		Where(
			"status = ? AND coupon_id IS NOT NULL AND ((COALESCE(payment_method, '') = '' AND expired_at <= ?) OR (COALESCE(payment_method, '') != '' AND expired_at <= ?))",
			OrderStatusPending, now, graceCutoff,
		).
		Order("expired_at, id").Limit(100).Pluck("id", &ids).Error
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := r.cancelOrder(ctx, id, true, now); err != nil && err != ErrCommercialState {
			return err
		}
	}
	return nil
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

// CancelExpired cancels all expired pending non-coupon orders (respecting a grace period when checkout started).
func (r *orderRepository) CancelExpired(ctx context.Context) (int64, error) {
	now := time.Now()
	graceCutoff := now.Add(-checkoutPaymentGracePeriod)
	result := r.db.WithContext(ctx).
		Model(&Order{}).
		Where(
			"status = ? AND coupon_id IS NULL AND ((COALESCE(payment_method, '') = '' AND expired_at < ?) OR (COALESCE(payment_method, '') != '' AND expired_at < ?))",
			OrderStatusPending, now, graceCutoff,
		).
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

// cashOrders selects orders that brought money in during the range. Balance
// and free payments move no new money (the recharge that funded the balance is
// reported separately), and a refund goes back to the wallet, not out of the
// business, so refunded orders still count.
func (r *orderRepository) cashOrders(ctx context.Context, start, end time.Time) *gorm.DB {
	return r.db.WithContext(ctx).Model(&Order{}).
		Where("status IN ? AND paid_at >= ? AND paid_at <= ? AND COALESCE(payment_method, '') NOT IN ?",
			[]string{OrderStatusPaid, OrderStatusCompleted, OrderStatusRefunded}, start, end, []string{"balance", "free"})
}

// GetRevenueByDateRange returns money received for orders in a date range.
func (r *orderRepository) GetRevenueByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	var total int64
	err := r.cashOrders(ctx, start, end).Select("COALESCE(SUM(pay_amount), 0)").Scan(&total).Error
	return total, err
}

// GetOrderCountByDateRange returns the number of orders counted by GetRevenueByDateRange.
func (r *orderRepository) GetOrderCountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := r.cashOrders(ctx, start, end).Count(&count).Error
	return count, err
}
