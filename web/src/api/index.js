/**
 * API 统一入口
 * 按领域组织所有 API 模块，提供统一的导出接口
 */

// 基础配置和工具
import api, { 
  generateErrorId, 
  getErrorMessage, 
  formatApiError 
} from './base'

// API 模块
import { authApi } from './modules/auth'
import { usersApi } from './modules/users'
import { proxiesApi } from './modules/proxies'
import { systemApi } from './modules/system'
import { settingsApi } from './modules/settings'
import { statsApi } from './modules/stats'
import { certificatesApi } from './modules/certificates'
import { rolesApi } from './modules/roles'
import { xrayApi } from './modules/xray'
import { logsApi } from './modules/logs'
import { subscriptionApi } from './modules/subscription'

// 商业化模块
import { plansApi } from './modules/plans'
import { ordersApi } from './modules/orders'
import { paymentsApi } from './modules/payments'
import { balanceApi } from './modules/balance'
import { couponsApi } from './modules/coupons'
import { invitesApi } from './modules/invites'
import { invoicesApi } from './modules/invoices'
import { planChangeApi } from './modules/planchange'
import { currencyApi } from './modules/currency'
import pauseApi from './modules/pause'
import { giftCardsApi } from './modules/giftcards'

// 节点管理模块
import { nodesApi } from './modules/nodes'
import { nodeGroupsApi } from './modules/nodeGroups'
import { nodeHealthApi } from './modules/nodeHealth'

// 导出所有 API 模块
export {
  // 基础
  api as default,
  generateErrorId,
  getErrorMessage,
  formatApiError,

  // 认证
  authApi,

  // 用户管理
  usersApi,

  // 代理管理
  proxiesApi,

  // 系统
  systemApi,

  // 设置
  settingsApi,

  // 统计
  statsApi,

  // 证书
  certificatesApi,

  // 角色
  rolesApi,

  // Xray
  xrayApi,

  // 日志
  logsApi,

  // 订阅
  subscriptionApi,

  // 商业化
  plansApi,
  ordersApi,
  paymentsApi,
  balanceApi,
  couponsApi,
  invitesApi,
  invoicesApi,
  planChangeApi,
  currencyApi,
  pauseApi,
  giftCardsApi,

  // 节点管理
  nodesApi,
  nodeGroupsApi,
  nodeHealthApi
}

// 兼容旧版导出（逐步废弃）
// @deprecated 请使用新的模块化导出
export const auth = authApi
export const users = usersApi
export const proxies = proxiesApi
export const roles = {
  getRoles: rolesApi.list,
  getRole: rolesApi.get,
  createRole: rolesApi.create,
  updateRole: rolesApi.update,
  deleteRole: rolesApi.delete,
  getPermissions: rolesApi.getPermissions
}
export const userApi = {
  getInfo: authApi.getProfile,
  updateInfo: authApi.updateProfile,
  changePassword: authApi.changePassword
}
