// Package handlers — audit_emit.go provides a single helper any handler can
// call to record an audit log entry. Each handler stores the AuditService on
// its struct and calls emitAudit() at the end of a state-changing operation.
//
// The helper:
//   1. No-ops when the service is nil or disabled (the admin's "启用操作日志"
//      switch in 系统设置 → 日志配置 toggles the enabled state at runtime).
//   2. Auto-fills IPAddress / UserAgent / RequestID from the gin context if
//      the caller hasn't already set them, so call sites stay short.
//   3. Swallows the underlying Log() error — audit failure must never break
//      a user-facing operation. The error is already logged inside the
//      AuditService impl.
package handlers

import (
	"github.com/gin-gonic/gin"

	"v/internal/monitor"
)

// emitAudit logs an audit entry. Safe to call with svc == nil.
func emitAudit(c *gin.Context, svc monitor.AuditService, entry monitor.AuditEntry) {
	if svc == nil || !svc.Enabled() {
		return
	}
	if entry.IPAddress == "" {
		entry.IPAddress = c.ClientIP()
	}
	if entry.UserAgent == "" {
		entry.UserAgent = c.Request.UserAgent()
	}
	if entry.RequestID == "" {
		entry.RequestID = c.GetString("request_id")
	}
	if entry.Status == "" {
		entry.Status = monitor.StatusSuccess
	}
	if entry.UserID == nil {
		if uid, ok := currentUserIDFromContext(c); ok {
			entry.UserID = &uid
		}
	}
	if entry.Username == "" {
		entry.Username = c.GetString("username")
	}
	_ = svc.Log(c.Request.Context(), &entry)
}
