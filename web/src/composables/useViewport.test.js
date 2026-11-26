import { describe, expect, it } from 'vitest'

import { VIEWPORT_BREAKPOINTS, calculateViewportScale } from './useViewport'

describe('calculateViewportScale', () => {
  it('clamps small viewports to the minimum scale', () => {
    expect(calculateViewportScale(320)).toBe(VIEWPORT_BREAKPOINTS.minScale)
  })

  it('keeps the design width at scale 1', () => {
    expect(calculateViewportScale(VIEWPORT_BREAKPOINTS.designWidth)).toBe(1)
  })

  it('clamps very wide viewports to the maximum scale', () => {
    expect(calculateViewportScale(2560)).toBe(VIEWPORT_BREAKPOINTS.maxScale)
  })
})
