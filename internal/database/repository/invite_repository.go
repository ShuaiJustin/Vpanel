// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// CommercialInviteCode represents an invite code in the database.
type CommercialInviteCode struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	UserID      int64     `gorm:"uniqueIndex;not null"`
	Code        string    `gorm:"uniqueIndex;size:16;not null"`
	InviteCount int       `gorm:"default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`

	User *User `gorm:"foreignKey:UserID"`
}

// TableName returns the table name for CommercialInviteCode.
func (CommercialInviteCode) TableName() string {
	return "commercial_invite_codes"
}

// Referral represents a referral relationship in the database.
type Referral struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	InviterID   int64      `gorm:"index;not null"`
	InviteeID   int64      `gorm:"uniqueIndex;not null"`
	InviteCode  string     `gorm:"size:16;not null"`
	Status      string     `gorm:"size:32;default:registered"`
	ConvertedAt *time.Time
	CreatedAt   time.Time  `gorm:"autoCreateTime"`

	Inviter *User `gorm:"foreignKey:InviterID"`
	Invitee *User `gorm:"foreignKey:InviteeID"`
}

// TableName returns the table name for Referral.
func (Referral) TableName() string {
	return "referrals"
}

// Referral status constants
const (
	ReferralStatusRegistered = "registered"
	ReferralStatusConverted  = "converted"
)

// Commission represents a commission record in the database.
type Commission struct {
	ID         int64      `gorm:"primaryKey;autoIncrement"`
	UserID     int64      `gorm:"index;not null"`
	FromUserID int64      `gorm:"index;not null"`
	OrderID    int64      `gorm:"index;not null"`
	Amount     int64      `gorm:"not null"`
	Rate       float64    `gorm:"not null"`
	Level      int        `gorm:"default:1"`
	Status     string     `gorm:"size:32;default:pending;index"`
	ConfirmAt  *time.Time
	CreatedAt  time.Time  `gorm:"autoCreateTime"`

	User     *User  `gorm:"foreignKey:UserID"`
	FromUser *User  `gorm:"foreignKey:FromUserID"`
	Order    *Order `gorm:"foreignKey:OrderID"`
}

// TableName returns the table name for Commission.
func (Commission) TableName() string {
	return "commissions"
}

// Commission status constants
const (
	CommissionStatusPending   = "pending"
	CommissionStatusConfirmed = "confirmed"
	CommissionStatusCancelled = "cancelled"
)

// InviteStats represents invite statistics.
type InviteStats struct {
	TotalInvites       int   `json:"total_invites"`
	ConvertedInvites   int   `json:"converted_invites"`
	PendingCommission  int64 `json:"pending_commission"`
	ConfirmedCommission int64 `json:"confirmed_commission"`
	TotalCommission    int64 `json:"total_commission"`
}

// InviteRepository defines the interface for invite data access.
type InviteRepository interface {
	// Invite code operations
	CreateInviteCode(ctx context.Context, code *CommercialInviteCode) error
	GetInviteCodeByUserID(ctx context.Context, userID int64) (*CommercialInviteCode, error)
	GetInviteCodeByCode(ctx context.Context, code string) (*CommercialInviteCode, error)
	IncrementInviteCount(ctx context.Context, userID int64) error

	// Referral operations
	CreateReferral(ctx context.Context, referral *Referral) error
	GetReferralByInviteeID(ctx context.Context, inviteeID int64) (*Referral, error)
	ListReferralsByInviter(ctx context.Context, inviterID int64, limit, offset int) ([]*Referral, int64, error)
	MarkReferralConverted(ctx context.Context, inviteeID int64) error

	// Commission operations
	CreateCommission(ctx context.Context, commission *Commission) error
	GetCommissionByID(ctx context.Context, id int64) (*Commission, error)
	GetCommissionByOrderID(ctx context.Context, orderID int64) (*Commission, error)
	ListCommissionsByUser(ctx context.Context, userID int64, status string, limit, offset int) ([]*Commission, int64, error)
	UpdateCommissionStatus(ctx context.Context, id int64, status string) error
	ListDuePendingCommissions(ctx context.Context, beforeDate time.Time, limit int) ([]*Commission, error)
	ClaimPendingCommission(ctx context.Context, id int64) (bool, error)
	RevertClaimedCommission(ctx context.Context, id int64) error
	ConfirmPendingCommissions(ctx context.Context, beforeDate time.Time) (int64, error)
	CancelCommissionsByOrder(ctx context.Context, orderID int64) error

	// Statistics
	GetInviteStats(ctx context.Context, userID int64) (*InviteStats, error)
	GetTotalCommissionByUser(ctx context.Context, userID int64) (int64, error)
	GetPendingCommissionByUser(ctx context.Context, userID int64) (int64, error)
}

// inviteRepository implements InviteRepository.
type inviteRepository struct {
	db *gorm.DB
}

// NewInviteRepository creates a new invite repository.
func NewInviteRepository(db *gorm.DB) InviteRepository {
	return &inviteRepository{db: db}
}

// CreateInviteCode creates a new invite code.
func (r *inviteRepository) CreateInviteCode(ctx context.Context, code *CommercialInviteCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

// GetInviteCodeByUserID retrieves an invite code by user ID.
func (r *inviteRepository) GetInviteCodeByUserID(ctx context.Context, userID int64) (*CommercialInviteCode, error) {
	var code CommercialInviteCode
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&code).Error
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// GetInviteCodeByCode retrieves an invite code by code string.
func (r *inviteRepository) GetInviteCodeByCode(ctx context.Context, code string) (*CommercialInviteCode, error) {
	var inviteCode CommercialInviteCode
	err := r.db.WithContext(ctx).Preload("User").Where("code = ?", code).First(&inviteCode).Error
	if err != nil {
		return nil, err
	}
	return &inviteCode, nil
}

// IncrementInviteCount increments the invite count for a user.
func (r *inviteRepository) IncrementInviteCount(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).
		Model(&CommercialInviteCode{}).
		Where("user_id = ?", userID).
		Update("invite_count", gorm.Expr("invite_count + 1")).Error
}

// CreateReferral creates a new referral.
func (r *inviteRepository) CreateReferral(ctx context.Context, referral *Referral) error {
	return r.db.WithContext(ctx).Create(referral).Error
}

// GetReferralByInviteeID retrieves a referral by invitee ID.
func (r *inviteRepository) GetReferralByInviteeID(ctx context.Context, inviteeID int64) (*Referral, error) {
	var referral Referral
	err := r.db.WithContext(ctx).Preload("Inviter").Where("invitee_id = ?", inviteeID).First(&referral).Error
	if err != nil {
		return nil, err
	}
	return &referral, nil
}

// ListReferralsByInviter lists referrals for an inviter.
func (r *inviteRepository) ListReferralsByInviter(ctx context.Context, inviterID int64, limit, offset int) ([]*Referral, int64, error) {
	var referrals []*Referral
	var total int64

	query := r.db.WithContext(ctx).Model(&Referral{}).Where("inviter_id = ?", inviterID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Invitee").
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&referrals).Error
	return referrals, total, err
}

// MarkReferralConverted marks a referral as converted.
func (r *inviteRepository) MarkReferralConverted(ctx context.Context, inviteeID int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Referral{}).
		Where("invitee_id = ?", inviteeID).
		Updates(map[string]interface{}{
			"status":       ReferralStatusConverted,
			"converted_at": now,
		}).Error
}

// CreateCommission creates a new commission.
func (r *inviteRepository) CreateCommission(ctx context.Context, commission *Commission) error {
	return r.db.WithContext(ctx).Create(commission).Error
}

// GetCommissionByID retrieves a commission by ID.
func (r *inviteRepository) GetCommissionByID(ctx context.Context, id int64) (*Commission, error) {
	var commission Commission
	err := r.db.WithContext(ctx).Preload("User").Preload("FromUser").Preload("Order").First(&commission, id).Error
	if err != nil {
		return nil, err
	}
	return &commission, nil
}

// GetCommissionByOrderID retrieves the commission record for an order.
func (r *inviteRepository) GetCommissionByOrderID(ctx context.Context, orderID int64) (*Commission, error) {
	var commission Commission
	err := r.db.WithContext(ctx).Where("order_id = ? AND status != ?", orderID, CommissionStatusCancelled).First(&commission).Error
	if err != nil {
		return nil, err
	}
	return &commission, nil
}

// ListCommissionsByUser lists commissions for a user.
func (r *inviteRepository) ListCommissionsByUser(ctx context.Context, userID int64, status string, limit, offset int) ([]*Commission, int64, error) {
	var commissions []*Commission
	var total int64

	query := r.db.WithContext(ctx).Model(&Commission{}).Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("FromUser").Preload("Order").
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&commissions).Error
	return commissions, total, err
}

// UpdateCommissionStatus updates the status of a commission.
func (r *inviteRepository) UpdateCommissionStatus(ctx context.Context, id int64, status string) error {
	updates := map[string]interface{}{"status": status}
	if status == CommissionStatusConfirmed {
		now := time.Now()
		updates["confirm_at"] = now
	}
	return r.db.WithContext(ctx).
		Model(&Commission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ListDuePendingCommissions returns pending commissions eligible for settlement.
func (r *inviteRepository) ListDuePendingCommissions(ctx context.Context, beforeDate time.Time, limit int) ([]*Commission, error) {
	if limit <= 0 {
		limit = 100
	}
	var commissions []*Commission
	err := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", CommissionStatusPending, beforeDate).
		Order("created_at ASC").
		Limit(limit).
		Find(&commissions).Error
	return commissions, err
}

// ClaimPendingCommission atomically transitions a pending commission to confirmed.
func (r *inviteRepository) ClaimPendingCommission(ctx context.Context, id int64) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&Commission{}).
		Where("id = ? AND status = ?", id, CommissionStatusPending).
		Updates(map[string]interface{}{
			"status":     CommissionStatusConfirmed,
			"confirm_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

// RevertClaimedCommission returns a confirmed commission back to pending if crediting balance failed.
func (r *inviteRepository) RevertClaimedCommission(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&Commission{}).
		Where("id = ? AND status = ?", id, CommissionStatusConfirmed).
		Updates(map[string]interface{}{
			"status":     CommissionStatusPending,
			"confirm_at": nil,
		}).Error
}

// ConfirmPendingCommissions confirms all pending commissions created before a date.
func (r *inviteRepository) ConfirmPendingCommissions(ctx context.Context, beforeDate time.Time) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&Commission{}).
		Where("status = ? AND created_at < ?", CommissionStatusPending, beforeDate).
		Updates(map[string]interface{}{
			"status":     CommissionStatusConfirmed,
			"confirm_at": now,
		})
	return result.RowsAffected, result.Error
}

// CancelCommissionsByOrder cancels all commissions for an order.
func (r *inviteRepository) CancelCommissionsByOrder(ctx context.Context, orderID int64) error {
	return r.db.WithContext(ctx).
		Model(&Commission{}).
		Where("order_id = ? AND status = ?", orderID, CommissionStatusPending).
		Update("status", CommissionStatusCancelled).Error
}

// GetInviteStats returns invite statistics for a user.
func (r *inviteRepository) GetInviteStats(ctx context.Context, userID int64) (*InviteStats, error) {
	stats := &InviteStats{}

	// Total invites
	var totalInvites int64
	if err := r.db.WithContext(ctx).Model(&Referral{}).Where("inviter_id = ?", userID).Count(&totalInvites).Error; err != nil {
		return nil, err
	}
	stats.TotalInvites = int(totalInvites)

	// Converted invites
	var convertedInvites int64
	if err := r.db.WithContext(ctx).Model(&Referral{}).
		Where("inviter_id = ? AND status = ?", userID, ReferralStatusConverted).
		Count(&convertedInvites).Error; err != nil {
		return nil, err
	}
	stats.ConvertedInvites = int(convertedInvites)

	// Pending commission
	if err := r.db.WithContext(ctx).Model(&Commission{}).
		Where("user_id = ? AND status = ?", userID, CommissionStatusPending).
		Select("COALESCE(SUM(amount), 0)").Scan(&stats.PendingCommission).Error; err != nil {
		return nil, err
	}

	// Confirmed commission
	if err := r.db.WithContext(ctx).Model(&Commission{}).
		Where("user_id = ? AND status = ?", userID, CommissionStatusConfirmed).
		Select("COALESCE(SUM(amount), 0)").Scan(&stats.ConfirmedCommission).Error; err != nil {
		return nil, err
	}

	stats.TotalCommission = stats.PendingCommission + stats.ConfirmedCommission

	return stats, nil
}

// GetTotalCommissionByUser returns total commission for a user.
func (r *inviteRepository) GetTotalCommissionByUser(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&Commission{}).
		Where("user_id = ? AND status IN (?, ?)", userID, CommissionStatusPending, CommissionStatusConfirmed).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	return total, err
}

// GetPendingCommissionByUser returns pending commission for a user.
func (r *inviteRepository) GetPendingCommissionByUser(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&Commission{}).
		Where("user_id = ? AND status = ?", userID, CommissionStatusPending).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	return total, err
}
