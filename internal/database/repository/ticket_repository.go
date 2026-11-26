// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// Ticket represents a support ticket in the database.
type Ticket struct {
	ID        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64      `gorm:"index;not null" json:"user_id"`
	Subject   string     `gorm:"size:256;not null" json:"subject"`
	Category  string     `gorm:"size:64;default:other;index" json:"category"`
	Status    string     `gorm:"size:32;default:open;index" json:"status"`
	Priority  string     `gorm:"size:32;default:normal" json:"priority"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at"`

	// Relations
	User     *User           `gorm:"foreignKey:UserID" json:"-"`
	Messages []TicketMessage `gorm:"foreignKey:TicketID" json:"-"`
}

// TableName returns the table name for Ticket.
func (Ticket) TableName() string {
	return "tickets"
}

// Ticket status constants
const (
	TicketStatusOpen     = "open"
	TicketStatusPending  = "pending"
	TicketStatusWaiting  = "waiting"
	TicketStatusAnswered = "answered"
	TicketStatusResolved = "resolved"
	TicketStatusClosed   = "closed"
)

// Ticket priority constants
const (
	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"
)

// TicketMessage represents a message in a ticket conversation.
type TicketMessage struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TicketID    int64     `gorm:"index;not null" json:"ticket_id"`
	UserID      *int64    `gorm:"index" json:"-"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	IsAdmin     bool      `gorm:"default:false" json:"is_admin"`
	Attachments string    `gorm:"type:text" json:"attachments,omitempty"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relations
	Ticket *Ticket `gorm:"foreignKey:TicketID" json:"-"`
	User   *User   `gorm:"foreignKey:UserID" json:"-"`
}

// TableName returns the table name for TicketMessage.
func (TicketMessage) TableName() string {
	return "ticket_messages"
}

// TicketFilter represents filter options for listing tickets.
type TicketFilter struct {
	UserID   *int64
	Status   *string
	Priority *string
	Limit    int
	Offset   int
}

// TicketRepository defines the interface for ticket data access.
type TicketRepository interface {
	// Create creates a new ticket.
	Create(ctx context.Context, ticket *Ticket) error
	CreateWithMessage(ctx context.Context, ticket *Ticket, message *TicketMessage) error

	// GetByID retrieves a ticket by its ID.
	GetByID(ctx context.Context, id int64) (*Ticket, error)

	// GetByIDWithMessages retrieves a ticket with all its messages.
	GetByIDWithMessages(ctx context.Context, id int64) (*Ticket, error)

	// Update updates an existing ticket.
	Update(ctx context.Context, ticket *Ticket) error

	// Delete deletes a ticket by ID.
	Delete(ctx context.Context, id int64) error

	// ListByUser retrieves tickets for a specific user.
	ListByUser(ctx context.Context, userID int64, filter *TicketFilter) ([]*Ticket, int64, error)

	// ListAll retrieves all tickets with optional filtering.
	ListAll(ctx context.Context, filter *TicketFilter) ([]*Ticket, int64, error)

	// UpdateStatus updates the status of a ticket.
	UpdateStatus(ctx context.Context, id int64, status string) error

	// Close closes a ticket.
	Close(ctx context.Context, id int64) error

	// AddMessage adds a message to a ticket.
	AddMessage(ctx context.Context, message *TicketMessage) error
	AddReply(ctx context.Context, message *TicketMessage) error

	// GetMessages retrieves all messages for a ticket.
	GetMessages(ctx context.Context, ticketID int64) ([]*TicketMessage, error)

	// CountByUser counts tickets for a specific user.
	CountByUser(ctx context.Context, userID int64) (int64, error)

	// CountByStatus counts tickets by status.
	CountByStatus(ctx context.Context) (map[string]int64, error)
}

// ticketRepository implements TicketRepository.
type ticketRepository struct {
	db *gorm.DB
}

// NewTicketRepository creates a new ticket repository.
func NewTicketRepository(db *gorm.DB) TicketRepository {
	return &ticketRepository{db: db}
}

// Create creates a new ticket.
func (r *ticketRepository) Create(ctx context.Context, ticket *Ticket) error {
	result := r.db.WithContext(ctx).Create(ticket)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create ticket", result.Error)
	}
	return nil
}

func (r *ticketRepository) CreateWithMessage(ctx context.Context, ticket *Ticket, message *TicketMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := &ticketRepository{db: tx}
		if err := repo.Create(ctx, ticket); err != nil {
			return err
		}
		message.TicketID = ticket.ID
		return repo.AddMessage(ctx, message)
	})
}

// GetByID retrieves a ticket by its ID.
func (r *ticketRepository) GetByID(ctx context.Context, id int64) (*Ticket, error) {
	var ticket Ticket
	result := r.db.WithContext(ctx).First(&ticket, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("ticket", id)
		}
		return nil, errors.NewDatabaseError("failed to get ticket", result.Error)
	}
	return &ticket, nil
}

// GetByIDWithMessages retrieves a ticket with all its messages.
func (r *ticketRepository) GetByIDWithMessages(ctx context.Context, id int64) (*Ticket, error) {
	var ticket Ticket
	result := r.db.WithContext(ctx).Preload("Messages", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}).Preload("Messages.User").First(&ticket, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("ticket", id)
		}
		return nil, errors.NewDatabaseError("failed to get ticket with messages", result.Error)
	}
	return &ticket, nil
}

// Update updates an existing ticket.
func (r *ticketRepository) Update(ctx context.Context, ticket *Ticket) error {
	result := r.db.WithContext(ctx).Save(ticket)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update ticket", result.Error)
	}
	return nil
}

// Delete deletes a ticket by ID.
func (r *ticketRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&Ticket{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete ticket", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("ticket", id)
	}
	return nil
}

// ListByUser retrieves tickets for a specific user.
func (r *ticketRepository) ListByUser(ctx context.Context, userID int64, filter *TicketFilter) ([]*Ticket, int64, error) {
	if filter == nil {
		filter = &TicketFilter{}
	}
	filter.UserID = &userID
	return r.ListAll(ctx, filter)
}

// ListAll retrieves all tickets with optional filtering.
func (r *ticketRepository) ListAll(ctx context.Context, filter *TicketFilter) ([]*Ticket, int64, error) {
	var tickets []*Ticket
	var total int64

	query := r.db.WithContext(ctx).Model(&Ticket{})

	// Apply filters
	if filter != nil {
		if filter.UserID != nil {
			query = query.Where("user_id = ?", *filter.UserID)
		}
		if filter.Status != nil {
			query = query.Where("status = ?", *filter.Status)
		}
		if filter.Priority != nil {
			query = query.Where("priority = ?", *filter.Priority)
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count tickets", err)
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
	if err := query.Order("updated_at DESC, id DESC").Find(&tickets).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to list tickets", err)
	}

	return tickets, total, nil
}

// UpdateStatus updates the status of a ticket.
func (r *ticketRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	result := r.db.WithContext(ctx).Model(&Ticket{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update ticket status", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("ticket", id)
	}
	return nil
}

// Close closes a ticket.
func (r *ticketRepository) Close(ctx context.Context, id int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&Ticket{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":    TicketStatusClosed,
			"closed_at": now,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to close ticket", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("ticket", id)
	}
	return nil
}

// AddMessage adds a message to a ticket.
func (r *ticketRepository) AddMessage(ctx context.Context, message *TicketMessage) error {
	result := r.db.WithContext(ctx).Create(message)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to add ticket message", result.Error)
	}
	return nil
}

// AddReply writes the message and updates the queue in the same transaction.
func (r *ticketRepository) AddReply(ctx context.Context, message *TicketMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		status := any(TicketStatusAnswered)
		if !message.IsAdmin {
			status = gorm.Expr("CASE WHEN status = ? THEN ? ELSE status END", TicketStatusAnswered, TicketStatusWaiting)
		}
		result := tx.Model(&Ticket{}).
			Where("id = ? AND status <> ?", message.TicketID, TicketStatusClosed).
			Updates(map[string]any{"status": status, "updated_at": time.Now()})
		if result.Error != nil {
			return errors.NewDatabaseError("failed to update ticket for reply", result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.NewValidationError("工单已关闭，无法回复", nil)
		}
		return (&ticketRepository{db: tx}).AddMessage(ctx, message)
	})
}

// GetMessages retrieves all messages for a ticket.
func (r *ticketRepository) GetMessages(ctx context.Context, ticketID int64) ([]*TicketMessage, error) {
	var messages []*TicketMessage
	result := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at ASC, id ASC").
		Find(&messages)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get ticket messages", result.Error)
	}
	return messages, nil
}

// CountByUser counts tickets for a specific user.
func (r *ticketRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&Ticket{}).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count user tickets", result.Error)
	}
	return count, nil
}

// CountByStatus counts tickets by status.
func (r *ticketRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	var results []statusCount

	err := r.db.WithContext(ctx).Model(&Ticket{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&results).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to count tickets by status", err)
	}

	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}
	return counts, nil
}
