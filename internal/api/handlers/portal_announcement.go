// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/portal/announcement"
)

// PortalAnnouncementHandler handles portal announcement requests.
type PortalAnnouncementHandler struct {
	announcementService *announcement.Service
	logger              logger.Logger
}

// NewPortalAnnouncementHandler creates a new PortalAnnouncementHandler.
func NewPortalAnnouncementHandler(announcementService *announcement.Service, log logger.Logger) *PortalAnnouncementHandler {
	return &PortalAnnouncementHandler{
		announcementService: announcementService,
		logger:              log,
	}
}

// ListAnnouncements returns announcements for the current user.
func (h *PortalAnnouncementHandler) ListAnnouncements(c *gin.Context) {
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

	announcements, total, err := h.announcementService.ListAnnouncements(c.Request.Context(), userID.(int64), limit, offset)
	if err != nil {
		h.logger.Error("failed to list announcements", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取公告列表失败"})
		return
	}

	// Get unread count
	unreadCount, _ := h.announcementService.GetUnreadCount(c.Request.Context(), userID.(int64))

	c.JSON(http.StatusOK, gin.H{
		"announcements": announcements,
		"total":         total,
		"unread_count":  unreadCount,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetAnnouncement returns a single announcement by ID.
func (h *PortalAnnouncementHandler) GetAnnouncement(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公告ID"})
		return
	}

	ann, err := h.announcementService.GetAnnouncement(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公告不存在"})
		return
	}

	// Check if read
	isRead, _ := h.announcementService.IsRead(c.Request.Context(), userID.(int64), id)

	c.JSON(http.StatusOK, gin.H{
		"announcement": ann,
		"is_read":      isRead,
	})
}

// MarkAsRead marks an announcement as read.
func (h *PortalAnnouncementHandler) MarkAsRead(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公告ID"})
		return
	}

	if err := h.announcementService.MarkAsRead(c.Request.Context(), userID.(int64), id); err != nil {
		h.logger.Error("failed to mark announcement as read", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "标记已读失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已标记为已读"})
}

// GetUnreadCount returns the count of unread announcements.
func (h *PortalAnnouncementHandler) GetUnreadCount(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	count, err := h.announcementService.GetUnreadCount(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("failed to get unread count", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取未读数量失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

type adminAnnouncementRequest struct {
	Title       string `json:"title" binding:"required"`
	Content     string `json:"content" binding:"required"`
	Category    string `json:"category"`
	IsPinned    bool   `json:"is_pinned"`
	IsPublished bool   `json:"is_published"`
}

// AdminListAnnouncements lists all announcements for staff management.
func (h *PortalAnnouncementHandler) AdminListAnnouncements(c *gin.Context) {
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

	filter := &repository.AnnouncementFilter{Limit: limit, Offset: offset}
	if cat := strings.TrimSpace(c.Query("category")); cat != "" {
		filter.Category = &cat
	}
	if pub := c.Query("is_published"); pub == "true" || pub == "false" {
		val := pub == "true"
		filter.IsPublished = &val
	}

	items, total, err := h.announcementService.ListAll(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list admin announcements", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取公告列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"announcements": items, "total": total})
}

// AdminCreateAnnouncement creates a new announcement.
func (h *PortalAnnouncementHandler) AdminCreateAnnouncement(c *gin.Context) {
	var req adminAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写完整的公告标题和内容"})
		return
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = repository.AnnouncementCategoryGeneral
	}
	ann := &repository.Announcement{
		Title:       strings.TrimSpace(req.Title),
		Content:     strings.TrimSpace(req.Content),
		Category:    category,
		IsPinned:    req.IsPinned,
		IsPublished: req.IsPublished,
	}
	if ann.Title == "" || ann.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公告标题和内容不能为空"})
		return
	}
	if ann.IsPublished {
		now := time.Now().UTC()
		ann.PublishedAt = &now
	}
	if err := h.announcementService.Create(c.Request.Context(), ann); err != nil {
		h.logger.Error("failed to create announcement", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建公告失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"announcement": ann})
}

// AdminUpdateAnnouncement updates an existing announcement.
func (h *PortalAnnouncementHandler) AdminUpdateAnnouncement(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公告 ID"})
		return
	}
	var req adminAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写完整的公告标题和内容"})
		return
	}
	ann, err := h.announcementService.GetAnnouncement(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公告不存在"})
		return
	}
	ann.Title = strings.TrimSpace(req.Title)
	ann.Content = strings.TrimSpace(req.Content)
	if ann.Title == "" || ann.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公告标题和内容不能为空"})
		return
	}
	if cat := strings.TrimSpace(req.Category); cat != "" {
		ann.Category = cat
	}
	ann.IsPinned = req.IsPinned
	ann.IsPublished = req.IsPublished
	if ann.IsPublished && ann.PublishedAt == nil {
		now := time.Now().UTC()
		ann.PublishedAt = &now
	}
	if err := h.announcementService.Update(c.Request.Context(), ann); err != nil {
		h.logger.Error("failed to update announcement", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新公告失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"announcement": ann})
}

// AdminDeleteAnnouncement deletes an announcement.
func (h *PortalAnnouncementHandler) AdminDeleteAnnouncement(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公告 ID"})
		return
	}
	if err := h.announcementService.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("failed to delete announcement", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除公告失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "公告已删除"})
}
