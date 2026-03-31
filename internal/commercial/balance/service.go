// Package balance provides balance management functionality.
package balance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrInvalidAmount         = errors.New("invalid amount")
	ErrUserNotFound          = errors.New("user not found")
	ErrNegativeBalance       = errors.New("balance cannot be negative")
	ErrRechargeUnavailable   = errors.New("recharge is unavailable")
	ErrRechargeOrderNotFound = errors.New("recharge order not found")
	ErrRechargeOrderNotReady = errors.New("recharge order is not pending")
	ErrInvalidRechargeMethod = errors.New("invalid recharge method")
)

// Transaction type constants
const (
	TxTypeRecharge   = repository.BalanceTxTypeRecharge
	TxTypePurchase   = repository.BalanceTxTypePurchase
	TxTypeRefund     = repository.BalanceTxTypeRefund
	TxTypeCommission = repository.BalanceTxTypeCommission
	TxTypeAdjustment = repository.BalanceTxTypeAdjustment
)

const rechargeOrderExpiration = 30 * time.Minute

// Transaction represents a balance transaction.
type Transaction struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Type        string `json:"type"`
	Amount      int64  `json:"amount"`
	Balance     int64  `json:"balance"`
	OrderID     *int64 `json:"order_id"`
	Description string `json:"description"`
	Operator    string `json:"operator"`
	CreatedAt   string `json:"created_at"`
}

type TransactionFilter struct {
	UserID    *int64
	Type      string
	StartDate *time.Time
	EndDate   *time.Time
}

// RechargeOrder represents a balance recharge order.
type RechargeOrder struct {
	ID        int64      `json:"id"`
	OrderNo   string     `json:"order_no"`
	UserID    int64      `json:"user_id"`
	Username  string     `json:"username,omitempty"`
	Amount    int64      `json:"amount"`
	Method    string     `json:"method"`
	Status    string     `json:"status"`
	PaymentNo string     `json:"payment_no"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
	ExpiredAt time.Time  `json:"expired_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type RechargeOrderFilter struct {
	UserID    *int64
	Search    string
	Status    string
	Method    string
	StartDate *time.Time
	EndDate   *time.Time
	MinAmount *int64
	MaxAmount *int64
}

// Service provides balance management operations.
type Service struct {
	balanceRepo  repository.BalanceRepository
	rechargeRepo repository.BalanceRechargeOrderRepository
	logger       logger.Logger
	mu           sync.Mutex
}

// NewService creates a new balance service.
func NewService(balanceRepo repository.BalanceRepository, log logger.Logger) *Service {
	return &Service{
		balanceRepo: balanceRepo,
		logger:      log,
	}
}

// WithRechargeRepository enables recharge order management for the balance service.
func (s *Service) WithRechargeRepository(rechargeRepo repository.BalanceRechargeOrderRepository) *Service {
	s.rechargeRepo = rechargeRepo
	return s
}

// GetBalance retrieves the current balance for a user.
func (s *Service) GetBalance(ctx context.Context, userID int64) (int64, error) {
	balance, err := s.balanceRepo.GetBalance(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get balance", logger.Err(err), logger.F("userID", userID))
		return 0, err
	}
	return balance, nil
}

// CanDeduct checks if a user has sufficient balance for a deduction.
func (s *Service) CanDeduct(ctx context.Context, userID int64, amount int64) bool {
	if amount <= 0 {
		return false
	}
	balance, err := s.GetBalance(ctx, userID)
	if err != nil {
		return false
	}
	return balance >= amount
}

// Recharge adds funds to a user's balance.
func (s *Service) Recharge(ctx context.Context, userID int64, amount int64, orderID *int64, description string) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentBalance, err := s.balanceRepo.GetBalance(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get balance for recharge", logger.Err(err), logger.F("userID", userID))
		return err
	}

	newBalance := currentBalance + amount

	if err := s.balanceRepo.IncrementBalance(ctx, userID, amount); err != nil {
		s.logger.Error("Failed to increment balance", logger.Err(err), logger.F("userID", userID))
		return err
	}

	tx := &repository.BalanceTransaction{
		UserID:      userID,
		Type:        TxTypeRecharge,
		Amount:      amount,
		Balance:     newBalance,
		OrderID:     orderID,
		Description: description,
		Operator:    "system",
	}

	if err := s.balanceRepo.CreateTransaction(ctx, tx); err != nil {
		s.logger.Error("Failed to create recharge transaction", logger.Err(err))
		return err
	}

	s.logger.Info("Balance recharged", logger.F("userID", userID), logger.F("amount", amount), logger.F("newBalance", newBalance))
	return nil
}

// Deduct subtracts funds from a user's balance.
func (s *Service) Deduct(ctx context.Context, userID int64, amount int64, orderID *int64, description string) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentBalance, err := s.balanceRepo.GetBalance(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get balance for deduction", logger.Err(err), logger.F("userID", userID))
		return err
	}

	if currentBalance < amount {
		return ErrInsufficientBalance
	}

	newBalance := currentBalance - amount
	if newBalance < 0 {
		return ErrNegativeBalance
	}

	if err := s.balanceRepo.DecrementBalance(ctx, userID, amount); err != nil {
		s.logger.Error("Failed to decrement balance", logger.Err(err), logger.F("userID", userID))
		return err
	}

	tx := &repository.BalanceTransaction{
		UserID:      userID,
		Type:        TxTypePurchase,
		Amount:      -amount,
		Balance:     newBalance,
		OrderID:     orderID,
		Description: description,
		Operator:    "system",
	}

	if err := s.balanceRepo.CreateTransaction(ctx, tx); err != nil {
		s.logger.Error("Failed to create deduction transaction", logger.Err(err))
		return err
	}

	s.logger.Info("Balance deducted", logger.F("userID", userID), logger.F("amount", amount), logger.F("newBalance", newBalance))
	return nil
}

// Refund adds refunded funds back to a user's balance.
func (s *Service) Refund(ctx context.Context, userID int64, amount int64, orderID *int64, description string) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentBalance, err := s.balanceRepo.GetBalance(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get balance for refund", logger.Err(err), logger.F("userID", userID))
		return err
	}

	newBalance := currentBalance + amount

	if err := s.balanceRepo.IncrementBalance(ctx, userID, amount); err != nil {
		s.logger.Error("Failed to increment balance for refund", logger.Err(err), logger.F("userID", userID))
		return err
	}

	tx := &repository.BalanceTransaction{
		UserID:      userID,
		Type:        TxTypeRefund,
		Amount:      amount,
		Balance:     newBalance,
		OrderID:     orderID,
		Description: description,
		Operator:    "system",
	}

	if err := s.balanceRepo.CreateTransaction(ctx, tx); err != nil {
		s.logger.Error("Failed to create refund transaction", logger.Err(err))
		return err
	}

	s.logger.Info("Balance refunded", logger.F("userID", userID), logger.F("amount", amount), logger.F("newBalance", newBalance))
	return nil
}

// AddCommission adds commission to a user's balance.
func (s *Service) AddCommission(ctx context.Context, userID int64, amount int64, description string) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentBalance, err := s.balanceRepo.GetBalance(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get balance for commission", logger.Err(err), logger.F("userID", userID))
		return err
	}

	newBalance := currentBalance + amount

	if err := s.balanceRepo.IncrementBalance(ctx, userID, amount); err != nil {
		s.logger.Error("Failed to increment balance for commission", logger.Err(err), logger.F("userID", userID))
		return err
	}

	tx := &repository.BalanceTransaction{
		UserID:      userID,
		Type:        TxTypeCommission,
		Amount:      amount,
		Balance:     newBalance,
		Description: description,
		Operator:    "system",
	}

	if err := s.balanceRepo.CreateTransaction(ctx, tx); err != nil {
		s.logger.Error("Failed to create commission transaction", logger.Err(err))
		return err
	}

	s.logger.Info("Commission added", logger.F("userID", userID), logger.F("amount", amount), logger.F("newBalance", newBalance))
	return nil
}

// Adjust manually adjusts a user's balance (admin operation).
func (s *Service) Adjust(ctx context.Context, userID int64, amount int64, reason string, operator string) error {
	if amount == 0 {
		return fmt.Errorf("%w: amount cannot be zero", ErrInvalidAmount)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentBalance, err := s.balanceRepo.GetBalance(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get balance for adjustment", logger.Err(err), logger.F("userID", userID))
		return err
	}

	newBalance := currentBalance + amount
	if newBalance < 0 {
		return ErrNegativeBalance
	}

	if amount > 0 {
		if err := s.balanceRepo.IncrementBalance(ctx, userID, amount); err != nil {
			s.logger.Error("Failed to increment balance for adjustment", logger.Err(err), logger.F("userID", userID))
			return err
		}
	} else {
		if err := s.balanceRepo.DecrementBalance(ctx, userID, -amount); err != nil {
			s.logger.Error("Failed to decrement balance for adjustment", logger.Err(err), logger.F("userID", userID))
			return err
		}
	}

	tx := &repository.BalanceTransaction{
		UserID:      userID,
		Type:        TxTypeAdjustment,
		Amount:      amount,
		Balance:     newBalance,
		Description: reason,
		Operator:    operator,
	}

	if err := s.balanceRepo.CreateTransaction(ctx, tx); err != nil {
		s.logger.Error("Failed to create adjustment transaction", logger.Err(err))
		return err
	}

	s.logger.Info("Balance adjusted", logger.F("userID", userID), logger.F("amount", amount), logger.F("newBalance", newBalance), logger.F("operator", operator))
	return nil
}

// ListTransactions retrieves transaction history with pagination and filters.
func (s *Service) ListTransactions(ctx context.Context, filter TransactionFilter, page, pageSize int) ([]*Transaction, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoTxs, total, err := s.balanceRepo.ListTransactions(ctx, repository.BalanceFilter{
		UserID:    filter.UserID,
		Type:      filter.Type,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
	}, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list transactions", logger.Err(err))
		return nil, 0, err
	}

	txs := make([]*Transaction, len(repoTxs))
	for i, rt := range repoTxs {
		txs[i] = s.toTransaction(rt)
	}

	return txs, total, nil
}

// GetTransactions retrieves transaction history for a user.
func (s *Service) GetTransactions(ctx context.Context, userID int64, page, pageSize int) ([]*Transaction, int64, error) {
	return s.ListTransactions(ctx, TransactionFilter{UserID: &userID}, page, pageSize)
}

// GetStatistics retrieves balance statistics for a user.
func (s *Service) GetStatistics(ctx context.Context, userID int64) (totalRecharge, totalSpent, totalCommission int64, err error) {
	totalRecharge, err = s.balanceRepo.GetTotalRecharge(ctx, userID)
	if err != nil {
		return 0, 0, 0, err
	}

	totalSpent, err = s.balanceRepo.GetTotalSpent(ctx, userID)
	if err != nil {
		return 0, 0, 0, err
	}

	totalCommission, err = s.balanceRepo.GetTotalCommission(ctx, userID)
	if err != nil {
		return 0, 0, 0, err
	}

	return totalRecharge, totalSpent, totalCommission, nil
}

// ListRechargeOrders retrieves recharge orders with pagination and filters.
func (s *Service) ListRechargeOrders(ctx context.Context, filter RechargeOrderFilter, page, pageSize int) ([]*RechargeOrder, int64, error) {
	if s.rechargeRepo == nil {
		return nil, 0, ErrRechargeUnavailable
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoOrders, total, err := s.rechargeRepo.List(ctx, repository.BalanceRechargeOrderFilter{
		UserID:    filter.UserID,
		Search:    filter.Search,
		Status:    filter.Status,
		Method:    filter.Method,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		MinAmount: filter.MinAmount,
		MaxAmount: filter.MaxAmount,
	}, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list recharge orders", logger.Err(err))
		return nil, 0, err
	}

	orders := make([]*RechargeOrder, len(repoOrders))
	for i, repoOrder := range repoOrders {
		orders[i] = s.toRechargeOrder(repoOrder)
	}

	return orders, total, nil
}

// CreateRechargeOrder creates a new online recharge order.
func (s *Service) CreateRechargeOrder(ctx context.Context, userID int64, amount int64, method string) (*RechargeOrder, error) {
	if s.rechargeRepo == nil {
		return nil, ErrRechargeUnavailable
	}
	if amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}

	method = strings.TrimSpace(method)
	if method == "" || method == "balance" {
		return nil, ErrInvalidRechargeMethod
	}

	repoOrder := &repository.BalanceRechargeOrder{
		OrderNo:   generateRechargeOrderNo(),
		UserID:    userID,
		Amount:    amount,
		Method:    method,
		Status:    repository.BalanceRechargeStatusPending,
		ExpiredAt: time.Now().Add(rechargeOrderExpiration),
	}

	if err := s.rechargeRepo.Create(ctx, repoOrder); err != nil {
		s.logger.Error("Failed to create recharge order", logger.Err(err), logger.F("userID", userID), logger.F("amount", amount), logger.F("method", method))
		return nil, err
	}

	return s.toRechargeOrder(repoOrder), nil
}

// GetRechargeOrderByOrderNo retrieves a recharge order.
func (s *Service) GetRechargeOrderByOrderNo(ctx context.Context, orderNo string) (*RechargeOrder, error) {
	if s.rechargeRepo == nil {
		return nil, ErrRechargeUnavailable
	}

	repoOrder, err := s.rechargeRepo.GetByOrderNo(ctx, strings.TrimSpace(orderNo))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRechargeOrderNotFound
		}
		return nil, err
	}

	return s.toRechargeOrder(repoOrder), nil
}

// GetRechargePaymentDetails returns the payment comparison fields used by payment callbacks.
func (s *Service) GetRechargePaymentDetails(ctx context.Context, orderNo string) (int64, string, string, error) {
	order, err := s.GetRechargeOrderByOrderNo(ctx, orderNo)
	if err != nil {
		return 0, "", "", err
	}
	return order.Amount, order.PaymentNo, order.Status, nil
}

// MarkRechargePaid marks a recharge order as paid and credits the user's balance.
func (s *Service) MarkRechargePaid(ctx context.Context, orderNo string, paymentNo string, paidAt time.Time) error {
	if s.rechargeRepo == nil {
		return ErrRechargeUnavailable
	}

	_, err := s.rechargeRepo.MarkPaidAndCredit(ctx, strings.TrimSpace(orderNo), strings.TrimSpace(paymentNo), paidAt, fmt.Sprintf("账户余额充值（订单 %s）", strings.TrimSpace(orderNo)))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRechargeOrderNotFound
		}
		if errors.Is(err, gorm.ErrInvalidData) {
			return ErrRechargeOrderNotReady
		}
		return err
	}

	return nil
}

// toTransaction converts a repository transaction to a service transaction.
func (s *Service) toTransaction(rt *repository.BalanceTransaction) *Transaction {
	return &Transaction{
		ID:          rt.ID,
		UserID:      rt.UserID,
		Type:        rt.Type,
		Amount:      rt.Amount,
		Balance:     rt.Balance,
		OrderID:     rt.OrderID,
		Description: rt.Description,
		Operator:    rt.Operator,
		CreatedAt:   rt.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (s *Service) toRechargeOrder(order *repository.BalanceRechargeOrder) *RechargeOrder {
	return &RechargeOrder{
		ID:      order.ID,
		OrderNo: order.OrderNo,
		UserID:  order.UserID,
		Username: func() string {
			if order.User != nil {
				return order.User.Username
			}
			return ""
		}(),
		Amount:    order.Amount,
		Method:    order.Method,
		Status:    order.Status,
		PaymentNo: order.PaymentNo,
		PaidAt:    order.PaidAt,
		ExpiredAt: order.ExpiredAt,
		CreatedAt: order.CreatedAt,
	}
}

func generateRechargeOrderNo() string {
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	return fmt.Sprintf("RCG-%s-%s", time.Now().Format("20060102"), hex.EncodeToString(randomBytes))
}
