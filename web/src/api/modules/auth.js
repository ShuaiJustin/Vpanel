/**
 * 认证相关 API
 * 处理用户登录、登出、Token 刷新等认证操作
 */
import api from '../base'

export const authApi = {
  /**
   * 用户登录
   * @param {Object} data - 登录数据
   * @param {string} data.username - 用户名
   * @param {string} data.password - 密码
   * @returns {Promise<Object>} 登录结果，包含 token
   */
  login: (data) => api.post('/auth/login', data),
  verify2FALogin: (data) => api.post('/auth/2fa/login', data),

  /**
   * 用户登出
   * @returns {Promise<void>}
   */
  logout: () => api.post('/auth/logout'),

  /**
   * 刷新 Token
   * @returns {Promise<Object>} 新的 token
   */
  refresh: (refreshToken) => api.post('/auth/refresh', { refresh_token: refreshToken }),

  /**
   * 获取当前用户信息
   * @returns {Promise<Object>} 用户信息
   */
  getProfile: () => api.get('/auth/me'),

  /**
   * 更新当前用户信息
   * @param {Object} data - 用户信息
   * @returns {Promise<Object>} 更新后的用户信息
   */
  updateProfile: (data) => api.put('/auth/me', data),

  /**
   * 修改密码
   * @param {Object} data - 密码数据
   * @param {string} data.oldPassword - 旧密码
   * @param {string} data.newPassword - 新密码
   * @returns {Promise<void>}
   */
  changePassword: (data) => api.put('/auth/password', data)
}

export default authApi
