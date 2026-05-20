// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/internal/monitor"
	"v/internal/node"
)

// NodeGroupHandler handles node group management API requests.
type NodeGroupHandler struct {
	groupService *node.GroupService
	auditService monitor.AuditService
	logger       logger.Logger
}

// NewNodeGroupHandler creates a new node group handler.
func NewNodeGroupHandler(groupService *node.GroupService, log logger.Logger) *NodeGroupHandler {
	return &NodeGroupHandler{
		groupService: groupService,
		logger:       log,
	}
}

// WithAuditService wires the audit emitter for state-changing node group ops.
func (h *NodeGroupHandler) WithAuditService(audit monitor.AuditService) *NodeGroupHandler {
	h.auditService = audit
	return h
}

// NodeGroupResponse represents a node group in API responses.
type NodeGroupResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Region      string `json:"region"`
	Strategy    string `json:"strategy"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// NodeGroupWithStatsResponse includes statistics.
type NodeGroupWithStatsResponse struct {
	NodeGroupResponse
	TotalNodes   int64 `json:"total_nodes"`
	HealthyNodes int64 `json:"healthy_nodes"`
	TotalUsers   int64 `json:"total_users"`
}

// CreateNodeGroupRequest represents a request to create a node group.
type CreateNodeGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Region      string `json:"region"`
	Strategy    string `json:"strategy"`
}

// UpdateNodeGroupRequest represents a request to update a node group.
type UpdateNodeGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Region      *string `json:"region"`
	Strategy    *string `json:"strategy"`
}

// SetNodesRequest represents a request to set nodes for a group.
type SetNodesRequest struct {
	NodeIDs []int64 `json:"node_ids" binding:"required"`
}

// toNodeGroupResponse converts a node group to API response format.
func toNodeGroupResponse(g *node.NodeGroup) *NodeGroupResponse {
	return &NodeGroupResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		Region:      g.Region,
		Strategy:    g.Strategy,
		CreatedAt:   g.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   g.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// List returns all node groups with optional pagination.
// GET /api/admin/node-groups
func (h *NodeGroupHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	groups, total, err := h.groupService.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list node groups", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list node groups"})
		return
	}

	response := make([]*NodeGroupResponse, len(groups))
	for i, g := range groups {
		response[i] = toNodeGroupResponse(g)
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": response,
		"total":  total,
	})
}

// ListWithStats returns all node groups with statistics.
// GET /api/admin/node-groups/with-stats
func (h *NodeGroupHandler) ListWithStats(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	groups, total, err := h.groupService.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list node groups", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list node groups"})
		return
	}

	response := make([]*NodeGroupWithStatsResponse, len(groups))
	for i, g := range groups {
		stats, err := h.groupService.GetStats(c.Request.Context(), g.ID)
		if err != nil {
			h.logger.Error("Failed to get group stats", logger.Err(err), logger.F("group_id", g.ID))
			// Continue with zero stats
			stats = &node.NodeGroupStats{}
		}
		response[i] = &NodeGroupWithStatsResponse{
			NodeGroupResponse: *toNodeGroupResponse(g),
			TotalNodes:        stats.TotalNodes,
			HealthyNodes:      stats.HealthyNodes,
			TotalUsers:        stats.TotalUsers,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": response,
		"total":  total,
	})
}

// Get returns a single node group by ID.
// GET /api/admin/node-groups/:id
func (h *NodeGroupHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	g, err := h.groupService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		h.logger.Error("Failed to get node group", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get node group"})
		return
	}

	c.JSON(http.StatusOK, toNodeGroupResponse(g))
}

// GetWithStats returns a single node group with statistics.
// GET /api/admin/node-groups/:id/stats
func (h *NodeGroupHandler) GetWithStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	g, err := h.groupService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		h.logger.Error("Failed to get node group", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get node group"})
		return
	}

	stats, err := h.groupService.GetStats(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get group stats", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group statistics"})
		return
	}

	c.JSON(http.StatusOK, &NodeGroupWithStatsResponse{
		NodeGroupResponse: *toNodeGroupResponse(g),
		TotalNodes:        stats.TotalNodes,
		HealthyNodes:      stats.HealthyNodes,
		TotalUsers:        stats.TotalUsers,
	})
}

// Create creates a new node group.
// POST /api/admin/node-groups
func (h *NodeGroupHandler) Create(c *gin.Context) {
	var req CreateNodeGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	createReq := &node.CreateGroupRequest{
		Name:        req.Name,
		Description: req.Description,
		Region:      req.Region,
		Strategy:    req.Strategy,
	}

	g, err := h.groupService.Create(c.Request.Context(), createReq)
	if err != nil {
		if err == node.ErrInvalidGroup {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group data"})
			return
		}
		h.logger.Error("Failed to create node group", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create node group"})
		return
	}

	h.logger.Info("Node group created", logger.F("group_id", g.ID), logger.F("name", g.Name))

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeGroupCreate,
		ResourceType: monitor.ResourceNodeGroup,
		ResourceID:   strconv.FormatInt(g.ID, 10),
		Details:      map[string]any{"name": g.Name, "region": g.Region},
	})

	c.JSON(http.StatusCreated, toNodeGroupResponse(g))
}

// Update updates an existing node group.
// PUT /api/admin/node-groups/:id
func (h *NodeGroupHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req UpdateNodeGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	updateReq := &node.UpdateGroupRequest{
		Name:        req.Name,
		Description: req.Description,
		Region:      req.Region,
		Strategy:    req.Strategy,
	}

	g, err := h.groupService.Update(c.Request.Context(), id, updateReq)
	if err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		if err == node.ErrInvalidGroup {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group data"})
			return
		}
		h.logger.Error("Failed to update node group", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node group"})
		return
	}

	h.logger.Info("Node group updated", logger.F("group_id", id))

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeGroupUpdate,
		ResourceType: monitor.ResourceNodeGroup,
		ResourceID:   strconv.FormatInt(id, 10),
		Details:      map[string]any{"name": g.Name},
	})

	c.JSON(http.StatusOK, toNodeGroupResponse(g))
}

// Delete deletes a node group.
// DELETE /api/admin/node-groups/:id
func (h *NodeGroupHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	if err := h.groupService.Delete(c.Request.Context(), id); err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		h.logger.Error("Failed to delete node group", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete node group"})
		return
	}

	h.logger.Info("Node group deleted", logger.F("group_id", id))

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeGroupDelete,
		ResourceType: monitor.ResourceNodeGroup,
		ResourceID:   strconv.FormatInt(id, 10),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Node group deleted successfully"})
}

// GetNodes returns all nodes in a group.
// GET /api/admin/node-groups/:id/nodes
func (h *NodeGroupHandler) GetNodes(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	nodes, err := h.groupService.GetNodes(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		h.logger.Error("Failed to get nodes in group", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get nodes"})
		return
	}

	response := make([]*NodeResponse, len(nodes))
	for i, n := range nodes {
		response[i] = toNodeResponse(n)
	}

	c.JSON(http.StatusOK, gin.H{"nodes": response})
}

// AddNode adds a node to a group.
// POST /api/admin/node-groups/:id/nodes/:node_id
func (h *NodeGroupHandler) AddNode(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	nodeID, err := strconv.ParseInt(c.Param("node_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	if err := h.groupService.AddNode(c.Request.Context(), groupID, nodeID); err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		if err == node.ErrNodeAlreadyInGroup {
			c.JSON(http.StatusConflict, gin.H{"error": "Node is already in this group"})
			return
		}
		h.logger.Error("Failed to add node to group", logger.Err(err),
			logger.F("group_id", groupID), logger.F("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add node to group"})
		return
	}

	h.logger.Info("Node added to group", logger.F("group_id", groupID), logger.F("node_id", nodeID))

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeGroupAddNode,
		ResourceType: monitor.ResourceNodeGroup,
		ResourceID:   strconv.FormatInt(groupID, 10),
		Details:      map[string]any{"node_id": nodeID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Node added to group successfully"})
}

// RemoveNode removes a node from a group.
// DELETE /api/admin/node-groups/:id/nodes/:node_id
func (h *NodeGroupHandler) RemoveNode(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	nodeID, err := strconv.ParseInt(c.Param("node_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	if err := h.groupService.RemoveNode(c.Request.Context(), groupID, nodeID); err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		if err == node.ErrNodeNotInGroup {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node is not in this group"})
			return
		}
		h.logger.Error("Failed to remove node from group", logger.Err(err),
			logger.F("group_id", groupID), logger.F("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove node from group"})
		return
	}

	h.logger.Info("Node removed from group", logger.F("group_id", groupID), logger.F("node_id", nodeID))

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeGroupRemoveNode,
		ResourceType: monitor.ResourceNodeGroup,
		ResourceID:   strconv.FormatInt(groupID, 10),
		Details:      map[string]any{"node_id": nodeID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Node removed from group successfully"})
}

// SetNodes sets the nodes for a group (replaces existing members).
// PUT /api/admin/node-groups/:id/nodes
func (h *NodeGroupHandler) SetNodes(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req SetNodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.groupService.SetNodes(c.Request.Context(), id, req.NodeIDs); err != nil {
		if err == node.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node group not found"})
			return
		}
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "One or more nodes not found"})
			return
		}
		h.logger.Error("Failed to set nodes for group", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set nodes"})
		return
	}

	h.logger.Info("Nodes set for group", logger.F("group_id", id), logger.F("node_count", len(req.NodeIDs)))

	c.JSON(http.StatusOK, gin.H{"message": "Nodes set successfully"})
}

// GetAllStats returns statistics for all groups.
// GET /api/admin/node-groups/stats
func (h *NodeGroupHandler) GetAllStats(c *gin.Context) {
	stats, err := h.groupService.GetAllStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get all group stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}
