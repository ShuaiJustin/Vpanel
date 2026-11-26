/**
 * 数据序列化测试
 * Property 33: Data Serialization Round-Trip
 * Validates: Requirements 9.5
 */

import { describe, it, expect } from 'vitest'
import * as fc from 'fast-check'
import { serialize, deserialize, isEqual, serializeParams, deserializeParams } from './serialization'

describe('Data Serialization Round-Trip - Property 33', () => {
  /**
   * Property 33: Data Serialization Round-Trip
   * For any valid data object, serializing then deserializing should produce an equivalent object
   * Validates: Requirements 9.5
   */
  
  describe('Basic Types Round-Trip', () => {
    it('should round-trip strings', () => {
      fc.assert(
        fc.property(
          fc.string(),
          (str) => {
            const serialized = serialize(str)
            const deserialized = deserialize(serialized)
            return deserialized === str
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip numbers', () => {
      fc.assert(
        fc.property(
          fc.double({ noNaN: true, noDefaultInfinity: true }),
          (num) => {
            const serialized = serialize(num)
            const deserialized = deserialize(serialized)
            return deserialized === num
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip integers', () => {
      fc.assert(
        fc.property(
          fc.integer(),
          (num) => {
            const serialized = serialize(num)
            const deserialized = deserialize(serialized)
            return deserialized === num
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip booleans', () => {
      fc.assert(
        fc.property(
          fc.boolean(),
          (bool) => {
            const serialized = serialize(bool)
            const deserialized = deserialize(serialized)
            return deserialized === bool
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip null', () => {
      const serialized = serialize(null)
      const deserialized = deserialize(serialized)
      expect(deserialized).toBe(null)
    })
  })
  
  describe('Array Round-Trip', () => {
    it('should round-trip arrays of primitives', () => {
      fc.assert(
        fc.property(
          fc.array(fc.oneof(
            fc.string(),
            fc.integer(),
            fc.boolean(),
            fc.constant(null)
          )),
          (arr) => {
            const serialized = serialize(arr)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, arr)
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip nested arrays', () => {
      fc.assert(
        fc.property(
          fc.array(fc.array(fc.integer())),
          (arr) => {
            const serialized = serialize(arr)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, arr)
          }
        ),
        { numRuns: 100 }
      )
    })
  })
  
  describe('Object Round-Trip', () => {
    it('should round-trip simple objects', () => {
      fc.assert(
        fc.property(
          fc.record({
            id: fc.integer(),
            name: fc.string(),
            active: fc.boolean()
          }),
          (obj) => {
            const serialized = serialize(obj)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, obj)
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip nested objects', () => {
      fc.assert(
        fc.property(
          fc.record({
            user: fc.record({
              id: fc.integer(),
              name: fc.string()
            }),
            settings: fc.record({
              theme: fc.string(),
              notifications: fc.boolean()
            })
          }),
          (obj) => {
            const serialized = serialize(obj)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, obj)
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip objects with arrays', () => {
      fc.assert(
        fc.property(
          fc.record({
            items: fc.array(fc.integer()),
            tags: fc.array(fc.string())
          }),
          (obj) => {
            const serialized = serialize(obj)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, obj)
          }
        ),
        { numRuns: 100 }
      )
    })
  })
  
  describe('Special Types Round-Trip', () => {
    it('should round-trip Date objects', () => {
      fc.assert(
        fc.property(
          fc.date({ noInvalidDate: true }), // Exclude invalid dates
          (date) => {
            const serialized = serialize(date)
            const deserialized = deserialize(serialized)
            return deserialized instanceof Date && 
                   deserialized.getTime() === date.getTime()
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip invalid Date objects', () => {
      const invalidDate = new Date(NaN)
      const serialized = serialize(invalidDate)
      const deserialized = deserialize(serialized)
      expect(deserialized instanceof Date).toBe(true)
      expect(isNaN(deserialized.getTime())).toBe(true)
    })
    
    it('should round-trip objects with Date fields', () => {
      fc.assert(
        fc.property(
          fc.record({
            createdAt: fc.date(),
            updatedAt: fc.date()
          }),
          (obj) => {
            const serialized = serialize(obj)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, obj)
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip NaN', () => {
      const serialized = serialize(NaN)
      const deserialized = deserialize(serialized)
      expect(Number.isNaN(deserialized)).toBe(true)
    })
    
    it('should round-trip Infinity', () => {
      const serialized = serialize(Infinity)
      const deserialized = deserialize(serialized)
      expect(deserialized).toBe(Infinity)
      
      const serializedNeg = serialize(-Infinity)
      const deserializedNeg = deserialize(serializedNeg)
      expect(deserializedNeg).toBe(-Infinity)
    })
  })
  
  describe('URL Params Round-Trip', () => {
    it('should round-trip simple params', () => {
      fc.assert(
        fc.property(
          fc.record({
            page: fc.integer({ min: 1, max: 100 }),
            limit: fc.integer({ min: 1, max: 100 }),
            search: fc.string().filter(s => !s.includes('&') && !s.includes('='))
          }),
          (params) => {
            const serialized = serializeParams(params)
            const deserialized = deserializeParams(serialized)
            // URL params are always strings
            return deserialized.page === String(params.page) &&
                   deserialized.limit === String(params.limit) &&
                   deserialized.search === params.search
          }
        ),
        { numRuns: 100 }
      )
    })
  })
  
  describe('Complex Data Structures', () => {
    it('should round-trip API response-like objects', () => {
      fc.assert(
        fc.property(
          fc.record({
            success: fc.boolean(),
            data: fc.record({
              id: fc.integer(),
              name: fc.string(),
              email: fc.emailAddress(),
              createdAt: fc.date()
            }),
            meta: fc.record({
              page: fc.integer({ min: 1, max: 100 }),
              total: fc.integer({ min: 0, max: 1000 })
            })
          }),
          (response) => {
            const serialized = serialize(response)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, response)
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip proxy configuration objects', () => {
      fc.assert(
        fc.property(
          fc.record({
            id: fc.integer(),
            name: fc.string(),
            protocol: fc.constantFrom('vmess', 'vless', 'trojan', 'shadowsocks'),
            port: fc.integer({ min: 1, max: 65535 }),
            enabled: fc.boolean(),
            settings: fc.record({
              network: fc.constantFrom('tcp', 'ws', 'grpc'),
              security: fc.constantFrom('none', 'tls', 'reality')
            })
          }),
          (proxy) => {
            const serialized = serialize(proxy)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, proxy)
          }
        ),
        { numRuns: 100 }
      )
    })
    
    it('should round-trip user objects', () => {
      fc.assert(
        fc.property(
          fc.record({
            id: fc.integer(),
            username: fc.string(),
            email: fc.emailAddress(),
            role: fc.constantFrom('admin', 'user'),
            status: fc.boolean(),
            trafficLimit: fc.integer({ min: 0 }),
            trafficUsed: fc.integer({ min: 0 }),
            expiresAt: fc.date()
          }),
          (user) => {
            const serialized = serialize(user)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, user)
          }
        ),
        { numRuns: 100 }
      )
    })
  })
  
  describe('Edge Cases', () => {
    it('should handle empty objects', () => {
      const serialized = serialize({})
      const deserialized = deserialize(serialized)
      expect(isEqual(deserialized, {})).toBe(true)
    })
    
    it('should handle empty arrays', () => {
      const serialized = serialize([])
      const deserialized = deserialize(serialized)
      expect(isEqual(deserialized, [])).toBe(true)
    })
    
    it('should handle deeply nested structures', () => {
      fc.assert(
        fc.property(
          fc.integer({ min: 1, max: 5 }),
          (depth) => {
            let obj = { value: 'leaf' }
            for (let i = 0; i < depth; i++) {
              obj = { nested: obj }
            }
            const serialized = serialize(obj)
            const deserialized = deserialize(serialized)
            return isEqual(deserialized, obj)
          }
        ),
        { numRuns: 50 }
      )
    })
  })
})
