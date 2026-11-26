package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/api/middleware"
	"v/internal/auth"
	"v/internal/commercial/invoice"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestReceiptIssuanceAndDownload(t *testing.T) {
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "receipts.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.Invoice{}))

	newOrder := func(number, status string) *repository.Order {
		order := &repository.Order{
			OrderNo: number, UserID: 10, PlanID: 1, Status: status,
			OriginalAmount: 1500, DiscountAmount: 266, PayAmount: 1000, BalanceUsed: 234,
		}
		require.NoError(t, db.Create(order).Error)
		return order
	}
	paid := newOrder("ORD-1", repository.OrderStatusPaid)
	pending := newOrder("ORD-2", repository.OrderStatusPending)
	historical := newOrder("ORD-3", repository.OrderStatusCompleted)
	second := newOrder("ORD-4", repository.OrderStatusCompleted)
	cancelled := newOrder("ORD-5", repository.OrderStatusCancelled)
	refunded := newOrder("ORD-6", repository.OrderStatusRefunded)
	legacyPaid := newOrder("ORD-7", repository.OrderStatusCancelled)
	require.NoError(t, db.Model(legacyPaid).UpdateColumn("paid_at", time.Now()).Error)
	repo := repository.NewInvoiceRepository(db)
	paidHistoricalReceipt := repository.Invoice{
		InvoiceNo: "INV-202501-0001", OrderID: historical.ID, UserID: 10, Amount: 1000, Content: "{}",
	}
	require.NoError(t, repo.Create(ctx, &paidHistoricalReceipt))
	pendingReceipt := repository.Invoice{
		InvoiceNo: "INV-202501-0002", OrderID: pending.ID, UserID: 10, Amount: 1000, Content: "{}",
	}
	require.NoError(t, repo.Create(ctx, &pendingReceipt))
	cancelledReceipt := repository.Invoice{
		InvoiceNo: "INV-202501-0003", OrderID: cancelled.ID, UserID: 10, Amount: 1000, Content: "{}",
	}
	require.NoError(t, repo.Create(ctx, &cancelledReceipt))
	refundedReceipt := repository.Invoice{
		InvoiceNo: "INV-202501-0004", OrderID: refunded.ID, UserID: 10, Amount: 1000, Content: "{}",
	}
	require.NoError(t, repo.Create(ctx, &refundedReceipt))
	legacyPaidReceipt := repository.Invoice{
		InvoiceNo: "INV-202501-0005", OrderID: legacyPaid.ID, UserID: 10, Amount: 1000, Content: "{}",
	}
	require.NoError(t, repo.Create(ctx, &legacyPaidReceipt))
	newService := func() *invoice.Service {
		return invoice.NewService(repo, repository.NewOrderRepository(db), logger.NewNopLogger(), nil)
	}
	newRouter := func(userID int64, role string) *gin.Engine {
		handler := NewInvoiceHandler(newService(), logger.NewNopLogger())
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Set(string(middleware.UserClaimsKey), &auth.Claims{UserID: userID, Role: role})
		})
		staff := router.Group("/admin/invoices", middleware.NewAuthMiddleware(nil, nil).RequirePermission("system:write"))
		staff.POST("/generate", handler.GenerateInvoice)
		router.GET("/invoices", handler.ListInvoices)
		router.GET("/invoices/:id/download", handler.DownloadInvoice)
		return router
	}
	request := func(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}
	owner := newRouter(10, "user")
	admin := newRouter(1, "admin")
	issue := func(router *gin.Engine, id int64) *httptest.ResponseRecorder {
		return request(router, http.MethodPost, "/admin/invoices/generate", fmt.Sprintf(`{"order_id":%d}`, id))
	}
	require.Equal(t, http.StatusForbidden, issue(owner, paid.ID).Code)
	require.Equal(t, http.StatusConflict, issue(admin, pending.ID).Code)
	require.Equal(t, http.StatusConflict, issue(admin, cancelled.ID).Code)
	require.Equal(t, http.StatusNotFound, issue(admin, 9999).Code)

	// Two service instances issue the same order concurrently.
	results := make(chan *httptest.ResponseRecorder, 2)
	var wait sync.WaitGroup
	for _, router := range []*gin.Engine{admin, newRouter(1, "admin")} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- issue(router, paid.ID)
		}()
	}
	wait.Wait()
	close(results)
	decode := func(response *httptest.ResponseRecorder) invoice.Invoice {
		require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
		var body struct {
			Invoice invoice.Invoice `json:"invoice"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		return body.Invoice
	}
	first := decode(<-results)
	require.Equal(t, first.ID, decode(<-results).ID)
	require.Equal(t, fmt.Sprintf("RCPT-%d", paid.ID), first.InvoiceNo)
	require.Equal(t, int64(1234), first.Amount)
	require.Equal(t, "ORD-1", first.OrderNo)
	require.NotEqual(t, first.InvoiceNo, decode(issue(newRouter(1, "admin"), second.ID)).InvoiceNo)
	require.Equal(t, paidHistoricalReceipt.InvoiceNo, decode(issue(admin, historical.ID)).InvoiceNo)
	require.Equal(t, refundedReceipt.InvoiceNo, decode(issue(admin, refunded.ID)).InvoiceNo)
	require.Equal(t, legacyPaidReceipt.InvoiceNo, decode(issue(admin, legacyPaid.ID)).InvoiceNo)

	list := request(owner, http.MethodGet, "/invoices", "")
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), `"order_no":"ORD-3"`)
	require.Contains(t, list.Body.String(), `"status":"generated"`)
	require.Contains(t, list.Body.String(), `"order_no":"ORD-6"`)
	require.Contains(t, list.Body.String(), `"order_no":"ORD-7"`)
	require.NotContains(t, list.Body.String(), pendingReceipt.InvoiceNo)
	require.NotContains(t, list.Body.String(), cancelledReceipt.InvoiceNo)
	require.Contains(t, list.Body.String(), `"total":5`)

	for _, record := range []repository.Invoice{pendingReceipt, cancelledReceipt} {
		path := fmt.Sprintf("/invoices/%d/download", record.ID)
		require.Equal(t, http.StatusNotFound, request(owner, http.MethodGet, path, "").Code)
		require.Equal(t, http.StatusNotFound, request(admin, http.MethodGet, path, "").Code)
	}

	downloadPath := fmt.Sprintf("/invoices/%d/download", first.ID)
	receipt := request(owner, http.MethodGet, downloadPath, "")
	require.Equal(t, http.StatusOK, receipt.Code)
	require.Equal(t, "application/pdf", receipt.Header().Get("Content-Type"))
	require.Equal(t, "private, no-store", receipt.Header().Get("Cache-Control"))
	require.Contains(t, receipt.Header().Get("Content-Disposition"), first.InvoiceNo+".pdf")
	pdf := receipt.Body.String()
	require.Contains(t, pdf, "Transaction receipt")
	require.Contains(t, pdf, "Order amount: CNY 12.34")
	requirePDFOffsets(t, pdf)
	require.Equal(t, http.StatusForbidden, request(newRouter(11, "user"), http.MethodGet, downloadPath, "").Code)
}

func requirePDFOffsets(t *testing.T, pdf string) {
	t.Helper()
	start := strings.LastIndex(pdf, "startxref\n")
	require.NotEqual(t, -1, start)
	xrefLines := strings.Split(pdf[start:], "\n")
	xref, err := strconv.Atoi(xrefLines[1])
	require.NoError(t, err)
	require.Greater(t, xref, 0)
	require.Less(t, xref, start)
	lines := strings.Split(pdf[xref:start], "\n")
	require.Equal(t, "xref", lines[0])
	for object := 1; object <= 5; object++ {
		fields := strings.Fields(lines[object+2])
		offset, err := strconv.Atoi(fields[0])
		require.NoError(t, err)
		require.Greater(t, offset, 0)
		require.Less(t, offset, xref)
		require.True(t, strings.HasPrefix(pdf[offset:], fmt.Sprintf("%d 0 obj\n", object)))
	}
}
