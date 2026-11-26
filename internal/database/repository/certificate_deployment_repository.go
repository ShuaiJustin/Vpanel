// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// CertificateDeployment 证书部署记录
type CertificateDeployment struct {
	ID            int64      `gorm:"primaryKey;autoIncrement"`
	CertificateID int64      `gorm:"not null;index"`
	NodeID        int64      `gorm:"not null;index"`
	Status        string     `gorm:"size:32;default:pending"` // pending, success, failed
	Message       string     `gorm:"type:text"`
	DeployedAt    *time.Time `gorm:""`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
}

// TableName returns the table name.
func (CertificateDeployment) TableName() string {
	return "certificate_deployments"
}

// CertificateDeploymentRepository defines the interface for certificate deployment data access.
type CertificateDeploymentRepository interface {
	Create(ctx context.Context, deployment *CertificateDeployment) error
	GetByID(ctx context.Context, id int64) (*CertificateDeployment, error)
	Update(ctx context.Context, deployment *CertificateDeployment) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*CertificateDeployment, error)
	GetByCertificate(ctx context.Context, certID int64) ([]*CertificateDeployment, error)
	GetByNode(ctx context.Context, nodeID int64) ([]*CertificateDeployment, error)
	GetLatestByNodeAndCert(ctx context.Context, nodeID, certID int64) (*CertificateDeployment, error)
}

// certificateDeploymentRepository implements CertificateDeploymentRepository.
type certificateDeploymentRepository struct {
	db *gorm.DB
}

// NewCertificateDeploymentRepository creates a new certificate deployment repository.
func NewCertificateDeploymentRepository(db *gorm.DB) CertificateDeploymentRepository {
	return &certificateDeploymentRepository{db: db}
}

// Create creates a new certificate deployment record.
func (r *certificateDeploymentRepository) Create(ctx context.Context, deployment *CertificateDeployment) error {
	return r.db.WithContext(ctx).Create(deployment).Error
}

// GetByID retrieves a certificate deployment by ID.
func (r *certificateDeploymentRepository) GetByID(ctx context.Context, id int64) (*CertificateDeployment, error) {
	var deployment CertificateDeployment
	err := r.db.WithContext(ctx).First(&deployment, id).Error
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

// Update updates a certificate deployment.
func (r *certificateDeploymentRepository) Update(ctx context.Context, deployment *CertificateDeployment) error {
	return r.db.WithContext(ctx).Save(deployment).Error
}

// Delete deletes a certificate deployment.
func (r *certificateDeploymentRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&CertificateDeployment{}, id).Error
}

// List lists certificate deployments with pagination.
func (r *certificateDeploymentRepository) List(ctx context.Context, limit, offset int) ([]*CertificateDeployment, error) {
	var deployments []*CertificateDeployment
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&deployments).Error
	return deployments, err
}

// GetByCertificate retrieves all deployments for a certificate.
func (r *certificateDeploymentRepository) GetByCertificate(ctx context.Context, certID int64) ([]*CertificateDeployment, error) {
	var deployments []*CertificateDeployment
	err := r.db.WithContext(ctx).
		Where("certificate_id = ?", certID).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}

// GetByNode retrieves all deployments for a node.
func (r *certificateDeploymentRepository) GetByNode(ctx context.Context, nodeID int64) ([]*CertificateDeployment, error) {
	var deployments []*CertificateDeployment
	err := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}

// GetLatestByNodeAndCert retrieves the latest deployment for a node and certificate.
func (r *certificateDeploymentRepository) GetLatestByNodeAndCert(ctx context.Context, nodeID, certID int64) (*CertificateDeployment, error) {
	var deployment CertificateDeployment
	err := r.db.WithContext(ctx).
		Where("node_id = ? AND certificate_id = ?", nodeID, certID).
		Order("created_at DESC").
		First(&deployment).Error
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}
