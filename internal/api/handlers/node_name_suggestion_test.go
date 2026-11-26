package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/ip"
	"v/internal/logger"
)

func TestNodeNameSuggestionHandlerSuggest(t *testing.T) {
	handler := NewNodeNameSuggestionHandler(logger.NewNopLogger(), nil)
	handler.resolveIP = func(context.Context, string) (string, error) {
		return "64.176.54.36", nil
	}
	handler.lookupExternal = func(context.Context, string) (*ip.GeoInfo, error) {
		return &ip.GeoInfo{
			IP:          "64.176.54.36",
			Country:     "United States",
			CountryCode: "US",
			Region:      "California",
			City:        "San Jose",
		}, nil
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/admin/nodes/name-suggestion?address=64.176.54.36",
		nil,
	)

	handler.Suggest(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response NodeNameSuggestionResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "美国-硅谷-36", response.SuggestedName)
	assert.Equal(t, "美国", response.SuggestedRegion)
	assert.Equal(t, "美国-硅谷", response.LocationLabel)
	assert.Equal(t, "external", response.Source)
}

func TestNodeNameSuggestionHandlerUsesManualRegionSafely(t *testing.T) {
	handler := NewNodeNameSuggestionHandler(logger.NewNopLogger(), nil)
	handler.resolveIP = func(context.Context, string) (string, error) {
		return "64.176.54.36", nil
	}
	handler.lookupExternal = func(context.Context, string) (*ip.GeoInfo, error) {
		return &ip.GeoInfo{
			IP:          "64.176.54.36",
			Country:     "United States",
			CountryCode: "US",
			Region:      "California",
			City:        "San Jose",
		}, nil
	}

	response := handler.suggestNodeName(context.Background(), "64.176.54.36", "日本")

	assert.Equal(t, "日本-36", response.SuggestedName)
	assert.Equal(t, "日本", response.SuggestedRegion)
	assert.Equal(t, "美国-硅谷", response.LocationLabel)
}

func TestNodeNameSuggestionHandlerFallsBackWithoutGeoData(t *testing.T) {
	handler := NewNodeNameSuggestionHandler(logger.NewNopLogger(), nil)
	handler.resolveIP = func(context.Context, string) (string, error) {
		return "64.176.54.36", nil
	}
	handler.lookupExternal = func(context.Context, string) (*ip.GeoInfo, error) {
		return nil, fmt.Errorf("lookup failed")
	}

	response := handler.suggestNodeName(context.Background(), "64.176.54.36", "")

	assert.Equal(t, "节点-64-176-54-36", response.SuggestedName)
	assert.Equal(t, "fallback", response.Source)
	assert.Empty(t, response.SuggestedRegion)
}

func TestNodeNameSuggestionHandlerCachesExternalLookup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ip.GeoCache{}))

	geoService, err := ip.NewGeolocationService(db, &ip.GeolocationConfig{
		DatabasePath: "",
		CacheTTL:     24 * time.Hour,
	})
	require.NoError(t, err)
	defer geoService.Close()

	handler := NewNodeNameSuggestionHandler(logger.NewNopLogger(), geoService)
	handler.lookupExternal = func(context.Context, string) (*ip.GeoInfo, error) {
		return &ip.GeoInfo{
			IP:          "1.1.1.1",
			Country:     "Japan",
			CountryCode: "JP",
			City:        "Tokyo",
		}, nil
	}

	info, source := handler.lookupGeolocation(context.Background(), "1.1.1.1")
	require.NotNil(t, info)
	assert.Equal(t, "external", source)

	var cache ip.GeoCache
	require.NoError(t, db.First(&cache, "ip = ?", "1.1.1.1").Error)
	assert.Equal(t, "Japan", cache.Country)
	assert.Equal(t, "Tokyo", cache.City)
}

func TestNodeNameSuggestionHandlerRequiresAddress(t *testing.T) {
	handler := NewNodeNameSuggestionHandler(logger.NewNopLogger(), nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/nodes/name-suggestion", nil)

	handler.Suggest(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
