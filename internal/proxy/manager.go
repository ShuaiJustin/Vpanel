package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"v/internal/cache"
	"v/internal/database/repository"
	"v/pkg/errors"
)

// Manager manages proxy protocols and configurations.
type Manager interface {
	// RegisterProtocol registers a protocol implementation.
	RegisterProtocol(protocol Protocol)

	// GetProtocol returns a protocol by name.
	GetProtocol(name string) (Protocol, bool)

	// ListProtocols returns all registered protocols.
	ListProtocols() []string

	// CreateProxy creates a new proxy configuration.
	CreateProxy(ctx context.Context, settings *Settings) error

	// UpdateProxy updates a proxy configuration.
	UpdateProxy(ctx context.Context, settings *Settings) error

	// DeleteProxy deletes a proxy configuration.
	DeleteProxy(ctx context.Context, id int64) error

	// GetProxy retrieves a proxy configuration.
	GetProxy(ctx context.Context, id int64) (*Settings, error)

	// ListProxies lists all proxy configurations.
	ListProxies(ctx context.Context, page, pageSize int) ([]*Settings, int64, error)

	// GetProxiesByUser retrieves proxies for a user.
	GetProxiesByUser(ctx context.Context, userID int64) ([]*Settings, error)

	// GenerateLink generates a share link for a proxy.
	GenerateLink(ctx context.Context, id int64) (string, error)

	// GenerateConfig generates Xray configuration for a proxy.
	GenerateConfig(ctx context.Context, id int64) (json.RawMessage, error)
}

// manager implements Manager.
type manager struct {
	mu          sync.RWMutex
	protocols   map[string]Protocol
	proxyRepo   repository.ProxyRepository
	cache       cache.Cache
	invalidator *cache.Invalidator
}

// NewManager creates a new proxy manager.
func NewManager(proxyRepo repository.ProxyRepository) Manager {
	return &manager{
		protocols: make(map[string]Protocol),
		proxyRepo: proxyRepo,
	}
}

// NewManagerWithCache creates a new proxy manager with caching support.
func NewManagerWithCache(proxyRepo repository.ProxyRepository, cacheInstance cache.Cache) Manager {
	return &manager{
		protocols:   make(map[string]Protocol),
		proxyRepo:   proxyRepo,
		cache:       cacheInstance,
		invalidator: cache.NewInvalidator(cacheInstance),
	}
}

// RegisterProtocol registers a protocol implementation.
func (m *manager) RegisterProtocol(protocol Protocol) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.protocols[protocol.Name()] = protocol
}

// GetProtocol returns a protocol by name.
func (m *manager) GetProtocol(name string) (Protocol, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.protocols[name]
	return p, ok
}

// ListProtocols returns all registered protocols.
func (m *manager) ListProtocols() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.protocols))
	for name := range m.protocols {
		names = append(names, name)
	}
	return names
}

// CreateProxy creates a new proxy configuration.
func (m *manager) CreateProxy(ctx context.Context, settings *Settings) error {
	protocol, ok := m.GetProtocol(settings.Protocol)
	if !ok {
		return errors.NewValidationError("unsupported protocol", settings.Protocol)
	}

	if err := protocol.Validate(settings); err != nil {
		return err
	}

	proxy := &repository.Proxy{
		Name:     settings.Name,
		Protocol: settings.Protocol,
		Port:     settings.Port,
		Host:     settings.Host,
		Settings: settings.Settings,
		Enabled:  settings.Enabled,
		Remark:   settings.Remark,
	}

	return m.proxyRepo.Create(ctx, proxy)
}

// UpdateProxy updates a proxy configuration.
func (m *manager) UpdateProxy(ctx context.Context, settings *Settings) error {
	protocol, ok := m.GetProtocol(settings.Protocol)
	if !ok {
		return errors.NewValidationError("unsupported protocol", settings.Protocol)
	}

	if err := protocol.Validate(settings); err != nil {
		return err
	}

	proxy, err := m.proxyRepo.GetByID(ctx, settings.ID)
	if err != nil {
		return err
	}

	proxy.Name = settings.Name
	proxy.Protocol = settings.Protocol
	proxy.Port = settings.Port
	proxy.Host = settings.Host
	proxy.Settings = settings.Settings
	proxy.Enabled = settings.Enabled
	proxy.Remark = settings.Remark

	if err := m.proxyRepo.Update(ctx, proxy); err != nil {
		return err
	}

	// Invalidate cache after successful update
	if m.invalidator != nil {
		_ = m.invalidator.InvalidateProxyConfig(ctx, uint(settings.ID))
	}

	return nil
}

// DeleteProxy deletes a proxy configuration.
func (m *manager) DeleteProxy(ctx context.Context, id int64) error {
	if err := m.proxyRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Invalidate cache after successful deletion
	if m.invalidator != nil {
		_ = m.invalidator.InvalidateProxyConfig(ctx, uint(id))
	}

	return nil
}

// GetProxy retrieves a proxy configuration.
func (m *manager) GetProxy(ctx context.Context, id int64) (*Settings, error) {
	// Try cache first if caching is enabled
	if m.cache != nil {
		cacheKey := fmt.Sprintf(cache.KeyProxyConfig, id)
		if data, err := m.cache.Get(ctx, cacheKey); err == nil {
			// Cache hit - deserialize and return
			var proxy repository.Proxy
			if err := json.Unmarshal(data, &proxy); err == nil {
				return m.proxyToSettings(&proxy), nil
			}
			// If deserialization fails, fall through to database query
		}
	}

	// Cache miss or caching disabled - query database
	proxy, err := m.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Store in cache if caching is enabled
	if m.cache != nil {
		cacheKey := fmt.Sprintf(cache.KeyProxyConfig, id)
		if data, err := json.Marshal(proxy); err == nil {
			ttl := cache.GetTTL("proxy:config")
			_ = m.cache.Set(ctx, cacheKey, data, ttl)
		}
	}

	return m.proxyToSettings(proxy), nil
}

// ListProxies lists proxy configurations with pagination.
func (m *manager) ListProxies(ctx context.Context, page, pageSize int) ([]*Settings, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	proxies, err := m.proxyRepo.List(ctx, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := m.proxyRepo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	settings := make([]*Settings, len(proxies))
	for i, proxy := range proxies {
		settings[i] = m.proxyToSettings(proxy)
	}

	return settings, total, nil
}

// GetProxiesByUser retrieves proxies for a user.
func (m *manager) GetProxiesByUser(ctx context.Context, userID int64) ([]*Settings, error) {
	proxies, err := m.proxyRepo.GetByUserID(ctx, userID, 1000, 0)
	if err != nil {
		return nil, err
	}

	settings := make([]*Settings, len(proxies))
	for i, proxy := range proxies {
		settings[i] = m.proxyToSettings(proxy)
	}

	return settings, nil
}

// GenerateLink generates a share link for a proxy.
func (m *manager) GenerateLink(ctx context.Context, id int64) (string, error) {
	settings, err := m.GetProxy(ctx, id)
	if err != nil {
		return "", err
	}

	protocol, ok := m.GetProtocol(settings.Protocol)
	if !ok {
		return "", errors.NewValidationError("unsupported protocol", settings.Protocol)
	}

	return protocol.GenerateLink(settings)
}

// GenerateConfig generates Xray configuration for a proxy.
func (m *manager) GenerateConfig(ctx context.Context, id int64) (json.RawMessage, error) {
	settings, err := m.GetProxy(ctx, id)
	if err != nil {
		return nil, err
	}

	protocol, ok := m.GetProtocol(settings.Protocol)
	if !ok {
		return nil, errors.NewValidationError("unsupported protocol", settings.Protocol)
	}

	return protocol.GenerateConfig(settings)
}

// proxyToSettings converts a repository.Proxy to Settings.
func (m *manager) proxyToSettings(proxy *repository.Proxy) *Settings {
	return &Settings{
		ID:       proxy.ID,
		Name:     proxy.Name,
		Protocol: proxy.Protocol,
		Port:     proxy.Port,
		Host:     proxy.Host,
		Settings: proxy.Settings,
		Enabled:  proxy.Enabled,
		Remark:   proxy.Remark,
	}
}
