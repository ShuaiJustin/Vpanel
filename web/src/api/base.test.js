import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('element-plus', () => ({
  ElMessage: {
    error: vi.fn(),
  },
}))

vi.mock('@/router', () => ({
  default: {
    replace: vi.fn(),
  },
}))

vi.mock('@/utils/requestManager', () => ({
  cancelManager: {
    getController: vi.fn(() => ({ signal: new AbortController().signal })),
    remove: vi.fn(),
    cancel: vi.fn(),
    cancelAll: vi.fn(),
  },
  deduplicator: {
    generateKey: vi.fn(() => 'dedupe-key'),
    execute: vi.fn((_, requestFn) => requestFn()),
  },
}))

import { formatApiError, generateErrorId, generateRequestId } from './base'

describe('request and error ids', () => {
  it('uses REQ prefix for normal request tracing ids', () => {
    const requestId = generateRequestId()

    expect(requestId).toMatch(/^REQ-[A-Z0-9]+-[A-Z0-9]+$/)
    expect(requestId.startsWith('ERR-')).toBe(false)
  })

  it('keeps ERR prefix for actual frontend error ids', () => {
    const errorId = generateErrorId()

    expect(errorId).toMatch(/^ERR-[A-Z0-9]+-[A-Z0-9]+$/)
  })
})

describe('formatApiError', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('prefers nested backend error codes and details', () => {
    const formatted = formatApiError({
      response: {
        status: 400,
        data: {
          error: {
            code: 'VALIDATION_ERROR',
            message: 'invalid payload',
            details: {
              fields: {
                username: 'required',
              },
            },
          },
          request_id: 'req-123',
        },
      },
    })

    expect(formatted.code).toBe('VALIDATION_ERROR')
    expect(formatted.message).toBe('invalid payload')
    expect(formatted.details).toEqual({
      fields: {
        username: 'required',
      },
    })
    expect(formatted.requestId).toBe('req-123')
    expect(formatted.status).toBe(400)
  })

  it('falls back to HTTP status mapping when backend code is absent', () => {
    const formatted = formatApiError({
      response: {
        status: 404,
        data: {
          message: 'missing',
        },
      },
    })

    expect(formatted.code).toBe('NOT_FOUND')
    expect(formatted.message).toBe('missing')
  })

  it('strips duplicated backend error code prefixes from message', () => {
    const formatted = formatApiError({
      response: {
        status: 403,
        data: {
          message: 'FORBIDDEN: 当前无有效订阅或试用',
        },
      },
    })

    expect(formatted.code).toBe('FORBIDDEN')
    expect(formatted.message).toBe('当前无有效订阅或试用')
  })

  it('normalizes generic payment creation messages to friendly chinese copy', () => {
    const formatted = formatApiError({
      response: {
        status: 400,
        data: {
          code: 'PAYMENT_ERROR',
          message: 'Failed to create payment',
        },
      },
    })

    expect(formatted.code).toBe('PAYMENT_ERROR')
    expect(formatted.message).toBe('创建支付失败，请稍后重试。')
  })
})
