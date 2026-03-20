// Package coupon provides coupon management functionality.
package coupon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrCouponNotFound     = errors.New("coupon not found")
	ErrCouponExpired      = errors.New("coupon has expired")
	ErrCouponInactive     = errors.New("coupon is not active")
	ErrCouponNotStarted   = errors.New("coupon is not yet valid")
	ErrCouponLimitReached = errors.New("coupon usage limit reached")
	ErrCouponUserLimit    = errors.New("user has reached coupon usage limit")
	ErrCouponMinAmount    = errors.New("order amount below minimum")
	ErrCouponPlanMismatch = errors.New("coupon not valid for this plan")
	ErrInvalidCoupon      = errors.New("invalid coupon data")
)

// Coupon type constants
const (
	TypeFixed      = repository.CouponTypeFixed
	TypePercentage = repository.CouponTypePercentage
)

// Coupon represents a coupon.
type Coupon struct {
	ID             int64     `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Value          int64     `json:"value"`
	MinOrderAmount int64     `json:"min_order_amount"`
	MaxDiscount    int64     `json:"max_discount"`
	TotalLimit     int       `json:"total_limit"`
	PerUserLimit   int       `json:"per_user_limit"`
	UsedCount      int       `json:"used_count"`
	PlanIDs        []int64   `json:"plan_ids"`
	StartAt        time.Time `json:"start_at"`
	ExpireAt       time.Time `json:"expire_at"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateCouponRequest represents a request to create a coupon.
type CreateCouponRequest struct {
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Value          int64     `json:"value"`
	MinOrderAmount int64     `json:"min_order_amount"`
	MaxDiscount    int64     `json:"max_discount"`
	TotalLimit     int       `json:"total_limit"`
	PerUserLimit   int       `json:"per_user_limit"`
	PlanIDs        []int64   `json:"plan_ids"`
	StartAt        time.Time `json:"start_at"`
	ExpireAt       time.Time `json:"expire_at"`
	IsActive       bool      `json:"is_active"`
}

// CouponFilter defines filter options for listing coupons.
type CouponFilter struct {
	IsActive  *bool
	Type      string
	StartDate *time.Time
	EndDate   *time.Time
}

// Service provides coupon management operations.
type Service struct {
	couponRepo repository.CouponRepository
	logger     logger.Logger
}

// NewService creates a new coupon service.
func NewService(couponRepo repository.CouponRepository, log logger.Logger) *Service {
	return &Service{
		couponRepo: couponRepo,
		logger:     log,
	}
}


// Create creates a new coupon.
func (s *Service) Create(ctx context.Context, req *CreateCouponRequest) (*Coupon, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidCoupon)
	}
	if req.Type != TypeFixed && req.Type != TypePercentage {
		return nil, fmt.Errorf("%w: invalid coupon type", ErrInvalidCoupon)
	}
	if req.Value <= 0 {
		return nil, fmt.Errorf("%w: value must be positive", ErrInvalidCoupon)
	}
	if req.ExpireAt.Before(req.StartAt) {
		return nil, fmt.Errorf("%w: expire date must be after start date", ErrInvalidCoupon)
	}
	if req.Code == "" {
		req.Code = s.generateCode("")
	}

	// Convert plan IDs to string
	planIDsStr := s.planIDsToString(req.PlanIDs)

	repoCoupon := &repository.Coupon{
		Code:           strings.ToUpper(req.Code),
		Name:           req.Name,
		Type:           req.Type,
		Value:          req.Value,
		MinOrderAmount: req.MinOrderAmount,
		MaxDiscount:    req.MaxDiscount,
		TotalLimit:     req.TotalLimit,
		PerUserLimit:   req.PerUserLimit,
		PlanIDs:        planIDsStr,
		StartAt:        req.StartAt,
		ExpireAt:       req.ExpireAt,
		IsActive:       req.IsActive,
	}

	if err := s.couponRepo.Create(ctx, repoCoupon); err != nil {
		s.logger.Error("Failed to create coupon", logger.Err(err))
		return nil, err
	}

	return s.toCoupon(repoCoupon), nil
}

// Update updates an existing coupon.
func (s *Service) Update(ctx context.Context, id int64, req *CreateCouponRequest) (*Coupon, error) {
	repoCoupon, err := s.couponRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrCouponNotFound
	}
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidCoupon)
	}
	if req.Type != TypeFixed && req.Type != TypePercentage {
		return nil, fmt.Errorf("%w: invalid coupon type", ErrInvalidCoupon)
	}
	if req.Value <= 0 {
		return nil, fmt.Errorf("%w: value must be positive", ErrInvalidCoupon)
	}
	if req.ExpireAt.Before(req.StartAt) {
		return nil, fmt.Errorf("%w: expire date must be after start date", ErrInvalidCoupon)
	}

	if req.Code != "" {
		repoCoupon.Code = strings.ToUpper(req.Code)
	}
	repoCoupon.Name = req.Name
	repoCoupon.Type = req.Type
	repoCoupon.Value = req.Value
	repoCoupon.MinOrderAmount = req.MinOrderAmount
	repoCoupon.MaxDiscount = req.MaxDiscount
	repoCoupon.TotalLimit = req.TotalLimit
	repoCoupon.PerUserLimit = req.PerUserLimit
	repoCoupon.PlanIDs = s.planIDsToString(req.PlanIDs)
	repoCoupon.StartAt = req.StartAt
	repoCoupon.ExpireAt = req.ExpireAt
	repoCoupon.IsActive = req.IsActive

	if err := s.couponRepo.Update(ctx, repoCoupon); err != nil {
		s.logger.Error("Failed to update coupon", logger.Err(err), logger.F("id", id))
		return nil, err
	}

	return s.toCoupon(repoCoupon), nil
}

// GetByID retrieves a coupon by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*Coupon, error) {
	repoCoupon, err := s.couponRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrCouponNotFound
	}
	return s.toCoupon(repoCoupon), nil
}

// GetByCode retrieves a coupon by code.
func (s *Service) GetByCode(ctx context.Context, code string) (*Coupon, error) {
	repoCoupon, err := s.couponRepo.GetByCode(ctx, strings.ToUpper(code))
	if err != nil {
		return nil, ErrCouponNotFound
	}
	return s.toCoupon(repoCoupon), nil
}

// Validate validates a coupon for use.
func (s *Service) Validate(ctx context.Context, code string, userID int64, planID int64, orderAmount int64) (*Coupon, int64, error) {
	coupon, err := s.GetByCode(ctx, code)
	if err != nil {
		return nil, 0, err
	}

	// Check if active
	if !coupon.IsActive {
		return nil, 0, ErrCouponInactive
	}

	// Check date validity
	now := time.Now()
	if now.Before(coupon.StartAt) {
		return nil, 0, ErrCouponNotStarted
	}
	if now.After(coupon.ExpireAt) {
		return nil, 0, ErrCouponExpired
	}

	// Check total usage limit
	if coupon.TotalLimit > 0 && coupon.UsedCount >= coupon.TotalLimit {
		return nil, 0, ErrCouponLimitReached
	}

	// Check per-user limit
	if coupon.PerUserLimit > 0 {
		userUsage, err := s.couponRepo.GetUserUsageCount(ctx, coupon.ID, userID)
		if err != nil {
			s.logger.Error("Failed to get user usage count", logger.Err(err))
			return nil, 0, err
		}
		if userUsage >= coupon.PerUserLimit {
			return nil, 0, ErrCouponUserLimit
		}
	}

	// Check minimum order amount
	if coupon.MinOrderAmount > 0 && orderAmount < coupon.MinOrderAmount {
		return nil, 0, ErrCouponMinAmount
	}

	// Check plan restriction
	if len(coupon.PlanIDs) > 0 {
		planAllowed := false
		for _, pid := range coupon.PlanIDs {
			if pid == planID {
				planAllowed = true
				break
			}
		}
		if !planAllowed {
			return nil, 0, ErrCouponPlanMismatch
		}
	}

	// Calculate discount
	discount := s.CalculateDiscount(coupon, orderAmount)

	return coupon, discount, nil
}

// CalculateDiscount calculates the discount amount for a coupon.
func (s *Service) CalculateDiscount(coupon *Coupon, orderAmount int64) int64 {
	var discount int64

	switch coupon.Type {
	case TypeFixed:
		discount = coupon.Value
	case TypePercentage:
		// Value is percentage * 100 (e.g., 1000 = 10%)
		discount = orderAmount * coupon.Value / 10000
		// Apply max discount limit
		if coupon.MaxDiscount > 0 && discount > coupon.MaxDiscount {
			discount = coupon.MaxDiscount
		}
	}

	// Discount cannot exceed order amount
	if discount > orderAmount {
		discount = orderAmount
	}

	return discount
}

// Use records coupon usage.
func (s *Service) Use(ctx context.Context, couponID, userID, orderID int64, discount int64) error {
	usage := &repository.CouponUsage{
		CouponID: couponID,
		UserID:   userID,
		OrderID:  orderID,
		Discount: discount,
		UsedAt:   time.Now(),
	}

	if err := s.couponRepo.CreateUsage(ctx, usage); err != nil {
		s.logger.Error("Failed to create coupon usage", logger.Err(err))
		return err
	}

	if err := s.couponRepo.IncrementUsedCount(ctx, couponID); err != nil {
		s.logger.Error("Failed to increment coupon used count", logger.Err(err))
		return err
	}

	return nil
}

// List lists coupons with filter and pagination.
func (s *Service) List(ctx context.Context, filter CouponFilter, page, pageSize int) ([]*Coupon, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoFilter := repository.CouponFilter{
		IsActive:  filter.IsActive,
		Type:      filter.Type,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
	}

	repoCoupons, total, err := s.couponRepo.List(ctx, repoFilter, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list coupons", logger.Err(err))
		return nil, 0, err
	}

	coupons := make([]*Coupon, len(repoCoupons))
	for i, rc := range repoCoupons {
		coupons[i] = s.toCoupon(rc)
	}

	return coupons, total, nil
}

// SetActive sets the active status of a coupon.
func (s *Service) SetActive(ctx context.Context, id int64, active bool) error {
	if err := s.couponRepo.SetActive(ctx, id, active); err != nil {
		s.logger.Error("Failed to set coupon active status", logger.Err(err), logger.F("id", id))
		return err
	}
	return nil
}

// Delete deletes a coupon.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.couponRepo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete coupon", logger.Err(err), logger.F("id", id))
		return err
	}
	return nil
}

// GenerateBatchCodes generates a batch of unique coupon codes.
func (s *Service) GenerateBatchCodes(prefix string, count int) ([]string, error) {
	if count <= 0 || count > 1000 {
		return nil, fmt.Errorf("count must be between 1 and 1000")
	}

	codes := make([]string, 0, count)
	generated := make(map[string]bool)

	for len(codes) < count {
		code := s.generateCode(prefix)
		if !generated[code] {
			generated[code] = true
			codes = append(codes, code)
		}
	}

	return codes, nil
}

// generateCode generates a single coupon code.
func (s *Service) generateCode(prefix string) string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	suffix := strings.ToUpper(hex.EncodeToString(bytes))

	if prefix != "" {
		return fmt.Sprintf("%s-%s", strings.ToUpper(prefix), suffix)
	}
	return suffix
}

// GetStatistics retrieves statistics for a coupon.
func (s *Service) GetStatistics(ctx context.Context, couponID int64) (usageCount int, totalDiscount int64, err error) {
	usageCount, err = s.couponRepo.GetUsageCount(ctx, couponID)
	if err != nil {
		return 0, 0, err
	}

	totalDiscount, err = s.couponRepo.GetTotalDiscountAmount(ctx, couponID)
	if err != nil {
		return 0, 0, err
	}

	return usageCount, totalDiscount, nil
}

// toCoupon converts a repository coupon to a service coupon.
func (s *Service) toCoupon(rc *repository.Coupon) *Coupon {
	return &Coupon{
		ID:             rc.ID,
		Code:           rc.Code,
		Name:           rc.Name,
		Type:           rc.Type,
		Value:          rc.Value,
		MinOrderAmount: rc.MinOrderAmount,
		MaxDiscount:    rc.MaxDiscount,
		TotalLimit:     rc.TotalLimit,
		PerUserLimit:   rc.PerUserLimit,
		UsedCount:      rc.UsedCount,
		PlanIDs:        s.stringToPlanIDs(rc.PlanIDs),
		StartAt:        rc.StartAt,
		ExpireAt:       rc.ExpireAt,
		IsActive:       rc.IsActive,
		CreatedAt:      rc.CreatedAt,
	}
}

// planIDsToString converts plan IDs slice to comma-separated string.
func (s *Service) planIDsToString(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(strs, ",")
}

// stringToPlanIDs converts comma-separated string to plan IDs slice.
func (s *Service) stringToPlanIDs(str string) []int64 {
	if str == "" {
		return nil
	}
	parts := strings.Split(str, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		var id int64
		if _, err := fmt.Sscanf(p, "%d", &id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
