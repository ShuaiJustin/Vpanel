// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"net/http"
	"strconv"
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
	Renew(ctx context.Context, certID int64) error
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
	ID          int64  `json:"id"`
	Domain      string `json:"domain"`
	AutoRenew   bool   `json:"auto_renew"`
	ExpiresAt   string `json:"expires_at"`
	DaysLeft    int    `json:"days_left"`
	Status      string `json:"status"` // valid, expiring, expired
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CreateCertificateRequest represents a request to create/upload a certificate.
type CreateCertificateRequest struct {
	Domain      string `json:"domain" binding:"required"`
	Certificate string `json:"certificate" binding:"required"`
	PrivateKey  string `json:"private_key" binding:"required"`
	AutoRenew   bool   `json:"auto_renew"`
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
	Provider    string            `json:"provider"`                // "letsencrypt" or "zerossl", default: "letsencrypt"
	Method      string            `json:"method"`                  // "http" or "dns", default: "http"
	DNSProvider string            `json:"dns_provider"`            // DNS provider for dns method, e.g., "dns_cf"
	Webroot     string            `json:"webroot"`                 // Webroot path for http method, default: "/var/www/html"
	DNSEnv      map[string]string `json:"dns_env"`                 // DNS API credentials
	AutoRenew   bool              `json:"auto_renew"`              // Auto renew certificate
}

// toCertificateResponse converts a certificate to API response format.
func toCertificateResponse(cert *repository.Certificate) *CertificateResponse {
	daysLeft := int(time.Until(cert.ExpiresAt).Hours() / 24)
	
	status := "valid"
	if daysLeft < 0 {
		status = "expired"
	} else if daysLeft < 30 {
		status = "expiring"
	}
	
	return &CertificateResponse{
		ID:        cert.ID,
		Domain:    cert.Domain,
		AutoRenew: cert.AutoRenew,
		ExpiresAt: cert.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		DaysLeft:  daysLeft,
		Status:    status,
		CreatedAt: cert.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: cert.UpdatedAt.Format("2006-01-02T15:04:05Z"),
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

	// TODO: 验证证书格式和有效性
	// TODO: 从证书中提取过期时间

	cert := &repository.Certificate{
		Domain:      req.Domain,
		Certificate: req.Certificate,
		PrivateKey:  req.PrivateKey,
		AutoRenew:   req.AutoRenew,
		ExpiresAt:   time.Now().AddDate(0, 3, 0), // 默认 3 个月后过期
	}

	if err := h.certRepo.Create(c.Request.Context(), cert); err != nil {
		if errors.IsConflict(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "该域名的证书已存在"})
			return
		}
		h.logger.Error("Failed to create certificate", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建证书失败"})
		return
	}

	h.logger.Info("Certificate created", logger.F("cert_id", cert.ID), logger.F("domain", cert.Domain))

	c.JSON(http.StatusCreated, toCertificateResponse(cert))
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
		middleware.HandleBadRequest(c, errors.MsgInvalidRequest)
		return
	}

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
	}

	// 调用 service 层申请证书
	cert, err := h.certSvc.Apply(c.Request.Context(), applyReq)
	if err != nil {
		h.logger.Error("证书申请失败",
			logger.F("domain", req.Domain),
			logger.Err(err))
		middleware.HandleInternalError(c, err.Error(), err)
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
	// 1. 使用 ACME 续期证书
	// 2. 更新数据库中的证书内容和过期时间

	h.logger.Info("Certificate renewal requested",
		logger.F("cert_id", id),
		logger.F("domain", cert.Domain))

	c.JSON(http.StatusAccepted, gin.H{
		"message": "证书续期已提交，请稍后查看结果",
		"domain":  cert.Domain,
	})
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

	// 更新每个节点的 certificate_id
	successCount := 0
	failedNodes := make([]int64, 0)

	for _, nodeID := range req.NodeIDs {
		// 获取节点
		node, err := h.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			h.logger.Warn("Node not found, skipping",
				logger.F("node_id", nodeID),
				logger.Err(err))
			failedNodes = append(failedNodes, nodeID)
			continue
		}

		// 更新节点的证书 ID
		node.CertificateID = &certID
		if err := h.nodeRepo.Update(ctx, node); err != nil {
			h.logger.Error("Failed to update node certificate",
				logger.F("node_id", nodeID),
				logger.F("cert_id", certID),
				logger.Err(err))
			failedNodes = append(failedNodes, nodeID)
			continue
		}

		successCount++
		h.logger.Info("Certificate assigned to node",
			logger.F("cert_id", certID),
			logger.F("cert_domain", cert.Domain),
			logger.F("node_id", nodeID),
			logger.F("node_name", node.Name))
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
