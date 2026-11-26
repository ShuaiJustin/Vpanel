import { describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const route = vi.hoisted(() => ({ path: '/admin/system-settings/auth/oauth', params: { section: 'oauth' } }))

vi.mock('vue-router', () => ({
  useRoute: () => route,
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
import { settingsApi } from '@/api'

function mountAuthSettings() {
  return shallowMount(AuthSettings, {
    global: {
      stubs: {
        AdminStickyChrome: { template: '<header><slot /></header>' },
        'el-button': { template: '<button @click="$emit(\'click\')"><slot /></button>' },
        'el-tabs': { template: '<div><slot /></div>' },
        'el-tab-pane': { template: '<div><slot /></div>' },
        'el-form': { template: '<form><slot /></form>' },
        'el-form-item': { template: '<label><slot /></label>' },
        'el-switch': {
          props: ['modelValue'],
          template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)">'
        },
        'el-select': true,
        'el-option': true,
        'el-input': true,
        'el-checkbox': true,
        'el-tag': { template: '<span><slot /></span>' }
      }
    }
  })
}

describe('AuthSettings view', () => {
  it('switches OAuth provider panel when provider nav is clicked', async () => {
    const wrapper = mountAuthSettings()
    await flushPromises()

    await wrapper.findAll('.provider-nav-item')[4].trigger('click')

    expect(wrapper.find('.provider-head h3').text()).toBe('企业微信')
    expect(wrapper.findAll('.provider-nav-item')[4].classes()).toContain('active')
  })

  it('saves email registration together with auth settings without dropping other tabs', async () => {
    route.path = '/admin/system-settings/auth/registration'
    route.params.section = 'registration'
    settingsApi.update.mockResolvedValueOnce({
      data: {
        allow_registration: true,
        auth: { oauth: { enabled: true, providers: {} } }
      }
    })
    const wrapper = mountAuthSettings()
    await flushPromises()
    await wrapper.get('input[aria-label="允许邮箱注册"]').setValue(true)
    await wrapper.get('.save-button').trigger('click')
    await flushPromises()
    expect(settingsApi.update).toHaveBeenCalledWith(
      expect.objectContaining({
        allow_registration: true,
        auth: expect.objectContaining({
          oauth: expect.objectContaining({ enabled: true })
        })
      })
    )
    expect(wrapper.get('input[aria-label="允许邮箱注册"]').element.checked).toBe(true)
    wrapper.unmount()
  })
})
