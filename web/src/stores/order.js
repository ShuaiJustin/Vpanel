import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { ordersApi, paymentsApi } from '@/api/index'

export const useOrderStore = defineStore('order', () => {
  // 状态
  const orders = ref([])
  const currentOrder = ref(null)
  const paymentInfo = ref(null)
  const loading = ref(false)
  const paymentLoading = ref(false)
  const error = ref(null)
  const pagination = ref({ page: 1, pageSize: 10, total: 0 })

  // 计算属性
  const pendingOrders = computed(() => orders.value.filter(o => o.status === 'pending'))
  const paidOrders = computed(() => orders.value.filter(o => o.status === 'paid'))
  const hasOrders = computed(() => orders.value.length > 0)
  const isPaying = computed(() => paymentLoading.value)

  // 订单状态映射
  const statusMap = {
    pending: { label: '待支付', type: 'warning' },
    paid: { label: '已支付', type: 'success' },
    completed: { label: '已完成', type: 'success' },
    cancelled: { label: '已取消', type: 'info' },
    refunded: { label: '已退款', type: 'danger' }
  }

  const getStatusInfo = (status) => statusMap[status] || { label: status, type: 'info' }

  // 格式化价格（分转元）
  const formatPrice = (price) => (price / 100).toFixed(2)

  // 方法
  const fetchOrders = async (params = {}) => {
    loading.value = true
    error.value = null

    try {
      const response = await ordersApi.list({
        page: pagination.value.page,
        page_size: pagination.value.pageSize,
        ...params
      })
      orders.value = response.orders || []
      pagination.value.total = response.total || 0
      return response
    } catch (err) {
      console.error('Fetch orders error:', err)
      error.value = err.message || '获取订单列表失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const fetchOrder = async (id) => {
    loading.value = true
    error.value = null

    try {
      const response = await ordersApi.get(id)
      currentOrder.value = response.order
      return response
    } catch (err) {
      console.error('Fetch order error:', err)
      error.value = err.message || '获取订单详情失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const fetchOrderByOrderNo = async (orderNo) => {
    loading.value = true
    error.value = null

    try {
      const response = await ordersApi.getByOrderNo(orderNo)
      currentOrder.value = response.order
      return response
    } catch (err) {
      console.error('Fetch order by number error:', err)
      error.value = err.message || '获取订单详情失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const createOrder = async (data) => {
    loading.value = true
    error.value = null

    try {
      const response = await ordersApi.create(data)
      currentOrder.value = response.order
      return response
    } catch (err) {
      console.error('Create order error:', err)
      error.value = err.message || '创建订单失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const cancelOrder = async (id) => {
    loading.value = true
    error.value = null

    try {
      const response = await ordersApi.cancel(id)
      // 更新本地订单状态
      const index = orders.value.findIndex(o => o.id === id)
      if (index !== -1) {
        orders.value[index].status = 'cancelled'
      }
      if (currentOrder.value?.id === id) {
        currentOrder.value.status = 'cancelled'
      }
      return response
    } catch (err) {
      console.error('Cancel order error:', err)
      error.value = err.message || '取消订单失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const createPayment = async (orderNo, method) => {
    paymentLoading.value = true
    error.value = null

    try {
      const response = await paymentsApi.create({ order_no: orderNo, method })
      paymentInfo.value = response.payment
      return response
    } catch (err) {
      console.error('Create payment error:', err)
      error.value = err.message || '创建支付失败'
      throw error.value
    } finally {
      paymentLoading.value = false
    }
  }

  const checkPaymentStatus = async (orderNo) => {
    try {
      const response = await paymentsApi.getStatus(orderNo)
      return response
    } catch (err) {
      console.error('Check payment status error:', err)
      throw err
    }
  }

  const pollPaymentStatus = async (orderNo, timeout = 300000, interval = 3000) => {
    paymentLoading.value = true
    error.value = null
    try {
      const result = await paymentsApi.pollStatus(orderNo, timeout, interval)
      if (currentOrder.value?.order_no === orderNo && result.status) {
        currentOrder.value.status = result.status
      }
      return result
    } catch (err) {
      console.error('Poll payment status error:', err)
      throw err
    } finally {
      paymentLoading.value = false
    }
  }

  const setPage = (page) => {
    pagination.value.page = page
  }

  const setPageSize = (size) => {
    pagination.value.pageSize = size
    pagination.value.page = 1
  }

  const clearOrder = () => {
    currentOrder.value = null
    paymentInfo.value = null
    error.value = null
  }

  const clearOrders = () => {
    orders.value = []
    currentOrder.value = null
    paymentInfo.value = null
    error.value = null
    pagination.value = { page: 1, pageSize: 10, total: 0 }
  }

  return {
    // 状态
    orders,
    currentOrder,
    paymentInfo,
    loading,
    paymentLoading,
    error,
    pagination,

    // 计算属性
    pendingOrders,
    paidOrders,
    hasOrders,
    isPaying,

    // 工具方法
    getStatusInfo,
    formatPrice,

    // 方法
    fetchOrders,
    fetchOrder,
    fetchOrderByOrderNo,
    createOrder,
    cancelOrder,
    createPayment,
    checkPaymentStatus,
    pollPaymentStatus,
    setPage,
    setPageSize,
    clearOrder,
    clearOrders
  }
})
