/**
 * 节点状态管理
 * 管理节点列表、当前节点、加载状态和错误状态
 * Requirements: 1.1-1.7
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { nodesApi } from '@/api'

export const useNodeStore = defineStore('node', () => {
  // 状态
  const nodes = ref([])
  const currentNode = ref(null)
  const total = ref(0)
  const loading = ref(false)
  const error = ref(null)
  const statistics = ref(null)
  const clusterHealth = ref(null)
  const filters = ref({
    status: '',
    region: '',
    groupId: null,
    search: ''
  })

  // 计算属性
  const onlineNodes = computed(() => 
    nodes.value.filter(n => n.status === 'online')
  )
  
  const offlineNodes = computed(() => 
    nodes.value.filter(n => n.status === 'offline')
  )
  
  const unhealthyNodes = computed(() => 
    nodes.value.filter(n => n.status === 'unhealthy')
  )
  
  const nodeCount = computed(() => nodes.value.length)
  
  const onlineCount = computed(() => onlineNodes.value.length)
  
  const healthyPercentage = computed(() => {
    if (nodes.value.length === 0) return 0
    return Math.round((onlineNodes.value.length / nodes.value.length) * 100)
  })

  const filteredNodes = computed(() => {
    let result = nodes.value
    
    if (filters.value.status) {
      result = result.filter(n => n.status === filters.value.status)
    }
    
    if (filters.value.region) {
      result = result.filter(n => n.region === filters.value.region)
    }
    
    if (filters.value.search) {
      const search = filters.value.search.toLowerCase()
      result = result.filter(n => 
        n.name.toLowerCase().includes(search) ||
        n.address?.toLowerCase().includes(search) ||
        n.region?.toLowerCase().includes(search)
      )
    }
    
    return result
  })

  const totalUsers = computed(() => 
    nodes.value.reduce((sum, n) => sum + (n.current_users || 0), 0)
  )

  const averageLatency = computed(() => {
    const onlineWithLatency = nodes.value.filter(n => n.status === 'online' && n.latency > 0)
    if (onlineWithLatency.length === 0) return 0
    return Math.round(
      onlineWithLatency.reduce((sum, n) => sum + n.latency, 0) / onlineWithLatency.length
    )
  })

  // 方法
  const setFilters = (newFilters) => {
    filters.value = { ...filters.value, ...newFilters }
  }

  const clearFilters = () => {
    filters.value = { status: '', region: '', groupId: null, search: '' }
  }

  // API 方法
  const fetchNodes = async (params = {}) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodesApi.list(params)
      
      if (Array.isArray(response)) {
        nodes.value = response
        total.value = response.length
      } else if (response?.nodes) {
        nodes.value = response.nodes
        total.value = response.total || response.nodes.length
      } else if (response?.data) {
        nodes.value = response.data
        total.value = response.total || response.data.length
      }
      
      return { nodes: nodes.value, total: total.value }
    } catch (err) {
      error.value = err.message || '获取节点列表失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchNode = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodesApi.get(id)
      currentNode.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取节点详情失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const createNode = async (data) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodesApi.create(data)
      
      // 添加到列表
      nodes.value.push(response)
      total.value++
      
      return response
    } catch (err) {
      error.value = err.message || '创建节点失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const updateNode = async (id, data) => {
    loading.value = true
    error.value = null
    
    const index = nodes.value.findIndex(n => n.id === id)
    const originalNode = index >= 0 ? { ...nodes.value[index] } : null
    
    try {
      // 乐观更新
      if (index >= 0) {
        nodes.value[index] = { ...nodes.value[index], ...data }
      }
      
      const response = await nodesApi.update(id, data)
      
      // 更新为服务器返回的数据
      if (index >= 0) {
        nodes.value[index] = response
      }
      
      if (currentNode.value?.id === id) {
        currentNode.value = response
      }
      
      return response
    } catch (err) {
      // 回滚
      if (index >= 0 && originalNode) {
        nodes.value[index] = originalNode
      }
      
      error.value = err.message || '更新节点失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const deleteNode = async (id) => {
    loading.value = true
    error.value = null
    
    const index = nodes.value.findIndex(n => n.id === id)
    const originalNode = index >= 0 ? nodes.value[index] : null
    
    try {
      // 乐观更新
      if (index >= 0) {
        nodes.value.splice(index, 1)
        total.value--
      }
      
      await nodesApi.delete(id)
      
      if (currentNode.value?.id === id) {
        currentNode.value = null
      }
      
      return true
    } catch (err) {
      // 回滚
      if (originalNode) {
        nodes.value.splice(index, 0, originalNode)
        total.value++
      }
      
      error.value = err.message || '删除节点失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const updateNodeStatus = async (id, status) => {
    loading.value = true
    error.value = null
    
    try {
      await nodesApi.updateStatus(id, status)
      
      // 更新本地状态
      const node = nodes.value.find(n => n.id === id)
      if (node) {
        node.status = status
      }
      
      if (currentNode.value?.id === id) {
        currentNode.value.status = status
      }
      
      return true
    } catch (err) {
      error.value = err.message || '更新节点状态失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  // Token 管理
  const generateToken = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodesApi.generateToken(id)
      return response
    } catch (err) {
      error.value = err.message || '生成 Token 失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const rotateToken = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodesApi.rotateToken(id)
      return response
    } catch (err) {
      error.value = err.message || '轮换 Token 失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const revokeToken = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      await nodesApi.revokeToken(id)
      return true
    } catch (err) {
      error.value = err.message || '撤销 Token 失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const startNodeCore = async (id) => {
    loading.value = true
    error.value = null

    try {
      return await nodesApi.startCore(id)
    } catch (err) {
      error.value = err.message || '启动节点内核失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const restartNodeCore = async (id) => {
    loading.value = true
    error.value = null

    try {
      return await nodesApi.restartCore(id)
    } catch (err) {
      error.value = err.message || '重启节点内核失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const syncNodeCoreConfig = async (id) => {
    loading.value = true
    error.value = null

    try {
      return await nodesApi.syncCoreConfig(id)
    } catch (err) {
      error.value = err.message || '同步节点配置失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const installNodeCoreVersion = async (id, version = '') => {
    loading.value = true
    error.value = null

    try {
      return await nodesApi.installCoreVersion(id, version)
    } catch (err) {
      error.value = err.message || '切换节点内核版本失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  // 统计和健康
  const fetchStatistics = async () => {
    error.value = null
    try {
      const response = await nodesApi.getStatistics()
      statistics.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取节点统计失败'
      throw err
    }
  }

  const fetchClusterHealth = async () => {
    error.value = null
    try {
      const response = await nodesApi.getClusterHealth()
      clusterHealth.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取集群健康状态失败'
      throw err
    }
  }

  // 流量统计
  const getNodeTraffic = async (id, params) => {
    try {
      return await nodesApi.getTraffic(id, params)
    } catch (err) {
      error.value = err.message || '获取节点流量失败'
      throw err
    }
  }

  const getTopUsers = async (id, params) => {
    try {
      return await nodesApi.getTopUsers(id, params)
    } catch (err) {
      error.value = err.message || '获取 Top 用户失败'
      throw err
    }
  }

  const getTrafficDiagnostic = async (id) => {
    try {
      return await nodesApi.getTrafficDiagnostic(id)
    } catch (err) {
      error.value = err.message || '获取流量诊断失败'
      throw err
    }
  }

  const getTotalTraffic = async (params) => {
    try {
      return await nodesApi.getTotalTraffic(params)
    } catch (err) {
      error.value = err.message || '获取总流量失败'
      throw err
    }
  }

  const getTrafficByNode = async (params) => {
    try {
      return await nodesApi.getTrafficByNode(params)
    } catch (err) {
      error.value = err.message || '获取节点流量统计失败'
      throw err
    }
  }

  const getRealTimeStats = async () => {
    try {
      return await nodesApi.getRealTimeStats()
    } catch (err) {
      error.value = err.message || '获取实时统计失败'
      throw err
    }
  }

  // 清除状态
  const clearCurrentNode = () => {
    currentNode.value = null
  }

  const clearError = () => {
    error.value = null
  }

  return {
    // 状态
    nodes,
    currentNode,
    total,
    loading,
    error,
    statistics,
    clusterHealth,
    filters,
    
    // 计算属性
    onlineNodes,
    offlineNodes,
    unhealthyNodes,
    nodeCount,
    onlineCount,
    healthyPercentage,
    filteredNodes,
    totalUsers,
    averageLatency,
    
    // 方法
    setFilters,
    clearFilters,
    fetchNodes,
    fetchNode,
    createNode,
    updateNode,
    deleteNode,
    updateNodeStatus,
    generateToken,
    rotateToken,
    revokeToken,
    startNodeCore,
    restartNodeCore,
    syncNodeCoreConfig,
    installNodeCoreVersion,
    fetchStatistics,
    fetchClusterHealth,
    getNodeTraffic,
    getTrafficDiagnostic,
    getTopUsers,
    getTotalTraffic,
    getTrafficByNode,
    getRealTimeStats,
    clearCurrentNode,
    clearError
  }
})
