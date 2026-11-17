package payment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"v/internal/commercial/order"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestDelayedCallbackUsesVerifiedPaymentTime(t *testing.T) {
	for _, tc := range []struct {
		name     string
		offset   time.Duration
		missing  bool
		wantPaid bool
	}{
		{"paid before cutoff", -time.Minute, false, true},
		{"paid after cutoff", time.Minute, false, false},
		{"missing paid time", 0, true, false},
		{"paid before order existed", -2 * time.Hour, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := serviceTestDB(t)
			ord := seedCallbackTestOrder(t, db)
			now := time.Now().Truncate(time.Second)
			deadline := now.Add(-5 * time.Minute)
			require.NoError(t, db.Model(&repository.Order{}).Where("id = ?", ord.ID).Updates(map[string]any{
				"created_at": now.Add(-35 * time.Minute), "expired_at": deadline,
			}).Error)
			paidAt := deadline.Add(tc.offset)
			if tc.missing {
				paidAt = time.Time{}
			}
			service := newPaymentServiceForCallbackTest(t, db)
			service.RegisterGateway(&callbackResultGateway{name: "mock", result: &PaymentResult{
				Success: true, OrderNo: ord.OrderNo, PaymentNo: "PAY-delayed", Amount: ord.PayAmount, PaidAt: paidAt,
			}})
			err := service.HandleCallback(t.Context(), "mock", nil, "")
			var stored repository.Order
			require.NoError(t, db.First(&stored, ord.ID).Error)
			if tc.wantPaid {
				require.NoError(t, err)
				require.Equal(t, order.StatusPaid, stored.Status)
				require.True(t, stored.PaidAt.Equal(paidAt))
				require.NoError(t, service.HandleCallback(t.Context(), "mock", nil, ""))
				var user repository.User
				require.NoError(t, db.First(&user, ord.UserID).Error)
				require.WithinDuration(t, now.AddDate(0, 0, 30), *user.ExpiresAt, 2*time.Second)
			} else {
				require.Error(t, err)
				require.Equal(t, order.StatusPending, stored.Status)
			}
		})
	}
}

type callbackResultGateway struct {
	name   string
	result *PaymentResult
	err    error
}

func (g *callbackResultGateway) Name() string { return g.name }

func (g *callbackResultGateway) CreatePayment(order *PaymentOrder) (*PaymentRequest, error) {
	return &PaymentRequest{}, nil
}

func (g *callbackResultGateway) VerifyCallback(data []byte, signature string) (*PaymentResult, error) {
	return g.result, g.err
}

func (g *callbackResultGateway) QueryPayment(paymentNo string) (*PaymentResult, error) {
	return &PaymentResult{Success: true, PaymentNo: paymentNo}, nil
}

func (g *callbackResultGateway) Refund(paymentNo string, amount int64, reason string) (*RefundResult, error) {
	return &RefundResult{Success: true, RefundNo: paymentNo, Amount: amount}, nil
}

func TestAlipayVerifyCallbackRequiresSignature(t *testing.T) {
	gateway, err := NewAlipayGateway(&AlipayConfig{AppID: "app-id"})
	if err != nil {
		t.Fatalf("failed to create alipay gateway: %v", err)
	}

	_, err = gateway.VerifyCallback([]byte("out_trade_no=ORD-1&trade_no=ALI-1&trade_status=TRADE_SUCCESS&total_amount=10.00"), "")
	if err == nil || !strings.Contains(err.Error(), "missing callback signature") {
		t.Fatalf("expected missing signature error, got %v", err)
	}
}

func TestWeChatVerifyCallbackRequiresSignature(t *testing.T) {
	gateway, err := NewWeChatGateway(&WeChatConfig{
		AppID:  "wx-app",
		MchID:  "mch-id",
		APIKey: "api-key",
	})
	if err != nil {
		t.Fatalf("failed to create wechat gateway: %v", err)
	}

	callback := []byte(`<xml><return_code>SUCCESS</return_code><result_code>SUCCESS</result_code><out_trade_no>ORD-1</out_trade_no><transaction_id>WX-1</transaction_id><total_fee>100</total_fee></xml>`)
	_, err = gateway.VerifyCallback(callback, "")
	if err == nil || !strings.Contains(err.Error(), "missing callback signature") {
		t.Fatalf("expected missing signature error, got %v", err)
	}
}

func TestHandleCallbackRejectsAmountMismatch(t *testing.T) {
	db := serviceTestDB(t)
	createdOrder := seedCallbackTestOrder(t, db)
	service := newPaymentServiceForCallbackTest(t, db)
	service.RegisterGateway(&callbackResultGateway{
		name: "mock",
		result: &PaymentResult{
			Success:   true,
			OrderNo:   createdOrder.OrderNo,
			PaymentNo: "PAY-123",
			Amount:    createdOrder.PayAmount - 1,
		},
	})

	err := service.HandleCallback(context.Background(), "mock", []byte("{}"), "")
	if !errors.Is(err, ErrAmountMismatch) {
		t.Fatalf("expected amount mismatch error, got %v", err)
	}
}

func TestHandleCallbackIsIdempotentAcrossRestart(t *testing.T) {
	db := serviceTestDB(t)
	createdOrder := seedCallbackTestOrder(t, db)
	service := newPaymentServiceForCallbackTest(t, db)
	gateway := &callbackResultGateway{
		name: "mock",
		result: &PaymentResult{
			Success:   true,
			OrderNo:   createdOrder.OrderNo,
			PaymentNo: "PAY-REPLAY-1",
			Amount:    createdOrder.PayAmount,
		},
	}
	service.RegisterGateway(gateway)

	if err := service.HandleCallback(context.Background(), "mock", []byte("{}"), ""); err != nil {
		t.Fatalf("first callback should succeed: %v", err)
	}

	restartedService := newPaymentServiceForCallbackTest(t, db)
	restartedService.RegisterGateway(gateway)

	if err := restartedService.HandleCallback(context.Background(), "mock", []byte("{}"), ""); err != nil {
		t.Fatalf("callback replay after restart should be treated as idempotent, got %v", err)
	}
}

func newPaymentServiceForCallbackTest(t *testing.T, db *gorm.DB) *Service {
	t.Helper()

	log := logger.NewNopLogger()
	orderRepo := repository.NewOrderRepository(db)
	planRepo := repository.NewPlanRepository(db)
	orderService := order.NewService(orderRepo, planRepo, log, nil).WithUserRepository(repository.NewUserRepository(db))
	return NewService(orderService, log)
}

func seedCallbackTestOrder(t *testing.T, db *gorm.DB) *order.Order {
	t.Helper()

	log := logger.NewNopLogger()
	orderRepo := repository.NewOrderRepository(db)
	planRepo := repository.NewPlanRepository(db)
	orderService := order.NewService(orderRepo, planRepo, log, nil)

	if err := db.Create(&repository.User{
		Username:     "callback-user",
		Email:        "callback-user@example.com",
		PasswordHash: "hashed",
		Role:         "user",
		Enabled:      true,
	}).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if err := db.Create(&repository.CommercialPlan{
		Name:     "Callback Plan",
		Price:    1000,
		Duration: 30,
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	var user repository.User
	if err := db.Where("username = ?", "callback-user").First(&user).Error; err != nil {
		t.Fatalf("failed to load user: %v", err)
	}

	var plan repository.CommercialPlan
	if err := db.Where("name = ?", "Callback Plan").First(&plan).Error; err != nil {
		t.Fatalf("failed to load plan: %v", err)
	}

	createdOrder, err := orderService.Create(context.Background(), &order.CreateOrderRequest{
		UserID: user.ID,
		PlanID: plan.ID,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	return createdOrder
}

func serviceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&repository.CommercialPlan{}, &repository.Order{}, &repository.User{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}
