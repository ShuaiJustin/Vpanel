package repository

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCommercialState    = errors.New("commercial state changed or operation is not allowed")
	ErrCommercialExpired  = errors.New("order has expired")
	ErrCurrentPlanUnknown = errors.New("current plan cannot be determined from a paid order; contact administrator")
)

// These narrow capabilities keep financial transactions inside the database.
type AtomicOrderRepository interface {
	PayAndApplyPlan(context.Context, string, string, string, bool, bool) (*Order, error)
	RefundToBalance(context.Context, int64, int64, string) (*Order, error)
	MarkFulfillmentSynced(context.Context, int64) error
	ListPendingFulfillments(context.Context, int) ([]*Order, error)
}

type AtomicGiftCardRepository interface {
	RedeemAndCredit(context.Context, int64, int64) (*GiftCard, error)
}

type AtomicPlanChangeRepository interface {
	CurrentPlan(context.Context, int64) (*CommercialPlan, error)
	UpgradeAndDebit(context.Context, int64, int64, int64) error
	ApplyDueDowngrade(context.Context, int64) error
}

func PlanProration(oldPrice, newPrice int64, remainingDays, totalDays int) (int64, error) {
	if oldPrice < 0 || newPrice < 0 || remainingDays < 0 || totalDays <= 0 {
		return 0, gorm.ErrInvalidData
	}
	value := new(big.Int).Sub(big.NewInt(newPrice), big.NewInt(oldPrice))
	value.Mul(value, big.NewInt(int64(remainingDays)))
	value.Quo(value, big.NewInt(int64(totalDays)))
	if !value.IsInt64() {
		return 0, gorm.ErrInvalidData
	}
	return value.Int64(), nil
}

func (r *orderRepository) PayAndApplyPlan(ctx context.Context, orderNo, paymentNo, method string, useBalance, applyPlan bool) (*Order, error) {
	var ord Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&ord).Error; err != nil {
			return err
		}
		if ord.Status == OrderStatusPaid || ord.Status == OrderStatusCompleted {
			if ord.PaymentNo == paymentNo || (useBalance && ord.PaymentMethod == "balance") {
				return nil
			}
			return ErrCommercialState
		}
		if ord.Status != OrderStatusPending {
			return ErrCommercialState
		}
		now := time.Now()
		if now.After(ord.ExpiredAt) {
			return ErrCommercialExpired
		}
		res := tx.Model(&Order{}).Where("id = ? AND status = ?", ord.ID, OrderStatusPending).Updates(map[string]any{
			"status": OrderStatusPaid, "payment_no": paymentNo, "payment_method": method, "paid_at": now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrCommercialState
		}
		if useBalance {
			if _, err := (&balanceRepository{db: tx}).DebitAtomic(ctx, ord.UserID, ord.PayAmount, &BalanceTransaction{Type: BalanceTxTypePurchase, OrderID: &ord.ID, Description: "Balance payment for order " + ord.OrderNo, Operator: "system"}); err != nil {
				return err
			}
		}
		if applyPlan {
			var plan CommercialPlan
			if err := tx.First(&plan, ord.PlanID).Error; err != nil {
				return err
			}
			var user User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, ord.UserID).Error; err != nil {
				return err
			}
			base := now
			if user.ExpiresAt != nil && user.ExpiresAt.After(base) {
				base = *user.ExpiresAt
			}
			expires := base.AddDate(0, 0, plan.Duration)
			if err := tx.Model(&User{}).Where("id = ?", user.ID).Updates(map[string]any{
				"enabled": true, "traffic_used": 0, "traffic_limit": plan.TrafficLimit, "expires_at": expires,
				"current_plan_id": plan.ID, "current_plan_price": ord.OriginalAmount, "current_plan_duration": plan.Duration,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Order{}).Where("id = ?", ord.ID).Updates(map[string]any{"fulfilled_at": now, "fulfillment_pending": true}).Error; err != nil {
				return err
			}
			ord.FulfilledAt, ord.FulfillmentPending = &now, true
		}
		ord.Status, ord.PaymentNo, ord.PaymentMethod, ord.PaidAt = OrderStatusPaid, paymentNo, method, &now
		return nil
	})
	return &ord, err
}

func (r *orderRepository) MarkFulfillmentSynced(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&Order{}).Where("id = ?", id).Update("fulfillment_pending", false).Error
}

func (r *orderRepository) ListPendingFulfillments(ctx context.Context, limit int) ([]*Order, error) {
	var orders []*Order
	err := r.db.WithContext(ctx).Where("fulfillment_pending = ? AND status IN ?", true, []string{OrderStatusPaid, OrderStatusCompleted}).Order("id").Limit(limit).Find(&orders).Error
	return orders, err
}

// A refund is one terminal operation per order, matching the existing API.
// Replays return the recorded amount; a different second amount is rejected.
func (r *orderRepository) RefundToBalance(ctx context.Context, id, amount int64, reason string) (*Order, error) {
	var ord Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ord, id).Error; err != nil {
			return err
		}
		if amount < 0 {
			return gorm.ErrInvalidData
		}
		if ord.Status == OrderStatusRefunded {
			if ord.RefundedAmount > 0 && (amount == 0 || amount == ord.RefundedAmount) {
				return nil
			}
			return ErrCommercialState
		}
		if ord.Status != OrderStatusPaid && ord.Status != OrderStatusCompleted {
			return ErrCommercialState
		}
		// Older releases could restore "paid" after refunding. Recover from
		// their existing refund ledger before considering another credit.
		var previouslyRefunded int64
		if err := tx.Model(&BalanceTransaction{}).Where("order_id = ? AND type = ? AND description LIKE ?", id, BalanceTxTypeRefund, "Refund for order #%").Select("COALESCE(SUM(amount), 0)").Scan(&previouslyRefunded).Error; err != nil {
			return err
		}
		if previouslyRefunded > 0 {
			if err := tx.Model(&Order{}).Where("id = ?", id).Updates(map[string]any{"status": OrderStatusRefunded, "refunded_amount": previouslyRefunded, "fulfillment_pending": false}).Error; err != nil {
				return err
			}
			ord.Status, ord.RefundedAmount = OrderStatusRefunded, previouslyRefunded
			return nil
		}
		total := ord.PayAmount + ord.BalanceUsed
		if amount == 0 {
			amount = total
		}
		if amount <= 0 || amount > total {
			return gorm.ErrInvalidData
		}
		notes := fmt.Sprintf("Refunded %d cents: %s", amount, reason)
		res := tx.Model(&Order{}).Where("id = ? AND status IN ?", id, []string{OrderStatusPaid, OrderStatusCompleted}).Updates(map[string]any{
			"status": OrderStatusRefunded, "notes": notes, "refunded_amount": amount, "fulfillment_pending": false,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrCommercialState
		}
		if _, err := (&balanceRepository{db: tx}).CreditAtomic(ctx, ord.UserID, amount, &BalanceTransaction{Type: BalanceTxTypeRefund, OrderID: &ord.ID, Description: notes, Operator: "system"}); err != nil {
			return err
		}
		if err := tx.Model(&Commission{}).Where("order_id = ? AND status = ?", id, CommissionStatusPending).Update("status", CommissionStatusCancelled).Error; err != nil {
			return err
		}
		ord.Status, ord.Notes, ord.RefundedAmount = OrderStatusRefunded, notes, amount
		return nil
	})
	return &ord, err
}

func (r *giftCardRepository) RedeemAndCredit(ctx context.Context, id, userID int64) (*GiftCard, error) {
	var card GiftCard
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		res := tx.Model(&GiftCard{}).Where("id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?) AND (purchased_by IS NULL OR purchased_by <> ?)", id, GiftCardStatusActive, now, userID).Updates(map[string]any{"status": GiftCardStatusRedeemed, "redeemed_by": userID, "redeemed_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.First(&card, id).Error; err != nil {
			return err
		}
		_, err := (&balanceRepository{db: tx}).CreditAtomic(ctx, userID, card.Value, &BalanceTransaction{Type: BalanceTxTypeRecharge, Description: fmt.Sprintf("Gift card redemption #%d", id), Operator: "system"})
		return err
	})
	return &card, err
}

// Legacy users are resolved from server-side paid orders only. Trial/manual
// grants with no purchase cannot establish an upgrade credit.
func currentPlan(tx *gorm.DB, user *User) (*CommercialPlan, error) {
	var plan CommercialPlan
	id, price, duration := user.CurrentPlanID, user.CurrentPlanPrice, user.CurrentPlanDuration
	if id == 0 {
		var ord Order
		if err := tx.Where("user_id = ? AND status IN ?", user.ID, []string{OrderStatusPaid, OrderStatusCompleted}).Order("paid_at DESC, id DESC").First(&ord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrCurrentPlanUnknown
			}
			return nil, err
		}
		id, price = ord.PlanID, ord.OriginalAmount
	}
	if err := tx.First(&plan, id).Error; err != nil {
		return nil, err
	}
	plan.Price = price
	if duration > 0 {
		plan.Duration = duration
	}
	return &plan, nil
}

func (r *planChangeRepository) CurrentPlan(ctx context.Context, userID int64) (*CommercialPlan, error) {
	var user User
	if err := r.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}
	return currentPlan(r.db.WithContext(ctx), &user)
}

func (r *planChangeRepository) UpgradeAndDebit(ctx context.Context, userID, expectedPlanID, newPlanID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Acquire a write lock before reading subscription state (also on SQLite).
		if err := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("balance", gorm.Expr("balance")).Error; err != nil {
			return err
		}
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		if user.ExpiresAt == nil || !user.ExpiresAt.After(time.Now()) {
			return ErrCommercialExpired
		}
		old, err := currentPlan(tx, &user)
		if err != nil {
			return err
		}
		if old.ID != expectedPlanID {
			return ErrCommercialState
		}
		var plan CommercialPlan
		if err := tx.First(&plan, newPlanID).Error; err != nil {
			return err
		}
		if !plan.IsActive || plan.ID == old.ID || plan.Price <= old.Price || old.Duration <= 0 {
			return ErrCommercialState
		}
		days := int(time.Until(*user.ExpiresAt).Hours() / 24)
		amount, err := PlanProration(old.Price, plan.Price, days, old.Duration)
		if err != nil {
			return err
		}
		if amount > 0 {
			if _, err := (&balanceRepository{db: tx}).DebitAtomic(ctx, userID, amount, &BalanceTransaction{Type: BalanceTxTypePurchase, Description: fmt.Sprintf("Plan upgrade %d -> %d", old.ID, plan.ID), Operator: "system"}); err != nil {
				return err
			}
		}
		if err := tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{"traffic_limit": plan.TrafficLimit, "current_plan_id": plan.ID, "current_plan_price": plan.Price, "current_plan_duration": plan.Duration}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&PendingDowngrade{}).Error
	})
}

func (r *planChangeRepository) ApplyDueDowngrade(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pending PendingDowngrade
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND effective_at <= ?", id, time.Now()).First(&pending).Error; err != nil {
			return err
		}
		res := tx.Delete(&PendingDowngrade{}, pending.ID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrCommercialState
		}
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, pending.UserID).Error; err != nil {
			return err
		}
		current, err := currentPlan(tx, &user)
		if err != nil {
			return err
		}
		// A renewal/upgrade invalidates a stale scheduled downgrade.
		if current.ID != pending.CurrentPlanID || (user.ExpiresAt != nil && user.ExpiresAt.After(pending.EffectiveAt)) {
			return nil
		}
		var plan CommercialPlan
		if err := tx.First(&plan, pending.NewPlanID).Error; err != nil {
			return err
		}
		if !plan.IsActive {
			return ErrCommercialState
		}
		return tx.Model(&User{}).Where("id = ?", user.ID).Updates(map[string]any{"traffic_limit": plan.TrafficLimit, "current_plan_id": plan.ID, "current_plan_price": plan.Price, "current_plan_duration": plan.Duration}).Error
	})
}
