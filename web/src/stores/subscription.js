import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { subscriptionApi } from '@/api/index'
import { extractErrorMessage, getErrorStatus } from '@/utils/entitlement'

function createStoreError(err, fallbackMessage) {
  const message = extractErrorMessage(err) || fallbackMessage

  if (typeof err === 'object' && err !== null) {
    return {
      ...err,
      message
    }
  }

  return { message }
}

export const useSubscriptionStore = defineStore('subscription', () => {
  // 状态
  const subscriptionInfo = ref(null)
  const loading = ref(false)
  const error = ref(null)
  let generation = 0
  let readRequest
  let resetRequest

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
  const request = async (operation, fallback, revision) => {
    loading.value = true
    error.value = null
    try {
      const response = await operation()
      if (revision !== generation) return undefined
      subscriptionInfo.value = response
      return response
    } catch (err) {
      if (revision !== generation) return undefined
      const normalizedError = createStoreError(err, fallback)
      if (getErrorStatus(err) === 403) subscriptionInfo.value = null
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      if (revision === generation) loading.value = false
    }
  }

  const read = (operation, fallback) => {
    if (resetRequest) return resetRequest
    if (readRequest?.operation === operation) return readRequest.promise
    const revision = ++generation
    const promise = request(operation, fallback, revision).finally(() => {
      if (readRequest?.promise === promise) readRequest = undefined
    })
    readRequest = { operation, promise }
    return promise
  }

  const fetchLink = () => read(subscriptionApi.getLink, '获取订阅链接失败')
  const fetchInfo = () => read(subscriptionApi.getInfo, '获取订阅信息失败')

  const regenerate = () => {
    if (resetRequest) return resetRequest
    readRequest = undefined
    const promise = request(subscriptionApi.regenerate, '重新生成订阅链接失败', ++generation).finally(() => {
      if (resetRequest === promise) resetRequest = undefined
    })
    resetRequest = promise
    return promise
  }

  const clearSubscription = () => {
    generation++
    readRequest = undefined
    resetRequest = undefined
    subscriptionInfo.value = null
    error.value = null
    loading.value = false
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
