import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const viewportWidth = ref(1200)

vi.mock('@/composables/useViewport', () => ({
  VIEWPORT_BREAKPOINTS: { portalLayout: 1080 },
  useViewport: () => ({ viewportWidth })
}))

vi.mock('./UserLayout.vue', () => ({
  default: { name: 'DesktopPortalLayout', template: '<div data-layout="desktop" />' }
}))

vi.mock('./MobileLayout.vue', () => ({
  default: { name: 'MobilePortalLayout', template: '<div data-layout="mobile" />' }
}))

import ResponsiveUserLayout from './ResponsiveUserLayout.vue'

describe('ResponsiveUserLayout', () => {
  beforeEach(() => {
    viewportWidth.value = 1200
  })

  it('keeps the active route shell mounted while the viewport changes', async () => {
    const wrapper = mount(ResponsiveUserLayout)
    expect(wrapper.get('[data-layout="desktop"]').exists()).toBe(true)

    viewportWidth.value = 900
    await nextTick()

    expect(wrapper.get('[data-layout="desktop"]').exists()).toBe(true)
    expect(wrapper.find('[data-layout="mobile"]').exists()).toBe(false)
  })

  it('selects the mobile shell on an initial mobile viewport', () => {
    viewportWidth.value = 390
    const wrapper = mount(ResponsiveUserLayout)

    expect(wrapper.get('[data-layout="mobile"]').exists()).toBe(true)
  })
})
