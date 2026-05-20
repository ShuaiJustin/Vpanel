// Package monitor provides monitoring and observability functionality.
package monitor

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"v/internal/database/repository"
	"v/internal/logger"
)

// AuditAction represents an auditable action.
type AuditAction string

// Audit actions
const (
	ActionLogin          AuditAction = "login"
	ActionLogout         AuditAction = "logout"
	ActionPasswordChange AuditAction = "password_change"
	ActionPasswordReset  AuditAction = "password_reset"
	ActionUserCreate     AuditAction = "user_create"
	ActionUserUpdate     AuditAction = "user_update"
	ActionUserDelete     AuditAction = "user_delete"
	ActionUserEnable     AuditAction = "user_enable"
	ActionUserDisable    AuditAction = "user_disable"
	ActionProxyCreate    AuditAction = "proxy_create"
	ActionProxyUpdate    AuditAction = "proxy_update"
	ActionProxyDelete    AuditAction = "proxy_delete"
	ActionProxyStart     AuditAction = "proxy_start"
	ActionProxyStop      AuditAction = "proxy_stop"
	ActionRoleCreate     AuditAction = "role_create"
	ActionRoleUpdate     AuditAction = "role_update"
	ActionRoleDelete     AuditAction = "role_delete"
	ActionNodeCreate     AuditAction = "node_create"
	ActionNodeUpdate     AuditAction = "node_update"
	ActionNodeDelete     AuditAction = "node_delete"
	ActionNodeTokenGen   AuditAction = "node_token_generate"
	ActionNodeTokenRot   AuditAction = "node_token_rotate"
	ActionNodeTokenRev   AuditAction = "node_token_revoke"
	ActionPlanCreate     AuditAction = "plan_create"
	ActionPlanUpdate     AuditAction = "plan_update"
	ActionPlanDelete     AuditAction = "plan_delete"
	ActionPlanToggle     AuditAction = "plan_toggle"
	ActionIPWhitelistAdd AuditAction = "ip_whitelist_add"
	ActionIPWhitelistDel AuditAction = "ip_whitelist_delete"
	ActionIPWhitelistImp AuditAction = "ip_whitelist_import"
	ActionIPBlacklistAdd AuditAction = "ip_blacklist_add"
	ActionIPBlacklistDel AuditAction = "ip_blacklist_delete"
	ActionIPKick         AuditAction = "ip_kick"
	ActionCertCreate     AuditAction = "cert_create"
	ActionCertUpdate     AuditAction = "cert_update"
	ActionCertDelete     AuditAction = "cert_delete"
	ActionCertApply      AuditAction = "cert_apply"
	ActionCertRenew      AuditAction = "cert_renew"
	ActionCertAssign     AuditAction = "cert_assign"
	ActionCertUnassign   AuditAction = "cert_unassign"
	ActionNodeGroupCreate AuditAction = "nodegroup_create"
	ActionNodeGroupUpdate AuditAction = "nodegroup_update"
	ActionNodeGroupDelete AuditAction = "nodegroup_delete"
	ActionNodeGroupAddNode AuditAction = "nodegroup_add_node"
	ActionNodeGroupRemoveNode AuditAction = "nodegroup_remove_node"
	ActionSettingsUpdate AuditAction = "settings_update"
	ActionXrayRestart    AuditAction = "xray_restart"
	ActionXrayConfig     AuditAction = "xray_config_update"
)

// AuditResourceType represents a resource type.
type AuditResourceType string

// Resource types
const (
	ResourceUser     AuditResourceType = "user"
	ResourceProxy    AuditResourceType = "proxy"
	ResourceRole     AuditResourceType = "role"
	ResourceNode     AuditResourceType = "node"
	ResourceNodeGroup AuditResourceType = "nodegroup"
	ResourcePlan     AuditResourceType = "plan"
	ResourceIP       AuditResourceType = "ip"
	ResourceCert     AuditResourceType = "certificate"
	ResourceSettings AuditResourceType = "settings"
	ResourceXray     AuditResourceType = "xray"
	ResourceSystem   AuditResourceType = "system"
)

// AuditStatus represents the status of an audited operation.
type AuditStatus string

// Audit statuses
const (
	StatusSuccess AuditStatus = "success"
	StatusFailure AuditStatus = "failure"
)

// AuditEntry represents an audit log entry to be created.
type AuditEntry struct {
	UserID       *int64
	Username     string
	Action       AuditAction
	ResourceType AuditResourceType
	ResourceID   string
	Details      map[string]any
	IPAddress    string
	UserAgent    string
	RequestID    string
	Status       AuditStatus
}

// AuditService provides audit logging functionality.
type AuditService interface {
	Log(ctx context.Context, entry *AuditEntry) error
	LogSuccess(ctx context.Context, entry *AuditEntry) error
	LogFailure(ctx context.Context, entry *AuditEntry) error
	// SetEnabled toggles persistence of audit entries. Callers (the admin
	// UI "启用操作日志" switch) push the configured value here. When
	// disabled, Log/LogSuccess/LogFailure return nil immediately without
	// hitting the database; the structured logger is also skipped so the
	// audit stream is fully muted.
	SetEnabled(enabled bool)
	// Enabled reports whether audit persistence is currently active.
	Enabled() bool
}

// auditService implements AuditService.
type auditService struct {
	repo    repository.AuditLogRepository
	logger  logger.Logger
	enabled atomic.Bool
}

// NewAuditService creates a new audit service. Audit logging is enabled by
// default; call SetEnabled(false) to mute it.
func NewAuditService(repo repository.AuditLogRepository, log logger.Logger) AuditService {
	s := &auditService{
		repo:   repo,
		logger: log,
	}
	s.enabled.Store(true)
	return s
}

// SetEnabled toggles audit persistence.
func (s *auditService) SetEnabled(enabled bool) { s.enabled.Store(enabled) }

// Enabled reports whether audit persistence is on.
func (s *auditService) Enabled() bool { return s.enabled.Load() }

// Log logs an audit entry.
func (s *auditService) Log(ctx context.Context, entry *AuditEntry) error {
	if entry == nil {
		return nil
	}
	if !s.enabled.Load() {
		return nil
	}

	// Convert details to JSON
	var detailsJSON string
	if entry.Details != nil {
		data, err := json.Marshal(entry.Details)
		if err != nil {
			s.logger.Warn("failed to marshal audit details", logger.F("error", err))
		} else {
			detailsJSON = string(data)
		}
	}

	// Set default status
	status := string(entry.Status)
	if status == "" {
		status = string(StatusSuccess)
	}

	log := &repository.AuditLog{
		UserID:       entry.UserID,
		Username:     entry.Username,
		Action:       string(entry.Action),
		ResourceType: string(entry.ResourceType),
		ResourceID:   entry.ResourceID,
		Details:      detailsJSON,
		IPAddress:    entry.IPAddress,
		UserAgent:    entry.UserAgent,
		RequestID:    entry.RequestID,
		Status:       status,
	}

	if err := s.repo.Create(ctx, log); err != nil {
		s.logger.Error("failed to create audit log",
			logger.F("error", err),
			logger.F("action", entry.Action),
			logger.F("resource_type", entry.ResourceType),
		)
		return err
	}

	// Also log to structured logger for real-time monitoring
	s.logger.Info("audit",
		logger.Action(string(entry.Action)),
		logger.ResourceType(string(entry.ResourceType)),
		logger.ResourceID(entry.ResourceID),
		logger.F("status", status),
		logger.F("user_id", entry.UserID),
		logger.F("username", entry.Username),
	)

	return nil
}

// LogSuccess logs a successful audit entry.
func (s *auditService) LogSuccess(ctx context.Context, entry *AuditEntry) error {
	if entry != nil {
		entry.Status = StatusSuccess
	}
	return s.Log(ctx, entry)
}

// LogFailure logs a failed audit entry.
func (s *auditService) LogFailure(ctx context.Context, entry *AuditEntry) error {
	if entry != nil {
		entry.Status = StatusFailure
	}
	return s.Log(ctx, entry)
}

// NopAuditService is a no-operation audit service for testing.
type NopAuditService struct{}

// NewNopAuditService creates a new no-operation audit service.
func NewNopAuditService() AuditService {
	return &NopAuditService{}
}

func (s *NopAuditService) Log(ctx context.Context, entry *AuditEntry) error        { return nil }
func (s *NopAuditService) LogSuccess(ctx context.Context, entry *AuditEntry) error { return nil }
func (s *NopAuditService) LogFailure(ctx context.Context, entry *AuditEntry) error { return nil }
func (s *NopAuditService) SetEnabled(enabled bool)                                 {}
func (s *NopAuditService) Enabled() bool                                           { return false }
