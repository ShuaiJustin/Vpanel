// Package repository provides data access layer implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExchangeRate represents an exchange rate record in the database.
type ExchangeRate struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	FromCurrency string    `json:"from_currency" gorm:"size:3;not null;uniqueIndex:idx_exchange_rate_pair,priority:1"`
	ToCurrency   string    `json:"to_currency" gorm:"size:3;not null;uniqueIndex:idx_exchange_rate_pair,priority:2"`
	Rate         float64   `json:"rate" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}

// TableName returns the table name for ExchangeRate.
func (ExchangeRate) TableName() string {
	return "exchange_rates"
}

// ExchangeRateRepository defines the interface for exchange rate data access.
type ExchangeRateRepository interface {
	// GetRate retrieves the exchange rate between two currencies.
	GetRate(ctx context.Context, from, to string) (*ExchangeRate, error)
	// SaveRate saves or updates an exchange rate.
	SaveRate(ctx context.Context, from, to string, rate float64, updatedAt time.Time) error
	// GetAllRates retrieves all exchange rates.
	GetAllRates(ctx context.Context) ([]*ExchangeRate, error)
	// DeleteRate deletes an exchange rate.
	DeleteRate(ctx context.Context, from, to string) error
	// GetRatesForCurrency retrieves all rates for a specific currency.
	GetRatesForCurrency(ctx context.Context, currency string) ([]*ExchangeRate, error)
}

// exchangeRateRepository implements ExchangeRateRepository.
type exchangeRateRepository struct {
	db *gorm.DB
}

// NewExchangeRateRepository creates a new exchange rate repository.
func NewExchangeRateRepository(db *gorm.DB) ExchangeRateRepository {
	return &exchangeRateRepository{db: db}
}

// GetRate retrieves the exchange rate between two currencies.
func (r *exchangeRateRepository) GetRate(ctx context.Context, from, to string) (*ExchangeRate, error) {
	var rate ExchangeRate
	err := r.db.WithContext(ctx).
		Where("from_currency = ? AND to_currency = ?", from, to).
		First(&rate).Error
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

// SaveRate saves or updates an exchange rate.
func (r *exchangeRateRepository) SaveRate(ctx context.Context, from, to string, rate float64, updatedAt time.Time) error {
	exchangeRate := ExchangeRate{
		FromCurrency: from,
		ToCurrency:   to,
		Rate:         rate,
		UpdatedAt:    updatedAt,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "from_currency"}, {Name: "to_currency"}},
			DoUpdates: clause.AssignmentColumns([]string{"rate", "updated_at"}),
		}).
		Create(&exchangeRate).Error
}

// GetAllRates retrieves all exchange rates.
func (r *exchangeRateRepository) GetAllRates(ctx context.Context) ([]*ExchangeRate, error) {
	var rates []*ExchangeRate
	err := r.db.WithContext(ctx).
		Order("from_currency, to_currency").
		Find(&rates).Error
	if err != nil {
		return nil, err
	}
	return rates, nil
}

// DeleteRate deletes an exchange rate.
func (r *exchangeRateRepository) DeleteRate(ctx context.Context, from, to string) error {
	return r.db.WithContext(ctx).
		Where("from_currency = ? AND to_currency = ?", from, to).
		Delete(&ExchangeRate{}).Error
}

// GetRatesForCurrency retrieves all rates for a specific currency.
func (r *exchangeRateRepository) GetRatesForCurrency(ctx context.Context, currency string) ([]*ExchangeRate, error) {
	var rates []*ExchangeRate
	err := r.db.WithContext(ctx).
		Where("from_currency = ? OR to_currency = ?", currency, currency).
		Find(&rates).Error
	if err != nil {
		return nil, err
	}
	return rates, nil
}
