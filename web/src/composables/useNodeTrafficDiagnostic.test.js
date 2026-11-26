import { describe, expect, it } from 'vitest'
import {
  getNodeTrafficDiagnosticStatusText,
  getNodeTrafficDiagnosticStatusType,
  hasNodeTrafficDiagnosticConfigMismatch,
  normalizeNodeTrafficDiagnostic
} from './useNodeTrafficDiagnostic'

describe('useNodeTrafficDiagnostic', () => {
  it('maps diagnostic status labels and types', () => {
    expect(getNodeTrafficDiagnosticStatusText('healthy_collecting')).toBe('采集正常（采集中）')
    expect(getNodeTrafficDiagnosticStatusType('collector_error')).toBe('danger')
    expect(getNodeTrafficDiagnosticStatusText('agent_unreachable')).toBe('Agent 不可达')
  })

  it('normalizes nested payloads', () => {
    const diagnostic = normalizeNodeTrafficDiagnostic({
      data: {
        node_id: 3,
        diagnostic_status: 'healthy_idle',
        message: 'idle',
        traffic: {
          status: 'healthy_idle',
          configured_config_path: '/etc/xray/config.json',
          resolved_config_path: '/usr/local/etc/xray/config.json',
          candidate_config_paths: ['/etc/xray/config.json', '/usr/local/etc/xray/config.json'],
          api_port: 62001,
          xray_running: true,
          last_record_count: 0
        }
      }
    })

    expect(diagnostic.node_id).toBe(3)
    expect(diagnostic.traffic.api_port).toBe(62001)
    expect(diagnostic.traffic.candidate_config_paths).toEqual([
      '/etc/xray/config.json',
      '/usr/local/etc/xray/config.json'
    ])
  })

  it('detects config path mismatch for historical manual nodes', () => {
    const mismatch = hasNodeTrafficDiagnosticConfigMismatch({
      traffic: {
        configured_config_path: '/etc/xray/config.json',
        resolved_config_path: '/usr/local/etc/xray/config.json'
      }
    })
    const noMismatch = hasNodeTrafficDiagnosticConfigMismatch({
      traffic: {
        configured_config_path: '/usr/local/etc/xray/config.json',
        resolved_config_path: '/usr/local/etc/xray/config.json'
      }
    })

    expect(mismatch).toBe(true)
    expect(noMismatch).toBe(false)
  })
})
