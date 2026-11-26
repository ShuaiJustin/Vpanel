package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"v/internal/commercial/coupon"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestCouponOrderReservations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.CouponUsage{}))

	user := repository.User{Username: "buyer", PasswordHash: "hash"}
	plan := repository.CommercialPlan{Name: "Paid", Price: 1250, Duration: 30, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&plan).Error)
	coupons := repository.NewCouponRepository(db)
	code := repository.Coupon{
		Code: "CHEAPER", Name: "Cheaper", Type: repository.CouponTypePercentage,
		Value: 2000, MaxDiscount: 150, TotalLimit: 1, PerUserLimit: 1,
		StartAt: time.Now().Add(-time.Hour), ExpireAt: time.Now().Add(time.Hour), IsActive: true,
	}
	require.NoError(t, coupons.Create(t.Context(), &code))
	disabled := repository.Coupon{
		Code: "DISABLED", Name: "Disabled", Type: repository.CouponTypeFixed, Value: 100,
		StartAt: code.StartAt, ExpireAt: code.ExpireAt,
	}
	require.NoError(t, coupons.Create(t.Context(), &disabled))
	stored, err := coupons.GetByID(t.Context(), disabled.ID)
	require.NoError(t, err)
	require.False(t, stored.IsActive)
	require.Zero(t, stored.PerUserLimit)
	orderRepo := repository.NewOrderRepository(db)
	service := NewService(orderRepo, repository.NewPlanRepository(db), logger.NewNopLogger(), nil)
	_, err = service.Create(t.Context(), &CreateOrderRequest{UserID: user.ID, PlanID: plan.ID, CouponCode: disabled.Code})
	require.ErrorIs(t, err, coupon.ErrCouponInactive)
	create := func() (*Order, error) {
		return service.Create(t.Context(), &CreateOrderRequest{UserID: user.ID, PlanID: plan.ID, CouponCode: " cheaper "})
	}
	type result struct {
		order *Order
		err   error
	}
	results := make(chan result, 4)
	for range cap(results) {
		go func() {
			order, err := create()
			results <- result{order: order, err: err}
		}()
	}
	var reserved *Order
	for range cap(results) {
		reply := <-results
		if reply.err == nil {
			require.Nil(t, reserved)
			reserved = reply.order
		} else {
			require.ErrorIs(t, reply.err, coupon.ErrCouponLimitReached)
		}
	}
	require.NotNil(t, reserved)
	require.Equal(t, code.ID, *reserved.CouponID)
	require.EqualValues(t, 150, reserved.DiscountAmount)
	require.EqualValues(t, 1100, reserved.PayAmount)

	require.NoError(t, service.Cancel(t.Context(), reserved.ID))
	require.ErrorIs(t, service.Cancel(t.Context(), reserved.ID), ErrOrderCannotCancel)
	expired, err := create()
	require.NoError(t, err)
	require.NoError(t, db.Model(&repository.Order{}).Where("id = ?", expired.ID).Update("expired_at", time.Now().Add(-time.Hour)).Error)
	require.NoError(t, service.ReleaseExpiredCouponOrders(t.Context()))
	require.NoError(t, service.ReleaseExpiredCouponOrders(t.Context()))
	checkout, err := create()
	require.NoError(t, err)
	require.NoError(t, orderRepo.UpdatePendingPaymentMethod(t.Context(), checkout.ID, "alipay"))
	require.ErrorIs(t, service.Cancel(t.Context(), checkout.ID), ErrOrderCannotCancel)
	expiry := time.Now().Add(-time.Second)
	require.NoError(t, db.Model(&repository.Order{}).Where("id = ?", checkout.ID).
		Updates(map[string]any{"created_at": expiry.Add(-time.Minute), "expired_at": expiry}).Error)
	require.NoError(t, service.ReleaseExpiredCouponOrders(t.Context()))
	_, err = create()
	require.ErrorIs(t, err, coupon.ErrCouponLimitReached)
	require.NoError(t, service.PayFromGateway(t.Context(), checkout.OrderNo, "verified-gateway-test", "alipay", expiry.Add(-time.Second)))

	// Once checkout grace window has long passed without payment, expired coupon order must release quota.
	abandoned, err := service.Create(t.Context(), &CreateOrderRequest{UserID: user.ID, PlanID: plan.ID, CouponCode: code.Code})
	require.ErrorIs(t, err, coupon.ErrCouponLimitReached)
	_ = abandoned
	staleOrder, err := service.Create(t.Context(), &CreateOrderRequest{UserID: user.ID, PlanID: plan.ID})
	require.NoError(t, err)
	require.NoError(t, db.Model(&repository.Order{}).Where("id = ?", staleOrder.ID).Updates(map[string]any{
		"payment_method": "alipay",
		"expired_at":     time.Now().Add(-30 * time.Minute),
	}).Error)
	require.NoError(t, service.CancelExpiredOrders(t.Context()))
	refetchedStale, err := service.GetByID(t.Context(), staleOrder.ID)
	require.NoError(t, err)
	require.Equal(t, StatusCancelled, refetchedStale.Status)

	updated, err := coupons.GetByID(t.Context(), code.ID)
	require.NoError(t, err)
	require.Equal(t, 1, updated.UsedCount)
	count, err := coupons.GetUserUsageCount(t.Context(), code.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPaidFulfillmentSurvivesFailureAndRestart(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.User{}, &repository.Order{}, &repository.BalanceTransaction{}))
	user := &repository.User{Username: "fulfillment", PasswordHash: "hash", Balance: 5000, TrafficUsed: 500}
	require.NoError(t, db.Create(user).Error)
	plan := &repository.CommercialPlan{Name: "Paid", Price: 1999, Duration: 30, TrafficLimit: 1000, IsActive: true}
	require.NoError(t, db.Create(plan).Error)
	newService := func() *Service {
		return NewService(repository.NewOrderRepository(db), repository.NewPlanRepository(db), logger.NewNopLogger(), nil).WithUserRepository(repository.NewUserRepository(db))
	}
	svc := newService().WithAfterPlanAppliedHook(func(context.Context, int64) error { return errors.New("node unavailable") })
	ord, err := svc.Create(context.Background(), &CreateOrderRequest{UserID: user.ID, PlanID: plan.ID})
	require.NoError(t, err)
	// An administrator editing the catalog must not change a pending order's terms.
	plan.Name, plan.Duration, plan.TrafficLimit, plan.Price, plan.ResetCycle = "Changed annual", 365, 12000, 9999, repository.ResetCycleNever
	plan.PlanType = repository.PlanTypeYearly
	require.NoError(t, repository.NewPlanRepository(db).Update(context.Background(), plan))
	listed, err := svc.GetByOrderNo(context.Background(), ord.OrderNo)
	require.NoError(t, err)
	require.Equal(t, "Paid", listed.PlanName)
	require.Error(t, svc.Pay(context.Background(), ord.OrderNo, "BALANCE-1", "balance", true))
	var stored repository.User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.Equal(t, int64(3001), stored.Balance)
	require.Equal(t, plan.ID, stored.CurrentPlanID)
	require.EqualValues(t, 1000, stored.TrafficLimit)
	require.Zero(t, stored.TrafficUsed)
	require.WithinDuration(t, time.Now().AddDate(0, 0, 30), *stored.ExpiresAt, 2*time.Second)
	current, err := repository.NewPlanChangeRepository(db).(repository.AtomicPlanChangeRepository).CurrentPlan(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, repository.PlanTypeMonthly, current.PlanType)
	require.Equal(t, 30, current.Duration)
	require.True(t, repository.SamePlanPeriod(current, &repository.CommercialPlan{PlanType: repository.PlanTypeMonthly, Duration: 30}))
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
