// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// Announcement represents a system announcement in the database.
type Announcement struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	Title       string     `gorm:"size:256;not null"`
	Content     string     `gorm:"type:text;not null"`
	Category    string     `gorm:"size:64;default:general;index"`
	IsPinned    bool       `gorm:"default:false"`
	IsPublished bool       `gorm:"default:false;index"`
	PublishedAt *time.Time `gorm:""`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName returns the table name for Announcement.
func (Announcement) TableName() string {
	return "announcements"
}

// Announcement category constants
const (
	AnnouncementCategoryGeneral     = "general"
	AnnouncementCategoryMaintenance = "maintenance"
	AnnouncementCategoryUpdate      = "update"
	AnnouncementCategoryPromotion   = "promotion"
)

// AnnouncementRead tracks read status per user.
type AnnouncementRead struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	UserID         int64     `gorm:"index;not null"`
	AnnouncementID int64     `gorm:"index;not null"`
	ReadAt         time.Time `gorm:"autoCreateTime"`

	// Relations
	User         *User         `gorm:"foreignKey:UserID"`
	Announcement *Announcement `gorm:"foreignKey:AnnouncementID"`
}

// TableName returns the table name for AnnouncementRead.
func (AnnouncementRead) TableName() string {
	return "announcement_reads"
}

// AnnouncementFilter represents filter options for listing announcements.
type AnnouncementFilter struct {
	Category    *string
	IsPublished *bool
	IsPinned    *bool
	Limit       int
	Offset      int
}

// AnnouncementWithReadStatus represents an announcement with user read status.
type AnnouncementWithReadStatus struct {
	Announcement
	IsRead bool `gorm:"-"`
}

// AnnouncementRepository defines the interface for announcement data access.
type AnnouncementRepository interface {
	// Create creates a new announcement.
	Create(ctx context.Context, announcement *Announcement) error

	// GetByID retrieves an announcement by its ID.
	GetByID(ctx context.Context, id int64) (*Announcement, error)

	// Update updates an existing announcement.
	Update(ctx context.Context, announcement *Announcement) error

	// Delete deletes an announcement by ID.
	Delete(ctx context.Context, id int64) error

	// List retrieves announcements with optional filtering.
	List(ctx context.Context, filter *AnnouncementFilter) ([]*Announcement, int64, error)

	// ListPublished retrieves published announcements.
	ListPublished(ctx context.Context, limit, offset int) ([]*Announcement, int64, error)

	// ListWithReadStatus retrieves announcements with read status for a user.
	ListWithReadStatus(ctx context.Context, userID int64, limit, offset int) ([]*AnnouncementWithReadStatus, int64, error)

	// Publish publishes an announcement.
	Publish(ctx context.Context, id int64) error

	// Unpublish unpublishes an announcement.
	Unpublish(ctx context.Context, id int64) error

	// MarkAsRead marks an announcement as read for a user.
	MarkAsRead(ctx context.Context, userID, announcementID int64) error

	// IsRead checks if an announcement is read by a user.
	IsRead(ctx context.Context, userID, announcementID int64) (bool, error)
	GetReadIDs(ctx context.Context, userID int64, announcementIDs []int64) ([]int64, error)

	// GetUnreadCount gets the count of unread announcements for a user.
	GetUnreadCount(ctx context.Context, userID int64) (int64, error)

	// GetRecent retrieves the most recent published announcements.
	GetRecent(ctx context.Context, limit int) ([]*Announcement, error)
}

// announcementRepository implements AnnouncementRepository.
type announcementRepository struct {
	db *gorm.DB
}

// NewAnnouncementRepository creates a new announcement repository.
func NewAnnouncementRepository(db *gorm.DB) AnnouncementRepository {
	return &announcementRepository{db: db}
}

// Create creates a new announcement.
func (r *announcementRepository) Create(ctx context.Context, announcement *Announcement) error {
	result := r.db.WithContext(ctx).Create(announcement)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create announcement", result.Error)
	}
	return nil
}

// GetByID retrieves an announcement by its ID.
func (r *announcementRepository) GetByID(ctx context.Context, id int64) (*Announcement, error) {
	var announcement Announcement
	result := r.db.WithContext(ctx).First(&announcement, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("announcement", id)
		}
		return nil, errors.NewDatabaseError("failed to get announcement", result.Error)
	}
	return &announcement, nil
}

// Update updates an existing announcement.
func (r *announcementRepository) Update(ctx context.Context, announcement *Announcement) error {
	result := r.db.WithContext(ctx).Save(announcement)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update announcement", result.Error)
	}
	return nil
}

// Delete deletes an announcement by ID.
func (r *announcementRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&Announcement{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete announcement", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("announcement", id)
	}
	return nil
}

// List retrieves announcements with optional filtering.
func (r *announcementRepository) List(ctx context.Context, filter *AnnouncementFilter) ([]*Announcement, int64, error) {
	var announcements []*Announcement
	var total int64

	query := r.db.WithContext(ctx).Model(&Announcement{})

	// Apply filters
	if filter != nil {
		if filter.Category != nil {
			query = query.Where("category = ?", *filter.Category)
		}
		if filter.IsPublished != nil {
			query = query.Where("is_published = ?", *filter.IsPublished)
		}
		if filter.IsPinned != nil {
			query = query.Where("is_pinned = ?", *filter.IsPinned)
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count announcements", err)
	}

	// Apply pagination
	if filter != nil {
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	// Fetch results (pinned first, then by published_at desc)
	if err := query.Order("is_pinned DESC, published_at DESC").Find(&announcements).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to list announcements", err)
	}

	return announcements, total, nil
}

// ListPublished retrieves published announcements.
func (r *announcementRepository) ListPublished(ctx context.Context, limit, offset int) ([]*Announcement, int64, error) {
	isPublished := true
	return r.List(ctx, &AnnouncementFilter{
		IsPublished: &isPublished,
		Limit:       limit,
		Offset:      offset,
	})
}

// ListWithReadStatus retrieves announcements with read status for a user.
func (r *announcementRepository) ListWithReadStatus(ctx context.Context, userID int64, limit, offset int) ([]*AnnouncementWithReadStatus, int64, error) {
	var total int64

	// Count total published announcements
	if err := r.db.WithContext(ctx).Model(&Announcement{}).Where("is_published = ?", true).Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count announcements", err)
	}

	// Get announcements with read status
	var results []*AnnouncementWithReadStatus
	query := r.db.WithContext(ctx).
		Table("announcements").
		Select("announcements.*, CASE WHEN announcement_reads.id IS NOT NULL THEN 1 ELSE 0 END as is_read").
		Joins("LEFT JOIN announcement_reads ON announcements.id = announcement_reads.announcement_id AND announcement_reads.user_id = ?", userID).
		Where("announcements.is_published = ?", true).
		Order("announcements.is_pinned DESC, announcements.published_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Scan(&results).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to list announcements with read status", err)
	}

	return results, total, nil
}

// Publish publishes an announcement.
func (r *announcementRepository) Publish(ctx context.Context, id int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&Announcement{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_published": true,
			"published_at": now,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to publish announcement", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("announcement", id)
	}
	return nil
}

// Unpublish unpublishes an announcement.
func (r *announcementRepository) Unpublish(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Model(&Announcement{}).
		Where("id = ?", id).
		Update("is_published", false)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to unpublish announcement", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("announcement", id)
	}
	return nil
}

// MarkAsRead marks an announcement as read for a user.
func (r *announcementRepository) MarkAsRead(ctx context.Context, userID, announcementID int64) error {
	read := AnnouncementRead{
		UserID:         userID,
		AnnouncementID: announcementID,
	}
	// Use FirstOrCreate to handle duplicate entries
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND announcement_id = ?", userID, announcementID).
		FirstOrCreate(&read)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to mark announcement as read", result.Error)
	}
	return nil
}

// IsRead checks if an announcement is read by a user.
func (r *announcementRepository) IsRead(ctx context.Context, userID, announcementID int64) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&AnnouncementRead{}).
		Where("user_id = ? AND announcement_id = ?", userID, announcementID).
		Count(&count)
	if result.Error != nil {
		return false, errors.NewDatabaseError("failed to check announcement read status", result.Error)
	}
	return count > 0, nil
}

// GetUnreadCount gets the count of unread announcements for a user.
func (r *announcementRepository) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Table("announcements").
		Where("is_published = ?", true).
		Where("id NOT IN (SELECT announcement_id FROM announcement_reads WHERE user_id = ?)", userID).
		Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to get unread announcement count", result.Error)
	}
	return count, nil
}

// GetRecent retrieves the most recent published announcements.
func (r *announcementRepository) GetRecent(ctx context.Context, limit int) ([]*Announcement, error) {
	var announcements []*Announcement
	result := r.db.WithContext(ctx).
		Where("is_published = ?", true).
		Order("is_pinned DESC, published_at DESC").
		Limit(limit).
		Find(&announcements)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get recent announcements", result.Error)
	}
	return announcements, nil
}

// GetReadIDs loads this user's read markers for one page in a single indexed query.
func (r *announcementRepository) GetReadIDs(ctx context.Context, userID int64, announcementIDs []int64) ([]int64, error) {
	if len(announcementIDs) == 0 {
		return []int64{}, nil
	}
	var ids []int64
	err := r.db.WithContext(ctx).Model(&AnnouncementRead{}).
		Where("user_id = ? AND announcement_id IN ?", userID, announcementIDs).
		Pluck("announcement_id", &ids).Error
	return ids, err
}
