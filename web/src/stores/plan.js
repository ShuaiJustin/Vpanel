import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { plansApi } from '@/api/index'
import { toNormalizedError } from '@/utils/entitlement'

export const usePlanStore = defineStore('plan', () => {
  // 状态
  const plans = ref([])
  const currentPlan = ref(null)
  const loading = ref(false)
  const error = ref(null)

  // 计算属性
  const activePlans = computed(() => plans.value.filter(p => p.is_active))
  const sortedPlans = computed(() => [...activePlans.value].sort((a, b) => a.sort_order - b.sort_order))
  const hasPlan = computed(() => plans.value.length > 0)

  // 格式化价格（分转元）
  const formatPrice = (price) => (price / 100).toFixed(2)

  // 计算月均价格
  const getMonthlyPrice = (plan) => {
    if (!plan || plan.duration <= 0) return 0
    return Math.round(plan.price / (plan.duration / 30))
  }

  // 方法
  const fetchPlans = async () => {
    loading.value = true
    error.value = null

    try {
      const response = await plansApi.list()
      plans.value = response.plans || []
      return response
    } catch (err) {
      console.error('Fetch plans error:', err)
      const normalizedError = toNormalizedError(err, '获取套餐列表失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const fetchPlan = async (id) => {
    loading.value = true
    error.value = null

    try {
      const response = await plansApi.get(id)
      currentPlan.value = response.plan
      return response
    } catch (err) {
      console.error('Fetch plan error:', err)
      const normalizedError = toNormalizedError(err, '获取套餐详情失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const comparePlans = async (ids) => {
    loading.value = true
    error.value = null

    try {
      const response = await plansApi.compare(ids)
      return response
    } catch (err) {
      console.error('Compare plans error:', err)
      const normalizedError = toNormalizedError(err, '比较套餐失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  const clearPlans = () => {
    plans.value = []
    currentPlan.value = null
    error.value = null
  }

  return {
    // 状态
    plans,
    currentPlan,
    loading,
    error,

    // 计算属性
    activePlans,
    sortedPlans,
    hasPlan,

    // 工具方法
    formatPrice,
    getMonthlyPrice,

    // 方法
    fetchPlans,
    fetchPlan,
    comparePlans,
    clearPlans
  }
})
