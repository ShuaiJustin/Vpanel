package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/proxy"
	"v/pkg/errors"
)

type ProxyHandler struct {
	proxyManager    proxy.Manager
	proxyRepo       repository.ProxyRepository
	nodeRepo        repository.NodeRepository
	trafficRepo     repository.TrafficRepository
	recoveryTracker *NodeRecoveryTracker
	logger          logger.Logger
}

func NewProxyHandler(proxyManager proxy.Manager, proxyRepo repository.ProxyRepository, log logger.Logger) *ProxyHandler {
	return &ProxyHandler{
		proxyManager: proxyManager,
		proxyRepo:    proxyRepo,
		logger:       log,
	}
}

// NewProxyHandlerWithTraffic creates a new proxy handler with traffic repository.
func NewProxyHandlerWithTraffic(proxyManager proxy.Manager, proxyRepo repository.ProxyRepository, trafficRepo repository.TrafficRepository, log logger.Logger) *ProxyHandler {
	return &ProxyHandler{
		proxyManager: proxyManager,
		proxyRepo:    proxyRepo,
		trafficRepo:  trafficRepo,
		logger:       log,
	}
}

// WithNodeRepository injects node repository for node-aware share link resolution.
func (h *ProxyHandler) WithNodeRepository(nodeRepo repository.NodeRepository) *ProxyHandler {
	h.nodeRepo = nodeRepo
	return h
}

// WithRecoveryTracker injects recovery tracker for node config sync commands.
func (h *ProxyHandler) WithRecoveryTracker(recoveryTracker *NodeRecoveryTracker) *ProxyHandler {
	h.recoveryTracker = recoveryTracker
	return h
}

func (h *ProxyHandler) queueNodeConfigSync(nodeID *int64, source, reason string) {
	if h == nil || h.recoveryTracker == nil || nodeID == nil || *nodeID <= 0 {
		return
	}
	h.recoveryTracker.QueueConfigSyncCommand(*nodeID, source, reason)
}

type ProxyResponse struct {
	ID        int64          `json:"id"`
	UserID    int64          `json:"user_id"`
	NodeID    *int64         `json:"node_id,omitempty"` // 节点 ID
	Name      string         `json:"name"`
	Protocol  string         `json:"protocol"`
	Port      int            `json:"port"`
	Host      string         `json:"host,omitempty"`
	Settings  map[string]any `json:"settings,omitempty"`
	Enabled   bool           `json:"enabled"`
	Remark    string         `json:"remark,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

// getUserFromContext extracts user information from the gin context.
func getUserFromContext(c *gin.Context) (userID int64, role string, isAdmin bool) {
	if id, exists := c.Get("user_id"); exists {
		userID = id.(int64)
	}
	if r, exists := c.Get("role"); exists {
		role = r.(string)
	}
	isAdmin = role == "admin"
	return
}

// canAccessProxy checks if the current user can access the given proxy.
func (h *ProxyHandler) canAccessProxy(c *gin.Context, proxy *repository.Proxy) bool {
	userID, _, isAdmin := getUserFromContext(c)
	return isAdmin || proxy.UserID == userID
}

// List returns proxies based on user role.
// Admin users can see all proxies, regular users can only see their own.
func (h *ProxyHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	userID, _, isAdmin := getUserFromContext(c)

	var proxies []*repository.Proxy
	var err error

	if isAdmin {
		// Admin can see all proxies
		proxies, err = h.proxyRepo.List(c.Request.Context(), limit, offset)
	} else {
		// Regular users can only see their own proxies
		proxies, err = h.proxyRepo.GetByUserID(c.Request.Context(), userID, limit, offset)
	}

	if err != nil {
		h.logger.Error("failed to list proxies", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list proxies"})
		return
	}

	response := make([]ProxyResponse, len(proxies))
	for i, p := range proxies {
		response[i] = ProxyResponse{
			ID:        p.ID,
			UserID:    p.UserID,
			NodeID:    p.NodeID,
			Name:      p.Name,
			Protocol:  p.Protocol,
			Port:      p.Port,
			Host:      p.Host,
			Settings:  p.Settings,
			Enabled:   p.Enabled,
			Remark:    p.Remark,
			CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, response)
}

type CreateProxyRequest struct {
	Name     string         `json:"name" binding:"required"`
	Protocol string         `json:"protocol" binding:"required"`
	Port     int            `json:"port" binding:"required,min=1,max=65535"`
	Host     string         `json:"host"`
	NodeID   *int64         `json:"node_id"` // 节点 ID
	Settings map[string]any `json:"settings"`
	Enabled  bool           `json:"enabled"`
	Remark   string         `json:"remark"`
}

// Create creates a new proxy for the authenticated user.
func (h *ProxyHandler) Create(c *gin.Context) {
	var req CreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userID, _, _ := getUserFromContext(c)

	protocol, ok := h.proxyManager.GetProtocol(req.Protocol)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported protocol"})
		return
	}

	settings := &proxy.Settings{
		Name:     req.Name,
		Protocol: req.Protocol,
		Port:     req.Port,
		Host:     req.Host,
		Settings: req.Settings,
		Enabled:  req.Enabled,
		Remark:   req.Remark,
	}

	if settings.Settings == nil {
		settings.Settings = protocol.DefaultSettings()
	}

	normalizedSettings, err := proxy.NormalizeSettings(req.Protocol, settings.Settings)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings.Settings = normalizedSettings

	if err := protocol.Validate(settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check for port conflict
	existingProxy, err := h.proxyRepo.GetByPort(c.Request.Context(), req.Port)
	if err != nil {
		h.logger.Error("failed to check port conflict", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check port availability"})
		return
	}
	if existingProxy != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Port is already in use",
			"details": gin.H{
				"conflicting_proxy_id":   existingProxy.ID,
				"conflicting_proxy_name": existingProxy.Name,
				"port":                   req.Port,
			},
		})
		return
	}

	proxyModel := &repository.Proxy{
		UserID:   userID,
		NodeID:   req.NodeID, // 设置节点 ID
		Name:     req.Name,
		Protocol: req.Protocol,
		Port:     req.Port,
		Host:     req.Host,
		Settings: settings.Settings,
		Enabled:  req.Enabled,
		Remark:   req.Remark,
	}

	if err := h.proxyRepo.Create(c.Request.Context(), proxyModel); err != nil {
		h.logger.Error("failed to create proxy", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create proxy"})
		return
	}

	h.logger.Info("proxy created", logger.F("proxy_id", proxyModel.ID), logger.F("user_id", userID))
	h.queueNodeConfigSync(proxyModel.NodeID, "proxy_create", "proxy created")

	c.JSON(http.StatusCreated, ProxyResponse{
		ID:        proxyModel.ID,
		UserID:    proxyModel.UserID,
		NodeID:    proxyModel.NodeID,
		Name:      proxyModel.Name,
		Protocol:  proxyModel.Protocol,
		Port:      proxyModel.Port,
		Host:      proxyModel.Host,
		Settings:  proxyModel.Settings,
		Enabled:   proxyModel.Enabled,
		Remark:    proxyModel.Remark,
		CreatedAt: proxyModel.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: proxyModel.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Get retrieves a proxy by ID.
// Users can only access their own proxies unless they are admin.
func (h *ProxyHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, ProxyResponse{
		ID:        p.ID,
		UserID:    p.UserID,
		NodeID:    p.NodeID,
		Name:      p.Name,
		Protocol:  p.Protocol,
		Port:      p.Port,
		Host:      p.Host,
		Settings:  p.Settings,
		Enabled:   p.Enabled,
		Remark:    p.Remark,
		CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

type UpdateProxyRequest struct {
	Name     string         `json:"name"`
	Port     int            `json:"port"`
	Host     string         `json:"host"`
	NodeID   *int64         `json:"node_id"` // 节点 ID
	Settings map[string]any `json:"settings"`
	Enabled  *bool          `json:"enabled"`
	Remark   string         `json:"remark"`
}

// Update updates a proxy.
// Users can only update their own proxies unless they are admin.
func (h *ProxyHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	var req UpdateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check for port conflict if port is being changed
	if req.Port > 0 && req.Port != p.Port {
		existingProxy, err := h.proxyRepo.GetByPort(c.Request.Context(), req.Port)
		if err != nil {
			h.logger.Error("failed to check port conflict", logger.F("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check port availability"})
			return
		}
		if existingProxy != nil && existingProxy.ID != id {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Port is already in use",
				"details": gin.H{
					"conflicting_proxy_id":   existingProxy.ID,
					"conflicting_proxy_name": existingProxy.Name,
					"port":                   req.Port,
				},
			})
			return
		}
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Port > 0 {
		p.Port = req.Port
	}
	if req.Host != "" {
		p.Host = req.Host
	}
	if req.NodeID != nil {
		p.NodeID = req.NodeID
	}
	if req.Settings != nil {
		normalizedSettings, err := proxy.NormalizeSettings(p.Protocol, req.Settings)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p.Settings = normalizedSettings
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	if req.Remark != "" {
		p.Remark = req.Remark
	}

	if err := h.proxyRepo.Update(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update proxy"})
		return
	}
	h.queueNodeConfigSync(p.NodeID, "proxy_update", "proxy updated")

	c.JSON(http.StatusOK, ProxyResponse{
		ID:        p.ID,
		UserID:    p.UserID,
		NodeID:    p.NodeID,
		Name:      p.Name,
		Protocol:  p.Protocol,
		Port:      p.Port,
		Host:      p.Host,
		Settings:  p.Settings,
		Enabled:   p.Enabled,
		Remark:    p.Remark,
		CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Delete deletes a proxy.
// Users can only delete their own proxies unless they are admin.
func (h *ProxyHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.proxyRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete proxy"})
		return
	}
	h.queueNodeConfigSync(p.NodeID, "proxy_delete", "proxy deleted")

	userID, _, _ := getUserFromContext(c)
	h.logger.Info("proxy deleted", logger.F("proxy_id", id), logger.F("user_id", userID))

	c.JSON(http.StatusOK, gin.H{"message": "Proxy deleted successfully"})
}

// GetShareLink generates a share link for a proxy.
func (h *ProxyHandler) GetShareLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	protocol, ok := h.proxyManager.GetProtocol(p.Protocol)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported protocol"})
		return
	}

	settingsMap := make(map[string]any, len(p.Settings)+1)
	for key, value := range p.Settings {
		settingsMap[key] = value
	}
	if normalizedSettings, normalizeErr := proxy.NormalizeSettings(p.Protocol, settingsMap); normalizeErr == nil {
		settingsMap = normalizedSettings
	}

	resolvedServer := ""
	if explicitServer, ok := settingsMap["server"].(string); ok {
		resolvedServer = proxy.NormalizeShareHost(explicitServer)
	}
	if resolvedServer == "" && p.NodeID != nil && h.nodeRepo != nil {
		node, nodeErr := h.nodeRepo.GetByID(c.Request.Context(), *p.NodeID)
		if nodeErr == nil {
			resolvedServer = proxy.NormalizeShareHost(node.Address)
		}
	}
	if resolvedServer == "" {
		resolvedServer = proxy.ResolveServerAddress(p.Host, settingsMap)
	}
	if resolvedServer == "" && p.NodeID == nil {
		if forwardedHost := proxy.NormalizeShareHost(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
			resolvedServer = forwardedHost
		} else if requestHost := proxy.NormalizeShareHost(c.Request.Host); requestHost != "" {
			resolvedServer = requestHost
		}
	}
	if resolvedServer != "" {
		settingsMap["server"] = resolvedServer
	}

	settings := &proxy.Settings{
		ID:       p.ID,
		Name:     p.Name,
		Protocol: p.Protocol,
		Port:     p.Port,
		Host:     p.Host,
		Settings: settingsMap,
		Enabled:  p.Enabled,
		Remark:   p.Remark,
	}

	link, err := protocol.GenerateLink(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate share link"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"link": link})
}

// Toggle toggles the enabled status of a proxy.
func (h *ProxyHandler) Toggle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	p.Enabled = !p.Enabled

	if err := h.proxyRepo.Update(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle proxy"})
		return
	}
	h.queueNodeConfigSync(p.NodeID, "proxy_toggle", "proxy toggled")

	c.JSON(http.StatusOK, gin.H{"enabled": p.Enabled})
}

// Start starts a proxy (enables it).
func (h *ProxyHandler) Start(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if p.Enabled {
		c.JSON(http.StatusOK, gin.H{"message": "Proxy is already running"})
		return
	}

	p.Enabled = true
	if err := h.proxyRepo.Update(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start proxy"})
		return
	}
	h.queueNodeConfigSync(p.NodeID, "proxy_start", "proxy started")

	userID, _, _ := getUserFromContext(c)
	h.logger.Info("proxy started", logger.F("proxy_id", id), logger.F("user_id", userID))

	c.JSON(http.StatusOK, gin.H{"message": "Proxy started successfully"})
}

// Stop stops a proxy (disables it).
func (h *ProxyHandler) Stop(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if !p.Enabled {
		c.JSON(http.StatusOK, gin.H{"message": "Proxy is already stopped"})
		return
	}

	p.Enabled = false
	if err := h.proxyRepo.Update(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop proxy"})
		return
	}
	h.queueNodeConfigSync(p.NodeID, "proxy_stop", "proxy stopped")

	userID, _, _ := getUserFromContext(c)
	h.logger.Info("proxy stopped", logger.F("proxy_id", id), logger.F("user_id", userID))

	c.JSON(http.StatusOK, gin.H{"message": "Proxy stopped successfully"})
}

// GetStats returns statistics for a proxy.
func (h *ProxyHandler) GetStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
		return
	}

	// Check access permission
	if !h.canAccessProxy(c, p) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get traffic statistics from traffic repository
	var upload, download int64
	if h.trafficRepo != nil {
		upload, download, err = h.trafficRepo.GetTotalByProxy(c.Request.Context(), id)
		if err != nil {
			h.logger.Error("failed to get proxy traffic stats", logger.F("error", err), logger.F("proxy_id", id))
			// Continue with zero values instead of failing
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"upload":           upload,
		"download":         download,
		"total":            upload + download,
		"connection_count": 0, // TODO: Implement connection tracking
		"last_active":      p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// BatchOperation represents a batch operation request.
type BatchOperationRequest struct {
	IDs       []int64 `json:"ids" binding:"required"`
	Operation string  `json:"operation" binding:"required,oneof=enable disable delete"`
}

// BatchOperation performs batch operations on proxies.
func (h *ProxyHandler) BatchOperation(c *gin.Context) {
	var req BatchOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userID, _, isAdmin := getUserFromContext(c)

	// Verify access to all proxies
	for _, id := range req.IDs {
		p, err := h.proxyRepo.GetByID(c.Request.Context(), id)
		if err != nil {
			if errors.IsNotFound(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found", "proxy_id": id})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy"})
			return
		}
		if !isAdmin && p.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied", "proxy_id": id})
			return
		}
	}

	var err error
	switch req.Operation {
	case "enable":
		for _, id := range req.IDs {
			p, _ := h.proxyRepo.GetByID(c.Request.Context(), id)
			p.Enabled = true
			if err = h.proxyRepo.Update(c.Request.Context(), p); err != nil {
				break
			}
		}
	case "disable":
		for _, id := range req.IDs {
			p, _ := h.proxyRepo.GetByID(c.Request.Context(), id)
			p.Enabled = false
			if err = h.proxyRepo.Update(c.Request.Context(), p); err != nil {
				break
			}
		}
	case "delete":
		err = h.proxyRepo.DeleteByIDs(c.Request.Context(), req.IDs)
	}

	if err != nil {
		h.logger.Error("batch operation failed", logger.F("error", err), logger.F("operation", req.Operation))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Batch operation failed"})
		return
	}

	h.logger.Info("batch operation completed",
		logger.F("operation", req.Operation),
		logger.F("count", len(req.IDs)),
		logger.F("user_id", userID))

	c.JSON(http.StatusOK, gin.H{
		"message": "Batch operation completed successfully",
		"count":   len(req.IDs),
	})
}
