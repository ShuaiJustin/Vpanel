package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"v/internal/logger"
)

func TestXrayReleasesFallsBackToCachedVersions(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/unavailable" {
			http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Query().Get("per_page") != "20" {
			http.Error(w, "unexpected page size", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`[{"tag_name":"v26.11.9"}]`))
	}))
	defer upstream.Close()

	h := NewXrayReleasesHandler(logger.NewNopLogger())
	h.apiURL = upstream.URL + "/unavailable"
	router := gin.New()
	router.GET("/versions", func(c *gin.Context) {
		c.Set("request_id", "req-releases-test")
		h.List(c)
	})
	refresh := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/versions?refresh=1", nil))
		return w
	}

	cold := refresh()
	require.Equal(t, http.StatusBadGateway, cold.Code)
	require.Contains(t, cold.Body.String(), `"request_id":"req-releases-test"`)
	h.apiURL = upstream.URL
	fresh := refresh()
	require.Equal(t, http.StatusOK, fresh.Code)
	require.Contains(t, fresh.Body.String(), `"cached":false`)
	require.Contains(t, fresh.Body.String(), `"version":"26.11.9"`)
	h.apiURL = upstream.URL + "/unavailable"
	fallback := refresh()
	require.Equal(t, http.StatusOK, fallback.Code)
	require.JSONEq(t, strings.Replace(fresh.Body.String(), `"cached":false`, `"cached":true`, 1), fallback.Body.String())
}
