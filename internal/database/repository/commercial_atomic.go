package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCommercialState    = errors.New("commercial state changed or operation is not allowed")
	ErrCommercialExpired  = errors.New("order has expired")
	ErrCurrentPlanUnknown = errors.New("current plan cannot be determined from a paid order; contact administrator")
	ErrPlanPeriodMismatch = errors.New("plan changes require the same billing period; purchase the new plan to change periods")
	ErrFreePlanClaimed    = errors.New("free plan has already been claimed")
)

// These narrow capabilities keep financial transactions inside the database.
type AtomicOrderRepository interface {
	PayAndApplyPlan(context.Context, string, string, string, bool, bool, time.Time) (*Order, error)
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

// AtomicPauseRepository freezes and restores a subscription in one transaction
// each, so a pause can be resumed exactly once.
type AtomicPauseRepository interface {
	PauseSubscription(context.Context, *SubscriptionPause, int) error
	ResumeSubscription(context.Context, int64, time.Time) (*SubscriptionPause, *time.Time, error)
}

// SamePlanPeriod limits proration to comparable durations and billing types.
// Different periods use a full new purchase rather than mixing monthly/yearly prices.
func SamePlanPeriod(old, target *CommercialPlan) bool {
	normalize := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return PlanTypeMonthly
		}
		return value
	}
	return old.Duration > 0 && old.Duration == target.Duration && normalize(old.PlanType) == normalize(target.PlanType)
}

func monthlyTrafficResetAt(plan *CommercialPlan, at time.Time) *time.Time {
	if !PlanUsesMonthlyTraffic(plan) {
		return nil
	}
	anchor := MonthlyTrafficAnchor(at)
	return &anchor
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

func (r *orderRepository) PayAndApplyPlan(ctx context.Context, orderNo, paymentNo, method string, useBalance, applyPlan bool, verifiedPaidAt time.Time) (*Order, error) {
	var ord Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		if useBalance || applyPlan {
			if err := tx.Select("user_id").Where("order_no = ?", orderNo).First(&ord).Error; err != nil {
				return err
			}
			// Refunds and payments for one account must acquire the user
			// lock before any order row they may update.
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, ord.UserID).Error; err != nil {
				return err
			}
		}
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
		if applyPlan && method == "free" && ord.OriginalAmount == 0 {
			var claimed int64
			if err := tx.Model(&Order{}).
				Where("user_id = ? AND plan_id = ? AND original_amount = 0 AND pay_amount = 0 AND paid_at IS NOT NULL", ord.UserID, ord.PlanID).
				Count(&claimed).Error; err != nil {
				return err
			}
			if claimed > 0 {
				return ErrFreePlanClaimed
			}
		}
		now := time.Now()
		paidAt := now
		if !verifiedPaidAt.IsZero() {
			// Payment gateways report timestamps with one-second precision.
			if verifiedPaidAt.Before(ord.CreatedAt.Truncate(time.Second)) || verifiedPaidAt.After(now.Add(time.Minute)) {
				return gorm.ErrInvalidData
			}
			paidAt = verifiedPaidAt
		}
		if !paidAt.Before(ord.ExpiredAt) {
			return ErrCommercialExpired
		}
		res := tx.Model(&Order{}).Where("id = ? AND status = ?", ord.ID, OrderStatusPending).Updates(map[string]any{
			"status": OrderStatusPaid, "payment_no": paymentNo, "payment_method": method, "paid_at": paidAt,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrCommercialState
		}
		if ord.PayAmount < 0 {
			return gorm.ErrInvalidData
		}
		if useBalance && ord.PayAmount > 0 {
			if _, err := (&balanceRepository{db: tx}).DebitAtomic(ctx, ord.UserID, ord.PayAmount, &BalanceTransaction{Type: BalanceTxTypePurchase, OrderID: &ord.ID, Description: "Balance payment for order " + ord.OrderNo, Operator: "system"}); err != nil {
				return err
			}
		}
		if applyPlan {
			plan, err := ord.PurchasedPlan()
			if err != nil {
				return err
			}
			if plan == nil {
				plan = &CommercialPlan{}
				if err := tx.First(plan, ord.PlanID).Error; err != nil {
					return err
				}
			}
			if plan.Duration <= 0 || plan.TrafficLimit < 0 {
				return gorm.ErrInvalidData
			}
			base := now
			if user.ExpiresAt != nil && user.ExpiresAt.After(base) {
				// Only renewal of the same paid plan carries remaining days.
				// A different plan is a full new purchase starting now.
				currentID := user.CurrentPlanID
				if currentID == 0 {
					var previous Order
					// Exclude this just-paid order when resolving legacy ownership.
					err := tx.Where("user_id = ? AND id <> ? AND status IN ?", user.ID, ord.ID, []string{OrderStatusPaid, OrderStatusCompleted}).Order("paid_at DESC, id DESC").First(&previous).Error
					if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}
					currentID = previous.PlanID
				}
				if currentID == plan.ID {
					base = *user.ExpiresAt
				}
			}
			expires := base.AddDate(0, 0, plan.Duration)
			trafficUsed := int64(0)
			if plan.ResetCycle == ResetCycleNever {
				trafficUsed = user.TrafficUsed
			}
			trafficResetAt := monthlyTrafficResetAt(plan, paidAt)
			if err := tx.Model(&User{}).Where("id = ?", user.ID).Updates(map[string]any{
				"enabled": true, "traffic_used": trafficUsed, "traffic_limit": plan.TrafficLimit, "traffic_reset_at": trafficResetAt, "expires_at": expires,
				"current_plan_id": plan.ID, "current_plan_price": ord.OriginalAmount, "current_plan_duration": plan.Duration, "current_plan_type": plan.PlanType,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Order{}).Where("id = ?", ord.ID).Updates(map[string]any{
				"fulfilled_at": now, "fulfillment_pending": true, "entitlement_start_at": base, "entitlement_end_at": expires,
			}).Error; err != nil {
				return err
			}
			ord.FulfilledAt, ord.FulfillmentPending = &now, true
		}
		ord.Status, ord.PaymentNo, ord.PaymentMethod, ord.PaidAt = OrderStatusPaid, paymentNo, method, &paidAt
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
		if err := tx.Select("user_id").First(&ord, id).Error; err != nil {
			return err
		}
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, ord.UserID).Error; err != nil {
			return err
		}
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
		if amount == total {
			if err := revokeRefundedEntitlement(tx, &ord, &user, time.Now()); err != nil {
				return err
			}
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

func revokeRefundedEntitlement(tx *gorm.DB, ord *Order, user *User, now time.Time) error {
	start, end := ord.EntitlementStartAt, ord.EntitlementEndAt
	if ord.FulfilledAt == nil || start == nil || end == nil || !end.After(now) || !end.After(*start) {
		// Legacy orders did not record their granted interval.
		return nil
	}
	if user.CurrentPlanID != ord.PlanID || user.ExpiresAt == nil || !user.ExpiresAt.After(now) {
		return nil
	}
	var later []Order
	if err := tx.Where("user_id = ? AND (fulfilled_at > ? OR (fulfilled_at = ? AND id > ?))",
		ord.UserID, ord.FulfilledAt, ord.FulfilledAt, ord.ID).
		Where("status IN ? OR (status = ? AND refunded_amount < pay_amount + balance_used)",
			[]string{OrderStatusPaid, OrderStatusCompleted}, OrderStatusRefunded).
		Select("id", "plan_id", "entitlement_start_at", "entitlement_end_at").Order("fulfilled_at, id").Find(&later).Error; err != nil {
		return err
	}
	tail := *end
	for _, next := range later {
		if next.PlanID != ord.PlanID || next.EntitlementStartAt == nil || next.EntitlementEndAt == nil ||
			!next.EntitlementStartAt.Equal(tail) {
			return nil
		}
		tail = *next.EntitlementEndAt
	}
	if !user.ExpiresAt.Equal(tail) {
		// A pause, admin edit, or later refund can move the expiry independently.
		return nil
	}
	unused := end.Sub(*start)
	if start.Before(now) {
		unused = end.Sub(now)
	}
	for _, next := range later {
		if err := tx.Model(&Order{}).Where("id = ?", next.ID).Updates(map[string]any{
			"entitlement_start_at": next.EntitlementStartAt.Add(-unused),
			"entitlement_end_at":   next.EntitlementEndAt.Add(-unused),
		}).Error; err != nil {
			return err
		}
	}
	return tx.Model(&User{}).Where("id = ?", user.ID).Update("expires_at", user.ExpiresAt.Add(-unused)).Error
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
	planType := user.CurrentPlanType
	if id == 0 {
		var ord Order
		if err := tx.Where("user_id = ? AND status IN ?", user.ID, []string{OrderStatusPaid, OrderStatusCompleted}).Order("paid_at DESC, id DESC").First(&ord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrCurrentPlanUnknown
			}
			return nil, err
		}
		id, price = ord.PlanID, ord.OriginalAmount
		purchased, err := ord.PurchasedPlan()
		if err != nil {
			return nil, err
		}
		if purchased != nil {
			duration, planType = purchased.Duration, purchased.PlanType
		}
	}
	if err := tx.First(&plan, id).Error; err != nil {
		return nil, err
	}
	plan.Price = price
	// Catalog edits affect future purchases, not an already granted quota.
	plan.TrafficLimit = user.TrafficLimit
	if duration > 0 {
		plan.Duration = duration
	}
	if planType != "" {
		plan.PlanType = planType
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
		if !SamePlanPeriod(old, &plan) {
			return ErrPlanPeriodMismatch
		}
		days := int(math.Ceil(time.Until(*user.ExpiresAt).Hours() / 24))
		amount, err := PlanProration(old.Price, plan.Price, days, old.Duration)
		if err != nil {
			return err
		}
		if amount < 1 {
			amount = 1
		}
		if _, err := (&balanceRepository{db: tx}).DebitAtomic(ctx, userID, amount, &BalanceTransaction{Type: BalanceTxTypePurchase, Description: fmt.Sprintf("Plan upgrade %d -> %d", old.ID, plan.ID), Operator: "system"}); err != nil {
			return err
		}
		// An upgrade keeps the current cycle and usage; only purchases/renewals reset it.
		updates := map[string]any{"traffic_limit": plan.TrafficLimit, "current_plan_id": plan.ID, "current_plan_price": plan.Price, "current_plan_duration": plan.Duration, "current_plan_type": plan.PlanType}
		if resetAt := monthlyTrafficResetAt(&plan, time.Now()); resetAt != nil {
			updates["traffic_reset_at"] = resetAt
		} else {
			updates["traffic_reset_at"] = nil
		}
		if err := tx.Model(&User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
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
		trafficUsed := int64(0)
		if plan.ResetCycle == ResetCycleNever {
			trafficUsed = user.TrafficUsed
		}
		updates := map[string]any{"traffic_used": trafficUsed, "traffic_limit": plan.TrafficLimit, "current_plan_id": plan.ID, "current_plan_price": plan.Price, "current_plan_duration": plan.Duration, "current_plan_type": plan.PlanType}
		if resetAt := monthlyTrafficResetAt(&plan, time.Now()); resetAt != nil {
			updates["traffic_reset_at"] = resetAt
		} else {
			updates["traffic_reset_at"] = nil
		}
		return tx.Model(&User{}).Where("id = ?", user.ID).Updates(updates).Error
	})
}

// PauseSubscription records the pause and ends the user's access at the pause time.
func (r *pauseRepository) PauseSubscription(ctx context.Context, pause *SubscriptionPause, maxPerCycle int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, pause.UserID).Error; err != nil {
			return err
		}
		now := pause.PausedAt
		if !user.Enabled || (user.ExpiresAt != nil && !user.ExpiresAt.After(now)) ||
			(user.ExpiresAt == nil && user.TrafficLimit <= 0) {
			return ErrCommercialState
		}
		var active int64
		if err := tx.Model(&SubscriptionPause{}).Where("user_id = ? AND resumed_at IS NULL", pause.UserID).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrCommercialState
		}
		// The limit counts pauses in the last 30 days. Anchoring the window to
		// the expiry put it in the future for quarterly and yearly plans, so
		// nothing was ever counted.
		cycleStart := now.AddDate(0, 0, -30)
		if cycleStart.Before(user.CreatedAt) {
			cycleStart = user.CreatedAt
		}
		pause.RemainingDays = -1 // Permanent access has no expiry to restore.
		if user.ExpiresAt != nil {
			pause.RemainingDays = max(1, int(user.ExpiresAt.Sub(now).Hours()/24))
		}
		count, err := (&pauseRepository{db: tx}).CountPausesInPeriod(ctx, pause.UserID, cycleStart, now)
		if err != nil {
			return err
		}
		if count >= maxPerCycle {
			return ErrCommercialState
		}
		pause.RemainingTraffic = max(user.TrafficLimit-user.TrafficUsed, 0)
		// SQLite has no SELECT FOR UPDATE. Claim the user with a write that
		// checks the expiry observed above before inserting the pause.
		claim := tx.Model(&User{}).Where("id = ?", pause.UserID)
		if user.ExpiresAt == nil {
			claim = claim.Where("expires_at IS NULL")
		} else {
			claim = claim.Where("expires_at = ?", *user.ExpiresAt)
		}
		result := claim.Update("expires_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCommercialState
		}
		if err := tx.Create(pause).Error; err != nil {
			return err
		}
		return nil
	})
}

// ResumeSubscription claims the user's active pause and gives the frozen days
// back. The claim (resumed_at IS NULL -> now) is what makes it happen once: a
// second request, or the auto-resume scheduler racing a click, finds no pause
// and gets gorm.ErrRecordNotFound.
func (r *pauseRepository) ResumeSubscription(ctx context.Context, userID int64, now time.Time) (*SubscriptionPause, *time.Time, error) {
	var pause SubscriptionPause
	var expiresAt *time.Time
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND resumed_at IS NULL", userID).Order("paused_at DESC").First(&pause).Error; err != nil {
			return err
		}
		claim := tx.Model(&SubscriptionPause{}).Where("id = ? AND resumed_at IS NULL", pause.ID).Update("resumed_at", now)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		pause.ResumedAt = &now

		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		// Pausing set expires_at to the pause time. Whatever it is now tells what
		// happened during the pause.
		switch {
		case user.ExpiresAt == nil:
			// Access was made permanent while paused: nothing to give back.
			return nil
		case user.ExpiresAt.After(now):
			// A plan was bought while paused: the frozen days go on top of it,
			// and its quota is not touched.
			if pause.RemainingDays >= 0 {
				restored := user.ExpiresAt.AddDate(0, 0, pause.RemainingDays)
				expiresAt = &restored
			}
			return tx.Model(&User{}).Where("id = ?", userID).Update("expires_at", expiresAt).Error
		}
		updates := map[string]any{"expires_at": nil}
		if pause.RemainingDays >= 0 {
			restored := now.AddDate(0, 0, pause.RemainingDays)
			expiresAt = &restored
			updates["expires_at"] = restored
		}
		if user.TrafficLimit > 0 {
			updates["traffic_used"] = max(user.TrafficLimit-pause.RemainingTraffic, 0)
		}
		return tx.Model(&User{}).Where("id = ?", userID).Updates(updates).Error
	})
	return &pause, expiresAt, err
}
