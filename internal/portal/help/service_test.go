// Package help provides help center services for the user portal.
package help

import (
	"context"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/database/repository"
)

// mockHelpArticleRepo is a mock implementation of HelpArticleRepository for testing.
type mockHelpArticleRepo struct {
	articles map[int64]*repository.HelpArticle
	slugMap  map[string]int64
	nextID   int64
}

func newMockHelpArticleRepo() *mockHelpArticleRepo {
	return &mockHelpArticleRepo{
		articles: make(map[int64]*repository.HelpArticle),
		slugMap:  make(map[string]int64),
		nextID:   1,
	}
}

func (m *mockHelpArticleRepo) Create(ctx context.Context, article *repository.HelpArticle) error {
	article.ID = m.nextID
	m.nextID++
	m.articles[article.ID] = article
	m.slugMap[article.Slug] = article.ID
	return nil
}

func (m *mockHelpArticleRepo) GetByID(ctx context.Context, id int64) (*repository.HelpArticle, error) {
	if a, ok := m.articles[id]; ok {
		return a, nil
	}
	return nil, &notFoundError{id: id}
}

func (m *mockHelpArticleRepo) GetBySlug(ctx context.Context, slug string) (*repository.HelpArticle, error) {
	if id, ok := m.slugMap[slug]; ok {
		return m.articles[id], nil
	}
	return nil, &notFoundError{slug: slug}
}

func (m *mockHelpArticleRepo) Update(ctx context.Context, article *repository.HelpArticle) error {
	m.articles[article.ID] = article
	return nil
}

func (m *mockHelpArticleRepo) Delete(ctx context.Context, id int64) error {
	if a, ok := m.articles[id]; ok {
		delete(m.slugMap, a.Slug)
		delete(m.articles, id)
	}
	return nil
}

func (m *mockHelpArticleRepo) List(ctx context.Context, filter *repository.HelpArticleFilter) ([]*repository.HelpArticle, int64, error) {
	var results []*repository.HelpArticle
	for _, a := range m.articles {
		if filter != nil && filter.IsPublished != nil && *filter.IsPublished != a.IsPublished {
			continue
		}
		if filter != nil && filter.Category != nil && *filter.Category != a.Category {
			continue
		}
		results = append(results, a)
	}
	return results, int64(len(results)), nil
}

func (m *mockHelpArticleRepo) ListPublished(ctx context.Context, limit, offset int) ([]*repository.HelpArticle, int64, error) {
	isPublished := true
	return m.List(ctx, &repository.HelpArticleFilter{IsPublished: &isPublished, Limit: limit, Offset: offset})
}

func (m *mockHelpArticleRepo) ListByCategory(ctx context.Context, category string, limit, offset int) ([]*repository.HelpArticle, int64, error) {
	isPublished := true
	return m.List(ctx, &repository.HelpArticleFilter{Category: &category, IsPublished: &isPublished, Limit: limit, Offset: offset})
}

func (m *mockHelpArticleRepo) Search(ctx context.Context, query string, limit, offset int) ([]*repository.HelpArticle, int64, error) {
	var results []*repository.HelpArticle
	queryLower := strings.ToLower(query)
	for _, a := range m.articles {
		if !a.IsPublished {
			continue
		}
		if strings.Contains(strings.ToLower(a.Title), queryLower) ||
			strings.Contains(strings.ToLower(a.Content), queryLower) ||
			strings.Contains(strings.ToLower(a.Tags), queryLower) {
			results = append(results, a)
		}
	}
	return results, int64(len(results)), nil
}

func (m *mockHelpArticleRepo) GetFeatured(ctx context.Context, limit int) ([]*repository.HelpArticle, error) {
	var results []*repository.HelpArticle
	for _, a := range m.articles {
		if a.IsPublished && a.IsFeatured {
			results = append(results, a)
		}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (m *mockHelpArticleRepo) IncrementViewCount(ctx context.Context, id int64) error {
	if a, ok := m.articles[id]; ok {
		a.ViewCount++
	}
	return nil
}

func (m *mockHelpArticleRepo) IncrementHelpfulCount(ctx context.Context, id int64) error {
	if a, ok := m.articles[id]; ok {
		a.HelpfulCount++
	}
	return nil
}

func (m *mockHelpArticleRepo) GetCategories(ctx context.Context) ([]string, error) {
	categorySet := make(map[string]bool)
	for _, a := range m.articles {
		if a.IsPublished && a.Category != "" {
			categorySet[a.Category] = true
		}
	}
	var categories []string
	for c := range categorySet {
		categories = append(categories, c)
	}
	return categories, nil
}

type notFoundError struct {
	id   int64
	slug string
}

func (e *notFoundError) Error() string {
	return "not found"
}

// Unit tests

func TestService_Search(t *testing.T) {
	repo := newMockHelpArticleRepo()
	service := NewService(repo)
	ctx := context.Background()

	// Create some articles
	articles := []*repository.HelpArticle{
		{Slug: "getting-started", Title: "Getting Started Guide", Content: "How to get started", Tags: "beginner,guide", IsPublished: true},
		{Slug: "advanced-config", Title: "Advanced Configuration", Content: "Advanced settings", Tags: "advanced,config", IsPublished: true},
		{Slug: "troubleshooting", Title: "Troubleshooting", Content: "Common problems and solutions", Tags: "help,problems", IsPublished: true},
		{Slug: "draft-article", Title: "Draft Article", Content: "This is a draft", Tags: "draft", IsPublished: false},
	}

	for _, a := range articles {
		repo.Create(ctx, a)
	}

	// Search for "started"
	results, total, err := service.Search(ctx, "started", 10, 0)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 result, got %d", total)
	}
	if len(results) != 1 || results[0].Slug != "getting-started" {
		t.Error("Expected to find 'getting-started' article")
	}

	// Search for "advanced"
	results, total, err = service.Search(ctx, "advanced", 10, 0)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 result, got %d", total)
	}

	// Search for "draft" should not return unpublished article
	results, total, err = service.Search(ctx, "draft", 10, 0)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if total != 0 {
		t.Errorf("Expected 0 results for unpublished article, got %d", total)
	}
}

func TestContainsSearchTerm(t *testing.T) {
	article := &repository.HelpArticle{
		Title:   "Getting Started Guide",
		Content: "How to configure your client",
		Tags:    "beginner,setup",
	}

	tests := []struct {
		query    string
		expected bool
	}{
		{"getting", true},
		{"GETTING", true}, // case insensitive
		{"configure", true},
		{"beginner", true},
		{"setup", true},
		{"notfound", false},
		{"", false},
	}

	for _, tt := range tests {
		result := ContainsSearchTerm(article, tt.query)
		if result != tt.expected {
			t.Errorf("ContainsSearchTerm(%q) = %v, expected %v", tt.query, result, tt.expected)
		}
	}

	// Test nil article
	if ContainsSearchTerm(nil, "test") {
		t.Error("Expected false for nil article")
	}
}

// Feature: user-portal, Property 14: Help Article Search Relevance
// Validates: Requirements 12.3
// *For any* search query, returned articles SHALL contain the search terms
// in their title, content, or tags.
func TestProperty_HelpArticleSearchRelevance(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: All search results contain the search term
	properties.Property("all search results contain the search term", prop.ForAll(
		func(seed int64) bool {
			repo := newMockHelpArticleRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create articles with various content
			searchTerms := []string{"guide", "setup", "config", "help", "tutorial"}
			searchTerm := searchTerms[int(seed)%len(searchTerms)]

			// Create some articles that contain the term
			for i := 0; i < 3; i++ {
				article := &repository.HelpArticle{
					Slug:        "article-" + string(rune('a'+i)),
					Title:       "Article " + searchTerm,
					Content:     "Content about " + searchTerm,
					Tags:        searchTerm,
					IsPublished: true,
				}
				repo.Create(ctx, article)
			}

			// Create some articles that don't contain the term
			for i := 0; i < 2; i++ {
				article := &repository.HelpArticle{
					Slug:        "other-" + string(rune('a'+i)),
					Title:       "Other Article",
					Content:     "Different content",
					Tags:        "other",
					IsPublished: true,
				}
				repo.Create(ctx, article)
			}

			// Search for the term
			results, _, err := service.Search(ctx, searchTerm, 100, 0)
			if err != nil {
				return false
			}

			// All results should contain the search term
			for _, article := range results {
				if !ContainsSearchTerm(article, searchTerm) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
	))

	// Property: Search is case-insensitive
	properties.Property("search is case-insensitive", prop.ForAll(
		func(seed int64) bool {
			repo := newMockHelpArticleRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create an article
			article := &repository.HelpArticle{
				Slug:        "test-article",
				Title:       "Getting Started Guide",
				Content:     "How to get started",
				Tags:        "beginner",
				IsPublished: true,
			}
			repo.Create(ctx, article)

			// Search with different cases
			queries := []string{"getting", "GETTING", "Getting", "gEtTiNg"}
			query := queries[int(seed)%len(queries)]

			results, _, err := service.Search(ctx, query, 10, 0)
			if err != nil {
				return false
			}

			return len(results) == 1
		},
		gen.Int64Range(0, 1000),
	))

	// Property: Empty search returns all published articles
	properties.Property("empty search returns all published articles", prop.ForAll(
		func(numArticles int) bool {
			if numArticles <= 0 || numArticles > 20 {
				return true
			}

			repo := newMockHelpArticleRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create published articles
			for i := 0; i < numArticles; i++ {
				article := &repository.HelpArticle{
					Slug:        "article-" + string(rune('a'+i)),
					Title:       "Article",
					Content:     "Content",
					IsPublished: true,
				}
				repo.Create(ctx, article)
			}

			// Empty search
			results, total, err := service.Search(ctx, "", 100, 0)
			if err != nil {
				return false
			}

			return int(total) == numArticles && len(results) == numArticles
		},
		gen.IntRange(1, 20),
	))

	// Property: Unpublished articles are not returned in search
	properties.Property("unpublished articles are not returned in search", prop.ForAll(
		func(seed int64) bool {
			repo := newMockHelpArticleRepo()
			service := NewService(repo)
			ctx := context.Background()

			searchTerm := "unique"

			// Create a published article
			published := &repository.HelpArticle{
				Slug:        "published",
				Title:       "Published " + searchTerm,
				Content:     "Content",
				IsPublished: true,
			}
			repo.Create(ctx, published)

			// Create an unpublished article with the same term
			unpublished := &repository.HelpArticle{
				Slug:        "unpublished",
				Title:       "Unpublished " + searchTerm,
				Content:     "Content",
				IsPublished: false,
			}
			repo.Create(ctx, unpublished)

			// Search
			results, _, err := service.Search(ctx, searchTerm, 10, 0)
			if err != nil {
				return false
			}

			// Should only return the published article
			if len(results) != 1 {
				return false
			}

			return results[0].Slug == "published"
		},
		gen.Int64Range(0, 1000),
	))

	// Property: Search matches in title, content, or tags
	properties.Property("search matches in title, content, or tags", prop.ForAll(
		func(matchLocation int) bool {
			if matchLocation < 0 || matchLocation > 2 {
				return true
			}

			repo := newMockHelpArticleRepo()
			service := NewService(repo)
			ctx := context.Background()

			searchTerm := "findme"

			// Create article with term in different locations
			article := &repository.HelpArticle{
				Slug:        "test",
				Title:       "Title",
				Content:     "Content",
				Tags:        "tags",
				IsPublished: true,
			}

			switch matchLocation {
			case 0:
				article.Title = "Title " + searchTerm
			case 1:
				article.Content = "Content " + searchTerm
			case 2:
				article.Tags = searchTerm
			}

			repo.Create(ctx, article)

			// Search should find it
			results, _, err := service.Search(ctx, searchTerm, 10, 0)
			if err != nil {
				return false
			}

			return len(results) == 1
		},
		gen.IntRange(0, 2),
	))

	properties.TestingRun(t)
}
