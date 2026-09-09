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
	_, err := repo.PayAndApplyPlan(context.Background(), ord.OrderNo, "PAY-1", "balance", true, true)
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
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := repo.PayAndApplyPlan(context.Background(), ord.OrderNo, "PAY-1", "balance", true, true)
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
	require.NoError(t, db.Model(ord).Update("status", OrderStatusPaid).Error)
	newPlan := &CommercialPlan{Name: "New", Price: 3999, Duration: 30, TrafficLimit: 2000, IsActive: true}
	require.NoError(t, db.Create(newPlan).Error)
	repo := NewPlanChangeRepository(db).(AtomicPlanChangeRepository)
	legacy, err := repo.CurrentPlan(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, old.ID, legacy.ID)
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
	require.Less(t, current.Balance, user.Balance)
	var count int64
	require.NoError(t, db.Model(&BalanceTransaction{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err = repo.CurrentPlan(context.Background(), 999)
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}
