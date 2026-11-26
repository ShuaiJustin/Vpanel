package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/commercial/balance"
	"v/internal/commercial/payment"
	"v/internal/database"
	"v/internal/database/repository"
	"v/internal/logger"
)

type failingRechargeGateway struct {
	name string
}

func (g *failingRechargeGateway) Name() string {
	return g.name
}

func (g *failingRechargeGateway) CreatePayment(*payment.PaymentOrder) (*payment.PaymentRequest, error) {
	return nil, errors.New("gateway create payment failed")
}

func (g *failingRechargeGateway) VerifyCallback([]byte, string) (*payment.PaymentResult, error) {
	return nil, errors.New("not implemented")
}

func (g *failingRechargeGateway) QueryPayment(string) (*payment.PaymentResult, error) {
	return nil, errors.New("not implemented")
}

func (g *failingRechargeGateway) Refund(string, int64, string) (*payment.RefundResult, error) {
	return nil, errors.New("not implemented")
}

func setupBalanceHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.AutoMigrate(&database.User{}, &repository.BalanceRechargeOrder{}, &repository.BalanceTransaction{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	if err := db.Create(&database.User{
		Username: "recharge-user",
		Email:    "recharge@test.local",
		Password: "hashed",
		Role:     "user",
		Enabled:  true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	log := logger.NewNopLogger()
	balanceRepo := repository.NewBalanceRepository(db)
	rechargeRepo := repository.NewBalanceRechargeOrderRepository(db)
	balanceService := balance.NewService(balanceRepo, log).WithRechargeRepository(rechargeRepo)
	paymentService := payment.NewService(nil, log).WithBalanceService(balanceService)
	balanceHandler := NewBalanceHandler(balanceService, log).WithPaymentService(paymentService)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	router.POST("/balance/recharge", balanceHandler.CreateRecharge)

	return router, db
}

func TestCreateRecharge_RejectsUnavailableMethodWithoutCreatingOrder(t *testing.T) {
	router, db := setupBalanceHandlerTest(t)

	body, _ := json.Marshal(map[string]any{
		"amount": 1000,
		"method": "wechat",
	})
	req := httptest.NewRequest(http.MethodPost, "/balance/recharge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var count int64
	if err := db.Model(&repository.BalanceRechargeOrder{}).Count(&count).Error; err != nil {
		t.Fatalf("count recharge orders: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no recharge order to be created, got %d", count)
	}
}

func TestCreateRecharge_CancelsOrderWhenGatewayCreateFails(t *testing.T) {
	router, db := setupBalanceHandlerTest(t)

	log := logger.NewNopLogger()
	balanceRepo := repository.NewBalanceRepository(db)
	rechargeRepo := repository.NewBalanceRechargeOrderRepository(db)
	balanceService := balance.NewService(balanceRepo, log).WithRechargeRepository(rechargeRepo)
	paymentService := payment.NewService(nil, log).WithBalanceService(balanceService)
	paymentService.RegisterGateway(&failingRechargeGateway{name: "wechat"})
	balanceHandler := NewBalanceHandler(balanceService, log).WithPaymentService(paymentService)

	router = gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	router.POST("/balance/recharge", balanceHandler.CreateRecharge)

	body, _ := json.Marshal(map[string]any{
		"amount": 1000,
		"method": "wechat",
	})
	req := httptest.NewRequest(http.MethodPost, "/balance/recharge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var order repository.BalanceRechargeOrder
	if err := db.First(&order).Error; err != nil {
		t.Fatalf("expected recharge order to exist: %v", err)
	}
	if order.Status != repository.BalanceRechargeStatusCancelled {
		t.Fatalf("expected recharge order to be cancelled, got %s", order.Status)
	}
}
