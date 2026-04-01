const TRAFFIC_UNITS = ['B', 'KB', 'MB', 'GB', 'TB']
const VALID_PORTAL_PERIODS = new Set(['day', 'week', 'month', 'year'])

export function formatTrafficBytes(bytes, fractionDigits = 2) {
  const normalizedBytes = Number(bytes)
  if (!Number.isFinite(normalizedBytes) || normalizedBytes <= 0) {
    return '0 B'
  }

  let unitIndex = 0
  let size = normalizedBytes
  while (size >= 1024 && unitIndex < TRAFFIC_UNITS.length - 1) {
    size /= 1024
    unitIndex += 1
  }

  return `${size.toFixed(fractionDigits)} ${TRAFFIC_UNITS[unitIndex]}`
}

export function isUnlimitedTrafficLimit(limit) {
  if (limit === null || limit === undefined || limit === '') {
    return false
  }

  const normalizedLimit = Number(limit)
  return Number.isFinite(normalizedLimit) && normalizedLimit <= 0
}

export function formatTrafficLimit(limit, fractionDigits = 2, unlimitedLabel = '不限流量') {
  if (isUnlimitedTrafficLimit(limit)) {
    return unlimitedLabel
  }

  return formatTrafficBytes(limit, fractionDigits)
}

export function formatRemainingTraffic(limit, used, fractionDigits = 2, unlimitedLabel = '不限流量') {
  if (isUnlimitedTrafficLimit(limit)) {
    return unlimitedLabel
  }

  return formatTrafficBytes(Math.max(0, Number(limit) - Number(used)), fractionDigits)
}

export function buildPortalStatsParams(presetPeriod = 'week', customRange = null) {
  if (
    Array.isArray(customRange) &&
    customRange.length === 2 &&
    customRange[0] &&
    customRange[1]
  ) {
    return {
      period: 'custom',
      start_date: customRange[0],
      end_date: customRange[1]
    }
  }

  return {
    period: VALID_PORTAL_PERIODS.has(presetPeriod) ? presetPeriod : 'week'
  }
}
