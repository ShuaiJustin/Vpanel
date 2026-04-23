package xray

import (
	"context"

	"v/internal/database/repository"
	apperrors "v/pkg/errors"
)

type mockProxyRepoForSync struct {
	proxies []*repository.Proxy
	nextID  int64
}

func newMockProxyRepoForSync() *mockProxyRepoForSync {
	return &mockProxyRepoForSync{nextID: 1}
}

func (m *mockProxyRepoForSync) Create(ctx context.Context, proxy *repository.Proxy) error {
	if proxy.ID == 0 {
		proxy.ID = m.nextID
		m.nextID++
	}
	m.proxies = append(m.proxies, proxy)
	return nil
}

func (m *mockProxyRepoForSync) GetByID(ctx context.Context, id int64) (*repository.Proxy, error) {
	for _, proxy := range m.proxies {
		if proxy.ID == id {
			return proxy, nil
		}
	}
	return nil, apperrors.NewNotFoundError("proxy", id)
}

func (m *mockProxyRepoForSync) Update(ctx context.Context, proxy *repository.Proxy) error {
	for i, existing := range m.proxies {
		if existing.ID == proxy.ID {
			m.proxies[i] = proxy
			return nil
		}
	}
	return apperrors.NewNotFoundError("proxy", proxy.ID)
}

func (m *mockProxyRepoForSync) Delete(ctx context.Context, id int64) error {
	for i, proxy := range m.proxies {
		if proxy.ID == id {
			m.proxies = append(m.proxies[:i], m.proxies[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFoundError("proxy", id)
}

func (m *mockProxyRepoForSync) List(ctx context.Context, limit, offset int) ([]*repository.Proxy, error) {
	return paginateMockProxies(m.proxies, limit, offset), nil
}

func (m *mockProxyRepoForSync) GetByProtocol(ctx context.Context, protocol string) ([]*repository.Proxy, error) {
	result := make([]*repository.Proxy, 0)
	for _, proxy := range m.proxies {
		if proxy.Protocol == protocol {
			result = append(result, proxy)
		}
	}
	return result, nil
}

func (m *mockProxyRepoForSync) GetEnabled(ctx context.Context) ([]*repository.Proxy, error) {
	result := make([]*repository.Proxy, 0)
	for _, proxy := range m.proxies {
		if proxy.Enabled {
			result = append(result, proxy)
		}
	}
	return result, nil
}

func (m *mockProxyRepoForSync) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*repository.Proxy, error) {
	result := make([]*repository.Proxy, 0)
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			result = append(result, proxy)
		}
	}
	return paginateMockProxies(result, limit, offset), nil
}

func (m *mockProxyRepoForSync) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	var count int64
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockProxyRepoForSync) GetByPort(ctx context.Context, port int) (*repository.Proxy, error) {
	for _, proxy := range m.proxies {
		if proxy.Port == port {
			return proxy, nil
		}
	}
	return nil, nil
}

func (m *mockProxyRepoForSync) GetByNodeID(ctx context.Context, nodeID int64) ([]*repository.Proxy, error) {
	result := make([]*repository.Proxy, 0)
	for _, proxy := range m.proxies {
		if proxy.NodeID != nil && *proxy.NodeID == nodeID {
			result = append(result, proxy)
		}
	}
	return result, nil
}

func (m *mockProxyRepoForSync) DeleteByIDs(ctx context.Context, ids []int64) error {
	remove := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		remove[id] = struct{}{}
	}
	kept := m.proxies[:0]
	for _, proxy := range m.proxies {
		if _, ok := remove[proxy.ID]; !ok {
			kept = append(kept, proxy)
		}
	}
	m.proxies = kept
	return nil
}

func (m *mockProxyRepoForSync) EnableByUserID(ctx context.Context, userID int64) error {
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			proxy.Enabled = true
		}
	}
	return nil
}

func (m *mockProxyRepoForSync) DisableByUserID(ctx context.Context, userID int64) error {
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			proxy.Enabled = false
		}
	}
	return nil
}

func (m *mockProxyRepoForSync) Count(ctx context.Context) (int64, error) {
	return int64(len(m.proxies)), nil
}

func (m *mockProxyRepoForSync) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	for _, proxy := range m.proxies {
		if proxy.Enabled {
			count++
		}
	}
	return count, nil
}

func (m *mockProxyRepoForSync) CountByProtocol(ctx context.Context) ([]*repository.ProtocolCount, error) {
	counts := map[string]int64{}
	for _, proxy := range m.proxies {
		counts[proxy.Protocol]++
	}
	result := make([]*repository.ProtocolCount, 0, len(counts))
	for protocol, count := range counts {
		result = append(result, &repository.ProtocolCount{Protocol: protocol, Count: count})
	}
	return result, nil
}

func paginateMockProxies(proxies []*repository.Proxy, limit, offset int) []*repository.Proxy {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(proxies) {
		return []*repository.Proxy{}
	}
	end := len(proxies)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return proxies[offset:end]
}
