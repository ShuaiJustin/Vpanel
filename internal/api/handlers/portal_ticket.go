// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
	userID, exists := c.Get("user_id")
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

	tickets, total, err := h.ticketService.ListTickets(c.Request.Context(), userID.(int64), status, limit, offset)
	if err != nil {
		h.logger.Error("failed to list tickets", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取工单列表失败"})
		return
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
	userID, exists := c.Get("user_id")
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

	newTicket, err := h.ticketService.CreateTicket(c.Request.Context(), userID.(int64), createReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.logger.Info("ticket created", logger.F("ticket_id", newTicket.ID), logger.F("user_id", userID))

	c.JSON(http.StatusCreated, newTicket)
}

// GetTicket returns a single ticket by ID.
func (h *PortalTicketHandler) GetTicket(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工单ID"})
		return
	}

	result, err := h.ticketService.GetTicket(c.Request.Context(), userID.(int64), id)
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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
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

	message, err := h.ticketService.ReplyTicket(c.Request.Context(), userID.(int64), id, replyReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, message)
}

// CloseTicket closes a ticket.
func (h *PortalTicketHandler) CloseTicket(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工单ID"})
		return
	}

	if err := h.ticketService.CloseTicket(c.Request.Context(), userID.(int64), id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "工单已关闭"})
}

// ReopenTicket reopens a closed ticket.
func (h *PortalTicketHandler) ReopenTicket(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工单ID"})
		return
	}

	if err := h.ticketService.ReopenTicket(c.Request.Context(), userID.(int64), id); err != nil {
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
	case containsStr(errStr, "validation"), containsStr(errStr, "不能为空"), containsStr(errStr, "不能超过"), containsStr(errStr, "已关闭"), containsStr(errStr, "未关闭"):
		c.JSON(http.StatusBadRequest, gin.H{"error": errStr})
	case containsStr(errStr, "forbidden"), containsStr(errStr, "无权"):
		c.JSON(http.StatusForbidden, gin.H{"error": errStr})
	case containsStr(errStr, "not found"), containsStr(errStr, "不存在"):
		c.JSON(http.StatusNotFound, gin.H{"error": errStr})
	default:
		h.logger.Error("portal ticket error", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
