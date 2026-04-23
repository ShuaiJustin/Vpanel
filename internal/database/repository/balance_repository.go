// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Common balance repository errors.
var (
	// ErrInsufficientBalance is returned by DecrementBalance when the row-level
	// balance guard rejects the update (0 rows affected). Prevents the service
	// layer from treating a losing race as success and writing a bogus audit row.
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// BalanceTransaction represents a balance transaction in the database.
type BalanceTransaction struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	UserID      int64     `gorm:"index;not null"`
	Type        string    `gorm:"size:32;not null"`
	Amount      int64     `gorm:"not null"`
	Balance     int64     `gorm:"not null"`
	OrderID     *int64    `gorm:"index"`
	Description string    `gorm:"size:256"`
	Operator    string    `gorm:"size:64"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index"`

	User  *User  `gorm:"foreignKey:UserID"`
	Order *Order `gorm:"foreignKey:OrderID"`
}

// TableName returns the table name for BalanceTransaction.
func (BalanceTransaction) TableName() string {
	return "balance_transactions"
}

// Balance transaction type constants
const (
	BalanceTxTypeRecharge   = "recharge"
	BalanceTxTypePurchase   = "purchase"
	BalanceTxTypeRefund     = "refund"
	BalanceTxTypeCommission = "commission"
	BalanceTxTypeAdjustment = "adjustment"
)

// BalanceFilter defines filter options for listing transactions.
type BalanceFilter struct {
	UserID    *int64
	Type      string
	StartDate *time.Time
	EndDate   *time.Time
}

// BalanceRepository defines the interface for balance data access.
type BalanceRepository interface {
	// User balance operations
	GetBalance(ctx context.Context, userID int64) (int64, error)
	UpdateBalance(ctx context.Context, userID int64, amount int64) error
	IncrementBalance(ctx context.Context, userID int64, amount int64) error
	DecrementBalance(ctx context.Context, userID int64, amount int64) error

	// Atomic balance mutations
	//
	// CreditAtomic adds `amount` to the user's balance AND inserts the supplied
	// audit transaction in a single DB transaction. Returns the authoritative
	// post-update balance read inside the same transaction so callers don't
	// have to estimate from a stale snapshot.
	//
	// DebitAtomic subtracts `amount` with a row-level guard in the same DB
	// transaction as the audit row. Returns ErrInsufficientBalance if the
	// guard rejects the update (no audit row is written on failure).
	CreditAtomic(ctx context.Context, userID int64, amount int64, tx *BalanceTransaction) (int64, error)
	DebitAtomic(ctx context.Context, userID int64, amount int64, tx *BalanceTransaction) (int64, error)

	// Transaction operations
	CreateTransaction(ctx context.Context, tx *BalanceTransaction) error
	GetTransactionByID(ctx context.Context, id int64) (*BalanceTransaction, error)
	ListTransactions(ctx context.Context, filter BalanceFilter, limit, offset int) ([]*BalanceTransaction, int64, error)
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*BalanceTransaction, int64, error)

	// Statistics
	GetTotalRecharge(ctx context.Context, userID int64) (int64, error)
	GetTotalSpent(ctx context.Context, userID int64) (int64, error)
	GetTotalCommission(ctx context.Context, userID int64) (int64, error)
}

// balanceRepository implements BalanceRepository.
type balanceRepository struct {
	db *gorm.DB
}

// NewBalanceRepository creates a new balance repository.
func NewBalanceRepository(db *gorm.DB) BalanceRepository {
	return &balanceRepository{db: db}
}

// GetBalance retrieves the balance for a user.
func (r *balanceRepository) GetBalance(ctx context.Context, userID int64) (int64, error) {
	var balance int64
	err := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Select("balance").
		Scan(&balance).Error
	return balance, err
}

// UpdateBalance sets the balance for a user.
func (r *balanceRepository) UpdateBalance(ctx context.Context, userID int64, amount int64) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Update("balance", amount).Error
}

// IncrementBalance adds to the balance for a user.
func (r *balanceRepository) IncrementBalance(ctx context.Context, userID int64, amount int64) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Update("balance", gorm.Expr("balance + ?", amount)).Error
}

// DecrementBalance subtracts from the balance for a user.
// Row-level guard keeps the subtract atomic at the DB, so concurrent deductions
// can never drive the balance negative. Returns ErrInsufficientBalance when
// the guard rejects the update.
func (r *balanceRepository) DecrementBalance(ctx context.Context, userID int64, amount int64) error {
	result := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ? AND balance >= ?", userID, amount).
		Update("balance", gorm.Expr("balance - ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInsufficientBalance
	}
	return nil
}

// CreateTransaction creates a new balance transaction.
func (r *balanceRepository) CreateTransaction(ctx context.Context, tx *BalanceTransaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

// CreditAtomic adds amount to the user's balance and inserts the audit row in
// a single DB transaction. Returns the authoritative post-update balance read
// inside the same transaction, so callers don't risk logging a snapshot that
// was stale by the time it was written.
func (r *balanceRepository) CreditAtomic(ctx context.Context, userID int64, amount int64, tx *BalanceTransaction) (int64, error) {
	if tx == nil {
		return 0, errors.New("nil balance transaction")
	}
	if amount <= 0 {
		return 0, errors.New("credit amount must be positive")
	}

	var newBalance int64
	err := r.db.WithContext(ctx).Transaction(func(dbTx *gorm.DB) error {
		if err := dbTx.Model(&User{}).
			Where("id = ?", userID).
			Update("balance", gorm.Expr("balance + ?", amount)).Error; err != nil {
			return err
		}

		if err := dbTx.Model(&User{}).
			Where("id = ?", userID).
			Select("balance").
			Scan(&newBalance).Error; err != nil {
			return err
		}

		// Always persist the definitive post-update balance. Callers can pass
		// a stale guess in tx.Balance; the repo overrides it here.
		tx.UserID = userID
		tx.Amount = amount
		tx.Balance = newBalance
		return dbTx.Create(tx).Error
	})
	if err != nil {
		return 0, err
	}
	return newBalance, nil
}

// DebitAtomic subtracts amount from the user's balance (row-level guard) and
// inserts the audit row in a single DB transaction. Returns
// ErrInsufficientBalance if the guard rejects the update; in that case no
// audit row is written so the transaction log never records a debit that
// didn't actually happen.
func (r *balanceRepository) DebitAtomic(ctx context.Context, userID int64, amount int64, tx *BalanceTransaction) (int64, error) {
	if tx == nil {
		return 0, errors.New("nil balance transaction")
	}
	if amount <= 0 {
		return 0, errors.New("debit amount must be positive")
	}

	var newBalance int64
	err := r.db.WithContext(ctx).Transaction(func(dbTx *gorm.DB) error {
		result := dbTx.Model(&User{}).
			Where("id = ? AND balance >= ?", userID, amount).
			Update("balance", gorm.Expr("balance - ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrInsufficientBalance
		}

		if err := dbTx.Model(&User{}).
			Where("id = ?", userID).
			Select("balance").
			Scan(&newBalance).Error; err != nil {
			return err
		}

		tx.UserID = userID
		tx.Amount = -amount
		tx.Balance = newBalance
		return dbTx.Create(tx).Error
	})
	if err != nil {
		return 0, err
	}
	return newBalance, nil
}

// GetTransactionByID retrieves a transaction by ID.
func (r *balanceRepository) GetTransactionByID(ctx context.Context, id int64) (*BalanceTransaction, error) {
	var tx BalanceTransaction
	err := r.db.WithContext(ctx).Preload("User").Preload("Order").First(&tx, id).Error
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

// ListTransactions lists transactions with filter and pagination.
func (r *balanceRepository) ListTransactions(ctx context.Context, filter BalanceFilter, limit, offset int) ([]*BalanceTransaction, int64, error) {
	var transactions []*BalanceTransaction
	var total int64

	query := r.db.WithContext(ctx).Model(&BalanceTransaction{})

	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", *filter.EndDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("User").
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error
	return transactions, total, err
}

// ListByUser lists transactions for a specific user.
func (r *balanceRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*BalanceTransaction, int64, error) {
	var transactions []*BalanceTransaction
	var total int64

	query := r.db.WithContext(ctx).Model(&BalanceTransaction{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error
	return transactions, total, err
}

// GetTotalRecharge returns total recharge amount for a user.
func (r *balanceRepository) GetTotalRecharge(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&BalanceTransaction{}).
		Where("user_id = ? AND type = ?", userID, BalanceTxTypeRecharge).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	return total, err
}

// GetTotalSpent returns total spent amount for a user.
func (r *balanceRepository) GetTotalSpent(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&BalanceTransaction{}).
		Where("user_id = ? AND type = ?", userID, BalanceTxTypePurchase).
		Select("COALESCE(SUM(ABS(amount)), 0)").Scan(&total).Error
	return total, err
}

// GetTotalCommission returns total commission earned for a user.
func (r *balanceRepository) GetTotalCommission(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&BalanceTransaction{}).
		Where("user_id = ? AND type = ?", userID, BalanceTxTypeCommission).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	return total, err
}
