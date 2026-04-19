// Package giftcard provides gift card management functionality.
package giftcard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"v/internal/commercial/balance"
	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrGiftCardNotFound     = errors.New("gift card not found")
	ErrGiftCardAlreadyUsed  = errors.New("gift card has already been redeemed")
	ErrGiftCardExpired      = errors.New("gift card has expired")
	ErrGiftCardDisabled     = errors.New("gift card is disabled")
	ErrGiftCardInvalid      = errors.New("invalid gift card")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrInvalidCount         = errors.New("invalid count")
	ErrSelfRedeem           = errors.New("cannot redeem your own gift card")
)

// Status constants
const (
	StatusActive   = repository.GiftCardStatusActive
	StatusRedeemed = repository.GiftCardStatusRedeemed
	StatusExpired  = repository.GiftCardStatusExpired
	StatusDisabled = repository.GiftCardStatusDisabled
)

// GiftCard represents a gift card.
type GiftCard struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	Value       int64      `json:"value"`
	Status      string     `json:"status"`
	CreatedBy   *int64     `json:"created_by"`
	PurchasedBy *int64     `json:"purchased_by"`
	RedeemedBy  *int64     `json:"redeemed_by"`
	BatchID     string     `json:"batch_id"`
	ExpiresAt   *time.Time `json:"expires_at"`
	RedeemedAt  *time.Time `json:"redeemed_at"`
	PurchasedAt *time.Time `json:"purchased_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateBatchRequest represents a request to create a batch of gift cards.
type CreateBatchRequest struct {
	Count     int        `json:"count"`
	Value     int64      `json:"value"`
	ExpiresAt *time.Time `json:"expires_at"`
	Prefix    string     `json:"prefix"`
}

// GiftCardFilter defines filter options for listing gift cards.
type GiftCardFilter struct {
	Status    string
	BatchID   string
	CreatedBy *int64
	MinValue  *int64
	MaxValue  *int64
}

// BatchStats represents statistics for a batch of gift cards.
type BatchStats struct {
	BatchID       string `json:"batch_id"`
	Total         int    `json:"total"`
	Redeemed      int    `json:"redeemed"`
	TotalValue    int64  `json:"total_value"`
	RedeemedValue int64  `json:"redeemed_value"`
}

// Stats represents overall gift card statistics.
type Stats struct {
	TotalCards      int64 `json:"total_cards"`
	ActiveCards     int64 `json:"active_cards"`
	RedeemedCards   int64 `json:"redeemed_cards"`
	ExpiredCards    int64 `json:"expired_cards"`
	TotalValue      int64 `json:"total_value"`
	ActiveValue     int64 `json:"active_value"`
	RedeemedValue   int64 `json:"redeemed_value"`
}

// Service provides gift card management operations.
type Service struct {
	giftCardRepo   repository.GiftCardRepository
	balanceService *balance.Service
	logger         logger.Logger
}

// NewService creates a new gift card service.
func NewService(giftCardRepo repository.GiftCardRepository, balanceService *balance.Service, log logger.Logger) *Service {
	return &Service{
		giftCardRepo:   giftCardRepo,
		balanceService: balanceService,
		logger:         log,
	}
}


// CreateBatch creates a batch of gift cards.
func (s *Service) CreateBatch(ctx context.Context, req *CreateBatchRequest, createdBy int64) ([]*GiftCard, string, error) {
	if req.Count <= 0 || req.Count > 1000 {
		return nil, "", fmt.Errorf("%w: count must be between 1 and 1000", ErrInvalidCount)
	}
	if req.Value <= 0 {
		return nil, "", fmt.Errorf("%w: value must be positive", ErrInvalidAmount)
	}

	// Generate batch ID
	batchID := s.generateBatchID()

	// Generate gift cards
	giftCards := make([]*repository.GiftCard, req.Count)
	codes := make(map[string]bool)

	for i := 0; i < req.Count; i++ {
		code := s.generateCode(req.Prefix)
		// Ensure uniqueness within batch
		for codes[code] {
			code = s.generateCode(req.Prefix)
		}
		codes[code] = true

		giftCards[i] = &repository.GiftCard{
			Code:      code,
			Value:     req.Value,
			Status:    StatusActive,
			CreatedBy: &createdBy,
			BatchID:   batchID,
			ExpiresAt: req.ExpiresAt,
		}
	}

	// Create in database
	if err := s.giftCardRepo.CreateBatch(ctx, giftCards); err != nil {
		s.logger.Error("Failed to create gift card batch", logger.Err(err))
		return nil, "", err
	}

	// Convert to service model
	result := make([]*GiftCard, len(giftCards))
	for i, gc := range giftCards {
		result[i] = s.toGiftCard(gc)
	}

	s.logger.Info("Gift card batch created",
		logger.F("batchID", batchID),
		logger.F("count", req.Count),
		logger.F("value", req.Value),
		logger.F("createdBy", createdBy))

	return result, batchID, nil
}

// GetByID retrieves a gift card by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*GiftCard, error) {
	repoGC, err := s.giftCardRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrGiftCardNotFound
	}
	return s.toGiftCard(repoGC), nil
}

// GetByCode retrieves a gift card by code.
func (s *Service) GetByCode(ctx context.Context, code string) (*GiftCard, error) {
	repoGC, err := s.giftCardRepo.GetByCode(ctx, strings.ToUpper(code))
	if err != nil {
		return nil, ErrGiftCardNotFound
	}
	return s.toGiftCard(repoGC), nil
}

// Redeem redeems a gift card and credits the value to user's balance.
// Flips the card status BEFORE crediting balance so a concurrent redemption
// attempt loses the status guard and cannot double-credit. If the balance
// credit fails after the flip, the card is locked in "redeemed" state with no
// corresponding credit — preferable to the double-credit failure mode and
// recoverable by admin adjustment.
func (s *Service) Redeem(ctx context.Context, code string, userID int64) (*GiftCard, error) {
	// Get gift card
	repoGC, err := s.giftCardRepo.GetByCode(ctx, strings.ToUpper(code))
	if err != nil {
		return nil, ErrGiftCardNotFound
	}

	// Validate gift card status (pre-check for friendly error messages)
	if repoGC.Status == StatusRedeemed {
		return nil, ErrGiftCardAlreadyUsed
	}
	if repoGC.Status == StatusDisabled {
		return nil, ErrGiftCardDisabled
	}
	if repoGC.Status == StatusExpired {
		return nil, ErrGiftCardExpired
	}

	// Check expiration
	if repoGC.ExpiresAt != nil && time.Now().After(*repoGC.ExpiresAt) {
		// Mark as expired
		_ = s.giftCardRepo.SetStatus(ctx, repoGC.ID, StatusExpired)
		return nil, ErrGiftCardExpired
	}

	// Prevent self-redemption (if purchased by user)
	if repoGC.PurchasedBy != nil && *repoGC.PurchasedBy == userID {
		return nil, ErrSelfRedeem
	}

	// Atomically flip status to redeemed. Race losers get ErrRecordNotFound
	// (they see the new status via the WHERE guard) and fail here cleanly.
	if err := s.giftCardRepo.MarkRedeemed(ctx, repoGC.ID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGiftCardAlreadyUsed
		}
		s.logger.Error("Failed to mark gift card as redeemed", logger.Err(err),
			logger.F("code", repoGC.Code))
		return nil, err
	}

	// Credit balance. If this fails the card is already marked redeemed
	// (prevents double-credit); surface the error and let admin reconcile.
	description := fmt.Sprintf("Gift card redemption: %s", repoGC.Code)
	if err := s.balanceService.Recharge(ctx, userID, repoGC.Value, nil, description); err != nil {
		s.logger.Error("Gift card redeemed but balance credit failed — admin must reconcile",
			logger.Err(err),
			logger.F("code", repoGC.Code),
			logger.F("userID", userID),
			logger.F("giftcard_id", repoGC.ID),
			logger.F("value", repoGC.Value))
		return nil, err
	}

	s.logger.Info("Gift card redeemed",
		logger.F("code", repoGC.Code),
		logger.F("value", repoGC.Value),
		logger.F("userID", userID))

	// Refresh and return
	repoGC, _ = s.giftCardRepo.GetByID(ctx, repoGC.ID)
	return s.toGiftCard(repoGC), nil
}

// Purchase marks a gift card as purchased by a user.
func (s *Service) Purchase(ctx context.Context, id int64, userID int64) (*GiftCard, error) {
	repoGC, err := s.giftCardRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrGiftCardNotFound
	}

	if repoGC.Status != StatusActive {
		return nil, ErrGiftCardInvalid
	}

	if err := s.giftCardRepo.MarkPurchased(ctx, id, userID); err != nil {
		s.logger.Error("Failed to mark gift card as purchased", logger.Err(err))
		return nil, err
	}

	s.logger.Info("Gift card purchased",
		logger.F("id", id),
		logger.F("userID", userID))

	// Refresh and return
	repoGC, _ = s.giftCardRepo.GetByID(ctx, id)
	return s.toGiftCard(repoGC), nil
}

// List lists gift cards with filter and pagination.
func (s *Service) List(ctx context.Context, filter GiftCardFilter, page, pageSize int) ([]*GiftCard, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoFilter := repository.GiftCardFilter{
		Status:    filter.Status,
		BatchID:   filter.BatchID,
		CreatedBy: filter.CreatedBy,
		MinValue:  filter.MinValue,
		MaxValue:  filter.MaxValue,
	}

	repoGCs, total, err := s.giftCardRepo.List(ctx, repoFilter, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list gift cards", logger.Err(err))
		return nil, 0, err
	}

	giftCards := make([]*GiftCard, len(repoGCs))
	for i, gc := range repoGCs {
		giftCards[i] = s.toGiftCard(gc)
	}

	return giftCards, total, nil
}

// ListByUser lists gift cards redeemed by a user.
func (s *Service) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]*GiftCard, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoGCs, total, err := s.giftCardRepo.ListByUser(ctx, userID, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list user gift cards", logger.Err(err))
		return nil, 0, err
	}

	giftCards := make([]*GiftCard, len(repoGCs))
	for i, gc := range repoGCs {
		giftCards[i] = s.toGiftCard(gc)
	}

	return giftCards, total, nil
}

// SetStatus sets the status of a gift card.
func (s *Service) SetStatus(ctx context.Context, id int64, status string) error {
	if status != StatusActive && status != StatusDisabled {
		return fmt.Errorf("%w: invalid status", ErrGiftCardInvalid)
	}

	if err := s.giftCardRepo.SetStatus(ctx, id, status); err != nil {
		s.logger.Error("Failed to set gift card status", logger.Err(err),
			logger.F("id", id),
			logger.F("status", status))
		return err
	}

	return nil
}

// Delete deletes a gift card.
func (s *Service) Delete(ctx context.Context, id int64) error {
	// Only allow deleting active/disabled cards
	gc, err := s.giftCardRepo.GetByID(ctx, id)
	if err != nil {
		return ErrGiftCardNotFound
	}

	if gc.Status == StatusRedeemed {
		return fmt.Errorf("%w: cannot delete redeemed gift card", ErrGiftCardInvalid)
	}

	if err := s.giftCardRepo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete gift card", logger.Err(err), logger.F("id", id))
		return err
	}

	return nil
}

// GetBatchStats returns statistics for a batch of gift cards.
func (s *Service) GetBatchStats(ctx context.Context, batchID string) (*BatchStats, error) {
	total, redeemed, totalValue, redeemedValue, err := s.giftCardRepo.GetBatchStats(ctx, batchID)
	if err != nil {
		s.logger.Error("Failed to get batch stats", logger.Err(err), logger.F("batchID", batchID))
		return nil, err
	}

	return &BatchStats{
		BatchID:       batchID,
		Total:         total,
		Redeemed:      redeemed,
		TotalValue:    totalValue,
		RedeemedValue: redeemedValue,
	}, nil
}

// GetStats returns overall gift card statistics.
func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	totalCards, err := s.giftCardRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	activeCards, err := s.giftCardRepo.CountByStatus(ctx, StatusActive)
	if err != nil {
		return nil, err
	}

	redeemedCards, err := s.giftCardRepo.CountByStatus(ctx, StatusRedeemed)
	if err != nil {
		return nil, err
	}

	expiredCards, err := s.giftCardRepo.CountByStatus(ctx, StatusExpired)
	if err != nil {
		return nil, err
	}

	activeValue, err := s.giftCardRepo.GetTotalValue(ctx, StatusActive)
	if err != nil {
		return nil, err
	}

	redeemedValue, err := s.giftCardRepo.GetTotalValue(ctx, StatusRedeemed)
	if err != nil {
		return nil, err
	}

	return &Stats{
		TotalCards:    totalCards,
		ActiveCards:   activeCards,
		RedeemedCards: redeemedCards,
		ExpiredCards:  expiredCards,
		TotalValue:    activeValue + redeemedValue,
		ActiveValue:   activeValue,
		RedeemedValue: redeemedValue,
	}, nil
}

// ExpireOldCards marks expired active cards as expired.
func (s *Service) ExpireOldCards(ctx context.Context) (int, error) {
	expiredCards, err := s.giftCardRepo.GetExpiredActive(ctx)
	if err != nil {
		s.logger.Error("Failed to get expired cards", logger.Err(err))
		return 0, err
	}

	count := 0
	for _, gc := range expiredCards {
		if err := s.giftCardRepo.SetStatus(ctx, gc.ID, StatusExpired); err != nil {
			s.logger.Error("Failed to expire gift card", logger.Err(err), logger.F("id", gc.ID))
			continue
		}
		count++
	}

	if count > 0 {
		s.logger.Info("Expired gift cards", logger.F("count", count))
	}

	return count, nil
}

// generateCode generates a unique gift card code.
func (s *Service) generateCode(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	code := strings.ToUpper(hex.EncodeToString(bytes))

	// Format as XXXX-XXXX-XXXX-XXXX
	formatted := fmt.Sprintf("%s-%s-%s-%s", code[0:4], code[4:8], code[8:12], code[12:16])

	if prefix != "" {
		return fmt.Sprintf("%s-%s", strings.ToUpper(prefix), formatted)
	}
	return formatted
}

// generateBatchID generates a unique batch ID.
func (s *Service) generateBatchID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return fmt.Sprintf("BATCH-%s-%s", time.Now().Format("20060102"), strings.ToUpper(hex.EncodeToString(bytes)))
}

// toGiftCard converts a repository gift card to a service gift card.
func (s *Service) toGiftCard(gc *repository.GiftCard) *GiftCard {
	return &GiftCard{
		ID:          gc.ID,
		Code:        gc.Code,
		Value:       gc.Value,
		Status:      gc.Status,
		CreatedBy:   gc.CreatedBy,
		PurchasedBy: gc.PurchasedBy,
		RedeemedBy:  gc.RedeemedBy,
		BatchID:     gc.BatchID,
		ExpiresAt:   gc.ExpiresAt,
		RedeemedAt:  gc.RedeemedAt,
		PurchasedAt: gc.PurchasedAt,
		CreatedAt:   gc.CreatedAt,
	}
}
