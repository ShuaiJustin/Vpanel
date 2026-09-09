// Package planchange provides plan upgrade and downgrade functionality.
package planchange

import (
	"context"
	"errors"
	"time"

	"v/internal/commercial/balance"
	"v/internal/commercial/order"
	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrPlanNotFound         = errors.New("plan not found")
	ErrPlanInactive         = errors.New("plan is not active")
	ErrUserNotFound         = errors.New("user not found")
	ErrNoActiveSubscription = errors.New("user has no active subscription")
	ErrSamePlan             = errors.New("cannot change to the same plan")
	ErrDowngradeNotAllowed  = errors.New("downgrade is not allowed for this plan")
	ErrUpgradeNotAllowed    = errors.New("upgrade is not allowed for this plan")
	ErrPendingDowngrade     = errors.New("user already has a pending downgrade")
	ErrNoPendingDowngrade   = errors.New("user has no pending downgrade")
	ErrInsufficientBalance  = errors.New("insufficient balance for upgrade")
	ErrCurrentPlanUnknown   = repository.ErrCurrentPlanUnknown
)

// PlanChangeRequest represents a request to change plans.
type PlanChangeRequest struct {
	UserID        int64 `json:"user_id"`
	CurrentPlanID int64 `json:"current_plan_id"`
	NewPlanID     int64 `json:"new_plan_id"`
	Immediate     bool  `json:"immediate"` // For downgrade: immediate or next cycle
}

// PlanChangeResult represents the result of a plan change calculation.
type PlanChangeResult struct {
	PriceDifference int64     `json:"price_difference"` // positive = pay more, negative = refund
	RemainingDays   int       `json:"remaining_days"`
	NewExpireAt     time.Time `json:"new_expire_at"`
	IsUpgrade       bool      `json:"is_upgrade"`
	CurrentPlan     *PlanInfo `json:"current_plan"`
	NewPlan         *PlanInfo `json:"new_plan"`
}

// PlanInfo represents basic plan information for display.
type PlanInfo struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Price        int64    `json:"price"`
	Duration     int      `json:"duration"`
	TrafficLimit int64    `json:"traffic_limit"`
	Features     []string `json:"features"`
}

// PendingDowngrade represents a scheduled plan downgrade.
type PendingDowngrade struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	CurrentPlanID int64     `json:"current_plan_id"`
	NewPlanID     int64     `json:"new_plan_id"`
	EffectiveAt   time.Time `json:"effective_at"`
	CurrentPlan   *PlanInfo `json:"current_plan"`
	NewPlan       *PlanInfo `json:"new_plan"`
	CreatedAt     time.Time `json:"created_at"`
}

// Service provides plan change operations.
type Service struct {
	planChangeRepo repository.PlanChangeRepository
	planRepo       repository.PlanRepository
	userRepo       repository.UserRepository
	orderService   *order.Service
	balanceService *balance.Service
	logger         logger.Logger
}

// NewService creates a new plan change service.
func NewService(
	planChangeRepo repository.PlanChangeRepository,
	planRepo repository.PlanRepository,
	userRepo repository.UserRepository,
	orderService *order.Service,
	balanceService *balance.Service,
	log logger.Logger,
) *Service {
	return &Service{
		planChangeRepo: planChangeRepo,
		planRepo:       planRepo,
		userRepo:       userRepo,
		orderService:   orderService,
		balanceService: balanceService,
		logger:         log,
	}
}

// GetCurrentPlan resolves the server-owned plan, including legacy paid orders.
func (s *Service) GetCurrentPlan(ctx context.Context, userID int64) (*PlanInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.ExpiresAt == nil || !user.ExpiresAt.After(time.Now()) {
		return nil, ErrNoActiveSubscription
	}
	repo, ok := s.planChangeRepo.(repository.AtomicPlanChangeRepository)
	if !ok {
		return nil, errors.New("atomic plan change repository is required")
	}
	plan, err := repo.CurrentPlan(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toPlanInfo(plan), nil
}

// CalculateChange calculates the price difference and details for a plan change.
// Formula for upgrade: (new_price - old_price) * (remaining_days / total_days)
func (s *Service) CalculateChange(ctx context.Context, req *PlanChangeRequest) (*PlanChangeResult, error) {
	// Validate request
	if req.UserID <= 0 {
		return nil, ErrUserNotFound
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Check if user has active subscription
	if user.ExpiresAt == nil || user.ExpiresAt.Before(time.Now()) {
		return nil, ErrNoActiveSubscription
	}

	// Get current plan
	atomicRepo, ok := s.planChangeRepo.(repository.AtomicPlanChangeRepository)
	if !ok {
		return nil, errors.New("atomic plan change repository is required")
	}
	currentPlan, err := atomicRepo.CurrentPlan(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if currentPlan.ID == req.NewPlanID {
		return nil, ErrSamePlan
	}
	if req.CurrentPlanID > 0 && req.CurrentPlanID != currentPlan.ID {
		return nil, ErrCurrentPlanUnknown
	}

	// Get new plan
	newPlan, err := s.planRepo.GetByID(ctx, req.NewPlanID)
	if err != nil {
		return nil, ErrPlanNotFound
	}
	if !newPlan.IsActive {
		return nil, ErrPlanInactive
	}

	// Calculate remaining days
	remainingDays := int(time.Until(*user.ExpiresAt).Hours() / 24)
	if remainingDays < 0 {
		remainingDays = 0
	}

	// Determine if this is an upgrade or downgrade
	isUpgrade := newPlan.Price > currentPlan.Price

	// Calculate price difference using proration formula
	// Formula: (new_price - old_price) * (remaining_days / total_days)
	priceDifference, err := repository.PlanProration(currentPlan.Price, newPlan.Price, remainingDays, currentPlan.Duration)
	if err != nil {
		return nil, err
	}

	// Calculate new expiration date
	// For upgrade: keep the same expiration date
	// For downgrade: will be applied at next cycle
	newExpireAt := *user.ExpiresAt

	return &PlanChangeResult{
		PriceDifference: priceDifference,
		RemainingDays:   remainingDays,
		NewExpireAt:     newExpireAt,
		IsUpgrade:       isUpgrade,
		CurrentPlan:     s.toPlanInfo(currentPlan),
		NewPlan:         s.toPlanInfo(newPlan),
	}, nil
}

// CalculateProration calculates the prorated price difference.
// Formula: (new_price - old_price) * (remaining_days / total_days)
func (s *Service) CalculateProration(oldPrice, newPrice int64, remainingDays, totalDays int) int64 {
	amount, _ := repository.PlanProration(oldPrice, newPrice, remainingDays, totalDays)
	return amount
}

// ExecuteUpgrade executes an immediate plan upgrade.
func (s *Service) ExecuteUpgrade(ctx context.Context, req *PlanChangeRequest) (*order.Order, error) {
	// Calculate the change first
	result, err := s.CalculateChange(ctx, req)
	if err != nil {
		return nil, err
	}

	// Verify this is an upgrade
	if !result.IsUpgrade {
		return nil, ErrUpgradeNotAllowed
	}

	if err := s.planChangeRepo.(repository.AtomicPlanChangeRepository).UpgradeAndDebit(ctx, req.UserID, result.CurrentPlan.ID, req.NewPlanID); err != nil {
		if errors.Is(err, repository.ErrInsufficientBalance) {
			return nil, ErrInsufficientBalance
		}
		return nil, err
	}

	s.logger.Info("Plan upgrade executed",
		logger.F("userID", req.UserID),
		logger.F("fromPlan", req.CurrentPlanID),
		logger.F("toPlan", req.NewPlanID),
		logger.F("priceDiff", result.PriceDifference))

	return nil, nil
}

// ScheduleDowngrade schedules a plan downgrade for the next billing cycle.
func (s *Service) ScheduleDowngrade(ctx context.Context, req *PlanChangeRequest) error {
	// Calculate the change first
	result, err := s.CalculateChange(ctx, req)
	if err != nil {
		return err
	}

	// Verify this is a downgrade
	if result.IsUpgrade {
		return ErrDowngradeNotAllowed
	}

	// Check if user already has a pending downgrade
	existing, err := s.planChangeRepo.GetPendingDowngradeByUserID(ctx, req.UserID)
	if err == nil && existing != nil {
		return ErrPendingDowngrade
	}

	// Get user's expiration date
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return ErrUserNotFound
	}

	// Create pending downgrade
	downgrade := &repository.PendingDowngrade{
		UserID:        req.UserID,
		CurrentPlanID: result.CurrentPlan.ID,
		NewPlanID:     req.NewPlanID,
		EffectiveAt:   *user.ExpiresAt, // Effective at subscription expiration
	}

	if err := s.planChangeRepo.CreatePendingDowngrade(ctx, downgrade); err != nil {
		s.logger.Error("Failed to create pending downgrade", logger.Err(err), logger.F("userID", req.UserID))
		return err
	}

	s.logger.Info("Plan downgrade scheduled",
		logger.F("userID", req.UserID),
		logger.F("fromPlan", req.CurrentPlanID),
		logger.F("toPlan", req.NewPlanID),
		logger.F("effectiveAt", downgrade.EffectiveAt))

	return nil
}

// GetPendingDowngrade retrieves a user's pending downgrade.
func (s *Service) ListPendingDowngrades(ctx context.Context, page, pageSize int) ([]*PendingDowngrade, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := s.planChangeRepo.ListAllPendingDowngrades(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*PendingDowngrade, 0, len(items))
	for _, item := range items {
		result = append(result, s.toPendingDowngrade(item))
	}
	return result, total, nil
}

func (s *Service) GetPendingDowngrade(ctx context.Context, userID int64) (*PendingDowngrade, error) {
	repoDowngrade, err := s.planChangeRepo.GetPendingDowngradeByUserID(ctx, userID)
	if err != nil {
		return nil, ErrNoPendingDowngrade
	}

	return s.toPendingDowngrade(repoDowngrade), nil
}

// CancelPendingDowngrade cancels a user's pending downgrade.
func (s *Service) CancelPendingDowngrade(ctx context.Context, userID int64) error {
	err := s.planChangeRepo.DeletePendingDowngradeByUserID(ctx, userID)
	if err != nil {
		return ErrNoPendingDowngrade
	}

	s.logger.Info("Pending downgrade cancelled", logger.F("userID", userID))
	return nil
}

// ProcessDueDowngrades processes all pending downgrades that are due.
// This should be called by a cron job.
func (s *Service) ProcessDueDowngrades(ctx context.Context) (int, error) {
	downgrades, err := s.planChangeRepo.ListDueDowngrades(ctx)
	if err != nil {
		s.logger.Error("Failed to list due downgrades", logger.Err(err))
		return 0, err
	}

	processed := 0
	var failures []error
	for _, downgrade := range downgrades {
		if err := s.executeDueDowngrade(ctx, downgrade); err != nil {
			s.logger.Error("Failed to execute downgrade",
				logger.Err(err),
				logger.F("userID", downgrade.UserID),
				logger.F("downgradeID", downgrade.ID))
			failures = append(failures, err)
			continue
		}
		processed++
	}

	if processed > 0 {
		s.logger.Info("Processed due downgrades", logger.F("count", processed))
	}

	return processed, errors.Join(failures...)
}

// executeDueDowngrade executes a single due downgrade.
func (s *Service) executeDueDowngrade(ctx context.Context, downgrade *repository.PendingDowngrade) error {
	atomicRepo, ok := s.planChangeRepo.(repository.AtomicPlanChangeRepository)
	if !ok {
		return errors.New("atomic plan change repository is required")
	}
	return atomicRepo.ApplyDueDowngrade(ctx, downgrade.ID)
}

// toPlanInfo converts a repository plan to PlanInfo.
func (s *Service) toPlanInfo(plan *repository.CommercialPlan) *PlanInfo {
	return &PlanInfo{
		ID:           plan.ID,
		Name:         plan.Name,
		Price:        plan.Price,
		Duration:     plan.Duration,
		TrafficLimit: plan.TrafficLimit,
		Features:     []string{}, // Features would need to be parsed from JSON
	}
}

// toPendingDowngrade converts a repository pending downgrade to service type.
func (s *Service) toPendingDowngrade(pd *repository.PendingDowngrade) *PendingDowngrade {
	result := &PendingDowngrade{
		ID:            pd.ID,
		UserID:        pd.UserID,
		CurrentPlanID: pd.CurrentPlanID,
		NewPlanID:     pd.NewPlanID,
		EffectiveAt:   pd.EffectiveAt,
		CreatedAt:     pd.CreatedAt,
	}

	if pd.CurrentPlan != nil {
		result.CurrentPlan = s.toPlanInfo(pd.CurrentPlan)
	}
	if pd.NewPlan != nil {
		result.NewPlan = s.toPlanInfo(pd.NewPlan)
	}

	return result
}
