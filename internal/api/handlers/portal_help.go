// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
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

type adminHelpArticleRequest struct {
	Slug        string `json:"slug" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Content     string `json:"content" binding:"required"`
	Category    string `json:"category"`
	Tags        string `json:"tags"`
	IsPublished bool   `json:"is_published"`
	IsFeatured  bool   `json:"is_featured"`
}

// AdminListArticles lists all help articles for staff management.
func (h *PortalHelpHandler) AdminListArticles(c *gin.Context) {
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

	filter := &repository.HelpArticleFilter{Limit: limit, Offset: offset}
	if cat := strings.TrimSpace(c.Query("category")); cat != "" {
		filter.Category = &cat
	}
	if pub := c.Query("is_published"); pub == "true" || pub == "false" {
		val := pub == "true"
		filter.IsPublished = &val
	}

	articles, total, err := h.helpService.ListAll(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list admin help articles", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取帮助文章失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"articles": articles, "total": total})
}

// AdminCreateArticle creates a help article.
func (h *PortalHelpHandler) AdminCreateArticle(c *gin.Context) {
	var req adminHelpArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写完整的文章标识、标题和内容"})
		return
	}
	slug := strings.TrimSpace(req.Slug)
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	if slug == "" || title == "" || content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章标识、标题和内容不能为空"})
		return
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = repository.HelpCategoryGettingStarted
	}
	article := &repository.HelpArticle{
		Slug:        slug,
		Title:       title,
		Content:     content,
		Category:    category,
		Tags:        strings.TrimSpace(req.Tags),
		IsPublished: req.IsPublished,
		IsFeatured:  req.IsFeatured,
	}
	if err := h.helpService.Create(c.Request.Context(), article); err != nil {
		h.logger.Error("failed to create help article", logger.F("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建帮助文章失败，请检查 Slug 是否重复"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"article": article})
}

// AdminUpdateArticle updates an existing help article.
func (h *PortalHelpHandler) AdminUpdateArticle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章 ID"})
		return
	}
	var req adminHelpArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写完整的文章标识、标题和内容"})
		return
	}
	article, err := h.helpService.GetArticleByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	article.Slug = strings.TrimSpace(req.Slug)
	article.Title = strings.TrimSpace(req.Title)
	article.Content = strings.TrimSpace(req.Content)
	if article.Slug == "" || article.Title == "" || article.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章标识、标题和内容不能为空"})
		return
	}
	if cat := strings.TrimSpace(req.Category); cat != "" {
		article.Category = cat
	}
	article.Tags = strings.TrimSpace(req.Tags)
	article.IsPublished = req.IsPublished
	article.IsFeatured = req.IsFeatured

	if err := h.helpService.Update(c.Request.Context(), article); err != nil {
		h.logger.Error("failed to update help article", logger.F("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "更新帮助文章失败，请检查 Slug 是否重复"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"article": article})
}

// AdminDeleteArticle deletes a help article.
func (h *PortalHelpHandler) AdminDeleteArticle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章 ID"})
		return
	}
	if err := h.helpService.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("failed to delete help article", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除帮助文章失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "文章已删除"})
}
