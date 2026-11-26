// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Setting represents a system setting in the database.
type Setting struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	Key       string    `gorm:"uniqueIndex;size:100;not null"`
	Value     string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName returns the table name for Setting.
func (Setting) TableName() string {
	return "settings"
}

// SettingsRepository defines the interface for settings data access.
type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, error)
	GetAll(ctx context.Context) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	SetMultiple(ctx context.Context, settings map[string]string) error
	Delete(ctx context.Context, key string) error
	Backup(ctx context.Context) ([]byte, error)
	Restore(ctx context.Context, data []byte) error
}

// settingsRepository implements SettingsRepository using GORM.
type settingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository creates a new settings repository.
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{db: db}
}

// Get retrieves a setting value by key.
func (r *settingsRepository) Get(ctx context.Context, key string) (string, error) {
	var setting Setting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return setting.Value, nil
}

// GetAll retrieves all settings as a map.
func (r *settingsRepository) GetAll(ctx context.Context) (map[string]string, error) {
	var settings []Setting
	err := r.db.WithContext(ctx).Find(&settings).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

// Set creates or updates a setting.
func (r *settingsRepository) Set(ctx context.Context, key, value string) error {
	var setting Setting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new setting
			setting = Setting{
				Key:   key,
				Value: value,
			}
			return r.db.WithContext(ctx).Create(&setting).Error
		}
		return err
	}

	// Update existing setting
	setting.Value = value
	return r.db.WithContext(ctx).Save(&setting).Error
}

// SetMultiple creates or updates multiple settings in a transaction.
func (r *settingsRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range settings {
			var setting Setting
			err := tx.Where("key = ?", key).First(&setting).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// Create new setting
					setting = Setting{
						Key:   key,
						Value: value,
					}
					if err := tx.Create(&setting).Error; err != nil {
						return err
					}
					continue
				}
				return err
			}

			// Update existing setting
			setting.Value = value
			if err := tx.Save(&setting).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete removes a setting by key.
func (r *settingsRepository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Where("key = ?", key).Delete(&Setting{}).Error
}

// Backup exports all settings as JSON.
func (r *settingsRepository) Backup(ctx context.Context) ([]byte, error) {
	settings, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(settings)
}

// Restore imports settings from JSON backup.
func (r *settingsRepository) Restore(ctx context.Context, data []byte) error {
	var settings map[string]string
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}
	return r.SetMultiple(ctx, settings)
}
