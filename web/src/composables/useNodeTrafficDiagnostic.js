export const NODE_TRAFFIC_DIAGNOSTIC_STATUS_TEXT = Object.freeze({
  agent_unreachable: 'Agent 不可达',
  agent_no_traffic_endpoint: 'Agent 版本过旧',
  xray_not_running: 'Xray 未运行',
  collector_error: '采集器异常',
  healthy_idle: '采集正常（空闲）',
  healthy_collecting: '采集正常（采集中）'
})

export const NODE_TRAFFIC_DIAGNOSTIC_STATUS_TYPES = Object.freeze({
  agent_unreachable: 'danger',
  agent_no_traffic_endpoint: 'warning',
  xray_not_running: 'warning',
  collector_error: 'danger',
  healthy_idle: 'info',
  healthy_collecting: 'success'
})

const normalizePathList = (values) => {
  if (!Array.isArray(values)) {
    return []
  }
  return values
    .map((value) => String(value || '').trim())
    .filter(Boolean)
}

export const normalizeNodeTrafficDiagnostic = (response) => {
  const payload = response?.data || response || {}
  const traffic = payload?.traffic || {}

  return {
    node_id: Number(payload?.node_id) || 0,
    node_name: String(payload?.node_name || '').trim(),
    address: String(payload?.address || '').trim(),
    port: Number(payload?.port) || 0,
    diagnostic_status: String(payload?.diagnostic_status || '').trim(),
    message: String(payload?.message || '').trim(),
    traffic: {
      status: String(traffic?.status || '').trim(),
      configured_config_path: String(traffic?.configured_config_path || '').trim(),
      resolved_config_path: String(traffic?.resolved_config_path || '').trim(),
      candidate_config_paths: normalizePathList(traffic?.candidate_config_paths),
      api_port: Number(traffic?.api_port) || 0,
      xray_running: Boolean(traffic?.xray_running),
      last_collection_at: traffic?.last_collection_at || '',
      last_success_at: traffic?.last_success_at || '',
      last_error: String(traffic?.last_error || '').trim(),
      last_error_at: traffic?.last_error_at || '',
      last_record_count: Number(traffic?.last_record_count) || 0
    }
  }
}

export const getNodeTrafficDiagnosticStatusText = (status) =>
  NODE_TRAFFIC_DIAGNOSTIC_STATUS_TEXT[String(status || '').trim()] || '状态未知'

export const getNodeTrafficDiagnosticStatusType = (status) =>
  NODE_TRAFFIC_DIAGNOSTIC_STATUS_TYPES[String(status || '').trim()] || 'info'

export const hasNodeTrafficDiagnosticConfigMismatch = (diagnostic) => {
  const configured = String(diagnostic?.traffic?.configured_config_path || '').trim()
  const resolved = String(diagnostic?.traffic?.resolved_config_path || '').trim()
  return Boolean(configured && resolved && configured !== resolved)
}
