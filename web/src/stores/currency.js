/**
 * 货币状态管理
 * 管理用户货币偏好和汇率信息
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { currencyApi } from '@/api/modules/currency'

export const useCurrencyStore = defineStore('currency', () => {
  // 状态
  const currencies = ref([])
  const baseCurrency = ref('CNY')
  const selectedCurrency = ref(localStorage.getItem('selectedCurrency') || '')
  const detectedCurrency = ref('')
  const exchangeRates = ref({})
  const loading = ref(false)
  const error = ref(null)

  // 计算属性
  const currentCurrency = computed(() => {
    return selectedCurrency.value || detectedCurrency.value || baseCurrency.value
  })

  const currencyInfo = computed(() => {
    return currencies.value.find(c => c.code === currentCurrency.value) || null
  })

  const currencySymbol = computed(() => {
    return currencyInfo.value?.symbol || '¥'
  })

  // 获取支持的货币列表
  async function fetchCurrencies() {
    loading.value = true
    error.value = null
    try {
      const response = await currencyApi.getSupportedCurrencies()
      currencies.value = response.currencies || []
      baseCurrency.value = response.base_currency || 'CNY'
    } catch (err) {
      error.value = err.message || '获取货币列表失败'
      console.error('Failed to fetch currencies:', err)
    } finally {
      loading.value = false
    }
  }

  // 检测用户货币
  async function detectUserCurrency() {
    try {
      const response = await currencyApi.detectCurrency()
      detectedCurrency.value = response.currency || baseCurrency.value
      // 如果用户没有手动选择货币，使用检测到的货币
      if (!selectedCurrency.value) {
        selectedCurrency.value = detectedCurrency.value
      }
    } catch (err) {
      console.error('Failed to detect currency:', err)
      detectedCurrency.value = baseCurrency.value
    }
  }

  // 设置用户选择的货币
  function setCurrency(currency) {
    selectedCurrency.value = currency
    localStorage.setItem('selectedCurrency', currency)
  }

  // 获取汇率
  async function getExchangeRate(from, to) {
    const cacheKey = `${from}_${to}`
    if (exchangeRates.value[cacheKey]) {
      return exchangeRates.value[cacheKey]
    }

    try {
      const response = await currencyApi.getExchangeRate(from, to)
      exchangeRates.value[cacheKey] = response.rate
      return response.rate
    } catch (err) {
      console.error('Failed to get exchange rate:', err)
      return 1
    }
  }

  // 转换金额
  async function convertAmount(amount, from, to) {
    if (from === to) return amount

    try {
      const response = await currencyApi.convertAmount({ amount, from, to })
      return response.converted_amount
    } catch (err) {
      console.error('Failed to convert amount:', err)
      // 尝试使用缓存的汇率
      const rate = await getExchangeRate(from, to)
      return Math.round(amount * rate)
    }
  }

  // 格式化价格
  function formatPrice(amount, currency = null) {
    const curr = currency || currentCurrency.value
    const info = currencies.value.find(c => c.code === curr)
    
    if (!info) {
      return `¥${(amount / 100).toFixed(2)}`
    }

    const value = amount / Math.pow(10, info.decimals || 2)
    const formatted = value.toFixed(info.decimals || 2)
    return `${info.symbol}${formatted}`
  }

  // 初始化
  async function initialize() {
    await fetchCurrencies()
    await detectUserCurrency()
  }

  return {
    // 状态
    currencies,
    baseCurrency,
    selectedCurrency,
    detectedCurrency,
    exchangeRates,
    loading,
    error,
    // 计算属性
    currentCurrency,
    currencyInfo,
    currencySymbol,
    // 方法
    fetchCurrencies,
    detectUserCurrency,
    setCurrency,
    getExchangeRate,
    convertAmount,
    formatPrice,
    initialize
  }
})

export default useCurrencyStore
