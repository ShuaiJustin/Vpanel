// Package payment provides payment gateway functionality.
package payment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"v/internal/commercial/balance"
	"v/internal/commercial/order"
	"v/internal/database/repository"
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
	settlement      map[string]PaymentGateway
	orderService    *order.Service
	balanceSvc      *balance.Service
	rechargeHandler RechargeCallbackHandler
	reconciliation  paymentReconciliationRepository
	logger          logger.Logger
	failures        *repository.PaymentFailureRepository
	mu              sync.RWMutex
}

type paymentReconciliationRepository interface {
	UpdatePendingPaymentMethod(context.Context, int64, string) error
	ListPaymentsToReconcile(context.Context, []string, time.Time, int) ([]*repository.Order, error)
	MarkPaymentChecked(context.Context, int64, time.Time) error
}

func (s *Service) WithReconciliation(repo repository.OrderRepository) *Service {
	s.reconciliation, _ = repo.(paymentReconciliationRepository)
	return s
}

func (s *Service) WithFailures(repo *repository.PaymentFailureRepository) *Service {
	s.failures = repo
	return s
}

// NewService creates a new payment service.
func NewService(orderService *order.Service, log logger.Logger) *Service {
	return &Service{
		gateways:     make(map[string]PaymentGateway),
		settlement:   make(map[string]PaymentGateway),
		orderService: orderService,
		logger:       log,
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
	s.settlement[gateway.Name()] = gateway
	s.logger.Info("Registered payment gateway", logger.F("name", gateway.Name()))
}

// ReplaceGateways replaces all externally registered gateways.
func (s *Service) ReplaceGateways(gateways map[string]PaymentGateway) {
	s.ConfigureGateways(gateways, nil)
}

// ConfigureGateways replaces the new-payment and settlement registries together.
func (s *Service) ConfigureGateways(gateways, settlement map[string]PaymentGateway) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gateways = make(map[string]PaymentGateway, len(gateways))
	for name, gateway := range gateways {
		s.gateways[name] = gateway
	}
	s.settlement = make(map[string]PaymentGateway, len(settlement)+len(gateways))
	for name, gateway := range settlement {
		s.settlement[name] = gateway
	}
	for name, gateway := range gateways {
		s.settlement[name] = gateway
	}
}

func (s *Service) settlementGateway(name string) (PaymentGateway, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gateway, ok := s.settlement[strings.TrimSpace(name)]
	if !ok {
		return nil, ErrGatewayNotFound
	}
	return gateway, nil
}

// GetGateway returns a payment gateway by name.
func (s *Service) GetGateway(name string) (PaymentGateway, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	name = strings.TrimSpace(name)
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
	sort.Strings(names)
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
	if !ord.ExpiredAt.After(time.Now()) {
		return nil, order.ErrOrderExpired
	}
	// A zero-price order needs neither a wallet debit nor an external gateway.
	if ord.PayAmount == 0 {
		if err := s.orderService.Pay(ctx, ord.OrderNo, "FREE-"+ord.OrderNo, "free", false); err != nil && !s.paidDespite(ctx, ord.OrderNo, err) {
			return nil, err
		}
		if err := s.orderService.Complete(ctx, ord.ID); err != nil {
			return nil, err
		}
		return &PaymentRequest{ExpireTime: time.Now()}, nil
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
		ExpiresAt:   ord.ExpiredAt,
	}

	if s.reconciliation != nil {
		method = strings.TrimSpace(method)
		if _, err := s.GetGateway(method); err != nil {
			return nil, err
		}
		if err := s.reconciliation.UpdatePendingPaymentMethod(ctx, ord.ID, method); err != nil {
			if errors.Is(err, repository.ErrCommercialState) {
				return nil, ErrOrderNotPending
			}
			return nil, err
		}
	}
	return s.CreateGatewayPaymentContext(ctx, method, paymentOrder)
}

// ReconcilePending checks recently attempted gateway orders for missing callbacks.
func (s *Service) ReconcilePending(ctx context.Context) error {
	if s.reconciliation == nil {
		return nil
	}
	s.mu.RLock()
	queriers := make(map[string]merchantOrderQuerier, len(s.settlement))
	methods := make([]string, 0, len(s.settlement))
	for method, gateway := range s.settlement {
		if query, ok := gateway.(merchantOrderQuerier); ok {
			methods = append(methods, method)
			queriers[method] = query
		}
	}
	s.mu.RUnlock()
	if len(methods) == 0 {
		return nil
	}
	now := time.Now()
	orders, err := s.reconciliation.ListPaymentsToReconcile(ctx, methods, now, 20)
	if err != nil {
		return err
	}
	for _, ord := range orders {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Query the recorded method first and the other supported gateways too.
		// A previously opened payment might settle after a method switch.
		candidates := append([]string{ord.PaymentMethod}, methods...)
		for index, method := range candidates {
			if index > 0 && method == ord.PaymentMethod {
				continue
			}
			query, ok := queriers[method]
			if !ok {
				continue
			}
			queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			result, queryErr := query.QueryMerchantOrder(queryCtx, ord.OrderNo)
			cancel()
			if queryErr != nil {
				if method == ord.PaymentMethod && ctx.Err() == nil {
					s.recordFailure(ctx, ord.OrderNo, method, repository.PaymentFailureQuery)
				}
				s.logger.Warn("Payment reconciliation query failed", logger.F("order_id", ord.ID), logger.F("method", method), logger.Err(queryErr))
				continue
			}
			if result == nil || !result.Success {
				continue
			}
			if result.OrderNo != ord.OrderNo || strings.TrimSpace(result.PaymentNo) == "" || result.Amount != ord.PayAmount || result.PaidAt.IsZero() {
				s.recordFailure(ctx, ord.OrderNo, method, repository.PaymentFailureMismatch)
				s.logger.Warn("Payment reconciliation result did not match order", logger.F("order_id", ord.ID), logger.F("method", method))
				continue
			}
			if err := s.orderService.PayFromGateway(ctx, ord.OrderNo, result.PaymentNo, method, result.PaidAt); err != nil {
				s.recordFailure(ctx, ord.OrderNo, method, repository.PaymentFailureApply)
				s.logger.Warn("Could not apply reconciled payment", logger.F("order_id", ord.ID), logger.Err(err))
			}
			break
		}
		if err := s.reconciliation.MarkPaymentChecked(ctx, ord.ID, now); err != nil {
			return err
		}
	}
	return nil
}

// CreateGatewayPayment creates an external gateway payment for a generic payment order.
func (s *Service) CreateGatewayPayment(method string, paymentOrder *PaymentOrder) (*PaymentRequest, error) {
	return s.CreateGatewayPaymentContext(context.Background(), method, paymentOrder)
}

func (s *Service) CreateGatewayPaymentContext(ctx context.Context, method string, paymentOrder *PaymentOrder) (*PaymentRequest, error) {
	if paymentOrder == nil || strings.TrimSpace(paymentOrder.OrderNo) == "" || paymentOrder.Amount <= 0 {
		return nil, fmt.Errorf("%w: invalid payment order", ErrPaymentFailed)
	}
	method = strings.TrimSpace(method)
	if method == "balance" {
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
		s.recordFailure(ctx, requestOrder.OrderNo, method, repository.PaymentFailureCreate)
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

	paymentNo := fmt.Sprintf("BALANCE-%d", time.Now().UnixNano())
	if err := s.orderService.Pay(ctx, ord.OrderNo, paymentNo, "balance", true); err != nil && !s.paidDespite(ctx, ord.OrderNo, err) {
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

// paidDespite reports whether the order was committed as paid although Pay
// returned an error. Pay debits the wallet and applies the plan in one
// transaction, then provisions nodes; a provisioning failure is kept as
// fulfillment_pending and retried every minute. Telling the buyer "payment
// failed" at that point shows an unpaid order for money already taken.
func (s *Service) paidDespite(ctx context.Context, orderNo string, payErr error) bool {
	ord, err := s.orderService.GetByOrderNo(ctx, orderNo)
	if err != nil || (ord.Status != order.StatusPaid && ord.Status != order.StatusCompleted) {
		return false
	}
	s.logger.Warn("Order paid; node provisioning is pending and will be retried",
		logger.Err(payErr), logger.F("orderNo", orderNo))
	return true
}

// HandleCallback handles a payment callback.
func (s *Service) HandleCallback(ctx context.Context, method string, data []byte, signature string) error {
	method = strings.TrimSpace(method)
	gateway, err := s.settlementGateway(method)
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

	if result == nil {
		return ErrInvalidCallback
	}

	orderNo := strings.TrimSpace(result.OrderNo)
	paymentNo := strings.TrimSpace(result.PaymentNo)
	if !result.Success {
		if !result.Pending && orderNo != "" && s.hasPaymentOrder(ctx, orderNo) {
			s.recordFailure(ctx, orderNo, method, repository.PaymentFailureRejected)
		}
		return nil
	}
	if orderNo == "" {
		return ErrInvalidCallback
	}

	ord, err := s.orderService.GetByOrderNo(ctx, orderNo)
	if err == nil {
		// SECURITY: a successful callback must always carry a positive
		// amount. Skipping the comparison when Amount == 0 would let a
		// malformed callback mark an order as paid without any amount check.
		if result.Amount <= 0 || paymentNo == "" {
			s.recordFailure(ctx, orderNo, method, repository.PaymentFailureMismatch)
			s.logger.Warn("Payment callback missing amount",
				logger.F("orderNo", orderNo),
				logger.F("method", method))
			return ErrInvalidCallback
		}
		if ord.PayAmount != result.Amount {
			s.recordFailure(ctx, orderNo, method, repository.PaymentFailureMismatch)
			s.logger.Warn("Payment callback amount mismatch",
				logger.F("orderNo", orderNo),
				logger.F("expected", ord.PayAmount),
				logger.F("actual", result.Amount))
			return ErrAmountMismatch
		}

		// Check if already processed in database
		if ord.PaymentNo == paymentNo && !ord.FulfillmentPending && (ord.Status == order.StatusPaid || ord.Status == order.StatusCompleted) {
			s.logger.Info("Duplicate callback ignored (already processed in database)",
				logger.F("orderNo", orderNo),
				logger.F("paymentNo", paymentNo))
			return nil
		}

		if err := s.orderService.PayFromGateway(ctx, orderNo, paymentNo, method, result.PaidAt); err != nil {
			// If a verified gateway settlement arrives after the order expired or was cancelled,
			// credit the user's balance idempotently rather than rejecting with 400 and losing funds.
			if (errors.Is(err, order.ErrOrderExpired) || ord.Status == order.StatusCancelled) && s.balanceSvc != nil && ord.PayAmount > 0 {
				if creditErr := s.creditLateOrderToBalance(ctx, ord, paymentNo); creditErr == nil {
					s.logger.Warn("Late payment arrived for closed order; credited user balance",
						logger.F("orderNo", orderNo),
						logger.F("paymentNo", paymentNo),
						logger.F("userID", ord.UserID),
						logger.F("amount", ord.PayAmount))
					return nil
				} else {
					s.logger.Error("Failed to credit late payment to balance",
						logger.Err(creditErr),
						logger.F("orderNo", orderNo),
						logger.F("paymentNo", paymentNo))
				}
			}
			s.recordFailure(ctx, orderNo, method, repository.PaymentFailureApply)
			s.logger.Error("Failed to mark order as paid",
				logger.Err(err),
				logger.F("orderNo", orderNo))
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

	if result.Amount <= 0 || paymentNo == "" {
		s.recordFailure(ctx, orderNo, method, repository.PaymentFailureMismatch)
		s.logger.Warn("Recharge callback missing amount",
			logger.F("orderNo", orderNo),
			logger.F("method", method))
		return ErrInvalidCallback
	}
	if amount != result.Amount {
		s.recordFailure(ctx, orderNo, method, repository.PaymentFailureMismatch)
		s.logger.Warn("Recharge callback amount mismatch",
			logger.F("orderNo", orderNo),
			logger.F("expected", amount),
			logger.F("actual", result.Amount))
		return ErrAmountMismatch
	}

	// Check if already processed in database
	if persistedPaymentNo == paymentNo && status == order.StatusPaid {
		s.logger.Info("Duplicate recharge callback ignored (already processed in database)",
			logger.F("orderNo", orderNo),
			logger.F("paymentNo", paymentNo))
		return nil
	}

	paidAt := result.PaidAt
	if paidAt.IsZero() {
		paidAt = time.Now()
	}
	if err := s.rechargeHandler.MarkRechargePaid(ctx, orderNo, paymentNo, paidAt); err != nil {
		s.recordFailure(ctx, orderNo, method, repository.PaymentFailureApply)
		s.logger.Error("Failed to mark recharge order as paid",
			logger.Err(err),
			logger.F("orderNo", orderNo))
		return err
	}

	s.logger.Info("Recharge payment callback processed",
		logger.F("orderNo", orderNo),
		logger.F("paymentNo", paymentNo),
		logger.F("amount", result.Amount))

	return nil
}

func (s *Service) creditLateOrderToBalance(ctx context.Context, ord *order.Order, paymentNo string) error {
	if s.balanceSvc == nil || ord == nil || ord.PayAmount <= 0 {
		return ErrPaymentFailed
	}
	txs, _, err := s.balanceSvc.ListTransactions(ctx, balance.TransactionFilter{
		UserID: &ord.UserID,
		Type:   balance.TxTypeRecharge,
	}, 1, 100)
	if err == nil {
		for _, tx := range txs {
			if tx.OrderID != nil && *tx.OrderID == ord.ID {
				return nil
			}
		}
	}
	return s.balanceSvc.Recharge(
		ctx,
		ord.UserID,
		ord.PayAmount,
		&ord.ID,
		fmt.Sprintf("订单 %s 超时后延迟到账自动转入余额 (支付流水: %s)", ord.OrderNo, paymentNo),
	)
}

func (s *Service) hasPaymentOrder(ctx context.Context, orderNo string) bool {
	if s.orderService != nil {
		if _, err := s.orderService.GetByOrderNo(ctx, orderNo); err == nil {
			return true
		}
	}
	if s.rechargeHandler != nil {
		_, _, _, err := s.rechargeHandler.GetRechargePaymentDetails(ctx, orderNo)
		return err == nil
	}
	return false
}

func (s *Service) recordFailure(ctx context.Context, orderNo, method string, reason string) {
	if s.failures == nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := s.failures.Record(writeCtx, orderNo, method, reason, time.Now()); err != nil {
		s.logger.Error("Failed to record payment failure", logger.Err(err))
	}
}

// QueryPayment queries the payment status.
func (s *Service) QueryPayment(ctx context.Context, method string, paymentNo string) (*PaymentResult, error) {
	gateway, err := s.settlementGateway(method)
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

	gateway, err := s.settlementGateway(ord.PaymentMethod)
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

// IsCallbackProcessed checks if a callback has been processed (deprecated - use database check).
func (s *Service) IsCallbackProcessed(method, paymentNo string) bool {
	// This method is deprecated - callback idempotency is now handled by database checks
	return false
}

// GetPaymentStatus returns the payment status for an order.
func (s *Service) GetPaymentStatus(ctx context.Context, orderNo string) (string, error) {
	ord, err := s.orderService.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return "", ErrOrderNotFound
	}
	return ord.Status, nil
}

// GetPaymentStatusForUser returns the payment status for an order, but only
// if the order belongs to userID. Prevents cross-user order probing (IDOR).
func (s *Service) GetPaymentStatusForUser(ctx context.Context, orderNo string, userID int64) (string, error) {
	ord, err := s.orderService.GetByOrderNo(ctx, orderNo)
	if err != nil || ord.UserID != userID {
		// Same error for "not found" and "not yours" so callers cannot
		// distinguish the two.
		return "", ErrOrderNotFound
	}
	return ord.Status, nil
}

// OrderBelongsToUser reports whether the order exists and belongs to userID.
// Used by handlers to gate order-scoped operations (retry, switch method).
func (s *Service) OrderBelongsToUser(ctx context.Context, orderID int64, userID int64) bool {
	ord, err := s.orderService.GetByID(ctx, orderID)
	return err == nil && ord.UserID == userID
}
