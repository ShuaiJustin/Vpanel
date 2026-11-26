// Package ticket provides ticket management services for the user portal.
package ticket

import (
	"context"
	"strings"
	"unicode/utf8"

	"v/internal/database/repository"
	"v/pkg/errors"
)

// Service provides ticket operations for the user portal.
type Service struct {
	ticketRepo repository.TicketRepository
	userRepo   repository.UserRepository
}

// NewService creates a new ticket service.
func NewService(ticketRepo repository.TicketRepository, userRepo repository.UserRepository) *Service {
	return &Service{
		ticketRepo: ticketRepo,
		userRepo:   userRepo,
	}
}

// CreateTicketRequest represents a request to create a ticket.
type CreateTicketRequest struct {
	Subject  string `json:"subject"`
	Content  string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
}

// Validate validates the create ticket request.
func (r *CreateTicketRequest) Validate() error {
	r.Subject = strings.TrimSpace(r.Subject)
	if r.Subject == "" {
		return errors.NewValidationError("工单主题不能为空", nil)
	}
	if utf8.RuneCountInString(r.Subject) > 200 {
		return errors.NewValidationError("工单主题不能超过200个字符", nil)
	}

	r.Content = strings.TrimSpace(r.Content)
	if r.Content == "" {
		return errors.NewValidationError("工单内容不能为空", nil)
	}
	if utf8.RuneCountInString(r.Content) > 10000 {
		return errors.NewValidationError("工单内容不能超过10000个字符", nil)
	}

	switch r.Category {
	case "":
		r.Category = "other"
	case "account", "connection", "subscription", "payment", "suggestion", "other", "technical", "billing":
	default:
		return errors.NewValidationError("无效的工单分类", nil)
	}

	switch r.Priority {
	case "":
		r.Priority = "medium"
	case "medium", repository.TicketPriorityLow, repository.TicketPriorityNormal, repository.TicketPriorityHigh, repository.TicketPriorityUrgent:
	default:
		return errors.NewValidationError("无效的优先级", nil)
	}

	return nil
}

// TicketResult represents a ticket with its messages.
type TicketResult struct {
	*repository.Ticket
	Messages []*repository.TicketMessage `json:"messages,omitempty"`
}

// CreateTicket creates a new ticket.
func (s *Service) CreateTicket(ctx context.Context, userID int64, req *CreateTicketRequest) (*repository.Ticket, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Verify user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	ticket := &repository.Ticket{
		UserID:   userID,
		Subject:  req.Subject,
		Category: req.Category,
		Priority: req.Priority,
		Status:   repository.TicketStatusOpen,
	}

	// Add the initial message
	message := &repository.TicketMessage{
		UserID:  &userID,
		Content: req.Content,
	}

	if err := s.ticketRepo.CreateWithMessage(ctx, ticket, message); err != nil {
		return nil, err
	}

	return ticket, nil
}

// GetTicket retrieves a ticket by ID for a user.
func (s *Service) GetTicket(ctx context.Context, userID int64, ticketID int64) (*TicketResult, error) {
	return s.getTicket(ctx, userID, ticketID, false)
}

// GetTicketForStaff retrieves a ticket for an authorized staff member.
func (s *Service) GetTicketForStaff(ctx context.Context, ticketID int64) (*TicketResult, error) {
	return s.getTicket(ctx, 0, ticketID, true)
}

func (s *Service) getTicket(ctx context.Context, userID, ticketID int64, staff bool) (*TicketResult, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if !staff && ticket.UserID != userID {
		return nil, errors.NewForbiddenError("无权访问此工单")
	}

	// Get messages
	messages, err := s.ticketRepo.GetMessages(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	return &TicketResult{
		Ticket:   ticket,
		Messages: messages,
	}, nil
}

// ListTickets lists tickets for a user.
func (s *Service) ListTickets(ctx context.Context, userID int64, status string, limit, offset int) ([]*repository.Ticket, int64, error) {
	return s.ticketRepo.ListByUser(ctx, userID, ticketFilter(status, limit, offset))
}

// ListStaffTickets lists the support queue for an authorized staff member.
func (s *Service) ListStaffTickets(ctx context.Context, status string, limit, offset int) ([]*repository.Ticket, int64, error) {
	return s.ticketRepo.ListAll(ctx, ticketFilter(status, limit, offset))
}

func ticketFilter(status string, limit, offset int) *repository.TicketFilter {
	filter := &repository.TicketFilter{
		Limit:  limit,
		Offset: offset,
	}

	if status != "" {
		filter.Status = &status
	}

	return filter
}

// ReplyTicketRequest represents a request to reply to a ticket.
type ReplyTicketRequest struct {
	Content string `json:"content"`
}

// Validate validates the reply request.
func (r *ReplyTicketRequest) Validate() error {
	r.Content = strings.TrimSpace(r.Content)
	if r.Content == "" {
		return errors.NewValidationError("回复内容不能为空", nil)
	}
	if utf8.RuneCountInString(r.Content) > 10000 {
		return errors.NewValidationError("回复内容不能超过10000个字符", nil)
	}
	return nil
}

// ReplyTicket adds a reply to a ticket.
func (s *Service) ReplyTicket(ctx context.Context, userID int64, ticketID int64, req *ReplyTicketRequest) (*repository.TicketMessage, error) {
	return s.replyTicket(ctx, userID, ticketID, req, false)
}

// ReplyToTicketAsStaff replies to a ticket for an authorized staff member.
func (s *Service) ReplyToTicketAsStaff(ctx context.Context, userID, ticketID int64, req *ReplyTicketRequest) (*repository.TicketMessage, error) {
	return s.replyTicket(ctx, userID, ticketID, req, true)
}

func (s *Service) replyTicket(ctx context.Context, userID, ticketID int64, req *ReplyTicketRequest, staff bool) (*repository.TicketMessage, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get ticket and verify ownership
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	if !staff && ticket.UserID != userID {
		return nil, errors.NewForbiddenError("无权访问此工单")
	}

	// Check if ticket is closed
	if ticket.Status == repository.TicketStatusClosed {
		return nil, errors.NewValidationError("工单已关闭，无法回复", nil)
	}

	// Add message
	message := &repository.TicketMessage{
		TicketID: ticketID,
		UserID:   &userID,
		Content:  req.Content,
		IsAdmin:  staff,
	}

	if err := s.ticketRepo.AddReply(ctx, message); err != nil {
		return nil, err
	}

	return message, nil
}

// CloseTicket closes a ticket.
func (s *Service) CloseTicket(ctx context.Context, userID int64, ticketID int64) error {
	return s.closeTicket(ctx, userID, ticketID, false)
}

// CloseTicketAsStaff closes a ticket for an authorized staff member.
func (s *Service) CloseTicketAsStaff(ctx context.Context, ticketID int64) error {
	return s.closeTicket(ctx, 0, ticketID, true)
}

func (s *Service) closeTicket(ctx context.Context, userID, ticketID int64, staff bool) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return err
	}

	if !staff && ticket.UserID != userID {
		return errors.NewForbiddenError("无权访问此工单")
	}

	if ticket.Status == repository.TicketStatusClosed {
		return errors.NewValidationError("工单已关闭", nil)
	}

	return s.ticketRepo.Close(ctx, ticketID)
}

// ReopenTicket reopens a closed ticket.
func (s *Service) ReopenTicket(ctx context.Context, userID int64, ticketID int64) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return err
	}

	if ticket.UserID != userID {
		return errors.NewForbiddenError("无权访问此工单")
	}

	if ticket.Status != repository.TicketStatusClosed {
		return errors.NewValidationError("工单未关闭", nil)
	}

	ticket.Status = repository.TicketStatusOpen
	ticket.ClosedAt = nil

	return s.ticketRepo.Update(ctx, ticket)
}

// TicketStatusTransition defines valid status transitions.
var TicketStatusTransition = map[string][]string{
	repository.TicketStatusOpen:     {repository.TicketStatusWaiting, repository.TicketStatusAnswered, repository.TicketStatusClosed},
	repository.TicketStatusWaiting:  {repository.TicketStatusAnswered, repository.TicketStatusClosed},
	repository.TicketStatusAnswered: {repository.TicketStatusWaiting, repository.TicketStatusClosed},
	repository.TicketStatusClosed:   {repository.TicketStatusOpen},
}

// CanTransition checks if a status transition is valid.
func CanTransition(from, to string) bool {
	validTransitions, ok := TicketStatusTransition[from]
	if !ok {
		return false
	}
	for _, valid := range validTransitions {
		if valid == to {
			return true
		}
	}
	return false
}
