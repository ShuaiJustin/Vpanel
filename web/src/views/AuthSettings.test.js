import { describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/admin/system-settings/auth/oauth', params: { section: 'oauth' } }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() })
}))

vi.mock('element-plus', () => ({
  ElMessage: {
    error: vi.fn(),
    success: vi.fn()
  }
}))

vi.mock('@/api', () => ({
  settingsApi: {
    getAll: vi.fn(() => Promise.resolve({ auth: { oauth: { enabled: true, providers: {} } } })),
    update: vi.fn()
  }
}))

vi.mock('@/composables/useViewport', () => ({
  useViewport: () => ({ isMobile: { value: false } })
}))

import AuthSettings from './AuthSettings.vue'

function mountAuthSettings() {
  return shallowMount(AuthSettings, {
    global: {
      stubs: {
        'el-button': { template: '<button @click="$emit(\'click\')"><slot /></button>' },
        'el-tabs': { template: '<div><slot /></div>' },
        'el-tab-pane': { template: '<div><slot /></div>' },
        'el-form': { template: '<form><slot /></form>' },
        'el-form-item': { template: '<label><slot /></label>' },
        'el-switch': true,
        'el-select': true,
        'el-option': true,
        'el-input': true,
        'el-checkbox': true
      }
    }
  })
}

describe('AuthSettings view', () => {
  it('switches OAuth provider panel when provider nav is clicked', async () => {
    const wrapper = mountAuthSettings()
    await flushPromises()

    await wrapper.findAll('.provider-nav-item')[5].trigger('click')

    expect(wrapper.find('.provider-head h3').text()).toBe('微信')
    expect(wrapper.findAll('.provider-nav-item')[5].classes()).toContain('active')
  })
})
