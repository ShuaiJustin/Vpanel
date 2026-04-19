// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
)

// XrayReleasesHandler serves an Xray version list sourced from GitHub releases.
// It is a thin, read-only proxy — no binaries are downloaded or stored on the
// panel host. The list is cached in memory for cacheTTL to stay within GitHub's
// unauthenticated rate limit (60 req/h per IP).
type XrayReleasesHandler struct {
	logger     logger.Logger
	httpClient *http.Client
	apiURL     string
	cacheTTL   time.Duration

	mu        sync.RWMutex
	cache     []XrayReleaseInfo
	cachedAt  time.Time
}

// XrayReleaseInfo is a trimmed view of a GitHub release, safe to ship to the UI.
type XrayReleaseInfo struct {
	Version     string    `json:"version"`
	ReleaseDate time.Time `json:"release_date"`
	Prerelease  bool      `json:"prerelease"`
}

// NewXrayReleasesHandler constructs a handler backed by the public GitHub API.
func NewXrayReleasesHandler(log logger.Logger) *XrayReleasesHandler {
	return &XrayReleasesHandler{
		logger: log,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		apiURL:   "https://api.github.com/repos/XTLS/Xray-core/releases",
		cacheTTL: 30 * time.Minute,
	}
}

// List returns the recent Xray release tags.
// GET /api/admin/xray/available-versions?refresh=1
func (h *XrayReleasesHandler) List(c *gin.Context) {
	forceRefresh, _ := strconv.ParseBool(c.Query("refresh"))

	versions, cached, err := h.fetch(c.Request.Context(), forceRefresh)
	if err != nil {
		h.logger.Warn("failed to fetch Xray releases from GitHub", logger.F("error", err))
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "获取 GitHub 版本列表失败，请稍后重试或直接输入版本号",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"versions":  versions,
		"cached":    cached,
		"cached_at": h.cachedAtTime().UTC().Format(time.RFC3339),
	})
}

func (h *XrayReleasesHandler) cachedAtTime() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.cachedAt
}

func (h *XrayReleasesHandler) fetch(ctx context.Context, forceRefresh bool) ([]XrayReleaseInfo, bool, error) {
	if !forceRefresh {
		h.mu.RLock()
		fresh := len(h.cache) > 0 && time.Since(h.cachedAt) < h.cacheTTL
		if fresh {
			out := make([]XrayReleaseInfo, len(h.cache))
			copy(out, h.cache)
			h.mu.RUnlock()
			return out, true, nil
		}
		h.mu.RUnlock()
	}

	versions, err := h.fetchFromGitHub(ctx)
	if err != nil {
		// Fall back to previously cached entries so the UI still gets a list.
		h.mu.RLock()
		if len(h.cache) > 0 {
			out := make([]XrayReleaseInfo, len(h.cache))
			copy(out, h.cache)
			h.mu.RUnlock()
			return out, true, nil
		}
		h.mu.RUnlock()
		return nil, false, err
	}

	h.mu.Lock()
	h.cache = versions
	h.cachedAt = time.Now()
	h.mu.Unlock()

	return versions, false, nil
}

func (h *XrayReleasesHandler) fetchFromGitHub(ctx context.Context) ([]XrayReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.apiURL+"?per_page=30", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "V-Panel/1.0")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call github: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw []struct {
		TagName     string    `json:"tag_name"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	versions := make([]XrayReleaseInfo, 0, len(raw))
	for _, r := range raw {
		if r.Prerelease {
			continue
		}
		tag := strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
		if tag == "" {
			continue
		}
		versions = append(versions, XrayReleaseInfo{
			Version:     tag,
			ReleaseDate: r.PublishedAt,
			Prerelease:  r.Prerelease,
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareSemverTags(versions[i].Version, versions[j].Version) > 0
	})

	if len(versions) > 20 {
		versions = versions[:20]
	}
	return versions, nil
}

// compareSemverTags compares two dot-separated numeric version strings.
// Returns >0 if a>b, <0 if a<b, 0 if equal. Non-numeric segments compare as 0.
func compareSemverTags(a, b string) int {
	aa := strings.Split(a, ".")
	bb := strings.Split(b, ".")
	maxLen := len(aa)
	if len(bb) > maxLen {
		maxLen = len(bb)
	}
	for i := 0; i < maxLen; i++ {
		ai, bi := 0, 0
		if i < len(aa) {
			ai, _ = strconv.Atoi(aa[i])
		}
		if i < len(bb) {
			bi, _ = strconv.Atoi(bb[i])
		}
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}
