/**
 * 用户前台认证 API 模块
 */
import api from '@/api/base'

const BASE_URL = '/portal/auth'

/**
 * 用户注册
 * @param {Object} data - 注册数据
 * @param {string} data.username - 用户名
 * @param {string} data.email - 邮箱
 * @param {string} data.password - 密码
 * @param {string} [data.invite_code] - 邀请码（可选）
 * @returns {Promise}
 */
export function register(data) {
  return api.post(`${BASE_URL}/register`, data)
}

/**
 * 用户登录
 * @param {Object} data - 登录数据
 * @param {string} data.username - 用户名或邮箱
 * @param {string} data.password - 密码
 * @param {boolean} [data.remember] - 记住我
 * @returns {Promise}
 */
export function login(data) {
  return api.post(`${BASE_URL}/login`, data)
}

export function getOAuthProviders() {
  return api.get(`${BASE_URL}/oauth/providers`)
}

export function getOAuthEmbedConfig(provider, redirect = '') {
  return api.get(`${BASE_URL}/oauth/${encodeURIComponent(provider)}/embed`, {
    params: redirect ? { redirect } : {}
  })
}

export function getOAuthStartUrl(provider, redirect = '') {
  const baseURL = api.defaults.baseURL || '/api'
  const params = new URLSearchParams()
  if (redirect) {
    params.set('redirect', redirect)
  }
  return `${baseURL}${BASE_URL}/oauth/${encodeURIComponent(provider)}/start${params.toString() ? `?${params.toString()}` : ''}`
}

/**
 * 完成两步验证登录
 * @param {Object} data - 验证数据
 * @param {number} data.user_id - 用户 ID
 * @param {string} data.code - 验证码或备份码
 * @returns {Promise}
 */
export function verify2FALogin(data) {
  return api.post(`${BASE_URL}/2fa/login`, data)
}

/**
 * 用户登出
 * @returns {Promise}
 */
export function logout() {
  return api.post(`${BASE_URL}/logout`)
}

/**
 * 请求密码重置
 * @param {Object} data - 请求数据
 * @param {string} data.email - 邮箱
 * @returns {Promise}
 */
export function forgotPassword(data) {
  return api.post(`${BASE_URL}/forgot-password`, data)
}

/**
 * 重置密码
 * @param {Object} data - 重置数据
 * @param {string} data.token - 重置令牌
 * @param {string} data.password - 新密码
 * @returns {Promise}
 */
export function resetPassword(data) {
  return api.post(`${BASE_URL}/reset-password`, data)
}

/**
 * 验证邮箱
 * @param {string} token - 验证令牌
 * @returns {Promise}
 */
export function verifyEmail(token) {
  return api.get(`${BASE_URL}/verify-email`, { params: { token } })
}

/**
 * 重发邮箱验证邮件
 * @returns {Promise}
 */
export function resendVerificationEmail() {
  return api.post(`${BASE_URL}/verify-email/resend`)
}

/**
 * 绑定 Telegram Chat ID
 * @param {Object} data
 * @param {string} data.chat_id
 * @returns {Promise}
 */
export function bindTelegram(data) {
  return api.post(`${BASE_URL}/telegram/bind`, data)
}

/**
 * 解绑 Telegram
 * @returns {Promise}
 */
export function unbindTelegram() {
  return api.delete(`${BASE_URL}/telegram/bind`)
}

/**
 * 获取用户资料
 * @returns {Promise}
 */
export function getProfile() {
  return api.get(`${BASE_URL}/profile`)
}

/**
 * 更新用户资料
 * @param {Object} data - 资料数据
 * @param {string} [data.display_name] - 显示名称
 * @param {string} [data.avatar_url] - 头像 URL
 * @returns {Promise}
 */
export function updateProfile(data) {
  return api.put(`${BASE_URL}/profile`, data)
}

/**
 * 上传头像
 * @param {File} file - 头像文件
 * @returns {Promise}
 */
export function uploadAvatar(file) {
  const formData = new FormData()
  formData.append('file', file)
  return api.post(`${BASE_URL}/avatar`, formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}

/**
 * 修改密码
 * @param {Object} data - 密码数据
 * @param {string} data.current_password - 当前密码
 * @param {string} data.new_password - 新密码
 * @returns {Promise}
 */
export function changePassword(data) {
  return api.put(`${BASE_URL}/password`, data)
}

/**
 * 启用两步验证
 * @returns {Promise} 返回 QR 码和备份码
 */
export function enable2FA() {
  return api.post(`${BASE_URL}/2fa/enable`)
}

/**
 * 验证两步验证码
 * @param {Object} data - 验证数据
 * @param {string} data.code - TOTP 验证码
 * @returns {Promise}
 */
export function verify2FA(data) {
  return api.post(`${BASE_URL}/2fa/verify`, data)
}

/**
 * 禁用两步验证
 * @param {Object} data - 验证数据
 * @param {string} data.password - 当前密码
 * @returns {Promise}
 */
export function disable2FA(data) {
  return api.post(`${BASE_URL}/2fa/disable`, data)
}

export default {
  register,
  login,
  verify2FALogin,
  logout,
  forgotPassword,
  resetPassword,
  verifyEmail,
  resendVerificationEmail,
  bindTelegram,
  unbindTelegram,
  getProfile,
  updateProfile,
  uploadAvatar,
  changePassword,
  enable2FA,
  verify2FA,
  disable2FA
}
