// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/certificate"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/pkg/errors"
)

// CertificateHandler handles certificate management requests.
type CertificateHandler struct {
	certRepo repository.CertificateRepository
	nodeRepo repository.NodeRepository
	certSvc  CertificateService
	logger   logger.Logger
}

// CertificateService defines the interface for certificate operations.
type CertificateService interface {
	Apply(ctx context.Context, req *certificate.ApplyRequest) (*repository.Certificate, error)
	Upload(ctx context.Context, domain string, certData, keyData []byte) (*repository.Certificate, error)
	Renew(ctx context.Context, certID int64) error
	DeployToAssignedNodes(ctx context.Context, certID int64) error
}

// NewCertificateHandler creates a new certificate handler.
func NewCertificateHandler(certRepo repository.CertificateRepository, nodeRepo repository.NodeRepository, certSvc CertificateService, log logger.Logger) *CertificateHandler {
	return &CertificateHandler{
		certRepo: certRepo,
		nodeRepo: nodeRepo,
		certSvc:  certSvc,
		logger:   log,
	}
}

// CertificateResponse represents a certificate in API responses.
type CertificateResponse struct {
	ID           int64  `json:"id"`
	Domain       string `json:"domain"`
	Provider     string `json:"provider"`
	AutoRenew    bool   `json:"auto_renew"`
	ExpiresAt    string `json:"expires_at"`
	DaysLeft     int    `json:"days_left"`
	Status       string `json:"status"` // pending, failed, valid, expiring, expired
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// CreateCertificateRequest represents a request to create/upload a certificate.
type CreateCertificateRequest struct {
	Domain      string  `json:"domain" binding:"required"`
	Certificate string  `json:"certificate" binding:"required"`
	PrivateKey  string  `json:"private_key" binding:"required"`
	AutoRenew   bool    `json:"auto_renew"`
	NodeIDs     []int64 `json:"node_ids"`
}

// UpdateCertificateRequest represents a request to update a certificate.
type UpdateCertificateRequest struct {
	Certificate *string `json:"certificate"`
	PrivateKey  *string `json:"private_key"`
	AutoRenew   *bool   `json:"auto_renew"`
}

// ApplyCertificateRequest represents a request to apply for a certificate using ACME.
type ApplyCertificateRequest struct {
	Domain      string            `json:"domain" binding:"required"`
	Email       string            `json:"email" binding:"required,email"`
	Provider    string            `json:"provider"`     // "letsencrypt" or "zerossl", default: "letsencrypt"
	Method      string            `json:"method"`       // "http" or "dns", default: "http"
	DNSProvider string            `json:"dns_provider"` // DNS provider for dns method, e.g., "dns_cf"
	Webroot     string            `json:"webroot"`      // Webroot path for http method, default: "/app/data/webroot"
	DNSEnv      map[string]string `json:"dns_env"`      // DNS API credentials
	AutoRenew   *bool             `json:"auto_renew"`   // Auto renew certificate (default: true)
	Wildcard    bool              `json:"wildcard"`     // 是否申请泛域名证书（*.domain.com）
	NodeIDs     []int64           `json:"node_ids"`     // 签发成功后自动关联到这些节点
}

func normalizeCertificateTLSDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if strings.HasPrefix(domain, "*.") {
		return ""
	}
	return domain
}

func deduplicateNodeIDs(nodeIDs []int64) []int64 {
	result := make([]int64, 0, len(nodeIDs))
	seen := make(map[int64]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if nodeID <= 0 {
			continue
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}
		result = append(result, nodeID)
	}
	return result
}

func (h *CertificateHandler) assignCertificateToNodes(ctx context.Context, cert *repository.Certificate, nodeIDs []int64) (int, []int64) {
	if cert == nil {
		return 0, deduplicateNodeIDs(nodeIDs)
	}

	tlsDomain := normalizeCertificateTLSDomain(cert.Domain)
	successCount := 0
	failedNodes := make([]int64, 0)

	for _, nodeID := range deduplicateNodeIDs(nodeIDs) {
		node, err := h.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			h.logger.Warn("Node not found, skipping",
				logger.F("node_id", nodeID),
				logger.Err(err))
			failedNodes = append(failedNodes, nodeID)
			continue
		}

		node.CertificateID = &cert.ID
		if tlsDomain != "" && strings.TrimSpace(node.TLSDomain) == "" {
			node.TLSDomain = tlsDomain
		}

		if err := h.nodeRepo.Update(ctx, node); err != nil {
			h.logger.Error("Failed to update node certificate",
				logger.F("node_id", nodeID),
				logger.F("cert_id", cert.ID),
				logger.Err(err))
			failedNodes = append(failedNodes, nodeID)
			continue
		}

		successCount++
		h.logger.Info("Certificate assigned to node",
			logger.F("cert_id", cert.ID),
			logger.F("cert_domain", cert.Domain),
			logger.F("node_id", nodeID),
			logger.F("node_name", node.Name))
	}

	return successCount, failedNodes
}

func (h *CertificateHandler) deployAssignedNodesAsync(cert *repository.Certificate) {
	if h.certSvc == nil || cert == nil {
		return
	}

	go func(certID int64, domain string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.certSvc.DeployToAssignedNodes(ctx, certID); err != nil {
			h.logger.Warn("Failed to auto deploy certificate to assigned nodes",
				logger.F("cert_id", certID),
				logger.F("domain", domain),
				logger.Err(err))
		}
	}(cert.ID, cert.Domain)
}

// toCertificateResponse converts a certificate to API response format.
func toCertificateResponse(cert *repository.Certificate) *CertificateResponse {
	expiresAt := cert.ExpiresAt
	if expiresAt.IsZero() && cert.ExpireDate != nil {
		expiresAt = *cert.ExpireDate
	}

	daysLeft := 0
	expiresAtStr := ""
	if !expiresAt.IsZero() {
		daysLeft = int(time.Until(expiresAt).Hours() / 24)
		expiresAtStr = expiresAt.UTC().Format(time.RFC3339)
	}

	status := cert.Status
	if status == "" || status == "active" {
		if expiresAt.IsZero() {
			status = "pending"
		} else if daysLeft < 0 {
			status = "expired"
		} else if daysLeft < 30 {
			status = "expiring"
		} else {
			status = "valid"
		}
	}

	return &CertificateResponse{
		ID:           cert.ID,
		Domain:       cert.Domain,
		Provider:     cert.Provider,
		AutoRenew:    cert.AutoRenew,
		ExpiresAt:    expiresAtStr,
		DaysLeft:     daysLeft,
		Status:       status,
		ErrorMessage: cert.ErrorMessage,
		CreatedAt:    cert.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    cert.UpdatedAt.Format(time.RFC3339),
	}
}

// List returns all certificates.
// GET /api/admin/certificates
func (h *CertificateHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	certs, err := h.certRepo.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list certificates", logger.Err(err))
		middleware.HandleInternalError(c, "获取证书列表失败", err)
		return
	}

	total, err := h.certRepo.Count(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to count certificates", logger.Err(err))
		middleware.HandleInternalError(c, "获取证书数量失败", err)
		return
	}

	response := make([]*CertificateResponse, len(certs))
	for i, cert := range certs {
		response[i] = toCertificateResponse(cert)
	}

	c.JSON(http.StatusOK, gin.H{
		"certificates": response,
		"total":        total,
	})
}

// Get returns a single certificate by ID.
// GET /api/admin/certificates/:id
func (h *CertificateHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	cert, err := h.certRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "证书不存在"})
			return
		}
		h.logger.Error("Failed to get certificate", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取证书失败"})
		return
	}

	c.JSON(http.StatusOK, toCertificateResponse(cert))
}

// GetByDomain returns a certificate by domain.
// GET /api/admin/certificates/domain/:domain
func (h *CertificateHandler) GetByDomain(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "域名不能为空"})
		return
	}

	cert, err := h.certRepo.GetByDomain(c.Request.Context(), domain)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "该域名的证书不存在"})
			return
		}
		h.logger.Error("Failed to get certificate by domain", logger.Err(err), logger.F("domain", domain))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取证书失败"})
		return
	}

	c.JSON(http.StatusOK, toCertificateResponse(cert))
}

// Create creates or uploads a new certificate.
// POST /api/admin/certificates
func (h *CertificateHandler) Create(c *gin.Context) {
	var req CreateCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if h.certSvc == nil {
		h.logger.Error("Certificate service is not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "证书服务未初始化"})
		return
	}

	cert, err := h.certSvc.Upload(c.Request.Context(), req.Domain, []byte(req.Certificate), []byte(req.PrivateKey))
	if err != nil {
		if errors.IsConflict(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "该域名的证书已存在"})
			return
		}
		h.logger.Error("Failed to upload certificate", logger.Err(err), logger.F("domain", req.Domain))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "上传证书失败"})
		return
	}

	successCount, failedNodes := h.assignCertificateToNodes(c.Request.Context(), cert, req.NodeIDs)
	if successCount > 0 {
		h.deployAssignedNodesAsync(cert)
	}

	h.logger.Info("Certificate created", logger.F("cert_id", cert.ID), logger.F("domain", cert.Domain))

	response := gin.H{
		"certificate": toCertificateResponse(cert),
	}
	if successCount > 0 || len(failedNodes) > 0 {
		response["assignment"] = gin.H{
			"success_count": successCount,
			"failed_nodes":  failedNodes,
		}
	}

	c.JSON(http.StatusCreated, response)
}

// Update updates an existing certificate.
// PUT /api/admin/certificates/:id
func (h *CertificateHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	var req UpdateCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	cert, err := h.certRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "证书不存在"})
			return
		}
		h.logger.Error("Failed to get certificate", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取证书失败"})
		return
	}

	// Update fields
	if req.Certificate != nil {
		cert.Certificate = *req.Certificate
		// TODO: 从新证书中提取过期时间
		cert.ExpiresAt = time.Now().AddDate(0, 3, 0)
		cert.ExpireDate = &cert.ExpiresAt
	}
	if req.PrivateKey != nil {
		cert.PrivateKey = *req.PrivateKey
	}
	if req.AutoRenew != nil {
		cert.AutoRenew = *req.AutoRenew
	}

	if err := h.certRepo.Update(c.Request.Context(), cert); err != nil {
		h.logger.Error("Failed to update certificate", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新证书失败"})
		return
	}

	h.logger.Info("Certificate updated", logger.F("cert_id", id))

	c.JSON(http.StatusOK, toCertificateResponse(cert))
}

// Delete deletes a certificate.
// DELETE /api/admin/certificates/:id
func (h *CertificateHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	if err := h.certRepo.Delete(c.Request.Context(), id); err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "证书不存在"})
			return
		}
		h.logger.Error("Failed to delete certificate", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除证书失败"})
		return
	}

	h.logger.Info("Certificate deleted", logger.F("cert_id", id))

	c.JSON(http.StatusOK, gin.H{"message": "证书删除成功"})
}

// Apply applies for a new certificate using ACME (Let's Encrypt).
// POST /api/admin/certificates/apply
func (h *CertificateHandler) Apply(c *gin.Context) {
	var req ApplyCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("解析证书申请请求失败", logger.Err(err))
		middleware.HandleBadRequest(c, errors.MsgInvalidRequest)
		return
	}

	h.logger.Info("收到证书申请请求",
		logger.F("domain", req.Domain),
		logger.F("email", req.Email),
		logger.F("provider", req.Provider),
		logger.F("method", req.Method),
		logger.F("dns_provider", req.DNSProvider),
		logger.F("webroot", req.Webroot))

	// 检查 service 是否初始化
	if h.certSvc == nil {
		h.logger.Error("证书服务未初始化")
		middleware.HandleInternalError(c, "证书服务未初始化，请联系管理员", nil)
		return
	}

	// 转换为 service 层的请求
	applyReq := &certificate.ApplyRequest{
		Domain:      req.Domain,
		Email:       req.Email,
		Provider:    req.Provider,
		Method:      req.Method,
		DNSProvider: req.DNSProvider,
		Webroot:     req.Webroot,
		DNSEnv:      req.DNSEnv,
		AutoRenew:   req.AutoRenew,
		Wildcard:    req.Wildcard,
		NodeIDs:     req.NodeIDs,
	}

	// 调用 service 层申请证书
	cert, err := h.certSvc.Apply(c.Request.Context(), applyReq)
	if err != nil {
		h.logger.Error("证书申请失败",
			logger.F("domain", req.Domain),
			logger.Err(err))

		statusCode := http.StatusInternalServerError
		errorCode := "CERTIFICATE_APPLY_FAILED"
		errCode := errors.GetCode(err)
		errorMessage := err.Error()
		if appErr, ok := errors.AsAppError(err); ok {
			errorMessage = appErr.Message
		}
		switch {
		case errors.IsConflict(err):
			// 如果同域名证书已在申请中，返回 202 而不是报错
			existingCert, getErr := h.certRepo.GetByDomain(c.Request.Context(), req.Domain)
			if getErr == nil && existingCert != nil && existingCert.Status == "pending" {
				c.JSON(http.StatusAccepted, gin.H{
					"message": "该域名证书正在后台申请中，请稍后刷新查看结果",
					"domain":  req.Domain,
					"cert_id": existingCert.ID,
					"status":  existingCert.Status,
				})
				return
			}
			statusCode = http.StatusConflict
			errorCode = "CERTIFICATE_APPLY_CONFLICT"
		case errCode == errors.ErrCodeBadRequest || errCode == errors.ErrCodeValidation:
			statusCode = http.StatusBadRequest
			errorCode = "CERTIFICATE_APPLY_INVALID_REQUEST"
		case errors.IsRateLimit(err):
			statusCode = http.StatusTooManyRequests
			errorCode = "CERTIFICATE_APPLY_RATE_LIMIT"
		}

		c.JSON(statusCode, gin.H{
			"success": false,
			"error": gin.H{
				"code":    errorCode,
				"message": errorMessage,
			},
		})
		return
	}

	h.logger.Info("证书申请已提交",
		logger.F("domain", req.Domain),
		logger.F("cert_id", cert.ID))

	c.JSON(http.StatusAccepted, gin.H{
		"message": "证书申请已提交，正在后台处理",
		"domain":  req.Domain,
		"cert_id": cert.ID,
		"status":  cert.Status,
	})
}

// Renew renews an existing certificate.
// POST /api/admin/certificates/:id/renew
func (h *CertificateHandler) Renew(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	cert, err := h.certRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "证书不存在"})
			return
		}
		h.logger.Error("Failed to get certificate", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取证书失败"})
		return
	}

	// TODO: 实现证书续期逻辑
	if h.certSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "证书服务不可用"})
		return
	}

	if err := h.certSvc.Renew(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to renew certificate", logger.Err(err), logger.F("id", id), logger.F("domain", cert.Domain))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	refreshed, refreshErr := h.certRepo.GetByID(c.Request.Context(), id)
	if refreshErr == nil && refreshed != nil {
		cert = refreshed
	}

	h.logger.Info("Certificate renewal requested",
		logger.F("cert_id", id),
		logger.F("domain", cert.Domain))

	c.JSON(http.StatusOK, gin.H{
		"message":     "证书续期成功",
		"domain":      cert.Domain,
		"expire_date": cert.ExpiresAt,
	})
}

// Validate validates a certificate and returns detailed metadata.
// GET /api/admin/certificates/:id/validate
func (h *CertificateHandler) Validate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	cert, err := h.certRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "证书不存在"})
			return
		}
		h.logger.Error("Failed to get certificate", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取证书失败"})
		return
	}

	// 申请中证书尚未落地，返回友好状态而不是错误
	if cert.Status == "pending" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "证书仍在申请中，请稍后重试验证",
			"details": fmt.Sprintf("域名: %s\n状态: pending", cert.Domain),
			"data": gin.H{
				"domain": cert.Domain,
				"status": "pending",
			},
		})
		return
	}

	certPEM, _, err := h.readCertificateMaterial(cert)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "证书内容不可用",
			"details": err.Error(),
		})
		return
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "证书格式无效",
			"details": "无法解析 PEM 证书内容",
		})
		return
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "证书解析失败",
			"details": err.Error(),
		})
		return
	}

	now := time.Now()
	daysLeft := int(time.Until(x509Cert.NotAfter).Hours() / 24)
	validNow := now.After(x509Cert.NotBefore) && now.Before(x509Cert.NotAfter)

	status := "valid"
	if !validNow {
		if now.Before(x509Cert.NotBefore) {
			status = "not_yet_valid"
		} else {
			status = "expired"
		}
	} else if daysLeft < 30 {
		status = "expiring"
	}

	details := fmt.Sprintf(
		"域名: %s\n颁发者: %s\n主题: %s\n序列号: %s\n生效时间: %s\n过期时间: %s\n剩余天数: %d\n状态: %s",
		cert.Domain,
		x509Cert.Issuer.CommonName,
		x509Cert.Subject.CommonName,
		x509Cert.SerialNumber.String(),
		x509Cert.NotBefore.UTC().Format(time.RFC3339),
		x509Cert.NotAfter.UTC().Format(time.RFC3339),
		daysLeft,
		status,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": validNow,
		"message": "证书验证完成",
		"details": details,
		"data": gin.H{
			"domain":       cert.Domain,
			"issuer":       x509Cert.Issuer.CommonName,
			"subject":      x509Cert.Subject.CommonName,
			"not_before":   x509Cert.NotBefore.UTC().Format(time.RFC3339),
			"not_after":    x509Cert.NotAfter.UTC().Format(time.RFC3339),
			"days_left":    daysLeft,
			"status":       status,
			"is_valid_now": validNow,
		},
	})
}

// Backup exports certificate and private key as a zip archive.
// GET /api/admin/certificates/:id/backup
func (h *CertificateHandler) Backup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	cert, err := h.certRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "证书不存在"})
			return
		}
		h.logger.Error("Failed to get certificate", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取证书失败"})
		return
	}

	certPEM, keyPEM, err := h.readCertificateMaterial(cert)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := addZipFile(zw, "fullchain.pem", certPEM); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打包证书失败"})
		return
	}
	if err := addZipFile(zw, "privkey.pem", keyPEM); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打包私钥失败"})
		return
	}

	meta := fmt.Sprintf(
		"domain=%s\nprovider=%s\nstatus=%s\nexported_at=%s\n",
		cert.Domain,
		cert.Provider,
		cert.Status,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err := addZipFile(zw, "metadata.txt", []byte(meta)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打包元数据失败"})
		return
	}

	if err := zw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成备份文件失败"})
		return
	}

	filenameDomain := strings.NewReplacer("*", "wildcard", "/", "_", "\\", "_").Replace(cert.Domain)
	filename := fmt.Sprintf("certificate_%s_%d.zip", filenameDomain, cert.ID)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

func addZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (h *CertificateHandler) readCertificateMaterial(cert *repository.Certificate) ([]byte, []byte, error) {
	var certPEM []byte
	if cert.Certificate != "" {
		certPEM = []byte(cert.Certificate)
	} else if cert.CertPath != "" {
		data, err := os.ReadFile(cert.CertPath)
		if err != nil {
			return nil, nil, fmt.Errorf("读取证书文件失败: %w", err)
		}
		certPEM = data
	}

	var keyPEM []byte
	if cert.PrivateKey != "" {
		keyPEM = []byte(cert.PrivateKey)
	} else if cert.KeyPath != "" {
		data, err := os.ReadFile(cert.KeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("读取私钥文件失败: %w", err)
		}
		keyPEM = data
	}

	if len(certPEM) == 0 {
		return nil, nil, fmt.Errorf("证书内容为空")
	}
	if len(keyPEM) == 0 {
		return nil, nil, fmt.Errorf("私钥内容为空")
	}

	return certPEM, keyPEM, nil
}

// ListAll returns all certificates (for dropdown selection).
// GET /api/admin/certificates/all
func (h *CertificateHandler) ListAll(c *gin.Context) {
	ctx := c.Request.Context()

	certs, err := h.certRepo.List(ctx, 0, 1000) // 获取所有证书
	if err != nil {
		h.logger.Error("Failed to list all certificates", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取证书列表失败",
		})
		return
	}

	// 简化的响应，只返回必要字段
	type SimpleCert struct {
		ID        int64  `json:"id"`
		Domain    string `json:"domain"`
		ExpiresAt string `json:"expires_at,omitempty"`
	}

	simpleCerts := make([]SimpleCert, 0, len(certs))
	for _, cert := range certs {
		sc := SimpleCert{
			ID:     cert.ID,
			Domain: cert.Domain,
		}
		if !cert.ExpiresAt.IsZero() {
			sc.ExpiresAt = cert.ExpiresAt.Format("2006-01-02")
		}
		simpleCerts = append(simpleCerts, sc)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    simpleCerts,
	})
}

// GetExpiring returns certificates that are expiring soon.
// GET /api/admin/certificates/expiring
func (h *CertificateHandler) GetExpiring(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	certs, err := h.certRepo.GetExpiring(c.Request.Context(), days)
	if err != nil {
		h.logger.Error("Failed to get expiring certificates", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取即将过期的证书失败"})
		return
	}

	response := make([]*CertificateResponse, len(certs))
	for i, cert := range certs {
		response[i] = toCertificateResponse(cert)
	}

	c.JSON(http.StatusOK, gin.H{
		"certificates": response,
		"total":        len(response),
		"days":         days,
	})
}

// AssignToNodesRequest represents a request to assign certificate to nodes.
type AssignToNodesRequest struct {
	NodeIDs []int64 `json:"node_ids" binding:"required"`
}

// AssignToNodes assigns a certificate to one or more nodes.
// POST /api/admin/certificates/:id/assign
func (h *CertificateHandler) AssignToNodes(c *gin.Context) {
	certID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的证书 ID",
		})
		return
	}

	var req AssignToNodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
		})
		return
	}

	if len(req.NodeIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请至少选择一个节点",
		})
		return
	}

	ctx := c.Request.Context()

	// 验证证书是否存在
	cert, err := h.certRepo.GetByID(ctx, certID)
	if err != nil {
		h.logger.Error("Failed to get certificate", logger.Err(err), logger.F("cert_id", certID))
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "证书不存在",
		})
		return
	}

	successCount, failedNodes := h.assignCertificateToNodes(ctx, cert, req.NodeIDs)
	if successCount > 0 {
		h.deployAssignedNodesAsync(cert)
	}

	// 返回结果
	if successCount == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "所有节点分配失败",
			"data": gin.H{
				"failed_nodes": failedNodes,
			},
		})
		return
	}

	if len(failedNodes) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "部分节点分配成功",
			"data": gin.H{
				"success_count": successCount,
				"failed_count":  len(failedNodes),
				"failed_nodes":  failedNodes,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "证书分配成功",
		"data": gin.H{
			"success_count": successCount,
			"certificate": gin.H{
				"id":     cert.ID,
				"domain": cert.Domain,
			},
		},
	})
}

// GetAssignedNodes returns all nodes assigned to a certificate.
// GET /api/admin/certificates/:id/nodes
func (h *CertificateHandler) GetAssignedNodes(c *gin.Context) {
	certID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的证书 ID",
		})
		return
	}

	ctx := c.Request.Context()

	// 验证证书是否存在
	_, err = h.certRepo.GetByID(ctx, certID)
	if err != nil {
		h.logger.Error("Failed to get certificate", logger.Err(err), logger.F("cert_id", certID))
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "证书不存在",
		})
		return
	}

	// 获取所有节点，筛选出使用此证书的节点
	allNodes, err := h.nodeRepo.List(ctx, &repository.NodeFilter{Limit: 10000})
	if err != nil {
		h.logger.Error("Failed to list nodes", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取节点列表失败",
		})
		return
	}

	// 筛选使用此证书的节点
	type NodeInfo struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Address string `json:"address"`
		Port    int    `json:"port"`
		Status  string `json:"status"`
	}

	assignedNodes := make([]NodeInfo, 0)
	for _, node := range allNodes {
		if node.CertificateID != nil && *node.CertificateID == certID {
			assignedNodes = append(assignedNodes, NodeInfo{
				ID:      node.ID,
				Name:    node.Name,
				Address: node.Address,
				Port:    node.Port,
				Status:  node.Status,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"nodes": assignedNodes,
			"total": len(assignedNodes),
		},
	})
}

// UnassignFromNode removes certificate assignment from a node.
// DELETE /api/admin/certificates/:id/nodes/:nodeId
func (h *CertificateHandler) UnassignFromNode(c *gin.Context) {
	certID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的证书 ID",
		})
		return
	}

	nodeID, err := strconv.ParseInt(c.Param("nodeId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的节点 ID",
		})
		return
	}

	ctx := c.Request.Context()

	// 获取节点
	node, err := h.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		h.logger.Error("Failed to get node", logger.Err(err), logger.F("node_id", nodeID))
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "节点不存在",
		})
		return
	}

	// 检查节点是否使用此证书
	if node.CertificateID == nil || *node.CertificateID != certID {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该节点未使用此证书",
		})
		return
	}

	// 移除证书分配
	node.CertificateID = nil
	if err := h.nodeRepo.Update(ctx, node); err != nil {
		h.logger.Error("Failed to unassign certificate from node",
			logger.F("node_id", nodeID),
			logger.F("cert_id", certID),
			logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "移除证书分配失败",
		})
		return
	}

	h.logger.Info("Certificate unassigned from node",
		logger.F("cert_id", certID),
		logger.F("node_id", nodeID),
		logger.F("node_name", node.Name))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "证书分配已移除",
	})
}
