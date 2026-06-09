/**
 * 用户前台门户 Store
 * 管理用户认证状态和个人信息
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { auth as authApi } from '@/api/modules/portal'
import { toNormalizedError } from '@/utils/entitlement'

export const useUserPortalStore = defineStore('userPortal', () => {
  const ADMIN_USER_INFO_KEY = 'adminUserInfo'
  const ADMIN_ROLE_KEY = 'adminRole'
  const adminRouteCandidates = [
    { path: '/admin/dashboard', permissionsAll: ['stats:read', 'system:read'] },
    { path: '/admin/inbounds', permissionsAll: ['proxy:read'] },
    { path: '/admin/users', permissionsAll: ['user:read'] },
    { path: '/admin/roles', permissionsAll: ['role:read'] },
    { path: '/admin/system-monitor', permissionsAll: ['system:read'] },
    { path: '/admin/stats', permissionsAll: ['stats:read'] },
    { path: '/admin/profile', permissionsAll: [] }
  ]

  function getStoredValue(key) {
    return sessionStorage.getItem(key) || localStorage.getItem(key)
  }

  function getCurrentAuthStorage() {
    const currentToken = token?.value || ''
    if (currentToken && sessionStorage.getItem('userToken') === currentToken) {
      return sessionStorage
    }
    if (currentToken && localStorage.getItem('userToken') === currentToken) {
      return localStorage
    }
    if (sessionStorage.getItem('userToken')) {
      return sessionStorage
    }
    if (localStorage.getItem('userToken')) {
      return localStorage
    }
    return sessionStorage
  }

  function getAuthStorage(remember) {
    return remember ? localStorage : sessionStorage
  }

  function syncUserInfoStorage(value) {
    const storage = getCurrentAuthStorage()
    const otherStorage = storage === localStorage ? sessionStorage : localStorage

    if (value == null) {
      storage.removeItem('userInfo')
      otherStorage.removeItem('userInfo')
      return
    }

    storage.setItem('userInfo', JSON.stringify(value))
    otherStorage.removeItem('userInfo')
    syncAdminBridge(storage, otherStorage, value, token.value || getStoredValue('userToken'), false)
  }

  function clearPersistedAuth() {
    const portalToken = token.value || getStoredValue('userToken')
    const adminToken = getStoredValue('token')

    for (const storage of [localStorage, sessionStorage]) {
      storage.removeItem('userToken')
      storage.removeItem('userInfo')
      if (portalToken && adminToken && portalToken === adminToken) {
        storage.removeItem('token')
        storage.removeItem(ADMIN_USER_INFO_KEY)
        storage.removeItem(ADMIN_ROLE_KEY)
      }
    }
  }

  function clearAdminBridge(storage, tokenValue, clearExisting) {
    const storedAdminToken = storage.getItem('token')
    if (!clearExisting && (!tokenValue || storedAdminToken !== tokenValue)) {
      return
    }
    storage.removeItem('token')
    storage.removeItem(ADMIN_USER_INFO_KEY)
    storage.removeItem(ADMIN_ROLE_KEY)
  }

  function syncAdminBridge(storage, otherStorage, userInfo, tokenValue, clearExisting = true) {
    if (userInfo?.role === 'admin' && tokenValue) {
      storage.setItem('token', tokenValue)
      storage.setItem(ADMIN_USER_INFO_KEY, JSON.stringify(userInfo))
      storage.setItem(ADMIN_ROLE_KEY, userInfo.role)
      otherStorage.removeItem('token')
      otherStorage.removeItem(ADMIN_USER_INFO_KEY)
      otherStorage.removeItem(ADMIN_ROLE_KEY)
      return
    }

    clearAdminBridge(storage, tokenValue, clearExisting)
    clearAdminBridge(otherStorage, tokenValue, clearExisting)
  }

  function hasPermission(userInfo, permission) {
    if (!permission) return true
    const permissions = Array.isArray(userInfo?.permissions) ? userInfo.permissions : []
    return permissions.includes('*') || permissions.includes(permission)
  }

  function hasAllPermissions(userInfo, permissions = []) {
    return permissions.every(permission => hasPermission(userInfo, permission))
  }

  function getAdminEntryPath(userInfo) {
    const match = adminRouteCandidates.find(candidate => hasAllPermissions(userInfo, candidate.permissionsAll))
    return match?.path || '/admin/profile'
  }

  // 状态
  const user = ref(null)
  const token = ref(getStoredValue('userToken'))
  const loading = ref(false)
  const error = ref(null)

  // 计算属性
  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const adminEntryPath = computed(() => getAdminEntryPath(user.value))
  const username = computed(() => user.value?.username || '')
  const email = computed(() => user.value?.email || '')
  const status = computed(() => user.value?.status || 'unknown')
  const hasActiveSubscription = computed(() => Boolean(user.value?.has_active_subscription))
  const hasActiveTrial = computed(() => Boolean(user.value?.has_active_trial))
  const entitlementType = computed(() => user.value?.entitlement_type || 'none')
  const hasEntitlement = computed(() => hasActiveSubscription.value || hasActiveTrial.value)
  const trafficUsed = computed(() => user.value?.traffic_used || 0)
  const trafficLimit = computed(() => user.value?.traffic_limit || 0)
  const trafficPercent = computed(() => {
    if (!trafficLimit.value) return 0
    return Math.min(100, Math.round((trafficUsed.value / trafficLimit.value) * 100))
  })
  const expiresAt = computed(() => user.value?.expires_at || null)
  const isExpired = computed(() => {
    if (!expiresAt.value) return false
    return new Date(expiresAt.value) < new Date()
  })
  const daysUntilExpiry = computed(() => {
    if (!expiresAt.value) return null
    const diff = new Date(expiresAt.value) - new Date()
    return Math.ceil(diff / (1000 * 60 * 60 * 24))
  })
  const twoFactorEnabled = computed(() => user.value?.two_factor_enabled || false)
  const availableNodes = computed(() => user.value?.available_nodes || 0)

  function clearStoredAuth() {
    token.value = null
    user.value = null
    clearPersistedAuth()
  }

  // 方法
  async function login(credentials) {
    loading.value = true
    error.value = null
    try {
      const response = await authApi.login(credentials)
      if (response.requires_2fa) {
        return response
      }

      const storage = getAuthStorage(Boolean(credentials?.remember))
      const otherStorage = storage === localStorage ? sessionStorage : localStorage

      token.value = response.token
      user.value = response.user
      storage.setItem('userToken', response.token)
      storage.setItem('userInfo', JSON.stringify(response.user))
      otherStorage.removeItem('userToken')
      otherStorage.removeItem('userInfo')
      syncAdminBridge(storage, otherStorage, response.user, response.token)
      
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '登录失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function completeTwoFactorLogin(data, remember = false) {
    loading.value = true
    error.value = null
    try {
      const response = await authApi.verify2FALogin(data)
      const storage = getAuthStorage(remember)
      const otherStorage = storage === localStorage ? sessionStorage : localStorage

      token.value = response.token
      user.value = response.user
      storage.setItem('userToken', response.token)
      storage.setItem('userInfo', JSON.stringify(response.user))
      otherStorage.removeItem('userToken')
      otherStorage.removeItem('userInfo')
      syncAdminBridge(storage, otherStorage, response.user, response.token)

      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '两步验证失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function register(data) {
    loading.value = true
    error.value = null
    try {
      const response = await authApi.register(data)
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '注册失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch (err) {
      console.error('Logout error:', err)
    } finally {
      clearStoredAuth()
    }
  }

  async function fetchProfile(options = {}) {
    if (!token.value) return

    const { silent = false } = options
    if (!silent) {
      loading.value = true
    }
    error.value = null
    try {
      const response = await authApi.getProfile()
      user.value = response
      syncUserInfoStorage(response)
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '获取用户信息失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      if (!silent) {
        loading.value = false
      }
    }
  }

  async function updateProfile(data) {
    loading.value = true
    error.value = null
    try {
      const response = await authApi.updateProfile(data)
      const updatedUser = response?.user || response
      user.value = { ...user.value, ...updatedUser }
      syncUserInfoStorage(user.value)
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '更新资料失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function uploadAvatar(file) {
    loading.value = true
    error.value = null
    try {
      const response = await authApi.uploadAvatar(file)
      const updatedUser = response?.user || response
      user.value = { ...user.value, ...updatedUser }
      syncUserInfoStorage(user.value)
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '上传头像失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function bindTelegram(chatId) {
    loading.value = true
    error.value = null
    try {
      const response = await authApi.bindTelegram({ chat_id: chatId })
      const updatedUser = response?.user || response
      user.value = { ...user.value, ...updatedUser }
      syncUserInfoStorage(user.value)
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '绑定 Telegram 失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function unbindTelegram() {
    loading.value = true
    error.value = null
    try {
      const response = await authApi.unbindTelegram()
      const updatedUser = response?.user || response
      user.value = { ...user.value, ...updatedUser }
      syncUserInfoStorage(user.value)
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '解绑 Telegram 失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function resendVerificationEmail() {
    loading.value = true
    error.value = null
    try {
      return await authApi.resendVerificationEmail()
    } catch (err) {
      const normalizedError = toNormalizedError(err, '发送验证邮件失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function changePassword(data) {
    loading.value = true
    error.value = null
    try {
      await authApi.changePassword(data)
      if (user.value) {
        user.value = { ...user.value, force_password_change: false, forcePasswordChange: false }
        syncUserInfoStorage(user.value)
      }
    } catch (err) {
      const normalizedError = toNormalizedError(err, '修改密码失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  function ensureAdminSession() {
    const tokenValue = token.value || getStoredValue('userToken')
    if (!user.value || user.value.role !== 'admin' || !tokenValue) {
      return false
    }

    const storage = getCurrentAuthStorage()
    const otherStorage = storage === localStorage ? sessionStorage : localStorage
    syncAdminBridge(storage, otherStorage, user.value, tokenValue)
    return true
  }

  async function forgotPassword(email) {
    loading.value = true
    error.value = null
    try {
      await authApi.forgotPassword({ email })
    } catch (err) {
      const normalizedError = toNormalizedError(err, '发送重置邮件失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function resetPassword(data) {
    loading.value = true
    error.value = null
    try {
      await authApi.resetPassword(data)
    } catch (err) {
      const normalizedError = toNormalizedError(err, '重置密码失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  // 初始化：从本地存储恢复用户信息
  function init() {
    const savedUser = getStoredValue('userInfo')
    if (savedUser) {
      try {
        user.value = JSON.parse(savedUser)
        ensureAdminSession()
      } catch (e) {
        console.error('Failed to parse saved user info:', e)
      }
    }
  }

  init()

  return {
    // 状态
    user,
    token,
    loading,
    error,
    // 计算属性
    isAuthenticated,
    isAdmin,
    adminEntryPath,
    username,
    email,
    status,
    hasActiveSubscription,
    hasActiveTrial,
    entitlementType,
    hasEntitlement,
    trafficUsed,
    trafficLimit,
    trafficPercent,
    expiresAt,
    isExpired,
    daysUntilExpiry,
    twoFactorEnabled,
    availableNodes,
    // 方法
    login,
    completeTwoFactorLogin,
    register,
    logout,
    fetchProfile,
    updateProfile,
    uploadAvatar,
    bindTelegram,
    unbindTelegram,
    resendVerificationEmail,
    changePassword,
    forgotPassword,
    resetPassword,
    ensureAdminSession
  }
})
