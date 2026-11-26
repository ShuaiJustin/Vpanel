/**
 * API 类型定义
 * 定义所有 API 请求和响应的 TypeScript 类型
 */

// ============ 通用类型 ============

/**
 * 分页参数
 */
export interface PaginationParams {
  page?: number
  pageSize?: number
}

/**
 * 分页响应
 */
export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

/**
 * API 错误响应
 */
export interface ApiError {
  errorId: string
  code: string
  message: string
  details?: Record<string, any>
  requestId?: string
  status: number
  timestamp: string
}

/**
 * 时间段类型
 */
export type TimePeriod = 'today' | 'week' | 'month' | 'year' | 'custom'

/**
 * 时间粒度
 */
export type TimeGranularity = 'hourly' | 'daily' | 'weekly' | 'monthly'

// ============ 认证相关 ============

/**
 * 登录请求
 */
export interface LoginRequest {
  username: string
  password: string
}

/**
 * 登录响应
 */
export interface LoginResponse {
  token: string
  refreshToken?: string
  expiresIn: number
  user: User
}

/**
 * 修改密码请求
 */
export interface ChangePasswordRequest {
  oldPassword: string
  newPassword: string
}

// ============ 用户相关 ============

/**
 * 用户
 */
export interface User {
  id: number
  username: string
  email?: string
  role: string
  enabled: boolean
  trafficLimit: number
  trafficUsed: number
  expiresAt?: string
  forcePasswordChange: boolean
  createdAt: string
  updatedAt: string
}

/**
 * 创建用户请求
 */
export interface CreateUserRequest {
  username: string
  password: string
  email?: string
  role?: string
  trafficLimit?: number
  expiresAt?: string
}

/**
 * 更新用户请求
 */
export interface UpdateUserRequest {
  email?: string
  role?: string
  trafficLimit?: number
  expiresAt?: string
}

/**
 * 用户列表查询参数
 */
export interface UserListParams extends PaginationParams {
  search?: string
  role?: string
  enabled?: boolean
}

/**
 * 登录历史记录
 */
export interface LoginHistory {
  id: number
  userId: number
  ip: string
  userAgent: string
  success: boolean
  createdAt: string
}

// ============ 代理相关 ============

/**
 * 代理协议类型
 */
export type ProxyProtocol = 'vmess' | 'vless' | 'trojan' | 'shadowsocks'

/**
 * 代理
 */
export interface Proxy {
  id: number
  userId: number
  name: string
  protocol: ProxyProtocol
  port: number
  host?: string
  settings: Record<string, any>
  enabled: boolean
  running: boolean
  remark?: string
  createdAt: string
  updatedAt: string
}

/**
 * 创建代理请求
 */
export interface CreateProxyRequest {
  name: string
  protocol: ProxyProtocol
  port: number
  host?: string
  settings: Record<string, any>
  remark?: string
}

/**
 * 更新代理请求
 */
export interface UpdateProxyRequest {
  name?: string
  port?: number
  host?: string
  settings?: Record<string, any>
  remark?: string
}

/**
 * 代理列表查询参数
 */
export interface ProxyListParams extends PaginationParams {
  protocol?: ProxyProtocol
  enabled?: boolean
}

/**
 * 代理统计
 */
export interface ProxyStats {
  upload: number
  download: number
  total: number
  connectionCount: number
  lastActive?: string
}

// ============ 角色相关 ============

/**
 * 角色
 */
export interface Role {
  id: number
  name: string
  description: string
  permissions: string[]
  isSystem: boolean
  userCount: number
  createdAt: string
  updatedAt: string
}

/**
 * 创建角色请求
 */
export interface CreateRoleRequest {
  name: string
  description?: string
  permissions: string[]
}

/**
 * 更新角色请求
 */
export interface UpdateRoleRequest {
  name?: string
  description?: string
  permissions?: string[]
}

// ============ 设置相关 ============

/**
 * 系统设置
 */
export interface SystemSettings {
  siteName: string
  siteDescription: string
  allowRegistration: boolean
  defaultTrafficLimit: number
  defaultExpiryDays: number
  smtpHost?: string
  smtpPort?: number
  smtpUser?: string
  telegramChatId?: string
  rateLimitEnabled: boolean
  rateLimitRequests: number
  rateLimitWindow: number
}

// ============ 统计相关 ============

/**
 * 仪表盘统计
 */
export interface DashboardStats {
  totalUsers: number
  activeUsers: number
  totalProxies: number
  activeProxies: number
  totalTraffic: number
  uploadTraffic: number
  downloadTraffic: number
  onlineCount: number
}

/**
 * 协议统计
 */
export interface ProtocolStats {
  protocol: ProxyProtocol
  count: number
  upload: number
  download: number
  total: number
}

/**
 * 用户统计
 */
export interface UserStats {
  userId: number
  username: string
  proxyCount: number
  upload: number
  download: number
  total: number
}

/**
 * 流量时间线数据点
 */
export interface TrafficTimelinePoint {
  timestamp: string
  upload: number
  download: number
  total: number
}

/**
 * 统计查询参数
 */
export interface StatsParams {
  period?: TimePeriod
  startDate?: string
  endDate?: string
}

/**
 * 时间线查询参数
 */
export interface TimelineParams extends StatsParams {
  granularity?: TimeGranularity
}

// ============ 系统相关 ============

/**
 * 系统信息
 */
export interface SystemInfo {
  hostname: string
  os: string
  platform: string
  arch: string
  cpuCores: number
  totalMemory: number
  uptime: number
}

/**
 * 系统状态
 */
export interface SystemStatus {
  cpuUsage: number
  memoryUsage: number
  diskUsage: number
  networkIn: number
  networkOut: number
  load: number[]
}

// ============ Xray 相关 ============

/**
 * Xray 状态
 */
export interface XrayStatus {
  running: boolean
  pid?: number
  uptime?: string
  version: string
  connections: number
  startedAt?: string
}

/**
 * Xray 版本信息
 */
export interface XrayVersion {
  current: string
  latest: string
  canUpdate: boolean
}

// ============ 日志相关 ============

/**
 * 日志级别
 */
export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

/**
 * 日志条目
 */
export interface LogEntry {
  id: number
  level: LogLevel
  message: string
  source: string
  details?: Record<string, any>
  createdAt: string
}

/**
 * 日志查询参数
 */
export interface LogListParams extends PaginationParams {
  level?: LogLevel
  search?: string
  startDate?: string
  endDate?: string
}

/**
 * 日志轮转配置
 */
export interface LogRotationConfig {
  maxSize: number
  maxAge: number
  maxBackups: number
  compress: boolean
}

// ============ 备份相关 ============

/**
 * 备份
 */
export interface Backup {
  id: number
  name: string
  size: number
  type: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  createdAt: string
}

/**
 * 备份配置
 */
export interface BackupConfig {
  autoBackup: boolean
  backupInterval: number
  maxBackups: number
  includeDatabase: boolean
  includeConfig: boolean
  includeLogs: boolean
}

/**
 * 备份调度配置
 */
export interface BackupScheduleConfig {
  enabled: boolean
  cron: string
  retention: number
}

/**
 * 备份存储配置
 */
export interface BackupStorageConfig {
  type: 'local' | 's3' | 'ftp'
  path?: string
  s3Bucket?: string
  s3Region?: string
  ftpHost?: string
  ftpPort?: number
}

// ============ 证书相关 ============

/**
 * SSL 证书
 */
export interface Certificate {
  id: number
  domain: string
  issuer: string
  validFrom: string
  validTo: string
  autoRenew: boolean
  status: 'valid' | 'expiring' | 'expired'
  createdAt: string
}

/**
 * 证书申请请求
 */
export interface CertificateApplyRequest {
  domain: string
  email?: string
}

// ============ 客户端相关 ============

/**
 * 客户端
 */
export interface Client {
  id: number
  name: string
  email?: string
  enabled: boolean
  trafficLimit: number
  trafficUsed: number
  expiresAt?: string
  createdAt: string
  updatedAt: string
}

/**
 * 创建客户端请求
 */
export interface CreateClientRequest {
  name: string
  email?: string
  trafficLimit?: number
  expiresAt?: string
}

/**
 * 更新客户端请求
 */
export interface UpdateClientRequest {
  name?: string
  email?: string
  trafficLimit?: number
  expiresAt?: string
}

// ============ 监控相关 ============

/**
 * 告警设置
 */
export interface AlertSettings {
  enabled: boolean
  cpuThreshold: number
  memoryThreshold: number
  diskThreshold: number
  emailEnabled: boolean
  webhookEnabled: boolean
  webhookUrl?: string
}

/**
 * 告警历史
 */
export interface AlertHistory {
  id: number
  type: string
  message: string
  severity: 'info' | 'warning' | 'critical'
  acknowledged: boolean
  createdAt: string
}

/**
 * 网络连接
 */
export interface NetworkConnection {
  id: string
  localAddr: string
  remoteAddr: string
  protocol: string
  status: string
  pid?: number
  process?: string
}

// ============ 事件相关 ============

/**
 * 系统事件
 */
export interface SystemEvent {
  id: number
  type: string
  message: string
  details?: Record<string, any>
  createdAt: string
}

/**
 * 事件查询参数
 */
export interface EventListParams extends PaginationParams {
  type?: string
}
