// Package database provides data models for the V Panel application.
package database

import (
	"time"

	"gorm.io/datatypes"
)

// User represents a user in the system.
type User struct {
	ID               int64     `json:"id" gorm:"primaryKey"`
	Username         string    `json:"username" gorm:"uniqueIndex;size:50;not null"`
	Password         string    `json:"-" gorm:"size:255;not null"`
	Email            string    `json:"email" gorm:"uniqueIndex;size:100"`
	Role             string    `json:"role" gorm:"size:20;default:user"`
	IsAdmin          bool      `json:"is_admin" gorm:"default:false"`
	Enabled          bool      `json:"enabled" gorm:"default:true"`
	TrafficLimit     int64     `json:"traffic_limit" gorm:"default:0"` // 0 means unlimited
	TrafficUsed      int64     `json:"traffic_used" gorm:"default:0"`
	MaxConcurrentIPs int       `json:"max_concurrent_ips" gorm:"default:-1"` // -1 means use plan default, 0 means unlimited
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// User Portal fields
	EmailVerified    bool       `json:"email_verified" gorm:"default:false"`
	EmailVerifiedAt  *time.Time `json:"email_verified_at"`
	TwoFactorEnabled bool       `json:"two_factor_enabled" gorm:"default:false"`
	LastLoginAt      *time.Time `json:"last_login_at"`
	LastLoginIP      string     `json:"last_login_ip" gorm:"size:45"`
	AvatarURL        string     `json:"avatar_url" gorm:"size:512"`
	DisplayName      string     `json:"display_name" gorm:"size:64"`
	TelegramID       *int64     `json:"telegram_id" gorm:"index"`
	NotifyEmail      bool       `json:"notify_email" gorm:"default:true"`
	NotifyTelegram   bool       `json:"notify_telegram" gorm:"default:false"`
	Theme            string     `json:"theme" gorm:"size:16;default:auto"` // auto, light, dark
	Language         string     `json:"language" gorm:"size:8;default:zh-CN"`
	InvitedBy        *int64     `json:"invited_by" gorm:"index"`

	// Commercial System fields
	Balance     int64 `json:"balance" gorm:"default:0"` // cents
	AutoRenewal bool  `json:"auto_renewal" gorm:"default:false"`
}

// TableName returns the table name for User.
func (User) TableName() string {
	return "users"
}

// Proxy represents a proxy configuration.
type Proxy struct {
	ID        int64          `json:"id" gorm:"primaryKey"`
	UserID    int64          `json:"user_id" gorm:"index;not null"`
	Name      string         `json:"name" gorm:"size:100;not null"`
	Protocol  string         `json:"protocol" gorm:"size:20;not null"` // vmess, vless, trojan, shadowsocks
	Port      int            `json:"port" gorm:"index;not null"`
	Settings  datatypes.JSON `json:"settings"` // Protocol-specific settings
	Enabled   bool           `json:"enabled" gorm:"default:true"`
	Remark    string         `json:"remark" gorm:"size:255"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// TableName returns the table name for Proxy.
func (Proxy) TableName() string {
	return "proxies"
}

// Traffic represents traffic statistics.
type Traffic struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	UserID     int64     `json:"user_id" gorm:"index;not null"`
	ProxyID    int64     `json:"proxy_id" gorm:"index"`
	Upload     int64     `json:"upload" gorm:"default:0"`
	Download   int64     `json:"download" gorm:"default:0"`
	RecordedAt time.Time `json:"recorded_at" gorm:"index;not null"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName returns the table name for Traffic.
func (Traffic) TableName() string {
	return "traffic"
}

// Certificate represents an SSL certificate.
type Certificate struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	Domain      string    `json:"domain" gorm:"uniqueIndex;size:255;not null"`
	Certificate string    `json:"certificate" gorm:"type:text"`
	PrivateKey  string    `json:"-" gorm:"type:text"`
	AutoRenew   bool      `json:"auto_renew" gorm:"default:true"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName returns the table name for Certificate.
func (Certificate) TableName() string {
	return "certificates"
}

// Setting represents a system setting.
type Setting struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"uniqueIndex;size:100;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for Setting.
func (Setting) TableName() string {
	return "settings"
}

// Log represents a system log entry.
type Log struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Level     string    `json:"level" gorm:"size:10;index"`
	Message   string    `json:"message" gorm:"type:text"`
	Source    string    `json:"source" gorm:"size:50;index"`
	UserID    *int64    `json:"user_id" gorm:"index"`
	IP        string    `json:"ip" gorm:"size:50"`
	UserAgent string    `json:"user_agent" gorm:"size:255"`
	RequestID string    `json:"request_id" gorm:"size:100;index"`
	Fields    string    `json:"fields" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName returns the table name for Log.
func (Log) TableName() string {
	return "logs"
}

// Plan represents a subscription plan with its features and limits.
type Plan struct {
	ID                      int64     `json:"id" gorm:"primaryKey"`
	Name                    string    `json:"name" gorm:"size:100;not null"`
	Description             string    `json:"description" gorm:"size:500"`
	TrafficLimit            int64     `json:"traffic_limit" gorm:"default:0"` // 0 means unlimited
	DurationDays            int       `json:"duration_days" gorm:"default:30"`
	DefaultMaxConcurrentIPs int       `json:"default_max_concurrent_ips" gorm:"default:3"` // default concurrent IP limit for this plan
	Price                   float64   `json:"price" gorm:"default:0"`
	Enabled                 bool      `json:"enabled" gorm:"default:true"`
	SortOrder               int       `json:"sort_order" gorm:"default:0"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// TableName returns the table name for Plan.
func (Plan) TableName() string {
	return "plans"
}

// Subscription represents a user's subscription record for proxy configurations.
type Subscription struct {
	ID           int64      `json:"id" gorm:"primaryKey"`
	UserID       int64      `json:"user_id" gorm:"uniqueIndex;not null"`
	Token        string     `json:"token" gorm:"uniqueIndex;size:64;not null"`
	ShortCode    string     `json:"short_code" gorm:"uniqueIndex;size:16"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastAccessAt *time.Time `json:"last_access_at"`
	AccessCount  int64      `json:"access_count" gorm:"default:0"`
	LastIP       string     `json:"last_ip" gorm:"size:45"`
	LastUA       string     `json:"last_ua" gorm:"size:256"`
}

// TableName returns the table name for Subscription.
func (Subscription) TableName() string {
	return "subscriptions"
}

// Ticket represents a support ticket submitted by a user.
type Ticket struct {
	ID        int64      `json:"id" gorm:"primaryKey"`
	UserID    int64      `json:"user_id" gorm:"index;not null"`
	Subject   string     `json:"subject" gorm:"size:256;not null"`
	Status    string     `json:"status" gorm:"size:32;default:open;index"` // open, pending, resolved, closed
	Priority  string     `json:"priority" gorm:"size:32;default:normal"`   // low, normal, high, urgent
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at"`

	User     *User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Messages []TicketMessage `json:"messages,omitempty" gorm:"foreignKey:TicketID"`
}

// TableName returns the table name for Ticket.
func (Ticket) TableName() string {
	return "tickets"
}

// TicketStatus constants
const (
	TicketStatusOpen     = "open"
	TicketStatusPending  = "pending"
	TicketStatusResolved = "resolved"
	TicketStatusClosed   = "closed"
)

// TicketPriority constants
const (
	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"
)

// TicketMessage represents a message in a ticket conversation.
type TicketMessage struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	TicketID    int64     `json:"ticket_id" gorm:"index;not null"`
	UserID      *int64    `json:"user_id" gorm:"index"` // null for system/admin messages
	Content     string    `json:"content" gorm:"type:text;not null"`
	IsAdmin     bool      `json:"is_admin" gorm:"default:false"`
	Attachments string    `json:"attachments" gorm:"type:text"` // JSON array of attachment URLs
	CreatedAt   time.Time `json:"created_at"`

	Ticket *Ticket `json:"ticket,omitempty" gorm:"foreignKey:TicketID"`
	User   *User   `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for TicketMessage.
func (TicketMessage) TableName() string {
	return "ticket_messages"
}

// Announcement represents a system announcement.
type Announcement struct {
	ID          int64      `json:"id" gorm:"primaryKey"`
	Title       string     `json:"title" gorm:"size:256;not null"`
	Content     string     `json:"content" gorm:"type:text;not null"`
	Category    string     `json:"category" gorm:"size:64;default:general;index"` // general, maintenance, update, promotion
	IsPinned    bool       `json:"is_pinned" gorm:"default:false"`
	IsPublished bool       `json:"is_published" gorm:"default:false;index"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName returns the table name for Announcement.
func (Announcement) TableName() string {
	return "announcements"
}

// AnnouncementCategory constants
const (
	AnnouncementCategoryGeneral     = "general"
	AnnouncementCategoryMaintenance = "maintenance"
	AnnouncementCategoryUpdate      = "update"
	AnnouncementCategoryPromotion   = "promotion"
)

// AnnouncementRead tracks read status per user.
type AnnouncementRead struct {
	ID             int64     `json:"id" gorm:"primaryKey"`
	UserID         int64     `json:"user_id" gorm:"index;not null"`
	AnnouncementID int64     `json:"announcement_id" gorm:"index;not null"`
	ReadAt         time.Time `json:"read_at"`

	User         *User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Announcement *Announcement `json:"announcement,omitempty" gorm:"foreignKey:AnnouncementID"`
}

// TableName returns the table name for AnnouncementRead.
func (AnnouncementRead) TableName() string {
	return "announcement_reads"
}

// HelpArticle represents a knowledge base article.
type HelpArticle struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	Slug         string    `json:"slug" gorm:"uniqueIndex;size:128;not null"`
	Title        string    `json:"title" gorm:"size:256;not null"`
	Content      string    `json:"content" gorm:"type:text;not null"`
	Category     string    `json:"category" gorm:"size:64;index"` // getting-started, client-setup, troubleshooting, faq
	Tags         string    `json:"tags" gorm:"size:512"`          // JSON array
	ViewCount    int64     `json:"view_count" gorm:"default:0"`
	HelpfulCount int64     `json:"helpful_count" gorm:"default:0"`
	IsPublished  bool      `json:"is_published" gorm:"default:false;index"`
	IsFeatured   bool      `json:"is_featured" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName returns the table name for HelpArticle.
func (HelpArticle) TableName() string {
	return "help_articles"
}

// HelpArticleCategory constants
const (
	HelpCategoryGettingStarted  = "getting-started"
	HelpCategoryClientSetup     = "client-setup"
	HelpCategoryTroubleshooting = "troubleshooting"
	HelpCategoryFAQ             = "faq"
)

// PasswordResetToken for password reset functionality.
type PasswordResetToken struct {
	ID        int64      `json:"id" gorm:"primaryKey"`
	UserID    int64      `json:"user_id" gorm:"index;not null"`
	Token     string     `json:"token" gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for PasswordResetToken.
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// EmailVerificationToken for email verification.
type EmailVerificationToken struct {
	ID         int64      `json:"id" gorm:"primaryKey"`
	UserID     int64      `json:"user_id" gorm:"index;not null"`
	Email      string     `json:"email" gorm:"size:256;not null"`
	Token      string     `json:"token" gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"not null"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for EmailVerificationToken.
func (EmailVerificationToken) TableName() string {
	return "email_verification_tokens"
}

// InviteCode for invitation system.
type InviteCode struct {
	ID        int64      `json:"id" gorm:"primaryKey"`
	Code      string     `json:"code" gorm:"uniqueIndex;size:32;not null"`
	CreatedBy *int64     `json:"created_by" gorm:"index"` // Admin or user who created
	UsedBy    *int64     `json:"used_by" gorm:"index"`    // User who used the code
	MaxUses   int        `json:"max_uses" gorm:"default:1"`
	UsedCount int        `json:"used_count" gorm:"default:0"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UsedAt    *time.Time `json:"used_at"`

	Creator    *User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	UsedByUser *User `json:"used_by_user,omitempty" gorm:"foreignKey:UsedBy"`
}

// TableName returns the table name for InviteCode.
func (InviteCode) TableName() string {
	return "invite_codes"
}

// TwoFactorSecret for 2FA.
type TwoFactorSecret struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	UserID      int64     `json:"user_id" gorm:"uniqueIndex;not null"`
	Secret      string    `json:"-" gorm:"size:64;not null"` // Encrypted TOTP secret
	BackupCodes string    `json:"-" gorm:"type:text"`        // JSON array of hashed backup codes
	EnabledAt   time.Time `json:"enabled_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for TwoFactorSecret.
func (TwoFactorSecret) TableName() string {
	return "two_factor_secrets"
}

// ============================================
// Commercial System Models
// ============================================

// CommercialPlan represents a commercial service plan with pricing and features.
type CommercialPlan struct {
	ID             int64     `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:128;not null"`
	Description    string    `json:"description" gorm:"type:text"`
	TrafficLimit   int64     `json:"traffic_limit" gorm:"default:0"`             // bytes, 0 = unlimited
	Duration       int       `json:"duration" gorm:"not null"`                   // days
	Price          int64     `json:"price" gorm:"not null"`                      // cents
	PlanType       string    `json:"plan_type" gorm:"size:32;default:monthly"`   // monthly, quarterly, yearly, traffic
	ResetCycle     string    `json:"reset_cycle" gorm:"size:32;default:monthly"` // monthly, on_purchase, never
	IPLimit        int       `json:"ip_limit" gorm:"default:0"`                  // 0 = unlimited
	SortOrder      int       `json:"sort_order" gorm:"default:0"`
	IsActive       bool      `json:"is_active" gorm:"default:true"`
	IsRecommended  bool      `json:"is_recommended" gorm:"default:false"`
	GroupID        *int64    `json:"group_id" gorm:"index"`
	PaymentMethods string    `json:"payment_methods" gorm:"type:text"` // JSON array
	Features       string    `json:"features" gorm:"type:text"`        // JSON array
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName returns the table name for CommercialPlan.
func (CommercialPlan) TableName() string {
	return "commercial_plans"
}

// PlanType constants
const (
	PlanTypeMonthly   = "monthly"
	PlanTypeQuarterly = "quarterly"
	PlanTypeYearly    = "yearly"
	PlanTypeTraffic   = "traffic"
)

// ResetCycle constants
const (
	ResetCycleMonthly    = "monthly"
	ResetCycleOnPurchase = "on_purchase"
	ResetCycleNever      = "never"
)

// PlanGroup represents a category for organizing plans.
type PlanGroup struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:64;not null"`
	SortOrder int       `json:"sort_order" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for PlanGroup.
func (PlanGroup) TableName() string {
	return "plan_groups"
}

// Order represents a purchase order.
type Order struct {
	ID             int64      `json:"id" gorm:"primaryKey"`
	OrderNo        string     `json:"order_no" gorm:"uniqueIndex;size:64;not null"` // ORD-20260114-XXXX
	UserID         int64      `json:"user_id" gorm:"index;not null"`
	PlanID         int64      `json:"plan_id" gorm:"index;not null"`
	CouponID       *int64     `json:"coupon_id" gorm:"index"`
	OriginalAmount int64      `json:"original_amount" gorm:"not null"`             // cents
	DiscountAmount int64      `json:"discount_amount" gorm:"default:0"`            // cents
	BalanceUsed    int64      `json:"balance_used" gorm:"default:0"`               // cents
	PayAmount      int64      `json:"pay_amount" gorm:"not null"`                  // cents (actual payment)
	Status         string     `json:"status" gorm:"size:32;default:pending;index"` // pending, paid, completed, cancelled, refunded
	PaymentMethod  string     `json:"payment_method" gorm:"size:32"`
	PaymentNo      string     `json:"payment_no" gorm:"size:128;index"` // external payment ID
	PaidAt         *time.Time `json:"paid_at"`
	ExpiredAt      time.Time  `json:"expired_at" gorm:"index;not null"`
	Notes          string     `json:"notes" gorm:"type:text"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	User   *User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Plan   *CommercialPlan `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
	Coupon *Coupon         `json:"coupon,omitempty" gorm:"foreignKey:CouponID"`
}

// TableName returns the table name for Order.
func (Order) TableName() string {
	return "orders"
}

// OrderStatus constants
const (
	OrderStatusPending   = "pending"
	OrderStatusPaid      = "paid"
	OrderStatusCompleted = "completed"
	OrderStatusCancelled = "cancelled"
	OrderStatusRefunded  = "refunded"
)

// PaymentMethod constants
const (
	PaymentMethodAlipay  = "alipay"
	PaymentMethodWechat  = "wechat"
	PaymentMethodPaypal  = "paypal"
	PaymentMethodCrypto  = "crypto"
	PaymentMethodBalance = "balance"
)

// BalanceTransaction represents a balance change record.
type BalanceTransaction struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	UserID      int64     `json:"user_id" gorm:"index;not null"`
	Type        string    `json:"type" gorm:"size:32;not null"` // recharge, purchase, refund, commission, adjustment
	Amount      int64     `json:"amount" gorm:"not null"`       // cents, positive or negative
	Balance     int64     `json:"balance" gorm:"not null"`      // balance after transaction
	OrderID     *int64    `json:"order_id" gorm:"index"`
	Description string    `json:"description" gorm:"size:256"`
	Operator    string    `json:"operator" gorm:"size:64"` // system, admin username
	CreatedAt   time.Time `json:"created_at" gorm:"index"`

	User  *User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Order *Order `json:"order,omitempty" gorm:"foreignKey:OrderID"`
}

// TableName returns the table name for BalanceTransaction.
func (BalanceTransaction) TableName() string {
	return "balance_transactions"
}

// BalanceTransactionType constants
const (
	BalanceTxTypeRecharge   = "recharge"
	BalanceTxTypePurchase   = "purchase"
	BalanceTxTypeRefund     = "refund"
	BalanceTxTypeCommission = "commission"
	BalanceTxTypeAdjustment = "adjustment"
)

// Coupon represents a discount coupon.
type Coupon struct {
	ID             int64     `json:"id" gorm:"primaryKey"`
	Code           string    `json:"code" gorm:"uniqueIndex;size:32;not null"`
	Name           string    `json:"name" gorm:"size:128;not null"`
	Type           string    `json:"type" gorm:"size:16;not null"`      // fixed, percentage
	Value          int64     `json:"value" gorm:"not null"`             // cents or percentage * 100
	MinOrderAmount int64     `json:"min_order_amount" gorm:"default:0"` // cents
	MaxDiscount    int64     `json:"max_discount" gorm:"default:0"`     // cents, for percentage type
	TotalLimit     int       `json:"total_limit" gorm:"default:0"`      // 0 = unlimited
	PerUserLimit   int       `json:"per_user_limit" gorm:"default:1"`   // 0 = unlimited
	UsedCount      int       `json:"used_count" gorm:"default:0"`
	PlanIDs        string    `json:"plan_ids" gorm:"type:text"` // JSON array, empty = all plans
	StartAt        time.Time `json:"start_at" gorm:"not null"`
	ExpireAt       time.Time `json:"expire_at" gorm:"not null;index"`
	IsActive       bool      `json:"is_active" gorm:"default:true;index"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName returns the table name for Coupon.
func (Coupon) TableName() string {
	return "coupons"
}

// CouponType constants
const (
	CouponTypeFixed      = "fixed"
	CouponTypePercentage = "percentage"
)

// CouponUsage tracks coupon usage per user.
type CouponUsage struct {
	ID       int64     `json:"id" gorm:"primaryKey"`
	CouponID int64     `json:"coupon_id" gorm:"index;not null"`
	UserID   int64     `json:"user_id" gorm:"index;not null"`
	OrderID  int64     `json:"order_id" gorm:"index;not null"`
	Discount int64     `json:"discount" gorm:"not null"` // cents
	UsedAt   time.Time `json:"used_at" gorm:"not null"`

	Coupon *Coupon `json:"coupon,omitempty" gorm:"foreignKey:CouponID"`
	User   *User   `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Order  *Order  `json:"order,omitempty" gorm:"foreignKey:OrderID"`
}

// TableName returns the table name for CouponUsage.
func (CouponUsage) TableName() string {
	return "coupon_usages"
}

// CommercialInviteCode represents a user's invite code for referral system.
type CommercialInviteCode struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	UserID      int64     `json:"user_id" gorm:"uniqueIndex;not null"`
	Code        string    `json:"code" gorm:"uniqueIndex;size:16;not null"`
	InviteCount int       `json:"invite_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for CommercialInviteCode.
func (CommercialInviteCode) TableName() string {
	return "commercial_invite_codes"
}

// Referral represents a referral relationship.
type Referral struct {
	ID          int64      `json:"id" gorm:"primaryKey"`
	InviterID   int64      `json:"inviter_id" gorm:"index;not null"`
	InviteeID   int64      `json:"invitee_id" gorm:"uniqueIndex;not null"`
	InviteCode  string     `json:"invite_code" gorm:"size:16;not null"`
	Status      string     `json:"status" gorm:"size:32;default:registered"` // registered, converted
	ConvertedAt *time.Time `json:"converted_at"`
	CreatedAt   time.Time  `json:"created_at"`

	Inviter *User `json:"inviter,omitempty" gorm:"foreignKey:InviterID"`
	Invitee *User `json:"invitee,omitempty" gorm:"foreignKey:InviteeID"`
}

// TableName returns the table name for Referral.
func (Referral) TableName() string {
	return "referrals"
}

// ReferralStatus constants
const (
	ReferralStatusRegistered = "registered"
	ReferralStatusConverted  = "converted"
)

// Commission represents a commission record for referrals.
type Commission struct {
	ID         int64      `json:"id" gorm:"primaryKey"`
	UserID     int64      `json:"user_id" gorm:"index;not null"`      // inviter
	FromUserID int64      `json:"from_user_id" gorm:"index;not null"` // invitee
	OrderID    int64      `json:"order_id" gorm:"index;not null"`
	Amount     int64      `json:"amount" gorm:"not null"` // cents
	Rate       float64    `json:"rate" gorm:"not null"`
	Level      int        `json:"level" gorm:"default:1"`                      // referral level
	Status     string     `json:"status" gorm:"size:32;default:pending;index"` // pending, confirmed, cancelled
	ConfirmAt  *time.Time `json:"confirm_at"`
	CreatedAt  time.Time  `json:"created_at"`

	User     *User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
	FromUser *User  `json:"from_user,omitempty" gorm:"foreignKey:FromUserID"`
	Order    *Order `json:"order,omitempty" gorm:"foreignKey:OrderID"`
}

// TableName returns the table name for Commission.
func (Commission) TableName() string {
	return "commissions"
}

// CommissionStatus constants
const (
	CommissionStatusPending   = "pending"
	CommissionStatusConfirmed = "confirmed"
	CommissionStatusCancelled = "cancelled"
)

// Invoice represents an invoice for an order.
type Invoice struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	InvoiceNo string    `json:"invoice_no" gorm:"uniqueIndex;size:64;not null"`
	OrderID   int64     `json:"order_id" gorm:"index;not null"`
	UserID    int64     `json:"user_id" gorm:"index;not null"`
	Amount    int64     `json:"amount" gorm:"not null"`            // cents
	Content   string    `json:"content" gorm:"type:text;not null"` // JSON with line items
	PDFPath   string    `json:"pdf_path" gorm:"size:256"`
	CreatedAt time.Time `json:"created_at"`

	Order *Order `json:"order,omitempty" gorm:"foreignKey:OrderID"`
	User  *User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for Invoice.
func (Invoice) TableName() string {
	return "invoices"
}

// UserBalance extends User with balance field (for migration reference).
// Note: Balance field should be added to User model via migration.
// ALTER TABLE users ADD COLUMN balance BIGINT DEFAULT 0;
// ALTER TABLE users ADD COLUMN auto_renewal BOOLEAN DEFAULT FALSE;

// ============================================
// Trial System Models
// ============================================

// Trial represents a user's trial subscription.
type Trial struct {
	ID          int64      `json:"id" gorm:"primaryKey"`
	UserID      int64      `json:"user_id" gorm:"uniqueIndex;not null"`
	Status      string     `json:"status" gorm:"size:32;default:active;index"` // active, expired, converted
	StartAt     time.Time  `json:"start_at" gorm:"not null"`
	ExpireAt    time.Time  `json:"expire_at" gorm:"not null;index"`
	TrafficUsed int64      `json:"traffic_used" gorm:"default:0"`
	ConvertedAt *time.Time `json:"converted_at"`
	CreatedAt   time.Time  `json:"created_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for Trial.
func (Trial) TableName() string {
	return "trials"
}

// TrialStatus constants
const (
	TrialStatusActive    = "active"
	TrialStatusExpired   = "expired"
	TrialStatusConverted = "converted"
)

// ============================================
// Plan Change Models
// ============================================

// PendingDowngrade represents a scheduled plan downgrade.
type PendingDowngrade struct {
	ID            int64     `json:"id" gorm:"primaryKey"`
	UserID        int64     `json:"user_id" gorm:"uniqueIndex;not null"`
	CurrentPlanID int64     `json:"current_plan_id" gorm:"not null"`
	NewPlanID     int64     `json:"new_plan_id" gorm:"not null"`
	EffectiveAt   time.Time `json:"effective_at" gorm:"not null;index"`
	CreatedAt     time.Time `json:"created_at"`

	User        *User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	CurrentPlan *CommercialPlan `json:"current_plan,omitempty" gorm:"foreignKey:CurrentPlanID"`
	NewPlan     *CommercialPlan `json:"new_plan,omitempty" gorm:"foreignKey:NewPlanID"`
}

// TableName returns the table name for PendingDowngrade.
func (PendingDowngrade) TableName() string {
	return "pending_downgrades"
}

// ============================================
// Currency Support Models
// ============================================

// ExchangeRate represents an exchange rate between two currencies.
type ExchangeRate struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	FromCurrency string    `json:"from_currency" gorm:"size:3;not null;uniqueIndex:idx_exchange_rate_pair,priority:1"`
	ToCurrency   string    `json:"to_currency" gorm:"size:3;not null;uniqueIndex:idx_exchange_rate_pair,priority:2"`
	Rate         float64   `json:"rate" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}

// TableName returns the table name for ExchangeRate.
func (ExchangeRate) TableName() string {
	return "exchange_rates"
}

// PlanPrice represents a price for a plan in a specific currency.
type PlanPrice struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	PlanID    int64     `json:"plan_id" gorm:"index;not null"`
	Currency  string    `json:"currency" gorm:"size:3;not null;uniqueIndex:idx_plan_price_unique,priority:2"`
	Price     int64     `json:"price" gorm:"not null"` // cents
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Plan *CommercialPlan `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
}

// TableName returns the table name for PlanPrice.
func (PlanPrice) TableName() string {
	return "plan_prices"
}

// ============================================
// Subscription Pause Models
// ============================================

// SubscriptionPause represents a subscription pause record.
type SubscriptionPause struct {
	ID               int64      `json:"id" gorm:"primaryKey"`
	UserID           int64      `json:"user_id" gorm:"index;not null"`
	PausedAt         time.Time  `json:"paused_at" gorm:"not null"`
	ResumedAt        *time.Time `json:"resumed_at"`
	RemainingDays    int        `json:"remaining_days" gorm:"not null"`
	RemainingTraffic int64      `json:"remaining_traffic" gorm:"not null"`
	AutoResumeAt     time.Time  `json:"auto_resume_at" gorm:"not null;index"`
	CreatedAt        time.Time  `json:"created_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for SubscriptionPause.
func (SubscriptionPause) TableName() string {
	return "subscription_pauses"
}

// PauseStatus constants
const (
	PauseStatusActive  = "active"
	PauseStatusResumed = "resumed"
)

// ============================================
// Gift Card Models
// ============================================

// GiftCard represents a gift card that can be redeemed for balance.
type GiftCard struct {
	ID          int64      `json:"id" gorm:"primaryKey"`
	Code        string     `json:"code" gorm:"uniqueIndex;size:32;not null"`
	Value       int64      `json:"value" gorm:"not null"`                      // cents
	Status      string     `json:"status" gorm:"size:32;default:active;index"` // active, redeemed, expired, disabled
	CreatedBy   *int64     `json:"created_by" gorm:"index"`                    // Admin who created
	PurchasedBy *int64     `json:"purchased_by" gorm:"index"`                  // User who purchased (if purchased)
	RedeemedBy  *int64     `json:"redeemed_by" gorm:"index"`                   // User who redeemed
	BatchID     string     `json:"batch_id" gorm:"size:64;index"`              // For batch creation tracking
	ExpiresAt   *time.Time `json:"expires_at" gorm:"index"`
	RedeemedAt  *time.Time `json:"redeemed_at"`
	PurchasedAt *time.Time `json:"purchased_at"`
	CreatedAt   time.Time  `json:"created_at"`

	Creator   *User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Purchaser *User `json:"purchaser,omitempty" gorm:"foreignKey:PurchasedBy"`
	Redeemer  *User `json:"redeemer,omitempty" gorm:"foreignKey:RedeemedBy"`
}

// TableName returns the table name for GiftCard.
func (GiftCard) TableName() string {
	return "gift_cards"
}

// GiftCardStatus constants
const (
	GiftCardStatusActive   = "active"
	GiftCardStatusRedeemed = "redeemed"
	GiftCardStatusExpired  = "expired"
	GiftCardStatusDisabled = "disabled"
)

// BalanceTxTypeGiftCard is the transaction type for gift card redemption
const BalanceTxTypeGiftCard = "gift_card"

// ============================================
// Multi-Server Management Models
// ============================================

// Node represents a remote Xray node server.
type Node struct {
	ID                int64      `json:"id" gorm:"primaryKey"`
	Name              string     `json:"name" gorm:"size:128;not null"`
	Address           string     `json:"address" gorm:"size:256;not null"`            // IP or domain
	Port              int        `json:"port" gorm:"default:18443"`                   // Agent port
	Token             string     `json:"-" gorm:"size:64;uniqueIndex"`                // Authentication token
	PanelURL          string     `json:"panel_url" gorm:"size:256"`                   // Panel server URL for agent connection
	Status            string     `json:"status" gorm:"size:32;default:offline;index"` // online, offline, unhealthy
	Tags              string     `json:"tags" gorm:"type:text"`                       // JSON array
	Region            string     `json:"region" gorm:"size:64;index"`
	Weight            int        `json:"weight" gorm:"default:1"`    // Load balancing weight
	MaxUsers          int        `json:"max_users" gorm:"default:0"` // 0 = unlimited
	CurrentUsers      int        `json:"current_users" gorm:"default:0"`
	Latency           int        `json:"latency" gorm:"default:0"` // milliseconds
	LastSeenAt        *time.Time `json:"last_seen_at"`
	SyncStatus        string     `json:"sync_status" gorm:"size:32;default:pending"` // synced, pending, failed
	SyncedAt          *time.Time `json:"synced_at"`
	InstallStatus     string     `json:"install_status" gorm:"size:32;default:idle;index"`
	InstallMessage    string     `json:"install_message" gorm:"size:255"`
	InstallSteps      string     `json:"install_steps" gorm:"type:text"`
	InstallLogs       string     `json:"install_logs" gorm:"type:text"`
	InstallStartedAt  *time.Time `json:"install_started_at"`
	InstallFinishedAt *time.Time `json:"install_finished_at"`
	InstallUpdatedAt  *time.Time `json:"install_updated_at"`
	IPWhitelist       string     `json:"ip_whitelist" gorm:"type:text"` // JSON array of allowed IPs
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// TableName returns the table name for Node.
func (Node) TableName() string {
	return "nodes"
}

// NodeStatus constants
const (
	NodeStatusOnline    = "online"
	NodeStatusOffline   = "offline"
	NodeStatusUnhealthy = "unhealthy"
)

// NodeSyncStatus constants
const (
	NodeSyncStatusSynced  = "synced"
	NodeSyncStatusPending = "pending"
	NodeSyncStatusFailed  = "failed"
)

// NodeGroup represents a group of nodes for organization and load balancing.
type NodeGroup struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	Description string    `json:"description" gorm:"size:256"`
	Region      string    `json:"region" gorm:"size:64"`
	Strategy    string    `json:"strategy" gorm:"size:32;default:round-robin"` // Load balancing strategy
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName returns the table name for NodeGroup.
func (NodeGroup) TableName() string {
	return "node_groups"
}

// NodeGroupMember represents the many-to-many relationship between nodes and groups.
type NodeGroupMember struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	NodeID    int64     `json:"node_id" gorm:"uniqueIndex:idx_node_group_member,priority:1;not null"`
	GroupID   int64     `json:"group_id" gorm:"uniqueIndex:idx_node_group_member,priority:2;index;not null"`
	CreatedAt time.Time `json:"created_at"`

	Node  *Node      `json:"node,omitempty" gorm:"foreignKey:NodeID"`
	Group *NodeGroup `json:"group,omitempty" gorm:"foreignKey:GroupID"`
}

// TableName returns the table name for NodeGroupMember.
func (NodeGroupMember) TableName() string {
	return "node_group_members"
}

// LoadBalanceStrategy constants
const (
	StrategyRoundRobin       = "round-robin"
	StrategyLeastConnections = "least-connections"
	StrategyWeighted         = "weighted"
	StrategyGeographic       = "geographic"
)

// HealthCheck represents a health check record for a node.
type HealthCheck struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	NodeID    int64     `json:"node_id" gorm:"index;not null"`
	Status    string    `json:"status" gorm:"size:32"` // success, failed
	Latency   int       `json:"latency"`               // milliseconds
	Message   string    `json:"message" gorm:"size:512"`
	TCPOk     bool      `json:"tcp_ok" gorm:"default:false"`
	APIOk     bool      `json:"api_ok" gorm:"default:false"`
	XrayOk    bool      `json:"xray_ok" gorm:"default:false"`
	CheckedAt time.Time `json:"checked_at" gorm:"index"`

	Node *Node `json:"node,omitempty" gorm:"foreignKey:NodeID"`
}

// TableName returns the table name for HealthCheck.
func (HealthCheck) TableName() string {
	return "health_checks"
}

// HealthCheckStatus constants
const (
	HealthCheckStatusSuccess = "success"
	HealthCheckStatusFailed  = "failed"
)

// UserNodeAssignment represents the assignment of a user to a specific node.
type UserNodeAssignment struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	UserID     int64     `json:"user_id" gorm:"uniqueIndex;not null"`
	NodeID     int64     `json:"node_id" gorm:"index;not null"`
	AssignedAt time.Time `json:"assigned_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Node *Node `json:"node,omitempty" gorm:"foreignKey:NodeID"`
}

// TableName returns the table name for UserNodeAssignment.
func (UserNodeAssignment) TableName() string {
	return "user_node_assignments"
}

// NodeTraffic represents traffic statistics for a node.
type NodeTraffic struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	NodeID     int64     `json:"node_id" gorm:"index;not null"`
	UserID     int64     `json:"user_id" gorm:"index;not null"`
	ProxyID    *int64    `json:"proxy_id" gorm:"index"`
	Upload     int64     `json:"upload" gorm:"default:0"`   // bytes
	Download   int64     `json:"download" gorm:"default:0"` // bytes
	RecordedAt time.Time `json:"recorded_at" gorm:"index"`
}

// TableName returns the table name for NodeTraffic.
func (NodeTraffic) TableName() string {
	return "node_traffic"
}

// NodeAuthFailure represents authentication failure records for rate limiting.
type NodeAuthFailure struct {
	ID           int64      `json:"id" gorm:"primaryKey"`
	IP           string     `json:"ip" gorm:"size:45;index;not null"`
	Attempts     int        `json:"attempts" gorm:"default:1"`
	BlockedUntil *time.Time `json:"blocked_until"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName returns the table name for NodeAuthFailure.
func (NodeAuthFailure) TableName() string {
	return "node_auth_failures"
}
