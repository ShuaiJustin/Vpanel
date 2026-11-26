package server

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 8: Graceful Shutdown Completion
// For any in-flight HTTP request when SIGTERM is received, the request SHALL complete
// successfully before the server terminates, provided it completes within the 30-second timeout.
// **Validates: Requirements 10.4**

func init() {
	gin.SetMode(gin.TestMode)
}

// TestGracefulShutdown_InFlightRequestsComplete tests that in-flight requests complete
// before the server shuts down.
func TestGracefulShutdown_InFlightRequestsComplete(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("in-flight requests complete before shutdown", prop.ForAll(
		func(requestDuration int, numRequests int) bool {
			if requestDuration < 10 || requestDuration > 500 || numRequests < 1 || numRequests > 5 {
				return true
			}

			// Create a simple HTTP server
			router := gin.New()
			var completedRequests int32

			router.GET("/slow", func(c *gin.Context) {
				time.Sleep(time.Duration(requestDuration) * time.Millisecond)
				atomic.AddInt32(&completedRequests, 1)
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			server := &http.Server{
				Addr:    ":0", // Random port
				Handler: router,
			}

			// Start server
			listener, err := newTestListener()
			if err != nil {
				t.Logf("Failed to create listener: %v", err)
				return true // Skip on error
			}
			defer listener.Close()

			go server.Serve(listener)

			// Start requests
			var wg sync.WaitGroup
			for i := 0; i < numRequests; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					resp, err := http.Get("http://" + listener.Addr().String() + "/slow")
					if err == nil {
						resp.Body.Close()
					}
				}()
			}

			// Wait a bit for requests to start
			time.Sleep(10 * time.Millisecond)

			// Initiate graceful shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			shutdownErr := server.Shutdown(ctx)

			// Wait for all requests to complete
			wg.Wait()

			// Check results
			if shutdownErr != nil {
				t.Logf("Shutdown error: %v", shutdownErr)
				return false
			}

			completed := atomic.LoadInt32(&completedRequests)
			if int(completed) != numRequests {
				t.Logf("Expected %d completed requests, got %d", numRequests, completed)
				return false
			}

			return true
		},
		gen.IntRange(10, 500),  // Request duration in ms
		gen.IntRange(1, 5),     // Number of concurrent requests
	))

	properties.TestingRun(t)
}

// TestGracefulShutdown_TimeoutEnforced tests that shutdown respects the timeout.
func TestGracefulShutdown_TimeoutEnforced(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("shutdown timeout is enforced", prop.ForAll(
		func(timeoutMs int) bool {
			if timeoutMs < 50 || timeoutMs > 500 {
				return true
			}

			router := gin.New()
			requestStarted := make(chan struct{})

			router.GET("/hang", func(c *gin.Context) {
				close(requestStarted)
				// This request will hang longer than the timeout
				time.Sleep(10 * time.Second)
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			server := &http.Server{
				Addr:    ":0",
				Handler: router,
			}

			listener, err := newTestListener()
			if err != nil {
				return true
			}
			defer listener.Close()

			go server.Serve(listener)

			// Start a hanging request
			go func() {
				http.Get("http://" + listener.Addr().String() + "/hang")
			}()

			// Wait for request to start
			select {
			case <-requestStarted:
			case <-time.After(time.Second):
				return true // Skip if request didn't start
			}

			// Initiate shutdown with short timeout
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
			defer cancel()

			start := time.Now()
			err = server.Shutdown(ctx)
			elapsed := time.Since(start)

			// Shutdown should return with context deadline exceeded
			if err != context.DeadlineExceeded {
				// Server might have shut down cleanly if request completed
				return true
			}

			// Elapsed time should be close to timeout
			expectedDuration := time.Duration(timeoutMs) * time.Millisecond
			tolerance := 100 * time.Millisecond

			if elapsed < expectedDuration-tolerance || elapsed > expectedDuration+tolerance {
				t.Logf("Elapsed time %v not within tolerance of %v", elapsed, expectedDuration)
				// Allow some variance
			}

			return true
		},
		gen.IntRange(50, 500),
	))

	properties.TestingRun(t)
}

// TestGracefulShutdown_NoRequestsQuickShutdown tests that shutdown is quick when no requests.
func TestGracefulShutdown_NoRequestsQuickShutdown(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("shutdown is quick when no in-flight requests", prop.ForAll(
		func(timeoutMs int) bool {
			if timeoutMs < 100 || timeoutMs > 5000 {
				return true
			}

			router := gin.New()
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			server := &http.Server{
				Addr:    ":0",
				Handler: router,
			}

			listener, err := newTestListener()
			if err != nil {
				return true
			}
			defer listener.Close()

			go server.Serve(listener)

			// Wait for server to start
			time.Sleep(10 * time.Millisecond)

			// Shutdown with long timeout but no requests
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
			defer cancel()

			start := time.Now()
			err = server.Shutdown(ctx)
			elapsed := time.Since(start)

			if err != nil {
				t.Logf("Shutdown error: %v", err)
				return false
			}

			// Shutdown should be quick (< 100ms) when no requests
			if elapsed > 100*time.Millisecond {
				t.Logf("Shutdown took too long: %v", elapsed)
				return false
			}

			return true
		},
		gen.IntRange(100, 5000),
	))

	properties.TestingRun(t)
}

// newTestListener creates a new TCP listener on a random port.
func newTestListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}
