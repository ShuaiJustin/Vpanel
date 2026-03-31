// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// User represents a user in the database.
type User struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement"`
	Username            string     `gorm:"uniqueIndex;size:50;not null"`
	PasswordHash        string     `gorm:"column:password;size:255;not null"`
	Email               string     `gorm:"size:100"`
	Role                string     `gorm:"size:20;default:user;index"`
	Enabled             bool       `gorm:"default:true;index"`
	TrafficLimit        int64      `gorm:"default:0"`
	TrafficUsed         int64      `gorm:"default:0"`
	ExpiresAt           *time.Time `gorm:"index"`
	ForcePasswordChange bool       `gorm:"default:false"`
	// Portal fields
	EmailVerified    bool `gorm:"default:false"`
	EmailVerifiedAt  *time.Time
	TwoFactorEnabled bool `gorm:"default:false"`
	LastLoginAt      *time.Time
	LastLoginIP      string `gorm:"size:45"`
	TelegramID       string `gorm:"size:50;index"`
	// Commercial fields
	Balance   int64     `gorm:"default:0"` // User balance in cents
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// IsExpired checks if the user account has expired.
func (u *User) IsExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*u.ExpiresAt)
}

// IsTrafficExceeded checks if the user has exceeded their traffic limit.
func (u *User) IsTrafficExceeded() bool {
	if u.TrafficLimit <= 0 {
		return false // No limit
	}
	return u.TrafficUsed >= u.TrafficLimit
}

// CanAccess checks if the user can access the system.
func (u *User) CanAccess() bool {
	return u.Enabled && !u.IsExpired() && !u.IsTrafficExceeded()
}

// TableName returns the table name for User.
func (User) TableName() string {
	return "users"
}

// BeforeDelete cleans up user-owned runtime resources before the user row is removed.
func (u *User) BeforeDelete(tx *gorm.DB) error {
	if u == nil || u.ID == 0 {
		return nil
	}

	if tx.Migrator().HasTable("subscriptions") && tx.Migrator().HasTable("subscription_ip_access") {
		if err := tx.Exec(
			"DELETE FROM subscription_ip_access WHERE subscription_id IN (SELECT id FROM subscriptions WHERE user_id = ?)",
			u.ID,
		).Error; err != nil {
			return err
		}
	}

	for _, table := range []string{
		"proxies",
		"subscriptions",
		"user_node_assignments",
		"login_history",
		"active_ips",
		"ip_history",
		"trials",
		"subscription_pauses",
		"password_reset_tokens",
		"email_verification_tokens",
		"two_factor_secrets",
		"announcement_reads",
		"commercial_invite_codes",
	} {
		if !tx.Migrator().HasTable(table) {
			continue
		}
		if err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", table), u.ID).Error; err != nil {
			return err
		}
	}

	if tx.Migrator().HasTable("pending_downgrades") {
		if err := tx.Exec("DELETE FROM pending_downgrades WHERE user_id = ?", u.ID).Error; err != nil {
			return err
		}
	}

	if tx.Migrator().HasTable("tickets") && tx.Migrator().HasTable("ticket_messages") {
		if err := tx.Exec("DELETE FROM ticket_messages WHERE ticket_id IN (SELECT id FROM tickets WHERE user_id = ?)", u.ID).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable("ticket_messages") {
		if err := tx.Exec("DELETE FROM ticket_messages WHERE user_id = ?", u.ID).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable("tickets") {
		if err := tx.Exec("DELETE FROM tickets WHERE user_id = ?", u.ID).Error; err != nil {
			return err
		}
	}

	return nil
}

// Proxy represents a proxy configuration in the database.
type Proxy struct {
	ID        int64          `gorm:"primaryKey;autoIncrement"`
	UserID    int64          `gorm:"index;not null"`
	NodeID    *int64         `gorm:"index"` // 代理所属节点
	Name      string         `gorm:"size:100;not null"`
	Protocol  string         `gorm:"size:20;not null"`
	Port      int            `gorm:"not null;index"`
	Host      string         `gorm:"size:255"`
	Settings  map[string]any `gorm:"serializer:json"`
	Enabled   bool           `gorm:"default:true"`
	Remark    string         `gorm:"size:255"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`

	// Relations
	Node *Node `gorm:"foreignKey:NodeID"`
}

// TableName returns the table name for Proxy.
func (Proxy) TableName() string {
	return "proxies"
}

// Traffic represents traffic statistics in the database.
type Traffic struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	UserID     int64     `gorm:"index"`
	ProxyID    int64     `gorm:"index"`
	Upload     int64     `gorm:"default:0"`
	Download   int64     `gorm:"default:0"`
	RecordedAt time.Time `gorm:"index"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// TableName returns the table name for Traffic.
func (Traffic) TableName() string {
	return "traffic"
}

// LoginHistory represents a login attempt record in the database.
type LoginHistory struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	UserID    int64     `gorm:"index;not null"`
	IP        string    `gorm:"size:50"`
	UserAgent string    `gorm:"size:255"`
	Success   bool      `gorm:"default:false"`
	CreatedAt time.Time `gorm:"autoCreateTime;index"`
}

// TableName returns the table name for LoginHistory.
func (LoginHistory) TableName() string {
	return "login_history"
}

// UserRepository defines the interface for user data access.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*User, error)
	// Statistics methods
	Count(ctx context.Context) (int64, error)
	CountActive(ctx context.Context) (int64, error)
}

// ProxyRepository defines the interface for proxy data access.
type ProxyRepository interface {
	Create(ctx context.Context, proxy *Proxy) error
	GetByID(ctx context.Context, id int64) (*Proxy, error)
	Update(ctx context.Context, proxy *Proxy) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*Proxy, error)
	GetByProtocol(ctx context.Context, protocol string) ([]*Proxy, error)
	GetEnabled(ctx context.Context) ([]*Proxy, error)
	// User-related methods
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*Proxy, error)
	CountByUserID(ctx context.Context, userID int64) (int64, error)
	GetByPort(ctx context.Context, port int) (*Proxy, error)
	// Node-related methods
	GetByNodeID(ctx context.Context, nodeID int64) ([]*Proxy, error)
	// Batch operations
	EnableByUserID(ctx context.Context, userID int64) error
	DisableByUserID(ctx context.Context, userID int64) error
	DeleteByIDs(ctx context.Context, ids []int64) error
	// Statistics methods
	Count(ctx context.Context) (int64, error)
	CountEnabled(ctx context.Context) (int64, error)
	CountByProtocol(ctx context.Context) ([]*ProtocolCount, error)
}

// ProtocolCount represents proxy count by protocol.
type ProtocolCount struct {
	Protocol string
	Count    int64
}

// TrafficRepository defines the interface for traffic data access.
type TrafficRepository interface {
	Create(ctx context.Context, traffic *Traffic) error
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*Traffic, error)
	GetByProxyID(ctx context.Context, proxyID int64, limit, offset int) ([]*Traffic, error)
	GetByDateRange(ctx context.Context, start, end time.Time) ([]*Traffic, error)
	GetTotalByUser(ctx context.Context, userID int64) (upload, download int64, err error)
	GetTotalByProxy(ctx context.Context, proxyID int64) (upload, download int64, err error)
	// Statistics methods
	GetTotalTraffic(ctx context.Context) (upload, download int64, err error)
	GetTotalTrafficByPeriod(ctx context.Context, start, end time.Time) (upload, download int64, err error)
	GetTrafficByProtocol(ctx context.Context, start, end time.Time) ([]*ProtocolTrafficStats, error)
	GetTrafficByUser(ctx context.Context, start, end time.Time, limit int) ([]*UserTrafficStats, error)
	GetTrafficTimeline(ctx context.Context, start, end time.Time, interval string) ([]*TrafficTimelinePoint, error)
	GetTrafficTimelineByUser(ctx context.Context, userID int64, start, end time.Time, interval string) ([]*TrafficTimelinePoint, error)
}

// ProtocolTrafficStats represents traffic statistics by protocol.
type ProtocolTrafficStats struct {
	Protocol string
	Count    int64
	Upload   int64
	Download int64
}

// UserTrafficStats represents traffic statistics by user.
type UserTrafficStats struct {
	UserID       int64
	Username     string
	Email        string
	Upload       int64
	Download     int64
	ProxyCount   int64
	TrafficLimit int64
	LastActive   *time.Time
}

// TrafficTimelinePoint represents a point in the traffic timeline.
type TrafficTimelinePoint struct {
	Time     time.Time
	Upload   int64
	Download int64
}

// LoginHistoryRepository defines the interface for login history data access.
type LoginHistoryRepository interface {
	Create(ctx context.Context, history *LoginHistory) error
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*LoginHistory, error)
	DeleteByUserID(ctx context.Context, userID int64) error
	Count(ctx context.Context, userID int64) (int64, error)
}

// AuditLog represents an audit log entry in the database.
type AuditLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	UserID       *int64    `gorm:"index"`
	Username     string    `gorm:"size:50"`
	Action       string    `gorm:"size:50;not null;index"`
	ResourceType string    `gorm:"size:50;not null;index"`
	ResourceID   string    `gorm:"size:100"`
	Details      string    `gorm:"type:text"`
	IPAddress    string    `gorm:"size:50"`
	UserAgent    string    `gorm:"size:255"`
	RequestID    string    `gorm:"size:100;index"`
	Status       string    `gorm:"size:20;default:success"`
	CreatedAt    time.Time `gorm:"autoCreateTime;index"`
}

// TableName returns the table name for AuditLog.
func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditLogRepository defines the interface for audit log data access.
type AuditLogRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	List(ctx context.Context, limit, offset int) ([]*AuditLog, error)
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*AuditLog, error)
	GetByAction(ctx context.Context, action string, limit, offset int) ([]*AuditLog, error)
	GetByResourceType(ctx context.Context, resourceType string, limit, offset int) ([]*AuditLog, error)
	GetByDateRange(ctx context.Context, start, end time.Time, limit, offset int) ([]*AuditLog, error)
	Count(ctx context.Context) (int64, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// Repositories holds all repository instances.
type Repositories struct {
	db                    *gorm.DB
	User                  UserRepository
	Proxy                 ProxyRepository
	Traffic               TrafficRepository
	LoginHistory          LoginHistoryRepository
	Role                  RoleRepository
	Settings              SettingsRepository
	AuditLog              AuditLogRepository
	Log                   LogRepository
	Subscription          SubscriptionRepository
	Ticket                TicketRepository
	Announcement          AnnouncementRepository
	HelpArticle           HelpArticleRepository
	AuthToken             AuthTokenRepository
	Certificate           CertificateRepository
	CertificateDeployment CertificateDeploymentRepository
	// Commercial System repositories
	Plan            PlanRepository
	Order           OrderRepository
	Balance         BalanceRepository
	BalanceRecharge BalanceRechargeOrderRepository
	Coupon          CouponRepository
	Invite          InviteRepository
	Invoice         InvoiceRepository
	Trial           TrialRepository
	PlanChange      PlanChangeRepository
	ExchangeRate    ExchangeRateRepository
	PlanPrice       PlanPriceRepository
	Pause           PauseRepository
	GiftCard        GiftCardRepository
	// Multi-Server Management repositories
	Node               NodeRepository
	NodeGroup          NodeGroupRepository
	HealthCheck        HealthCheckRepository
	UserNodeAssignment UserNodeAssignmentRepository
	NodeTraffic        NodeTrafficRepository
}

// DB returns the underlying database connection.
func (r *Repositories) DB() *gorm.DB {
	return r.db
}

// NewRepositories creates all repository instances.
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		db:           db,
		User:         NewUserRepository(db),
		Proxy:        NewProxyRepository(db),
		Traffic:      NewTrafficRepository(db),
		LoginHistory: NewLoginHistoryRepository(db),
		Role:         NewRoleRepository(db),
		Settings:     NewSettingsRepository(db),
		AuditLog:     NewAuditLogRepository(db),
		Log:          NewLogRepository(db),
		Subscription: NewSubscriptionRepository(db),
		Ticket:       NewTicketRepository(db),
		Announcement: NewAnnouncementRepository(db),
		HelpArticle:  NewHelpArticleRepository(db),
		AuthToken:    NewAuthTokenRepository(db),
		// Commercial System repositories
		Plan:            NewPlanRepository(db),
		Order:           NewOrderRepository(db),
		Balance:         NewBalanceRepository(db),
		BalanceRecharge: NewBalanceRechargeOrderRepository(db),
		Coupon:          NewCouponRepository(db),
		Invite:          NewInviteRepository(db),
		Invoice:         NewInvoiceRepository(db),
		Trial:           NewTrialRepository(db),
		PlanChange:      NewPlanChangeRepository(db),
		ExchangeRate:    NewExchangeRateRepository(db),
		PlanPrice:       NewPlanPriceRepository(db),
		Pause:           NewPauseRepository(db),
		GiftCard:        NewGiftCardRepository(db),
		// Multi-Server Management repositories
		Node:                  NewNodeRepository(db),
		NodeGroup:             NewNodeGroupRepository(db),
		HealthCheck:           NewHealthCheckRepository(db),
		UserNodeAssignment:    NewUserNodeAssignmentRepository(db),
		NodeTraffic:           NewNodeTrafficRepository(db),
		Certificate:           NewCertificateRepository(db),
		CertificateDeployment: NewCertificateDeploymentRepository(db),
	}
}
