// Package repository provides data access implementations.
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// Certificate represents an SSL/TLS certificate in the database.
type Certificate struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	Domain       string     `gorm:"uniqueIndex;size:255;not null"`
	Provider     string     `gorm:"size:50;default:manual"` // letsencrypt, zerossl, manual, self-signed
	CertPath     string     `gorm:"size:500"`               // 证书文件路径
	KeyPath      string     `gorm:"size:500"`               // 私钥文件路径
	Certificate  string     `gorm:"type:text"`              // 证书内容（可选，用于备份）
	PrivateKey   string     `gorm:"type:text"`              // 私钥内容（可选，用于备份）
	AutoRenew    bool       `gorm:"default:true"`
	Status       string     `gorm:"size:32;default:active"` // pending, active, failed, expired
	ErrorMessage string     `gorm:"type:text"`              // 错误信息
	IssueDate    *time.Time `gorm:""`                       // 签发日期
	ExpireDate   *time.Time `gorm:"index"`                  // 过期日期
	ExpiresAt    time.Time  `gorm:"not null;index"`         // 兼容旧字段
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

// TableName returns the table name for Certificate.
func (Certificate) TableName() string {
	return "certificates"
}

// CertificateRepository defines the interface for certificate data access.
type CertificateRepository interface {
	// CRUD operations
	Create(ctx context.Context, cert *Certificate) error
	GetByID(ctx context.Context, id int64) (*Certificate, error)
	GetByDomain(ctx context.Context, domain string) (*Certificate, error)
	Update(ctx context.Context, cert *Certificate) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*Certificate, error)
	Count(ctx context.Context) (int64, error)

	// Query operations
	GetExpiring(ctx context.Context, days int) ([]*Certificate, error)
	GetAutoRenew(ctx context.Context) ([]*Certificate, error)
}

// certificateRepository implements CertificateRepository.
type certificateRepository struct {
	db *gorm.DB
}

// NewCertificateRepository creates a new certificate repository.
func NewCertificateRepository(db *gorm.DB) CertificateRepository {
	return &certificateRepository{db: db}
}

// Create creates a new certificate.
func (r *certificateRepository) Create(ctx context.Context, cert *Certificate) error {
	result := r.db.WithContext(ctx).Create(cert)
	if result.Error != nil {
		// Check for duplicate key error (SQLite: UNIQUE constraint failed)
		if strings.Contains(result.Error.Error(), "UNIQUE constraint failed") ||
			strings.Contains(result.Error.Error(), "duplicate key") {
			return errors.NewConflictError("certificate", "domain", cert.Domain)
		}
		return errors.NewDatabaseError("failed to create certificate", result.Error)
	}
	return nil
}

// GetByID retrieves a certificate by ID.
func (r *certificateRepository) GetByID(ctx context.Context, id int64) (*Certificate, error) {
	var cert Certificate
	result := r.db.WithContext(ctx).First(&cert, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("certificate", id)
		}
		return nil, errors.NewDatabaseError("failed to get certificate", result.Error)
	}
	return &cert, nil
}

// GetByDomain retrieves a certificate by domain.
func (r *certificateRepository) GetByDomain(ctx context.Context, domain string) (*Certificate, error) {
	var cert Certificate
	result := r.db.WithContext(ctx).Where("domain = ?", domain).First(&cert)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("certificate", domain)
		}
		return nil, errors.NewDatabaseError("failed to get certificate by domain", result.Error)
	}
	return &cert, nil
}

// Update updates a certificate.
func (r *certificateRepository) Update(ctx context.Context, cert *Certificate) error {
	result := r.db.WithContext(ctx).Save(cert)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update certificate", result.Error)
	}
	return nil
}

// Delete deletes a certificate by ID.
func (r *certificateRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&Certificate{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete certificate", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("certificate", id)
	}
	return nil
}

// List retrieves certificates with pagination.
func (r *certificateRepository) List(ctx context.Context, limit, offset int) ([]*Certificate, error) {
	var certs []*Certificate
	query := r.db.WithContext(ctx).Model(&Certificate{})

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	result := query.Order("expires_at ASC").Find(&certs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to list certificates", result.Error)
	}
	return certs, nil
}

// Count counts all certificates.
func (r *certificateRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&Certificate{}).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count certificates", result.Error)
	}
	return count, nil
}

// GetExpiring retrieves certificates expiring within the specified number of days.
func (r *certificateRepository) GetExpiring(ctx context.Context, days int) ([]*Certificate, error) {
	var certs []*Certificate
	expiryDate := time.Now().AddDate(0, 0, days)
	
	result := r.db.WithContext(ctx).
		Where("expires_at <= ? AND expires_at > ?", expiryDate, time.Now()).
		Order("expires_at ASC").
		Find(&certs)
	
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get expiring certificates", result.Error)
	}
	return certs, nil
}

// GetAutoRenew retrieves all certificates with auto-renew enabled.
func (r *certificateRepository) GetAutoRenew(ctx context.Context) ([]*Certificate, error) {
	var certs []*Certificate
	result := r.db.WithContext(ctx).
		Where("auto_renew = ?", true).
		Find(&certs)
	
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get auto-renew certificates", result.Error)
	}
	return certs, nil
}
