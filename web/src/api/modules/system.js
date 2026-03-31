/**
 * 系统相关 API
 * 处理系统状态、进程管理、资源监控等操作
 */
import api from '../base'

export const systemApi = {
  /**
   * 获取系统状态
   * @returns {Promise<Object>} 系统状态信息
   */
  getStatus: () => api.get('/system/status'),

  /**
   * 获取系统状态（别名，兼容旧代码）
   * @returns {Promise<Object>} 系统状态信息
   */
  getSystemStatus: () => api.get('/system/status'),

  /**
   * 获取系统信息
   * @returns {Promise<Object>} 系统信息
   */
  getInfo: () => api.get('/system/info'),

  /**
   * 获取系统统计
   * @returns {Promise<Object>} 系统统计数据
   */
  getStats: () => api.get('/system/stats'),

  /**
   * 获取 CPU 使用情况
   * @returns {Promise<Object>} CPU 使用数据
   */
  getCpuUsage: () => api.get('/system/cpu'),

  /**
   * 获取内存使用情况
   * @returns {Promise<Object>} 内存使用数据
   */
  getMemoryUsage: () => api.get('/system/memory'),

  /**
   * 获取磁盘使用情况
   * @returns {Promise<Object>} 磁盘使用数据
   */
  getDiskUsage: () => api.get('/system/disk'),

  /**
   * 获取进程列表
   * @returns {Promise<Array>} 进程列表
   */
  getProcesses: () => api.get('/system/processes'),

  /**
   * 终止进程
   * @param {number} pid - 进程 ID
   * @returns {Promise<void>}
   */
  killProcess: (pid) => api.post(`/system/processes/${pid}/kill`),

  /**
   * 获取 CPU 趋势数据
   * @param {string} timeRange - 时间范围
   * @returns {Promise<Array>} CPU 趋势数据
   */
  getCpuTrend: (timeRange) => api.get('/system/trends/cpu', { params: { timeRange } }),

  /**
   * 获取内存趋势数据
   * @param {string} timeRange - 时间范围
   * @returns {Promise<Array>} 内存趋势数据
   */
  getMemoryTrend: (timeRange) => api.get('/system/trends/memory', { params: { timeRange } }),

  /**
   * 获取网络趋势数据
   * @param {string} timeRange - 时间范围
   * @returns {Promise<Array>} 网络趋势数据
   */
  getNetworkTrend: (timeRange) => api.get('/system/trends/network', { params: { timeRange } }),

  /**
   * 获取负载趋势数据
   * @param {string} timeRange - 时间范围
   * @returns {Promise<Array>} 负载趋势数据
   */
  getLoadTrend: (timeRange) => api.get('/system/trends/load', { params: { timeRange } }),

  /**
   * 获取磁盘趋势数据
   * @param {string} timeRange - 时间范围
   * @returns {Promise<Array>} 磁盘趋势数据
   */
  getDiskTrend: (timeRange) => api.get('/system/trends/disk', { params: { timeRange } }),

  /**
   * 获取网络连接列表
   * @returns {Promise<Array>} 网络连接列表
   */
  getNetworkConnections: () => api.get('/system/network/connections'),

  /**
   * 获取网络接口列表
   * @returns {Promise<Array>} 网络接口列表
   */
  getNetworkInterfaces: () => api.get('/system/network/interfaces'),

  /**
   * 获取接口流量
   * @param {string} interfaceName - 接口名称
   * @returns {Promise<Object>} 接口流量数据
   */
  getInterfaceTraffic: (interfaceName) => api.get(`/system/network/interfaces/${interfaceName}/traffic`),

  /**
   * 获取磁盘分区信息
   * @returns {Promise<Array>} 磁盘分区列表
   */
  getDiskPartitions: () => api.get('/system/disk/partitions'),

  /**
   * 获取 IO 统计
   * @param {string} timeRange - 时间范围
   * @returns {Promise<Object>} IO 统计数据
   */
  getIOStats: (timeRange) => api.get('/system/io/stats', { params: { timeRange } }),

  /**
   * 清除网络连接
   * @returns {Promise<void>}
   */
  clearConnections: () => api.post('/system/network/connections/clear'),

  /**
   * 终止网络连接
   * @param {string} id - 连接 ID
   * @returns {Promise<void>}
   */
  killConnection: (id) => api.post(`/system/network/connections/${id}/kill`),

  /**
   * 手动触发运行时巡检
   * @returns {Promise<Object>} 巡检统计结果
   */
  triggerRuntimeReconcile: () => api.post('/admin/system/runtime-reconcile')
}

export default systemApi
