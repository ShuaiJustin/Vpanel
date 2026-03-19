package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/portal/ticket"
)

// ticketMockTicketRepo is a mock implementation of TicketRepository for testing.
type ticketMockTicketRepo struct {
	tickets   map[int64]*repository.Ticket
	messages  map[int64][]*repository.TicketMessage
	nextID    int64
	nextMsgID int64
}

func newTicketMockTicketRepo() *ticketMockTicketRepo {
	return &ticketMockTicketRepo{
		tickets:   make(map[int64]*repository.Ticket),
		messages:  make(map[int64][]*repository.TicketMessage),
		nextID:    1,
		nextMsgID: 1,
	}
}

func (m *ticketMockTicketRepo) Create(ctx context.Context, t *repository.Ticket) error {
	t.ID = m.nextID
	m.nextID++
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	m.tickets[t.ID] = t
	return nil
}

func (m *ticketMockTicketRepo) GetByID(ctx context.Context, id int64) (*repository.Ticket, error) {
	if t, ok := m.tickets[id]; ok {
		return t, nil
	}
	return nil, &ticketNotFoundError{}
}

func (m *ticketMockTicketRepo) GetByIDWithMessages(ctx context.Context, id int64) (*repository.Ticket, error) {
	if t, ok := m.tickets[id]; ok {
		// Convert []*TicketMessage to []TicketMessage
		msgs := m.messages[id]
		t.Messages = make([]repository.TicketMessage, len(msgs))
		for i, msg := range msgs {
			t.Messages[i] = *msg
		}
		return t, nil
	}
	return nil, &ticketNotFoundError{}
}

func (m *ticketMockTicketRepo) Update(ctx context.Context, t *repository.Ticket) error {
	t.UpdatedAt = time.Now()
	m.tickets[t.ID] = t
	return nil
}

func (m *ticketMockTicketRepo) Delete(ctx context.Context, id int64) error {
	delete(m.tickets, id)
	delete(m.messages, id)
	return nil
}

func (m *ticketMockTicketRepo) ListByUser(ctx context.Context, userID int64, filter *repository.TicketFilter) ([]*repository.Ticket, int64, error) {
	var result []*repository.Ticket
	for _, t := range m.tickets {
		if t.UserID == userID {
			if filter != nil && filter.Status != nil && t.Status != *filter.Status {
				continue
			}
			result = append(result, t)
		}
	}
	return result, int64(len(result)), nil
}

func (m *ticketMockTicketRepo) ListAll(ctx context.Context, filter *repository.TicketFilter) ([]*repository.Ticket, int64, error) {
	var result []*repository.Ticket
	for _, t := range m.tickets {
		if filter != nil {
			if filter.UserID != nil && t.UserID != *filter.UserID {
				continue
			}
			if filter.Status != nil && t.Status != *filter.Status {
				continue
			}
		}
		result = append(result, t)
	}
	return result, int64(len(result)), nil
}

func (m *ticketMockTicketRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	if t, ok := m.tickets[id]; ok {
		t.Status = status
		t.UpdatedAt = time.Now()
		return nil
	}
	return &ticketNotFoundError{}
}

func (m *ticketMockTicketRepo) Close(ctx context.Context, id int64) error {
	if t, ok := m.tickets[id]; ok {
		t.Status = repository.TicketStatusClosed
		now := time.Now()
		t.ClosedAt = &now
		t.UpdatedAt = now
		return nil
	}
	return &ticketNotFoundError{}
}

func (m *ticketMockTicketRepo) AddMessage(ctx context.Context, msg *repository.TicketMessage) error {
	msg.ID = m.nextMsgID
	m.nextMsgID++
	msg.CreatedAt = time.Now()
	m.messages[msg.TicketID] = append(m.messages[msg.TicketID], msg)
	return nil
}

func (m *ticketMockTicketRepo) GetMessages(ctx context.Context, ticketID int64) ([]*repository.TicketMessage, error) {
	return m.messages[ticketID], nil
}

func (m *ticketMockTicketRepo) CountByUser(ctx context.Context, userID int64) (int64, error) {
	count := int64(0)
	for _, t := range m.tickets {
		if t.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *ticketMockTicketRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	counts := make(map[string]int64)
	for _, t := range m.tickets {
		counts[t.Status]++
	}
	return counts, nil
}

type ticketNotFoundError struct{}

func (e *ticketNotFoundError) Error() string {
	return "not found"
}

// ticketMockUserRepo is a mock user repo for ticket tests.
type ticketMockUserRepo struct {
	users  map[int64]*repository.User
	nextID int64
}

func newTicketMockUserRepo() *ticketMockUserRepo {
	return &ticketMockUserRepo{
		users:  make(map[int64]*repository.User),
		nextID: 1,
	}
}

func (m *ticketMockUserRepo) Create(ctx context.Context, user *repository.User) error {
	user.ID = m.nextID
	m.nextID++
	m.users[user.ID] = user
	return nil
}

func (m *ticketMockUserRepo) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, &ticketNotFoundError{}
}

func (m *ticketMockUserRepo) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	return nil, &ticketNotFoundError{}
}

func (m *ticketMockUserRepo) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	return nil, &ticketNotFoundError{}
}

func (m *ticketMockUserRepo) Update(ctx context.Context, user *repository.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *ticketMockUserRepo) Delete(ctx context.Context, id int64) error {
	delete(m.users, id)
	return nil
}

func (m *ticketMockUserRepo) List(ctx context.Context, limit, offset int) ([]*repository.User, error) {
	return nil, nil
}

func (m *ticketMockUserRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func (m *ticketMockUserRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}

type ticketMockLogger struct{}

func (m *ticketMockLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *ticketMockLogger) Info(msg string, fields ...logger.Field)   {}
func (m *ticketMockLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *ticketMockLogger) Error(msg string, fields ...logger.Field)  {}
func (m *ticketMockLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *ticketMockLogger) With(fields ...logger.Field) logger.Logger { return m }
func (m *ticketMockLogger) SetLevel(level logger.Level)               {}
func (m *ticketMockLogger) GetLevel() logger.Level                    { return logger.InfoLevel }

func setupTicketTestRouter() (*gin.Engine, *PortalTicketHandler, *ticketMockTicketRepo) {
	gin.SetMode(gin.TestMode)

	ticketRepo := newTicketMockTicketRepo()
	userRepo := newTicketMockUserRepo()

	// Create a test user
	userRepo.Create(context.Background(), &repository.User{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
		Enabled:  true,
	})

	ticketService := ticket.NewService(ticketRepo, userRepo)
	handler := NewPortalTicketHandler(ticketService, &ticketMockLogger{})

	router := gin.New()

	// Add middleware to set user_id
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})

	// Setup routes
	portal := router.Group("/api/portal")
	{
		tickets := portal.Group("/tickets")
		{
			tickets.GET("", handler.ListTickets)
			tickets.POST("", handler.CreateTicket)
			tickets.GET("/:id", handler.GetTicket)
			tickets.POST("/:id/reply", handler.ReplyTicket)
			tickets.POST("/:id/close", handler.CloseTicket)
			tickets.POST("/:id/reopen", handler.ReopenTicket)
		}
	}

	return router, handler, ticketRepo
}

func TestPortalTicketHandler_CreateTicket(t *testing.T) {
	router, _, ticketRepo := setupTicketTestRouter()

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "successful creation",
			body: map[string]interface{}{
				"subject":  "Test Ticket",
				"content":  "This is a test ticket content",
				"category": "technical",
				"priority": "medium",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing subject",
			body: map[string]interface{}{
				"content": "This is a test ticket content",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing content",
			body: map[string]interface{}{
				"subject": "Test Ticket",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/portal/tickets", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("CreateTicket() status = %v, want %v, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}

	// Verify ticket was created
	if len(ticketRepo.tickets) != 1 {
		t.Errorf("Expected 1 ticket to be created, got %d", len(ticketRepo.tickets))
	}
}

func TestPortalTicketHandler_ListTickets(t *testing.T) {
	router, _, ticketRepo := setupTicketTestRouter()

	// Create some test tickets
	ticketRepo.Create(context.Background(), &repository.Ticket{
		UserID:   1,
		Subject:  "Ticket 1",
		Status:   repository.TicketStatusOpen,
		Category: "technical",
	})
	ticketRepo.Create(context.Background(), &repository.Ticket{
		UserID:   1,
		Subject:  "Ticket 2",
		Status:   repository.TicketStatusClosed,
		Category: "billing",
	})
	ticketRepo.Create(context.Background(), &repository.Ticket{
		UserID:   2, // Different user
		Subject:  "Ticket 3",
		Status:   repository.TicketStatusOpen,
		Category: "technical",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/portal/tickets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListTickets() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	tickets := response["tickets"].([]interface{})
	if len(tickets) != 2 {
		t.Errorf("Expected 2 tickets for user 1, got %d", len(tickets))
	}
}

func TestPortalTicketHandler_GetTicket(t *testing.T) {
	router, _, ticketRepo := setupTicketTestRouter()

	// Create a test ticket
	ticketRepo.Create(context.Background(), &repository.Ticket{
		UserID:   1,
		Subject:  "Test Ticket",
		Status:   repository.TicketStatusOpen,
		Category: "technical",
	})

	// Add a message
	ticketRepo.AddMessage(context.Background(), &repository.TicketMessage{
		TicketID: 1,
		Content:  "Initial message",
		IsAdmin:  false,
	})

	tests := []struct {
		name       string
		ticketID   string
		wantStatus int
	}{
		{
			name:       "existing ticket",
			ticketID:   "1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent ticket",
			ticketID:   "999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid ticket ID",
			ticketID:   "invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/portal/tickets/"+tt.ticketID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GetTicket() status = %v, want %v, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestPortalTicketHandler_ReplyTicket(t *testing.T) {
	router, _, ticketRepo := setupTicketTestRouter()

	// Create a test ticket
	ticketRepo.Create(context.Background(), &repository.Ticket{
		UserID:   1,
		Subject:  "Test Ticket",
		Status:   repository.TicketStatusOpen,
		Category: "technical",
	})

	tests := []struct {
		name       string
		ticketID   string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name:     "successful reply",
			ticketID: "1",
			body: map[string]interface{}{
				"content": "This is a reply",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:     "empty content",
			ticketID: "1",
			body: map[string]interface{}{
				"content": "",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:     "non-existent ticket",
			ticketID: "999",
			body: map[string]interface{}{
				"content": "This is a reply",
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/portal/tickets/"+tt.ticketID+"/reply", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ReplyTicket() status = %v, want %v, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestPortalTicketHandler_CloseAndReopenTicket(t *testing.T) {
	router, _, ticketRepo := setupTicketTestRouter()

	// Create a test ticket
	ticketRepo.Create(context.Background(), &repository.Ticket{
		UserID:   1,
		Subject:  "Test Ticket",
		Status:   repository.TicketStatusOpen,
		Category: "technical",
	})

	// Close the ticket
	req := httptest.NewRequest(http.MethodPost, "/api/portal/tickets/1/close", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CloseTicket() status = %v, want %v", w.Code, http.StatusOK)
	}

	// Verify ticket is closed
	ticketObj, _ := ticketRepo.GetByID(context.Background(), 1)
	if ticketObj.Status != repository.TicketStatusClosed {
		t.Errorf("Expected ticket status to be closed, got %s", ticketObj.Status)
	}

	// Reopen the ticket
	req = httptest.NewRequest(http.MethodPost, "/api/portal/tickets/1/reopen", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ReopenTicket() status = %v, want %v", w.Code, http.StatusOK)
	}

	// Verify ticket is reopened
	ticketObj, _ = ticketRepo.GetByID(context.Background(), 1)
	if ticketObj.Status != repository.TicketStatusOpen {
		t.Errorf("Expected ticket status to be open, got %s", ticketObj.Status)
	}
}

func TestPortalTicketHandler_CannotReplyToClosedTicket(t *testing.T) {
	router, _, ticketRepo := setupTicketTestRouter()

	// Create a closed ticket
	ticketRepo.Create(context.Background(), &repository.Ticket{
		UserID:   1,
		Subject:  "Closed Ticket",
		Status:   repository.TicketStatusClosed,
		Category: "technical",
	})

	body, _ := json.Marshal(map[string]interface{}{
		"content": "Trying to reply to closed ticket",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/tickets/1/reply", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected BadRequest for replying to closed ticket, got %d: %s", w.Code, w.Body.String())
	}
}
