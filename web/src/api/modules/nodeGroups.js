/**
 * 节点分组管理 API
 * 处理节点分组的增删改查、成员管理、统计等操作
 * Requirements: 6.1-6.6
 */
import api from '../base'

export const nodeGroupsApi = {
  /**
   * 获取分组列表
   * @param {Object} params - 查询参数
   * @param {number} params.limit - 每页数量
   * @param {number} params.offset - 偏移量
   * @returns {Promise<Object>} 分组列表和总数
   */
  list: (params) => api.get('/admin/node-groups', { params }),

  /**
   * 获取分组列表（包含统计信息）
   * @param {Object} params - 查询参数
   * @param {number} params.limit - 每页数量
   * @param {number} params.offset - 偏移量
   * @returns {Promise<Object>} 分组列表（含统计）和总数
   */
  listWithStats: (params) => api.get('/admin/node-groups/with-stats', { params }),

  /**
   * 获取单个分组
   * @param {number|string} id - 分组 ID
   * @returns {Promise<Object>} 分组信息
   */
  get: (id) => api.get(`/admin/node-groups/${id}`),

  /**
   * 获取分组统计信息
   * @param {number|string} id - 分组 ID
   * @returns {Promise<Object>} 分组统计（含节点数、健康节点数、用户数）
   */
  getStats: (id) => api.get(`/admin/node-groups/${id}/stats`),

  /**
   * 创建分组
   * @param {Object} data - 分组数据
   * @param {string} data.name - 分组名称
   * @param {string} data.description - 分组描述
   * @param {string} data.region - 地区
   * @param {string} data.strategy - 负载均衡策略
   * @returns {Promise<Object>} 创建的分组
   */
  create: (data) => api.post('/admin/node-groups', data),

  /**
   * 更新分组
   * @param {number|string} id - 分组 ID
   * @param {Object} data - 分组数据
   * @returns {Promise<Object>} 更新后的分组
   */
  update: (id, data) => api.put(`/admin/node-groups/${id}`, data),

  /**
   * 删除分组
   * @param {number|string} id - 分组 ID
   * @returns {Promise<void>}
   */
  delete: (id) => api.delete(`/admin/node-groups/${id}`),

  /**
   * 获取分组内的节点列表
   * @param {number|string} id - 分组 ID
   * @returns {Promise<Object>} 节点列表
   */
  getNodes: (id) => api.get(`/admin/node-groups/${id}/nodes`),

  /**
   * 添加节点到分组
   * @param {number|string} groupId - 分组 ID
   * @param {number|string} nodeId - 节点 ID
   * @returns {Promise<void>}
   */
  addNode: (groupId, nodeId) => api.post(`/admin/node-groups/${groupId}/nodes/${nodeId}`),

  /**
   * 从分组移除节点
   * @param {number|string} groupId - 分组 ID
   * @param {number|string} nodeId - 节点 ID
   * @returns {Promise<void>}
   */
  removeNode: (groupId, nodeId) => api.delete(`/admin/node-groups/${groupId}/nodes/${nodeId}`),

  /**
   * 设置分组的节点列表（替换现有成员）
   * @param {number|string} id - 分组 ID
   * @param {number[]} nodeIds - 节点 ID 列表
   * @returns {Promise<void>}
   */
  setNodes: (id, nodeIds) => api.put(`/admin/node-groups/${id}/nodes`, { node_ids: nodeIds }),

  /**
   * 获取所有分组的统计信息
   * @returns {Promise<Object>} 所有分组的统计
   */
  getAllStats: () => api.get('/admin/node-groups/stats')
}

export default nodeGroupsApi
