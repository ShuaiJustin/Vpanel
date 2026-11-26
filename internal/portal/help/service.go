// Package help provides help center services for the user portal.
package help

import (
	"context"
	"strings"

	"v/internal/database/repository"
)

// Service provides help article operations for the user portal.
type Service struct {
	helpRepo repository.HelpArticleRepository
}

// NewService creates a new help service.
func NewService(helpRepo repository.HelpArticleRepository) *Service {
	return &Service{
		helpRepo: helpRepo,
	}
}

// ListArticles retrieves published help articles.
func (s *Service) ListArticles(ctx context.Context, limit, offset int) ([]*repository.HelpArticle, int64, error) {
	return s.helpRepo.ListPublished(ctx, limit, offset)
}

// ListByCategory retrieves help articles by category.
func (s *Service) ListByCategory(ctx context.Context, category string, limit, offset int) ([]*repository.HelpArticle, int64, error) {
	return s.helpRepo.ListByCategory(ctx, category, limit, offset)
}

// GetArticle retrieves a help article by slug and increments view count.
func (s *Service) GetArticle(ctx context.Context, slug string) (*repository.HelpArticle, error) {
	article, err := s.helpRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Only count views for published articles
	if article.IsPublished {
		// Increment view count asynchronously (ignore errors)
		go func() {
			_ = s.helpRepo.IncrementViewCount(context.Background(), article.ID)
		}()
	}

	return article, nil
}

// GetArticleByID retrieves a help article by ID.
func (s *Service) GetArticleByID(ctx context.Context, id int64) (*repository.HelpArticle, error) {
	return s.helpRepo.GetByID(ctx, id)
}

// Search searches help articles by query string.
// The search looks for matches in title, content, and tags.
func (s *Service) Search(ctx context.Context, query string, limit, offset int) ([]*repository.HelpArticle, int64, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.ListArticles(ctx, limit, offset)
	}
	return s.helpRepo.Search(ctx, query, limit, offset)
}

// GetFeaturedArticles retrieves featured help articles.
func (s *Service) GetFeaturedArticles(ctx context.Context, limit int) ([]*repository.HelpArticle, error) {
	return s.helpRepo.GetFeatured(ctx, limit)
}

// GetCategories retrieves all unique categories.
func (s *Service) GetCategories(ctx context.Context) ([]string, error) {
	return s.helpRepo.GetCategories(ctx)
}

// MarkHelpful increments the helpful count for an article.
func (s *Service) MarkHelpful(ctx context.Context, id int64) error {
	return s.helpRepo.IncrementHelpfulCount(ctx, id)
}

// SearchResult represents a search result with relevance information.
type SearchResult struct {
	*repository.HelpArticle
	MatchedIn []string `json:"matched_in"` // "title", "content", "tags"
}

// SearchWithRelevance searches articles and returns results with match information.
func (s *Service) SearchWithRelevance(ctx context.Context, query string, limit, offset int) ([]*SearchResult, int64, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		articles, total, err := s.ListArticles(ctx, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		results := make([]*SearchResult, len(articles))
		for i, a := range articles {
			results[i] = &SearchResult{HelpArticle: a, MatchedIn: []string{}}
		}
		return results, total, nil
	}

	articles, total, err := s.helpRepo.Search(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	queryLower := strings.ToLower(query)
	results := make([]*SearchResult, len(articles))
	for i, a := range articles {
		matchedIn := []string{}
		if strings.Contains(strings.ToLower(a.Title), queryLower) {
			matchedIn = append(matchedIn, "title")
		}
		if strings.Contains(strings.ToLower(a.Content), queryLower) {
			matchedIn = append(matchedIn, "content")
		}
		if strings.Contains(strings.ToLower(a.Tags), queryLower) {
			matchedIn = append(matchedIn, "tags")
		}
		results[i] = &SearchResult{HelpArticle: a, MatchedIn: matchedIn}
	}

	return results, total, nil
}

// ContainsSearchTerm checks if an article contains the search term in title, content, or tags.
func ContainsSearchTerm(article *repository.HelpArticle, query string) bool {
	if article == nil || query == "" {
		return false
	}
	queryLower := strings.ToLower(query)
	return strings.Contains(strings.ToLower(article.Title), queryLower) ||
		strings.Contains(strings.ToLower(article.Content), queryLower) ||
		strings.Contains(strings.ToLower(article.Tags), queryLower)
}

// ListAll retrieves help articles for admin management.
func (s *Service) ListAll(ctx context.Context, filter *repository.HelpArticleFilter) ([]*repository.HelpArticle, int64, error) {
	return s.helpRepo.List(ctx, filter)
}

// Create creates a help article.
func (s *Service) Create(ctx context.Context, article *repository.HelpArticle) error {
	return s.helpRepo.Create(ctx, article)
}

// Update updates a help article.
func (s *Service) Update(ctx context.Context, article *repository.HelpArticle) error {
	return s.helpRepo.Update(ctx, article)
}

// Delete deletes a help article.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.helpRepo.Delete(ctx, id)
}
