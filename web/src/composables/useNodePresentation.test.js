import { describe, expect, it } from 'vitest'

import {
  formatCoreVersionCompact,
  formatUsersLimitDisplay,
  getNodeLatencyClass,
  getNodeRegionFlag,
  getNodeRegionLabel,
  getNodeStatusText,
  getNodeStatusType,
  getNodeSyncStatusText,
  getProtocolDisplayName,
  parseNodeTags
} from './useNodePresentation'

describe('useNodePresentation helpers', () => {
  it('formats user limits with unlimited fallback', () => {
    expect(formatUsersLimitDisplay(12, 48)).toBe('12 / 48')
    expect(formatUsersLimitDisplay(7, 0)).toBe('7 / ∞')
  })

  it('maps node status labels and types', () => {
    expect(getNodeStatusType('online')).toBe('success')
    expect(getNodeStatusText('offline')).toBe('离线')
    expect(getNodeSyncStatusText('pending')).toBe('待同步')
  })

  it('normalizes latency classes', () => {
    expect(getNodeLatencyClass(0)).toBe('')
    expect(getNodeLatencyClass(88)).toBe('latency-good')
    expect(getNodeLatencyClass(180)).toBe('latency-medium')
    expect(getNodeLatencyClass(360)).toBe('latency-bad')
  })

  it('extracts compact core version text', () => {
    expect(formatCoreVersionCompact('Xray 1.8.24\nBuild')).toBe('Xray 1.8.24')
    expect(formatCoreVersionCompact('custom-version')).toBe('custom-version')
  })

  it('parses tag arrays safely', () => {
    expect(parseNodeTags('["a","b"]')).toEqual(['a', 'b'])
    expect(parseNodeTags('invalid')).toEqual([])
  })
})


  it('maps region and protocol presentation helpers', () => {
    expect(getNodeRegionFlag('日本')).toBe('🇯🇵')
    expect(getNodeRegionLabel('jp')).toBe('日本')
    expect(getNodeRegionLabel('中国')).toBe('中国')
    expect(getProtocolDisplayName('vmess')).toBe('VMess')
    expect(getProtocolDisplayName('shadowsocks')).toBe('Shadowsocks')
  })
