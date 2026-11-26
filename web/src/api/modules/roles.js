/**
 * 角色管理 API
 * 处理角色的增删改查和权限管理
 */
import api from '../base'

export const rolesApi = {
  /**
   * 获取角色列表
   * @returns {Promise<Array>} 角色列表
   */
  list: () => api.get('/roles'),

  /**
   * 获取单个角色
   * @param {number|string} id - 角色 ID
   * @returns {Promise<Object>} 角色信息
   */
  get: (id) => api.get(`/roles/${id}`),

  /**
   * 创建角色
   * @param {Object} data - 角色数据
   * @param {string} data.name - 角色名称
   * @param {string} data.description - 角色描述
   * @param {Array<string>} data.permissions - 权限列表
   * @returns {Promise<Object>} 创建的角色
   */
  create: (data) => api.post('/roles', data),

  /**
   * 更新角色
   * @param {number|string} id - 角色 ID
   * @param {Object} data - 角色数据
   * @returns {Promise<Object>} 更新后的角色
   */
  update: (id, data) => api.put(`/roles/${id}`, data),

  /**
   * 删除角色
   * @param {number|string} id - 角色 ID
   * @returns {Promise<void>}
   */
  delete: (id) => api.delete(`/roles/${id}`),

  /**
   * 获取所有权限列表
   * @returns {Promise<Array>} 权限列表
   */
  getPermissions: () => api.get('/permissions')
}

export default rolesApi
