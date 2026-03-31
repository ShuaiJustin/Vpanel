package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/commercial/balance"
	"v/internal/commercial/order"
	"v/internal/commercial/payment"
	"v/internal/database"
	"v/internal/database/repository"
	"v/internal/logger"
)

// setupPaymentTestDB creates an in-memory SQLite database for testing.
func setupPaymentTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	err = db.AutoMigrate(&database.CommercialPlan{}, &database.Order{}, &database.User{})
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// MockPaymentGateway is a mock payment gateway for testing.
type MockPaymentGateway struct {
	name           string
	shouldSucceed  bool
	callbackCalled int
}

func (m *MockPaymentGateway) Name() string { return m.name }

func (m *MockPaymentGateway) CreatePayment(o *payment.PaymentOrder) (*payment.PaymentRequest, error) {
	return &payment.PaymentRequest{
		PaymentURL: "https://mock.payment.com/pay/" + o.OrderNo,
		QRCodeData: "mock_qr_data",
	}, nil
}

func (m *MockPaymentGateway) VerifyCallback(data []byte, signature string) (*payment.PaymentResult, error) {
	m.callbackCalled++
	var callbackData map[string]interface{}
	json.Unmarshal(data, &callbackData)

	orderNo, _ := callbackData["order_no"].(string)
	paymentNo, _ := callbackData["payment_no"].(string)

	return &payment.PaymentResult{
		Success:   m.shouldSucceed,
		OrderNo:   orderNo,
		PaymentNo: paymentNo,
	}, nil
}

func (m *MockPaymentGateway) QueryPayment(paymentNo string) (*payment.PaymentResult, error) {
	return &payment.PaymentResult{
		Success:   m.shouldSucceed,
		PaymentNo: paymentNo,
	}, nil
}

func (m *MockPaymentGateway) Refund(paymentNo string, amount int64, reason string) (*payment.RefundResult, error) {
	return &payment.RefundResult{
		Success:  m.shouldSucceed,
		RefundNo: "REF-" + paymentNo,
	}, nil
}

// setupPaymentTestRouter creates a test router with payment handler.
func setupPaymentTestRouter(db *gorm.DB) (*gin.Engine, *payment.Service, *order.Service) {
	log := logger.NewNopLogger()
	orderRepo := repository.NewOrderRepository(db)
	planRepo := repository.NewPlanRepository(db)
	balanceRepo := repository.NewBalanceRepository(db)

	orderService := order.NewService(orderRepo, planRepo, log, nil)
	balanceService := balance.NewService(balanceRepo, log)
	paymentService := payment.NewService(orderService, log).WithBalanceService(balanceService)

	// Register mock gateway
	mockGateway := &MockPaymentGateway{name: "mock", shouldSucceed: true}
	paymentService.RegisterGateway(mockGateway)

	paymentHandler := NewPaymentHandler(paymentService, log)

	router := gin.New()
	router.POST("/payments/create", paymentHandler.CreatePayment)
	router.POST("/payments/callback/:method", paymentHandler.HandleCallback)
	router.GET("/payments/status/:orderNo", paymentHandler.GetPaymentStatus)

	return router, paymentService, orderService
}

// TestPaymentCreation tests that payment can be created for a pending order.
// **Validates: Requirements 4.5**
func TestPaymentCreation(t *testing.T) {
	db := setupPaymentTestDB(t)
	router, _, orderService := setupPaymentTestRouter(db)

	// Create test data
	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	// Create an order
	ctx := context.Background()
	createdOrder, err := orderService.Create(ctx, &order.CreateOrderRequest{
		UserID: int64(testUser.ID),
		PlanID: int64(testPlan.ID),
	})
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Create payment
	body := map[string]interface{}{
		"order_no": createdOrder.OrderNo,
		"method":   "mock",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	paymentData, ok := response["payment"].(map[string]interface{})
	if !ok {
		t.Fatal("Response should contain payment data")
	}

	if paymentData["payment_url"] == nil || paymentData["payment_url"] == "" {
		t.Error("Payment should have a payment URL")
	}
}

// TestPaymentCallback tests that payment callback updates order status.
// **Validates: Requirements 4.6, 4.7**
func TestPaymentCallback(t *testing.T) {
	db := setupPaymentTestDB(t)
	router, _, orderService := setupPaymentTestRouter(db)

	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	ctx := context.Background()
	createdOrder, _ := orderService.Create(ctx, &order.CreateOrderRequest{
		UserID: int64(testUser.ID),
		PlanID: int64(testPlan.ID),
	})

	// Simulate payment callback
	callbackData := map[string]interface{}{
		"order_no":   createdOrder.OrderNo,
		"payment_no": "PAY-123456",
		"status":     "success",
	}
	jsonBody, _ := json.Marshal(callbackData)

	req := httptest.NewRequest(http.MethodPost, "/payments/callback/mock", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify order status is updated
	updatedOrder, _ := orderService.GetByOrderNo(ctx, createdOrder.OrderNo)
	if updatedOrder.Status != "paid" {
		t.Errorf("Order should be paid after callback, got %s", updatedOrder.Status)
	}
}

// TestPaymentCallbackIdempotency_Property tests that payment callbacks are idempotent.
// *For any* payment callback processed multiple times with the same payment_no,
// the order status and balance SHALL only be updated once.
// **Validates: Requirements 14.8**
func TestPaymentCallbackIdempotency_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 30

	properties := gopter.NewProperties(parameters)

	properties.Property("duplicate callbacks do not change order state", prop.ForAll(
		func(callbackCount int) bool {
			if callbackCount < 2 || callbackCount > 10 {
				return true
			}

			db := setupPaymentTestDB(t)
			router, _, orderService := setupPaymentTestRouter(db)

			testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
			testUser := createOrderTestUser(db, "testuser")

			ctx := context.Background()
			createdOrder, _ := orderService.Create(ctx, &order.CreateOrderRequest{
				UserID: int64(testUser.ID),
				PlanID: int64(testPlan.ID),
			})

			callbackData := map[string]interface{}{
				"order_no":   createdOrder.OrderNo,
				"payment_no": "PAY-IDEMPOTENT-123",
				"status":     "success",
			}
			jsonBody, _ := json.Marshal(callbackData)

			// Send callback multiple times
			for i := 0; i < callbackCount; i++ {
				req := httptest.NewRequest(http.MethodPost, "/payments/callback/mock", bytes.NewBuffer(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}

			// Verify order is still in correct state
			updatedOrder, _ := orderService.GetByOrderNo(ctx, createdOrder.OrderNo)
			return updatedOrder.Status == "paid"
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t)
}

// TestPaymentStatus tests that payment status can be queried.
// **Validates: Requirements 4.6**
func TestPaymentStatus(t *testing.T) {
	db := setupPaymentTestDB(t)
	router, _, orderService := setupPaymentTestRouter(db)

	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	ctx := context.Background()
	createdOrder, _ := orderService.Create(ctx, &order.CreateOrderRequest{
		UserID: int64(testUser.ID),
		PlanID: int64(testPlan.ID),
	})

	// Query payment status
	req := httptest.NewRequest(http.MethodGet, "/payments/status/"+createdOrder.OrderNo, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "pending" {
		t.Errorf("Expected pending status, got %v", response["status"])
	}
}

// TestPaymentInvalidOrder tests that payment creation fails for invalid orders.
// **Validates: Requirements 4.5**
func TestPaymentInvalidOrder(t *testing.T) {
	db := setupPaymentTestDB(t)
	router, _, _ := setupPaymentTestRouter(db)

	// Try to create payment for non-existent order
	body := map[string]interface{}{
		"order_no": "ORD-NONEXISTENT",
		"method":   "mock",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for invalid order, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["message"] != "订单不存在或已失效，请返回订单页重新创建。" {
		t.Errorf("Expected friendly order not found message, got %v", response["message"])
	}
}

// TestPaymentInvalidMethod tests that payment creation fails for invalid payment method.
// **Validates: Requirements 4.1-4.4**
func TestPaymentInvalidMethod(t *testing.T) {
	db := setupPaymentTestDB(t)
	router, _, orderService := setupPaymentTestRouter(db)

	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	ctx := context.Background()
	createdOrder, _ := orderService.Create(ctx, &order.CreateOrderRequest{
		UserID: int64(testUser.ID),
		PlanID: int64(testPlan.ID),
	})

	// Try to create payment with invalid method
	body := map[string]interface{}{
		"order_no": createdOrder.OrderNo,
		"method":   "invalid_method",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid method, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["code"] != "PAYMENT_METHOD_UNAVAILABLE" {
		t.Errorf("Expected PAYMENT_METHOD_UNAVAILABLE code, got %v", response["code"])
	}

	if response["message"] != "当前支付方式暂不可用，请选择其他支付方式。" {
		t.Errorf("Expected friendly invalid method message, got %v", response["message"])
	}
}

func TestBalancePaymentInsufficientBalance(t *testing.T) {
	db := setupPaymentTestDB(t)
	router, _, orderService := setupPaymentTestRouter(db)

	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	ctx := context.Background()
	createdOrder, _ := orderService.Create(ctx, &order.CreateOrderRequest{
		UserID: int64(testUser.ID),
		PlanID: int64(testPlan.ID),
	})

	body := map[string]interface{}{
		"order_no": createdOrder.OrderNo,
		"method":   "balance",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for insufficient balance, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["code"] != "INSUFFICIENT_BALANCE" {
		t.Errorf("Expected INSUFFICIENT_BALANCE code, got %v", response["code"])
	}

	if response["message"] != "余额不足，请先充值后再支付。" {
		t.Errorf("Expected friendly insufficient balance message, got %v", response["message"])
	}
}
