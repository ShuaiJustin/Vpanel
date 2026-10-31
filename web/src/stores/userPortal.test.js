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

  it('keeps a password challenge unauthenticated and sends it to the second factor endpoint', async () => {
    const store = useUserPortalStore()
    const challenge = { requires_2fa: true, user_id: 1, challenge_token: 'one-time-challenge' }
    portalAuthMocks.login.mockResolvedValueOnce(challenge)
    expect(await store.login({ username: 'admin', password: 'secret' })).toEqual(challenge)
    expect(sessionStorage.getItem('userToken')).toBeNull()
    expect(sessionStorage.getItem('token')).toBeNull()
    portalAuthMocks.verify2FALogin.mockResolvedValueOnce({ token: 'verified-token', user: { id: 1, role: 'admin', permissions: ['*'] } })
    const secondStep = { user_id: 1, challenge_token: challenge.challenge_token, code: '123456' }
    await store.completeTwoFactorLogin(secondStep)
    expect(portalAuthMocks.verify2FALogin).toHaveBeenCalledWith(secondStep)
    expect(sessionStorage.getItem('userToken')).toBe('verified-token')
    expect(sessionStorage.getItem('token')).toBe('verified-token')
  })

  it('clears the portal and shared administrator session after a password change', async () => {
    const store = useUserPortalStore()
    store.completeOAuthLogin({ token: 'shared-session', user: { id: 1, role: 'admin' } })
    portalAuthMocks.changePassword.mockResolvedValueOnce({})
    await store.changePassword({ current_password: 'old-password', new_password: 'new-password' })
    expect(store.isAuthenticated).toBe(false)
    expect(sessionStorage.getItem('userToken')).toBeNull()
    expect(sessionStorage.getItem('token')).toBeNull()
    expect(sessionStorage.getItem('adminUserInfo')).toBeNull()
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


describe('profile request coalescing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    sessionStorage.setItem('userToken', 'session-a')
    setActivePinia(createPinia())
  })

  it('shares simultaneous layout/page requests and retries after failure', async () => {
    let resolve
    portalAuthMocks.getProfile.mockImplementationOnce(() => new Promise(r => { resolve = r }))
    const store = useUserPortalStore()
    const a = store.fetchProfile({ silent: true })
    const b = store.fetchProfile()
    expect(portalAuthMocks.getProfile).toHaveBeenCalledTimes(1)
    resolve({ id: 1, username: 'first', role: 'user' })
    await Promise.all([a,b])
    expect(store.user.id).toBe(1)
    portalAuthMocks.getProfile.mockRejectedValueOnce(new Error('offline'))
    await expect(store.fetchProfile()).rejects.toThrow('offline')
    portalAuthMocks.getProfile.mockResolvedValueOnce({ id: 1, username: 'refreshed', role: 'user' })
    await store.fetchProfile()
    expect(store.user.username).toBe('refreshed')
  })

  it('does not restore user information after logout', async () => {
    let resolve
    portalAuthMocks.getProfile.mockImplementationOnce(() => new Promise(r => { resolve = r }))
    const store = useUserPortalStore()
    const pending = store.fetchProfile({ silent: true })
    await store.logout()
    resolve({ id: 1, role: 'admin', permissions: ['*'] })
    await pending
    expect(store.user).toBeNull()
    expect(sessionStorage.getItem('token')).toBeNull()
    expect(sessionStorage.getItem('userInfo')).toBeNull()
  })
})
