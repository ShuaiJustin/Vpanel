// Package payment provides payment gateway functionality.
package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"v/internal/commercial/balance"
	"v/internal/commercial/order"
	"v/internal/logger"
)

// Common errors
var (
	ErrGatewayNotFound   = errors.New("payment gateway not found")
	ErrOrderNotFound     = errors.New("order not found")
	ErrOrderNotPending   = errors.New("order is not pending")
	ErrPaymentFailed     = errors.New("payment failed")
	ErrRefundFailed      = errors.New("refund failed")
	ErrDuplicateCallback = errors.New("duplicate callback")
	ErrInvalidCallback   = errors.New("invalid payment callback")
	ErrAmountMismatch    = errors.New("payment amount mismatch")
)

// RechargeCallbackHandler provides recharge payment callback handling.
type RechargeCallbackHandler interface {
	GetRechargePaymentDetails(ctx context.Context, orderNo string) (amount int64, paymentNo string, status string, err error)
	MarkRechargePaid(ctx context.Context, orderNo string, paymentNo string, paidAt time.Time) error
}

// Service provides payment management operations.
type Service struct {
	gateways        map[string]PaymentGateway
	orderService    *order.Service
	balanceSvc      *balance.Service
	rechargeHandler RechargeCallbackHandler
	logger          logger.Logger
	mu              sync.RWMutex

	processedCallbacks map[string]bool
	callbackMu         sync.Mutex
}

// NewService creates a new payment service.
func NewService(orderService *order.Service, log logger.Logger) *Service {
	return &Service{
		gateways:           make(map[string]PaymentGateway),
		orderService:       orderService,
		logger:             log,
		processedCallbacks: make(map[string]bool),
	}
}

// WithBalanceService enables direct balance payments without an external gateway.
func (s *Service) WithBalanceService(balanceSvc *balance.Service) *Service {
	s.balanceSvc = balanceSvc
	return s
}

// WithRechargeHandler enables recharge callback handling for non-order payments.
func (s *Service) WithRechargeHandler(handler RechargeCallbackHandler) *Service {
	s.rechargeHandler = handler
	return s
}

// RegisterGateway registers a payment gateway.
func (s *Service) RegisterGateway(gateway PaymentGateway) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gateways[gateway.Name()] = gateway
	s.logger.Info("Registered payment gateway", logger.F("name", gateway.Name()))
}

// ReplaceGateways replaces all externally registered gateways.
func (s *Service) ReplaceGateways(gateways map[string]PaymentGateway) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gateways = make(map[string]PaymentGateway, len(gateways))
	for name, gateway := range gateways {
		s.gateways[name] = gateway
	}
}

// GetGateway returns a payment gateway by name.
func (s *Service) GetGateway(name string) (PaymentGateway, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	gateway, ok := s.gateways[name]
	if !ok {
		return nil, ErrGatewayNotFound
	}
	return gateway, nil
}

// ListGateways returns all registered gateway names.
func (s *Service) ListGateways() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.gateways))
	for name := range s.gateways {
		names = append(names, name)
	}
	if s.balanceSvc != nil {
		names = append(names, "balance")
	}
	return names
}

// CreatePayment creates a payment for an order.
func (s *Service) CreatePayment(ctx context.Context, orderNo string, method string, clientIP string) (*PaymentRequest, error) {
	ord, err := s.orderService.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	if ord.Status != order.StatusPending {
		return nil, ErrOrderNotPending
	}

	if method == "balance" {
		return s.createBalancePayment(ctx, ord)
	}

	paymentOrder := &PaymentOrder{
		OrderNo:     ord.OrderNo,
		Amount:      ord.PayAmount,
		Subject:     fmt.Sprintf("Order %s", ord.OrderNo),
		Description: fmt.Sprintf("Payment for order %s", ord.OrderNo),
		ClientIP:    clientIP,
	}

	return s.CreateGatewayPayment(method, paymentOrder)
}

// CreateGatewayPayment creates an external gateway payment for a generic payment order.
func (s *Service) CreateGatewayPayment(method string, paymentOrder *PaymentOrder) (*PaymentRequest, error) {
	if paymentOrder == nil || strings.TrimSpace(paymentOrder.OrderNo) == "" || paymentOrder.Amount <= 0 {
		return nil, fmt.Errorf("%w: invalid payment order", ErrPaymentFailed)
	}
	if strings.TrimSpace(method) == "balance" {
		return nil, ErrGatewayNotFound
	}

	gateway, err := s.GetGateway(method)
	if err != nil {
		return nil, err
	}

	requestOrder := *paymentOrder
	if requestOrder.ClientIP == "" {
		requestOrder.ClientIP = "127.0.0.1"
	}

	request, err := gateway.CreatePayment(&requestOrder)
	if err != nil {
		s.logger.Error("Failed to create payment",
			logger.Err(err),
			logger.F("orderNo", requestOrder.OrderNo),
			logger.F("method", method))
		return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}

	s.logger.Info("Payment created",
		logger.F("orderNo", requestOrder.OrderNo),
		logger.F("method", method))

	return request, nil
}

func (s *Service) createBalancePayment(ctx context.Context, ord *order.Order) (*PaymentRequest, error) {
	if s.balanceSvc == nil {
		return nil, ErrGatewayNotFound
	}

	if err := s.balanceSvc.Deduct(ctx, ord.UserID, ord.PayAmount, &ord.ID, fmt.Sprintf("Balance payment for order %s", ord.OrderNo)); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}

	paymentNo := fmt.Sprintf("BALANCE-%d", time.Now().UnixNano())
	if err := s.orderService.MarkPaid(ctx, ord.OrderNo, paymentNo); err != nil {
		_ = s.balanceSvc.Refund(ctx, ord.UserID, ord.PayAmount, &ord.ID, "Rollback failed balance payment")
		return nil, err
	}

	if updatedOrder, err := s.orderService.GetByOrderNo(ctx, ord.OrderNo); err == nil {
		_ = s.orderService.Complete(ctx, updatedOrder.ID)
	}

	s.logger.Info("Balance payment completed",
		logger.F("orderNo", ord.OrderNo),
		logger.F("user_id", ord.UserID),
		logger.F("amount", ord.PayAmount))

	return &PaymentRequest{
		ExpireTime: time.Now(),
		Extra: map[string]string{
			"method":     "balance",
			"payment_no": paymentNo,
			"status":     order.StatusCompleted,
		},
	}, nil
}

// HandleCallback handles a payment callback.
func (s *Service) HandleCallback(ctx context.Context, method string, data []byte, signature string) error {
	gateway, err := s.GetGateway(method)
	if err != nil {
		return err
	}

	result, err := gateway.VerifyCallback(data, signature)
	if err != nil {
		s.logger.Error("Failed to verify callback",
			logger.Err(err),
			logger.F("method", method))
		return err
	}

	if !result.Success {
		s.logger.Warn("Payment callback indicates failure",
			logger.F("orderNo", result.OrderNo),
			logger.F("error", result.Error))
		return nil
	}

	orderNo := strings.TrimSpace(result.OrderNo)
	paymentNo := strings.TrimSpace(result.PaymentNo)
	if orderNo == "" || paymentNo == "" {
		return ErrInvalidCallback
	}

	callbackKey := fmt.Sprintf("%s:%s", method, paymentNo)

	ord, err := s.orderService.GetByOrderNo(ctx, orderNo)
	if err == nil {
		if result.Amount > 0 && ord.PayAmount != result.Amount {
			s.logger.Warn("Payment callback amount mismatch",
				logger.F("orderNo", orderNo),
				logger.F("expected", ord.PayAmount),
				logger.F("actual", result.Amount))
			return ErrAmountMismatch
		}

		if ord.PaymentNo == paymentNo && (ord.Status == order.StatusPaid || ord.Status == order.StatusCompleted) {
			s.logger.Info("Duplicate callback ignored after persisted payment lookup",
				logger.F("orderNo", orderNo),
				logger.F("paymentNo", paymentNo))
			return nil
		}

		if s.markCallbackProcessing(callbackKey, paymentNo) {
			return nil
		}

		if err := s.orderService.MarkPaid(ctx, orderNo, paymentNo); err != nil {
			s.logger.Error("Failed to mark order as paid",
				logger.Err(err),
				logger.F("orderNo", orderNo))
			s.unmarkCallbackProcessing(callbackKey)
			return err
		}

		s.logger.Info("Payment callback processed",
			logger.F("orderNo", orderNo),
			logger.F("paymentNo", paymentNo),
			logger.F("amount", result.Amount))
		return nil
	}

	if s.rechargeHandler == nil {
		return ErrOrderNotFound
	}

	amount, persistedPaymentNo, status, rechargeErr := s.rechargeHandler.GetRechargePaymentDetails(ctx, orderNo)
	if rechargeErr != nil {
		return ErrOrderNotFound
	}

	if result.Amount > 0 && amount != result.Amount {
		s.logger.Warn("Recharge callback amount mismatch",
			logger.F("orderNo", orderNo),
			logger.F("expected", amount),
			logger.F("actual", result.Amount))
		return ErrAmountMismatch
	}

	if persistedPaymentNo == paymentNo && status == order.StatusPaid {
		s.logger.Info("Duplicate recharge callback ignored after persisted payment lookup",
			logger.F("orderNo", orderNo),
			logger.F("paymentNo", paymentNo))
		return nil
	}

	if s.markCallbackProcessing(callbackKey, paymentNo) {
		return nil
	}

	if err := s.rechargeHandler.MarkRechargePaid(ctx, orderNo, paymentNo, result.PaidAt); err != nil {
		s.logger.Error("Failed to mark recharge order as paid",
			logger.Err(err),
			logger.F("orderNo", orderNo))
		s.unmarkCallbackProcessing(callbackKey)
		return err
	}

	s.logger.Info("Recharge payment callback processed",
		logger.F("orderNo", orderNo),
		logger.F("paymentNo", paymentNo),
		logger.F("amount", result.Amount))

	return nil
}

func (s *Service) markCallbackProcessing(callbackKey string, paymentNo string) bool {
	s.callbackMu.Lock()
	defer s.callbackMu.Unlock()

	if s.processedCallbacks[callbackKey] {
		s.logger.Info("Duplicate callback ignored",
			logger.F("paymentNo", paymentNo))
		return true
	}

	s.processedCallbacks[callbackKey] = true
	return false
}

func (s *Service) unmarkCallbackProcessing(callbackKey string) {
	s.callbackMu.Lock()
	defer s.callbackMu.Unlock()
	delete(s.processedCallbacks, callbackKey)
}

// QueryPayment queries the payment status.
func (s *Service) QueryPayment(ctx context.Context, method string, paymentNo string) (*PaymentResult, error) {
	gateway, err := s.GetGateway(method)
	if err != nil {
		return nil, err
	}

	result, err := gateway.QueryPayment(paymentNo)
	if err != nil {
		s.logger.Error("Failed to query payment",
			logger.Err(err),
			logger.F("method", method),
			logger.F("paymentNo", paymentNo))
		return nil, err
	}

	return result, nil
}

// ProcessRefund processes a refund for an order.
func (s *Service) ProcessRefund(ctx context.Context, orderID int64, amount int64, reason string) (*RefundResult, error) {
	ord, err := s.orderService.GetByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	if ord.PaymentNo == "" || ord.PaymentMethod == "" {
		return nil, fmt.Errorf("order has no payment information")
	}

	gateway, err := s.GetGateway(ord.PaymentMethod)
	if err != nil {
		return nil, err
	}

	result, err := gateway.Refund(ord.PaymentNo, amount, reason)
	if err != nil {
		s.logger.Error("Failed to process refund",
			logger.Err(err),
			logger.F("orderID", orderID),
			logger.F("amount", amount))
		return nil, fmt.Errorf("%w: %v", ErrRefundFailed, err)
	}

	if !result.Success {
		s.logger.Warn("Refund failed",
			logger.F("orderID", orderID),
			logger.F("error", result.Error))
		return result, nil
	}

	if err := s.orderService.UpdateStatus(ctx, orderID, order.StatusRefunded); err != nil {
		s.logger.Error("Failed to update order status after refund",
			logger.Err(err),
			logger.F("orderID", orderID))
	}

	s.logger.Info("Refund processed",
		logger.F("orderID", orderID),
		logger.F("refundNo", result.RefundNo),
		logger.F("amount", result.Amount))

	return result, nil
}

// IsCallbackProcessed checks if a callback has been processed (for idempotency).
func (s *Service) IsCallbackProcessed(method, paymentNo string) bool {
	callbackKey := fmt.Sprintf("%s:%s", method, paymentNo)
	s.callbackMu.Lock()
	defer s.callbackMu.Unlock()
	return s.processedCallbacks[callbackKey]
}

// GetPaymentStatus returns the payment status for an order.
func (s *Service) GetPaymentStatus(ctx context.Context, orderNo string) (string, error) {
	ord, err := s.orderService.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return "", ErrOrderNotFound
	}
	return ord.Status, nil
}
