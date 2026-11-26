/**
 * 系统设置 API
 * 处理系统配置的获取、更新、备份和恢复
 */
import api from '../base'

export const settingsApi = {
  /**
   * 获取所有设置
   * @returns {Promise<Object>} 系统设置
   */
  getAll: () => api.get('/settings'),

  /**
   * 更新设置
   * @param {Object} data - 设置数据
   * @returns {Promise<Object>} 更新后的设置
   */
  update: (data) => api.put('/settings', data),

  /**
   * 发送测试邮件
   * @param {Object} data - 测试邮件参数
   * @param {string} [data.to] - 收件人邮箱
   * @returns {Promise<Object>} 发送结果
   */
  testEmail: (data) => api.post('/settings/test-email', data),

  /**
   * 测试数据库连接
   * @param {Object} data - 数据库配置
   * @returns {Promise<Object>}
   */
  testDatabase: (data) => api.post('/settings/test-db', data),

  /**
   * 创建数据库备份
   * @returns {Promise<Object>}
   */
  backupDatabase: () => api.post('/settings/backup-db'),

  /**
   * 创建设置备份
   * @returns {Promise<Object>} 备份信息
   */
  createBackup: () => api.post('/settings/backup'),

  /**
   * 恢复设置
   * @param {Object} data - 备份数据
   * @returns {Promise<void>}
   */
  restore: (data) => api.post('/settings/restore', data)
}

export default settingsApi
