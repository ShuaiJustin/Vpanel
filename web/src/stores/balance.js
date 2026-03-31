import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { balanceApi } from '@/api/index'
import { toNormalizedError } from '@/utils/entitlement'

export const useBalanceStore = defineStore('balance', () => {
  // 状态
  const balance = ref(0)
  const transactions = ref([])
  const loading = ref(false)
  const error = ref(null)
  const pagination = ref({ page: 1, pageSize: 10, total: 0 })

  // 计算属性
  const formattedBalance = computed(() => (balance.value / 100).toFixed(2))
  const hasBalance = computed(() => balance.value > 0)

  // 交易类型映射
  const typeMap = {
    recharge: { label: '充值', type: 'success', icon: 'Plus' },
    purchase: { label: '消费', type: 'danger', icon: 'Minus' },
    refund: { label: '退款', type: 'warning', icon: 'RefreshRight' },
    commission: { label: '佣金', type: 'success', icon: 'Coin' },
    adjustment: { label: '调整', type: 'info', icon: 'Edit' }
  }

  const getTypeInfo = (type) => typeMap[type] || { label: type, type: 'info', icon: 'Document' }

  // 格式化价格（分转元）
  const formatPrice = (price) => {
    const yuan = (Math.abs(price) / 100).toFixed(2)
    return price >= 0 ? `+${yuan}` : `-${yuan}`
  }

  // 方法
  const fetchBalance = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await balanceApi.get()
      balance.value = response.balance || 0
      return response
    } catch (err) {
      console.error('Fetch balance error:', err)
      const normalizedError = toNormalizedError(err, '获取余额失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const fetchTransactions = async (params = {}) => {
    loading.value = true
    error.value = null

    try {
      const response = await balanceApi.getTransactions({
        page: pagination.value.page,
        page_size: pagination.value.pageSize,
        ...params
      })
      transactions.value = response.transactions || []
      pagination.value.total = response.total || 0
      return response
    } catch (err) {
      console.error('Fetch transactions error:', err)
      const normalizedError = toNormalizedError(err, '获取交易记录失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const recharge = async (amount, method) => {
    loading.value = true
    error.value = null

    try {
      const response = await balanceApi.recharge({ amount, method })
      return response
    } catch (err) {
      console.error('Recharge error:', err)
      const normalizedError = toNormalizedError(err, '充值失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const setPage = (page) => {
    pagination.value.page = page
  }

  const setPageSize = (size) => {
    pagination.value.pageSize = size
    pagination.value.page = 1
  }

  const clearBalance = () => {
    balance.value = 0
    transactions.value = []
    error.value = null
    pagination.value = { page: 1, pageSize: 10, total: 0 }
  }

  return {
    // 状态
    balance,
    transactions,
    loading,
    error,
    pagination,

    // 计算属性
    formattedBalance,
    hasBalance,

    // 工具方法
    getTypeInfo,
    formatPrice,

    // 方法
    fetchBalance,
    fetchTransactions,
    recharge,
    setPage,
    setPageSize,
    clearBalance
  }
})
