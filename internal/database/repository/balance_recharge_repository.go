// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BalanceRechargeOrder represents a balance recharge order in the database.
type BalanceRechargeOrder struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	OrderNo   string     `gorm:"uniqueIndex;size:64;not null"`
	UserID    int64      `gorm:"index;not null"`
	Amount    int64      `gorm:"not null"`
	Method    string     `gorm:"size:32;not null"`
	Status    string     `gorm:"size:32;default:pending;index"`
	PaymentNo string     `gorm:"size:128;index"`
	PaidAt    *time.Time `gorm:"index"`
	ExpiredAt time.Time  `gorm:"index;not null"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`

	User *User `gorm:"foreignKey:UserID"`
}

// TableName returns the table name for BalanceRechargeOrder.
func (BalanceRechargeOrder) TableName() string {
	return "balance_recharge_orders"
}

// Balance recharge order status constants.
const (
	BalanceRechargeStatusPending   = "pending"
	BalanceRechargeStatusPaid      = "paid"
	BalanceRechargeStatusCancelled = "cancelled"
	BalanceRechargeStatusExpired   = "expired"
)

// BalanceRechargeOrderFilter defines filter options for listing recharge orders.
type BalanceRechargeOrderFilter struct {
	UserID    *int64
	Search    string
	Status    string
	Method    string
	StartDate *time.Time
	EndDate   *time.Time
	MinAmount *int64
	MaxAmount *int64
}

// BalanceRechargeOrderRepository defines the interface for balance recharge order data access.
type BalanceRechargeOrderRepository interface {
	Create(ctx context.Context, order *BalanceRechargeOrder) error
	GetByOrderNo(ctx context.Context, orderNo string) (*BalanceRechargeOrder, error)
	GetByPaymentNo(ctx context.Context, paymentNo string) (*BalanceRechargeOrder, error)
	Update(ctx context.Context, order *BalanceRechargeOrder) error
	Cancel(ctx context.Context, orderNo string) error
	List(ctx context.Context, filter BalanceRechargeOrderFilter, limit, offset int) ([]*BalanceRechargeOrder, int64, error)
	MarkPaidAndCredit(ctx context.Context, orderNo string, paymentNo string, paidAt time.Time, description string) (*BalanceRechargeOrder, error)
}

type balanceRechargeOrderRepository struct {
	db *gorm.DB
}

// NewBalanceRechargeOrderRepository creates a new balance recharge order repository.
func NewBalanceRechargeOrderRepository(db *gorm.DB) BalanceRechargeOrderRepository {
	return &balanceRechargeOrderRepository{db: db}
}

// Create creates a new balance recharge order.
func (r *balanceRechargeOrderRepository) Create(ctx context.Context, order *BalanceRechargeOrder) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// GetByOrderNo retrieves a recharge order by order number.
func (r *balanceRechargeOrderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*BalanceRechargeOrder, error) {
	var order BalanceRechargeOrder
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetByPaymentNo retrieves a recharge order by external payment number.
func (r *balanceRechargeOrderRepository) GetByPaymentNo(ctx context.Context, paymentNo string) (*BalanceRechargeOrder, error) {
	var order BalanceRechargeOrder
	err := r.db.WithContext(ctx).Where("payment_no = ?", paymentNo).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Update updates a recharge order.
func (r *balanceRechargeOrderRepository) Update(ctx context.Context, order *BalanceRechargeOrder) error {
	return r.db.WithContext(ctx).Save(order).Error
}

// Cancel marks a pending recharge order as cancelled.
func (r *balanceRechargeOrderRepository) Cancel(ctx context.Context, orderNo string) error {
	result := r.db.WithContext(ctx).
		Model(&BalanceRechargeOrder{}).
		Where("order_no = ? AND status = ?", orderNo, BalanceRechargeStatusPending).
		Update("status", BalanceRechargeStatusCancelled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var order BalanceRechargeOrder
		if err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return err
		}
		return gorm.ErrInvalidData
	}

	return nil
}

// List lists recharge orders with filter and pagination.
func (r *balanceRechargeOrderRepository) List(ctx context.Context, filter BalanceRechargeOrderFilter, limit, offset int) ([]*BalanceRechargeOrder, int64, error) {
	var orders []*BalanceRechargeOrder
	var total int64

	query := r.db.WithContext(ctx).Model(&BalanceRechargeOrder{})

	if filter.UserID != nil {
		query = query.Where("balance_recharge_orders.user_id = ?", *filter.UserID)
	}
	if filter.Status != "" {
		query = query.Where("balance_recharge_orders.status = ?", filter.Status)
	}
	if filter.Method != "" {
		query = query.Where("balance_recharge_orders.method = ?", filter.Method)
	}
	if filter.StartDate != nil {
		query = query.Where("balance_recharge_orders.created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("balance_recharge_orders.created_at <= ?", *filter.EndDate)
	}
	if filter.MinAmount != nil {
		query = query.Where("balance_recharge_orders.amount >= ?", *filter.MinAmount)
	}
	if filter.MaxAmount != nil {
		query = query.Where("balance_recharge_orders.amount <= ?", *filter.MaxAmount)
	}
	if searchQuery := strings.TrimSpace(filter.Search); searchQuery != "" {
		if numericID, err := strconv.ParseInt(searchQuery, 10, 64); err == nil {
			query = query.Where("(balance_recharge_orders.user_id = ? OR balance_recharge_orders.id = ?)", numericID, numericID)
		} else {
			searchLike := "%" + searchQuery + "%"
			query = query.Joins("LEFT JOIN users ON users.id = balance_recharge_orders.user_id").
				Select("balance_recharge_orders.*").
				Where("(balance_recharge_orders.order_no LIKE ? OR balance_recharge_orders.payment_no LIKE ? OR users.username LIKE ?)", searchLike, searchLike, searchLike)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("User").Order("balance_recharge_orders.created_at DESC").Limit(limit).Offset(offset).Find(&orders).Error
	return orders, total, err
}

// MarkPaidAndCredit marks the recharge order as paid and credits the user's balance atomically.
func (r *balanceRechargeOrderRepository) MarkPaidAndCredit(ctx context.Context, orderNo string, paymentNo string, paidAt time.Time, description string) (*BalanceRechargeOrder, error) {
	var updated BalanceRechargeOrder

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order BalanceRechargeOrder
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return err
		}

		if order.PaymentNo == paymentNo && order.Status == BalanceRechargeStatusPaid {
			updated = order
			return nil
		}

		if order.Status != BalanceRechargeStatusPending {
			return gorm.ErrInvalidData
		}

		if paidAt.IsZero() {
			paidAt = time.Now()
		}

		result := tx.Model(&BalanceRechargeOrder{}).
			Where("id = ? AND status = ?", order.ID, BalanceRechargeStatusPending).
			Updates(map[string]any{
				"status":     BalanceRechargeStatusPaid,
				"payment_no": paymentNo,
				"paid_at":    paidAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var latest BalanceRechargeOrder
			if err := tx.Where("id = ?", order.ID).First(&latest).Error; err != nil {
				return err
			}
			if latest.PaymentNo == paymentNo && latest.Status == BalanceRechargeStatusPaid {
				updated = latest
				return nil
			}
			return gorm.ErrInvalidData
		}

		var currentBalance int64
		if err := tx.Model(&User{}).
			Where("id = ?", order.UserID).
			Select("balance").
			Scan(&currentBalance).Error; err != nil {
			return err
		}

		balanceResult := tx.Model(&User{}).
			Where("id = ?", order.UserID).
			Update("balance", gorm.Expr("balance + ?", order.Amount))
		if balanceResult.Error != nil {
			return balanceResult.Error
		}
		if balanceResult.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		balanceTx := &BalanceTransaction{
			UserID:      order.UserID,
			Type:        BalanceTxTypeRecharge,
			Amount:      order.Amount,
			Balance:     currentBalance + order.Amount,
			Description: description,
			Operator:    "system",
		}
		if err := tx.Create(balanceTx).Error; err != nil {
			return err
		}

		order.Status = BalanceRechargeStatusPaid
		order.PaymentNo = paymentNo
		order.PaidAt = &paidAt
		updated = order
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrInvalidData) {
			return nil, err
		}
		return nil, err
	}

	return &updated, nil
}
