/**
 * 用户管理 API
 * 处理用户的增删改查、状态管理、密码重置等操作
 */
import api from '../base'

export const usersApi = {
  /**
   * 获取用户列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.pageSize - 每页数量
   * @param {string} params.search - 搜索关键词
   * @returns {Promise<Object>} 用户列表
   */
  list: (params) => api.get('/users', { params }),

  /**
   * 获取单个用户
   * @param {number|string} id - 用户 ID
   * @returns {Promise<Object>} 用户信息
   */
  get: (id) => api.get(`/users/${id}`),

  /**
   * 创建用户
   * @param {Object} data - 用户数据
   * @returns {Promise<Object>} 创建的用户
   */
  create: (data) => api.post('/users', data),

  /**
   * 更新用户
   * @param {number|string} id - 用户 ID
   * @param {Object} data - 用户数据
   * @returns {Promise<Object>} 更新后的用户
   */
  update: (id, data) => api.put(`/users/${id}`, data),

  /**
   * 删除用户
   * @param {number|string} id - 用户 ID
   * @returns {Promise<void>}
   */
  delete: (id) => api.delete(`/users/${id}`),

  /**
   * 启用用户
   * @param {number|string} id - 用户 ID
   * @returns {Promise<void>}
   */
  enable: (id) => api.post(`/users/${id}/enable`),

  /**
   * 禁用用户
   * @param {number|string} id - 用户 ID
   * @returns {Promise<void>}
   */
  disable: (id) => api.post(`/users/${id}/disable`),

  /**
   * 重置用户密码
   * @param {number|string} id - 用户 ID
   * @returns {Promise<Object>} 包含临时密码
   */
  resetPassword: (id) => api.post(`/users/${id}/reset-password`),

  /**
   * 重建用户自动生成的代理
   * @param {number|string} id - 用户 ID
   * @param {Object} data - 可选参数
   * @param {number} data.node_id - 可选，仅重建指定节点
   * @returns {Promise<Object>} 重建结果
   */
  rebuildAutoProxies: (id, data = undefined) => api.post(`/users/${id}/rebuild-auto-proxies`, data),

  /**
   * 获取用户登录历史
   * @param {number|string} id - 用户 ID
   * @param {Object} params - 查询参数
   * @returns {Promise<Object>} 登录历史列表
   */
  getLoginHistory: (id, params) => api.get(`/users/${id}/login-history`, { params }),

  /**
   * 清除用户登录历史
   * @param {number|string} id - 用户 ID
   * @returns {Promise<void>}
   */
  clearLoginHistory: (id) => api.delete(`/users/${id}/login-history`)
}

export default usersApi
