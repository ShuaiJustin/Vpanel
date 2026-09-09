package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestPaidFulfillmentSurvivesFailureAndRestart(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.User{}, &repository.Order{}, &repository.BalanceTransaction{}))
	user := &repository.User{Username: "fulfillment", PasswordHash: "hash", Balance: 5000}
	require.NoError(t, db.Create(user).Error)
	plan := &repository.CommercialPlan{Name: "Paid", Price: 1999, Duration: 30, TrafficLimit: 1000, IsActive: true}
	require.NoError(t, db.Create(plan).Error)
	newService := func() *Service {
		return NewService(repository.NewOrderRepository(db), repository.NewPlanRepository(db), logger.NewNopLogger(), nil).WithUserRepository(repository.NewUserRepository(db))
	}
	svc := newService().WithAfterPlanAppliedHook(func(context.Context, int64) error { return errors.New("node unavailable") })
	ord, err := svc.Create(context.Background(), &CreateOrderRequest{UserID: user.ID, PlanID: plan.ID})
	require.NoError(t, err)
	require.Error(t, svc.Pay(context.Background(), ord.OrderNo, "BALANCE-1", "balance", true))
	var stored repository.User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.Equal(t, int64(3001), stored.Balance)
	require.Equal(t, plan.ID, stored.CurrentPlanID)
	expires := *stored.ExpiresAt
	var paid repository.Order
	require.NoError(t, db.First(&paid, ord.ID).Error)
	require.Equal(t, StatusPaid, paid.Status)
	require.True(t, paid.FulfillmentPending)
	// Simulate traffic and a restarted process before runtime recovery.
	require.NoError(t, db.Model(&stored).Update("traffic_used", 17).Error)
	attempts := 0
	restarted := newService().WithAfterPlanAppliedHook(func(context.Context, int64) error { attempts++; return nil })
	require.NoError(t, restarted.RetryPendingFulfillments(context.Background()))
	require.NoError(t, restarted.Pay(context.Background(), ord.OrderNo, "BALANCE-1", "balance", true))
	require.Equal(t, 1, attempts)
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.Equal(t, int64(3001), stored.Balance)
	require.Equal(t, int64(17), stored.TrafficUsed)
	require.WithinDuration(t, expires, *stored.ExpiresAt, time.Millisecond)
	require.NoError(t, db.First(&paid, ord.ID).Error)
	require.False(t, paid.FulfillmentPending)
	var count int64
	require.NoError(t, db.Model(&repository.BalanceTransaction{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
