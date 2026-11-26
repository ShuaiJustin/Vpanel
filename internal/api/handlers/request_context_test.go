package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	apperrors "v/pkg/errors"
)

func TestHandleRequestContextError_ClientCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/stats/traffic", nil)

	err := apperrors.NewDatabaseError("failed to get total traffic by period", context.Canceled)

	handled := handleRequestContextError(ctx, err)

	assert.True(t, handled)
	assert.Equal(t, statusClientClosedRequest, recorder.Code)
}

func TestHandleRequestContextError_DeadlineExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/stats/traffic", nil)

	err := apperrors.NewDatabaseError("failed to get total traffic by period", context.DeadlineExceeded)

	handled := handleRequestContextError(ctx, err)

	assert.True(t, handled)
	assert.Equal(t, http.StatusGatewayTimeout, recorder.Code)
}

func TestHandleRequestContextError_UnrelatedError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/stats/traffic", nil)

	handled := handleRequestContextError(ctx, assert.AnError)

	assert.False(t, handled)
	assert.Equal(t, http.StatusOK, recorder.Code)
}
