const ERROR_PREFIX_PATTERN = /^(VALIDATION_ERROR|UNAUTHORIZED|FORBIDDEN|NOT_FOUND|CONFLICT|RATE_LIMIT_EXCEEDED|INTERNAL_ERROR|DATABASE_ERROR|CACHE_ERROR|XRAY_ERROR|NETWORK_ERROR|TIMEOUT_ERROR|UNKNOWN_ERROR)\s*:\s*/i

const NO_ENTITLEMENT_PATTERNS = [
  '当前无有效订阅或试用',
  '无有效订阅或试用',
  'No active subscription or trial',
  'No active subscription'
]

const KNOWN_ERROR_MESSAGE_MAPPINGS = [
  [/^Failed to create payment$/i, '创建支付失败，请稍后重试。'],
  [/^Selected payment method is not available$/i, '当前支付方式暂不可用，请选择其他支付方式。'],
  [/^payment failed:\s*insufficient balance$/i, '余额不足，请先充值后再支付。'],
  [/^insufficient balance$/i, '余额不足，请先充值后再支付。'],
  [/^order not found$/i, '订单不存在或已失效，请返回订单页重新创建。'],
  [/^order is not pending$/i, '当前订单状态不支持继续支付，请返回订单页刷新后重试。'],
  [/^Invalid request body$/i, '请求参数不正确，请刷新页面后重试。'],
  [/^Order number is required$/i, '订单号不能为空，请返回订单页后重试。']
]

export function normalizeBackendErrorMessage(message) {
  const raw = String(message || '').trim()
  if (!raw) return ''

  const normalized = raw.replace(ERROR_PREFIX_PATTERN, '').trim()
  const knownMessage = KNOWN_ERROR_MESSAGE_MAPPINGS.find(([pattern]) => pattern.test(normalized))

  return knownMessage?.[1] || normalized
}

export function extractErrorMessage(error) {
  const candidates = [
    error?.message,
    error?.error?.message,
    error?.response?.data?.message,
    error?.response?.data?.error?.message,
    typeof error?.response?.data?.error === 'string' ? error.response.data.error : '',
    typeof error === 'string' ? error : ''
  ]

  for (const candidate of candidates) {
    const normalized = normalizeBackendErrorMessage(candidate)
    if (normalized) {
      return normalized
    }
  }

  return ''
}

export function getErrorStatus(error) {
  return Number(error?.status || error?.response?.status || 0)
}

export function getErrorCode(error) {
  return String(
    error?.code ||
    error?.response?.data?.code ||
    error?.response?.data?.error?.code ||
    ''
  ).trim()
}

export function getDisplayErrorMessage(error, fallbackMessage = '') {
  return extractErrorMessage(error) || fallbackMessage
}

export function toNormalizedError(error, fallbackMessage = '') {
  const normalized = error && typeof error === 'object' ? error : {}
  const result = {
    ...normalized,
    message: getDisplayErrorMessage(error, fallbackMessage),
    status: getErrorStatus(error) || normalized.status || 0,
    code: getErrorCode(error) || normalized.code || ''
  }

  if (normalized.response) {
    result.response = normalized.response
  }

  return result
}

export function isNoEntitlementError(error) {
  const status = Number(error?.status || error?.response?.status || 0)
  const message = extractErrorMessage(error)

  if (!message) {
    return false
  }

  return status === 403 && NO_ENTITLEMENT_PATTERNS.some(pattern => message.includes(pattern))
}

export function getNoEntitlementMessage(scene = 'default') {
  const messages = {
    default: '当前无有效订阅或试用，请先购买或续费套餐。',
    nodes: '当前无有效订阅或试用，购买或续费后即可查看节点列表。',
    subscription: '当前暂无可用订阅链接，请先购买或续费套餐。',
    download: '当前暂无可用订阅链接，请先购买或续费套餐后再导入客户端。'
  }

  return messages[scene] || messages.default
}
