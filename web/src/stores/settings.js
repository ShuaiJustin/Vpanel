/**
 * 设置状态管理
 * 管理系统设置的获取、更新、备份和恢复
 */
import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { settingsApi } from '@/api'

// 持久化 key
const STORAGE_KEY = 'v_panel_settings_cache'

export const useSettingsStore = defineStore('settings', () => {
  // 状态
  const settings = ref(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref(null)
  const isDirty = ref(false)
  const originalSettings = ref(null)

  // 计算属性
  const siteName = computed(() => settings.value?.siteName || 'V Panel')
  const siteDescription = computed(() => settings.value?.siteDescription || '')
  const allowRegistration = computed(() => settings.value?.allowRegistration || false)
  const rateLimitEnabled = computed(() => settings.value?.rateLimitEnabled || false)

  // 从 sessionStorage 恢复
  const restoreFromStorage = () => {
    try {
      const cached = sessionStorage.getItem(STORAGE_KEY)
      if (cached) {
        const data = JSON.parse(cached)
        settings.value = data.settings
        originalSettings.value = data.originalSettings
        isDirty.value = data.isDirty
      }
    } catch (e) {
      console.error('Failed to restore settings from storage:', e)
    }
  }

  // 保存到 sessionStorage
  const saveToStorage = () => {
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify({
        settings: settings.value,
        originalSettings: originalSettings.value,
        isDirty: isDirty.value
      }))
    } catch (e) {
      console.error('Failed to save settings to storage:', e)
    }
  }

  // 监听变化并持久化
  watch([settings, isDirty], () => {
    saveToStorage()
  }, { deep: true })

  // 初始化时恢复
  restoreFromStorage()

  // 方法
  const updateLocalSetting = (key, value) => {
    if (settings.value) {
      settings.value[key] = value
      isDirty.value = true
    }
  }

  const resetChanges = () => {
    if (originalSettings.value) {
      settings.value = { ...originalSettings.value }
      isDirty.value = false
    }
  }

  // API 方法
  const fetchSettings = async () => {
    loading.value = true
    error.value = null
    
    try {
      const response = await settingsApi.getAll()
      settings.value = response
      originalSettings.value = { ...response }
      isDirty.value = false
      return response
    } catch (err) {
      error.value = err.message || '获取设置失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const saveSettings = async (data = null) => {
    saving.value = true
    error.value = null
    
    const dataToSave = data || settings.value
    
    try {
      const response = await settingsApi.update(dataToSave)
      settings.value = response
      originalSettings.value = { ...response }
      isDirty.value = false
      return response
    } catch (err) {
      error.value = err.message || '保存设置失败'
      throw err
    } finally {
      saving.value = false
    }
  }

  const createBackup = async () => {
    loading.value = true
    error.value = null
    
    try {
      const response = await settingsApi.createBackup()
      return response
    } catch (err) {
      error.value = err.message || '创建备份失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const restoreBackup = async (backupData) => {
    loading.value = true
    error.value = null
    
    try {
      await settingsApi.restore(backupData)
      // 重新获取设置
      await fetchSettings()
      return true
    } catch (err) {
      error.value = err.message || '恢复备份失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  const clearCache = () => {
    sessionStorage.removeItem(STORAGE_KEY)
    settings.value = null
    originalSettings.value = null
    isDirty.value = false
  }

  return {
    // 状态
    settings,
    loading,
    saving,
    error,
    isDirty,
    
    // 计算属性
    siteName,
    siteDescription,
    allowRegistration,
    rateLimitEnabled,
    
    // 方法
    updateLocalSetting,
    resetChanges,
    fetchSettings,
    saveSettings,
    createBackup,
    restoreBackup,
    clearCache
  }
})
