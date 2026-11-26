import { describe, expect, it } from 'vitest'

import {
  buildPortalStatsParams,
  formatRemainingTraffic,
  formatTrafficBytes,
  formatTrafficLimit,
  isUnlimitedTrafficLimit,
} from './traffic'

describe('traffic utils', () => {
  it('formats traffic bytes with the requested precision', () => {
    expect(formatTrafficBytes(0)).toBe('0 B')
    expect(formatTrafficBytes(1536)).toBe('1.50 KB')
    expect(formatTrafficBytes(1536, 0)).toBe('2 KB')
  })

  it('treats zero or negative limits as unlimited', () => {
    expect(isUnlimitedTrafficLimit(0)).toBe(true)
    expect(isUnlimitedTrafficLimit(-1)).toBe(true)
    expect(isUnlimitedTrafficLimit(undefined)).toBe(false)
    expect(isUnlimitedTrafficLimit(1024)).toBe(false)
    expect(formatTrafficLimit(0)).toBe('不限流量')
    expect(formatRemainingTraffic(0, 2048)).toBe('不限流量')
  })

  it('builds custom stats params only for complete custom ranges', () => {
    expect(buildPortalStatsParams('month')).toEqual({ period: 'month' })
    expect(buildPortalStatsParams('custom', null)).toEqual({ period: 'week' })
    expect(buildPortalStatsParams('invalid', ['2026-03-01', '2026-03-31'])).toEqual({
      period: 'custom',
      start_date: '2026-03-01',
      end_date: '2026-03-31',
    })
  })
})
