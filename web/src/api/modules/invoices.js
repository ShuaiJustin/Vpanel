/**
 * 交易凭证列表、下载及签发
 */
import api from '../base'

export const invoicesApi = {
  /**
   * 获取交易凭证列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @returns {Promise<Object>} 交易凭证列表
   */
  list: (params = {}) => api.get('/invoices', { params }),

  download: (id) => api.get(`/invoices/${id}/download`, { responseType: 'blob' }),

  admin: {
    issue: (orderId) => api.post('/admin/invoices/generate', { order_id: orderId }, { silent: true })
  }
}

export default invoicesApi
