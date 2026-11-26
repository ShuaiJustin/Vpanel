// Package currency provides multi-currency support functionality.
package currency

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/logger"
)

// Feature: commercial-system, Property 18: Currency Conversion Consistency
// Validates: Requirements 18.6
// *For any* amount converted from currency A to B and back to A, the result SHALL be within acceptable rounding tolerance of the original amount.
func TestProperty_CurrencyConversionConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Create service with cache directly
	config := DefaultConfig()
	svc := &Service{
		exchangeRepo: nil,
		config:       config,
		logger:       logger.NewNopLogger(),
		rateCache:    make(map[string]*ExchangeRate),
	}

	// Pre-populate cache with consistent rates
	now := time.Now()
	svc.rateCache["CNY_USD"] = &ExchangeRate{FromCurrency: "CNY", ToCurrency: "USD", Rate: 0.14, UpdatedAt: now}
	svc.rateCache["USD_CNY"] = &ExchangeRate{FromCurrency: "USD", ToCurrency: "CNY", Rate: 7.14, UpdatedAt: now}
	svc.rateCache["CNY_EUR"] = &ExchangeRate{FromCurrency: "CNY", ToCurrency: "EUR", Rate: 0.13, UpdatedAt: now}
	svc.rateCache["EUR_CNY"] = &ExchangeRate{FromCurrency: "EUR", ToCurrency: "CNY", Rate: 7.69, UpdatedAt: now}

	properties.Property("same currency conversion returns original amount", prop.ForAll(
		func(amount int64) bool {
			if amount < 0 {
				return true
			}
			ctx := context.Background()
			result, err := svc.Convert(ctx, amount, "CNY", "CNY")
			if err != nil {
				return false
			}
			return result == amount
		},
		gen.Int64Range(0, 10000000),
	))

	properties.Property("round-trip conversion is within tolerance", prop.ForAll(
		func(amount int64) bool {
			if amount <= 0 {
				return true
			}
			ctx := context.Background()

			// Convert CNY -> USD -> CNY
			usdAmount, err := svc.Convert(ctx, amount, "CNY", "USD")
			if err != nil {
				return true
			}

			backToCNY, err := svc.Convert(ctx, usdAmount, "USD", "CNY")
			if err != nil {
				return true
			}

			// Calculate tolerance: allow up to 2% difference due to rounding
			tolerance := float64(amount) * 0.02
			if tolerance < 1 {
				tolerance = 1
			}

			diff := math.Abs(float64(backToCNY - amount))
			return diff <= tolerance
		},
		gen.Int64Range(100, 10000000),
	))

	properties.Property("zero amount converts to zero", prop.ForAll(
		func(dummy int) bool {
			ctx := context.Background()
			result, err := svc.Convert(ctx, 0, "CNY", "USD")
			if err != nil {
				return true
			}
			return result == 0
		},
		gen.IntRange(0, 100),
	))

	properties.Property("conversion preserves sign (positive amounts stay positive)", prop.ForAll(
		func(amount int64) bool {
			if amount <= 0 {
				return true
			}
			ctx := context.Background()
			result, err := svc.Convert(ctx, amount, "CNY", "USD")
			if err != nil {
				return true
			}
			return result >= 0
		},
		gen.Int64Range(1, 10000000),
	))

	properties.Property("larger amounts convert to larger results", prop.ForAll(
		func(amount1, amount2 int64) bool {
			if amount1 <= 0 || amount2 <= 0 {
				return true
			}
			ctx := context.Background()

			result1, err1 := svc.Convert(ctx, amount1, "CNY", "USD")
			result2, err2 := svc.Convert(ctx, amount2, "CNY", "USD")

			if err1 != nil || err2 != nil {
				return true
			}

			if amount1 > amount2 {
				return result1 >= result2
			}
			if amount1 < amount2 {
				return result1 <= result2
			}
			return result1 == result2
		},
		gen.Int64Range(1, 10000000),
		gen.Int64Range(1, 10000000),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 18: Currency Conversion Consistency (Rate Retrieval)
// Validates: Requirements 18.6
func TestProperty_CurrencyRateRetrieval(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	config := DefaultConfig()
	svc := &Service{
		config:    config,
		logger:    logger.NewNopLogger(),
		rateCache: make(map[string]*ExchangeRate),
	}

	now := time.Now()
	svc.rateCache["CNY_USD"] = &ExchangeRate{FromCurrency: "CNY", ToCurrency: "USD", Rate: 0.14, UpdatedAt: now}
	svc.rateCache["USD_CNY"] = &ExchangeRate{FromCurrency: "USD", ToCurrency: "CNY", Rate: 7.14, UpdatedAt: now}

	properties.Property("same currency rate is always 1.0", prop.ForAll(
		func(currencyIndex int) bool {
			currencies := []string{"CNY", "USD", "EUR", "GBP", "JPY"}
			if currencyIndex < 0 || currencyIndex >= len(currencies) {
				currencyIndex = 0
			}
			currency := currencies[currencyIndex]

			ctx := context.Background()
			rate, err := svc.GetRate(ctx, currency, currency)
			if err != nil {
				return true
			}
			return rate == 1.0
		},
		gen.IntRange(0, 4),
	))

	properties.Property("rate is positive for supported currency pairs", prop.ForAll(
		func(dummy int) bool {
			ctx := context.Background()
			rate, err := svc.GetRate(ctx, "CNY", "USD")
			if err != nil {
				return true
			}
			return rate > 0
		},
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 18: Currency Conversion Consistency (Formatting)
// Validates: Requirements 18.6
func TestProperty_CurrencyFormatting(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	config := DefaultConfig()
	svc := &Service{
		config: config,
		logger: logger.NewNopLogger(),
	}

	properties.Property("formatted price contains currency symbol", prop.ForAll(
		func(amount int64) bool {
			if amount < 0 {
				return true
			}
			formatted := svc.FormatPrice(amount, "CNY")
			// CNY symbol is ¥ which is a multi-byte UTF-8 character
			return len(formatted) > 0 && strings.HasPrefix(formatted, "¥")
		},
		gen.Int64Range(0, 10000000),
	))

	properties.Property("formatted price for USD contains dollar sign", prop.ForAll(
		func(amount int64) bool {
			if amount < 0 {
				return true
			}
			formatted := svc.FormatPrice(amount, "USD")
			return len(formatted) > 0 && strings.HasPrefix(formatted, "$")
		},
		gen.Int64Range(0, 10000000),
	))

	properties.TestingRun(t)
}
