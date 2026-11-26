/**
 * 错误处理工具属性测试
 * Property 30: Frontend Error Code Mapping
 * Validates: Requirements 13.7
 * 
 * 测试：对于任何 API 错误响应，错误码应该被映射到本地化的用户友好消息
 */

import { describe, it, expect } from 'vitest'
import * as fc from 'fast-check'
import { 
  getErrorMessage, 
  generateErrorId, 
  getErrorSeverity,
  formatValidationErrors 
} from './errorHandler'

// 已知的错误码列表
const KNOWN_ERROR_CODES = [
  'VALIDATION_ERROR',
  'UNAUTHORIZED',
  'FORBIDDEN',
  'NOT_FOUND',
  'CONFLICT',
  'RATE_LIMIT_EXCEEDED',
  'INTERNAL_ERROR',
  'DATABASE_ERROR',
  'CACHE_ERROR',
  'XRAY_ERROR',
  'NETWORK_ERROR',
  'TIMEOUT_ERROR',
  'UNKNOWN_ERROR',
  'INVALID_CREDENTIALS',
  'ACCOUNT_DISABLED',
  'ACCOUNT_EXPIRED',
  'TOKEN_EXPIRED',
  'TOKEN_INVALID',
  'PASSWORD_WEAK',
  'USER_NOT_FOUND',
  'USERNAME_EXISTS',
  'EMAIL_EXISTS',
  'EMAIL_INVALID',
  'TRAFFIC_EXCEEDED',
  'PROXY_NOT_FOUND',
  'PORT_CONFLICT',
  'PROTOCOL_INVALID',
  'CONFIG_INVALID',
  'ROLE_NOT_FOUND',
  'ROLE_SYSTEM_PROTECTED',
  'PERMISSION_INVALID',
  'XRAY_NOT_RUNNING',
  'XRAY_CONFIG_INVALID',
  'XRAY_RESTART_FAILED'
]

describe('Error Handler - Property Tests', () => {
  /**
   * Feature: project-optimization, Property 30: Frontend Error Code Mapping
   * Validates: Requirements 13.7
   * 
   * 属性：对于任何已知的错误码，getErrorMessage 应该返回非空的本地化消息
   */
  it('Property 30: All known error codes should map to non-empty localized messages', () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...KNOWN_ERROR_CODES),
        (errorCode) => {
          const message = getErrorMessage(errorCode)
          
          // 消息应该是非空字符串
          expect(typeof message).toBe('string')
          expect(message.length).toBeGreaterThan(0)
          
          // 消息不应该是错误码本身（应该是人类可读的）
          expect(message).not.toBe(errorCode)
          
          // 消息应该包含中文字符（本地化）
          const hasChinese = /[\u4e00-\u9fa5]/.test(message)
          expect(hasChinese).toBe(true)
          
          return true
        }
      ),
      { numRuns: 100 }
    )
  })

  /**
   * 属性：对于任何未知的错误码，应该返回默认消息或提供的默认消息
   */
  it('Unknown error codes should return default message', () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 50 }).filter(s => 
          !KNOWN_ERROR_CODES.includes(s) && 
          !['constructor', 'prototype', '__proto__', 'toString', 'valueOf'].includes(s)
        ),
        fc.string({ minLength: 1, maxLength: 100 }),
        (unknownCode, defaultMessage) => {
          const message = getErrorMessage(unknownCode, defaultMessage)
          
          // 应该返回默认消息或 UNKNOWN_ERROR 的消息
          expect(typeof message).toBe('string')
          expect(message.length).toBeGreaterThan(0)
          
          return true
        }
      ),
      { numRuns: 100 }
    )
  })

  /**
   * 属性：生成的错误 ID 应该是唯一的且格式正确
   */
  it('Generated error IDs should be unique and properly formatted', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 100 }),
        (count) => {
          const ids = new Set()
          
          for (let i = 0; i < count; i++) {
            const id = generateErrorId()
            
            // ID 应该以 ERR- 开头
            expect(id.startsWith('ERR-')).toBe(true)
            
            // ID 应该是大写的
            expect(id).toBe(id.toUpperCase())
            
            // ID 应该包含时间戳和随机部分
            const parts = id.split('-')
            expect(parts.length).toBe(3)
            
            // ID 应该是唯一的
            expect(ids.has(id)).toBe(false)
            ids.add(id)
          }
          
          return true
        }
      ),
      { numRuns: 50 }
    )
  })

  /**
   * 属性：错误严重程度应该是有效的值
   */
  it('Error severity should be valid for all error codes', () => {
    const validSeverities = ['warning', 'error', 'info']
    
    fc.assert(
      fc.property(
        fc.constantFrom(...KNOWN_ERROR_CODES),
        (errorCode) => {
          const severity = getErrorSeverity(errorCode)
          
          // 严重程度应该是有效值之一
          expect(validSeverities).toContain(severity)
          
          return true
        }
      ),
      { numRuns: 100 }
    )
  })

  /**
   * 属性：验证错误格式化应该正确处理字段错误
   */
  it('Validation errors should be properly formatted', () => {
    fc.assert(
      fc.property(
        fc.dictionary(
          fc.string({ minLength: 1, maxLength: 20 }).filter(s => /^[a-zA-Z_]+$/.test(s)),
          fc.string({ minLength: 1, maxLength: 100 })
        ),
        (fields) => {
          if (Object.keys(fields).length === 0) {
            return true // 跳过空对象
          }
          
          const details = { fields }
          const formatted = formatValidationErrors(details)
          
          // 格式化结果应该是字符串
          expect(typeof formatted).toBe('string')
          
          // 格式化结果应该包含所有字段名
          for (const fieldName of Object.keys(fields)) {
            expect(formatted).toContain(fieldName)
          }
          
          return true
        }
      ),
      { numRuns: 100 }
    )
  })

  /**
   * 属性：空或无效的验证详情应该返回默认消息
   */
  it('Empty or invalid validation details should return default message', () => {
    fc.assert(
      fc.property(
        fc.oneof(
          fc.constant(null),
          fc.constant(undefined),
          fc.constant({}),
          fc.constant({ fields: null }),
          fc.constant({ fields: {} })
        ),
        (details) => {
          const formatted = formatValidationErrors(details)
          
          // 应该返回默认消息
          expect(typeof formatted).toBe('string')
          expect(formatted.length).toBeGreaterThan(0)
          
          return true
        }
      ),
      { numRuns: 20 }
    )
  })
})

describe('Error Handler - Unit Tests', () => {
  it('should return correct message for VALIDATION_ERROR', () => {
    const message = getErrorMessage('VALIDATION_ERROR')
    expect(message).toBe('输入数据验证失败，请检查填写的内容')
  })

  it('should return correct message for UNAUTHORIZED', () => {
    const message = getErrorMessage('UNAUTHORIZED')
    expect(message).toBe('登录已过期，请重新登录')
  })

  it('should return correct message for NETWORK_ERROR', () => {
    const message = getErrorMessage('NETWORK_ERROR')
    expect(message).toBe('网络连接失败，请检查网络')
  })

  it('should return default message for unknown code', () => {
    const message = getErrorMessage('SOME_UNKNOWN_CODE')
    expect(message).toBe('发生未知错误')
  })

  it('should return provided default message for unknown code', () => {
    const message = getErrorMessage('SOME_UNKNOWN_CODE', '自定义错误消息')
    expect(message).toBe('自定义错误消息')
  })

  it('should generate unique error IDs', () => {
    const id1 = generateErrorId()
    const id2 = generateErrorId()
    expect(id1).not.toBe(id2)
  })

  it('should format validation errors correctly', () => {
    const details = {
      fields: {
        username: '用户名不能为空',
        email: '邮箱格式不正确'
      }
    }
    const formatted = formatValidationErrors(details)
    expect(formatted).toContain('username')
    expect(formatted).toContain('email')
    expect(formatted).toContain('用户名不能为空')
    expect(formatted).toContain('邮箱格式不正确')
  })
})
