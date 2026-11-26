// Package monitor provides monitoring and observability functionality.
package monitor

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	// HTTP metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge

	// Business metrics
	activeUsers    prometheus.Gauge
	activeProxies  prometheus.Gauge
	totalTraffic   *prometheus.CounterVec
	loginAttempts  *prometheus.CounterVec
	proxyOperations *prometheus.CounterVec

	// System metrics
	xrayStatus     prometheus.Gauge
	dbConnections  prometheus.Gauge
	cacheHitRatio  prometheus.Gauge
}

// NewMetrics creates a new Metrics instance with all metrics registered.
func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "vpanel"
	}

	m := &Metrics{
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path"},
		),
		httpRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "http_requests_in_flight",
				Help:      "Number of HTTP requests currently being processed",
			},
		),
		activeUsers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_users",
				Help:      "Number of active users",
			},
		),
		activeProxies: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_proxies",
				Help:      "Number of active proxies",
			},
		),
		totalTraffic: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "traffic_bytes_total",
				Help:      "Total traffic in bytes",
			},
			[]string{"direction"}, // "upload" or "download"
		),
		loginAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "login_attempts_total",
				Help:      "Total number of login attempts",
			},
			[]string{"success"}, // "true" or "false"
		),
		proxyOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "proxy_operations_total",
				Help:      "Total number of proxy operations",
			},
			[]string{"operation"}, // "create", "update", "delete", "start", "stop"
		),
		xrayStatus: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "xray_status",
				Help:      "Xray process status (1 = running, 0 = stopped)",
			},
		),
		dbConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "db_connections",
				Help:      "Number of active database connections",
			},
		),
		cacheHitRatio: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "cache_hit_ratio",
				Help:      "Cache hit ratio (0-1)",
			},
		),
	}

	return m
}

// RecordHTTPRequest records an HTTP request metric.
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	m.httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// IncrementInFlight increments the in-flight request counter.
func (m *Metrics) IncrementInFlight() {
	m.httpRequestsInFlight.Inc()
}

// DecrementInFlight decrements the in-flight request counter.
func (m *Metrics) DecrementInFlight() {
	m.httpRequestsInFlight.Dec()
}

// SetActiveUsers sets the active users gauge.
func (m *Metrics) SetActiveUsers(count float64) {
	m.activeUsers.Set(count)
}

// SetActiveProxies sets the active proxies gauge.
func (m *Metrics) SetActiveProxies(count float64) {
	m.activeProxies.Set(count)
}

// AddTraffic adds traffic to the counter.
func (m *Metrics) AddTraffic(upload, download int64) {
	m.totalTraffic.WithLabelValues("upload").Add(float64(upload))
	m.totalTraffic.WithLabelValues("download").Add(float64(download))
}

// RecordLoginAttempt records a login attempt.
func (m *Metrics) RecordLoginAttempt(success bool) {
	m.loginAttempts.WithLabelValues(strconv.FormatBool(success)).Inc()
}

// RecordProxyOperation records a proxy operation.
func (m *Metrics) RecordProxyOperation(operation string) {
	m.proxyOperations.WithLabelValues(operation).Inc()
}

// SetXrayStatus sets the Xray status gauge.
func (m *Metrics) SetXrayStatus(running bool) {
	if running {
		m.xrayStatus.Set(1)
	} else {
		m.xrayStatus.Set(0)
	}
}

// SetDBConnections sets the database connections gauge.
func (m *Metrics) SetDBConnections(count float64) {
	m.dbConnections.Set(count)
}

// SetCacheHitRatio sets the cache hit ratio gauge.
func (m *Metrics) SetCacheHitRatio(ratio float64) {
	m.cacheHitRatio.Set(ratio)
}

// Handler returns the Prometheus HTTP handler.
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

// GinHandler returns a Gin handler for the /metrics endpoint.
func (m *Metrics) GinHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// Middleware returns a Gin middleware that records HTTP metrics.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		m.IncrementInFlight()
		defer m.DecrementInFlight()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method

		m.RecordHTTPRequest(method, path, status, duration)
	}
}
