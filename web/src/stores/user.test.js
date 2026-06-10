import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const authApiMocks = vi.hoisted(() => ({
  login: vi.fn(),
  logout: vi.fn(),
  getProfile: vi.fn(),
  updateProfile: vi.fn(),
  changePassword: vi.fn(),
}))

const usersApiMocks = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  enable: vi.fn(),
  disable: vi.fn(),
}))

vi.mock('@/api', () => ({
  authApi: authApiMocks,
  usersApi: usersApiMocks,
}))

vi.mock('@/utils/entitlement', () => ({
  toNormalizedError: vi.fn((error, fallbackMessage) => ({
    message: error?.message || fallbackMessage,
  })),
}))

import { useUserStore } from './user'

describe('admin user store permissions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    setActivePinia(createPinia())
  })

  it('restores admin sessions with wildcard permissions when cached permissions are missing', () => {
    localStorage.setItem('token', 'admin-token')
    localStorage.setItem('adminUserInfo', JSON.stringify({
      id: 1,
      username: 'admin',
      role: 'admin',
    }))

    const store = useUserStore()

    expect(store.user).toMatchObject({
      role: 'admin',
      permissions: ['*'],
    })
  })

  it('normalizes admin profile responses that omit permissions', async () => {
    localStorage.setItem('token', 'admin-token')
    authApiMocks.getProfile.mockResolvedValueOnce({
      id: 1,
      username: 'admin',
      role: 'admin',
      status: true,
    })

    const store = useUserStore()
    await store.getUser()

    expect(store.user.permissions).toEqual(['*'])
    expect(JSON.parse(localStorage.getItem('adminUserInfo'))).toMatchObject({
      role: 'admin',
      permissions: ['*'],
    })
  })
})
