package notification

import (
	"encoding/json"
	"time"
)

// UserNotificationPreferences stores user's notification preferences
type UserNotificationPreferences struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// IP notification preferences
	NotifyNewDevice      bool `json:"notify_new_device" gorm:"default:true"`
	NotifyIPLimitReached bool `json:"notify_ip_limit_reached" gorm:"default:true"`
	NotifyDeviceKicked   bool `json:"notify_device_kicked" gorm:"default:true"`

	// Notification channels
	EnableEmail    bool `json:"enable_email" gorm:"default:true"`
	EnableTelegram bool `json:"enable_telegram" gorm:"default:false"`

	// Telegram settings (user-specific)
	TelegramChatID string `json:"telegram_chat_id"`
}

// TableName returns the table name for GORM
func (UserNotificationPreferences) TableName() string {
	return "user_notification_preferences"
}

// ToJSON converts preferences to JSON string
func (p *UserNotificationPreferences) ToJSON() string {
	data, _ := json.Marshal(p)
	return string(data)
}

// DefaultPreferences returns default notification preferences
func DefaultPreferences(userID uint) *UserNotificationPreferences {
	return &UserNotificationPreferences{
		UserID:               userID,
		NotifyNewDevice:      true,
		NotifyIPLimitReached: true,
		NotifyDeviceKicked:   true,
		EnableEmail:          true,
		EnableTelegram:       false,
	}
}

// PreferencesRepository handles notification preferences storage
type PreferencesRepository struct {
	// This would typically use GORM or another ORM
	// For now, we'll define the interface
}

// GetByUserID retrieves preferences for a user
func (r *PreferencesRepository) GetByUserID(userID uint) (*UserNotificationPreferences, error) {
	// Implementation would query the database
	return DefaultPreferences(userID), nil
}

// Save saves or updates user preferences
func (r *PreferencesRepository) Save(prefs *UserNotificationPreferences) error {
	// Implementation would save to database
	return nil
}
