/**
 * 用户前台节点 Store
 * 管理节点列表和延迟测试
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { nodes as nodesApi } from '@/api/modules/portal'

export const usePortalNodesStore = defineStore('portalNodes', () => {
  // 状态
  const nodes = ref([])
  const regions = ref([])
  const protocols = ref([])
  const loading = ref(false)
  const error = ref(null)
  const latencyResults = ref({})
  const testingLatency = ref({})

  // 筛选和排序状态
  const filters = ref({
    region: '',
    protocol: '',
    status: ''
  })
  const sortBy = ref('name')
  const sortOrder = ref('asc')

  // 计算属性
  const filteredNodes = computed(() => {
    let result = [...nodes.value]

    // 应用筛选
    if (filters.value.region) {
      result = result.filter(n => n.region === filters.value.region)
    }
    if (filters.value.protocol) {
      result = result.filter(n => n.protocol === filters.value.protocol)
    }
    if (filters.value.status) {
      result = result.filter(n => n.status === filters.value.status)
    }

    // 应用排序
    result.sort((a, b) => {
      let aVal, bVal
      switch (sortBy.value) {
        case 'latency':
          aVal = latencyResults.value[a.id] ?? Infinity
          bVal = latencyResults.value[b.id] ?? Infinity
          break
        case 'region':
          aVal = a.region || ''
          bVal = b.region || ''
          break
        case 'name':
        default:
          aVal = a.name || ''
          bVal = b.name || ''
      }
      
      if (typeof aVal === 'string') {
        return sortOrder.value === 'asc' 
          ? aVal.localeCompare(bVal)
          : bVal.localeCompare(aVal)
      }
      return sortOrder.value === 'asc' ? aVal - bVal : bVal - aVal
    })

    return result
  })

  const onlineCount = computed(() => 
    nodes.value.filter(n => n.status === 'online').length
  )

  const totalCount = computed(() => nodes.value.length)

  // 方法
  async function fetchNodes(params = {}) {
    loading.value = true
    error.value = null
    try {
      const response = await nodesApi.getNodes(params)
      nodes.value = response.nodes || response
      regions.value = response.regions || regions.value
      protocols.value = response.protocols || protocols.value
      return response
    } catch (err) {
      error.value = err.message || '获取节点列表失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchRegions() {
    try {
      if (!nodes.value.length) {
        await fetchNodes()
      }
      const response = await nodesApi.getRegions()
      regions.value = response.regions || response
      return response
    } catch (err) {
      console.error('Failed to fetch regions:', err)
    }
  }

  async function fetchProtocols() {
    try {
      if (!nodes.value.length) {
        await fetchNodes()
      }
      const response = await nodesApi.getProtocols()
      protocols.value = response.protocols || response
      return response
    } catch (err) {
      console.error('Failed to fetch protocols:', err)
    }
  }

  async function testNodeLatency(nodeId) {
    testingLatency.value[nodeId] = true
    try {
      const response = await nodesApi.testLatency(nodeId)
      latencyResults.value[nodeId] = response.latency
      return response.latency
    } catch (err) {
      latencyResults.value[nodeId] = -1 // 表示测试失败
      throw err
    } finally {
      testingLatency.value[nodeId] = false
    }
  }

  async function testAllLatency() {
    const onlineNodes = nodes.value.filter(n => n.status === 'online')
    const promises = onlineNodes.map(node => 
      testNodeLatency(node.id).catch(() => {})
    )
    await Promise.all(promises)
  }

  function setFilter(key, value) {
    filters.value[key] = value
  }

  function setSort(field, order = 'asc') {
    sortBy.value = field
    sortOrder.value = order
  }

  function clearFilters() {
    filters.value = {
      region: '',
      protocol: '',
      status: ''
    }
  }

  function getLatencyClass(nodeId) {
    const latency = latencyResults.value[nodeId]
    if (latency === undefined || latency === null) return ''
    if (latency < 0) return 'latency-error'
    if (latency < 100) return 'latency-good'
    if (latency < 300) return 'latency-medium'
    return 'latency-bad'
  }

  return {
    // 状态
    nodes,
    regions,
    protocols,
    loading,
    error,
    latencyResults,
    testingLatency,
    filters,
    sortBy,
    sortOrder,
    // 计算属性
    filteredNodes,
    onlineCount,
    totalCount,
    // 方法
    fetchNodes,
    fetchRegions,
    fetchProtocols,
    testNodeLatency,
    testAllLatency,
    setFilter,
    setSort,
    clearFilters,
    getLatencyClass
  }
})
