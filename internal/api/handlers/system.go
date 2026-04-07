// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	"v/internal/config"
	"v/internal/entitlement"
	"v/internal/logger"
)

type RuntimeReconciler interface {
	RunOnce(ctx context.Context) (*entitlement.RuntimeReconcileStats, error)
}

// SystemHandler handles system-related requests.
type SystemHandler struct {
	config            *config.Config
	logger            logger.Logger
	startTime         time.Time
	runtimeReconciler RuntimeReconciler
	restartHook       func() error
}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler(cfg *config.Config, log logger.Logger) *SystemHandler {
	return &SystemHandler{
		config:    cfg,
		logger:    log,
		startTime: time.Now(),
	}
}

// WithRuntimeReconciler enables manual triggering of stale runtime cleanup.
func (h *SystemHandler) WithRuntimeReconciler(runtimeReconciler RuntimeReconciler) *SystemHandler {
	h.runtimeReconciler = runtimeReconciler
	return h
}

// WithRestartHook configures a process restart hook.
func (h *SystemHandler) WithRestartHook(hook func() error) *SystemHandler {
	h.restartHook = hook
	return h
}

// SystemInfoResponse represents system information.
type SystemInfoResponse struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	GoVersion   string `json:"go_version"`
	NumCPU      int    `json:"num_cpu"`
	Goroutines  int    `json:"goroutines"`
	Uptime      string `json:"uptime"`
	UptimeSecs  int64  `json:"uptime_secs"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
}

// GetInfo returns system information.
func (h *SystemHandler) GetInfo(c *gin.Context) {
	hostname, _ := os.Hostname()
	uptime := time.Since(h.startTime)

	c.JSON(http.StatusOK, SystemInfoResponse{
		Hostname:    hostname,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		GoVersion:   runtime.Version(),
		NumCPU:      runtime.NumCPU(),
		Goroutines:  runtime.NumGoroutine(),
		Uptime:      uptime.Round(time.Second).String(),
		UptimeSecs:  int64(uptime.Seconds()),
		Version:     h.config.Version,
		Environment: h.config.Server.Mode,
	})
}

// SystemStatusResponse represents system status.
type SystemStatusResponse struct {
	Status     string     `json:"status"`
	CPU        CPUInfo    `json:"cpu"`
	Memory     MemoryInfo `json:"memory"`
	Goroutines int        `json:"goroutines"`
	Uptime     string     `json:"uptime"`
}

// CPUInfo represents CPU information.
type CPUInfo struct {
	Cores int     `json:"cores"`
	Usage float64 `json:"usage"`
}

// MemoryInfo represents memory information.
type MemoryInfo struct {
	Alloc        uint64  `json:"alloc"`
	TotalAlloc   uint64  `json:"total_alloc"`
	Sys          uint64  `json:"sys"`
	HeapAlloc    uint64  `json:"heap_alloc"`
	HeapSys      uint64  `json:"heap_sys"`
	UsagePercent float64 `json:"usage_percent"`
}

// GetStatus returns system status.
func (h *SystemHandler) GetStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(h.startTime)

	// Calculate memory usage percentage (heap used / heap sys)
	usagePercent := float64(0)
	if m.HeapSys > 0 {
		usagePercent = float64(m.HeapAlloc) / float64(m.HeapSys) * 100
	}

	c.JSON(http.StatusOK, SystemStatusResponse{
		Status: "running",
		CPU: CPUInfo{
			Cores: runtime.NumCPU(),
			Usage: 0, // Would need external package for actual CPU usage
		},
		Memory: MemoryInfo{
			Alloc:        m.Alloc / 1024 / 1024,      // MB
			TotalAlloc:   m.TotalAlloc / 1024 / 1024, // MB
			Sys:          m.Sys / 1024 / 1024,        // MB
			HeapAlloc:    m.HeapAlloc / 1024 / 1024,  // MB
			HeapSys:      m.HeapSys / 1024 / 1024,    // MB
			UsagePercent: usagePercent,
		},
		Goroutines: runtime.NumGoroutine(),
		Uptime:     uptime.Round(time.Second).String(),
	})
}

// SystemStatsResponse represents system statistics.
type SystemStatsResponse struct {
	TotalProxies    int64 `json:"total_proxies"`
	ActiveProxies   int64 `json:"active_proxies"`
	TotalUsers      int64 `json:"total_users"`
	TotalTraffic    int64 `json:"total_traffic"`
	UploadTraffic   int64 `json:"upload_traffic"`
	DownloadTraffic int64 `json:"download_traffic"`
}

// GetStats returns system statistics.
func (h *SystemHandler) GetStats(c *gin.Context) {
	// In a real implementation, these would come from the database
	// For now, return placeholder values
	c.JSON(http.StatusOK, SystemStatsResponse{
		TotalProxies:    0,
		ActiveProxies:   0,
		TotalUsers:      0,
		TotalTraffic:    0,
		UploadTraffic:   0,
		DownloadTraffic: 0,
	})
}

// AdminTriggerRuntimeReconcile manually triggers a stale runtime reconciliation pass (admin only).
// POST /api/admin/system/runtime-reconcile
func (h *SystemHandler) AdminTriggerRuntimeReconcile(c *gin.Context) {
	if h.runtimeReconciler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Runtime reconciler is unavailable"})
		return
	}

	stats, err := h.runtimeReconciler.RunOnce(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to trigger runtime reconciliation", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger runtime reconciliation"})
		return
	}

	h.logger.Info("runtime reconciliation triggered manually")
	c.JSON(http.StatusOK, gin.H{
		"message": "Runtime reconciliation completed",
		"stats":   stats,
	})
}

// AdminRestartPanel triggers a graceful process restart.
// POST /api/admin/system/restart-panel
func (h *SystemHandler) AdminRestartPanel(c *gin.Context) {
	if h.restartHook != nil {
		if err := h.restartHook(); err != nil {
			h.logger.Error("failed to restart panel via hook", logger.Err(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restart panel"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Panel restart requested"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Panel restart scheduled",
		"note":    "The current process will exit and should be restarted by its service manager.",
	})
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		process, err := os.FindProcess(os.Getpid())
		if err != nil {
			h.logger.Error("failed to locate current process for restart", logger.Err(err))
			return
		}

		if runtime.GOOS == "windows" {
			if err := process.Kill(); err != nil {
				h.logger.Error("failed to terminate process on windows restart", logger.Err(err))
			}
			return
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			h.logger.Error("failed to terminate process for restart", logger.Err(err))
		}
	}()
}

// DetailedSystemStatusResponse represents detailed system status for SystemMonitor.vue
type DetailedSystemStatusResponse struct {
	CPUInfo     CPUInfoDetail    `json:"cpuInfo"`
	CPUUsage    float64          `json:"cpuUsage"`
	MemoryInfo  MemoryInfoDetail `json:"memoryInfo"`
	MemoryUsage float64          `json:"memoryUsage"`
	DiskInfo    DiskInfoDetail   `json:"diskInfo"`
	DiskUsage   float64          `json:"diskUsage"`
	SystemInfo  SystemInfoDetail `json:"systemInfo"`
	Processes   []ProcessInfo    `json:"processes"`
}

// CPUInfoDetail represents detailed CPU information.
type CPUInfoDetail struct {
	Cores int    `json:"cores"`
	Model string `json:"model"`
}

// MemoryInfoDetail represents detailed memory information.
type MemoryInfoDetail struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

// DiskInfoDetail represents detailed disk information.
type DiskInfoDetail struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

// SystemInfoDetail represents detailed system information.
type SystemInfoDetail struct {
	OS        string    `json:"os"`
	Kernel    string    `json:"kernel"`
	Hostname  string    `json:"hostname"`
	Uptime    string    `json:"uptime"`
	Load      []float64 `json:"load"`
	IPAddress string    `json:"ipAddress"`
}

// ProcessInfo represents process information.
type ProcessInfo struct {
	PID        int32  `json:"pid"`
	Name       string `json:"name"`
	User       string `json:"user"`
	CPU        string `json:"cpu"`
	Memory     string `json:"memory"`
	MemoryUsed uint64 `json:"memoryUsed"`
	Started    string `json:"started"`
	State      string `json:"state"`
}

// GetDetailedStatus returns detailed system status for SystemMonitor.vue
func (h *SystemHandler) GetDetailedStatus(c *gin.Context) {
	response := DetailedSystemStatusResponse{}

	// Get CPU info
	cpuInfos, err := cpu.Info()
	if err == nil && len(cpuInfos) > 0 {
		response.CPUInfo = CPUInfoDetail{
			Cores: runtime.NumCPU(),
			Model: cpuInfos[0].ModelName,
		}
	} else {
		response.CPUInfo = CPUInfoDetail{
			Cores: runtime.NumCPU(),
			Model: "Unknown",
		}
	}

	// Get CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		response.CPUUsage = cpuPercent[0]
	}

	// Get memory info
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		response.MemoryInfo = MemoryInfoDetail{
			Used:  memInfo.Used,
			Total: memInfo.Total,
		}
		response.MemoryUsage = memInfo.UsedPercent
	}

	// Get disk info
	diskInfo, err := disk.Usage("/")
	if err == nil {
		response.DiskInfo = DiskInfoDetail{
			Used:  diskInfo.Used,
			Total: diskInfo.Total,
		}
		response.DiskUsage = diskInfo.UsedPercent
	}

	// Get system info
	hostInfo, err := host.Info()
	hostname, _ := os.Hostname()
	if err == nil {
		uptime := time.Duration(hostInfo.Uptime) * time.Second
		days := int(uptime.Hours() / 24)
		hours := int(uptime.Hours()) % 24
		minutes := int(uptime.Minutes()) % 60

		response.SystemInfo = SystemInfoDetail{
			OS:       hostInfo.OS,
			Kernel:   hostInfo.KernelVersion,
			Hostname: hostname,
			Uptime:   fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes),
		}
	} else {
		response.SystemInfo = SystemInfoDetail{
			OS:       runtime.GOOS,
			Kernel:   "Unknown",
			Hostname: hostname,
			Uptime:   time.Since(h.startTime).Round(time.Second).String(),
		}
	}

	// Get load average
	loadInfo, err := load.Avg()
	if err == nil {
		response.SystemInfo.Load = []float64{loadInfo.Load1, loadInfo.Load5, loadInfo.Load15}
	} else {
		response.SystemInfo.Load = []float64{0, 0, 0}
	}

	// Get IP address
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if len(iface.Addrs) > 0 && iface.Name != "lo" && iface.Name != "lo0" {
				for _, addr := range iface.Addrs {
					if addr.Addr != "" && addr.Addr != "127.0.0.1/8" && addr.Addr != "::1/128" {
						response.SystemInfo.IPAddress = addr.Addr
						break
					}
				}
				if response.SystemInfo.IPAddress != "" {
					break
				}
			}
		}
	}
	if response.SystemInfo.IPAddress == "" {
		response.SystemInfo.IPAddress = "127.0.0.1"
	}

	// Get process list
	processes, err := process.Processes()
	if err == nil {
		response.Processes = make([]ProcessInfo, 0)
		for i, p := range processes {
			if i >= 20 { // Limit to 20 processes
				break
			}

			name, _ := p.Name()
			username, _ := p.Username()
			cpuPercent, _ := p.CPUPercent()
			memPercent, _ := p.MemoryPercent()
			memInfoProc, _ := p.MemoryInfo()
			createTime, _ := p.CreateTime()
			status, _ := p.Status()

			var memUsed uint64
			if memInfoProc != nil {
				memUsed = memInfoProc.RSS
			}

			startTime := time.Unix(createTime/1000, 0).Format("2006-01-02 15:04:05")

			statusStr := "running"
			if len(status) > 0 {
				switch status[0] {
				case "S":
					statusStr = "sleeping"
				case "R":
					statusStr = "running"
				case "Z":
					statusStr = "zombie"
				case "T":
					statusStr = "stopped"
				case "I":
					statusStr = "idle"
				}
			}

			response.Processes = append(response.Processes, ProcessInfo{
				PID:        p.Pid,
				Name:       name,
				User:       username,
				CPU:        fmt.Sprintf("%.1f", cpuPercent),
				Memory:     fmt.Sprintf("%.1f", memPercent),
				MemoryUsed: memUsed,
				Started:    startTime,
				State:      statusStr,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    response,
	})
}
