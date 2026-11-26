// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// HelpArticle represents a knowledge base article in the database.
type HelpArticle struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	Slug         string    `gorm:"uniqueIndex;size:128;not null"`
	Title        string    `gorm:"size:256;not null"`
	Content      string    `gorm:"type:text;not null"`
	Category     string    `gorm:"size:64;index"`
	Tags         string    `gorm:"size:512"`
	ViewCount    int64     `gorm:"default:0"`
	HelpfulCount int64     `gorm:"default:0"`
	IsPublished  bool      `gorm:"default:false;index"`
	IsFeatured   bool      `gorm:"default:false"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName returns the table name for HelpArticle.
func (HelpArticle) TableName() string {
	return "help_articles"
}

// Help article category constants
const (
	HelpCategoryGettingStarted  = "getting-started"
	HelpCategoryClientSetup     = "client-setup"
	HelpCategoryTroubleshooting = "troubleshooting"
	HelpCategoryFAQ             = "faq"
)

// HelpArticleFilter represents filter options for listing help articles.
type HelpArticleFilter struct {
	Category    *string
	IsPublished *bool
	IsFeatured  *bool
	Limit       int
	Offset      int
}

// HelpArticleRepository defines the interface for help article data access.
type HelpArticleRepository interface {
	// Create creates a new help article.
	Create(ctx context.Context, article *HelpArticle) error

	// GetByID retrieves a help article by its ID.
	GetByID(ctx context.Context, id int64) (*HelpArticle, error)

	// GetBySlug retrieves a help article by its slug.
	GetBySlug(ctx context.Context, slug string) (*HelpArticle, error)

	// Update updates an existing help article.
	Update(ctx context.Context, article *HelpArticle) error

	// Delete deletes a help article by ID.
	Delete(ctx context.Context, id int64) error

	// List retrieves help articles with optional filtering.
	List(ctx context.Context, filter *HelpArticleFilter) ([]*HelpArticle, int64, error)

	// ListPublished retrieves published help articles.
	ListPublished(ctx context.Context, limit, offset int) ([]*HelpArticle, int64, error)

	// ListByCategory retrieves help articles by category.
	ListByCategory(ctx context.Context, category string, limit, offset int) ([]*HelpArticle, int64, error)

	// Search searches help articles by title, content, or tags.
	Search(ctx context.Context, query string, limit, offset int) ([]*HelpArticle, int64, error)

	// GetFeatured retrieves featured help articles.
	GetFeatured(ctx context.Context, limit int) ([]*HelpArticle, error)

	// IncrementViewCount increments the view count for an article.
	IncrementViewCount(ctx context.Context, id int64) error

	// IncrementHelpfulCount increments the helpful count for an article.
	IncrementHelpfulCount(ctx context.Context, id int64) error

	// GetCategories retrieves all unique categories.
	GetCategories(ctx context.Context) ([]string, error)
}

// helpArticleRepository implements HelpArticleRepository.
type helpArticleRepository struct {
	db *gorm.DB
}

// NewHelpArticleRepository creates a new help article repository.
func NewHelpArticleRepository(db *gorm.DB) HelpArticleRepository {
	return &helpArticleRepository{db: db}
}

// Create creates a new help article.
func (r *helpArticleRepository) Create(ctx context.Context, article *HelpArticle) error {
	result := r.db.WithContext(ctx).Create(article)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create help article", result.Error)
	}
	return nil
}

// GetByID retrieves a help article by its ID.
func (r *helpArticleRepository) GetByID(ctx context.Context, id int64) (*HelpArticle, error) {
	var article HelpArticle
	result := r.db.WithContext(ctx).First(&article, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("help_article", id)
		}
		return nil, errors.NewDatabaseError("failed to get help article", result.Error)
	}
	return &article, nil
}

// GetBySlug retrieves a help article by its slug.
func (r *helpArticleRepository) GetBySlug(ctx context.Context, slug string) (*HelpArticle, error) {
	var article HelpArticle
	result := r.db.WithContext(ctx).Where("slug = ?", slug).First(&article)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("help_article", slug)
		}
		return nil, errors.NewDatabaseError("failed to get help article by slug", result.Error)
	}
	return &article, nil
}

// Update updates an existing help article.
func (r *helpArticleRepository) Update(ctx context.Context, article *HelpArticle) error {
	result := r.db.WithContext(ctx).Save(article)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update help article", result.Error)
	}
	return nil
}

// Delete deletes a help article by ID.
func (r *helpArticleRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&HelpArticle{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete help article", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("help_article", id)
	}
	return nil
}

// List retrieves help articles with optional filtering.
func (r *helpArticleRepository) List(ctx context.Context, filter *HelpArticleFilter) ([]*HelpArticle, int64, error) {
	var articles []*HelpArticle
	var total int64

	query := r.db.WithContext(ctx).Model(&HelpArticle{})

	// Apply filters
	if filter != nil {
		if filter.Category != nil {
			query = query.Where("category = ?", *filter.Category)
		}
		if filter.IsPublished != nil {
			query = query.Where("is_published = ?", *filter.IsPublished)
		}
		if filter.IsFeatured != nil {
			query = query.Where("is_featured = ?", *filter.IsFeatured)
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count help articles", err)
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

	// Fetch results
	if err := query.Order("is_featured DESC, updated_at DESC").Find(&articles).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to list help articles", err)
	}

	return articles, total, nil
}

// ListPublished retrieves published help articles.
func (r *helpArticleRepository) ListPublished(ctx context.Context, limit, offset int) ([]*HelpArticle, int64, error) {
	isPublished := true
	return r.List(ctx, &HelpArticleFilter{
		IsPublished: &isPublished,
		Limit:       limit,
		Offset:      offset,
	})
}

// ListByCategory retrieves help articles by category.
func (r *helpArticleRepository) ListByCategory(ctx context.Context, category string, limit, offset int) ([]*HelpArticle, int64, error) {
	isPublished := true
	return r.List(ctx, &HelpArticleFilter{
		Category:    &category,
		IsPublished: &isPublished,
		Limit:       limit,
		Offset:      offset,
	})
}

// Search searches help articles by title, content, or tags.
func (r *helpArticleRepository) Search(ctx context.Context, query string, limit, offset int) ([]*HelpArticle, int64, error) {
	var articles []*HelpArticle
	var total int64

	searchPattern := "%" + query + "%"
	dbQuery := r.db.WithContext(ctx).Model(&HelpArticle{}).
		Where("is_published = ?", true).
		Where("title LIKE ? OR content LIKE ? OR tags LIKE ?", searchPattern, searchPattern, searchPattern)

	// Count total
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count search results", err)
	}

	// Apply pagination
	if limit > 0 {
		dbQuery = dbQuery.Limit(limit)
	}
	if offset > 0 {
		dbQuery = dbQuery.Offset(offset)
	}

	// Fetch results
	if err := dbQuery.Order("view_count DESC, updated_at DESC").Find(&articles).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to search help articles", err)
	}

	return articles, total, nil
}

// GetFeatured retrieves featured help articles.
func (r *helpArticleRepository) GetFeatured(ctx context.Context, limit int) ([]*HelpArticle, error) {
	var articles []*HelpArticle
	result := r.db.WithContext(ctx).
		Where("is_published = ? AND is_featured = ?", true, true).
		Order("updated_at DESC").
		Limit(limit).
		Find(&articles)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get featured help articles", result.Error)
	}
	return articles, nil
}

// IncrementViewCount increments the view count for an article.
func (r *helpArticleRepository) IncrementViewCount(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Model(&HelpArticle{}).
		Where("id = ?", id).
		Update("view_count", gorm.Expr("view_count + 1"))
	if result.Error != nil {
		return errors.NewDatabaseError("failed to increment view count", result.Error)
	}
	return nil
}

// IncrementHelpfulCount increments the helpful count for an article.
func (r *helpArticleRepository) IncrementHelpfulCount(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Model(&HelpArticle{}).
		Where("id = ?", id).
		Update("helpful_count", gorm.Expr("helpful_count + 1"))
	if result.Error != nil {
		return errors.NewDatabaseError("failed to increment helpful count", result.Error)
	}
	return nil
}

// GetCategories retrieves all unique categories.
func (r *helpArticleRepository) GetCategories(ctx context.Context) ([]string, error) {
	var categories []string
	result := r.db.WithContext(ctx).Model(&HelpArticle{}).
		Where("is_published = ?", true).
		Distinct("category").
		Pluck("category", &categories)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get categories", result.Error)
	}
	return categories, nil
}
