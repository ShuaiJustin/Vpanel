/**
 * 节点分组状态管理
 * 管理节点分组列表、当前分组、成员管理等
 * Requirements: 6.1-6.6
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { nodeGroupsApi } from '@/api'

export const useNodeGroupStore = defineStore('nodeGroup', () => {
  // 状态
  const groups = ref([])
  const currentGroup = ref(null)
  const currentGroupNodes = ref([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref(null)
  const allStats = ref(null)
  const filters = ref({
    region: '',
    search: ''
  })

  // 计算属性
  const groupCount = computed(() => groups.value.length)
  
  const filteredGroups = computed(() => {
    let result = groups.value
    
    if (filters.value.region) {
      result = result.filter(g => g.region === filters.value.region)
    }
    
    if (filters.value.search) {
      const search = filters.value.search.toLowerCase()
      result = result.filter(g => 
        g.name.toLowerCase().includes(search) ||
        g.description?.toLowerCase().includes(search) ||
        g.region?.toLowerCase().includes(search)
      )
    }
    
    return result
  })

  const totalNodesInGroups = computed(() => 
    groups.value.reduce((sum, g) => sum + (g.total_nodes || g.node_count || 0), 0)
  )

  const totalHealthyNodes = computed(() => 
    groups.value.reduce((sum, g) => sum + (g.healthy_nodes || 0), 0)
  )

  const totalUsersInGroups = computed(() => 
    groups.value.reduce((sum, g) => sum + (g.total_users || g.user_count || 0), 0)
  )

  const regions = computed(() => {
    const regionSet = new Set(groups.value.map(g => g.region).filter(Boolean))
    return Array.from(regionSet)
  })

  const strategies = computed(() => {
    const strategySet = new Set(groups.value.map(g => g.strategy).filter(Boolean))
    return Array.from(strategySet)
  })

  // 方法
  const setFilters = (newFilters) => {
    filters.value = { ...filters.value, ...newFilters }
  }

  const clearFilters = () => {
    filters.value = { region: '', search: '' }
  }

  // API 方法
  const fetchGroups = async (params = {}) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodeGroupsApi.list(params)
      
      if (Array.isArray(response)) {
        groups.value = response
        total.value = response.length
      } else if (response?.groups) {
        groups.value = response.groups
        total.value = response.total || response.groups.length
      } else if (response?.data) {
        groups.value = response.data
        total.value = response.total || response.data.length
      }
      
      return { groups: groups.value, total: total.value }
    } catch (err) {
      error.value = err.message || '获取分组列表失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchGroupsWithStats = async (params = {}) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodeGroupsApi.listWithStats(params)
      
      if (Array.isArray(response)) {
        groups.value = response
        total.value = response.length
      } else if (response?.groups) {
        groups.value = response.groups
        total.value = response.total || response.groups.length
      } else if (response?.data) {
        groups.value = response.data
        total.value = response.total || response.data.length
      }
      
      return { groups: groups.value, total: total.value }
    } catch (err) {
      error.value = err.message || '获取分组列表失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchGroup = async (id) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodeGroupsApi.get(id)
      currentGroup.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取分组详情失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const fetchGroupStats = async (id) => {
    try {
      const response = await nodeGroupsApi.getStats(id)
      
      // 更新当前分组的统计信息
      if (currentGroup.value?.id === id) {
        currentGroup.value = { ...currentGroup.value, ...response }
      }
      
      // 更新列表中的分组统计
      const index = groups.value.findIndex(g => g.id === id)
      if (index >= 0) {
        groups.value[index] = { ...groups.value[index], ...response }
      }
      
      return response
    } catch (err) {
      error.value = err.message || '获取分组统计失败'
      throw err
    }
  }

  const createGroup = async (data) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodeGroupsApi.create(data)
      
      // 添加到列表
      groups.value.push(response)
      total.value++
      
      return response
    } catch (err) {
      error.value = err.message || '创建分组失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const updateGroup = async (id, data) => {
    loading.value = true
    error.value = null
    
    const index = groups.value.findIndex(g => g.id === id)
    const originalGroup = index >= 0 ? { ...groups.value[index] } : null
    
    try {
      // 乐观更新
      if (index >= 0) {
        groups.value[index] = { ...groups.value[index], ...data }
      }
      
      const response = await nodeGroupsApi.update(id, data)
      
      // 更新为服务器返回的数据
      if (index >= 0) {
        groups.value[index] = response
      }
      
      if (currentGroup.value?.id === id) {
        currentGroup.value = response
      }
      
      return response
    } catch (err) {
      // 回滚
      if (index >= 0 && originalGroup) {
        groups.value[index] = originalGroup
      }
      
      error.value = err.message || '更新分组失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const deleteGroup = async (id) => {
    loading.value = true
    error.value = null
    
    const index = groups.value.findIndex(g => g.id === id)
    const originalGroup = index >= 0 ? groups.value[index] : null
    
    try {
      // 乐观更新
      if (index >= 0) {
        groups.value.splice(index, 1)
        total.value--
      }
      
      await nodeGroupsApi.delete(id)
      
      if (currentGroup.value?.id === id) {
        currentGroup.value = null
        currentGroupNodes.value = []
      }
      
      return true
    } catch (err) {
      // 回滚
      if (originalGroup) {
        groups.value.splice(index, 0, originalGroup)
        total.value++
      }
      
      error.value = err.message || '删除分组失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  // 成员管理
  const fetchGroupNodes = async (groupId) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await nodeGroupsApi.getNodes(groupId)
      
      if (Array.isArray(response)) {
        currentGroupNodes.value = response
      } else if (response?.nodes) {
        currentGroupNodes.value = response.nodes
      } else if (response?.data) {
        currentGroupNodes.value = response.data
      }
      
      return currentGroupNodes.value
    } catch (err) {
      error.value = err.message || '获取分组节点失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const addNodeToGroup = async (groupId, nodeId) => {
    loading.value = true
    error.value = null
    
    try {
      await nodeGroupsApi.addNode(groupId, nodeId)
      
      // 刷新分组节点列表
      if (currentGroup.value?.id === groupId) {
        await fetchGroupNodes(groupId)
      }
      
      // 更新分组统计
      const index = groups.value.findIndex(g => g.id === groupId)
      if (index >= 0 && groups.value[index].total_nodes !== undefined) {
        groups.value[index].total_nodes++
      }
      
      return true
    } catch (err) {
      error.value = err.message || '添加节点到分组失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const removeNodeFromGroup = async (groupId, nodeId) => {
    loading.value = true
    error.value = null
    
    try {
      await nodeGroupsApi.removeNode(groupId, nodeId)
      
      // 从本地列表移除
      const nodeIndex = currentGroupNodes.value.findIndex(n => n.id === nodeId)
      if (nodeIndex >= 0) {
        currentGroupNodes.value.splice(nodeIndex, 1)
      }
      
      // 更新分组统计
      const index = groups.value.findIndex(g => g.id === groupId)
      if (index >= 0 && groups.value[index].total_nodes !== undefined) {
        groups.value[index].total_nodes--
      }
      
      return true
    } catch (err) {
      error.value = err.message || '从分组移除节点失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const setGroupNodes = async (groupId, nodeIds) => {
    loading.value = true
    error.value = null
    
    try {
      await nodeGroupsApi.setNodes(groupId, nodeIds)
      
      // 刷新分组节点列表
      if (currentGroup.value?.id === groupId) {
        await fetchGroupNodes(groupId)
      }
      
      // 更新分组统计
      const index = groups.value.findIndex(g => g.id === groupId)
      if (index >= 0) {
        groups.value[index].total_nodes = nodeIds.length
      }
      
      return true
    } catch (err) {
      error.value = err.message || '设置分组节点失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  // 统计
  const fetchAllStats = async () => {
    try {
      const response = await nodeGroupsApi.getAllStats()
      allStats.value = response
      return response
    } catch (err) {
      error.value = err.message || '获取所有分组统计失败'
      throw err
    }
  }

  // 清除状态
  const clearCurrentGroup = () => {
    currentGroup.value = null
    currentGroupNodes.value = []
  }

  const clearError = () => {
    error.value = null
  }

  return {
    // 状态
    groups,
    currentGroup,
    currentGroupNodes,
    total,
    loading,
    error,
    allStats,
    filters,
    
    // 计算属性
    groupCount,
    filteredGroups,
    totalNodesInGroups,
    totalHealthyNodes,
    totalUsersInGroups,
    regions,
    strategies,
    
    // 方法
    setFilters,
    clearFilters,
    fetchGroups,
    fetchGroupsWithStats,
    fetchGroup,
    fetchGroupStats,
    createGroup,
    updateGroup,
    deleteGroup,
    fetchGroupNodes,
    addNodeToGroup,
    removeNodeFromGroup,
    setGroupNodes,
    fetchAllStats,
    clearCurrentGroup,
    clearError
  }
})
