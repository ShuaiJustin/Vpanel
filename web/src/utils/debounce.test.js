/**
 * 防抖工具属性测试
 * Property 32: Request Debouncing
 * Validates: Requirements 8.4
 * 
 * 测试：对于任何搜索输入触发的 API 请求，在防抖窗口内的多次快速输入应该只产生一次 API 请求
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import * as fc from 'fast-check'
import { debounce, throttle, createDebouncedSearch } from './debounce'

describe('Debounce - Property Tests', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  /**
   * Feature: project-optimization, Property 32: Request Debouncing
   * Validates: Requirements 8.4
   * 
   * 属性：在防抖窗口内的多次调用应该只执行一次
   */
  it('Property 32: Multiple rapid calls within debounce window should result in single execution', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 2, max: 20 }), // 调用次数
        fc.integer({ min: 50, max: 500 }), // 防抖延迟
        (callCount, delay) => {
          const fn = vi.fn()
          const debouncedFn = debounce(fn, delay)
          
          // 在防抖窗口内快速调用多次
          for (let i = 0; i < callCount; i++) {
            debouncedFn(i)
            vi.advanceTimersByTime(delay / (callCount + 1)) // 确保在窗口内
          }
          
          // 等待防抖完成
          vi.advanceTimersByTime(delay)
          
          // 应该只执行一次
          expect(fn).toHaveBeenCalledTimes(1)
          
          // 应该使用最后一次调用的参数
          expect(fn).toHaveBeenCalledWith(callCount - 1)
          
          return true
        }
      ),
      { numRuns: 100 }
    )
  })

  /**
   * 属性：防抖窗口过后的调用应该触发新的执行
   */
  it('Calls after debounce window should trigger new execution', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 2, max: 10 }), // 批次数
        fc.integer({ min: 50, max: 200 }), // 防抖延迟
        (batchCount, delay) => {
          const fn = vi.fn()
          const debouncedFn = debounce(fn, delay)
          
          // 多批次调用，每批次之间等待足够时间
          for (let batch = 0; batch < batchCount; batch++) {
            debouncedFn(batch)
            vi.advanceTimersByTime(delay + 10) // 等待防抖完成
          }
          
          // 每批次应该执行一次
          expect(fn).toHaveBeenCalledTimes(batchCount)
          
          return true
        }
      ),
      { numRuns: 50 }
    )
  })

  /**
   * 属性：取消防抖应该阻止执行
   */
  it('Cancelling debounce should prevent execution', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10 }), // 调用次数
        fc.integer({ min: 50, max: 200 }), // 防抖延迟
        (callCount, delay) => {
          const fn = vi.fn()
          const debouncedFn = debounce(fn, delay)
          
          // 调用多次
          for (let i = 0; i < callCount; i++) {
            debouncedFn(i)
          }
          
          // 取消
          debouncedFn.cancel()
          
          // 等待防抖时间
          vi.advanceTimersByTime(delay * 2)
          
          // 不应该执行
          expect(fn).not.toHaveBeenCalled()
          
          return true
        }
      ),
      { numRuns: 50 }
    )
  })

  /**
   * 属性：flush 应该立即执行
   */
  it('Flush should execute immediately', () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 20 }),
        fc.integer({ min: 100, max: 500 }),
        (value, delay) => {
          const fn = vi.fn()
          const debouncedFn = debounce(fn, delay)
          
          debouncedFn(value)
          
          // 不等待，直接 flush
          debouncedFn.flush()
          
          // 应该立即执行
          expect(fn).toHaveBeenCalledTimes(1)
          expect(fn).toHaveBeenCalledWith(value)
          
          return true
        }
      ),
      { numRuns: 50 }
    )
  })

  /**
   * 属性：pending 应该正确反映状态
   */
  it('Pending should correctly reflect debounce state', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 100, max: 500 }),
        (delay) => {
          const fn = vi.fn()
          const debouncedFn = debounce(fn, delay)
          
          // 初始状态不应该 pending
          expect(debouncedFn.pending()).toBe(false)
          
          // 调用后应该 pending
          debouncedFn()
          expect(debouncedFn.pending()).toBe(true)
          
          // 等待完成后不应该 pending
          vi.advanceTimersByTime(delay)
          expect(debouncedFn.pending()).toBe(false)
          
          return true
        }
      ),
      { numRuns: 50 }
    )
  })
})

describe('Throttle - Property Tests', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  /**
   * 属性：节流应该限制执行频率
   */
  it('Throttle should limit execution frequency', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 10, max: 50 }), // 调用次数
        fc.integer({ min: 100, max: 300 }), // 节流间隔
        (callCount, wait) => {
          const fn = vi.fn()
          const throttledFn = throttle(fn, wait)
          
          // 快速调用多次
          for (let i = 0; i < callCount; i++) {
            throttledFn(i)
            vi.advanceTimersByTime(10) // 每次调用间隔很短
          }
          
          // 等待所有节流完成
          vi.advanceTimersByTime(wait)
          
          // 执行次数应该受限
          const totalTime = callCount * 10 + wait
          const expectedMaxCalls = Math.ceil(totalTime / wait) + 1
          
          expect(fn.mock.calls.length).toBeLessThanOrEqual(expectedMaxCalls)
          expect(fn.mock.calls.length).toBeGreaterThan(0)
          
          return true
        }
      ),
      { numRuns: 50 }
    )
  })
})

describe('Debounce - Unit Tests', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('should debounce function calls', () => {
    const fn = vi.fn()
    const debouncedFn = debounce(fn, 100)
    
    debouncedFn('a')
    debouncedFn('b')
    debouncedFn('c')
    
    expect(fn).not.toHaveBeenCalled()
    
    vi.advanceTimersByTime(100)
    
    expect(fn).toHaveBeenCalledTimes(1)
    expect(fn).toHaveBeenCalledWith('c')
  })

  it('should support leading option', () => {
    const fn = vi.fn()
    const debouncedFn = debounce(fn, 100, { leading: true, trailing: false })
    
    debouncedFn('a')
    expect(fn).toHaveBeenCalledTimes(1)
    expect(fn).toHaveBeenCalledWith('a')
    
    debouncedFn('b')
    debouncedFn('c')
    
    vi.advanceTimersByTime(100)
    
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('should cancel pending execution', () => {
    const fn = vi.fn()
    const debouncedFn = debounce(fn, 100)
    
    debouncedFn('test')
    debouncedFn.cancel()
    
    vi.advanceTimersByTime(200)
    
    expect(fn).not.toHaveBeenCalled()
  })

  it('should flush pending execution', () => {
    const fn = vi.fn()
    const debouncedFn = debounce(fn, 100)
    
    debouncedFn('test')
    debouncedFn.flush()
    
    expect(fn).toHaveBeenCalledTimes(1)
    expect(fn).toHaveBeenCalledWith('test')
  })
})

describe('createDebouncedSearch - Unit Tests', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('should create debounced search function', () => {
    const searchFn = vi.fn()
    const { search } = createDebouncedSearch(searchFn, 300)
    
    search('test')
    search('testing')
    search('testing123')
    
    expect(searchFn).not.toHaveBeenCalled()
    
    vi.advanceTimersByTime(300)
    
    expect(searchFn).toHaveBeenCalledTimes(1)
    expect(searchFn).toHaveBeenCalledWith('testing123')
  })

  it('should allow cancellation', () => {
    const searchFn = vi.fn()
    const { search, cancel } = createDebouncedSearch(searchFn, 300)
    
    search('test')
    cancel()
    
    vi.advanceTimersByTime(300)
    
    expect(searchFn).not.toHaveBeenCalled()
  })
})
