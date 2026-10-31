// Package announcement provides announcement services for the user portal.
package announcement

import (
	"context"
	"encoding/json"
	"strconv"

	"v/internal/cache"
	"v/internal/database/repository"
)

// Service provides announcement operations for the user portal.
type Service struct {
	announcementRepo repository.AnnouncementRepository
	cache            cache.Cache
	cacheKeyBuilder  *cache.CacheKeyBuilder
}

// NewService creates a new announcement service.
func NewService(announcementRepo repository.AnnouncementRepository) *Service {
	return &Service{
		announcementRepo: announcementRepo,
		cacheKeyBuilder:  cache.NewCacheKeyBuilder(),
	}
}

// WithCache injects cache for announcement list caching.
func (s *Service) WithCache(c cache.Cache) *Service {
	s.cache = c
	return s
}

// AnnouncementResult represents an announcement with read status.
type AnnouncementResult struct {
	*repository.Announcement
	IsRead bool `json:"is_read"`
}

// ListAnnouncements retrieves published announcements for a user with read status.
// Uses cache with 2-minute TTL for public announcement data shared across users.
func (s *Service) ListAnnouncements(ctx context.Context, userID int64, limit, offset int) ([]*AnnouncementResult, int64, error) {
	// Calculate page number for cache key
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	cacheKey := s.cacheKeyBuilder.AnnouncementListKey(offset/limit, limit)
	if offset%limit != 0 {
		cacheKey += ":offset:" + strconv.Itoa(offset)
	}

	// Try to get from cache if cache is available
	if s.cache != nil {

		// Try to get cached data
		cachedData, err := s.cache.Get(ctx, cacheKey)
		if err == nil {
			// Cache hit - deserialize and return
			var cached struct {
				Announcements []*repository.Announcement
				Total         int64
			}
			if err := json.Unmarshal(cachedData, &cached); err == nil {
				ids := make([]int64, 0, len(cached.Announcements))
				for _, a := range cached.Announcements {
					ids = append(ids, a.ID)
				}
				// Read markers are best effort, as before: their failure must not hide announcements.
				readIDs, readErr := s.announcementRepo.GetReadIDs(ctx, userID, ids)
				if readErr != nil {
					readIDs = nil
				}
				read := make(map[int64]bool, len(readIDs))
				for _, id := range readIDs {
					read[id] = true
				}
				results := make([]*AnnouncementResult, len(cached.Announcements))
				for i, a := range cached.Announcements {
					results[i] = &AnnouncementResult{Announcement: a, IsRead: read[a.ID]}
				}
				return results, cached.Total, nil
			}
		}
	}

	// Cache miss or cache unavailable - query database
	announcements, total, err := s.announcementRepo.ListWithReadStatus(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	results := make([]*AnnouncementResult, len(announcements))
	for i, a := range announcements {
		results[i] = &AnnouncementResult{
			Announcement: &a.Announcement,
			IsRead:       a.IsRead,
		}
	}

	// Cache the announcement list (without user-specific read status) if cache is available
	if s.cache != nil {

		// Extract just the announcements for caching (public data)
		announcementsOnly := make([]*repository.Announcement, len(announcements))
		for i, a := range announcements {
			announcementsOnly[i] = &a.Announcement
		}

		cacheData := struct {
			Announcements []*repository.Announcement
			Total         int64
		}{
			Announcements: announcementsOnly,
			Total:         total,
		}

		if data, err := json.Marshal(cacheData); err == nil {
			// Use 2-minute TTL for announcement lists (public data)
			ttl := cache.GetTTL("announcements")
			_ = s.cache.Set(ctx, cacheKey, data, ttl)
		}
	}

	return results, total, nil
}

// GetAnnouncement retrieves a single announcement by ID.
func (s *Service) GetAnnouncement(ctx context.Context, id int64) (*repository.Announcement, error) {
	return s.announcementRepo.GetByID(ctx, id)
}

// MarkAsRead marks an announcement as read for a user.
func (s *Service) MarkAsRead(ctx context.Context, userID, announcementID int64) error {
	// Verify announcement exists and is published
	announcement, err := s.announcementRepo.GetByID(ctx, announcementID)
	if err != nil {
		return err
	}

	if !announcement.IsPublished {
		return nil // Silently ignore unpublished announcements
	}

	return s.announcementRepo.MarkAsRead(ctx, userID, announcementID)
}

// IsRead checks if an announcement is read by a user.
func (s *Service) IsRead(ctx context.Context, userID, announcementID int64) (bool, error) {
	return s.announcementRepo.IsRead(ctx, userID, announcementID)
}

// GetUnreadCount gets the count of unread announcements for a user.
func (s *Service) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.announcementRepo.GetUnreadCount(ctx, userID)
}

// GetRecentAnnouncements retrieves the most recent published announcements.
func (s *Service) GetRecentAnnouncements(ctx context.Context, limit int) ([]*repository.Announcement, error) {
	return s.announcementRepo.GetRecent(ctx, limit)
}

// ListByCategory retrieves announcements by category.
func (s *Service) ListByCategory(ctx context.Context, category string, limit, offset int) ([]*repository.Announcement, int64, error) {
	isPublished := true
	return s.announcementRepo.List(ctx, &repository.AnnouncementFilter{
		Category:    &category,
		IsPublished: &isPublished,
		Limit:       limit,
		Offset:      offset,
	})
}

// InvalidateCache invalidates all announcement list caches.
// This should be called when announcements are created, updated, deleted, published, or unpublished.
func (s *Service) InvalidateCache(ctx context.Context) error {
	if s.cache == nil {
		return nil
	}

	invalidator := cache.NewInvalidator(s.cache)
	return invalidator.InvalidateAnnouncementList(ctx)
}
