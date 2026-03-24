import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, usersApi } from '@/api'

export const useUserStore = defineStore('user', () => {
  // 状态
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(null)
  const loading = ref(false)
  const error = ref(null)

  // 计算属性
  const isLoggedIn = computed(() => !!token.value)
  const username = computed(() => user.value?.username || '')
  const role = computed(() => user.value?.role || '')
  const userId = computed(() => user.value?.id || null)

  // 方法
  const setToken = (newToken) => {
    token.value = newToken
    localStorage.setItem('token', newToken)
  }

  const setUser = (userInfo) => {
    user.value = userInfo
    // 同步角色到 localStorage，供路由守卫使用
    if (userInfo?.role) {
      localStorage.setItem('userRole', userInfo.role)
    }
  }

  const clearAuth = () => {
    token.value = ''
    user.value = null
    loading.value = false
    error.value = null
    for (const storage of [localStorage, sessionStorage]) {
      storage.removeItem('token')
      storage.removeItem('userRole')
      storage.removeItem('userToken')
      storage.removeItem('userInfo')
    }
  }

  const extractErrorMessage = (err, fallback) => {
    if (typeof err === 'string') {
      return err
    }
    return err?.message || err?.error || fallback
  }

  // API方法
  const login = async (credentials) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await authApi.login(credentials)

      if (!response.token || !response.user) {
        error.value = '服务器返回的数据格式不正确'
        throw new Error('服务器返回的数据格式不正确')
      }
      
      const { token: newToken, user: userInfo } = response
      
      setToken(newToken)
      setUser(userInfo)
      return true
    } catch (err) {
      // 根据错误类型返回友好的错误消息
      if (err.code === 'UNAUTHORIZED' || err.status === 401) {
        error.value = '用户名或密码错误'
      } else if (err.code === 'NETWORK_ERROR') {
        error.value = '网络连接失败，请检查网络'
      } else {
        error.value = extractErrorMessage(err, '登录失败，请稍后重试')
      }
      
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const logout = async () => {
    try {
      await authApi.logout()
    } catch (err) {
      console.error('Admin logout failed:', err)
    } finally {
      clearAuth()
    }
  }

  const getUser = async () => {
    if (!token.value) return null
    
    loading.value = true
    error.value = null
    
    try {
      const response = await authApi.getProfile()

      if (!response.id) {
        error.value = '服务器返回的数据格式不正确'
        throw new Error('服务器返回的数据格式不正确')  
      }
      
      setUser(response)
      return user.value
    } catch (err) {
      error.value = extractErrorMessage(err, '获取用户信息失败')
      
      throw error.value
    } finally {
      loading.value = false
    }
  }

  // 用户管理方法
  const fetchUsers = async (params) => {
    loading.value = true
    error.value = null
    
    try {
      const response = await usersApi.list(params)

      return {
        users: response?.users || [],
        total: response?.total || response?.users?.length || 0
      }
    } catch (err) {
      error.value = extractErrorMessage(err, '获取用户列表失败')
      
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const createUser = async (userData) => {
    loading.value = true
    error.value = null
    
    try {
      return await usersApi.create(userData)
    } catch (err) {
      error.value = extractErrorMessage(err, '创建用户失败')
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const updateUser = async (userId, userData) => {
    loading.value = true
    error.value = null
    
    try {
      return await usersApi.update(userId, userData)
    } catch (err) {
      error.value = extractErrorMessage(err, '更新用户失败')
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const deleteUser = async (userId) => {
    loading.value = true
    error.value = null
    
    try {
      await usersApi.delete(userId)
      return true
    } catch (err) {
      error.value = extractErrorMessage(err, '删除用户失败')
      throw error.value
    } finally {
      loading.value = false
    }
  }

  const updateUserStatus = async (userId, status) => {
    loading.value = true
    error.value = null
    
    try {
      if (status) {
        await usersApi.enable(userId)
      } else {
        await usersApi.disable(userId)
      }
      return true
    } catch (err) {
      error.value = extractErrorMessage(err, '更新用户状态失败')
      throw error.value
    } finally {
      loading.value = false
    }
  }
  
  const updateUserProfile = async (profileData) => {
    loading.value = true
    error.value = null
    
    try {
      const updatedUser = await authApi.updateProfile(profileData)
      setUser(updatedUser)
      return updatedUser
    } catch (err) {
      error.value = extractErrorMessage(err, '更新个人资料失败')
      throw error.value
    } finally {
      loading.value = false
    }
  }
  
  const changePassword = async (passwordData) => {
    loading.value = true
    error.value = null
    
    try {
      await authApi.changePassword({
        old_password: passwordData.currentPassword || passwordData.oldPassword,
        new_password: passwordData.newPassword
      })
      return true
    } catch (err) {
      error.value = extractErrorMessage(err, '修改密码失败')
      throw error.value
    } finally {
      loading.value = false
    }
  }

  return {
    // 状态
    token,
    user,
    loading,
    error,
    
    // 计算属性
    isLoggedIn,
    username,
    role,
    userId,
    
    // 方法
    login,
    logout,
    getUser,
    fetchUsers,
    createUser,
    updateUser,
    deleteUser,
    updateUserStatus,
    updateUserProfile,
    changePassword
  }
})
