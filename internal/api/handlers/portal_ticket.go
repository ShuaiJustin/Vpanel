// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/portal/ticket"
	"v/pkg/errors"
)

// PortalTicketHandler handles portal ticket requests.
type PortalTicketHandler struct {
	ticketService *ticket.Service
	logger        logger.Logger
}

// NewPortalTicketHandler creates a new PortalTicketHandler.
func NewPortalTicketHandler(ticketService *ticket.Service, log logger.Logger) *PortalTicketHandler {
	return &PortalTicketHandler{
		ticketService: ticketService,
		logger:        log,
	}
}

// ListTickets returns tickets for the current user.
func (h *PortalTicketHandler) ListTickets(c *gin.Context) {
	h.listTickets(c, false)
}

func (h *PortalTicketHandler) ListStaffTickets(c *gin.Context) {
	h.listTickets(c, true)
}

func (h *PortalTicketHandler) listTickets(c *gin.Context, staff bool) {
	userID, exists := currentUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// Parse pagination
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Parse status filter
	status := c.Query("status")

	var tickets []*repository.Ticket
	var total int64
	var err error
	if staff {
		tickets, total, err = h.ticketService.ListStaffTickets(c.Request.Context(), status, limit, offset)
	} else {
		tickets, total, err = h.ticketService.ListTickets(c.Request.Context(), userID, status, limit, offset)
	}
	if err != nil {
		h.logger.Error("failed to list tickets", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取工单列表失败"})
		return
	}

	if tickets == nil {
		tickets = []*repository.Ticket{}
	}
	c.JSON(http.StatusOK, gin.H{
		"tickets": tickets,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// CreateTicketRequest represents a create ticket request.
type CreateTicketRequest struct {
	Subject  string `json:"subject" binding:"required"`
	Content  string `json:"content" binding:"required"`
	Category string `json:"category"`
	Priority string `json:"priority"`
}

// CreateTicket creates a new ticket.
func (h *PortalTicketHandler) CreateTicket(c *gin.Context) {
	userID, exists := currentUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	createReq := &ticket.CreateTicketRequest{
		Subject:  req.Subject,
		Content:  req.Content,
		Category: req.Category,
		Priority: req.Priority,
	}

	newTicket, err := h.ticketService.CreateTicket(c.Request.Context(), userID, createReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.logger.Info("ticket created", logger.F("ticket_id", newTicket.ID), logger.F("user_id", userID))

	c.JSON(http.StatusCreated, newTicket)
}

// GetTicket returns a single ticket by ID.
func (h *PortalTicketHandler) GetTicket(c *gin.Context) {
	h.getTicket(c, false)
}

func (h *PortalTicketHandler) GetStaffTicket(c *gin.Context) {
	h.getTicket(c, true)
}

func (h *PortalTicketHandler) getTicket(c *gin.Context, staff bool) {
	userID, exists := currentUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工单ID"})
		return
	}

	var result *ticket.TicketResult
	if staff {
		result, err = h.ticketService.GetTicketForStaff(c.Request.Context(), id)
	} else {
		result, err = h.ticketService.GetTicket(c.Request.Context(), userID, id)
	}
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// ReplyTicketRequest represents a reply ticket request.
type ReplyTicketRequest struct {
	Content string `json:"content" binding:"required"`
}

// ReplyTicket adds a reply to a ticket.
func (h *PortalTicketHandler) ReplyTicket(c *gin.Context) {
	h.replyTicket(c, false)
}

func (h *PortalTicketHandler) ReplyAsStaff(c *gin.Context) {
	h.replyTicket(c, true)
}

func (h *PortalTicketHandler) replyTicket(c *gin.Context, staff bool) {
	userID, exists := currentUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工单ID"})
		return
	}

	var req ReplyTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	replyReq := &ticket.ReplyTicketRequest{
		Content: req.Content,
	}

	var message *repository.TicketMessage
	if staff {
		message, err = h.ticketService.ReplyToTicketAsStaff(c.Request.Context(), userID, id, replyReq)
	} else {
		message, err = h.ticketService.ReplyTicket(c.Request.Context(), userID, id, replyReq)
	}
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, message)
}

// CloseTicket closes a ticket.
func (h *PortalTicketHandler) CloseTicket(c *gin.Context) {
	h.closeTicket(c, false)
}

func (h *PortalTicketHandler) CloseAsStaff(c *gin.Context) {
	h.closeTicket(c, true)
}

func (h *PortalTicketHandler) closeTicket(c *gin.Context, staff bool) {
	userID, exists := currentUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工单ID"})
		return
	}

	if staff {
		err = h.ticketService.CloseTicketAsStaff(c.Request.Context(), id)
	} else {
		err = h.ticketService.CloseTicket(c.Request.Context(), userID, id)
	}
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "工单已关闭"})
}

// ReopenTicket reopens a closed ticket.
func (h *PortalTicketHandler) ReopenTicket(c *gin.Context) {
	userID, exists := currentUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工单ID"})
		return
	}

	if err := h.ticketService.ReopenTicket(c.Request.Context(), userID, id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "工单已重新打开"})
}

// handleError handles errors and returns appropriate HTTP responses.
func (h *PortalTicketHandler) handleError(c *gin.Context, err error) {
	// Use the errors package for proper error type checking
	if appErr, ok := errors.AsAppError(err); ok {
		c.JSON(appErr.HTTPStatus(), gin.H{"error": appErr.Message})
		return
	}

	// Fallback for non-AppError errors
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "validation"), strings.Contains(errStr, "不能为空"), strings.Contains(errStr, "不能超过"), strings.Contains(errStr, "已关闭"), strings.Contains(errStr, "未关闭"):
		c.JSON(http.StatusBadRequest, gin.H{"error": errStr})
	case strings.Contains(errStr, "forbidden"), strings.Contains(errStr, "无权"):
		c.JSON(http.StatusForbidden, gin.H{"error": errStr})
	case strings.Contains(errStr, "not found"), strings.Contains(errStr, "不存在"):
		c.JSON(http.StatusNotFound, gin.H{"error": errStr})
	default:
		h.logger.Error("portal ticket error", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
	}
}
