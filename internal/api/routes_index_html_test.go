package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// sampleIndex mirrors the shape of web/dist/index.html: a <head> containing the
// module entry script, which is where the base-path marker gets injected.
const sampleIndex = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>V</title>
  <script type="module" crossorigin src="/assets/js/index-abc.js"></script>
  <link rel="stylesheet" crossorigin href="/assets/css/index-abc.css">
</head>
<body>
  <div id="app"></div>
</body>
</html>
`

func writeSampleIndex(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(path, []byte(sampleIndex), 0o600); err != nil {
		t.Fatalf("write sample index: %v", err)
	}
	return path
}

// The CSP set by SecureHeaders() allows script-src 'self' only, so the marker
// must never be emitted as an inline block - otherwise the browser blocks it and
// the SPA silently falls back to an empty base path.
func TestLoadAndRewriteIndexHTMLUsesSameOriginScriptNotInline(t *testing.T) {
	path := writeSampleIndex(t)

	got, err := loadAndRewriteIndexHTML(path, "")
	if err != nil {
		t.Fatalf("loadAndRewriteIndexHTML: %v", err)
	}
	html := string(got)

	if strings.Contains(html, "<script>window.__VPANEL_BASE__") {
		t.Errorf("index.html still contains an inline base-path block, which the CSP blocks:\n%s", html)
	}
	if !strings.Contains(html, `<script src="/vpbase.js"></script>`) {
		t.Errorf("expected same-origin vpbase.js reference, got:\n%s", html)
	}
	// The marker must be parsed before the module entry so axios/router see it.
	if strings.Index(html, "vpbase.js") > strings.Index(html, "assets/js/index-abc.js") {
		t.Errorf("vpbase.js must precede the module entry script:\n%s", html)
	}
}

func TestLoadAndRewriteIndexHTMLPrefixesScriptWithBasePath(t *testing.T) {
	path := writeSampleIndex(t)

	got, err := loadAndRewriteIndexHTML(path, "/panel")
	if err != nil {
		t.Fatalf("loadAndRewriteIndexHTML: %v", err)
	}
	html := string(got)

	if !strings.Contains(html, `<script src="/panel/vpbase.js"></script>`) {
		t.Errorf("expected base-path prefixed script src, got:\n%s", html)
	}
	if !strings.Contains(html, `<base href="/panel/">`) {
		t.Errorf("expected <base> tag, got:\n%s", html)
	}
	if !strings.Contains(html, `href="/panel/assets/css/index-abc.css"`) {
		t.Errorf("expected rewritten stylesheet href, got:\n%s", html)
	}
}

func TestServePanelBaseScript(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		basePath string
		wantBody string
	}{
		{"", `window.__VPANEL_BASE__="";`},
		{"/panel", `window.__VPANEL_BASE__="/panel";`},
	}

	for _, tc := range cases {
		router := gin.New()
		router.GET("/vpbase.js", servePanelBaseScript(tc.basePath))

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/vpbase.js", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("basePath=%q: status = %d, want 200", tc.basePath, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/javascript; charset=utf-8" {
			t.Errorf("basePath=%q: content-type = %q, want application/javascript", tc.basePath, ct)
		}
		if got := rec.Body.String(); got != tc.wantBody {
			t.Errorf("basePath=%q: body = %q, want %q", tc.basePath, got, tc.wantBody)
		}
	}
}

// The endpoint must behave as a script resource under CSP script-src 'self',
// i.e. served from the same origin path the rewritten index.html points at.
func TestServePanelBaseScriptIsReachableUnderBasePathGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	group := router.Group("/panel")
	group.GET("/vpbase.js", servePanelBaseScript("/panel"))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panel/vpbase.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (index.html references /panel/vpbase.js)", rec.Code)
	}
	if got := rec.Body.String(); got != `window.__VPANEL_BASE__="/panel";` {
		t.Errorf("body = %q, want the base-path marker", got)
	}
}
