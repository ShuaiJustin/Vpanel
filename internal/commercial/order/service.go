// Package order provides order management functionality.
package order

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	trialsvc "v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrOrderExpired      = errors.New("order has expired")
	ErrOrderAlreadyPaid  = errors.New("order is already paid")
	ErrOrderCannotCancel = errors.New("order cannot be cancelled")
	ErrInvalidOrder      = errors.New("invalid order data")
	ErrPlanNotFound      = errors.New("plan not found")
	ErrPlanInactive      = errors.New("plan is not active")
)

// Order status constants
const (
	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusRefunded  = "refunded"
)

// Order represents an order.
type Order struct {
	ID             int64      `json:"id"`
	OrderNo        string     `json:"order_no"`
	UserID         int64      `json:"user_id"`
	PlanID         int64      `json:"plan_id"`
	CouponID       *int64     `json:"coupon_id"`
	OriginalAmount int64      `json:"original_amount"`
	DiscountAmount int64      `json:"discount_amount"`
	BalanceUsed    int64      `json:"balance_used"`
	PayAmount      int64      `json:"pay_amount"`
	Status         string     `json:"status"`
	PaymentMethod  string     `json:"payment_method"`
	PaymentNo      string     `json:"payment_no"`
	PaidAt         *time.Time `json:"paid_at"`
	ExpiredAt      time.Time  `json:"expired_at"`
	Notes          string     `json:"notes"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateOrderRequest represents a request to create an order.
type CreateOrderRequest struct {
	UserID     int64  `json:"user_id"`
	PlanID     int64  `json:"plan_id"`
	CouponCode string `json:"coupon_code"`
}

// OrderFilter defines filter options for listing orders.
type OrderFilter struct {
	UserID        *int64
	Search        string
	Status        string
	PaymentMethod string
	StartDate     *time.Time
	EndDate       *time.Time
}

// Config holds order service configuration.
type Config struct {
	OrderExpiration time.Duration // Default: 30 minutes
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		OrderExpiration: 30 * time.Minute,
	}
}

// Service provides order management operations.
type Service struct {
	orderRepo repository.OrderRepository
	planRepo  repository.PlanRepository
	userRepo  repository.UserRepository
	trialRepo trialMarker
	logger    logger.Logger
	config    *Config
	mu        sync.Mutex
	orderNos  map[string]bool // Track generated order numbers for uniqueness
}

type trialMarker interface {
	MarkConverted(ctx context.Context, userID int64) error
}

// NewService creates a new order service.
func NewService(
	orderRepo repository.OrderRepository,
	planRepo repository.PlanRepository,
	log logger.Logger,
	config *Config,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		orderRepo: orderRepo,
		planRepo:  planRepo,
		logger:    log,
		config:    config,
		orderNos:  make(map[string]bool),
	}
}

// WithUserRepository enables plan activation on successful payment.
func (s *Service) WithUserRepository(userRepo repository.UserRepository) *Service {
	s.userRepo = userRepo
	return s
}

// WithTrialMarker marks active trials as converted when a paid plan is applied.
func (s *Service) WithTrialMarker(trialRepo trialMarker) *Service {
	s.trialRepo = trialRepo
	return s
}

// GenerateOrderNo generates a unique order number.
// Format: ORD-YYYYMMDD-XXXX where XXXX is a random hex string.
func (s *Service) GenerateOrderNo() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		// Generate random bytes
		bytes := make([]byte, 4)
		rand.Read(bytes)
		randomPart := hex.EncodeToString(bytes)

		// Format: ORD-20260114-XXXXXXXX
		orderNo := fmt.Sprintf("ORD-%s-%s",
			time.Now().Format("20060102"),
			randomPart[:8])

		// Check uniqueness in memory cache
		if !s.orderNos[orderNo] {
			s.orderNos[orderNo] = true
			return orderNo
		}
	}
}

// Create creates a new order.
func (s *Service) Create(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
	if req.UserID <= 0 {
		return nil, fmt.Errorf("%w: user ID is required", ErrInvalidOrder)
	}
	if req.PlanID <= 0 {
		return nil, fmt.Errorf("%w: plan ID is required", ErrInvalidOrder)
	}

	// Get plan
	plan, err := s.planRepo.GetByID(ctx, req.PlanID)
	if err != nil {
		return nil, ErrPlanNotFound
	}
	if !plan.IsActive {
		return nil, ErrPlanInactive
	}

	// Generate order number
	orderNo := s.GenerateOrderNo()

	// Calculate amounts
	originalAmount := plan.Price
	discountAmount := int64(0)
	balanceUsed := int64(0)
	payAmount := originalAmount - discountAmount - balanceUsed

	// Create order
	repoOrder := &repository.Order{
		OrderNo:        orderNo,
		UserID:         req.UserID,
		PlanID:         req.PlanID,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		BalanceUsed:    balanceUsed,
		PayAmount:      payAmount,
		Status:         StatusPending,
		ExpiredAt:      time.Now().Add(s.config.OrderExpiration),
	}

	if err := s.orderRepo.Create(ctx, repoOrder); err != nil {
		s.logger.Error("Failed to create order", logger.Err(err))
		return nil, err
	}

	return s.toOrder(repoOrder), nil
}

// GetByID retrieves an order by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*Order, error) {
	repoOrder, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	return s.toOrder(repoOrder), nil
}

// GetByOrderNo retrieves an order by order number.
func (s *Service) GetByOrderNo(ctx context.Context, orderNo string) (*Order, error) {
	repoOrder, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	return s.toOrder(repoOrder), nil
}

// ListByUser lists orders for a user.
func (s *Service) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]*Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoOrders, total, err := s.orderRepo.ListByUser(ctx, userID, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list orders", logger.Err(err))
		return nil, 0, err
	}

	orders := make([]*Order, len(repoOrders))
	for i, ro := range repoOrders {
		orders[i] = s.toOrder(ro)
	}

	return orders, total, nil
}

// List lists all orders with filter.
func (s *Service) List(ctx context.Context, filter OrderFilter, page, pageSize int) ([]*Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoFilter := repository.OrderFilter{
		UserID:        filter.UserID,
		Search:        filter.Search,
		Status:        filter.Status,
		PaymentMethod: filter.PaymentMethod,
		StartDate:     filter.StartDate,
		EndDate:       filter.EndDate,
	}
	repoOrders, total, err := s.orderRepo.List(ctx, repoFilter, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list orders", logger.Err(err))
		return nil, 0, err
	}

	orders := make([]*Order, len(repoOrders))
	for i, ro := range repoOrders {
		orders[i] = s.toOrder(ro)
	}

	return orders, total, nil
}

// Cancel cancels a pending order.
func (s *Service) Cancel(ctx context.Context, id int64) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return ErrOrderNotFound
	}

	if order.Status != StatusPending {
		return ErrOrderCannotCancel
	}

	if err := s.orderRepo.UpdateStatus(ctx, id, StatusCancelled); err != nil {
		s.logger.Error("Failed to cancel order", logger.Err(err), logger.F("id", id))
		return err
	}

	return nil
}

// MarkPaid marks an order as paid.
func (s *Service) MarkPaid(ctx context.Context, orderNo string, paymentNo string) error {
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return ErrOrderNotFound
	}

	if order.Status != StatusPending {
		return ErrOrderAlreadyPaid
	}

	if time.Now().After(order.ExpiredAt) {
		return ErrOrderExpired
	}

	if err := s.orderRepo.MarkPaid(ctx, order.ID, paymentNo, time.Now()); err != nil {
		s.logger.Error("Failed to mark order as paid", logger.Err(err), logger.F("orderNo", orderNo))
		return err
	}

	if err := s.applyPlanToUser(ctx, order); err != nil {
		s.logger.Error("Failed to apply plan after payment", logger.Err(err), logger.F("orderNo", orderNo), logger.F("user_id", order.UserID))
		return err
	}

	return nil
}

// Complete marks an order as completed.
func (s *Service) Complete(ctx context.Context, id int64) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return ErrOrderNotFound
	}

	if order.Status != StatusPaid {
		return fmt.Errorf("order must be paid before completing")
	}

	if err := s.orderRepo.UpdateStatus(ctx, id, StatusCompleted); err != nil {
		s.logger.Error("Failed to complete order", logger.Err(err), logger.F("id", id))
		return err
	}

	return nil
}

// ExpirePendingOrders cancels all expired pending orders.
func (s *Service) ExpirePendingOrders(ctx context.Context) (int64, error) {
	count, err := s.orderRepo.CancelExpired(ctx)
	if err != nil {
		s.logger.Error("Failed to expire pending orders", logger.Err(err))
		return 0, err
	}

	if count > 0 {
		s.logger.Info("Expired pending orders", logger.F("count", count))
	}

	return count, nil
}

// UpdateStatus updates the status of an order.
func (s *Service) UpdateStatus(ctx context.Context, id int64, status string) error {
	// Validate status transition
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return ErrOrderNotFound
	}

	if !s.isValidStatusTransition(order.Status, status) {
		return fmt.Errorf("invalid status transition from %s to %s", order.Status, status)
	}

	if err := s.orderRepo.UpdateStatus(ctx, id, status); err != nil {
		s.logger.Error("Failed to update order status", logger.Err(err), logger.F("id", id), logger.F("status", status))
		return err
	}

	return nil
}

// isValidStatusTransition checks if a status transition is valid.
// Valid transitions:
// - pending -> paid, cancelled
// - paid -> completed, refunded
// - completed -> refunded
func (s *Service) isValidStatusTransition(from, to string) bool {
	validTransitions := map[string][]string{
		StatusPending:   {StatusPaid, StatusCancelled},
		StatusPaid:      {StatusCompleted, StatusRefunded},
		StatusCompleted: {StatusRefunded},
		StatusCancelled: {},
		StatusRefunded:  {},
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// GetRevenueByDateRange returns total revenue for a date range.
func (s *Service) GetRevenueByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	return s.orderRepo.GetRevenueByDateRange(ctx, start, end)
}

// GetOrderCountByDateRange returns order count for a date range.
func (s *Service) GetOrderCountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	return s.orderRepo.GetOrderCountByDateRange(ctx, start, end)
}

// GetOrderCount returns total order count.
func (s *Service) GetOrderCount(ctx context.Context) (int64, error) {
	return s.orderRepo.Count(ctx)
}

// GetOrderCountByStatus returns order count by status.
func (s *Service) GetOrderCountByStatus(ctx context.Context, status string) (int64, error) {
	return s.orderRepo.CountByStatus(ctx, status)
}

// toOrder converts a repository order to a service order.
func (s *Service) toOrder(ro *repository.Order) *Order {
	return &Order{
		ID:             ro.ID,
		OrderNo:        ro.OrderNo,
		UserID:         ro.UserID,
		PlanID:         ro.PlanID,
		CouponID:       ro.CouponID,
		OriginalAmount: ro.OriginalAmount,
		DiscountAmount: ro.DiscountAmount,
		BalanceUsed:    ro.BalanceUsed,
		PayAmount:      ro.PayAmount,
		Status:         ro.Status,
		PaymentMethod:  ro.PaymentMethod,
		PaymentNo:      ro.PaymentNo,
		PaidAt:         ro.PaidAt,
		ExpiredAt:      ro.ExpiredAt,
		Notes:          ro.Notes,
		CreatedAt:      ro.CreatedAt,
		UpdatedAt:      ro.UpdatedAt,
	}
}

func (s *Service) applyPlanToUser(ctx context.Context, ord *repository.Order) error {
	if s.userRepo == nil {
		return nil
	}

	plan, err := s.planRepo.GetByID(ctx, ord.PlanID)
	if err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, ord.UserID)
	if err != nil {
		return err
	}

	now := time.Now()
	baseExpireAt := now
	if user.ExpiresAt != nil && user.ExpiresAt.After(now) {
		baseExpireAt = *user.ExpiresAt
	}
	newExpireAt := baseExpireAt.AddDate(0, 0, plan.Duration)

	user.Enabled = true
	user.TrafficUsed = 0
	user.TrafficLimit = plan.TrafficLimit
	user.ExpiresAt = &newExpireAt

	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}
	if s.trialRepo != nil {
		if err := s.trialRepo.MarkConverted(ctx, ord.UserID); err != nil {
			if !errors.Is(err, trialsvc.ErrTrialNotFound) {
				s.logger.Warn("failed to mark trial converted after plan activation",
					logger.Err(err),
					logger.UserID(ord.UserID),
					logger.F("order_id", ord.ID),
				)
			}
		}
	}
	return nil
}
