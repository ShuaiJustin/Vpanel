// Package repository provides data access implementations.
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// PasswordResetToken represents a password reset token in the database.
type PasswordResetToken struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	UserID    int64      `gorm:"index;not null"`
	Token     string     `gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	UsedAt    *time.Time `gorm:""`
	CreatedAt time.Time  `gorm:"autoCreateTime"`

	// Relations
	User *User `gorm:"foreignKey:UserID"`
}

// TableName returns the table name for PasswordResetToken.
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// IsExpired checks if the token has expired.
func (t *PasswordResetToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsUsed checks if the token has been used.
func (t *PasswordResetToken) IsUsed() bool {
	return t.UsedAt != nil
}

// EmailVerificationToken represents an email verification token in the database.
type EmailVerificationToken struct {
	ID         int64      `gorm:"primaryKey;autoIncrement"`
	UserID     int64      `gorm:"index;not null"`
	Email      string     `gorm:"size:256;not null"`
	Token      string     `gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt  time.Time  `gorm:"not null"`
	VerifiedAt *time.Time `gorm:""`
	CreatedAt  time.Time  `gorm:"autoCreateTime"`

	// Relations
	User *User `gorm:"foreignKey:UserID"`
}

// TableName returns the table name for EmailVerificationToken.
func (EmailVerificationToken) TableName() string {
	return "email_verification_tokens"
}

// IsExpired checks if the token has expired.
func (t *EmailVerificationToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsVerified checks if the email has been verified.
func (t *EmailVerificationToken) IsVerified() bool {
	return t.VerifiedAt != nil
}

// InviteCode represents an invite code in the database.
type InviteCode struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	Code      string     `gorm:"uniqueIndex;size:32;not null"`
	CreatedBy *int64     `gorm:"index"`
	UsedBy    *int64     `gorm:"index"`
	MaxUses   int        `gorm:"default:1"`
	UsedCount int        `gorm:"default:0"`
	ExpiresAt *time.Time `gorm:""`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	UsedAt    *time.Time `gorm:""`

	// Relations
	Creator    *User `gorm:"foreignKey:CreatedBy"`
	UsedByUser *User `gorm:"foreignKey:UsedBy"`
}

// TableName returns the table name for InviteCode.
func (InviteCode) TableName() string {
	return "invite_codes"
}

// IsExpired checks if the invite code has expired.
func (c *InviteCode) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
}

// IsExhausted checks if the invite code has reached its usage limit.
func (c *InviteCode) IsExhausted() bool {
	return c.UsedCount >= c.MaxUses
}

// IsValid checks if the invite code is valid for use.
func (c *InviteCode) IsValid() bool {
	return !c.IsExpired() && !c.IsExhausted()
}

// TwoFactorSecret represents a 2FA secret in the database.
type TwoFactorSecret struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	UserID      int64  `gorm:"uniqueIndex;not null"`
	Secret      string `gorm:"size:64;not null"`
	BackupCodes string `gorm:"type:text"`
	Enabled     bool   `gorm:"default:false"`
	EnabledAt   *time.Time
	CreatedAt   time.Time `gorm:"autoCreateTime"`

	// Relations
	User *User `gorm:"foreignKey:UserID"`
}

// TableName returns the table name for TwoFactorSecret.
func (TwoFactorSecret) TableName() string {
	return "two_factor_secrets"
}

// AuthTokenRepository defines the interface for authentication token data access.
type AuthTokenRepository interface {
	// Password Reset Tokens
	CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error
	GetPasswordResetTokenByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, id int64) error
	DeleteExpiredPasswordResetTokens(ctx context.Context) (int64, error)
	CountPasswordResetTokensByUser(ctx context.Context, userID int64, since time.Time) (int64, error)

	// Email Verification Tokens
	CreateEmailVerificationToken(ctx context.Context, token *EmailVerificationToken) error
	GetEmailVerificationTokenByToken(ctx context.Context, token string) (*EmailVerificationToken, error)
	MarkEmailVerified(ctx context.Context, id int64) error
	CountPendingEmailVerificationTokensByUser(ctx context.Context, userID int64) (int64, error)
	DeleteExpiredEmailVerificationTokens(ctx context.Context) (int64, error)

	// Invite Codes
	CreateInviteCode(ctx context.Context, code *InviteCode) error
	GetInviteCodeByCode(ctx context.Context, code string) (*InviteCode, error)
	UseInviteCode(ctx context.Context, code string, userID int64) error
	ListInviteCodes(ctx context.Context, limit, offset int) ([]*InviteCode, int64, error)
	DeleteInviteCode(ctx context.Context, id int64) error

	// Two-Factor Secrets
	CreateTwoFactorSecret(ctx context.Context, secret *TwoFactorSecret) error
	GetTwoFactorSecretByUserID(ctx context.Context, userID int64) (*TwoFactorSecret, error)
	UpdateTwoFactorSecret(ctx context.Context, secret *TwoFactorSecret) error
	DeleteTwoFactorSecret(ctx context.Context, userID int64) error
	EnableTwoFactor(ctx context.Context, userID int64) error
	VerifyBackupCode(ctx context.Context, userID int64, code string) (bool, error)
}

// authTokenRepository implements AuthTokenRepository.
type authTokenRepository struct {
	db *gorm.DB
}

// NewAuthTokenRepository creates a new auth token repository.
func NewAuthTokenRepository(db *gorm.DB) AuthTokenRepository {
	return &authTokenRepository{db: db}
}

// CreatePasswordResetToken creates a new password reset token.
func (r *authTokenRepository) CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error {
	result := r.db.WithContext(ctx).Create(token)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create password reset token", result.Error)
	}
	return nil
}

// GetPasswordResetTokenByToken retrieves a password reset token by its token string.
func (r *authTokenRepository) GetPasswordResetTokenByToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	var resetToken PasswordResetToken
	result := r.db.WithContext(ctx).Where("token = ?", token).First(&resetToken)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("password_reset_token", token)
		}
		return nil, errors.NewDatabaseError("failed to get password reset token", result.Error)
	}
	return &resetToken, nil
}

// MarkPasswordResetTokenUsed marks a password reset token as used.
func (r *authTokenRepository) MarkPasswordResetTokenUsed(ctx context.Context, id int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&PasswordResetToken{}).
		Where("id = ?", id).
		Update("used_at", now)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to mark password reset token as used", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("password_reset_token", id)
	}
	return nil
}

// DeleteExpiredPasswordResetTokens deletes expired password reset tokens.
func (r *authTokenRepository) DeleteExpiredPasswordResetTokens(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&PasswordResetToken{})
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to delete expired password reset tokens", result.Error)
	}
	return result.RowsAffected, nil
}

// CountPasswordResetTokensByUser counts password reset tokens for a user since a given time.
func (r *authTokenRepository) CountPasswordResetTokensByUser(ctx context.Context, userID int64, since time.Time) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&PasswordResetToken{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count password reset tokens", result.Error)
	}
	return count, nil
}

// CreateEmailVerificationToken creates a new email verification token.
func (r *authTokenRepository) CreateEmailVerificationToken(ctx context.Context, token *EmailVerificationToken) error {
	result := r.db.WithContext(ctx).Create(token)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create email verification token", result.Error)
	}
	return nil
}

// GetEmailVerificationTokenByToken retrieves an email verification token by its token string.
func (r *authTokenRepository) GetEmailVerificationTokenByToken(ctx context.Context, token string) (*EmailVerificationToken, error) {
	var verificationToken EmailVerificationToken
	result := r.db.WithContext(ctx).Where("token = ?", token).First(&verificationToken)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("email_verification_token", token)
		}
		return nil, errors.NewDatabaseError("failed to get email verification token", result.Error)
	}
	return &verificationToken, nil
}

// MarkEmailVerified marks an email verification token as verified.
func (r *authTokenRepository) MarkEmailVerified(ctx context.Context, id int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&EmailVerificationToken{}).
		Where("id = ?", id).
		Update("verified_at", now)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to mark email as verified", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("email_verification_token", id)
	}
	return nil
}

// CountPendingEmailVerificationTokensByUser counts unverified and unexpired verification tokens for a user.
func (r *authTokenRepository) CountPendingEmailVerificationTokensByUser(ctx context.Context, userID int64) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&EmailVerificationToken{}).
		Where("user_id = ? AND verified_at IS NULL AND expires_at > ?", userID, time.Now()).
		Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count pending email verification tokens", result.Error)
	}
	return count, nil
}

// DeleteExpiredEmailVerificationTokens deletes expired email verification tokens.
func (r *authTokenRepository) DeleteExpiredEmailVerificationTokens(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&EmailVerificationToken{})
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to delete expired email verification tokens", result.Error)
	}
	return result.RowsAffected, nil
}

// CreateInviteCode creates a new invite code.
func (r *authTokenRepository) CreateInviteCode(ctx context.Context, code *InviteCode) error {
	result := r.db.WithContext(ctx).Create(code)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create invite code", result.Error)
	}
	return nil
}

// GetInviteCodeByCode retrieves an invite code by its code string.
func (r *authTokenRepository) GetInviteCodeByCode(ctx context.Context, code string) (*InviteCode, error) {
	var inviteCode InviteCode
	result := r.db.WithContext(ctx).Where("code = ?", code).First(&inviteCode)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("invite_code", code)
		}
		return nil, errors.NewDatabaseError("failed to get invite code", result.Error)
	}
	return &inviteCode, nil
}

// UseInviteCode marks an invite code as used by a user.
func (r *authTokenRepository) UseInviteCode(ctx context.Context, code string, userID int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&InviteCode{}).
		Where("code = ?", code).
		Updates(map[string]interface{}{
			"used_by":    userID,
			"used_count": gorm.Expr("used_count + 1"),
			"used_at":    now,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to use invite code", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("invite_code", code)
	}
	return nil
}

// ListInviteCodes retrieves all invite codes with pagination.
func (r *authTokenRepository) ListInviteCodes(ctx context.Context, limit, offset int) ([]*InviteCode, int64, error) {
	var codes []*InviteCode
	var total int64

	query := r.db.WithContext(ctx).Model(&InviteCode{})

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count invite codes", err)
	}

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	// Fetch results
	if err := query.Order("created_at DESC").Find(&codes).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to list invite codes", err)
	}

	return codes, total, nil
}

// DeleteInviteCode deletes an invite code by ID.
func (r *authTokenRepository) DeleteInviteCode(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&InviteCode{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete invite code", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("invite_code", id)
	}
	return nil
}

// CreateTwoFactorSecret creates a new 2FA secret.
func (r *authTokenRepository) CreateTwoFactorSecret(ctx context.Context, secret *TwoFactorSecret) error {
	result := r.db.WithContext(ctx).Create(secret)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create 2FA secret", result.Error)
	}
	return nil
}

// GetTwoFactorSecretByUserID retrieves a 2FA secret by user ID.
func (r *authTokenRepository) GetTwoFactorSecretByUserID(ctx context.Context, userID int64) (*TwoFactorSecret, error) {
	var secret TwoFactorSecret
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&secret)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("two_factor_secret", userID)
		}
		return nil, errors.NewDatabaseError("failed to get 2FA secret", result.Error)
	}
	return &secret, nil
}

// UpdateTwoFactorSecret updates a 2FA secret.
func (r *authTokenRepository) UpdateTwoFactorSecret(ctx context.Context, secret *TwoFactorSecret) error {
	result := r.db.WithContext(ctx).Save(secret)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update 2FA secret", result.Error)
	}
	return nil
}

// DeleteTwoFactorSecret deletes a 2FA secret by user ID.
func (r *authTokenRepository) DeleteTwoFactorSecret(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&TwoFactorSecret{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete 2FA secret", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("two_factor_secret", userID)
	}
	return nil
}

// EnableTwoFactor enables 2FA for a user.
func (r *authTokenRepository) EnableTwoFactor(ctx context.Context, userID int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&TwoFactorSecret{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"enabled":    true,
			"enabled_at": now,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to enable 2FA", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("two_factor_secret", userID)
	}

	// Also update user's two_factor_enabled flag
	r.db.WithContext(ctx).Model(&User{}).
		Where("id = ?", userID).
		Update("two_factor_enabled", true)

	return nil
}

// VerifyBackupCode verifies and consumes a backup code.
func (r *authTokenRepository) VerifyBackupCode(ctx context.Context, userID int64, code string) (bool, error) {
	secret, err := r.GetTwoFactorSecretByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	// Parse backup codes
	codes := strings.Split(secret.BackupCodes, ",")
	found := false
	newCodes := make([]string, 0, len(codes))

	for _, c := range codes {
		if c == code && !found {
			found = true
			// Don't add this code to newCodes (consume it)
		} else if c != "" {
			newCodes = append(newCodes, c)
		}
	}

	if !found {
		return false, nil
	}

	// Update backup codes
	secret.BackupCodes = strings.Join(newCodes, ",")
	if err := r.UpdateTwoFactorSecret(ctx, secret); err != nil {
		return false, err
	}

	return true, nil
}
