// Package refund provides refund management functionality.
package refund

import (
	"context"
	"errors"
	"gorm.io/gorm"

	"v/internal/commercial/balance"
	"v/internal/commercial/commission"
	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrOrderNotFound       = errors.New("order not found")
	ErrOrderNotRefundable  = errors.New("order is not refundable")
	ErrInvalidRefundAmount = errors.New("invalid refund amount")
	ErrRefundExceedsAmount = errors.New("refund amount exceeds order amount")
	ErrAlreadyRefunded     = errors.New("order already refunded")
)

// Order status constants
const (
	StatusPaid      = "paid"
	StatusCompleted = "completed"
	StatusRefunded  = "refunded"
)

// RefundRequest represents a refund request.
type RefundRequest struct {
	OrderID int64  `json:"order_id"`
	Amount  int64  `json:"amount"` // 0 = full refund
	Reason  string `json:"reason"`
}

// RefundResult represents the result of a refund operation.
type RefundResult struct {
	OrderID          int64  `json:"order_id"`
	RefundAmount     int64  `json:"refund_amount"`
	BalanceRestored  int64  `json:"balance_restored"`
	CommissionCancel int64  `json:"commission_cancelled"`
	Status           string `json:"status"`
}

// Service provides refund management operations.
type Service struct {
	orderRepo         repository.OrderRepository
	balanceService    *balance.Service
	commissionService *commission.Service
	logger            logger.Logger
}

// NewService creates a new refund service.
func NewService(
	orderRepo repository.OrderRepository,
	balanceService *balance.Service,
	commissionService *commission.Service,
	log logger.Logger,
) *Service {
	return &Service{
		orderRepo:         orderRepo,
		balanceService:    balanceService,
		commissionService: commissionService,
		logger:            log,
	}
}

// ProcessRefund processes a refund for an order.
func (s *Service) ProcessRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	if req == nil || req.OrderID <= 0 || req.Amount < 0 {
		return nil, ErrInvalidRefundAmount
	}
	repo, ok := s.orderRepo.(repository.AtomicOrderRepository)
	if !ok {
		return nil, errors.New("atomic order repository is required")
	}
	ord, err := repo.RefundToBalance(ctx, req.OrderID, req.Amount, req.Reason)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrderNotFound
	}
	if errors.Is(err, repository.ErrCommercialState) {
		return nil, ErrOrderNotRefundable
	}
	if errors.Is(err, gorm.ErrInvalidData) {
		return nil, ErrInvalidRefundAmount
	}
	if err != nil {
		return nil, err
	}
	return &RefundResult{OrderID: ord.ID, RefundAmount: ord.RefundedAmount, BalanceRestored: ord.RefundedAmount, Status: ord.Status}, nil
}

// ProcessPartialRefund processes a partial refund for an order.
func (s *Service) ProcessPartialRefund(ctx context.Context, orderID int64, amount int64, reason string) (*RefundResult, error) {
	if amount <= 0 {
		return nil, ErrInvalidRefundAmount
	}

	return s.ProcessRefund(ctx, &RefundRequest{
		OrderID: orderID,
		Amount:  amount,
		Reason:  reason,
	})
}

// ProcessFullRefund processes a full refund for an order.
func (s *Service) ProcessFullRefund(ctx context.Context, orderID int64, reason string) (*RefundResult, error) {
	return s.ProcessRefund(ctx, &RefundRequest{
		OrderID: orderID,
		Amount:  0, // 0 means full refund
		Reason:  reason,
	})
}

// isRefundable checks if an order status allows refund.
func (s *Service) isRefundable(status string) bool {
	return status == StatusPaid || status == StatusCompleted
}

// CanRefund checks if an order can be refunded.
func (s *Service) CanRefund(ctx context.Context, orderID int64) (bool, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return false, ErrOrderNotFound
	}

	return s.isRefundable(order.Status), nil
}

// GetMaxRefundAmount returns the maximum refundable amount for an order.
func (s *Service) GetMaxRefundAmount(ctx context.Context, orderID int64) (int64, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return 0, ErrOrderNotFound
	}

	if !s.isRefundable(order.Status) {
		return 0, ErrOrderNotRefundable
	}

	return order.PayAmount + order.BalanceUsed, nil
}
