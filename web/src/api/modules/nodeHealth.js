/**
 * 节点健康检查 API
 * 处理节点健康检查、历史记录、健康检查器管理等操作
 * Requirements: 2.1-2.8
 */
import api from '../base'

export const nodeHealthApi = {
  /**
   * 对单个节点执行健康检查
   * @param {number|string} id - 节点 ID
   * @returns {Promise<Object>} 健康检查结果
   */
  checkNode: (id) => api.post(`/admin/nodes/${id}/health-check`),

  /**
   * 对所有节点执行健康检查
   * @returns {Promise<Object>} 所有节点的健康检查结果
   */
  checkAll: () => api.post('/admin/nodes/health-check'),

  /**
   * 获取节点健康检查历史
   * @param {number|string} id - 节点 ID
   * @param {Object} params - 查询参数
   * @param {number} params.limit - 返回数量
   * @returns {Promise<Object>} 健康检查历史列表
   */
  getHistory: (id, params) => api.get(`/admin/nodes/${id}/health-history`, { params }),

  /**
   * 获取节点最新健康检查结果
   * @param {number|string} id - 节点 ID
   * @returns {Promise<Object>} 最新健康检查结果
   */
  getLatest: (id) => api.get(`/admin/nodes/${id}/health-latest`),

  /**
   * 获取节点健康统计
   * @param {number|string} id - 节点 ID
   * @param {Object} params - 查询参数
   * @param {string} params.since - 开始时间 (RFC3339)
   * @returns {Promise<Object>} 健康统计（成功率、平均延迟等）
   */
  getStats: (id, params) => api.get(`/admin/nodes/${id}/health-stats`, { params }),

  /**
   * 获取健康检查器状态
   * @returns {Promise<Object>} 健康检查器状态（运行状态、间隔、阈值）
   */
  getCheckerStatus: () => api.get('/admin/health-checker/status'),

  /**
   * 启动健康检查器
   * @returns {Promise<void>}
   */
  startChecker: () => api.post('/admin/health-checker/start'),

  /**
   * 停止健康检查器
   * @returns {Promise<void>}
   */
  stopChecker: () => api.post('/admin/health-checker/stop'),

  /**
   * 更新健康检查器配置
   * @param {Object} config - 配置数据
   * @param {string} config.interval - 检查间隔 (如 "30s", "1m")
   * @param {number} config.unhealthy_threshold - 不健康阈值（连续失败次数）
   * @param {number} config.healthy_threshold - 健康阈值（连续成功次数）
   * @returns {Promise<Object>} 更新后的配置
   */
  updateCheckerConfig: (config) => api.put('/admin/health-checker/config', config),

  /**
   * 获取集群健康概览
   * @returns {Promise<Object>} 集群健康状态（总节点数、在线数、离线数、不健康数）
   */
  getClusterHealth: () => api.get('/admin/nodes/cluster-health')
}

export default nodeHealthApi
