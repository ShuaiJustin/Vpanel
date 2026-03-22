package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBackgroundTaskContext_IgnoresParentCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), "request_id", "req-123"))
	cancelParent()

	ctx, cancel := newBackgroundTaskContextWithTimeout(parent, 50*time.Millisecond)
	defer cancel()

	assert.Nil(t, ctx.Err())
	assert.Equal(t, "req-123", ctx.Value("request_id"))
}

func TestNewBackgroundTaskContext_TimeoutStillApplies(t *testing.T) {
	ctx, cancel := newBackgroundTaskContextWithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	<-ctx.Done()
	assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}
