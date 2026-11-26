package refund

import (
	"context"
	"fmt"
	"testing"
	"testing/quick"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestProperty_RefundBalanceRestoration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.User{}, &repository.Order{}, &repository.BalanceTransaction{}, &repository.Commission{}))
	svc := NewService(repository.NewOrderRepository(db), nil, nil, logger.NewNopLogger())
	sequence := 0
	check := func(paid uint16, partial bool) bool {
		if paid == 0 {
			return true
		}
		sequence++
		user := &repository.User{Username: fmt.Sprintf("buyer-%d", sequence), PasswordHash: "hash", Balance: 100}
		if db.Create(user).Error != nil {
			return false
		}
		ord := &repository.Order{OrderNo: fmt.Sprintf("ORD-%d", sequence), UserID: user.ID, PlanID: 1, PayAmount: int64(paid), Status: StatusPaid, ExpiredAt: time.Now()}
		if db.Create(ord).Error != nil {
			return false
		}
		amount := int64(0)
		expected := int64(paid)
		if partial {
			amount = int64(paid)/2 + 1
			expected = amount
		}
		result, err := svc.ProcessRefund(context.Background(), &RefundRequest{OrderID: ord.ID, Amount: amount})
		if err != nil || result.RefundAmount != expected {
			return false
		}
		_, err = svc.ProcessRefund(context.Background(), &RefundRequest{OrderID: ord.ID, Amount: amount})
		if err != nil {
			return false
		}
		var stored repository.User
		var storedOrder repository.Order
		if db.First(&stored, user.ID).Error != nil || db.First(&storedOrder, ord.ID).Error != nil {
			return false
		}
		return stored.Balance == 100+expected && storedOrder.Status == StatusRefunded
	}
	require.NoError(t, quick.Check(check, &quick.Config{MaxCount: 50}))
}
