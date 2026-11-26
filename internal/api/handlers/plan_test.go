package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"v/internal/commercial/plan"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestCreatePlanPriceAndInactiveContract(t *testing.T) {
	for _, tc := range []struct {
		name  string
		price any
		omit  bool
		want  int
	}{
		{"free", 0, false, http.StatusCreated},
		{"paid", 1500, false, http.StatusCreated},
		{"negative", -1, false, http.StatusBadRequest},
		{"null", nil, false, http.StatusBadRequest},
		{"missing", nil, true, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupPaymentTestDB(t)
			repo := repository.NewPlanRepository(db)
			handler := NewPlanHandler(plan.NewService(repo, logger.NewNopLogger()), logger.NewNopLogger())
			router := gin.New()
			router.POST("/plans", handler.CreatePlan)
			body := map[string]any{"name": "Draft", "duration": 45, "plan_type": "custom", "is_active": false}
			if !tc.omit {
				body["price"] = tc.price
			}
			payload, err := json.Marshal(body)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/plans", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code, w.Body.String())
			active, err := repo.ListActive(req.Context())
			require.NoError(t, err)
			require.Empty(t, active)
		})
	}
}
