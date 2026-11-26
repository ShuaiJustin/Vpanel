// Package currency provides multi-currency support functionality.
package currency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/ip"
	"v/internal/logger"
)

// Common errors
var (
	ErrCurrencyNotSupported = errors.New("currency not supported")
	ErrExchangeRateNotFound = errors.New("exchange rate not found")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrAPIUnavailable       = errors.New("exchange rate API unavailable")
)

// Config holds currency service configuration.
type Config struct {
	BaseCurrency          string   `json:"base_currency"`           // e.g., "CNY"
	SupportedCurrencies   []string `json:"supported_currencies"`    // e.g., ["CNY", "USD", "EUR"]
	ExchangeRateAPIURL    string   `json:"exchange_rate_api_url"`   // API endpoint
	ExchangeRateAPIKey    string   `json:"exchange_rate_api_key"`   // API key
	CacheTTL              int      `json:"cache_ttl"`               // minutes
	DefaultCurrency       string   `json:"default_currency"`        // fallback currency
	RoundingPrecision     int      `json:"rounding_precision"`      // decimal places
}

// DefaultConfig returns default currency configuration.
func DefaultConfig() *Config {
	return &Config{
		BaseCurrency:        "CNY",
		SupportedCurrencies: []string{"CNY", "USD", "EUR", "GBP", "JPY", "KRW", "HKD", "TWD", "SGD", "AUD"},
		ExchangeRateAPIURL:  "https://api.exchangerate-api.com/v4/latest",
		CacheTTL:            60, // 1 hour
		DefaultCurrency:     "CNY",
		RoundingPrecision:   2,
	}
}

// ExchangeRate represents an exchange rate between two currencies.
type ExchangeRate struct {
	ID           int64     `json:"id"`
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CurrencyInfo provides information about a currency.
type CurrencyInfo struct {
	Code     string `json:"code"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Decimals int    `json:"decimals"`
}

// Service provides currency conversion and management operations.
type Service struct {
	exchangeRepo repository.ExchangeRateRepository
	geoService   *ip.GeolocationService
	config       *Config
	logger       logger.Logger
	httpClient   *http.Client
	rateCache    map[string]*ExchangeRate
	cacheMu      sync.RWMutex
}

// NewService creates a new currency service.
func NewService(
	exchangeRepo repository.ExchangeRateRepository,
	geoService *ip.GeolocationService,
	config *Config,
	log logger.Logger,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}

	return &Service{
		exchangeRepo: exchangeRepo,
		geoService:   geoService,
		config:       config,
		logger:       log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateCache: make(map[string]*ExchangeRate),
	}
}

// GetRate retrieves the exchange rate between two currencies.
func (s *Service) GetRate(ctx context.Context, from, to string) (float64, error) {
	if from == to {
		return 1.0, nil
	}

	if !s.IsCurrencySupported(from) {
		return 0, fmt.Errorf("%w: %s", ErrCurrencyNotSupported, from)
	}
	if !s.IsCurrencySupported(to) {
		return 0, fmt.Errorf("%w: %s", ErrCurrencyNotSupported, to)
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s_%s", from, to)
	s.cacheMu.RLock()
	cached, ok := s.rateCache[cacheKey]
	s.cacheMu.RUnlock()

	if ok && time.Since(cached.UpdatedAt) < time.Duration(s.config.CacheTTL)*time.Minute {
		return cached.Rate, nil
	}

	// Try to get from database
	rate, err := s.exchangeRepo.GetRate(ctx, from, to)
	if err == nil && time.Since(rate.UpdatedAt) < time.Duration(s.config.CacheTTL)*time.Minute {
		s.updateCache(from, to, rate.Rate, rate.UpdatedAt)
		return rate.Rate, nil
	}

	// Fetch from API if not found or expired
	apiRate, err := s.fetchRateFromAPI(from, to)
	if err != nil {
		// Return stale rate if available
		if rate != nil {
			return rate.Rate, nil
		}
		return 0, err
	}

	// Save to database and cache
	now := time.Now()
	if err := s.exchangeRepo.SaveRate(ctx, from, to, apiRate, now); err != nil {
		s.logger.Error("Failed to save exchange rate", logger.Err(err))
	}
	s.updateCache(from, to, apiRate, now)

	return apiRate, nil
}

// Convert converts an amount from one currency to another.
// Amount is in cents (smallest currency unit).
func (s *Service) Convert(ctx context.Context, amount int64, from, to string) (int64, error) {
	if amount < 0 {
		return 0, ErrInvalidAmount
	}
	if amount == 0 {
		return 0, nil
	}
	if from == to {
		return amount, nil
	}

	rate, err := s.GetRate(ctx, from, to)
	if err != nil {
		return 0, err
	}

	// Convert and round
	converted := float64(amount) * rate
	return s.roundAmount(converted, to), nil
}

// ConvertToBase converts an amount to the base currency.
func (s *Service) ConvertToBase(ctx context.Context, amount int64, from string) (int64, error) {
	return s.Convert(ctx, amount, from, s.config.BaseCurrency)
}

// ConvertFromBase converts an amount from the base currency.
func (s *Service) ConvertFromBase(ctx context.Context, amount int64, to string) (int64, error) {
	return s.Convert(ctx, amount, s.config.BaseCurrency, to)
}

// UpdateRates fetches and updates all exchange rates from the API.
func (s *Service) UpdateRates(ctx context.Context) error {
	s.logger.Info("Updating exchange rates")

	for _, currency := range s.config.SupportedCurrencies {
		if currency == s.config.BaseCurrency {
			continue
		}

		// Get rate from base to this currency
		rate, err := s.fetchRateFromAPI(s.config.BaseCurrency, currency)
		if err != nil {
			s.logger.Error("Failed to fetch rate", logger.Err(err), logger.F("currency", currency))
			continue
		}

		now := time.Now()
		if err := s.exchangeRepo.SaveRate(ctx, s.config.BaseCurrency, currency, rate, now); err != nil {
			s.logger.Error("Failed to save rate", logger.Err(err), logger.F("currency", currency))
			continue
		}
		s.updateCache(s.config.BaseCurrency, currency, rate, now)

		// Also save reverse rate
		reverseRate := 1.0 / rate
		if err := s.exchangeRepo.SaveRate(ctx, currency, s.config.BaseCurrency, reverseRate, now); err != nil {
			s.logger.Error("Failed to save reverse rate", logger.Err(err), logger.F("currency", currency))
		}
		s.updateCache(currency, s.config.BaseCurrency, reverseRate, now)
	}

	s.logger.Info("Exchange rates updated successfully")
	return nil
}

// FormatPrice formats a price amount for display.
func (s *Service) FormatPrice(amount int64, currency string) string {
	info := s.GetCurrencyInfo(currency)
	if info == nil {
		info = s.GetCurrencyInfo(s.config.DefaultCurrency)
	}

	// Convert cents to main unit
	divisor := math.Pow(10, float64(info.Decimals))
	value := float64(amount) / divisor

	// Format based on currency
	format := fmt.Sprintf("%%.%df", info.Decimals)
	formatted := fmt.Sprintf(format, value)

	return fmt.Sprintf("%s%s", info.Symbol, formatted)
}

// DetectCurrency detects the preferred currency based on IP address.
func (s *Service) DetectCurrency(ctx context.Context, ipAddr string) string {
	if s.geoService == nil {
		return s.config.DefaultCurrency
	}

	geoInfo, err := s.geoService.Lookup(ctx, ipAddr)
	if err != nil || geoInfo == nil {
		return s.config.DefaultCurrency
	}

	currency := s.countryToCurrency(geoInfo.CountryCode)
	if s.IsCurrencySupported(currency) {
		return currency
	}

	return s.config.DefaultCurrency
}

// IsCurrencySupported checks if a currency is supported.
func (s *Service) IsCurrencySupported(currency string) bool {
	for _, c := range s.config.SupportedCurrencies {
		if c == currency {
			return true
		}
	}
	return false
}

// GetSupportedCurrencies returns the list of supported currencies.
func (s *Service) GetSupportedCurrencies() []string {
	return s.config.SupportedCurrencies
}

// GetBaseCurrency returns the base currency.
func (s *Service) GetBaseCurrency() string {
	return s.config.BaseCurrency
}

// GetCurrencyInfo returns information about a currency.
func (s *Service) GetCurrencyInfo(code string) *CurrencyInfo {
	info, ok := currencyInfoMap[code]
	if !ok {
		return nil
	}
	return &info
}

// GetAllCurrencyInfo returns information about all supported currencies.
func (s *Service) GetAllCurrencyInfo() []*CurrencyInfo {
	var result []*CurrencyInfo
	for _, code := range s.config.SupportedCurrencies {
		if info := s.GetCurrencyInfo(code); info != nil {
			result = append(result, info)
		}
	}
	return result
}

// fetchRateFromAPI fetches exchange rate from external API.
func (s *Service) fetchRateFromAPI(from, to string) (float64, error) {
	url := fmt.Sprintf("%s/%s", s.config.ExchangeRateAPIURL, from)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	if s.config.ExchangeRateAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.ExchangeRateAPIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrAPIUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%w: status %d", ErrAPIUnavailable, resp.StatusCode)
	}

	var result struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	rate, ok := result.Rates[to]
	if !ok {
		return 0, fmt.Errorf("%w: %s to %s", ErrExchangeRateNotFound, from, to)
	}

	return rate, nil
}

// updateCache updates the rate cache.
func (s *Service) updateCache(from, to string, rate float64, updatedAt time.Time) {
	cacheKey := fmt.Sprintf("%s_%s", from, to)
	s.cacheMu.Lock()
	s.rateCache[cacheKey] = &ExchangeRate{
		FromCurrency: from,
		ToCurrency:   to,
		Rate:         rate,
		UpdatedAt:    updatedAt,
	}
	s.cacheMu.Unlock()
}

// roundAmount rounds an amount based on currency precision.
func (s *Service) roundAmount(amount float64, currency string) int64 {
	info := s.GetCurrencyInfo(currency)
	if info == nil {
		return int64(math.Round(amount))
	}

	// Round to the nearest cent
	return int64(math.Round(amount))
}

// countryToCurrency maps country codes to currencies.
func (s *Service) countryToCurrency(countryCode string) string {
	currency, ok := countryToCurrencyMap[countryCode]
	if !ok {
		return s.config.DefaultCurrency
	}
	return currency
}

// Currency information map
var currencyInfoMap = map[string]CurrencyInfo{
	"CNY": {Code: "CNY", Symbol: "¥", Name: "Chinese Yuan", Decimals: 2},
	"USD": {Code: "USD", Symbol: "$", Name: "US Dollar", Decimals: 2},
	"EUR": {Code: "EUR", Symbol: "€", Name: "Euro", Decimals: 2},
	"GBP": {Code: "GBP", Symbol: "£", Name: "British Pound", Decimals: 2},
	"JPY": {Code: "JPY", Symbol: "¥", Name: "Japanese Yen", Decimals: 0},
	"KRW": {Code: "KRW", Symbol: "₩", Name: "South Korean Won", Decimals: 0},
	"HKD": {Code: "HKD", Symbol: "HK$", Name: "Hong Kong Dollar", Decimals: 2},
	"TWD": {Code: "TWD", Symbol: "NT$", Name: "Taiwan Dollar", Decimals: 2},
	"SGD": {Code: "SGD", Symbol: "S$", Name: "Singapore Dollar", Decimals: 2},
	"AUD": {Code: "AUD", Symbol: "A$", Name: "Australian Dollar", Decimals: 2},
	"CAD": {Code: "CAD", Symbol: "C$", Name: "Canadian Dollar", Decimals: 2},
	"RUB": {Code: "RUB", Symbol: "₽", Name: "Russian Ruble", Decimals: 2},
	"INR": {Code: "INR", Symbol: "₹", Name: "Indian Rupee", Decimals: 2},
	"BRL": {Code: "BRL", Symbol: "R$", Name: "Brazilian Real", Decimals: 2},
	"MYR": {Code: "MYR", Symbol: "RM", Name: "Malaysian Ringgit", Decimals: 2},
	"THB": {Code: "THB", Symbol: "฿", Name: "Thai Baht", Decimals: 2},
	"VND": {Code: "VND", Symbol: "₫", Name: "Vietnamese Dong", Decimals: 0},
	"PHP": {Code: "PHP", Symbol: "₱", Name: "Philippine Peso", Decimals: 2},
	"IDR": {Code: "IDR", Symbol: "Rp", Name: "Indonesian Rupiah", Decimals: 0},
}

// Country to currency mapping
var countryToCurrencyMap = map[string]string{
	"CN": "CNY", "US": "USD", "GB": "GBP", "DE": "EUR", "FR": "EUR",
	"IT": "EUR", "ES": "EUR", "NL": "EUR", "BE": "EUR", "AT": "EUR",
	"PT": "EUR", "IE": "EUR", "FI": "EUR", "GR": "EUR", "JP": "JPY",
	"KR": "KRW", "HK": "HKD", "TW": "TWD", "SG": "SGD", "AU": "AUD",
	"CA": "CAD", "RU": "RUB", "IN": "INR", "BR": "BRL", "MY": "MYR",
	"TH": "THB", "VN": "VND", "PH": "PHP", "ID": "IDR",
}
