// Package invite provides invite and referral management functionality.
package invite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrInviteCodeNotFound = errors.New("invite code not found")
	ErrSelfReferral       = errors.New("cannot use own invite code")
	ErrAlreadyReferred    = errors.New("user already has a referrer")
	ErrInvalidInviteCode  = errors.New("invalid invite code")
)

// InviteCode represents an invite code.
type InviteCode struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Code        string `json:"code"`
	InviteCount int    `json:"invite_count"`
	CreatedAt   string `json:"created_at"`
}

// Referral represents a referral relationship.
type Referral struct {
	ID          int64   `json:"id"`
	InviterID   int64   `json:"inviter_id"`
	InviteeID   int64   `json:"invitee_id"`
	InviteCode  string  `json:"invite_code"`
	Status      string  `json:"status"`
	ConvertedAt *string `json:"converted_at"`
	CreatedAt   string  `json:"created_at"`
}

// InviteStats represents invite statistics.
type InviteStats struct {
	TotalInvites        int   `json:"total_invites"`
	ConvertedInvites    int   `json:"converted_invites"`
	ConversionRate      float64 `json:"conversion_rate"`
	PendingCommission   int64 `json:"pending_commission"`
	ConfirmedCommission int64 `json:"confirmed_commission"`
	TotalCommission     int64 `json:"total_commission"`
}

// Config holds invite service configuration.
type Config struct {
	BaseURL string // Base URL for invite links
}

// Service provides invite management operations.
type Service struct {
	inviteRepo repository.InviteRepository
	logger     logger.Logger
	config     *Config
	mu         sync.Mutex
	codes      map[string]bool // Track generated codes for uniqueness
}

// NewService creates a new invite service.
func NewService(inviteRepo repository.InviteRepository, log logger.Logger, config *Config) *Service {
	if config == nil {
		config = &Config{BaseURL: "https://example.com"}
	}
	return &Service{
		inviteRepo: inviteRepo,
		logger:     log,
		config:     config,
		codes:      make(map[string]bool),
	}
}

// GetOrCreateCode gets or creates an invite code for a user.
func (s *Service) GetOrCreateCode(ctx context.Context, userID int64) (*InviteCode, error) {
	// Try to get existing code
	repoCode, err := s.inviteRepo.GetInviteCodeByUserID(ctx, userID)
	if err == nil {
		return s.toInviteCode(repoCode), nil
	}

	// Generate new code
	code := s.generateCode()

	repoCode = &repository.CommercialInviteCode{
		UserID: userID,
		Code:   code,
	}

	if err := s.inviteRepo.CreateInviteCode(ctx, repoCode); err != nil {
		s.logger.Error("Failed to create invite code", logger.Err(err), logger.F("userID", userID))
		return nil, err
	}

	s.logger.Info("Created invite code", logger.F("userID", userID), logger.F("code", code))
	return s.toInviteCode(repoCode), nil
}

// GetByCode retrieves an invite code by code string.
func (s *Service) GetByCode(ctx context.Context, code string) (*InviteCode, error) {
	repoCode, err := s.inviteRepo.GetInviteCodeByCode(ctx, strings.ToUpper(code))
	if err != nil {
		return nil, ErrInviteCodeNotFound
	}
	return s.toInviteCode(repoCode), nil
}

// RecordReferral records a referral relationship.
func (s *Service) RecordReferral(ctx context.Context, inviteCode string, inviteeID int64) error {
	// Get invite code
	code, err := s.GetByCode(ctx, inviteCode)
	if err != nil {
		return err
	}

	// Check for self-referral
	if code.UserID == inviteeID {
		return ErrSelfReferral
	}

	// Check if already referred
	existing, err := s.inviteRepo.GetReferralByInviteeID(ctx, inviteeID)
	if err == nil && existing != nil {
		return ErrAlreadyReferred
	}

	// Create referral
	referral := &repository.Referral{
		InviterID:  code.UserID,
		InviteeID:  inviteeID,
		InviteCode: code.Code,
		Status:     repository.ReferralStatusRegistered,
	}

	if err := s.inviteRepo.CreateReferral(ctx, referral); err != nil {
		s.logger.Error("Failed to create referral", logger.Err(err))
		return err
	}

	// Increment invite count
	if err := s.inviteRepo.IncrementInviteCount(ctx, code.UserID); err != nil {
		s.logger.Error("Failed to increment invite count", logger.Err(err))
		// Don't fail the operation
	}

	s.logger.Info("Recorded referral",
		logger.F("inviterID", code.UserID),
		logger.F("inviteeID", inviteeID),
		logger.F("code", inviteCode))

	return nil
}

// MarkConverted marks a referral as converted (invitee made first purchase).
func (s *Service) MarkConverted(ctx context.Context, inviteeID int64) error {
	if err := s.inviteRepo.MarkReferralConverted(ctx, inviteeID); err != nil {
		s.logger.Error("Failed to mark referral as converted", logger.Err(err), logger.F("inviteeID", inviteeID))
		return err
	}

	s.logger.Info("Marked referral as converted", logger.F("inviteeID", inviteeID))
	return nil
}

// GetReferrer gets the referrer for a user.
func (s *Service) GetReferrer(ctx context.Context, inviteeID int64) (*Referral, error) {
	repoReferral, err := s.inviteRepo.GetReferralByInviteeID(ctx, inviteeID)
	if err != nil {
		return nil, nil // No referrer
	}
	return s.toReferral(repoReferral), nil
}

// GetReferrals lists referrals for a user.
func (s *Service) GetReferrals(ctx context.Context, userID int64, page, pageSize int) ([]*Referral, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoReferrals, total, err := s.inviteRepo.ListReferralsByInviter(ctx, userID, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list referrals", logger.Err(err), logger.F("userID", userID))
		return nil, 0, err
	}

	referrals := make([]*Referral, len(repoReferrals))
	for i, rr := range repoReferrals {
		referrals[i] = s.toReferral(rr)
	}

	return referrals, total, nil
}

// GetStats retrieves invite statistics for a user.
func (s *Service) GetStats(ctx context.Context, userID int64) (*InviteStats, error) {
	repoStats, err := s.inviteRepo.GetInviteStats(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get invite stats", logger.Err(err), logger.F("userID", userID))
		return nil, err
	}

	stats := &InviteStats{
		TotalInvites:        repoStats.TotalInvites,
		ConvertedInvites:    repoStats.ConvertedInvites,
		PendingCommission:   repoStats.PendingCommission,
		ConfirmedCommission: repoStats.ConfirmedCommission,
		TotalCommission:     repoStats.TotalCommission,
	}

	if stats.TotalInvites > 0 {
		stats.ConversionRate = float64(stats.ConvertedInvites) / float64(stats.TotalInvites) * 100
	}

	return stats, nil
}

// GenerateInviteLink generates an invite link for a code.
func (s *Service) GenerateInviteLink(code string) string {
	return fmt.Sprintf("%s/user/register?ref=%s", s.config.BaseURL, code)
}

// generateCode generates a unique invite code.
func (s *Service) generateCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		bytes := make([]byte, 4)
		rand.Read(bytes)
		code := strings.ToUpper(hex.EncodeToString(bytes))

		if !s.codes[code] {
			s.codes[code] = true
			return code
		}
	}
}

// toInviteCode converts a repository invite code to a service invite code.
func (s *Service) toInviteCode(rc *repository.CommercialInviteCode) *InviteCode {
	return &InviteCode{
		ID:          rc.ID,
		UserID:      rc.UserID,
		Code:        rc.Code,
		InviteCount: rc.InviteCount,
		CreatedAt:   rc.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// toReferral converts a repository referral to a service referral.
func (s *Service) toReferral(rr *repository.Referral) *Referral {
	referral := &Referral{
		ID:         rr.ID,
		InviterID:  rr.InviterID,
		InviteeID:  rr.InviteeID,
		InviteCode: rr.InviteCode,
		Status:     rr.Status,
		CreatedAt:  rr.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	if rr.ConvertedAt != nil {
		converted := rr.ConvertedAt.Format("2006-01-02 15:04:05")
		referral.ConvertedAt = &converted
	}

	return referral
}
