/**
 * 货币相关 API
 * 处理多币种支持、汇率转换等操作
 */
import api from '../base'

export const currencyApi = {
  /**
   * 获取支持的货币列表
   * @returns {Promise<Object>} 货币列表和基础货币
   */
  getSupportedCurrencies: () => api.get('/currencies'),

  /**
   * 检测用户货币（基于IP）
   * @returns {Promise<Object>} 检测到的货币信息
   */
  detectCurrency: () => api.get('/currencies/detect'),

  /**
   * 获取汇率
   * @param {string} from - 源货币代码
   * @param {string} to - 目标货币代码
   * @returns {Promise<Object>} 汇率信息
   */
  getExchangeRate: (from, to) => api.get('/currencies/rate', { params: { from, to } }),

  /**
   * 转换金额
   * @param {Object} data - 转换数据
   * @param {number} data.amount - 金额（分）
   * @param {string} data.from - 源货币代码
   * @param {string} data.to - 目标货币代码
   * @returns {Promise<Object>} 转换结果
   */
  convertAmount: (data) => api.post('/currencies/convert', data),

  /**
   * 获取带价格的套餐列表
   * @param {string} currency - 货币代码（可选，不传则自动检测）
   * @returns {Promise<Object>} 套餐列表
   */
  getPlansWithPrices: (currency) => api.get('/plans-with-prices', { params: { currency } }),

  /**
   * 获取套餐的所有货币价格
   * @param {number} planId - 套餐ID
   * @param {string} currency - 用户货币代码（可选）
   * @returns {Promise<Object>} 套餐价格信息
   */
  getPlanPrices: (planId, currency) => api.get(`/plans/${planId}/prices`, { params: { currency } }),

  // 管理员 API
  admin: {
    /**
     * 设置套餐的多币种价格
     * @param {number} planId - 套餐ID
     * @param {Object} prices - 价格映射 { currency: price }
     * @returns {Promise<Object>} 操作结果
     */
    setPlanPrices: (planId, prices) => api.put(`/admin/plans/${planId}/prices`, { prices }),

    /**
     * 删除套餐的特定货币价格
     * @param {number} planId - 套餐ID
     * @param {string} currency - 货币代码
     * @returns {Promise<Object>} 操作结果
     */
    deletePlanPrice: (planId, currency) => api.delete(`/admin/plans/${planId}/prices/${currency}`),

    /**
     * 更新汇率
     * @returns {Promise<Object>} 操作结果
     */
    updateExchangeRates: () => api.post('/admin/currencies/update-rates')
  }
}

export default currencyApi
