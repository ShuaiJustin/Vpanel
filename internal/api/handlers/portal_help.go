// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/internal/portal/help"
)

// PortalHelpHandler handles portal help center requests.
type PortalHelpHandler struct {
	helpService *help.Service
	logger      logger.Logger
}

// NewPortalHelpHandler creates a new PortalHelpHandler.
func NewPortalHelpHandler(helpService *help.Service, log logger.Logger) *PortalHelpHandler {
	return &PortalHelpHandler{
		helpService: helpService,
		logger:      log,
	}
}

// ListArticles returns help articles.
func (h *PortalHelpHandler) ListArticles(c *gin.Context) {
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

	// Check for category filter
	category := c.Query("category")

	var articles interface{}
	var total int64
	var err error

	if category != "" {
		articles, total, err = h.helpService.ListByCategory(c.Request.Context(), category, limit, offset)
	} else {
		articles, total, err = h.helpService.ListArticles(c.Request.Context(), limit, offset)
	}

	if err != nil {
		h.logger.Error("failed to list articles", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文章列表失败"})
		return
	}

	// Get categories for filtering
	categories, _ := h.helpService.GetCategories(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"articles":   articles,
		"total":      total,
		"categories": categories,
		"limit":      limit,
		"offset":     offset,
	})
}

// GetArticle returns a single article by slug.
func (h *PortalHelpHandler) GetArticle(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章标识不能为空"})
		return
	}

	article, err := h.helpService.GetArticle(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	c.JSON(http.StatusOK, article)
}

// SearchArticles searches help articles.
func (h *PortalHelpHandler) SearchArticles(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		// Return all articles if no query
		h.ListArticles(c)
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

	results, total, err := h.helpService.SearchWithRelevance(c.Request.Context(), query, limit, offset)
	if err != nil {
		h.logger.Error("failed to search articles", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索文章失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results":  results,
		"articles": results,
		"total":    total,
		"query":    query,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetFeaturedArticles returns featured help articles.
func (h *PortalHelpHandler) GetFeaturedArticles(c *gin.Context) {
	limit := 5
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 20 {
			limit = parsed
		}
	}

	articles, err := h.helpService.GetFeaturedArticles(c.Request.Context(), limit)
	if err != nil {
		h.logger.Error("failed to get featured articles", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取推荐文章失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"articles": articles,
	})
}

// MarkHelpful marks an article as helpful.
func (h *PortalHelpHandler) MarkHelpful(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章标识不能为空"})
		return
	}

	// Get article by slug first to get ID
	article, err := h.helpService.GetArticle(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	if err := h.helpService.MarkHelpful(c.Request.Context(), article.ID); err != nil {
		h.logger.Error("failed to mark article as helpful", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "感谢您的反馈"})
}

// GetCategories returns all article categories.
func (h *PortalHelpHandler) GetCategories(c *gin.Context) {
	categories, err := h.helpService.GetCategories(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get categories", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分类失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}
