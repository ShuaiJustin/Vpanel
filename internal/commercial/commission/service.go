// Package commission provides commission management functionality.
package commission

import (
	"context"
	"errors"
	"fmt"
	"time"

	"v/internal/commercial/balance"
	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrCommissionNotFound = errors.New("commission not found")
	ErrInvalidCommission  = errors.New("invalid commission data")
	ErrAlreadyConfirmed   = errors.New("commission already confirmed")
	ErrAlreadyCancelled   = errors.New("commission already cancelled")
)

// Commission status constants
const (
	StatusPending   = repository.CommissionStatusPending
	StatusConfirmed = repository.CommissionStatusConfirmed
	StatusCancelled = repository.CommissionStatusCancelled
)

// Commission represents a commission record.
type Commission struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	FromUserID int64   `json:"from_user_id"`
	OrderID    int64   `json:"order_id"`
	Amount     int64   `json:"amount"`
	Rate       float64 `json:"rate"`
	Level      int     `json:"level"`
	Status     string  `json:"status"`
	ConfirmAt  *string `json:"confirm_at"`
	CreatedAt  string  `json:"created_at"`
}

// Config holds commission service configuration.
type Config struct {
	Enabled         bool    `json:"enabled"`
	Rate            float64 `json:"rate"`              // e.g., 0.1 = 10%
	FixedBonus      int64   `json:"fixed_bonus"`       // cents, one-time bonus
	SettlementDelay int     `json:"settlement_delay"`  // days before confirmation
	MinWithdraw     int64   `json:"min_withdraw"`      // minimum withdrawal amount
	MultiLevel      bool    `json:"multi_level"`       // enable multi-level referral
	MaxLevel        int     `json:"max_level"`         // max referral depth
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		Rate:            0.1, // 10%
		FixedBonus:      0,
		SettlementDelay: 7, // 7 days
		MinWithdraw:     1000, // 10 yuan
		MultiLevel:      false,
		MaxLevel:        1,
	}
}

// Service provides commission management operations.
type Service struct {
	inviteRepo     repository.InviteRepository
	balanceService *balance.Service
	config         *Config
	logger         logger.Logger
}

// NewService creates a new commission service.
func NewService(
	inviteRepo repository.InviteRepository,
	balanceService *balance.Service,
	log logger.Logger,
	config *Config,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		inviteRepo:     inviteRepo,
		balanceService: balanceService,
		config:         config,
		logger:         log,
	}
}

// Calculate calculates commission for an order.
func (s *Service) Calculate(orderAmount int64, inviterID int64) (int64, error) {
	if !s.config.Enabled {
		return 0, nil
	}

	if orderAmount <= 0 {
		return 0, nil
	}

	// Calculate commission based on rate
	commission := int64(float64(orderAmount) * s.config.Rate)

	return commission, nil
}

// Create creates a commission record.
func (s *Service) Create(ctx context.Context, userID, fromUserID, orderID int64, amount int64, rate float64, level int) (*Commission, error) {
	if !s.config.Enabled {
		return nil, nil
	}

	if amount <= 0 {
		return nil, ErrInvalidCommission
	}

	if existing, err := s.inviteRepo.GetCommissionByOrderID(ctx, orderID); err == nil && existing != nil {
		return s.toCommission(existing), nil
	}

	repoCommission := &repository.Commission{
		UserID:     userID,
		FromUserID: fromUserID,
		OrderID:    orderID,
		Amount:     amount,
		Rate:       rate,
		Level:      level,
		Status:     StatusPending,
	}

	if err := s.inviteRepo.CreateCommission(ctx, repoCommission); err != nil {
		s.logger.Error("Failed to create commission", logger.Err(err))
		return nil, err
	}

	s.logger.Info("Commission created",
		logger.F("userID", userID),
		logger.F("fromUserID", fromUserID),
		logger.F("orderID", orderID),
		logger.F("amount", amount))

	return s.toCommission(repoCommission), nil
}

// Confirm confirms a pending commission and credits to user's balance.
func (s *Service) Confirm(ctx context.Context, id int64) error {
	commission, err := s.inviteRepo.GetCommissionByID(ctx, id)
	if err != nil {
		return ErrCommissionNotFound
	}

	if commission.Status == StatusConfirmed {
		return ErrAlreadyConfirmed
	}

	if commission.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}

	claimed, claimErr := s.inviteRepo.ClaimPendingCommission(ctx, id)
	if claimErr != nil {
		s.logger.Error("Failed to confirm commission", logger.Err(claimErr), logger.F("id", id))
		return claimErr
	}
	if !claimed {
		return ErrAlreadyConfirmed
	}

	// Credit to user's balance; revert status if crediting fails so commission money is never lost.
	if s.balanceService != nil && commission.Amount > 0 {
		if err := s.balanceService.AddCommission(ctx, commission.UserID, commission.Amount, fmt.Sprintf("订单 #%d 邀请佣金结算", commission.OrderID)); err != nil {
			s.logger.Error("Failed to credit commission to balance, reverting status", logger.Err(err), logger.F("id", id))
			if revertErr := s.inviteRepo.RevertClaimedCommission(ctx, id); revertErr != nil {
				s.logger.Error("Failed to revert claimed commission", logger.Err(revertErr), logger.F("id", id))
			}
			return err
		}
	}

	s.logger.Info("Commission confirmed", logger.F("id", id), logger.F("amount", commission.Amount))
	return nil
}

// Cancel cancels a pending commission.
func (s *Service) Cancel(ctx context.Context, id int64) error {
	commission, err := s.inviteRepo.GetCommissionByID(ctx, id)
	if err != nil {
		return ErrCommissionNotFound
	}

	if commission.Status == StatusConfirmed {
		return ErrAlreadyConfirmed
	}

	if commission.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}

	if err := s.inviteRepo.UpdateCommissionStatus(ctx, id, StatusCancelled); err != nil {
		s.logger.Error("Failed to cancel commission", logger.Err(err), logger.F("id", id))
		return err
	}

	s.logger.Info("Commission cancelled", logger.F("id", id))
	return nil
}

// CancelByOrder cancels all pending commissions for an order.
func (s *Service) CancelByOrder(ctx context.Context, orderID int64) error {
	if err := s.inviteRepo.CancelCommissionsByOrder(ctx, orderID); err != nil {
		s.logger.Error("Failed to cancel commissions by order", logger.Err(err), logger.F("orderID", orderID))
		return err
	}

	s.logger.Info("Cancelled commissions for order", logger.F("orderID", orderID))
	return nil
}

// ConfirmPendingCommissions confirms all pending commissions older than settlement delay
// and credits each inviter's balance atomically.
func (s *Service) ConfirmPendingCommissions(ctx context.Context) (int64, error) {
	beforeDate := time.Now().AddDate(0, 0, -s.config.SettlementDelay)

	due, err := s.inviteRepo.ListDuePendingCommissions(ctx, beforeDate, 200)
	if err != nil {
		s.logger.Error("Failed to list due pending commissions", logger.Err(err))
		return 0, err
	}

	var confirmed int64
	for _, comm := range due {
		claimed, claimErr := s.inviteRepo.ClaimPendingCommission(ctx, comm.ID)
		if claimErr != nil {
			s.logger.Error("Failed to claim pending commission", logger.Err(claimErr), logger.F("id", comm.ID))
			return confirmed, claimErr
		}
		if !claimed {
			continue
		}
		if s.balanceService != nil && comm.Amount > 0 {
			if err := s.balanceService.AddCommission(ctx, comm.UserID, comm.Amount, fmt.Sprintf("订单 #%d 邀请佣金结算", comm.OrderID)); err != nil {
				s.logger.Error("Failed to credit confirmed commission to balance, reverting claim", logger.Err(err), logger.F("id", comm.ID), logger.F("user_id", comm.UserID))
				if revertErr := s.inviteRepo.RevertClaimedCommission(ctx, comm.ID); revertErr != nil {
					s.logger.Error("Failed to revert claimed commission", logger.Err(revertErr), logger.F("id", comm.ID))
				}
				return confirmed, err
			}
		}
		confirmed++
	}

	if confirmed > 0 {
		s.logger.Info("Confirmed pending commissions", logger.F("count", confirmed))
	}

	return confirmed, nil
}

// GetByID retrieves a commission by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*Commission, error) {
	repoCommission, err := s.inviteRepo.GetCommissionByID(ctx, id)
	if err != nil {
		return nil, ErrCommissionNotFound
	}
	return s.toCommission(repoCommission), nil
}

// ListPending lists pending commissions for a user.
func (s *Service) ListPending(ctx context.Context, userID int64, page, pageSize int) ([]*Commission, int64, error) {
	return s.listByStatus(ctx, userID, StatusPending, page, pageSize)
}

// ListConfirmed lists confirmed commissions for a user.
func (s *Service) ListConfirmed(ctx context.Context, userID int64, page, pageSize int) ([]*Commission, int64, error) {
	return s.listByStatus(ctx, userID, StatusConfirmed, page, pageSize)
}

// ListAll lists all commissions for a user.
func (s *Service) ListAll(ctx context.Context, userID int64, page, pageSize int) ([]*Commission, int64, error) {
	return s.listByStatus(ctx, userID, "", page, pageSize)
}

func (s *Service) listByStatus(ctx context.Context, userID int64, status string, page, pageSize int) ([]*Commission, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoCommissions, total, err := s.inviteRepo.ListCommissionsByUser(ctx, userID, status, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list commissions", logger.Err(err), logger.F("userID", userID))
		return nil, 0, err
	}

	commissions := make([]*Commission, len(repoCommissions))
	for i, rc := range repoCommissions {
		commissions[i] = s.toCommission(rc)
	}

	return commissions, total, nil
}

// GetTotalEarnings returns total earnings for a user.
func (s *Service) GetTotalEarnings(ctx context.Context, userID int64) (int64, error) {
	return s.inviteRepo.GetTotalCommissionByUser(ctx, userID)
}

// GetPendingEarnings returns pending earnings for a user.
func (s *Service) GetPendingEarnings(ctx context.Context, userID int64) (int64, error) {
	return s.inviteRepo.GetPendingCommissionByUser(ctx, userID)
}

// GetConfig returns the commission configuration.
func (s *Service) GetConfig() *Config {
	return s.config
}

// toCommission converts a repository commission to a service commission.
func (s *Service) toCommission(rc *repository.Commission) *Commission {
	commission := &Commission{
		ID:         rc.ID,
		UserID:     rc.UserID,
		FromUserID: rc.FromUserID,
		OrderID:    rc.OrderID,
		Amount:     rc.Amount,
		Rate:       rc.Rate,
		Level:      rc.Level,
		Status:     rc.Status,
		CreatedAt:  rc.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	if rc.ConfirmAt != nil {
		confirmAt := rc.ConfirmAt.Format("2006-01-02 15:04:05")
		commission.ConfirmAt = &confirmAt
	}

	return commission
}
