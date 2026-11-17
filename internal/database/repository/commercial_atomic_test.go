package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func commerceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&User{}, &Order{}, &GiftCard{}, &BalanceTransaction{}, &Commission{}, &PendingDowngrade{}))
	return db
}

func commerceSeed(t *testing.T, db *gorm.DB) (*User, *CommercialPlan, *Order) {
	t.Helper()
	expires := time.Now().Add(10 * 24 * time.Hour)
	u := &User{Username: "buyer", PasswordHash: "hash", Balance: 10000, TrafficLimit: 1, ExpiresAt: &expires}
	p := &CommercialPlan{Name: "Plan", Price: 1999, Duration: 30, TrafficLimit: 1000, IsActive: true}
	require.NoError(t, db.Create(u).Error)
	require.NoError(t, db.Create(p).Error)
	o := &Order{OrderNo: "ORD-atomic", UserID: u.ID, PlanID: p.ID, OriginalAmount: p.Price, PayAmount: p.Price, Status: OrderStatusPending, ExpiredAt: time.Now().Add(time.Hour)}
	require.NoError(t, db.Create(o).Error)
	return u, p, o
}

func TestPayAndApplyRollsBackAndReplays(t *testing.T) {
	db := commerceDB(t)
	user, plan, ord := commerceSeed(t, db)
	repo := NewOrderRepository(db).(AtomicOrderRepository)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_entitlement BEFORE UPDATE OF traffic_limit ON users BEGIN SELECT RAISE(ABORT, 'injected entitlement failure'); END").Error)
	_, err := repo.PayAndApplyPlan(context.Background(), ord.OrderNo, "PAY-1", "balance", true, true, time.Time{})
	require.Error(t, err)
	var stored Order
	require.NoError(t, db.First(&stored, ord.ID).Error)
	require.Equal(t, OrderStatusPending, stored.Status)
	var current User
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance, current.Balance)
	require.Equal(t, user.TrafficLimit, current.TrafficLimit)
	var count int64
	require.NoError(t, db.Model(&BalanceTransaction{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Exec("DROP TRIGGER fail_entitlement").Error)
	require.NoError(t, db.Model(user).Update("current_plan_id", plan.ID).Error)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := repo.PayAndApplyPlan(context.Background(), ord.OrderNo, "PAY-1", "balance", true, true, time.Time{})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	for e := range results {
		require.NoError(t, e)
	}
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance-plan.Price, current.Balance)
	require.Equal(t, plan.ID, current.CurrentPlanID)
	require.Equal(t, plan.Price, current.CurrentPlanPrice)
	require.WithinDuration(t, user.ExpiresAt.AddDate(0, 0, plan.Duration), *current.ExpiresAt, time.Second)
	require.NoError(t, db.Model(&BalanceTransaction{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.First(&stored, ord.ID).Error)
	require.True(t, stored.FulfillmentPending)
	require.NotNil(t, stored.FulfilledAt)
}

func TestPayAndApplyPlanNeverResetKeepsUsage(t *testing.T) {
	db := commerceDB(t)
	user, plan, order := commerceSeed(t, db)
	user.TrafficUsed = 500
	require.NoError(t, db.Save(user).Error)
	plan.ResetCycle = ResetCycleNever
	require.NoError(t, db.Save(plan).Error)

	_, err := NewOrderRepository(db).(AtomicOrderRepository).
		PayAndApplyPlan(context.Background(), order.OrderNo, "PAY-never", "balance", true, true, time.Time{})
	require.NoError(t, err)

	var updated User
	require.NoError(t, db.First(&updated, user.ID).Error)
	require.Equal(t, int64(500), updated.TrafficUsed)
}

func TestRefundAtomicFailureAndConcurrentReplay(t *testing.T) {
	db := commerceDB(t)
	user, _, ord := commerceSeed(t, db)
	require.NoError(t, db.Model(ord).Update("status", OrderStatusPaid).Error)
	repo := NewOrderRepository(db).(AtomicOrderRepository)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_ledger BEFORE INSERT ON balance_transactions BEGIN SELECT RAISE(ABORT, 'injected ledger failure'); END").Error)
	_, err := repo.RefundToBalance(context.Background(), ord.ID, 0, "test")
	require.Error(t, err)
	var stored Order
	require.NoError(t, db.First(&stored, ord.ID).Error)
	require.Equal(t, OrderStatusPaid, stored.Status)
	var current User
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance, current.Balance)
	require.NoError(t, db.Exec("DROP TRIGGER fail_ledger").Error)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := repo.RefundToBalance(context.Background(), ord.ID, 0, "test")
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	for e := range results {
		require.NoError(t, e)
	}
	require.NoError(t, db.First(&stored, ord.ID).Error)
	require.Equal(t, OrderStatusRefunded, stored.Status)
	require.Equal(t, ord.PayAmount, stored.RefundedAmount)
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance+ord.PayAmount, current.Balance)
	var count int64
	require.NoError(t, db.Model(&BalanceTransaction{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err = repo.RefundToBalance(context.Background(), ord.ID, 1, "different request")
	require.ErrorIs(t, err, ErrCommercialState)
}

func TestGiftCardCreditFailureDoesNotConsumeCard(t *testing.T) {
	db := commerceDB(t)
	user, _, _ := commerceSeed(t, db)
	card := &GiftCard{Code: "ATOMIC-CARD", Value: 1999, Status: GiftCardStatusActive}
	require.NoError(t, db.Create(card).Error)
	repo := NewGiftCardRepository(db).(AtomicGiftCardRepository)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_ledger BEFORE INSERT ON balance_transactions BEGIN SELECT RAISE(ABORT, 'injected ledger failure'); END").Error)
	_, err := repo.RedeemAndCredit(context.Background(), card.ID, user.ID)
	require.Error(t, err)
	var stored GiftCard
	require.NoError(t, db.First(&stored, card.ID).Error)
	require.Equal(t, GiftCardStatusActive, stored.Status)
	require.Nil(t, stored.RedeemedBy)
	var current User
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance, current.Balance)
	require.NoError(t, db.Exec("DROP TRIGGER fail_ledger").Error)
	_, err = repo.RedeemAndCredit(context.Background(), card.ID, user.ID)
	require.NoError(t, err)
	_, err = repo.RedeemAndCredit(context.Background(), card.ID, user.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance+card.Value, current.Balance)
}

func TestLegacyRefundLedgerPreventsAnotherCredit(t *testing.T) {
	db := commerceDB(t)
	user, _, ord := commerceSeed(t, db)
	require.NoError(t, db.Model(ord).Update("status", OrderStatusPaid).Error)
	require.NoError(t, db.Create(&BalanceTransaction{UserID: user.ID, Type: BalanceTxTypeRefund, Amount: ord.PayAmount, Balance: user.Balance, OrderID: &ord.ID, Description: "Refund for order #1: old release"}).Error)
	result, err := NewOrderRepository(db).(AtomicOrderRepository).RefundToBalance(context.Background(), ord.ID, 0, "retry")
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, result.Status)
	var current User
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance, current.Balance)
	var count int64
	require.NoError(t, db.Model(&BalanceTransaction{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestUpgradeUsesServerPlanAndRollsBackDebit(t *testing.T) {
	db := commerceDB(t)
	user, old, ord := commerceSeed(t, db)
	require.NoError(t, db.Model(&User{}).Where("id = ?", user.ID).Update("traffic_used", 500).Error)
	require.NoError(t, db.Model(ord).Update("status", OrderStatusPaid).Error)
	newPlan := &CommercialPlan{Name: "New", Price: 3999, Duration: 30, TrafficLimit: 2000, IsActive: true}
	require.NoError(t, db.Create(newPlan).Error)
	repo := NewPlanChangeRepository(db).(AtomicPlanChangeRepository)
	legacy, err := repo.CurrentPlan(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, old.ID, legacy.ID)
	require.Equal(t, user.TrafficLimit, legacy.TrafficLimit)
	require.ErrorIs(t, repo.UpgradeAndDebit(context.Background(), user.ID, 999, newPlan.ID), ErrCommercialState)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_upgrade BEFORE UPDATE OF current_plan_id ON users BEGIN SELECT RAISE(ABORT, 'injected upgrade failure'); END").Error)
	require.Error(t, repo.UpgradeAndDebit(context.Background(), user.ID, old.ID, newPlan.ID))
	var current User
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, user.Balance, current.Balance)
	require.Zero(t, current.CurrentPlanID)
	require.NoError(t, db.Exec("DROP TRIGGER fail_upgrade").Error)
	require.NoError(t, repo.UpgradeAndDebit(context.Background(), user.ID, old.ID, newPlan.ID))
	require.ErrorIs(t, repo.UpgradeAndDebit(context.Background(), user.ID, old.ID, newPlan.ID), ErrCommercialState)
	require.NoError(t, db.First(&current, user.ID).Error)
	require.Equal(t, newPlan.ID, current.CurrentPlanID)
	require.EqualValues(t, 500, current.TrafficUsed)
	require.Less(t, current.Balance, user.Balance)
	var count int64
	require.NoError(t, db.Model(&BalanceTransaction{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err = repo.CurrentPlan(context.Background(), 999)
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestCreateInactivePlanStaysUnavailable(t *testing.T) {
	db := commerceDB(t)
	repo := NewPlanRepository(db)
	plan := &CommercialPlan{Name: "Draft", Price: 1000, Duration: 30, IsActive: false}
	require.NoError(t, repo.Create(context.Background(), plan))
	stored, err := repo.GetByID(context.Background(), plan.ID)
	require.NoError(t, err)
	require.False(t, stored.IsActive)
	active, err := repo.ListActive(context.Background())
	require.NoError(t, err)
	require.Empty(t, active)
}

func TestUpgradeRejectsDifferentBillingPeriod(t *testing.T) {
	for _, tc := range []struct {
		name, planType string
		duration       int
	}{
		{"annual", PlanTypeYearly, 365},
		{"custom duration", PlanTypeCustom, 45},
		{"different type", PlanTypeCustom, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := commerceDB(t)
			user, old, ord := commerceSeed(t, db)
			require.NoError(t, db.Model(ord).Update("status", OrderStatusPaid).Error)
			plan := &CommercialPlan{Name: tc.name, Price: 3999, Duration: tc.duration, PlanType: tc.planType, IsActive: true}
			require.NoError(t, db.Create(plan).Error)
			err := NewPlanChangeRepository(db).(AtomicPlanChangeRepository).UpgradeAndDebit(context.Background(), user.ID, old.ID, plan.ID)
			require.ErrorIs(t, err, ErrPlanPeriodMismatch)
			var stored User
			require.NoError(t, db.First(&stored, user.ID).Error)
			require.Equal(t, user.Balance, stored.Balance)
			require.Zero(t, stored.CurrentPlanID)
		})
	}
}

func TestLastDayUpgradePreservesUsedTraffic(t *testing.T) {
	db := commerceDB(t)
	user, old, ord := commerceSeed(t, db)
	expires := time.Now().Add(12 * time.Hour)
	require.NoError(t, db.Model(user).Updates(map[string]any{"expires_at": expires, "traffic_used": 900}).Error)
	require.NoError(t, db.Model(ord).Update("status", OrderStatusPaid).Error)
	plan := &CommercialPlan{Name: "Higher quota", Price: 3999, Duration: 30, TrafficLimit: 2000, ResetCycle: ResetCycleOnPurchase, IsActive: true}
	require.NoError(t, db.Create(plan).Error)
	require.NoError(t, NewPlanChangeRepository(db).(AtomicPlanChangeRepository).UpgradeAndDebit(context.Background(), user.ID, old.ID, plan.ID))
	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.EqualValues(t, 900, stored.TrafficUsed)
	require.WithinDuration(t, expires, *stored.ExpiresAt, time.Millisecond)
	require.Less(t, stored.Balance, user.Balance)
}

func TestNewPurchaseOnlyCarriesDaysForSamePlan(t *testing.T) {
	for _, kind := range []string{"same plan", "different plan", "legacy same plan", "legacy different plan", "trial"} {
		t.Run(kind, func(t *testing.T) {
			db := commerceDB(t)
			user, plan, ord := commerceSeed(t, db)
			switch kind {
			case "same plan":
				require.NoError(t, db.Model(user).Update("current_plan_id", plan.ID).Error)
			case "different plan":
				other := &CommercialPlan{Name: "Annual", Price: 9999, Duration: 365, IsActive: true}
				require.NoError(t, db.Create(other).Error)
				require.NoError(t, db.Model(user).Update("current_plan_id", other.ID).Error)
			case "legacy same plan", "legacy different plan":
				planID := plan.ID
				if kind == "legacy different plan" {
					other := &CommercialPlan{Name: "Annual", Price: 9999, Duration: 365, IsActive: true}
					require.NoError(t, db.Create(other).Error)
					planID = other.ID
				}
				paid := &Order{OrderNo: "legacy", UserID: user.ID, PlanID: planID, OriginalAmount: plan.Price, Status: OrderStatusCompleted, ExpiredAt: time.Now()}
				require.NoError(t, db.Create(paid).Error)
			}
			start := time.Now()
			_, err := NewOrderRepository(db).(AtomicOrderRepository).PayAndApplyPlan(context.Background(), ord.OrderNo, "PAY", "balance", true, true, time.Time{})
			require.NoError(t, err)
			var stored User
			require.NoError(t, db.First(&stored, user.ID).Error)
			expected := start.AddDate(0, 0, plan.Duration)
			if kind == "same plan" || kind == "legacy same plan" {
				expected = user.ExpiresAt.AddDate(0, 0, plan.Duration)
			}
			require.WithinDuration(t, expected, *stored.ExpiresAt, 2*time.Second)
		})
	}
}

func TestZeroPricePurchaseNeedsNoWalletDebit(t *testing.T) {
	db := commerceDB(t)
	user, plan, ord := commerceSeed(t, db)
	require.NoError(t, db.Model(user).Update("balance", 0).Error)
	require.NoError(t, db.Model(ord).Updates(map[string]any{"original_amount": 0, "pay_amount": 0}).Error)
	_, err := NewOrderRepository(db).(AtomicOrderRepository).PayAndApplyPlan(context.Background(), ord.OrderNo, "FREE", "balance", true, true, time.Time{})
	require.NoError(t, err)
	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.Zero(t, stored.Balance)
	require.Equal(t, plan.ID, stored.CurrentPlanID)
	var count int64
	require.NoError(t, db.Model(&BalanceTransaction{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestLegacyPendingOrderTermsFreezeBeforePlanEdit(t *testing.T) {
	db := commerceDB(t)
	user, plan, ord := commerceSeed(t, db)
	plan.Duration, plan.TrafficLimit, plan.Price = 365, 12000, 9999
	require.NoError(t, NewPlanRepository(db).Update(context.Background(), plan))
	var frozen Order
	require.NoError(t, db.First(&frozen, ord.ID).Error)
	snapshot, err := frozen.PurchasedPlan()
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, 30, snapshot.Duration)
	require.EqualValues(t, 1000, snapshot.TrafficLimit)
	_, err = NewOrderRepository(db).(AtomicOrderRepository).PayAndApplyPlan(context.Background(), ord.OrderNo, "PAY-frozen", "balance", true, true, time.Time{})
	require.NoError(t, err)
	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.EqualValues(t, 1000, stored.TrafficLimit)
	require.EqualValues(t, 1999, stored.CurrentPlanPrice)
	require.Equal(t, 30, stored.CurrentPlanDuration)
}

func TestPlanUpdateSyncsActiveEntitlementsAndPreservesUsage(t *testing.T) {
	db := commerceDB(t)
	user, plan, order := commerceSeed(t, db)
	require.NoError(t, db.Model(order).Updates(map[string]any{
		"status":  OrderStatusCompleted,
		"paid_at": time.Now(),
	}).Error)
	require.NoError(t, db.Model(user).Updates(map[string]any{
		"current_plan_id": plan.ID,
		"traffic_used":    int64(500),
	}).Error)
	oldExpiry := *user.ExpiresAt
	plan.TrafficLimit = 2000
	require.NoError(t, NewPlanRepository(db).Update(context.Background(), plan))

	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.EqualValues(t, 2000, stored.TrafficLimit)
	require.EqualValues(t, 500, stored.TrafficUsed)
	require.WithinDuration(t, oldExpiry, *stored.ExpiresAt, time.Millisecond)
}

func TestPlanUpdateSyncsLegacyActiveEntitlementFromLatestPaidOrder(t *testing.T) {
	db := commerceDB(t)
	user, plan, order := commerceSeed(t, db)
	require.NoError(t, db.Model(order).Updates(map[string]any{
		"status":  OrderStatusCompleted,
		"paid_at": time.Now(),
	}).Error)
	require.NoError(t, db.Model(user).Updates(map[string]any{
		"traffic_used": int64(700),
	}).Error)
	plan.TrafficLimit = 3000
	require.NoError(t, NewPlanRepository(db).Update(context.Background(), plan))

	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.EqualValues(t, 3000, stored.TrafficLimit)
	require.EqualValues(t, 700, stored.TrafficUsed)
}

func TestPlanUpdateDoesNotSyncExpiredEntitlement(t *testing.T) {
	db := commerceDB(t)
	user, plan, order := commerceSeed(t, db)
	expired := time.Now().Add(-time.Hour)
	require.NoError(t, db.Model(user).Updates(map[string]any{
		"current_plan_id": plan.ID,
		"expires_at":      expired,
	}).Error)
	require.NoError(t, db.Model(order).Update("status", OrderStatusCompleted).Error)
	plan.TrafficLimit = 4000
	require.NoError(t, NewPlanRepository(db).Update(context.Background(), plan))

	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.EqualValues(t, 1, stored.TrafficLimit)
}

func TestPlanUpdateIgnoresLegacyUserWhoseLatestPaidOrderIsAnotherPlan(t *testing.T) {
	db := commerceDB(t)
	user, plan, order := commerceSeed(t, db)
	firstPaidAt := time.Now().Add(-time.Hour)
	require.NoError(t, db.Model(order).Updates(map[string]any{
		"status":  OrderStatusCompleted,
		"paid_at": firstPaidAt,
	}).Error)
	other := &CommercialPlan{Name: "Other", Price: 2999, Duration: 30, TrafficLimit: 5000, IsActive: true}
	require.NoError(t, db.Create(other).Error)
	newerPaidAt := firstPaidAt.Add(time.Minute)
	require.NoError(t, db.Create(&Order{
		OrderNo:        "ORD-newer-plan",
		UserID:         user.ID,
		PlanID:         other.ID,
		OriginalAmount: other.Price,
		PayAmount:      other.Price,
		Status:         OrderStatusCompleted,
		PaidAt:         &newerPaidAt,
		ExpiredAt:      time.Now().Add(time.Hour),
	}).Error)
	plan.TrafficLimit = 3000
	require.NoError(t, NewPlanRepository(db).Update(context.Background(), plan))

	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.EqualValues(t, 1, stored.TrafficLimit)
}

func TestInvalidOrderSnapshotRollsBackPayment(t *testing.T) {
	db := commerceDB(t)
	user, _, ord := commerceSeed(t, db)
	require.NoError(t, db.Model(ord).Update("plan_snapshot", "{broken").Error)
	_, err := NewOrderRepository(db).(AtomicOrderRepository).PayAndApplyPlan(context.Background(), ord.OrderNo, "PAY-bad", "balance", true, true, time.Time{})
	require.Error(t, err)
	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.Equal(t, user.Balance, stored.Balance)
	var storedOrder Order
	require.NoError(t, db.First(&storedOrder, ord.ID).Error)
	require.Equal(t, OrderStatusPending, storedOrder.Status)
}
