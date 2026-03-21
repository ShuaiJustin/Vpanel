// Package repository provides data access implementations.
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// userRepository implements UserRepository.
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create creates a new user.
func (r *userRepository) Create(ctx context.Context, user *User) error {
	if user.Email != "" {
		user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	}
	result := r.db.WithContext(ctx).Create(user)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create user", result.Error)
	}
	return nil
}

// GetByID retrieves a user by ID.
func (r *userRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	var user User
	result := r.db.WithContext(ctx).First(&user, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("user", id)
		}
		return nil, errors.NewDatabaseError("failed to get user", result.Error)
	}
	return &user, nil
}

// GetByUsername retrieves a user by username.
func (r *userRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	result := r.db.WithContext(ctx).Where("username = ?", username).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("user", username)
		}
		return nil, errors.NewDatabaseError("failed to get user", result.Error)
	}
	return &user, nil
}

// GetByEmail retrieves a user by email.
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	result := r.db.WithContext(ctx).Where("LOWER(TRIM(email)) = ?", normalizedEmail).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("user", normalizedEmail)
		}
		return nil, errors.NewDatabaseError("failed to get user", result.Error)
	}
	return &user, nil
}

// Update updates a user.
func (r *userRepository) Update(ctx context.Context, user *User) error {
	if user.Email != "" {
		user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	}
	result := r.db.WithContext(ctx).Save(user)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update user", result.Error)
	}
	return nil
}

// Delete deletes a user by ID.
func (r *userRepository) Delete(ctx context.Context, id int64) error {
	user, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	result := r.db.WithContext(ctx).Delete(user)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete user", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("user", id)
	}
	return nil
}

// List retrieves users with pagination.
func (r *userRepository) List(ctx context.Context, limit, offset int) ([]*User, error) {
	var users []*User
	result := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&users)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to list users", result.Error)
	}
	return users, nil
}

// Count returns the total number of users.
func (r *userRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&User{}).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count users", result.Error)
	}
	return count, nil
}

// CountActive returns the number of active users (enabled and not expired).
func (r *userRepository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&User{}).
		Where("enabled = ?", true).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count active users", result.Error)
	}
	return count, nil
}
