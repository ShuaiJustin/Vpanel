package planchange

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestPlanOwnershipAndScheduledDowngrade(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.User{}, &repository.Order{}, &repository.PendingDowngrade{}, &repository.BalanceTransaction{}))
	expires := time.Now().Add(24 * time.Hour)
	user := &repository.User{Username: "plan-owner", PasswordHash: "hash", ExpiresAt: &expires, TrafficLimit: 2000}
	require.NoError(t, db.Create(user).Error)
	old := &repository.CommercialPlan{Name: "High", Price: 2000, TrafficLimit: 2000, Duration: 30, IsActive: true}
	require.NoError(t, db.Create(old).Error)
	low := &repository.CommercialPlan{Name: "Low", Price: 1000, TrafficLimit: 1000, Duration: 30, IsActive: true}
	require.NoError(t, db.Create(low).Error)
	svc := NewService(repository.NewPlanChangeRepository(db), repository.NewPlanRepository(db), repository.NewUserRepository(db), nil, nil, logger.NewNopLogger())
	request := &PlanChangeRequest{UserID: user.ID, CurrentPlanID: old.ID, NewPlanID: low.ID}
	_, err = svc.CalculateChange(context.Background(), request)
	require.ErrorIs(t, err, ErrCurrentPlanUnknown)
	paid := &repository.Order{OrderNo: "LEGACY-PAID", UserID: user.ID, PlanID: old.ID, OriginalAmount: old.Price, PayAmount: old.Price, Status: "paid", ExpiredAt: time.Now()}
	require.NoError(t, db.Create(paid).Error)
	current, err := svc.GetCurrentPlan(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, old.ID, current.ID)
	require.Equal(t, old.Price, current.Price)
	_, err = svc.CalculateChange(context.Background(), &PlanChangeRequest{UserID: user.ID, CurrentPlanID: 999, NewPlanID: low.ID})
	require.ErrorIs(t, err, ErrCurrentPlanUnknown)
	require.NoError(t, svc.ScheduleDowngrade(context.Background(), request))
	items, total, err := svc.ListPendingDowngrades(context.Background(), 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, old.ID, items[0].CurrentPlan.ID)
	past := time.Now().Add(-time.Minute)
	require.NoError(t, db.Model(&repository.PendingDowngrade{}).Where("user_id = ?", user.ID).Update("effective_at", past).Error)
	require.NoError(t, db.Model(&repository.User{}).Where("id = ?", user.ID).Update("expires_at", past).Error)
	scheduler := NewScheduler(svc, time.Minute, logger.NewNopLogger())
	require.NoError(t, scheduler.RunOnce(context.Background()))
	require.NoError(t, scheduler.RunOnce(context.Background()))
	items, total, err = svc.ListPendingDowngrades(context.Background(), 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)
	var updated repository.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	require.Equal(t, low.TrafficLimit, updated.TrafficLimit)
	require.Equal(t, low.ID, updated.CurrentPlanID)
	// Scheduled downgrade changes the next plan, not an unpaid renewal.
	require.True(t, updated.ExpiresAt.Before(time.Now()))
}
