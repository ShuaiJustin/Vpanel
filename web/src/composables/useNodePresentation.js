const NODE_STATUS_TYPES = Object.freeze({
  online: 'success',
  offline: 'info',
  unhealthy: 'danger'
})

const NODE_STATUS_TEXT = Object.freeze({
  online: '在线',
  offline: '离线',
  unhealthy: '不健康'
})

const NODE_SYNC_STATUS_TYPES = Object.freeze({
  synced: 'success',
  pending: 'warning',
  failed: 'danger'
})

const NODE_SYNC_STATUS_TEXT = Object.freeze({
  synced: '已同步',
  pending: '待同步',
  failed: '同步失败'
})

const RECOVERY_STATUS_TYPES = Object.freeze({
  success: 'success',
  failed: 'danger',
  dispatched: 'warning',
  queued: 'info'
})

const RECOVERY_STATUS_TEXT = Object.freeze({
  success: '已恢复',
  failed: '恢复失败',
  dispatched: '已下发',
  queued: '已排队'
})

const RECOVERY_STATUS_COLORS = Object.freeze({
  success: 'var(--el-color-success)',
  failed: 'var(--el-color-danger)',
  dispatched: 'var(--el-color-warning)',
  queued: 'var(--el-color-info)'
})

const RECOVERY_SOURCE_TEXT = Object.freeze({
  heartbeat: '节点心跳',
  health_checker: '健康检查器',
  admin: '管理员',
  portal_ping: '用户入口探测'
})

const RECOVERY_COMMAND_TEXT = Object.freeze({
  xray_start: '启动 Xray',
  xray_restart: '重启 Xray',
  xray_status: '刷新 Xray 状态',
  config_sync: '同步节点配置'
})

export const formatUsersLimitDisplay = (currentUsers, maxUsers) => (
  maxUsers
    ? `${Number(currentUsers) || 0} / ${maxUsers}`
    : `${Number(currentUsers) || 0} / ∞`
)

export const getNodeStatusType = (status) => NODE_STATUS_TYPES[status] || 'info'

export const getNodeStatusText = (status) => NODE_STATUS_TEXT[status] || status || '未知'

export const getNodeSyncStatusType = (status) => NODE_SYNC_STATUS_TYPES[status] || 'info'

export const getNodeSyncStatusText = (status) => NODE_SYNC_STATUS_TEXT[status] || status || '未知'

export const getNodeLatencyClass = (latency) => {
  const value = Number(latency) || 0
  if (value <= 0) return ''
  if (value < 100) return 'latency-good'
  if (value < 300) return 'latency-medium'
  return 'latency-bad'
}

export const getRecoveryStatusType = (status) => RECOVERY_STATUS_TYPES[status] || 'info'

export const getRecoveryStatusText = (status) => RECOVERY_STATUS_TEXT[status] || status || '未知'

export const getRecoveryStatusColor = (status) => RECOVERY_STATUS_COLORS[status] || 'var(--el-border-color)'

export const getRecoverySourceText = (source) => RECOVERY_SOURCE_TEXT[source] || source || '系统'

export const getRecoveryCommandText = (commandType) => RECOVERY_COMMAND_TEXT[commandType] || commandType || '未知命令'

export const formatNodeTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

export const formatCoreVersion = (version) => {
  if (!version) return '-'
  return String(version).split('\n')[0]
}

export const formatCoreVersionCompact = (version) => {
  const normalized = formatCoreVersion(version)
  if (normalized === '-') return normalized

  const matched = normalized.match(/(Xray\s+\d+(?:\.\d+)+)/i)
  return matched?.[1] || normalized
}

export const parseNodeTags = (tags) => {
  if (Array.isArray(tags)) return tags
  if (typeof tags === 'string') {
    try {
      return JSON.parse(tags)
    } catch {
      return []
    }
  }
  return []
}
