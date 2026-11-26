package middleware

import (
	"context"
	"time"
)

const backgroundTaskTimeout = 5 * time.Second

func newBackgroundTaskContext(parent context.Context) (context.Context, context.CancelFunc) {
	return newBackgroundTaskContextWithTimeout(parent, backgroundTaskTimeout)
}

func newBackgroundTaskContextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}
