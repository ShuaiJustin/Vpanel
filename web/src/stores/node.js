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
  const regions = ref([])
  const loading = ref(false)
  const error = ref(null)
  const statistics = ref(null)
  const clusterHealth = ref(null)
  let latestListRequest = 0
  let latestDetailRequest = 0
  let tokenRevision = 0
  const tokenChanges = new Map()
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

  const applyRecentTokenChange = (node, requestRevision) => {
    const change = tokenChanges.get(String(node?.id))
    return change && change.revision > requestRevision
      ? { ...node, has_token: change.hasToken }
      : node
  }

  // API 方法
  const fetchNodes = async (params = {}) => {
    const request = ++latestListRequest
    const requestRevision = tokenRevision
    loading.value = true
    error.value = null
    
    try {
      const response = await nodesApi.list(params)
      if (request !== latestListRequest) return
      
      const items = Array.isArray(response) ? response : response?.nodes ?? response?.data
      if (Array.isArray(response?.regions)) regions.value = response.regions
      if (Array.isArray(items)) {
        nodes.value = items.map(node => applyRecentTokenChange(node, requestRevision))
        total.value = response.total ?? items.length
      }
      
      return { nodes: nodes.value, total: total.value }
    } catch (err) {
      if (request !== latestListRequest) return
      error.value = err.message || '获取节点列表失败'
      throw err
    } finally {
      if (request === latestListRequest) loading.value = false
    }
  }

  const fetchNode = async (id) => {
    const request = ++latestDetailRequest
    const requestRevision = tokenRevision
    loading.value = true
    error.value = null
    
    try {
      const response = await nodesApi.get(id)
      const node = applyRecentTokenChange(response, requestRevision)
      if (request === latestDetailRequest) currentNode.value = node
      return node
    } catch (err) {
      if (request === latestDetailRequest) error.value = err.message || '获取节点详情失败'
      throw err
    } finally {
      if (request === latestDetailRequest) loading.value = false
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
    const requestRevision = tokenRevision
    const matchesNode = node => String(node?.id) === String(id)

    try {
      const response = await nodesApi.update(id, data)
      const updatedNode = applyRecentTokenChange(response, requestRevision)
      const index = nodes.value.findIndex(matchesNode)
      if (index >= 0) nodes.value[index] = updatedNode
      if (matchesNode(currentNode.value)) currentNode.value = updatedNode
      return updatedNode
    } catch (err) {
      error.value = err.message || '更新节点失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const deleteNode = async (id) => {
    loading.value = true
    error.value = null

    try {
      await nodesApi.delete(id)
      const matchesNode = node => String(node?.id) === String(id)
      const index = nodes.value.findIndex(matchesNode)
      if (index >= 0) {
        nodes.value.splice(index, 1)
        total.value--
      }
      if (matchesNode(currentNode.value)) currentNode.value = null
      return true
    } catch (err) {
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
  const setTokenStatus = (id, hasToken) => {
    const nodeId = String(id)
    tokenChanges.set(nodeId, { revision: ++tokenRevision, hasToken })
    const listedNode = nodes.value.find(node => String(node.id) === nodeId)
    if (listedNode) listedNode.has_token = hasToken
    if (String(currentNode.value?.id) === nodeId) currentNode.value.has_token = hasToken
  }

  const generateToken = async (id) => {
    const response = await nodesApi.generateToken(id)
    setTokenStatus(id, true)
    return response
  }

  const rotateToken = async (id) => {
    const response = await nodesApi.rotateToken(id)
    setTokenStatus(id, true)
    return response
  }

  const revokeToken = async (id) => {
    await nodesApi.revokeToken(id)
    setTokenStatus(id, false)
    return true
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

  const listAvailableCoreVersions = async (refresh = false) => {
    try {
      return await nodesApi.listAvailableCoreVersions(refresh)
    } catch (err) {
      error.value = err.message || '获取可用版本失败'
      throw err
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

  const getTrafficDiagnostic = async (id, refresh = false) => {
    try {
      return await nodesApi.getTrafficDiagnostic(id, refresh)
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
    regions,
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
    listAvailableCoreVersions,
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
