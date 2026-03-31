/**
 * 用户前台工单 Store
 * 管理工单列表和详情
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { tickets as ticketsApi } from '@/api/modules/portal'
import { toNormalizedError } from '@/utils/entitlement'

export const usePortalTicketsStore = defineStore('portalTickets', () => {
  // 状态
  const tickets = ref([])
  const currentTicket = ref(null)
  const total = ref(0)
  const loading = ref(false)
  const error = ref(null)

  // 分页状态
  const pagination = ref({
    limit: 10,
    offset: 0
  })

  // 筛选状态
  const statusFilter = ref('')

  // 计算属性
  const openTickets = computed(() => 
    tickets.value.filter(t => t.status === 'open' || t.status === 'waiting')
  )

  const closedTickets = computed(() => 
    tickets.value.filter(t => t.status === 'closed')
  )

  const hasMore = computed(() => 
    pagination.value.offset + tickets.value.length < total.value
  )

  // 方法
  async function fetchTickets(params = {}) {
    loading.value = true
    error.value = null
    try {
      const response = await ticketsApi.getTickets({
        ...pagination.value,
        status: statusFilter.value || undefined,
        ...params
      })
      tickets.value = response.tickets || []
      total.value = response.total || 0
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '获取工单列表失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function fetchTicket(id) {
    loading.value = true
    error.value = null
    try {
      const response = await ticketsApi.getTicket(id)
      currentTicket.value = response
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '获取工单详情失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function createTicket(data) {
    loading.value = true
    error.value = null
    try {
      const response = await ticketsApi.createTicket(data)
      // 添加到列表开头
      tickets.value.unshift(response)
      total.value++
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '创建工单失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function replyTicket(id, content) {
    loading.value = true
    error.value = null
    try {
      const response = await ticketsApi.replyTicket(id, { content })
      // 更新当前工单的消息列表
      if (currentTicket.value && currentTicket.value.id === id) {
        if (!currentTicket.value.messages) {
          currentTicket.value.messages = []
        }
        currentTicket.value.messages.push(response)
        currentTicket.value.status = 'waiting'
        currentTicket.value.updated_at = new Date().toISOString()
      }
      // 更新列表中的工单状态
      const ticketIndex = tickets.value.findIndex(t => t.id === id)
      if (ticketIndex !== -1) {
        tickets.value[ticketIndex].status = 'waiting'
        tickets.value[ticketIndex].updated_at = new Date().toISOString()
      }
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '回复工单失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function closeTicket(id) {
    loading.value = true
    error.value = null
    try {
      await ticketsApi.closeTicket(id)
      // 更新当前工单状态
      if (currentTicket.value && currentTicket.value.id === id) {
        currentTicket.value.status = 'closed'
        currentTicket.value.closed_at = new Date().toISOString()
      }
      // 更新列表中的工单状态
      const ticketIndex = tickets.value.findIndex(t => t.id === id)
      if (ticketIndex !== -1) {
        tickets.value[ticketIndex].status = 'closed'
      }
    } catch (err) {
      const normalizedError = toNormalizedError(err, '关闭工单失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function reopenTicket(id) {
    loading.value = true
    error.value = null
    try {
      await ticketsApi.reopenTicket(id)
      // 更新当前工单状态
      if (currentTicket.value && currentTicket.value.id === id) {
        currentTicket.value.status = 'open'
        currentTicket.value.closed_at = null
      }
      // 更新列表中的工单状态
      const ticketIndex = tickets.value.findIndex(t => t.id === id)
      if (ticketIndex !== -1) {
        tickets.value[ticketIndex].status = 'open'
      }
    } catch (err) {
      const normalizedError = toNormalizedError(err, '重新打开工单失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  function setStatusFilter(status) {
    statusFilter.value = status
    pagination.value.offset = 0
  }

  function nextPage() {
    pagination.value.offset += pagination.value.limit
  }

  function prevPage() {
    pagination.value.offset = Math.max(0, pagination.value.offset - pagination.value.limit)
  }

  function resetPagination() {
    pagination.value.offset = 0
  }

  function clearCurrentTicket() {
    currentTicket.value = null
  }

  function getStatusLabel(status) {
    const labels = {
      open: '待处理',
      waiting: '等待回复',
      answered: '已回复',
      closed: '已关闭'
    }
    return labels[status] || status
  }

  function getStatusType(status) {
    const types = {
      open: 'warning',
      waiting: 'info',
      answered: 'success',
      closed: ''
    }
    return types[status] || ''
  }

  return {
    // 状态
    tickets,
    currentTicket,
    total,
    loading,
    error,
    pagination,
    statusFilter,
    // 计算属性
    openTickets,
    closedTickets,
    hasMore,
    // 方法
    fetchTickets,
    fetchTicket,
    createTicket,
    replyTicket,
    closeTicket,
    reopenTicket,
    setStatusFilter,
    nextPage,
    prevPage,
    resetPagination,
    clearCurrentTicket,
    getStatusLabel,
    getStatusType
  }
})
