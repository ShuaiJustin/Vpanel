/**
 * 全局错误处理器测试
 * Property 31: Frontend Error ID
 * Validates: Requirements 13.10
 */

import { describe, it, expect } from 'vitest'
import * as fc from 'fast-check'
import { generateErrorId } from './errorHandler'

describe('Frontend Error ID - Property 31', () => {
  /**
   * Property 31: Frontend Error ID
   * For any error, the generated error ID should be unique and follow the expected format
   * Validates: Requirements 13.10
   */
  
  describe('Error ID Generation', () => {
    it('should generate unique error IDs', () => {
      fc.assert(
        fc.property(
          fc.integer({ min: 10, max: 100 }),
          (count) => {
            const ids = new Set()
            for (let i = 0; i < count; i++) {
              ids.add(generateErrorId())
            }
            // All generated IDs should be unique
            return ids.size === count
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should generate error IDs with correct format', () => {
      fc.assert(
        fc.property(
          fc.constant(null), // No input needed
          () => {
            const errorId = generateErrorId()
            // Format: ERR-{timestamp}-{random}
            const pattern = /^ERR-[A-Z0-9]+-[A-Z0-9]+$/
            return pattern.test(errorId)
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should generate error IDs starting with ERR-', () => {
      fc.assert(
        fc.property(
          fc.constant(null),
          () => {
            const errorId = generateErrorId()
            return errorId.startsWith('ERR-')
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should generate error IDs with uppercase characters', () => {
      fc.assert(
        fc.property(
          fc.constant(null),
          () => {
            const errorId = generateErrorId()
            return errorId === errorId.toUpperCase()
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should generate error IDs with reasonable length', () => {
      fc.assert(
        fc.property(
          fc.constant(null),
          () => {
            const errorId = generateErrorId()
            // ERR- (4) + timestamp (6-8) + - (1) + random (6) = 17-19 chars
            return errorId.length >= 15 && errorId.length <= 25
          }
        ),
        { numRuns: 100 }
      )
    })
  })
  
  describe('Error ID Uniqueness Over Time', () => {
    it('should generate different IDs even when called rapidly', () => {
      const ids = []
      for (let i = 0; i < 1000; i++) {
        ids.push(generateErrorId())
      }
      const uniqueIds = new Set(ids)
      expect(uniqueIds.size).toBe(1000)
    })
    
    it('should not generate duplicate IDs across multiple batches', () => {
      fc.assert(
        fc.property(
          fc.integer({ min: 2, max: 5 }),
          fc.integer({ min: 10, max: 50 }),
          (batches, batchSize) => {
            const allIds = new Set()
            for (let b = 0; b < batches; b++) {
              for (let i = 0; i < batchSize; i++) {
                allIds.add(generateErrorId())
              }
            }
            return allIds.size === batches * batchSize
          }
        ),
        { numRuns: 50 }
      )
    })
  })
  
  describe('Error ID Format Consistency', () => {
    it('should always have exactly 3 parts separated by hyphens', () => {
      fc.assert(
        fc.property(
          fc.constant(null),
          () => {
            const errorId = generateErrorId()
            const parts = errorId.split('-')
            // ERR-timestamp-random = 3 parts
            return parts.length === 3 && parts[0] === 'ERR'
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should have alphanumeric timestamp and random parts', () => {
      fc.assert(
        fc.property(
          fc.constant(null),
          () => {
            const errorId = generateErrorId()
            const parts = errorId.split('-')
            const alphanumeric = /^[A-Z0-9]+$/
            return alphanumeric.test(parts[1]) && alphanumeric.test(parts[2])
          }
        ),
        { numRuns: 100 }
      )
    })
  })
})
