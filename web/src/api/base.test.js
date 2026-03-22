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

import { formatApiError } from './base'

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
})
