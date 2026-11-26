// Package invoice provides invoice management functionality.
package invoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrInvoiceNotFound = errors.New("invoice not found")
	ErrOrderNotFound   = errors.New("order not found")
	ErrInvalidInvoice  = errors.New("invalid invoice data")
)

// Invoice represents an invoice.
type Invoice struct {
	ID        int64    `json:"id"`
	InvoiceNo string   `json:"invoice_no"`
	OrderID   int64    `json:"order_id"`
	OrderNo   string   `json:"order_no"`
	UserID    int64    `json:"user_id"`
	Amount    int64    `json:"amount"`
	Content   *Content `json:"content"`
	PDFPath   string   `json:"pdf_path"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
}

// Content represents invoice content with line items.
type Content struct {
	CompanyName    string     `json:"company_name"`
	CompanyAddress string     `json:"company_address"`
	TaxID          string     `json:"tax_id"`
	CustomerName   string     `json:"customer_name"`
	CustomerEmail  string     `json:"customer_email"`
	Items          []LineItem `json:"items"`
	Subtotal       int64      `json:"subtotal"`
	Discount       int64      `json:"discount"`
	Total          int64      `json:"total"`
}

// LineItem represents a line item in an invoice.
type LineItem struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	Amount      int64  `json:"amount"`
}

// Config holds invoice service configuration.
type Config struct {
	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`
	TaxID          string `json:"tax_id"`
	PDFStoragePath string `json:"pdf_storage_path"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		CompanyName:    "V Panel",
		CompanyAddress: "",
		TaxID:          "",
		PDFStoragePath: "data/invoices",
	}
}

// Service provides invoice management operations.
type Service struct {
	invoiceRepo repository.InvoiceRepository
	orderRepo   repository.OrderRepository
	config      *Config
	logger      logger.Logger
}

// NewService creates a new invoice service.
func NewService(
	invoiceRepo repository.InvoiceRepository,
	orderRepo repository.OrderRepository,
	log logger.Logger,
	config *Config,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		invoiceRepo: invoiceRepo,
		orderRepo:   orderRepo,
		config:      config,
		logger:      log,
	}
}

// Generate generates an invoice for an order.
func (s *Service) Generate(ctx context.Context, orderID int64) (*Invoice, error) {
	existing, err := s.GetByOrder(ctx, orderID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrInvoiceNotFound) {
		return nil, err
	}

	// Get order details
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if order.Status != repository.OrderStatusPaid && order.Status != repository.OrderStatusCompleted {
		return nil, repository.ErrInvoiceOrderNotPaid
	}
	total := order.PayAmount + order.BalanceUsed

	// Build content
	content := &Content{
		CompanyName:    s.config.CompanyName,
		CompanyAddress: s.config.CompanyAddress,
		TaxID:          s.config.TaxID,
		Items: []LineItem{
			{
				Description: fmt.Sprintf("Plan subscription - Order %s", order.OrderNo),
				Quantity:    1,
				UnitPrice:   order.OriginalAmount,
				Amount:      order.OriginalAmount,
			},
		},
		Subtotal: order.OriginalAmount,
		Discount: order.DiscountAmount,
		Total:    total,
	}

	// Get user info if available
	if order.User != nil {
		content.CustomerName = order.User.Username
		content.CustomerEmail = order.User.Email
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		s.logger.Error("Failed to marshal invoice content", logger.Err(err))
		return nil, ErrInvalidInvoice
	}

	// ponytail: order IDs and the existing unique receipt number index make
	// issuance idempotent without a separate sequence table.
	repoInvoice := &repository.Invoice{
		InvoiceNo: fmt.Sprintf("RCPT-%d", orderID),
		OrderID:   orderID,
		Content:   string(contentJSON),
		CreatedAt: time.Now(),
	}

	if err := s.invoiceRepo.IssueForPaidOrder(ctx, repoInvoice); err != nil {
		if existing, lookupErr := s.GetByOrder(ctx, orderID); lookupErr == nil {
			return existing, nil
		}
		return nil, err
	}

	s.logger.Info("Invoice generated",
		logger.F("invoiceNo", repoInvoice.InvoiceNo),
		logger.F("orderID", orderID),
		logger.F("amount", total))

	return s.GetByOrder(ctx, orderID)
}

// GetByID retrieves an invoice by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*Invoice, error) {
	repoInvoice, err := s.invoiceRepo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvoiceNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.toInvoice(repoInvoice), nil
}

// GetByInvoiceNo retrieves an invoice by invoice number.
func (s *Service) GetByInvoiceNo(ctx context.Context, invoiceNo string) (*Invoice, error) {
	repoInvoice, err := s.invoiceRepo.GetByInvoiceNo(ctx, invoiceNo)
	if err != nil {
		return nil, ErrInvoiceNotFound
	}
	return s.toInvoice(repoInvoice), nil
}

// GetByOrder retrieves an invoice by order ID.
func (s *Service) GetByOrder(ctx context.Context, orderID int64) (*Invoice, error) {
	repoInvoice, err := s.invoiceRepo.GetByOrderID(ctx, orderID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvoiceNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.toInvoice(repoInvoice), nil
}

// ListByUser lists invoices for a user with pagination.
func (s *Service) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]*Invoice, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoInvoices, total, err := s.invoiceRepo.ListByUser(ctx, userID, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list invoices", logger.Err(err), logger.F("userID", userID))
		return nil, 0, err
	}

	invoices := make([]*Invoice, len(repoInvoices))
	for i, ri := range repoInvoices {
		invoices[i] = s.toInvoice(ri)
	}

	return invoices, total, nil
}

// List lists all invoices with filter and pagination.
func (s *Service) List(ctx context.Context, filter repository.InvoiceFilter, page, pageSize int) ([]*Invoice, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	repoInvoices, total, err := s.invoiceRepo.List(ctx, filter, pageSize, offset)
	if err != nil {
		s.logger.Error("Failed to list invoices", logger.Err(err))
		return nil, 0, err
	}

	invoices := make([]*Invoice, len(repoInvoices))
	for i, ri := range repoInvoices {
		invoices[i] = s.toInvoice(ri)
	}

	return invoices, total, nil
}

// GeneratePDF produces a one-page transaction receipt using the stored amount.
func (s *Service) GeneratePDF(_ context.Context, invoice *Invoice) ([]byte, error) {
	if invoice == nil {
		return nil, ErrInvoiceNotFound
	}

	issuer := s.config.CompanyName
	if invoice.Content != nil && invoice.Content.CompanyName != "" {
		issuer = invoice.Content.CompanyName
	}
	orderNo := invoice.OrderNo
	if orderNo == "" {
		orderNo = fmt.Sprintf("#%d", invoice.OrderID)
	}
	amount := fmt.Sprintf("%d.%02d", invoice.Amount/100, invoice.Amount%100)
	lines := []string{
		"Receipt number: " + invoice.InvoiceNo,
		"Order number: " + orderNo,
		"Issued on: " + invoice.CreatedAt,
		"Issuer: " + issuer,
		"Order amount: CNY " + amount,
	}
	var stream bytes.Buffer
	fmt.Fprint(&stream, "BT\n/F1 24 Tf\n54 722 Td\n(Transaction receipt) Tj\n/F1 12 Tf\n")
	for _, line := range lines {
		fmt.Fprintf(&stream, "0 -26 Td (%s) Tj\n", receiptPDFText(line))
	}
	fmt.Fprint(&stream, "ET\n")

	// ponytail: one page and a standard PDF font suffice for a receipt's
	// identifiers and amounts. Use embedded fonts if multilingual PDFs are needed.
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", stream.Len(), stream.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var document bytes.Buffer
	fmt.Fprint(&document, "%PDF-1.4\n")
	offsets := make([]int, 0, len(objects))
	for i, object := range objects {
		offsets = append(offsets, document.Len())
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xrefOffset := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&document, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&document, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return document.Bytes(), nil
}

func receiptPDFText(value string) string {
	var text strings.Builder
	for i, r := range value {
		if i >= 84 {
			break
		}
		switch r {
		case '(', ')', '\\':
			text.WriteByte('\\')
		}
		if r < 32 || r > 126 {
			r = '?'
		}
		text.WriteRune(r)
	}
	return text.String()
}

// UpdatePDFPath updates the PDF path for an invoice.
func (s *Service) UpdatePDFPath(ctx context.Context, id int64, pdfPath string) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return ErrInvoiceNotFound
	}

	invoice.PDFPath = pdfPath
	return s.invoiceRepo.Update(ctx, invoice)
}

// GetConfig returns the invoice configuration.
func (s *Service) GetConfig() *Config {
	return s.config
}

// toInvoice converts a repository invoice to a service invoice.
func (s *Service) toInvoice(ri *repository.Invoice) *Invoice {
	invoice := &Invoice{
		ID:        ri.ID,
		InvoiceNo: ri.InvoiceNo,
		OrderID:   ri.OrderID,
		UserID:    ri.UserID,
		Amount:    ri.Amount,
		PDFPath:   ri.PDFPath,
		Status:    "generated",
		CreatedAt: ri.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if ri.Order != nil {
		invoice.OrderNo = ri.Order.OrderNo
	}

	// Parse content JSON
	if ri.Content != "" {
		var content Content
		if err := json.Unmarshal([]byte(ri.Content), &content); err == nil {
			invoice.Content = &content
		}
	}

	return invoice
}
