import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const portalStatsMocks = vi.hoisted(() => ({
  getTrafficStats: vi.fn(),
  getUsageStats: vi.fn(),
  exportStats: vi.fn(),
}))

vi.mock('@/api/modules/portal', () => ({
  stats: {
    getTrafficStats: portalStatsMocks.getTrafficStats,
    getUsageStats: portalStatsMocks.getUsageStats,
    exportStats: portalStatsMocks.exportStats,
  },
}))

vi.mock('@/utils/entitlement', () => ({
  toNormalizedError: vi.fn((error, fallbackMessage) => ({
    message: error?.message || fallbackMessage,
  })),
}))

import { usePortalStatsStore } from './portalStats'

describe('portalStats store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    setActivePinia(createPinia())
  })

  it('preserves last successful traffic data when usage request only partially succeeds', async () => {
    const store = usePortalStatsStore()

    portalStatsMocks.getTrafficStats.mockResolvedValueOnce({
      total_upload: 10,
      total_download: 20,
      total_traffic: 30,
      daily: [{ date: '2026-03-21', upload: 10, download: 20 }],
    })
    portalStatsMocks.getUsageStats.mockResolvedValueOnce({
      by_node: [{ node_id: 1, node_name: 'node-1', traffic: 30 }],
      by_protocol: [{ protocol: 'vmess', traffic: 30 }],
    })

    const initial = await store.fetchStats({ period: 'week' })
    expect(initial.summary.total).toBe(30)
    expect(store.trafficStats).toHaveLength(1)
    expect(store.usageStats.by_node).toHaveLength(1)

    portalStatsMocks.getTrafficStats.mockRejectedValueOnce(new Error('traffic failed'))
    portalStatsMocks.getUsageStats.mockResolvedValueOnce({
      by_node: [{ node_id: 2, node_name: 'node-2', traffic: 30 }],
      by_protocol: [{ protocol: 'trojan', traffic: 30 }],
    })

    const partial = await store.fetchStats({ period: 'week' })
    expect(partial.summary.upload).toBe(10)
    expect(partial.summary.download).toBe(20)
    expect(partial.summary.total).toBe(30)
    expect(store.trafficStats).toEqual([{ date: '2026-03-21', upload: 10, download: 20 }])
    expect(store.usageStats.by_node).toEqual([{ node_id: 2, node_name: 'node-2', traffic: 30 }])
  })
})
