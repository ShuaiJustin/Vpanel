package database

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/ip"
)

// Config holds database configuration.
type Config struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// Health check and retry settings
	HealthCheckInterval time.Duration
	MaxRetries          int
	RetryInterval       time.Duration
	SlowQueryThreshold  time.Duration
}

// DefaultConfig returns default database configuration.
func DefaultConfig() Config {
	return Config{
		Driver:              "sqlite",
		MaxOpenConns:        25,
		MaxIdleConns:        5,
		ConnMaxLifetime:     5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		MaxRetries:          3,
		RetryInterval:       time.Second,
		SlowQueryThreshold:  200 * time.Millisecond,
	}
}

// Database wraps the GORM database connection.
type Database struct {
	db     *gorm.DB
	config *Config
	mu     sync.RWMutex

	// Health check
	healthy   bool
	lastCheck time.Time
	stopCh    chan struct{}
}

// New creates a new database connection.
func New(cfg *Config) (*Database, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "sqlite", "sqlite3", "":
		dialector = sqlite.Open(cfg.DSN)
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.DSN)
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s (supported: sqlite, postgres, mysql)", cfg.Driver)
	}

	// Set default values
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = time.Second
	}
	if cfg.SlowQueryThreshold <= 0 {
		cfg.SlowQueryThreshold = 200 * time.Millisecond
	}

	// Create slow query logger
	slowLogger := newSlowQueryLogger(cfg.SlowQueryThreshold)

	gormConfig := &gorm.Config{
		Logger: slowLogger,
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying DB: %w", err)
	}

	// Configure connection pool
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	database := &Database{
		db:      db,
		config:  cfg,
		healthy: true,
		stopCh:  make(chan struct{}),
	}

	// Start health check goroutine
	go database.healthCheckLoop()

	return database, nil
}

// DB returns the underlying GORM database.
func (d *Database) DB() *gorm.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db
}

// Close closes the database connection.
func (d *Database) Close() error {
	// Stop health check
	close(d.stopCh)

	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// AutoMigrate runs database migrations.
func (d *Database) AutoMigrate() error {
	if err := d.ensureTrafficTablesSupportSharedUsers(context.Background()); err != nil {
		return err
	}

	// Only run GORM auto migrations
	// SQL migrations are disabled for PostgreSQL compatibility
	if err := d.db.AutoMigrate(
		// Core models
		&repository.User{},
		&repository.Proxy{},
		&repository.Traffic{},
		&repository.LoginHistory{},
		&repository.Role{},
		&repository.AuditLog{},
		&repository.Setting{},
		&repository.Log{},
		&repository.Subscription{},
		// Portal models
		&repository.Ticket{},
		&repository.TicketMessage{},
		&repository.Announcement{},
		&repository.AnnouncementRead{},
		&repository.HelpArticle{},
		// Auth token models
		&repository.PasswordResetToken{},
		&repository.EmailVerificationToken{},
		&repository.InviteCode{},
		&repository.TwoFactorSecret{},
		// Commercial System models
		&repository.CommercialPlan{},
		&repository.Order{},
		&repository.BalanceTransaction{},
		&repository.BalanceRechargeOrder{},
		&repository.Coupon{},
		&repository.CouponUsage{},
		&repository.CommercialInviteCode{},
		&repository.Referral{},
		&repository.Commission{},
		&repository.Invoice{},
		&repository.Trial{},
		&repository.PendingDowngrade{},
		&repository.ExchangeRate{},
		&repository.PlanPrice{},
		&repository.SubscriptionPause{},
		&repository.GiftCard{},
		// Multi-Server Management models
		&repository.Node{},
		&repository.NodeGroup{},
		&repository.NodeGroupMember{},
		&repository.HealthCheck{},
		&repository.UserNodeAssignment{},
		&repository.NodeTraffic{},
		// Certificate Management models
		&repository.Certificate{},
		&repository.CertificateDeployment{},
		// IP Restriction models
		&ip.IPWhitelist{},
		&ip.IPBlacklist{},
		&ip.ActiveIP{},
		&ip.IPHistory{},
		&ip.SubscriptionIPAccess{},
		&ip.GeoCache{},
		&ip.FailedAttempt{},
	); err != nil {
		return err
	}

	if err := d.normalizeUserEmails(context.Background()); err != nil {
		return err
	}

	if err := d.ensureUserEmailUniqueIndex(context.Background()); err != nil {
		return err
	}

	return nil
}

func (d *Database) normalizeUserEmails(ctx context.Context) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE users SET email = NULL WHERE email IS NOT NULL AND TRIM(email) = ''`).Error; err != nil {
			return fmt.Errorf("clear blank user emails: %w", err)
		}

		if err := tx.Exec(`UPDATE users SET email = LOWER(TRIM(email)) WHERE email IS NOT NULL AND email <> LOWER(TRIM(email))`).Error; err != nil {
			return fmt.Errorf("normalize user emails: %w", err)
		}

		return nil
	})
}

func (d *Database) ensureUserEmailUniqueIndex(ctx context.Context) error {
	duplicates, err := d.findDuplicateNormalizedUserEmails(ctx)
	if err != nil {
		return err
	}
	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate user emails prevent unique email index: %s", strings.Join(duplicates, "; "))
	}

	switch d.db.Dialector.Name() {
	case "sqlite", "sqlite3":
		return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`DROP INDEX IF EXISTS idx_users_email`).Error; err != nil {
				return fmt.Errorf("drop legacy user email index: %w", err)
			}
			if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users (LOWER(TRIM(email))) WHERE email IS NOT NULL AND TRIM(email) <> ''`).Error; err != nil {
				return fmt.Errorf("create unique user email index: %w", err)
			}
			return nil
		})
	case "postgres", "postgresql":
		return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`DROP INDEX IF EXISTS idx_users_email`).Error; err != nil {
				return fmt.Errorf("drop legacy user email index: %w", err)
			}
			if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users ((LOWER(TRIM(email)))) WHERE email IS NOT NULL AND TRIM(email) <> ''`).Error; err != nil {
				return fmt.Errorf("create unique user email index: %w", err)
			}
			return nil
		})
	case "mysql":
		return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			migrator := tx.Migrator()
			if migrator.HasIndex(&repository.User{}, "idx_users_email") {
				if err := migrator.DropIndex(&repository.User{}, "idx_users_email"); err != nil {
					return fmt.Errorf("drop legacy user email index: %w", err)
				}
			}
			if !migrator.HasIndex(&repository.User{}, "idx_users_email_unique") {
				if err := tx.Exec(`CREATE UNIQUE INDEX idx_users_email_unique ON users (email)`).Error; err != nil {
					return fmt.Errorf("create unique user email index: %w", err)
				}
			}
			return nil
		})
	default:
		return nil
	}
}

func (d *Database) findDuplicateNormalizedUserEmails(ctx context.Context) ([]string, error) {
	type duplicateEmailRow struct {
		NormalizedEmail string
		Count           int64
	}

	rows := make([]duplicateEmailRow, 0)
	if err := d.db.WithContext(ctx).
		Raw(`SELECT LOWER(TRIM(email)) AS normalized_email, COUNT(*) AS count
			FROM users
			WHERE email IS NOT NULL AND TRIM(email) <> ''
			GROUP BY LOWER(TRIM(email))
			HAVING COUNT(*) > 1`).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("scan duplicate user emails: %w", err)
	}

	duplicates := make([]string, 0, len(rows))
	for _, row := range rows {
		duplicates = append(duplicates, fmt.Sprintf("%s (%d)", row.NormalizedEmail, row.Count))
	}

	return duplicates, nil
}

func (d *Database) ensureTrafficTablesSupportSharedUsers(ctx context.Context) error {
	for _, table := range []string{"traffic", "node_traffic"} {
		if err := d.ensureSharedTrafficTableWithoutForeignKeys(ctx, table); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) ensureSharedTrafficTableWithoutForeignKeys(ctx context.Context, tableName string) error {
	if d == nil || d.db == nil {
		return nil
	}
	if !d.db.Migrator().HasTable(tableName) {
		return nil
	}

	switch d.db.Dialector.Name() {
	case "sqlite", "sqlite3":
		return d.ensureSQLiteSharedTrafficTableWithoutForeignKeys(ctx, tableName)
	case "postgres", "postgresql":
		return d.dropPostgresTableForeignKeys(ctx, tableName)
	case "mysql":
		return d.dropMySQLTableForeignKeys(ctx, tableName)
	default:
		return nil
	}
}

func (d *Database) ensureSQLiteSharedTrafficTableWithoutForeignKeys(ctx context.Context, tableName string) error {
	var createSQL string
	if err := d.db.WithContext(ctx).
		Raw(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).
		Row().
		Scan(&createSQL); err != nil {
		return fmt.Errorf("inspect sqlite %s schema: %w", tableName, err)
	}

	normalizedSQL := strings.ToLower(createSQL)
	if !strings.Contains(normalizedSQL, "foreign key") {
		return nil
	}

	var createReplacement string
	var indexStatements []string
	switch tableName {
	case "traffic":
		createReplacement = `
			CREATE TABLE traffic__shared_user_tmp (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				proxy_id INTEGER,
				upload INTEGER DEFAULT 0,
				download INTEGER DEFAULT 0,
				recorded_at TIMESTAMP NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
		indexStatements = []string{
			`CREATE INDEX IF NOT EXISTS idx_traffic_user_id ON traffic(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_traffic_proxy_id ON traffic(proxy_id)`,
			`CREATE INDEX IF NOT EXISTS idx_traffic_recorded_at ON traffic(recorded_at)`,
		}
	case "node_traffic":
		createReplacement = `
			CREATE TABLE node_traffic__shared_user_tmp (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id INTEGER NOT NULL,
				user_id INTEGER NOT NULL,
				proxy_id INTEGER,
				upload INTEGER DEFAULT 0,
				download INTEGER DEFAULT 0,
				recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`
		indexStatements = []string{
			`CREATE INDEX IF NOT EXISTS idx_node_traffic_node ON node_traffic(node_id)`,
			`CREATE INDEX IF NOT EXISTS idx_node_traffic_user ON node_traffic(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_node_traffic_proxy ON node_traffic(proxy_id)`,
			`CREATE INDEX IF NOT EXISTS idx_node_traffic_recorded ON node_traffic(recorded_at)`,
			`CREATE INDEX IF NOT EXISTS idx_node_traffic_node_recorded ON node_traffic(node_id, recorded_at)`,
			`CREATE INDEX IF NOT EXISTS idx_node_traffic_user_recorded ON node_traffic(user_id, recorded_at)`,
		}
	default:
		return nil
	}

	tempTableName := tableName + "__shared_user_tmp"

	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`PRAGMA foreign_keys = OFF`).Error; err != nil {
			return fmt.Errorf("disable sqlite foreign keys: %w", err)
		}
		if err := tx.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tempTableName)).Error; err != nil {
			return fmt.Errorf("drop stale sqlite %s temp table: %w", tableName, err)
		}
		if err := tx.Exec(createReplacement).Error; err != nil {
			return fmt.Errorf("create sqlite %s temp table: %w", tableName, err)
		}
		if err := tx.Exec(fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`, tempTableName, tableName)).Error; err != nil {
			return fmt.Errorf("copy sqlite %s rows: %w", tableName, err)
		}
		if err := tx.Exec(fmt.Sprintf(`DROP TABLE %s`, tableName)).Error; err != nil {
			return fmt.Errorf("drop sqlite %s: %w", tableName, err)
		}
		if err := tx.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, tempTableName, tableName)).Error; err != nil {
			return fmt.Errorf("rename sqlite %s temp table: %w", tableName, err)
		}
		for _, stmt := range indexStatements {
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("recreate sqlite %s index: %w", tableName, err)
			}
		}
		if err := tx.Exec(`PRAGMA foreign_keys = ON`).Error; err != nil {
			return fmt.Errorf("enable sqlite foreign keys: %w", err)
		}
		return nil
	})
}

func (d *Database) dropPostgresTableForeignKeys(ctx context.Context, tableName string) error {
	type constraintRow struct {
		ConstraintName string `gorm:"column:constraint_name"`
	}

	rows := make([]constraintRow, 0)
	if err := d.db.WithContext(ctx).Raw(`
		SELECT tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = CURRENT_SCHEMA()
			AND tc.table_name = ?
	`, tableName).Scan(&rows).Error; err != nil {
		return fmt.Errorf("inspect postgres %s foreign keys: %w", tableName, err)
	}

	for _, row := range rows {
		if row.ConstraintName == "" {
			continue
		}
		stmt := fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS "%s"`, tableName, row.ConstraintName)
		if err := d.db.WithContext(ctx).Exec(stmt).Error; err != nil {
			return fmt.Errorf("drop postgres %s foreign key %s: %w", tableName, row.ConstraintName, err)
		}
	}

	return nil
}

func (d *Database) dropMySQLTableForeignKeys(ctx context.Context, tableName string) error {
	type constraintRow struct {
		ConstraintName string `gorm:"column:constraint_name"`
	}

	rows := make([]constraintRow, 0)
	if err := d.db.WithContext(ctx).Raw(`
		SELECT constraint_name
		FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE()
			AND table_name = ?
			AND referenced_table_name IS NOT NULL
	`, tableName).Scan(&rows).Error; err != nil {
		return fmt.Errorf("inspect mysql %s foreign keys: %w", tableName, err)
	}

	for _, row := range rows {
		if row.ConstraintName == "" {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY `%s`", tableName, row.ConstraintName)
		if err := d.db.WithContext(ctx).Exec(stmt).Error; err != nil {
			return fmt.Errorf("drop mysql %s foreign key %s: %w", tableName, row.ConstraintName, err)
		}
	}

	return nil
}

// Ping checks the database connection.
func (d *Database) Ping() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// IsHealthy returns the current health status.
func (d *Database) IsHealthy() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.healthy
}

// LastHealthCheck returns the time of the last health check.
func (d *Database) LastHealthCheck() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastCheck
}

// healthCheckLoop periodically checks database health.
func (d *Database) healthCheckLoop() {
	ticker := time.NewTicker(d.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.checkHealth()
		case <-d.stopCh:
			return
		}
	}
}

// checkHealth performs a health check and attempts reconnection if needed.
func (d *Database) checkHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.PingContext(ctx)

	d.mu.Lock()
	d.lastCheck = time.Now()
	if err != nil {
		d.healthy = false
		d.mu.Unlock()

		// Attempt reconnection
		d.reconnect()
	} else {
		d.healthy = true
		d.mu.Unlock()
	}
}

// PingContext checks the database connection with context.
func (d *Database) PingContext(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// reconnect attempts to reconnect to the database with exponential backoff.
func (d *Database) reconnect() {
	for i := 0; i < d.config.MaxRetries; i++ {
		// Exponential backoff
		backoff := d.config.RetryInterval * time.Duration(1<<uint(i))
		time.Sleep(backoff)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := d.PingContext(ctx)
		cancel()

		if err == nil {
			d.mu.Lock()
			d.healthy = true
			d.mu.Unlock()
			return
		}
	}
}

// WithRetry executes a database operation with retry logic.
func (d *Database) WithRetry(ctx context.Context, fn func(*gorm.DB) error) error {
	var lastErr error

	for i := 0; i <= d.config.MaxRetries; i++ {
		if i > 0 {
			// Exponential backoff
			backoff := d.config.RetryInterval * time.Duration(1<<uint(i-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := fn(d.db.WithContext(ctx))
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", d.config.MaxRetries, lastErr)
}

// isRetryableError checks if an error is retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	retryableErrors := []string{
		"database is locked",
		"connection refused",
		"connection reset",
		"broken pipe",
		"timeout",
		"deadlock",
	}

	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// HealthStatus represents database health status.
type HealthStatus struct {
	Healthy   bool      `json:"healthy"`
	LastCheck time.Time `json:"last_check"`
	Latency   string    `json:"latency,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Health returns detailed health status.
func (d *Database) Health(ctx context.Context) HealthStatus {
	start := time.Now()
	err := d.PingContext(ctx)
	latency := time.Since(start)

	status := HealthStatus{
		Healthy:   err == nil,
		LastCheck: time.Now(),
		Latency:   latency.String(),
	}

	if err != nil {
		status.Error = err.Error()
	}

	return status
}

// slowQueryLogger implements GORM logger interface for slow query logging.
type slowQueryLogger struct {
	threshold time.Duration
	logger.Interface
}

// newSlowQueryLogger creates a new slow query logger.
func newSlowQueryLogger(threshold time.Duration) *slowQueryLogger {
	return &slowQueryLogger{
		threshold: threshold,
		Interface: logger.Default.LogMode(logger.Silent),
	}
}

// LogMode implements logger.Interface.
func (l *slowQueryLogger) LogMode(level logger.LogLevel) logger.Interface {
	return l
}

// Info implements logger.Interface.
func (l *slowQueryLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	// Silent
}

// Warn implements logger.Interface.
func (l *slowQueryLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	// Silent
}

// Error implements logger.Interface.
func (l *slowQueryLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	// Log errors
	fmt.Printf("[DB ERROR] %s %v\n", msg, data)
}

// SlowQueryLog represents a slow query log entry.
type SlowQueryLog struct {
	SQL      string        `json:"sql"`
	Duration time.Duration `json:"duration"`
	Rows     int64         `json:"rows"`
	Time     time.Time     `json:"time"`
}

// slowQueryLogs stores recent slow queries for monitoring.
var (
	slowQueryLogs   []SlowQueryLog
	slowQueryLogsMu sync.RWMutex
	maxSlowQueries  = 100
)

// Trace implements logger.Interface for query tracing.
func (l *slowQueryLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	// Log slow queries
	if elapsed > l.threshold {
		log := SlowQueryLog{
			SQL:      sql,
			Duration: elapsed,
			Rows:     rows,
			Time:     begin,
		}

		// Store in memory for monitoring
		slowQueryLogsMu.Lock()
		slowQueryLogs = append(slowQueryLogs, log)
		if len(slowQueryLogs) > maxSlowQueries {
			slowQueryLogs = slowQueryLogs[1:]
		}
		slowQueryLogsMu.Unlock()

		// Print to stdout (can be captured by logging system)
		fmt.Printf("[SLOW QUERY] duration=%s rows=%d sql=%s\n", elapsed, rows, sql)
	}

	// Log errors
	if err != nil && err != gorm.ErrRecordNotFound {
		fmt.Printf("[DB ERROR] duration=%s err=%v sql=%s\n", elapsed, err, sql)
	}
}

// GetSlowQueries returns recent slow queries.
func GetSlowQueries() []SlowQueryLog {
	slowQueryLogsMu.RLock()
	defer slowQueryLogsMu.RUnlock()

	result := make([]SlowQueryLog, len(slowQueryLogs))
	copy(result, slowQueryLogs)
	return result
}

// ClearSlowQueries clears the slow query log.
func ClearSlowQueries() {
	slowQueryLogsMu.Lock()
	defer slowQueryLogsMu.Unlock()
	slowQueryLogs = nil
}

// SetSlowQueryThreshold updates the slow query threshold.
func (d *Database) SetSlowQueryThreshold(threshold time.Duration) {
	if l, ok := d.db.Logger.(*slowQueryLogger); ok {
		l.threshold = threshold
	}
}

// GetSlowQueryThreshold returns the current slow query threshold.
func (d *Database) GetSlowQueryThreshold() time.Duration {
	if l, ok := d.db.Logger.(*slowQueryLogger); ok {
		return l.threshold
	}
	return 0
}
