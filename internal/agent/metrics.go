// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"os"
	"runtime"
	"time"

	"v/internal/logger"
)

// MetricsCollector collects system metrics from the node.
type MetricsCollector struct {
	logger    logger.Logger
	startTime time.Time
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(log logger.Logger) *MetricsCollector {
	return &MetricsCollector{
		logger:    log,
		startTime: time.Now(),
	}
}

// Collect collects current system metrics.
func (c *MetricsCollector) Collect() *NodeMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := &NodeMetrics{
		CPUUsage:    c.getCPUUsage(),
		MemoryUsage: c.getMemoryUsage(),
		MemoryTotal: memStats.Sys,
		MemoryUsed:  memStats.Alloc,
		DiskUsage:   c.getDiskUsage(),
		NetworkIn:   0, // Would need platform-specific implementation
		NetworkOut:  0, // Would need platform-specific implementation
		Connections: 0, // Would need to query Xray API
		Uptime:      int64(time.Since(c.startTime).Seconds()),
		Timestamp:   time.Now().Unix(),
	}

	return metrics
}

// getCPUUsage returns the current CPU usage percentage.
// This is a simplified implementation - a production version would use
// platform-specific APIs or libraries like gopsutil.
func (c *MetricsCollector) getCPUUsage() float64 {
	// Simplified: return number of goroutines as a proxy for activity
	// In production, use gopsutil or similar
	return float64(runtime.NumGoroutine()) / 100.0
}

// getMemoryUsage returns the current memory usage percentage.
func (c *MetricsCollector) getMemoryUsage() float64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	if memStats.Sys == 0 {
		return 0
	}

	return float64(memStats.Alloc) / float64(memStats.Sys) * 100
}

// getDiskUsage returns the current disk usage percentage.
// This is a simplified implementation.
func (c *MetricsCollector) getDiskUsage() float64 {
	// Get current working directory disk usage
	// In production, use syscall.Statfs or gopsutil
	wd, err := os.Getwd()
	if err != nil {
		return 0
	}

	// Simplified: just check if we can access the directory
	_, err = os.Stat(wd)
	if err != nil {
		return 0
	}

	// Return a placeholder value
	// In production, calculate actual disk usage
	return 0
}

// GetUptime returns the agent uptime in seconds.
func (c *MetricsCollector) GetUptime() int64 {
	return int64(time.Since(c.startTime).Seconds())
}
