// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// GiftCard represents a gift card in the database.
type GiftCard struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	Code        string     `gorm:"uniqueIndex;size:32;not null"`
	Value       int64      `gorm:"not null"`
	Status      string     `gorm:"size:32;default:active;index"`
	CreatedBy   *int64     `gorm:"index"`
	PurchasedBy *int64     `gorm:"index"`
	RedeemedBy  *int64     `gorm:"index"`
	BatchID     string     `gorm:"size:64;index"`
	ExpiresAt   *time.Time `gorm:"index"`
	RedeemedAt  *time.Time
	PurchasedAt *time.Time
	CreatedAt   time.Time `gorm:"autoCreateTime"`

	Creator   *User `gorm:"foreignKey:CreatedBy"`
	Purchaser *User `gorm:"foreignKey:PurchasedBy"`
	Redeemer  *User `gorm:"foreignKey:RedeemedBy"`
}

// TableName returns the table name for GiftCard.
func (GiftCard) TableName() string {
	return "gift_cards"
}

// GiftCard status constants
const (
	GiftCardStatusActive   = "active"
	GiftCardStatusRedeemed = "redeemed"
	GiftCardStatusExpired  = "expired"
	GiftCardStatusDisabled = "disabled"
)

// GiftCardFilter defines filter options for listing gift cards.
type GiftCardFilter struct {
	Status    string
	BatchID   string
	CreatedBy *int64
	MinValue  *int64
	MaxValue  *int64
}

// GiftCardRepository defines the interface for gift card data access.
type GiftCardRepository interface {
	// CRUD operations
	Create(ctx context.Context, giftCard *GiftCard) error
	CreateBatch(ctx context.Context, giftCards []*GiftCard) error
	GetByID(ctx context.Context, id int64) (*GiftCard, error)
	GetByCode(ctx context.Context, code string) (*GiftCard, error)
	Update(ctx context.Context, giftCard *GiftCard) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter GiftCardFilter, limit, offset int) ([]*GiftCard, int64, error)

	// Status operations
	SetStatus(ctx context.Context, id int64, status string) error
	MarkRedeemed(ctx context.Context, id int64, userID int64) error
	MarkPurchased(ctx context.Context, id int64, userID int64) error

	// Query operations
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*GiftCard, int64, error)
	ListByBatch(ctx context.Context, batchID string, limit, offset int) ([]*GiftCard, int64, error)
	GetExpiredActive(ctx context.Context) ([]*GiftCard, error)

	// Statistics
	Count(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	GetTotalValue(ctx context.Context, status string) (int64, error)
	GetBatchStats(ctx context.Context, batchID string) (total int, redeemed int, totalValue int64, redeemedValue int64, err error)
}

// giftCardRepository implements GiftCardRepository.
type giftCardRepository struct {
	db *gorm.DB
}

// NewGiftCardRepository creates a new gift card repository.
func NewGiftCardRepository(db *gorm.DB) GiftCardRepository {
	return &giftCardRepository{db: db}
}

// Create creates a new gift card.
func (r *giftCardRepository) Create(ctx context.Context, giftCard *GiftCard) error {
	return r.db.WithContext(ctx).Create(giftCard).Error
}

// CreateBatch creates multiple gift cards in a single transaction.
func (r *giftCardRepository) CreateBatch(ctx context.Context, giftCards []*GiftCard) error {
	return r.db.WithContext(ctx).CreateInBatches(giftCards, 100).Error
}

// GetByID retrieves a gift card by ID.
func (r *giftCardRepository) GetByID(ctx context.Context, id int64) (*GiftCard, error) {
	var giftCard GiftCard
	err := r.db.WithContext(ctx).First(&giftCard, id).Error
	if err != nil {
		return nil, err
	}
	return &giftCard, nil
}

// GetByCode retrieves a gift card by code.
func (r *giftCardRepository) GetByCode(ctx context.Context, code string) (*GiftCard, error) {
	var giftCard GiftCard
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&giftCard).Error
	if err != nil {
		return nil, err
	}
	return &giftCard, nil
}

// Update updates a gift card.
func (r *giftCardRepository) Update(ctx context.Context, giftCard *GiftCard) error {
	return r.db.WithContext(ctx).Save(giftCard).Error
}

// Delete deletes a gift card by ID.
func (r *giftCardRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&GiftCard{}, id).Error
}

// List lists gift cards with filter and pagination.
func (r *giftCardRepository) List(ctx context.Context, filter GiftCardFilter, limit, offset int) ([]*GiftCard, int64, error) {
	var giftCards []*GiftCard
	var total int64

	query := r.db.WithContext(ctx).Model(&GiftCard{})

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.BatchID != "" {
		query = query.Where("batch_id = ?", filter.BatchID)
	}
	if filter.CreatedBy != nil {
		query = query.Where("created_by = ?", *filter.CreatedBy)
	}
	if filter.MinValue != nil {
		query = query.Where("value >= ?", *filter.MinValue)
	}
	if filter.MaxValue != nil {
		query = query.Where("value <= ?", *filter.MaxValue)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&giftCards).Error
	return giftCards, total, err
}

// SetStatus sets the status of a gift card.
func (r *giftCardRepository) SetStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).
		Model(&GiftCard{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// MarkRedeemed marks a gift card as redeemed.
// Uses a status guard so concurrent redemption attempts cannot both succeed.
// Returns gorm.ErrRecordNotFound when the card no longer satisfies the guard
// (e.g. already redeemed, disabled, expired), allowing callers to treat the
// race loser as "already used".
func (r *giftCardRepository) MarkRedeemed(ctx context.Context, id int64, userID int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&GiftCard{}).
		Where("id = ? AND status = ?", id, GiftCardStatusActive).
		Updates(map[string]interface{}{
			"status":      GiftCardStatusRedeemed,
			"redeemed_by": userID,
			"redeemed_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// MarkPurchased marks a gift card as purchased.
func (r *giftCardRepository) MarkPurchased(ctx context.Context, id int64, userID int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&GiftCard{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"purchased_by": userID,
			"purchased_at": now,
		}).Error
}

// ListByUser lists gift cards redeemed by a user.
func (r *giftCardRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*GiftCard, int64, error) {
	var giftCards []*GiftCard
	var total int64

	query := r.db.WithContext(ctx).Model(&GiftCard{}).Where("redeemed_by = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("redeemed_at DESC").Limit(limit).Offset(offset).Find(&giftCards).Error
	return giftCards, total, err
}

// ListByBatch lists gift cards in a batch.
func (r *giftCardRepository) ListByBatch(ctx context.Context, batchID string, limit, offset int) ([]*GiftCard, int64, error) {
	var giftCards []*GiftCard
	var total int64

	query := r.db.WithContext(ctx).Model(&GiftCard{}).Where("batch_id = ?", batchID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&giftCards).Error
	return giftCards, total, err
}

// GetExpiredActive returns active gift cards that have expired.
func (r *giftCardRepository) GetExpiredActive(ctx context.Context) ([]*GiftCard, error) {
	var giftCards []*GiftCard
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", GiftCardStatusActive, now).
		Find(&giftCards).Error
	return giftCards, err
}

// Count returns the total number of gift cards.
func (r *giftCardRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&GiftCard{}).Count(&count).Error
	return count, err
}

// CountByStatus returns the number of gift cards with a specific status.
func (r *giftCardRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&GiftCard{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// GetTotalValue returns the total value of gift cards with a specific status.
func (r *giftCardRepository) GetTotalValue(ctx context.Context, status string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&GiftCard{}).
		Where("status = ?", status).
		Select("COALESCE(SUM(value), 0)").Scan(&total).Error
	return total, err
}

// GetBatchStats returns statistics for a batch of gift cards.
func (r *giftCardRepository) GetBatchStats(ctx context.Context, batchID string) (total int, redeemed int, totalValue int64, redeemedValue int64, err error) {
	// Get total count and value
	var totalResult struct {
		Count int
		Value int64
	}
	err = r.db.WithContext(ctx).Model(&GiftCard{}).
		Where("batch_id = ?", batchID).
		Select("COUNT(*) as count, COALESCE(SUM(value), 0) as value").
		Scan(&totalResult).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Get redeemed count and value
	var redeemedResult struct {
		Count int
		Value int64
	}
	err = r.db.WithContext(ctx).Model(&GiftCard{}).
		Where("batch_id = ? AND status = ?", batchID, GiftCardStatusRedeemed).
		Select("COUNT(*) as count, COALESCE(SUM(value), 0) as value").
		Scan(&redeemedResult).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return totalResult.Count, redeemedResult.Count, totalResult.Value, redeemedResult.Value, nil
}
