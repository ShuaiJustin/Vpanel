/**
 * 用户前台公告 API 模块
 */
import api from '@/api/base'

const BASE_URL = '/portal/announcements'

/**
 * 获取公告列表
 * @param {Object} [params] - 查询参数
 * @param {string} [params.category] - 分类筛选
 * @param {number} [params.limit] - 每页数量
 * @param {number} [params.offset] - 偏移量
 * @returns {Promise}
 */
export function getAnnouncements(params = {}) {
  return api.get(BASE_URL, { params })
}

/**
 * 获取公告详情
 * @param {number} id - 公告 ID
 * @returns {Promise}
 */
export function getAnnouncement(id) {
  return api.get(`${BASE_URL}/${id}`)
}

/**
 * 标记公告为已读
 * @param {number} id - 公告 ID
 * @returns {Promise}
 */
export function markAsRead(id) {
  return api.post(`${BASE_URL}/${id}/read`)
}

/**
 * 获取未读公告数量
 * @returns {Promise}
 */
export function getUnreadCount() {
  return api.get(`${BASE_URL}/unread-count`)
}

/**
 * 批量标记为已读
 * @param {number[]} ids - 公告 ID 列表
 * @returns {Promise}
 */
export function batchMarkAsRead(ids) {
  return Promise.all((ids || []).map(id => markAsRead(id)))
}

/**
 * 标记所有为已读
 * @returns {Promise}
 */
export function markAllAsRead(ids = []) {
  return batchMarkAsRead(ids)
}

export default {
  getAnnouncements,
  getAnnouncement,
  markAsRead,
  getUnreadCount,
  batchMarkAsRead,
  markAllAsRead
}
