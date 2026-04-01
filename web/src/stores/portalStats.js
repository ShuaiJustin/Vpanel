/**
 * 用户前台统计 Store
 * 管理流量统计和使用数据
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { stats as statsApi } from '@/api/modules/portal'
import { toNormalizedError } from '@/utils/entitlement'

export const usePortalStatsStore = defineStore('portalStats', () => {
  // 状态
  const trafficStats = ref([])
  const usageStats = ref({ by_node: [], by_protocol: [] })
  const loading = ref(false)
  const error = ref(null)

  // 计算属性
  const totalUpload = computed(() => {
    if (!Array.isArray(trafficStats.value)) return 0
    return trafficStats.value.reduce((sum, item) => sum + (item.upload || 0), 0)
  })

  const totalDownload = computed(() => {
    if (!Array.isArray(trafficStats.value)) return 0
    return trafficStats.value.reduce((sum, item) => sum + (item.download || 0), 0)
  })

  const totalTraffic = computed(() => totalUpload.value + totalDownload.value)

  const peakUsage = computed(() => {
    if (!Array.isArray(trafficStats.value) || !trafficStats.value.length) return null
    return trafficStats.value.reduce((max, item) => {
      const total = (item.upload || 0) + (item.download || 0)
      return total > max.total ? { ...item, total } : max
    }, { total: 0 })
  })

  // 方法
  async function fetchStats(params = {}) {
    loading.value = true
    error.value = null
    try {
      const previousTrafficStats = Array.isArray(trafficStats.value) ? [...trafficStats.value] : []
      const previousUsageStats = {
        by_node: Array.isArray(usageStats.value?.by_node) ? [...usageStats.value.by_node] : [],
        by_protocol: Array.isArray(usageStats.value?.by_protocol) ? [...usageStats.value.by_protocol] : []
      }

      const [trafficResult, usageResult] = await Promise.allSettled([
        statsApi.getTrafficStats(params),
        statsApi.getUsageStats(params)
      ])

      if (trafficResult.status === 'rejected') {
        console.error('获取流量统计失败:', trafficResult.reason)
      }
      if (usageResult.status === 'rejected') {
        console.error('获取使用统计失败:', usageResult.reason)
      }
      if (trafficResult.status === 'rejected' && usageResult.status === 'rejected') {
        throw trafficResult.reason || usageResult.reason
      }

      const trafficResponse = trafficResult.status === 'fulfilled'
        ? trafficResult.value
        : {
            total_upload: previousTrafficStats.reduce((sum, item) => sum + (item?.upload || 0), 0),
            total_download: previousTrafficStats.reduce((sum, item) => sum + (item?.download || 0), 0),
            total_traffic: previousTrafficStats.reduce((sum, item) => sum + (item?.upload || 0) + (item?.download || 0), 0),
            daily: previousTrafficStats
          }

      const usageResponse = usageResult.status === 'fulfilled'
        ? usageResult.value
        : previousUsageStats

      const daily = Array.isArray(trafficResponse?.daily) ? trafficResponse.daily : previousTrafficStats
      trafficStats.value = daily
      usageStats.value = {
        by_node: Array.isArray(usageResponse?.by_node) ? usageResponse.by_node : previousUsageStats.by_node,
        by_protocol: Array.isArray(usageResponse?.by_protocol) ? usageResponse.by_protocol : previousUsageStats.by_protocol
      }

      // 组合数据
      const data = {
        summary: {
          upload: trafficResponse?.total_upload || 0,
          download: trafficResponse?.total_download || 0,
          total: trafficResponse?.total_traffic || 0,
          nodes: Array.isArray(usageResponse?.by_node) ? usageResponse.by_node.length : 0
        },
        node_usage: Array.isArray(usageResponse?.by_node) ? usageResponse.by_node : [],
        protocol_usage: Array.isArray(usageResponse?.by_protocol) ? usageResponse.by_protocol : [],
        records: daily,
        chart_data: {
          labels: daily.map(d => d?.date || ''),
          upload: daily.map(d => d?.upload || 0),
          download: daily.map(d => d?.download || 0)
        }
      }
      
      return data
    } catch (err) {
      console.error('获取统计数据失败:', err)
      const normalizedError = toNormalizedError(err, '获取统计数据失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function exportStats(params = {}) {
    try {
      const blob = await statsApi.exportStats({
        format: 'csv',
        ...params
      })
      // 创建下载链接
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      const now = new Date()
      const localDate = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
      link.href = url
      link.download = `traffic-stats-${localDate}.csv`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
    } catch (err) {
      const normalizedError = toNormalizedError(err, '导出统计数据失败')
      error.value = normalizedError.message
      throw normalizedError
    }
  }

  return {
    // 状态
    trafficStats,
    usageStats,
    loading,
    error,
    // 计算属性
    totalUpload,
    totalDownload,
    totalTraffic,
    peakUsage,
    // 方法
    fetchStats,
    exportStats
  }
})
