import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { invitesApi } from '@/api/index'
import { toNormalizedError } from '@/utils/entitlement'

export const useInviteStore = defineStore('invite', () => {
  // 状态
  const inviteCode = ref(null)
  const referrals = ref([])
  const commissions = ref([])
  const stats = ref(null)
  const earnings = ref(null)
  const loading = ref(false)
  const error = ref(null)
  const referralPagination = ref({ page: 1, pageSize: 10, total: 0 })
  const commissionPagination = ref({ page: 1, pageSize: 10, total: 0 })

  // 计算属性
  const code = computed(() => inviteCode.value?.code || '')
  const inviteLink = computed(() => inviteCode.value?.invite_link || '')
  const inviteCount = computed(() => inviteCode.value?.invite_count || 0)
  const totalInvites = computed(() => stats.value?.total_invites || 0)
  const convertedInvites = computed(() => stats.value?.converted_invites || 0)
  const conversionRate = computed(() => stats.value?.conversion_rate || 0)
  const totalEarnings = computed(() => earnings.value?.total_earnings || 0)
  const pendingEarnings = computed(() => earnings.value?.pending_earnings || 0)
  const formattedTotalEarnings = computed(() => (totalEarnings.value / 100).toFixed(2))
  const formattedPendingEarnings = computed(() => (pendingEarnings.value / 100).toFixed(2))

  // 推荐状态映射
  const referralStatusMap = {
    registered: { label: '已注册', type: 'info' },
    converted: { label: '已转化', type: 'success' }
  }

  // 佣金状态映射
  const commissionStatusMap = {
    pending: { label: '待确认', type: 'warning' },
    confirmed: { label: '已确认', type: 'success' },
    cancelled: { label: '已取消', type: 'danger' }
  }

  const getReferralStatusInfo = (status) => referralStatusMap[status] || { label: status, type: 'info' }
  const getCommissionStatusInfo = (status) => commissionStatusMap[status] || { label: status, type: 'info' }

  // 格式化价格（分转元）
  const formatPrice = (price) => (price / 100).toFixed(2)

  // 方法
  const fetchInviteCode = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await invitesApi.getCode()
      inviteCode.value = response.invite
      return response
    } catch (err) {
      console.error('Fetch invite code error:', err)
      const normalizedError = toNormalizedError(err, '获取邀请码失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const fetchReferrals = async (params = {}) => {
    loading.value = true
    error.value = null

    try {
      const response = await invitesApi.getReferrals({
        page: referralPagination.value.page,
        page_size: referralPagination.value.pageSize,
        ...params
      })
      referrals.value = response.referrals || []
      referralPagination.value.total = response.total || 0
      return response
    } catch (err) {
      console.error('Fetch referrals error:', err)
      const normalizedError = toNormalizedError(err, '获取推荐列表失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const fetchStats = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await invitesApi.getStats()
      stats.value = response.stats
      return response
    } catch (err) {
      console.error('Fetch stats error:', err)
      const normalizedError = toNormalizedError(err, '获取邀请统计失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const fetchCommissions = async (params = {}) => {
    loading.value = true
    error.value = null

    try {
      const response = await invitesApi.getCommissions({
        page: commissionPagination.value.page,
        page_size: commissionPagination.value.pageSize,
        ...params
      })
      commissions.value = response.commissions || []
      commissionPagination.value.total = response.total || 0
      return response
    } catch (err) {
      console.error('Fetch commissions error:', err)
      const normalizedError = toNormalizedError(err, '获取佣金列表失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const fetchEarnings = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await invitesApi.getEarnings()
      earnings.value = response
      return response
    } catch (err) {
      console.error('Fetch earnings error:', err)
      const normalizedError = toNormalizedError(err, '获取收益汇总失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const fetchAll = async () => {
    await Promise.all([
      fetchInviteCode(),
      fetchStats(),
      fetchEarnings()
    ])
  }

  const setReferralPage = (page) => {
    referralPagination.value.page = page
  }

  const setCommissionPage = (page) => {
    commissionPagination.value.page = page
  }

  const clearInvite = () => {
    inviteCode.value = null
    referrals.value = []
    commissions.value = []
    stats.value = null
    earnings.value = null
    error.value = null
    referralPagination.value = { page: 1, pageSize: 10, total: 0 }
    commissionPagination.value = { page: 1, pageSize: 10, total: 0 }
  }

  return {
    // 状态
    inviteCode,
    referrals,
    commissions,
    stats,
    earnings,
    loading,
    error,
    referralPagination,
    commissionPagination,

    // 计算属性
    code,
    inviteLink,
    inviteCount,
    totalInvites,
    convertedInvites,
    conversionRate,
    totalEarnings,
    pendingEarnings,
    formattedTotalEarnings,
    formattedPendingEarnings,

    // 工具方法
    getReferralStatusInfo,
    getCommissionStatusInfo,
    formatPrice,

    // 方法
    fetchInviteCode,
    fetchReferrals,
    fetchStats,
    fetchCommissions,
    fetchEarnings,
    fetchAll,
    setReferralPage,
    setCommissionPage,
    clearInvite
  }
})
