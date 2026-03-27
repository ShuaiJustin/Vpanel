import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { subscriptionApi } from '@/api/index'

export const useSubscriptionStore = defineStore('subscription', () => {
  // 状态
  const subscriptionInfo = ref(null)
  const loading = ref(false)
  const error = ref(null)

  // 计算属性
  const link = computed(() => subscriptionInfo.value?.link || '')
  const shortLink = computed(() => subscriptionInfo.value?.short_link || '')
  const token = computed(() => subscriptionInfo.value?.token || '')
  const shortCode = computed(() => subscriptionInfo.value?.short_code || '')
  const formats = computed(() => subscriptionInfo.value?.formats || [])
  const accessCount = computed(() => subscriptionInfo.value?.access_count || 0)
  const lastAccessAt = computed(() => subscriptionInfo.value?.last_access_at || null)
  const hasSubscription = computed(() => !!subscriptionInfo.value?.token)

  // 方法
  const fetchLink = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await subscriptionApi.getLink()
      subscriptionInfo.value = response
      return response
    } catch (err) {
      console.error('Fetch subscription link error:', err)
      subscriptionInfo.value = null
      error.value = err.response?.data?.error || err.message || '获取订阅链接失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const fetchInfo = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await subscriptionApi.getInfo()
      subscriptionInfo.value = response
      return response
    } catch (err) {
      console.error('Fetch subscription info error:', err)
      subscriptionInfo.value = null
      error.value = err.response?.data?.error || err.message || '获取订阅信息失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const regenerate = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await subscriptionApi.regenerate()
      subscriptionInfo.value = response
      return response
    } catch (err) {
      console.error('Regenerate subscription error:', err)
      subscriptionInfo.value = null
      error.value = err.response?.data?.error || err.message || '重新生成订阅链接失败'
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const clearSubscription = () => {
    subscriptionInfo.value = null
    error.value = null
  }

  return {
    // 状态
    subscriptionInfo,
    loading,
    error,

    // 计算属性
    link,
    shortLink,
    token,
    shortCode,
    formats,
    accessCount,
    lastAccessAt,
    hasSubscription,

    // 方法
    fetchLink,
    fetchInfo,
    regenerate,
    clearSubscription
  }
})
