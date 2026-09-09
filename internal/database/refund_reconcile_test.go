package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
)

func TestReconcileLegacyRefundedOrdersUsesExistingLedger(t *testing.T) {
	db, err := New(&Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "refund.db")})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.AutoMigrate())

	user := &repository.User{Username: "legacy-refund", PasswordHash: "hash", Balance: 100}
	require.NoError(t, db.db.Create(user).Error)
	order := &repository.Order{
		OrderNo: "LEGACY-REFUND", UserID: user.ID, PlanID: 1,
		OriginalAmount: 100, PayAmount: 100, Status: repository.OrderStatusCompleted,
		ExpiredAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, db.db.Create(order).Error)
	require.NoError(t, db.db.Create(&repository.BalanceTransaction{
		UserID: user.ID, OrderID: &order.ID, Type: repository.BalanceTxTypeRefund,
		Amount: 100, Balance: 100, Description: "Refund for order #1: legacy",
	}).Error)

	require.NoError(t, db.reconcileLegacyRefundedOrders(context.Background()))
	require.NoError(t, db.db.First(order, order.ID).Error)
	require.Equal(t, repository.OrderStatusRefunded, order.Status)
	require.Equal(t, int64(100), order.RefundedAmount)
	require.False(t, order.FulfillmentPending)
}
