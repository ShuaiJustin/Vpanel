package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"v/pkg/errors"
)

func setupTicketTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&User{}, &Ticket{}, &TicketMessage{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

func createTicketTestUser(t *testing.T, db *gorm.DB, username string) int64 {
	user := &User{
		Username:     username,
		PasswordHash: "hashedpassword",
		Email:        username + "@example.com",
		Role:         "user",
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user.ID
}

func TestTicketRepository_Create(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	ticket := &Ticket{
		UserID:   userID,
		Subject:  "Test Ticket",
		Status:   TicketStatusOpen,
		Priority: TicketPriorityNormal,
	}

	err := repo.Create(ctx, ticket)
	if err != nil {
		t.Fatalf("Failed to create ticket: %v", err)
	}

	if ticket.ID == 0 {
		t.Error("Expected ticket ID to be set after creation")
	}
}

func TestTicketRepository_GetByID(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	ticket := &Ticket{
		UserID:   userID,
		Subject:  "Test Ticket",
		Status:   TicketStatusOpen,
		Priority: TicketPriorityNormal,
	}
	repo.Create(ctx, ticket)

	found, err := repo.GetByID(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("Failed to get ticket: %v", err)
	}

	if found.Subject != ticket.Subject {
		t.Errorf("Expected subject %s, got %s", ticket.Subject, found.Subject)
	}

	// Test not found
	_, err = repo.GetByID(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestTicketRepository_UpdateStatus(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	ticket := &Ticket{
		UserID:   userID,
		Subject:  "Test Ticket",
		Status:   TicketStatusOpen,
		Priority: TicketPriorityNormal,
	}
	repo.Create(ctx, ticket)

	// Update status
	err := repo.UpdateStatus(ctx, ticket.ID, TicketStatusPending)
	if err != nil {
		t.Fatalf("Failed to update ticket status: %v", err)
	}

	// Verify update
	found, _ := repo.GetByID(ctx, ticket.ID)
	if found.Status != TicketStatusPending {
		t.Errorf("Expected status %s, got %s", TicketStatusPending, found.Status)
	}
}

func TestTicketRepository_Close(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	ticket := &Ticket{
		UserID:   userID,
		Subject:  "Test Ticket",
		Status:   TicketStatusOpen,
		Priority: TicketPriorityNormal,
	}
	repo.Create(ctx, ticket)

	// Close ticket
	err := repo.Close(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("Failed to close ticket: %v", err)
	}

	// Verify close
	found, _ := repo.GetByID(ctx, ticket.ID)
	if found.Status != TicketStatusClosed {
		t.Errorf("Expected status %s, got %s", TicketStatusClosed, found.Status)
	}
	if found.ClosedAt == nil {
		t.Error("Expected closed_at to be set")
	}
}

func TestTicketRepository_AddMessage(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	ticket := &Ticket{
		UserID:   userID,
		Subject:  "Test Ticket",
		Status:   TicketStatusOpen,
		Priority: TicketPriorityNormal,
	}
	repo.Create(ctx, ticket)

	// Add message
	message := &TicketMessage{
		TicketID: ticket.ID,
		UserID:   &userID,
		Content:  "This is a test message",
		IsAdmin:  false,
	}
	err := repo.AddMessage(ctx, message)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	if message.ID == 0 {
		t.Error("Expected message ID to be set after creation")
	}
}

func TestTicketRepository_GetMessages(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	ticket := &Ticket{
		UserID:   userID,
		Subject:  "Test Ticket",
		Status:   TicketStatusOpen,
		Priority: TicketPriorityNormal,
	}
	repo.Create(ctx, ticket)

	// Add multiple messages
	for i := 0; i < 3; i++ {
		message := &TicketMessage{
			TicketID: ticket.ID,
			UserID:   &userID,
			Content:  "Message " + string(rune('1'+i)),
			IsAdmin:  i%2 == 1,
		}
		repo.AddMessage(ctx, message)
	}

	// Get messages
	messages, err := repo.GetMessages(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}
}

func TestTicketRepository_ReplyUpdatesQueueAtomically(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()
	userID := createTicketTestUser(t, db, "ticket-owner")
	staffID := createTicketTestUser(t, db, "ticket-staff")
	ticket := &Ticket{UserID: userID, Subject: "Test thread"}
	require.NoError(t, repo.CreateWithMessage(ctx, ticket, &TicketMessage{UserID: &userID, Content: "Initial report"}))

	reply := &TicketMessage{TicketID: ticket.ID, UserID: &staffID, Content: "Staff reply", IsAdmin: true}
	require.NoError(t, repo.AddReply(ctx, reply))
	persisted, err := repo.GetByID(ctx, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, TicketStatusAnswered, persisted.Status)

	require.NoError(t, db.Exec(`CREATE TRIGGER fail_ticket_reply BEFORE INSERT ON ticket_messages BEGIN SELECT RAISE(ABORT, 'test write failure'); END`).Error)
	userReply := &TicketMessage{TicketID: ticket.ID, UserID: &userID, Content: "User follow-up"}
	require.Error(t, repo.AddReply(ctx, userReply))
	persisted, err = repo.GetByID(ctx, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, TicketStatusAnswered, persisted.Status)

	require.NoError(t, db.Exec("DROP TRIGGER fail_ticket_reply").Error)
	require.NoError(t, repo.AddReply(ctx, userReply))
	persisted, err = repo.GetByID(ctx, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, TicketStatusWaiting, persisted.Status)
	require.NoError(t, repo.Close(ctx, ticket.ID))
	require.Error(t, repo.AddReply(ctx, &TicketMessage{TicketID: ticket.ID, IsAdmin: true, Content: "Closed"}))
	messages, err := repo.GetMessages(ctx, ticket.ID)
	require.NoError(t, err)
	require.Len(t, messages, 3)
}

func TestTicketRepository_ListByUser(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")
	otherUserID := createTicketTestUser(t, db, "otheruser")

	// Create tickets for test user
	for i := 0; i < 5; i++ {
		ticket := &Ticket{
			UserID:   userID,
			Subject:  "User Ticket " + string(rune('1'+i)),
			Status:   TicketStatusOpen,
			Priority: TicketPriorityNormal,
		}
		repo.Create(ctx, ticket)
	}

	// Create tickets for other user
	for i := 0; i < 3; i++ {
		ticket := &Ticket{
			UserID:   otherUserID,
			Subject:  "Other Ticket " + string(rune('1'+i)),
			Status:   TicketStatusOpen,
			Priority: TicketPriorityNormal,
		}
		repo.Create(ctx, ticket)
	}

	// List by user
	tickets, total, err := repo.ListByUser(ctx, userID, nil)
	if err != nil {
		t.Fatalf("Failed to list tickets: %v", err)
	}

	if total != 5 {
		t.Errorf("Expected 5 tickets, got %d", total)
	}

	if len(tickets) != 5 {
		t.Errorf("Expected 5 tickets in result, got %d", len(tickets))
	}
}

func TestTicketRepository_CountByStatus(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	// Create tickets with different statuses
	statuses := []string{TicketStatusOpen, TicketStatusOpen, TicketStatusPending, TicketStatusResolved, TicketStatusClosed}
	for i, status := range statuses {
		ticket := &Ticket{
			UserID:   userID,
			Subject:  "Ticket " + string(rune('1'+i)),
			Status:   status,
			Priority: TicketPriorityNormal,
		}
		repo.Create(ctx, ticket)
	}

	// Count by status
	counts, err := repo.CountByStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to count by status: %v", err)
	}

	if counts[TicketStatusOpen] != 2 {
		t.Errorf("Expected 2 open tickets, got %d", counts[TicketStatusOpen])
	}
	if counts[TicketStatusPending] != 1 {
		t.Errorf("Expected 1 pending ticket, got %d", counts[TicketStatusPending])
	}
	if counts[TicketStatusResolved] != 1 {
		t.Errorf("Expected 1 resolved ticket, got %d", counts[TicketStatusResolved])
	}
	if counts[TicketStatusClosed] != 1 {
		t.Errorf("Expected 1 closed ticket, got %d", counts[TicketStatusClosed])
	}
}

func TestTicketRepository_GetByIDWithMessages(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	ticket := &Ticket{
		UserID:   userID,
		Subject:  "Test Ticket",
		Status:   TicketStatusOpen,
		Priority: TicketPriorityNormal,
	}
	repo.Create(ctx, ticket)

	// Add messages
	for i := 0; i < 3; i++ {
		message := &TicketMessage{
			TicketID: ticket.ID,
			UserID:   &userID,
			Content:  "Message " + string(rune('1'+i)),
			IsAdmin:  false,
		}
		repo.AddMessage(ctx, message)
	}

	// Get ticket with messages
	found, err := repo.GetByIDWithMessages(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("Failed to get ticket with messages: %v", err)
	}

	if len(found.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(found.Messages))
	}
}

// Feature: user-portal, Property 11: Ticket ID Uniqueness
// Validates: Requirements 10.6
// *For any* two tickets in the system, their ticket IDs SHALL be unique.
func TestProperty_TicketIDUniqueness(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := NewTicketRepository(db)
	ctx := context.Background()

	userID := createTicketTestUser(t, db, "testuser")

	// Create multiple tickets
	ticketIDs := make(map[int64]bool)
	for i := 0; i < 100; i++ {
		ticket := &Ticket{
			UserID:   userID,
			Subject:  "Ticket " + string(rune('0'+i%10)) + string(rune('0'+i/10)),
			Status:   TicketStatusOpen,
			Priority: TicketPriorityNormal,
		}
		err := repo.Create(ctx, ticket)
		if err != nil {
			t.Fatalf("Failed to create ticket: %v", err)
		}

		// Check uniqueness
		if ticketIDs[ticket.ID] {
			t.Errorf("Duplicate ticket ID found: %d", ticket.ID)
		}
		ticketIDs[ticket.ID] = true
	}

	if len(ticketIDs) != 100 {
		t.Errorf("Expected 100 unique ticket IDs, got %d", len(ticketIDs))
	}
}
