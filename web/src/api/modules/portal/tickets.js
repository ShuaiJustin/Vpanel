/**
 * 用户前台工单 API 模块
 */
import api from '@/api/base'

const BASE_URL = '/portal/tickets'

/**
 * 获取工单列表
 * @param {Object} [params] - 查询参数
 * @param {string} [params.status] - 状态筛选
 * @param {number} [params.limit] - 每页数量
 * @param {number} [params.offset] - 偏移量
 * @returns {Promise}
 */
export function getTickets(params = {}) {
  return api.get(BASE_URL, { params })
}

/**
 * 创建工单
 * @param {Object} data - 工单数据
 * @param {string} data.subject - 主题
 * @param {string} data.content - 内容
 * @param {string} [data.category] - 分类
 * @param {string} [data.priority] - 优先级
 * @returns {Promise}
 */
export function createTicket(data) {
  return api.post(BASE_URL, data)
}

/**
 * 获取工单详情
 * @param {number} id - 工单 ID
 * @returns {Promise}
 */
export function getTicket(id) {
  return api.get(`${BASE_URL}/${id}`)
}

/**
 * 回复工单
 * @param {number} id - 工单 ID
 * @param {Object} data - 回复数据
 * @param {string} data.content - 回复内容
 * @returns {Promise}
 */
export function replyTicket(id, data) {
  return api.post(`${BASE_URL}/${id}/reply`, data)
}

/**
 * 关闭工单
 * @param {number} id - 工单 ID
 * @returns {Promise}
 */
export function closeTicket(id) {
  return api.post(`${BASE_URL}/${id}/close`)
}

/**
 * 重新打开工单
 * @param {number} id - 工单 ID
 * @returns {Promise}
 */
export function reopenTicket(id) {
  return api.post(`${BASE_URL}/${id}/reopen`)
}

/**
 * 上传附件
 * @param {number} ticketId - 工单 ID
 * @param {File} file - 文件
 * @returns {Promise}
 */
export function uploadAttachment(ticketId, file) {
  const formData = new FormData()
  formData.append('file', file)
  return api.post(`${BASE_URL}/${ticketId}/attachments`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export default {
  getTickets,
  createTicket,
  getTicket,
  replyTicket,
  closeTicket,
  reopenTicket,
  uploadAttachment
}
