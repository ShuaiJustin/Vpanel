package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"v/internal/config"
	"v/internal/entitlement"
	"v/internal/logger"
)

type mockRuntimeReconciler struct {
	stats *entitlement.RuntimeReconcileStats
	err   error
	calls int
}

func (m *mockRuntimeReconciler) RunOnce(context.Context) (*entitlement.RuntimeReconcileStats, error) {
	m.calls++
	return m.stats, m.err
}

func TestSystemHandler_AdminTriggerRuntimeReconcile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reconciler := &mockRuntimeReconciler{
		stats: &entitlement.RuntimeReconcileStats{
			ScannedProxies:         12,
			DeletedMissingNode:     3,
			EvaluatedUsers:         4,
			ForbiddenUsersDetected: 2,
		},
	}
	handler := NewSystemHandler(&config.Config{}, logger.NewNopLogger()).WithRuntimeReconciler(reconciler)

	router := gin.New()
	router.POST("/admin/system/runtime-reconcile", handler.AdminTriggerRuntimeReconcile)

	req := httptest.NewRequest(http.MethodPost, "/admin/system/runtime-reconcile", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if reconciler.calls != 1 {
		t.Fatalf("expected reconciler to be called once, got %d", reconciler.calls)
	}

	var response struct {
		Message string                            `json:"message"`
		Stats   entitlement.RuntimeReconcileStats `json:"stats"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Message != "Runtime reconciliation completed" {
		t.Fatalf("unexpected message %q", response.Message)
	}
	if response.Stats.DeletedMissingNode != 3 {
		t.Fatalf("expected deleted_missing_node 3, got %d", response.Stats.DeletedMissingNode)
	}
	if response.Stats.ForbiddenUsersDetected != 2 {
		t.Fatalf("expected forbidden_users_detected 2, got %d", response.Stats.ForbiddenUsersDetected)
	}
}

func TestSystemHandler_AdminTriggerRuntimeReconcileUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewSystemHandler(&config.Config{}, logger.NewNopLogger())
	router := gin.New()
	router.POST("/admin/system/runtime-reconcile", handler.AdminTriggerRuntimeReconcile)

	req := httptest.NewRequest(http.MethodPost, "/admin/system/runtime-reconcile", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", w.Code, w.Body.String())
	}
}
