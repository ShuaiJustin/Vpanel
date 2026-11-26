package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"v/internal/database/repository"
)

func TestAutoMigrateNormalizesLegacyCommercialPlans(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(&repository.User{}, &repository.Traffic{}, &repository.Order{}, &repository.CommercialPlan{}, &repository.Setting{}))
	legacyAnnual := &repository.CommercialPlan{
		Name: "plan-paid365", Duration: 365, Price: 10000, TrafficLimit: 12000 * 1024 * 1024 * 1024,
		PlanType: repository.PlanTypeMonthly, ResetCycle: repository.ResetCycleMonthly,
	}
	legacyMonthly := &repository.CommercialPlan{
		Name: "legacy monthly", Duration: 30, Price: 100,
		PlanType: repository.PlanTypeMonthly, ResetCycle: repository.ResetCycleMonthly,
	}
	customMonthly := &repository.CommercialPlan{
		Name: "custom 365 day monthly", Duration: 365, Price: 10000,
		PlanType: repository.PlanTypeCustom, ResetCycle: repository.ResetCycleOnPurchase,
	}
	require.NoError(t, db.Create(legacyAnnual).Error)
	require.NoError(t, db.Create(legacyMonthly).Error)
	require.NoError(t, db.Create(customMonthly).Error)
	user := &repository.User{Username: "custom-quota", Enabled: true, CurrentPlanID: legacyAnnual.ID, TrafficLimit: 12345}
	require.NoError(t, db.Create(user).Error)

	// Startup invokes AutoMigrate on every boot; the data repair must be
	// idempotent and must not depend on the standalone SQL migrator.
	require.NoError(t, (&Database{db: db}).normalizeCommercialPlanData(t.Context()))
	require.NoError(t, db.Model(user).Update("traffic_limit", 54321).Error)
	require.NoError(t, (&Database{db: db}).normalizeCommercialPlanData(t.Context()))
	require.NoError(t, db.First(user, user.ID).Error)
	require.EqualValues(t, 54321, user.TrafficLimit)

	var annual, monthly, custom repository.CommercialPlan
	require.NoError(t, db.First(&annual, legacyAnnual.ID).Error)
	require.NoError(t, db.First(&monthly, legacyMonthly.ID).Error)
	require.NoError(t, db.First(&custom, customMonthly.ID).Error)
	require.Equal(t, repository.PlanTypeYearly, annual.PlanType)
	require.Equal(t, repository.ResetCycleMonthly, annual.ResetCycle)
	require.EqualValues(t, 1000*1024*1024*1024, annual.TrafficLimit)
	require.Equal(t, repository.PlanTypeMonthly, monthly.PlanType)
	require.Equal(t, repository.ResetCycleMonthly, monthly.ResetCycle)
	require.Equal(t, repository.PlanTypeCustom, custom.PlanType)
	require.Equal(t, repository.ResetCycleOnPurchase, custom.ResetCycle)
}
