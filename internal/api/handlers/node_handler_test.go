package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"v/internal/node"
)

func TestNormalizeNodeGroupIDs(t *testing.T) {
	fallback := int64(5)

	tests := []struct {
		name     string
		groupIDs []int64
		fallback *int64
		want     []int64
	}{
		{
			name:     "deduplicates and ignores invalid values",
			groupIDs: []int64{3, 0, -1, 3, 4},
			fallback: &fallback,
			want:     []int64{3, 4},
		},
		{
			name:     "uses fallback when explicit groups are empty",
			groupIDs: nil,
			fallback: &fallback,
			want:     []int64{5},
		},
		{
			name:     "returns empty when no valid groups exist",
			groupIDs: []int64{0, -1},
			fallback: nil,
			want:     []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeNodeGroupIDs(tt.groupIDs, tt.fallback))
		})
	}
}

func TestNodeMutationErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{
			name:       "duplicate node returns conflict",
			err:        fmt.Errorf("%w: 节点地址 1.1.1.1:18443 已存在", node.ErrDuplicateNode),
			wantStatus: http.StatusConflict,
			wantError:  "节点地址 1.1.1.1:18443 已存在",
		},
		{
			name:       "invalid node returns bad request",
			err:        fmt.Errorf("%w: 节点名称不能为空", node.ErrInvalidNode),
			wantStatus: http.StatusBadRequest,
			wantError:  "节点名称不能为空",
		},
		{
			name:       "not found returns 404",
			err:        node.ErrNodeNotFound,
			wantStatus: http.StatusNotFound,
			wantError:  "Node not found",
		},
		{
			name:       "unknown error returns fallback",
			err:        fmt.Errorf("boom"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to create node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, payload := nodeMutationErrorResponse(tt.err, "Failed to create node")
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, gin.H{"error": tt.wantError}, payload)
		})
	}
}
