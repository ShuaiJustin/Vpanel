/**
 * 系统状态管理
 * 管理系统信息、状态、统计数据
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { systemApi, statsApi } from '@/api'

export const useSystemStore = defineStore('system', () => {
  // 状态
  const info = ref(null)
  const status = ref(null)
  const stats = ref(null)
  const loading = ref(false)
  const error = ref(null)
  const lastUpdated = ref(null)

  // 计算属性
  const cpuUsage = computed(() => status.value?.cpuUsage || 0)
  const memoryUsage = computed(() => status.value?.memoryUsage || 0)
  const diskUsage = computed(() => status.value?.diskUsage || 0)
  
  const isHealthy = computed(() => {
    if (!status.value) return false
    return cpuUsage.value < 90 && memoryUsage.value < 90 && diskUsage.value < 90
  })

  const totalUsers = computed(() => stats.value?.totalUsers || 0)
  const activeUsers = computed(() => stats.value?.activeUsers || 0)
  const totalProxies = computed(() => stats.value?.totalProxies || 0)
  const activeProxies = computed(() => stats.value?.activeProxies || 0)
  const totalTraffic = computed(() => stats.value?.totalTraffic || 0)

  // API 方法
  const fetchInfo = async () => {
    loading.value = true
    error.value = null
    
    try {
      const response = await systemApi.getInfo()
      info.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取系统信息失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchStatus = async () => {
    loading.value = true
    error.value = null
    
    try {
      const response = await systemApi.getStatus({ include_processes: false })
      status.value = response
      lastUpdated.value = new Date().toISOString()
      return response
    } catch (err) {
      error.value = err.message || '获取系统状态失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchStats = async () => {
    loading.value = true
    error.value = null
    
    try {
      const response = await statsApi.getDashboard()
      stats.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取统计数据失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchAll = async () => {
    loading.value = true
    error.value = null
    
    try {
      const [infoRes, statusRes, statsRes] = await Promise.allSettled([
        systemApi.getInfo(),
        systemApi.getStatus({ include_processes: false }),
        statsApi.getDashboard()
      ])
      
      if (infoRes.status === 'fulfilled') {
        info.value = infoRes.value
      }
      
      if (statusRes.status === 'fulfilled') {
        status.value = statusRes.value
      }
      
      if (statsRes.status === 'fulfilled') {
        stats.value = statsRes.value
      }
      
      lastUpdated.value = new Date().toISOString()
      
      return { info: info.value, status: status.value, stats: stats.value }
    } catch (err) {
      error.value = err.message || '获取系统数据失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const getCpuTrend = async (timeRange) => {
    try {
      return await systemApi.getCpuTrend(timeRange)
    } catch (err) {
      error.value = err.message || '获取 CPU 趋势失败'
      throw err
    }
  }

  const getMemoryTrend = async (timeRange) => {
    try {
      return await systemApi.getMemoryTrend(timeRange)
    } catch (err) {
      error.value = err.message || '获取内存趋势失败'
      throw err
    }
  }

  const getNetworkTrend = async (timeRange) => {
    try {
      return await systemApi.getNetworkTrend(timeRange)
    } catch (err) {
      error.value = err.message || '获取网络趋势失败'
      throw err
    }
  }

  const getProcesses = async () => {
    try {
      return await systemApi.getProcesses()
    } catch (err) {
      error.value = err.message || '获取进程列表失败'
      throw err
    }
  }

  const killProcess = async (pid) => {
    try {
      return await systemApi.killProcess(pid)
    } catch (err) {
      error.value = err.message || '终止进程失败'
      throw err
    }
  }

  return {
    // 状态
    info,
    status,
    stats,
    loading,
    error,
    lastUpdated,
    
    // 计算属性
    cpuUsage,
    memoryUsage,
    diskUsage,
    isHealthy,
    totalUsers,
    activeUsers,
    totalProxies,
    activeProxies,
    totalTraffic,
    
    // 方法
    fetchInfo,
    fetchStatus,
    fetchStats,
    fetchAll,
    getCpuTrend,
    getMemoryTrend,
    getNetworkTrend,
    getProcesses,
    killProcess
  }
})
