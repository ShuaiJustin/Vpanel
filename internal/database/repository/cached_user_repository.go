package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"v/internal/cache"
)

// CachedUserRepository wraps UserRepository with caching.
type CachedUserRepository struct {
	repo  UserRepository
	cache cache.Cache
	ttl   time.Duration
}

// NewCachedUserRepository creates a new cached user repository.
func NewCachedUserRepository(repo UserRepository, c cache.Cache, ttl time.Duration) *CachedUserRepository {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachedUserRepository{
		repo:  repo,
		cache: c,
		ttl:   ttl,
	}
}

// Cache key helpers
func userIDKey(id int64) string {
	return fmt.Sprintf("user:id:%d", id)
}

func userUsernameKey(username string) string {
	return fmt.Sprintf("user:username:%s", username)
}

// Create creates a new user and invalidates related cache.
func (r *CachedUserRepository) Create(ctx context.Context, user *User) error {
	if err := r.repo.Create(ctx, user); err != nil {
		return err
	}
	// Cache the new user
	r.cacheUser(ctx, user)
	return nil
}

// GetByID retrieves a user by ID, using cache if available.
func (r *CachedUserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	key := userIDKey(id)

	// Try cache first
	if data, err := r.cache.Get(ctx, key); err == nil {
		var user User
		if err := json.Unmarshal(data, &user); err == nil {
			return &user, nil
		}
	}

	// Cache miss, get from database
	user, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result
	r.cacheUser(ctx, user)
	return user, nil
}

// GetByUsername retrieves a user by username, using cache if available.
func (r *CachedUserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	key := userUsernameKey(username)

	// Try cache first
	if data, err := r.cache.Get(ctx, key); err == nil {
		var user User
		if err := json.Unmarshal(data, &user); err == nil {
			return &user, nil
		}
	}

	// Cache miss, get from database
	user, err := r.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	// Cache the result
	r.cacheUser(ctx, user)
	return user, nil
}

// GetByEmail retrieves a user by email (not cached to avoid complexity).
func (r *CachedUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return r.repo.GetByEmail(ctx, email)
}

// Update updates a user and invalidates cache.
func (r *CachedUserRepository) Update(ctx context.Context, user *User) error {
	// Get old user to invalidate old username cache if changed
	oldUser, _ := r.repo.GetByID(ctx, user.ID)

	if err := r.repo.Update(ctx, user); err != nil {
		return err
	}

	// Invalidate old cache entries
	r.invalidateUser(ctx, user.ID)
	if oldUser != nil && oldUser.Username != user.Username {
		r.cache.Delete(ctx, userUsernameKey(oldUser.Username))
	}

	// Cache updated user
	r.cacheUser(ctx, user)
	return nil
}

// Delete deletes a user and invalidates cache.
func (r *CachedUserRepository) Delete(ctx context.Context, id int64) error {
	// Get user to invalidate username cache
	user, _ := r.repo.GetByID(ctx, id)

	if err := r.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Invalidate cache
	r.invalidateUser(ctx, id)
	if user != nil {
		r.cache.Delete(ctx, userUsernameKey(user.Username))
	}

	return nil
}

// List retrieves users with pagination (not cached due to pagination complexity).
func (r *CachedUserRepository) List(ctx context.Context, limit, offset int) ([]*User, error) {
	return r.repo.List(ctx, limit, offset)
}

// Count returns the total number of users (not cached).
func (r *CachedUserRepository) Count(ctx context.Context) (int64, error) {
	return r.repo.Count(ctx)
}

// CountActive returns the number of active users (not cached).
func (r *CachedUserRepository) CountActive(ctx context.Context) (int64, error) {
	return r.repo.CountActive(ctx)
}

// cacheUser caches a user by both ID and username.
func (r *CachedUserRepository) cacheUser(ctx context.Context, user *User) {
	data, err := json.Marshal(user)
	if err != nil {
		return
	}

	// Cache by ID
	r.cache.Set(ctx, userIDKey(user.ID), data, r.ttl)
	// Cache by username
	r.cache.Set(ctx, userUsernameKey(user.Username), data, r.ttl)
}

// invalidateUser removes a user from cache by ID.
func (r *CachedUserRepository) invalidateUser(ctx context.Context, id int64) {
	r.cache.Delete(ctx, userIDKey(id))
}

// InvalidateAll invalidates all user cache entries.
func (r *CachedUserRepository) InvalidateAll(ctx context.Context) error {
	return r.cache.InvalidatePattern(ctx, "user:*")
}
