package refund

import (
	"context"
	"testing"
	"testing/quick"
	"time"

	"v/internal/commercial/balance"
	"v/internal/database/repository"
	"v/internal/logger"
)

type mockOrderRepo struct{ orders map[int64]*repository.Order }

func newMockOrderRepo() *mockOrderRepo { return &mockOrderRepo{orders: make(map[int64]*repository.Order)} }
func (m *mockOrderRepo) Create(ctx context.Context, o *repository.Order) error { o.ID = int64(len(m.orders) + 1); m.orders[o.ID] = o; return nil }
func (m *mockOrderRepo) GetByID(ctx context.Context, id int64) (*repository.Order, error) { if o, ok := m.orders[id]; ok { return o, nil }; return nil, ErrOrderNotFound }
func (m *mockOrderRepo) GetByOrderNo(ctx context.Context, no string) (*repository.Order, error) { for _, o := range m.orders { if o.OrderNo == no { return o, nil } }; return nil, ErrOrderNotFound }
func (m *mockOrderRepo) GetByPaymentNo(ctx context.Context, pn string) (*repository.Order, error) { for _, o := range m.orders { if o.PaymentNo == pn { return o, nil } }; return nil, ErrOrderNotFound }
func (m *mockOrderRepo) Update(ctx context.Context, o *repository.Order) error { m.orders[o.ID] = o; return nil }
func (m *mockOrderRepo) UpdateStatus(ctx context.Context, id int64, s string) error { if o, ok := m.orders[id]; ok { o.Status = s; return nil }; return ErrOrderNotFound }
func (m *mockOrderRepo) Delete(ctx context.Context, id int64) error { delete(m.orders, id); return nil }
func (m *mockOrderRepo) List(ctx context.Context, f repository.OrderFilter, l, o int) ([]*repository.Order, int64, error) { return nil, 0, nil }
func (m *mockOrderRepo) ListByUser(ctx context.Context, uid int64, l, o int) ([]*repository.Order, int64, error) { return nil, 0, nil }
func (m *mockOrderRepo) MarkPaid(ctx context.Context, id int64, pn string, pa time.Time) error { return nil }
func (m *mockOrderRepo) GetExpiredPending(ctx context.Context) ([]*repository.Order, error) { return nil, nil }
func (m *mockOrderRepo) CancelExpired(ctx context.Context) (int64, error) { return 0, nil }
func (m *mockOrderRepo) Count(ctx context.Context) (int64, error) { return int64(len(m.orders)), nil }
func (m *mockOrderRepo) CountByStatus(ctx context.Context, s string) (int64, error) { return 0, nil }
func (m *mockOrderRepo) GetRevenueByDateRange(ctx context.Context, s, e time.Time) (int64, error) { return 0, nil }
func (m *mockOrderRepo) GetOrderCountByDateRange(ctx context.Context, s, e time.Time) (int64, error) { return 0, nil }

type mockBalRepo struct{ balances map[int64]int64; txs []*repository.BalanceTransaction }

func newMockBalRepo() *mockBalRepo { return &mockBalRepo{balances: make(map[int64]int64), txs: []*repository.BalanceTransaction{}} }
func (m *mockBalRepo) GetBalance(ctx context.Context, uid int64) (int64, error) { return m.balances[uid], nil }
func (m *mockBalRepo) UpdateBalance(ctx context.Context, uid, amt int64) error { m.balances[uid] = amt; return nil }
func (m *mockBalRepo) IncrementBalance(ctx context.Context, uid, amt int64) error { m.balances[uid] += amt; return nil }
func (m *mockBalRepo) DecrementBalance(ctx context.Context, uid, amt int64) error {
	if m.balances[uid] < amt {
		return repository.ErrInsufficientBalance
	}
	m.balances[uid] -= amt
	return nil
}
func (m *mockBalRepo) CreditAtomic(ctx context.Context, uid, amt int64, tx *repository.BalanceTransaction) (int64, error) {
	m.balances[uid] += amt
	tx.UserID = uid
	tx.Amount = amt
	tx.Balance = m.balances[uid]
	tx.ID = int64(len(m.txs) + 1)
	m.txs = append(m.txs, tx)
	return m.balances[uid], nil
}
func (m *mockBalRepo) DebitAtomic(ctx context.Context, uid, amt int64, tx *repository.BalanceTransaction) (int64, error) {
	if m.balances[uid] < amt {
		return 0, repository.ErrInsufficientBalance
	}
	m.balances[uid] -= amt
	tx.UserID = uid
	tx.Amount = -amt
	tx.Balance = m.balances[uid]
	tx.ID = int64(len(m.txs) + 1)
	m.txs = append(m.txs, tx)
	return m.balances[uid], nil
}
func (m *mockBalRepo) CreateTransaction(ctx context.Context, tx *repository.BalanceTransaction) error { tx.ID = int64(len(m.txs) + 1); m.txs = append(m.txs, tx); return nil }
func (m *mockBalRepo) GetTransactionByID(ctx context.Context, id int64) (*repository.BalanceTransaction, error) { return nil, nil }
func (m *mockBalRepo) ListTransactions(ctx context.Context, f repository.BalanceFilter, l, o int) ([]*repository.BalanceTransaction, int64, error) { return nil, 0, nil }
func (m *mockBalRepo) ListByUser(ctx context.Context, uid int64, l, o int) ([]*repository.BalanceTransaction, int64, error) { return nil, 0, nil }
func (m *mockBalRepo) GetTotalRecharge(ctx context.Context, uid int64) (int64, error) { return 0, nil }
func (m *mockBalRepo) GetTotalSpent(ctx context.Context, uid int64) (int64, error) { return 0, nil }
func (m *mockBalRepo) GetTotalCommission(ctx context.Context, uid int64) (int64, error) { return 0, nil }


// TestProperty_RefundBalanceRestoration tests Property 12: Refund Balance Restoration
// For any refund to balance, the user's balance SHALL increase by exactly the refund amount.
// Validates: Requirements 13.4, 13.5
func TestProperty_RefundBalanceRestoration(t *testing.T) {
	f := func(initBal uint32, payAmt uint16, balUsed uint16) bool {
		if payAmt == 0 && balUsed == 0 {
			return true
		}
		ctx := context.Background()
		oRepo := newMockOrderRepo()
		bRepo := newMockBalRepo()
		bSvc := balance.NewService(bRepo, logger.NewNopLogger())
		rSvc := NewService(oRepo, bSvc, nil, logger.NewNopLogger())
		uid, oid := int64(1), int64(1)
		bRepo.balances[uid] = int64(initBal)
		oRepo.orders[oid] = &repository.Order{ID: oid, OrderNo: "ORD-1", UserID: uid, PlanID: 1, PayAmount: int64(payAmt), BalanceUsed: int64(balUsed), Status: StatusPaid, ExpiredAt: time.Now().Add(time.Hour)}
		before, _ := bSvc.GetBalance(ctx, uid)
		res, err := rSvc.ProcessFullRefund(ctx, oid, "test")
		if err != nil {
			return false
		}
		after, _ := bSvc.GetBalance(ctx, uid)
		return after == before+res.BalanceRestored && res.RefundAmount == int64(payAmt)+int64(balUsed)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}


// TestProperty_PartialRefundBalanceRestoration tests partial refund balance restoration.
// Validates: Requirements 13.4, 13.5
func TestProperty_PartialRefundBalanceRestoration(t *testing.T) {
	f := func(initBal uint32, payAmt uint16, balUsed uint16, pct uint8) bool {
		if payAmt == 0 && balUsed == 0 {
			return true
		}
		ctx := context.Background()
		oRepo := newMockOrderRepo()
		bRepo := newMockBalRepo()
		bSvc := balance.NewService(bRepo, logger.NewNopLogger())
		rSvc := NewService(oRepo, bSvc, nil, logger.NewNopLogger())
		uid, oid := int64(1), int64(1)
		total := int64(payAmt) + int64(balUsed)
		refAmt := (total * (int64(pct%100) + 1)) / 100
		if refAmt == 0 {
			refAmt = 1
		}
		bRepo.balances[uid] = int64(initBal)
		oRepo.orders[oid] = &repository.Order{ID: oid, OrderNo: "ORD-2", UserID: uid, PlanID: 1, PayAmount: int64(payAmt), BalanceUsed: int64(balUsed), Status: StatusPaid, ExpiredAt: time.Now().Add(time.Hour)}
		before, _ := bSvc.GetBalance(ctx, uid)
		res, err := rSvc.ProcessPartialRefund(ctx, oid, refAmt, "partial")
		if err != nil {
			return false
		}
		after, _ := bSvc.GetBalance(ctx, uid)
		return after == before+res.BalanceRestored && res.RefundAmount == refAmt
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
