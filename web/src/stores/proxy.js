/**
 * 代理状态管理
 * 管理代理列表、当前代理、加载状态和错误状态
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { proxiesApi } from '@/api'

export const useProxyStore = defineStore('proxy', () => {
  // 状态
  const proxies = ref([])
  const currentProxy = ref(null)
  const total = ref(0)
  const loading = ref(false)
  const error = ref(null)
  const filters = ref({
    protocol: '',
    enabled: null,
    search: ''
  })

  // 计算属性
  const activeProxies = computed(() => 
    proxies.value.filter(p => p.enabled && p.running)
  )
  
  const proxyCount = computed(() => proxies.value.length)
  
  const activeCount = computed(() => activeProxies.value.length)

  const filteredProxies = computed(() => {
    let result = proxies.value
    
    if (filters.value.protocol) {
      result = result.filter(p => p.protocol === filters.value.protocol)
    }
    
    if (filters.value.enabled !== null) {
      result = result.filter(p => p.enabled === filters.value.enabled)
    }
    
    if (filters.value.search) {
      const search = filters.value.search.toLowerCase()
      result = result.filter(p => 
        p.name.toLowerCase().includes(search) ||
        p.remark?.toLowerCase().includes(search)
      )
    }
    
    return result
  })

  // 方法
  const setFilters = (newFilters) => {
    filters.value = { ...filters.value, ...newFilters }
  }

  const clearFilters = () => {
    filters.value = { protocol: '', enabled: null, search: '' }
  }

  // API 方法
  const fetchProxies = async (params = {}) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await proxiesApi.list(params)
      
      if (Array.isArray(response)) {
        proxies.value = response
        total.value = response.length
      } else if (response?.data) {
        proxies.value = response.data
        total.value = response.total || response.data.length
      } else if (response?.proxies) {
        proxies.value = response.proxies
        total.value = response.total || response.proxies.length
      }
      
      return { proxies: proxies.value, total: total.value }
    } catch (err) {
      error.value = err.message || '获取代理列表失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchProxy = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await proxiesApi.get(id)
      currentProxy.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取代理详情失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const createProxy = async (data) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await proxiesApi.create(data)
      
      // 乐观更新
      proxies.value.push(response)
      total.value++
      
      return response
    } catch (err) {
      error.value = err.message || '创建代理失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const updateProxy = async (id, data) => {
    loading.value = true
    error.value = null
    
    // 保存原始数据用于回滚
    const index = proxies.value.findIndex(p => p.id === id)
    const originalProxy = index >= 0 ? { ...proxies.value[index] } : null
    
    try {
      // 乐观更新
      if (index >= 0) {
        proxies.value[index] = { ...proxies.value[index], ...data }
      }
      
      const response = await proxiesApi.update(id, data)
      
      // 更新为服务器返回的数据
      if (index >= 0) {
        proxies.value[index] = response
      }
      
      if (currentProxy.value?.id === id) {
        currentProxy.value = response
      }
      
      return response
    } catch (err) {
      // 回滚
      if (index >= 0 && originalProxy) {
        proxies.value[index] = originalProxy
      }
      
      error.value = err.message || '更新代理失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const deleteProxy = async (id) => {
    loading.value = true
    error.value = null
    
    // 保存原始数据用于回滚
    const index = proxies.value.findIndex(p => p.id === id)
    const originalProxy = index >= 0 ? proxies.value[index] : null
    
    try {
      // 乐观更新
      if (index >= 0) {
        proxies.value.splice(index, 1)
        total.value--
      }
      
      await proxiesApi.delete(id)
      
      if (currentProxy.value?.id === id) {
        currentProxy.value = null
      }
      
      return true
    } catch (err) {
      // 回滚
      if (originalProxy) {
        proxies.value.splice(index, 0, originalProxy)
        total.value++
      }
      
      error.value = err.message || '删除代理失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const startProxy = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      await proxiesApi.start(id)
      
      // 更新本地状态
      const proxy = proxies.value.find(p => p.id === id)
      if (proxy) {
        proxy.running = true
      }
      
      return true
    } catch (err) {
      error.value = err.message || '启动代理失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const stopProxy = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      await proxiesApi.stop(id)
      
      // 更新本地状态
      const proxy = proxies.value.find(p => p.id === id)
      if (proxy) {
        proxy.running = false
      }
      
      return true
    } catch (err) {
      error.value = err.message || '停止代理失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const getProxyStats = async (id) => {
    try {
      return await proxiesApi.getStats(id)
    } catch (err) {
      error.value = err.message || '获取代理统计失败'
      throw err
    }
  }

  const batchEnable = async (ids) => {
    loading.value = true
    error.value = null
    
    try {
      await proxiesApi.batchEnable(ids)
      
      // 更新本地状态
      proxies.value.forEach(p => {
        if (ids.includes(p.id)) {
          p.enabled = true
        }
      })
      
      return true
    } catch (err) {
      error.value = err.message || '批量启用失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const batchDisable = async (ids) => {
    loading.value = true
    error.value = null
    
    try {
      await proxiesApi.batchDisable(ids)
      
      // 更新本地状态
      proxies.value.forEach(p => {
        if (ids.includes(p.id)) {
          p.enabled = false
        }
      })
      
      return true
    } catch (err) {
      error.value = err.message || '批量禁用失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const batchDelete = async (ids) => {
    loading.value = true
    error.value = null
    
    try {
      await proxiesApi.batchDelete(ids)
      
      // 更新本地状态
      proxies.value = proxies.value.filter(p => !ids.includes(p.id))
      total.value -= ids.length
      
      return true
    } catch (err) {
      error.value = err.message || '批量删除失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  return {
    // 状态
    proxies,
    currentProxy,
    total,
    loading,
    error,
    filters,
    
    // 计算属性
    activeProxies,
    proxyCount,
    activeCount,
    filteredProxies,
    
    // 方法
    setFilters,
    clearFilters,
    fetchProxies,
    fetchProxy,
    createProxy,
    updateProxy,
    deleteProxy,
    startProxy,
    stopProxy,
    getProxyStats,
    batchEnable,
    batchDisable,
    batchDelete
  }
})
