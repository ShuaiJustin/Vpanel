import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const portalAuthMocks = vi.hoisted(() => ({
  getProfile: vi.fn(),
  login: vi.fn(),
  verify2FALogin: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
  updateProfile: vi.fn(),
  uploadAvatar: vi.fn(),
  bindTelegram: vi.fn(),
  unbindTelegram: vi.fn(),
  resendVerificationEmail: vi.fn(),
  changePassword: vi.fn(),
  forgotPassword: vi.fn(),
  resetPassword: vi.fn(),
}))

vi.mock('@/api/modules/portal', () => ({
  auth: portalAuthMocks,
}))

vi.mock('@/utils/entitlement', () => ({
  toNormalizedError: vi.fn((error, fallbackMessage) => ({
    message: error?.message || fallbackMessage,
  })),
}))

import { useUserPortalStore } from './userPortal'

describe('userPortal store admin bridge', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    setActivePinia(createPinia())
  })

  it('syncs admin profile refresh into the admin session bridge', async () => {
    sessionStorage.setItem('userToken', 'portal-admin-token')
    const store = useUserPortalStore()

    portalAuthMocks.getProfile.mockResolvedValueOnce({
      id: 1,
      username: 'admin',
      role: 'admin',
      permissions: ['*'],
    })

    await store.fetchProfile({ silent: true })

    expect(store.isAdmin).toBe(true)
    expect(store.adminEntryPath).toBe('/admin/dashboard')
    expect(sessionStorage.getItem('token')).toBe('portal-admin-token')
    expect(JSON.parse(sessionStorage.getItem('adminUserInfo'))).toMatchObject({
      role: 'admin',
      permissions: ['*'],
    })
    expect(sessionStorage.getItem('adminRole')).toBe('admin')
  })

  it('normalizes admin bridge permissions when the profile omits them', async () => {
    sessionStorage.setItem('userToken', 'portal-admin-token')
    const store = useUserPortalStore()

    portalAuthMocks.getProfile.mockResolvedValueOnce({
      id: 1,
      username: 'admin',
      role: 'admin',
    })

    await store.fetchProfile({ silent: true })

    expect(store.isAdmin).toBe(true)
    expect(store.adminEntryPath).toBe('/admin/dashboard')
    expect(JSON.parse(sessionStorage.getItem('adminUserInfo'))).toMatchObject({
      role: 'admin',
      permissions: ['*'],
    })
  })

  it('does not remove an independent admin session during non-admin profile refresh', async () => {
    sessionStorage.setItem('userToken', 'portal-user-token')
    sessionStorage.setItem('token', 'separate-admin-token')
    sessionStorage.setItem('adminUserInfo', JSON.stringify({ role: 'admin', permissions: ['*'] }))
    sessionStorage.setItem('adminRole', 'admin')
    const store = useUserPortalStore()

    portalAuthMocks.getProfile.mockResolvedValueOnce({
      id: 2,
      username: 'user',
      role: 'user',
      permissions: [],
    })

    await store.fetchProfile({ silent: true })

    expect(store.isAdmin).toBe(false)
    expect(sessionStorage.getItem('token')).toBe('separate-admin-token')
    expect(JSON.parse(sessionStorage.getItem('adminUserInfo'))).toMatchObject({ role: 'admin' })
    expect(sessionStorage.getItem('adminRole')).toBe('admin')
  })

  it('clears an admin bridge that belongs to the same portal token after role downgrade', async () => {
    sessionStorage.setItem('userToken', 'shared-token')
    sessionStorage.setItem('token', 'shared-token')
    sessionStorage.setItem('adminUserInfo', JSON.stringify({ role: 'admin', permissions: ['*'] }))
    sessionStorage.setItem('adminRole', 'admin')
    const store = useUserPortalStore()

    portalAuthMocks.getProfile.mockResolvedValueOnce({
      id: 3,
      username: 'former-admin',
      role: 'user',
      permissions: [],
    })

    await store.fetchProfile({ silent: true })

    expect(store.isAdmin).toBe(false)
    expect(sessionStorage.getItem('token')).toBeNull()
    expect(sessionStorage.getItem('adminUserInfo')).toBeNull()
    expect(sessionStorage.getItem('adminRole')).toBeNull()
  })
})
