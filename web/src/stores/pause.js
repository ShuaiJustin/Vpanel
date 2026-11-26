import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { 
  getPauseStatus, 
  pauseSubscription, 
  resumeSubscription, 
  getPauseHistory 
} from '@/api/modules/pause'
import { toNormalizedError } from '@/utils/entitlement'

export const usePauseStore = defineStore('pause', () => {
  // State
  const status = ref(null)
  const history = ref([])
  const historyTotal = ref(0)
  const loading = ref(false)
  const error = ref(null)

  const normalizeCannotPauseReason = (reason) => {
    const raw = String(reason || '').trim()
    if (!raw) return ''

    const map = {
      'No active subscription': '当前无有效订阅',
      'Pause feature is disabled': '暂停功能未开启',
      'Subscription is already paused': '订阅已处于暂停状态',
      'Failed to verify user': '用户校验失败',
      'Failed to check pause status': '暂停状态检查失败',
      'Failed to check pause frequency': '暂停次数检查失败'
    }

    if (map[raw]) return map[raw]
    const limitMatch = raw.match(/^Maximum\\s+(\\d+)\\s+pause\\(s\\) per billing cycle reached$/i)
    if (limitMatch) return `当前计费周期最多可暂停 ${limitMatch[1]} 次，已达到上限`
    return raw
  }

  // Computed
  const isPaused = computed(() => status.value?.is_paused || false)
  const canPause = computed(() => status.value?.can_pause || false)
  const cannotPauseReason = computed(() => normalizeCannotPauseReason(status.value?.cannot_pause_reason))
  const remainingPauses = computed(() => status.value?.remaining_pauses || 0)
  const maxDuration = computed(() => status.value?.max_duration_days || 30)
  const activePause = computed(() => status.value?.pause || null)
  const autoResumeAt = computed(() => {
    if (activePause.value?.auto_resume_at) {
      return new Date(activePause.value.auto_resume_at)
    }
    return null
  })

  // Actions
  async function fetchPauseStatus() {
    loading.value = true
    error.value = null
    try {
      const response = await getPauseStatus()
      status.value = response.data || response
      return status.value
    } catch (err) {
      const normalizedError = toNormalizedError(err, '加载暂停状态失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function pause() {
    loading.value = true
    error.value = null
    try {
      const response = await pauseSubscription()
      // Refresh status after pausing
      await fetchPauseStatus()
      return response.data || response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '暂停失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function resume() {
    loading.value = true
    error.value = null
    try {
      const response = await resumeSubscription()
      // Refresh status after resuming
      await fetchPauseStatus()
      return response.data || response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '恢复失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function fetchHistory(page = 1, pageSize = 10) {
    loading.value = true
    error.value = null
    try {
      const response = await getPauseHistory({ page, page_size: pageSize })
      const data = response.data || response
      history.value = data.pauses || []
      historyTotal.value = data.total || 0
      return data
    } catch (err) {
      const normalizedError = toNormalizedError(err, '获取暂停记录失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  function reset() {
    status.value = null
    history.value = []
    historyTotal.value = 0
    loading.value = false
    error.value = null
  }

  return {
    // State
    status,
    history,
    historyTotal,
    loading,
    error,
    // Computed
    isPaused,
    canPause,
    cannotPauseReason,
    remainingPauses,
    maxDuration,
    activePause,
    autoResumeAt,
    // Actions
    fetchPauseStatus,
    pause,
    resume,
    fetchHistory,
    reset
  }
})
