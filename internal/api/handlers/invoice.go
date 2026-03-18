// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/invoice"
	"v/internal/logger"
)

// InvoiceHandler handles invoice-related requests.
type InvoiceHandler struct {
	invoiceService *invoice.Service
	logger         logger.Logger
}

// NewInvoiceHandler creates a new InvoiceHandler.
func NewInvoiceHandler(invoiceService *invoice.Service, log logger.Logger) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: invoiceService, logger: log}
}

// ListInvoices returns the current user's invoices.
func (h *InvoiceHandler) ListInvoices(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	invoices, total, err := h.invoiceService.ListByUser(c.Request.Context(), userID.(int64), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list invoices", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list invoices"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invoices": invoices, "total": total, "page": page, "page_size": pageSize})
}

// DownloadInvoice downloads an invoice PDF.
func (h *InvoiceHandler) DownloadInvoice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invoice ID"})
		return
	}
	inv, err := h.invoiceService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}
	// Check ownership
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "admin" && inv.UserID != userID.(int64) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}
	pdf, err := h.invoiceService.GeneratePDF(c.Request.Context(), inv)
	if err != nil {
		h.logger.Error("Failed to generate PDF", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=invoice-"+inv.InvoiceNo+".pdf")
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// GenerateInvoice generates an invoice for an order (admin only).
func (h *InvoiceHandler) GenerateInvoice(c *gin.Context) {
	var req struct {
		OrderID int64 `json:"order_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	inv, err := h.invoiceService.Generate(c.Request.Context(), req.OrderID)
	if err != nil {
		h.logger.Error("Failed to generate invoice", logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"invoice": inv})
}
