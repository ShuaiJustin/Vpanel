/**
 * 节点管理 API
 * 处理节点的增删改查、状态管理、Token 管理等操作
 * Requirements: 1.1-1.7
 */
import api from "../base";

export const nodesApi = {
  /**
   * 获取节点列表
   * @param {Object} params - 查询参数
   * @param {number} params.limit - 每页数量
   * @param {number} params.offset - 偏移量
   * @param {string} params.status - 状态过滤 (online, offline, unhealthy)
   * @param {string} params.region - 地区过滤
   * @param {number} params.group_id - 分组 ID 过滤
   * @returns {Promise<Object>} 节点列表和总数
   */
  list: (params) => api.get("/admin/nodes", { params }),

  /**
   * 获取单个节点
   * @param {number|string} id - 节点 ID
   * @returns {Promise<Object>} 节点信息
   */
  get: (id) => api.get(`/admin/nodes/${id}`),

  /**
   * 创建节点
   * @param {Object} data - 节点数据
   * @param {string} data.name - 节点名称
   * @param {string} data.address - 节点地址 (IP 或域名)
   * @param {number} data.port - Agent 端口
   * @param {string[]} data.tags - 标签
   * @param {string} data.region - 地区
   * @param {number} data.weight - 负载均衡权重
   * @param {number} data.max_users - 最大用户数 (0 = 无限制)
   * @param {string[]} data.ip_whitelist - IP 白名单
   * @returns {Promise<Object>} 创建的节点 (包含 token)
   */
  create: (data) =>
    api.post("/admin/nodes", data, {
      timeout: 300000, // 5 分钟超时（用于自动部署）
    }),

  /**
   * 更新节点
   * @param {number|string} id - 节点 ID
   * @param {Object} data - 节点数据
   * @returns {Promise<Object>} 更新后的节点
   */
  update: (id, data) => api.put(`/admin/nodes/${id}`, data),

  /**
   * 删除节点
   * @param {number|string} id - 节点 ID
   * @returns {Promise<void>}
   */
  delete: (id) => api.delete(`/admin/nodes/${id}`),

  /**
   * 更新节点状态
   * @param {number|string} id - 节点 ID
   * @param {string} status - 状态 (online, offline, unhealthy)
   * @returns {Promise<void>}
   */
  updateStatus: (id, status) =>
    api.put(`/admin/nodes/${id}/status`, { status }),

  /**
   * 生成节点 Token
   * @param {number|string} id - 节点 ID
   * @returns {Promise<Object>} 包含新 token
   */
  generateToken: (id) => api.post(`/admin/nodes/${id}/token`),

  /**
   * 轮换节点 Token
   * @param {number|string} id - 节点 ID
   * @returns {Promise<Object>} 包含新 token
   */
  rotateToken: (id) => api.post(`/admin/nodes/${id}/token/rotate`),

  /**
   * 撤销节点 Token
   * @param {number|string} id - 节点 ID
   * @returns {Promise<void>}
   */
  revokeToken: (id) => api.post(`/admin/nodes/${id}/token/revoke`),

  /**
   * 获取节点统计信息
   * @returns {Promise<Object>} 按状态分组的统计和总用户数
   */
  getStatistics: () => api.get("/admin/nodes/statistics"),

  /**
   * 获取节点流量统计
   * @param {number|string} id - 节点 ID
   * @param {Object} params - 查询参数
   * @param {string} params.start - 开始时间 (RFC3339)
   * @param {string} params.end - 结束时间 (RFC3339)
   * @returns {Promise<Object>} 流量统计
   */
  getTraffic: (id, params) => api.get(`/admin/nodes/${id}/traffic`, { params }),

  /**
   * 获取节点流量 Top 用户
   * @param {number|string} id - 节点 ID
   * @param {Object} params - 查询参数
   * @param {number} params.limit - 返回数量
   * @param {string} params.start - 开始时间
   * @param {string} params.end - 结束时间
   * @returns {Promise<Object>} Top 用户列表
   */
  getTopUsers: (id, params) =>
    api.get(`/admin/nodes/${id}/traffic/top-users`, { params }),

  /**
   * 获取总流量统计
   * @param {Object} params - 查询参数
   * @param {string} params.start - 开始时间
   * @param {string} params.end - 结束时间
   * @returns {Promise<Object>} 总流量统计
   */
  getTotalTraffic: (params) =>
    api.get("/admin/nodes/traffic/total", { params }),

  /**
   * 获取按节点分组的流量统计
   * @param {Object} params - 查询参数
   * @returns {Promise<Object>} 按节点分组的流量
   */
  getTrafficByNode: (params) =>
    api.get("/admin/nodes/traffic/by-node", { params }),

  /**
   * 获取按分组的流量统计
   * @param {Object} params - 查询参数
   * @returns {Promise<Object>} 按分组的流量
   */
  getTrafficByGroup: (params) =>
    api.get("/admin/nodes/traffic/by-group", { params }),

  /**
   * 获取聚合流量统计
   * @param {Object} params - 查询参数
   * @returns {Promise<Object>} 聚合统计
   */
  getAggregatedStats: (params) =>
    api.get("/admin/nodes/traffic/aggregated", { params }),

  /**
   * 获取实时流量统计
   * @returns {Promise<Object>} 实时流量数据
   */
  getRealTimeStats: () => api.get("/admin/nodes/traffic/realtime"),

  /**
   * 获取集群健康状态
   * @returns {Promise<Object>} 集群健康概览
   */
  getClusterHealth: () => api.get("/admin/nodes/cluster-health"),

  /**
   * 远程部署 Agent
   * @param {number|string} id - 节点 ID
   * @param {Object} data - 部署配置
   * @param {string} data.host - 远程服务器 IP/域名
   * @param {number} data.port - SSH 端口 (默认 22)
   * @param {string} data.username - SSH 用户名
   * @param {string} data.password - SSH 密码 (可选)
   * @param {string} data.private_key - SSH 私钥 (可选)
   * @returns {Promise<Object>} 部署结果
   */
  deployAgent: (id, data) =>
    api.post(`/admin/nodes/${id}/deploy`, data, {
      timeout: 300000, // 5 分钟超时（上传和安装需要时间）
    }),

  /**
   * 获取节点自动安装状态
   * @param {number|string} id - 节点 ID
   * @returns {Promise<Object>} 安装状态、步骤和日志
   */
  getInstallStatus: (id) => api.get(`/admin/nodes/${id}/install-status`),

  /**
   * 获取部署脚本
   * @param {number|string} id - 节点 ID
   * @param {string} panelUrl - 面板 URL (可选)
   * @returns {Promise<Blob>} 脚本文件
   */
  getDeployScript: (id, panelUrl) => {
    const params = panelUrl ? { panel_url: panelUrl } : {};
    return api.get(`/admin/nodes/${id}/deploy/script`, {
      params,
      responseType: "blob",
    });
  },

  /**
   * 测试 SSH 连接
   * @param {Object} data - 连接配置
   * @param {string} data.host - 远程服务器 IP/域名
   * @param {number} data.port - SSH 端口 (默认 22)
   * @param {string} data.username - SSH 用户名
   * @param {string} data.password - SSH 密码 (可选)
   * @param {string} data.private_key - SSH 私钥 (可选)
   * @returns {Promise<Object>} 测试结果
   */
  testConnection: (data) => api.post("/admin/nodes/test-connection", data),
};

export default nodesApi;
