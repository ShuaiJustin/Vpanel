// Package repository provides data access layer implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PlanPrice represents a price for a plan in a specific currency.
type PlanPrice struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	PlanID    int64     `json:"plan_id" gorm:"index;not null;uniqueIndex:idx_plan_price_unique,priority:1"`
	Currency  string    `json:"currency" gorm:"size:3;not null;uniqueIndex:idx_plan_price_unique,priority:2"`
	Price     int64     `json:"price" gorm:"not null"` // cents
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for PlanPrice.
func (PlanPrice) TableName() string {
	return "plan_prices"
}

// PlanPriceRepository defines the interface for plan price data access.
type PlanPriceRepository interface {
	// GetPrice retrieves the price for a plan in a specific currency.
	GetPrice(ctx context.Context, planID int64, currency string) (*PlanPrice, error)
	// GetPricesForPlan retrieves all prices for a plan.
	GetPricesForPlan(ctx context.Context, planID int64) ([]*PlanPrice, error)
	// SetPrice sets or updates the price for a plan in a specific currency.
	SetPrice(ctx context.Context, planID int64, currency string, price int64) error
	// DeletePrice deletes a price for a plan in a specific currency.
	DeletePrice(ctx context.Context, planID int64, currency string) error
	// DeletePricesForPlan deletes all prices for a plan.
	DeletePricesForPlan(ctx context.Context, planID int64) error
	// GetPricesForCurrency retrieves all plan prices in a specific currency.
	GetPricesForCurrency(ctx context.Context, currency string) ([]*PlanPrice, error)
	// BatchSetPrices sets multiple prices for a plan.
	BatchSetPrices(ctx context.Context, planID int64, prices map[string]int64) error
}

// planPriceRepository implements PlanPriceRepository.
type planPriceRepository struct {
	db *gorm.DB
}

// NewPlanPriceRepository creates a new plan price repository.
func NewPlanPriceRepository(db *gorm.DB) PlanPriceRepository {
	return &planPriceRepository{db: db}
}

// GetPrice retrieves the price for a plan in a specific currency.
func (r *planPriceRepository) GetPrice(ctx context.Context, planID int64, currency string) (*PlanPrice, error) {
	var price PlanPrice
	err := r.db.WithContext(ctx).
		Where("plan_id = ? AND currency = ?", planID, currency).
		First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// GetPricesForPlan retrieves all prices for a plan.
func (r *planPriceRepository) GetPricesForPlan(ctx context.Context, planID int64) ([]*PlanPrice, error) {
	var prices []*PlanPrice
	err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Order("currency").
		Find(&prices).Error
	if err != nil {
		return nil, err
	}
	return prices, nil
}

// SetPrice sets or updates the price for a plan in a specific currency.
func (r *planPriceRepository) SetPrice(ctx context.Context, planID int64, currency string, price int64) error {
	now := time.Now()
	planPrice := PlanPrice{
		PlanID:    planID,
		Currency:  currency,
		Price:     price,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "plan_id"}, {Name: "currency"}},
			DoUpdates: clause.AssignmentColumns([]string{"price", "updated_at"}),
		}).
		Create(&planPrice).Error
}

// DeletePrice deletes a price for a plan in a specific currency.
func (r *planPriceRepository) DeletePrice(ctx context.Context, planID int64, currency string) error {
	return r.db.WithContext(ctx).
		Where("plan_id = ? AND currency = ?", planID, currency).
		Delete(&PlanPrice{}).Error
}

// DeletePricesForPlan deletes all prices for a plan.
func (r *planPriceRepository) DeletePricesForPlan(ctx context.Context, planID int64) error {
	return r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Delete(&PlanPrice{}).Error
}

// GetPricesForCurrency retrieves all plan prices in a specific currency.
func (r *planPriceRepository) GetPricesForCurrency(ctx context.Context, currency string) ([]*PlanPrice, error) {
	var prices []*PlanPrice
	err := r.db.WithContext(ctx).
		Where("currency = ?", currency).
		Find(&prices).Error
	if err != nil {
		return nil, err
	}
	return prices, nil
}

// BatchSetPrices sets multiple prices for a plan.
func (r *planPriceRepository) BatchSetPrices(ctx context.Context, planID int64, prices map[string]int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for currency, price := range prices {
			planPrice := PlanPrice{
				PlanID:    planID,
				Currency:  currency,
				Price:     price,
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "plan_id"}, {Name: "currency"}},
				DoUpdates: clause.AssignmentColumns([]string{"price", "updated_at"}),
			}).Create(&planPrice).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
