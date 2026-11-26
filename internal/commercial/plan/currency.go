// Package plan provides plan management functionality.
package plan

import (
	"context"

	"v/internal/commercial/currency"
	"v/internal/database/repository"
	"v/internal/logger"
)

// PlanWithPrices represents a plan with multi-currency prices.
type PlanWithPrices struct {
	*Plan
	Prices       map[string]int64 `json:"prices"`        // currency -> price in cents
	DisplayPrice int64            `json:"display_price"` // price in user's currency
	Currency     string           `json:"currency"`      // user's currency
}

// CurrencyService provides currency-aware plan operations.
type CurrencyService struct {
	planService     *Service
	currencyService *currency.Service
	priceRepo       repository.PlanPriceRepository
	logger          logger.Logger
}

// NewCurrencyService creates a new currency-aware plan service.
func NewCurrencyService(
	planService *Service,
	currencyService *currency.Service,
	priceRepo repository.PlanPriceRepository,
	log logger.Logger,
) *CurrencyService {
	return &CurrencyService{
		planService:     planService,
		currencyService: currencyService,
		priceRepo:       priceRepo,
		logger:          log,
	}
}

// GetPlanWithPrices retrieves a plan with all its currency prices.
func (s *CurrencyService) GetPlanWithPrices(ctx context.Context, planID int64, userCurrency string) (*PlanWithPrices, error) {
	plan, err := s.planService.GetByID(ctx, planID)
	if err != nil {
		return nil, err
	}

	prices, err := s.getPricesMap(ctx, planID)
	if err != nil {
		s.logger.Error("Failed to get plan prices", logger.Err(err), logger.F("plan_id", planID))
		prices = make(map[string]int64)
	}

	// Add base price if not in prices map
	baseCurrency := s.currencyService.GetBaseCurrency()
	if _, ok := prices[baseCurrency]; !ok {
		prices[baseCurrency] = plan.Price
	}

	// Get display price in user's currency
	displayPrice, err := s.GetPriceInCurrency(ctx, planID, userCurrency)
	if err != nil {
		displayPrice = plan.Price
		userCurrency = baseCurrency
	}

	return &PlanWithPrices{
		Plan:         plan,
		Prices:       prices,
		DisplayPrice: displayPrice,
		Currency:     userCurrency,
	}, nil
}

// ListPlansWithPrices lists all active plans with prices in user's currency.
func (s *CurrencyService) ListPlansWithPrices(ctx context.Context, userCurrency string) ([]*PlanWithPrices, error) {
	plans, err := s.planService.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*PlanWithPrices, len(plans))
	for i, plan := range plans {
		planWithPrices, err := s.GetPlanWithPrices(ctx, plan.ID, userCurrency)
		if err != nil {
			s.logger.Error("Failed to get plan with prices", logger.Err(err), logger.F("plan_id", plan.ID))
			// Use base price as fallback
			result[i] = &PlanWithPrices{
				Plan:         plan,
				Prices:       map[string]int64{s.currencyService.GetBaseCurrency(): plan.Price},
				DisplayPrice: plan.Price,
				Currency:     s.currencyService.GetBaseCurrency(),
			}
			continue
		}
		result[i] = planWithPrices
	}

	return result, nil
}

// GetPriceInCurrency gets the price for a plan in a specific currency.
// If a specific price is set for the currency, it returns that.
// Otherwise, it converts from the base price.
func (s *CurrencyService) GetPriceInCurrency(ctx context.Context, planID int64, targetCurrency string) (int64, error) {
	// First, try to get a specific price for this currency
	price, err := s.priceRepo.GetPrice(ctx, planID, targetCurrency)
	if err == nil {
		return price.Price, nil
	}

	// If no specific price, convert from base price
	plan, err := s.planService.GetByID(ctx, planID)
	if err != nil {
		return 0, err
	}

	baseCurrency := s.currencyService.GetBaseCurrency()
	if targetCurrency == baseCurrency {
		return plan.Price, nil
	}

	return s.currencyService.Convert(ctx, plan.Price, baseCurrency, targetCurrency)
}

// SetPriceInCurrency sets a specific price for a plan in a currency.
func (s *CurrencyService) SetPriceInCurrency(ctx context.Context, planID int64, currencyCode string, price int64) error {
	// Verify plan exists
	_, err := s.planService.GetByID(ctx, planID)
	if err != nil {
		return err
	}

	// Verify currency is supported
	if !s.currencyService.IsCurrencySupported(currencyCode) {
		return currency.ErrCurrencyNotSupported
	}

	return s.priceRepo.SetPrice(ctx, planID, currencyCode, price)
}

// SetPricesForPlan sets multiple currency prices for a plan.
func (s *CurrencyService) SetPricesForPlan(ctx context.Context, planID int64, prices map[string]int64) error {
	// Verify plan exists
	_, err := s.planService.GetByID(ctx, planID)
	if err != nil {
		return err
	}

	// Verify all currencies are supported
	for currencyCode := range prices {
		if !s.currencyService.IsCurrencySupported(currencyCode) {
			return currency.ErrCurrencyNotSupported
		}
	}

	return s.priceRepo.BatchSetPrices(ctx, planID, prices)
}

// DeletePriceInCurrency removes a specific currency price for a plan.
func (s *CurrencyService) DeletePriceInCurrency(ctx context.Context, planID int64, currencyCode string) error {
	return s.priceRepo.DeletePrice(ctx, planID, currencyCode)
}

// GetAllPricesForPlan retrieves all currency prices for a plan.
func (s *CurrencyService) GetAllPricesForPlan(ctx context.Context, planID int64) (map[string]int64, error) {
	return s.getPricesMap(ctx, planID)
}

// getPricesMap retrieves all prices for a plan as a map.
func (s *CurrencyService) getPricesMap(ctx context.Context, planID int64) (map[string]int64, error) {
	prices, err := s.priceRepo.GetPricesForPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, p := range prices {
		result[p.Currency] = p.Price
	}

	return result, nil
}

// CalculateMonthlyPriceInCurrency calculates the monthly price in a specific currency.
func (s *CurrencyService) CalculateMonthlyPriceInCurrency(ctx context.Context, planID int64, targetCurrency string) (int64, error) {
	plan, err := s.planService.GetByID(ctx, planID)
	if err != nil {
		return 0, err
	}

	price, err := s.GetPriceInCurrency(ctx, planID, targetCurrency)
	if err != nil {
		return 0, err
	}

	if plan.Duration <= 0 {
		return 0, nil
	}

	return (price * 30) / int64(plan.Duration), nil
}

// FormatPlanPrice formats a plan's price for display.
func (s *CurrencyService) FormatPlanPrice(ctx context.Context, planID int64, targetCurrency string) (string, error) {
	price, err := s.GetPriceInCurrency(ctx, planID, targetCurrency)
	if err != nil {
		return "", err
	}

	return s.currencyService.FormatPrice(price, targetCurrency), nil
}
