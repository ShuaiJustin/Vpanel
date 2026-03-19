package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/commercial/order"
	"v/internal/database"
	"v/internal/database/repository"
	"v/internal/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupOrderTestDB creates an in-memory SQLite database for testing.
func setupOrderTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate models
	err = db.AutoMigrate(&database.CommercialPlan{}, &database.Order{}, &database.User{})
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// createOrderTestPlan creates a test plan in the database.
func createOrderTestPlan(db *gorm.DB, name string, price int64, duration int) *database.CommercialPlan {
	p := &database.CommercialPlan{
		Name:     name,
		Price:    price,
		Duration: duration,
		IsActive: true,
	}
	db.Create(p)
	return p
}

// createOrderTestUser creates a test user in the database.
func createOrderTestUser(db *gorm.DB, username string) *database.User {
	u := &database.User{
		Username: username,
		Email:    username + "@test.com",
		Password: "hashed_password",
		Role:     "user",
		Enabled:  true,
	}
	db.Create(u)
	return u
}

// setupOrderTestRouter creates a test router with order handler.
func setupOrderTestRouter(db *gorm.DB) (*gin.Engine, *order.Service) {
	log := logger.NewNopLogger()
	orderRepo := repository.NewOrderRepository(db)
	planRepo := repository.NewPlanRepository(db)

	orderService := order.NewService(orderRepo, planRepo, log, nil)
	orderHandler := NewOrderHandler(orderService, log)

	router := gin.New()

	// Mock auth middleware that sets userID
	router.Use(func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-ID")
		if userIDStr != "" {
			var id int64
			fmt.Sscanf(userIDStr, "%d", &id)
			c.Set("userID", id)
			c.Set("user_id", id)
		}
		role := c.GetHeader("X-User-Role")
		if role != "" {
			c.Set("role", role)
		}
		c.Next()
	})

	router.POST("/orders", orderHandler.CreateOrder)
	router.GET("/orders", orderHandler.ListUserOrders)
	router.GET("/orders/:id", orderHandler.GetOrder)
	router.POST("/orders/:id/cancel", orderHandler.CancelOrder)
	router.GET("/admin/orders", orderHandler.ListAllOrders)
	router.GET("/admin/orders/:id", orderHandler.GetOrder)
	router.PUT("/admin/orders/:id/status", orderHandler.UpdateOrderStatus)

	return router, orderService
}

// TestOrderCreation_ValidPlan tests that creating an order with a valid plan succeeds.
// **Validates: Requirements 3.1, 3.2**
func TestOrderCreation_ValidPlan(t *testing.T) {
	db := setupOrderTestDB(t)
	router, _ := setupOrderTestRouter(db)

	// Create test data
	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	// Create order request
	body := map[string]interface{}{
		"plan_id": testPlan.ID,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", testUser.ID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	orderData, ok := response["order"].(map[string]interface{})
	if !ok {
		t.Fatal("Response should contain order data")
	}

	if orderData["order_no"] == nil || orderData["order_no"] == "" {
		t.Error("Order should have an order number")
	}

	if orderData["status"] != "pending" {
		t.Errorf("New order should have pending status, got %v", orderData["status"])
	}
}

// TestOrderCreation_InvalidPlan tests that creating an order with an invalid plan fails.
// **Validates: Requirements 3.1**
func TestOrderCreation_InvalidPlan(t *testing.T) {
	db := setupOrderTestDB(t)
	router, _ := setupOrderTestRouter(db)

	testUser := createOrderTestUser(db, "testuser")

	// Create order with non-existent plan
	body := map[string]interface{}{
		"plan_id": 99999,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", string(rune(testUser.ID)))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid plan, got %d", w.Code)
	}
}

// TestOrderCancellation_PendingOrder tests that pending orders can be cancelled.
// **Validates: Requirements 5.5**
func TestOrderCancellation_PendingOrder(t *testing.T) {
	db := setupOrderTestDB(t)
	router, orderService := setupOrderTestRouter(db)

	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	// Create an order first
	ctx := context.Background()
	createdOrder, err := orderService.Create(ctx, &order.CreateOrderRequest{
		UserID: int64(testUser.ID),
		PlanID: int64(testPlan.ID),
	})
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Cancel the order
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/orders/%d/cancel", createdOrder.ID), nil)
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", testUser.ID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify order is cancelled
	updatedOrder, _ := orderService.GetByID(ctx, createdOrder.ID)
	if updatedOrder.Status != "cancelled" {
		t.Errorf("Order should be cancelled, got %s", updatedOrder.Status)
	}
}

func TestAdminGetOrder_UsesAdminRoute(t *testing.T) {
	db := setupOrderTestDB(t)
	router, orderService := setupOrderTestRouter(db)

	testPlan := createOrderTestPlan(db, "Admin Detail Plan", 2000, 30)
	testUser := createOrderTestUser(db, "adminorderdetail")

	createdOrder, err := orderService.Create(context.Background(), &order.CreateOrderRequest{
		UserID: int64(testUser.ID),
		PlanID: int64(testPlan.ID),
	})
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/orders/%d", createdOrder.ID), nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["order"]["order_no"] != createdOrder.OrderNo {
		t.Fatalf("Expected order_no %s, got %v", createdOrder.OrderNo, response["order"]["order_no"])
	}
}

func TestListAllOrders_AppliesSearchAndDateFilter(t *testing.T) {
	db := setupOrderTestDB(t)
	router, orderService := setupOrderTestRouter(db)

	testPlan := createOrderTestPlan(db, "Search Plan", 3000, 30)
	firstUser := createOrderTestUser(db, "searchuserone")
	secondUser := createOrderTestUser(db, "searchusertwo")

	firstOrder, err := orderService.Create(context.Background(), &order.CreateOrderRequest{
		UserID: int64(firstUser.ID),
		PlanID: int64(testPlan.ID),
	})
	if err != nil {
		t.Fatalf("Failed to create first order: %v", err)
	}
	if _, err := orderService.Create(context.Background(), &order.CreateOrderRequest{
		UserID: int64(secondUser.ID),
		PlanID: int64(testPlan.ID),
	}); err != nil {
		t.Fatalf("Failed to create second order: %v", err)
	}

	startDate := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	endDate := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	requestURL := fmt.Sprintf(
		"/admin/orders?search=%d&start_date=%s&end_date=%s",
		firstUser.ID,
		url.QueryEscape(startDate),
		url.QueryEscape(endDate),
	)
	req := httptest.NewRequest(http.MethodGet, requestURL, nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Orders []map[string]interface{} `json:"orders"`
		Total  int64                    `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Total != 1 || len(response.Orders) != 1 {
		t.Fatalf("Expected exactly 1 filtered order, got total=%d len=%d body=%s", response.Total, len(response.Orders), w.Body.String())
	}

	if response.Orders[0]["order_no"] != firstOrder.OrderNo {
		t.Fatalf("Expected filtered order %s, got %v", firstOrder.OrderNo, response.Orders[0]["order_no"])
	}
}

// TestOrderFlow_Property tests the complete order flow property.
// *For any* valid plan and user, creating an order should result in a pending order
// with correct amount and expiration time.
// **Validates: Requirements 3.1-3.3, 3.7**
func TestOrderFlow_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	properties.Property("order creation sets correct initial state", prop.ForAll(
		func(planPrice int64, planDuration int) bool {
			if planPrice <= 0 || planDuration <= 0 {
				return true // Skip invalid inputs
			}

			db := setupOrderTestDB(t)
			log := logger.NewNopLogger()
			orderRepo := repository.NewOrderRepository(db)
			planRepo := repository.NewPlanRepository(db)
			orderService := order.NewService(orderRepo, planRepo, log, nil)

			// Create test plan
			testPlan := createOrderTestPlan(db, "Test Plan", planPrice, planDuration)
			testUser := createOrderTestUser(db, "testuser")

			// Create order
			ctx := context.Background()
			createdOrder, err := orderService.Create(ctx, &order.CreateOrderRequest{
				UserID: int64(testUser.ID),
				PlanID: int64(testPlan.ID),
			})
			if err != nil {
				return false
			}

			// Verify order properties
			if createdOrder.Status != "pending" {
				return false
			}
			if createdOrder.OriginalAmount != planPrice {
				return false
			}
			if createdOrder.PayAmount != planPrice {
				return false
			}
			if createdOrder.ExpiredAt.Before(time.Now()) {
				return false
			}
			if createdOrder.OrderNo == "" {
				return false
			}

			return true
		},
		gen.Int64Range(100, 100000), // price in cents
		gen.IntRange(1, 365),        // duration in days
	))

	properties.TestingRun(t)
}

// TestOrderStatusTransitions_Property tests that order status transitions follow valid paths.
// *For any* order, status transitions SHALL follow: pending → paid → completed,
// pending → cancelled, paid → refunded.
// **Validates: Requirements 5.4**
func TestOrderStatusTransitions_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	// Valid transitions from pending
	validFromPending := map[string]bool{
		"paid":      true,
		"cancelled": true,
	}

	// Valid transitions from paid
	validFromPaid := map[string]bool{
		"completed": true,
		"refunded":  true,
	}

	properties.Property("pending orders can only transition to paid or cancelled", prop.ForAll(
		func(targetStatus string) bool {
			db := setupOrderTestDB(t)
			log := logger.NewNopLogger()
			orderRepo := repository.NewOrderRepository(db)
			planRepo := repository.NewPlanRepository(db)
			orderService := order.NewService(orderRepo, planRepo, log, nil)

			testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
			testUser := createOrderTestUser(db, "testuser")

			ctx := context.Background()
			createdOrder, _ := orderService.Create(ctx, &order.CreateOrderRequest{
				UserID: int64(testUser.ID),
				PlanID: int64(testPlan.ID),
			})

			err := orderService.UpdateStatus(ctx, createdOrder.ID, targetStatus)

			if validFromPending[targetStatus] {
				return err == nil
			}
			return err != nil
		},
		gen.OneConstOf("paid", "cancelled", "completed", "refunded"),
	))

	properties.Property("paid orders can only transition to completed or refunded", prop.ForAll(
		func(targetStatus string) bool {
			db := setupOrderTestDB(t)
			log := logger.NewNopLogger()
			orderRepo := repository.NewOrderRepository(db)
			planRepo := repository.NewPlanRepository(db)
			orderService := order.NewService(orderRepo, planRepo, log, nil)

			testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
			testUser := createOrderTestUser(db, "testuser")

			ctx := context.Background()
			createdOrder, _ := orderService.Create(ctx, &order.CreateOrderRequest{
				UserID: int64(testUser.ID),
				PlanID: int64(testPlan.ID),
			})

			// First transition to paid
			orderService.UpdateStatus(ctx, createdOrder.ID, "paid")

			// Then try to transition to target status
			err := orderService.UpdateStatus(ctx, createdOrder.ID, targetStatus)

			if validFromPaid[targetStatus] {
				return err == nil
			}
			return err != nil
		},
		gen.OneConstOf("paid", "cancelled", "completed", "refunded"),
	))

	properties.TestingRun(t)
}

// TestOrderListPagination tests that order listing returns correct pagination.
// **Validates: Requirements 5.1, 5.2**
func TestOrderListPagination(t *testing.T) {
	db := setupOrderTestDB(t)
	log := logger.NewNopLogger()
	orderRepo := repository.NewOrderRepository(db)
	planRepo := repository.NewPlanRepository(db)
	orderService := order.NewService(orderRepo, planRepo, log, nil)

	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	testUser := createOrderTestUser(db, "testuser")

	// Create multiple orders
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		orderService.Create(ctx, &order.CreateOrderRequest{
			UserID: int64(testUser.ID),
			PlanID: int64(testPlan.ID),
		})
	}

	// Test pagination
	orders, total, err := orderService.ListByUser(ctx, int64(testUser.ID), 1, 10)
	if err != nil {
		t.Fatalf("Failed to list orders: %v", err)
	}

	if total != 25 {
		t.Errorf("Expected total 25, got %d", total)
	}

	if len(orders) != 10 {
		t.Errorf("Expected 10 orders on first page, got %d", len(orders))
	}

	// Test second page
	orders2, _, _ := orderService.ListByUser(ctx, int64(testUser.ID), 2, 10)
	if len(orders2) != 10 {
		t.Errorf("Expected 10 orders on second page, got %d", len(orders2))
	}

	// Test third page
	orders3, _, _ := orderService.ListByUser(ctx, int64(testUser.ID), 3, 10)
	if len(orders3) != 5 {
		t.Errorf("Expected 5 orders on third page, got %d", len(orders3))
	}
}

// TestOrderAccessControl tests that users can only access their own orders.
// **Validates: Requirements 5.3**
func TestOrderAccessControl(t *testing.T) {
	db := setupOrderTestDB(t)
	log := logger.NewNopLogger()
	orderRepo := repository.NewOrderRepository(db)
	planRepo := repository.NewPlanRepository(db)
	orderService := order.NewService(orderRepo, planRepo, log, nil)

	testPlan := createOrderTestPlan(db, "Test Plan", 1000, 30)
	user1 := createOrderTestUser(db, "user1")
	user2 := createOrderTestUser(db, "user2")

	// Create order for user1
	ctx := context.Background()
	order1, _ := orderService.Create(ctx, &order.CreateOrderRequest{
		UserID: int64(user1.ID),
		PlanID: int64(testPlan.ID),
	})

	// User1 should see their order
	orders1, _, _ := orderService.ListByUser(ctx, int64(user1.ID), 1, 10)
	if len(orders1) != 1 {
		t.Errorf("User1 should see 1 order, got %d", len(orders1))
	}

	// User2 should not see user1's order
	orders2, _, _ := orderService.ListByUser(ctx, int64(user2.ID), 1, 10)
	if len(orders2) != 0 {
		t.Errorf("User2 should see 0 orders, got %d", len(orders2))
	}

	// Verify order belongs to user1
	if order1.UserID != int64(user1.ID) {
		t.Errorf("Order should belong to user1")
	}
}
