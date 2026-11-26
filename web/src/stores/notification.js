/**
 * 通知状态管理
 * 管理应用内通知、消息队列和未读计数
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// 通知类型
export const NotificationType = {
  SUCCESS: 'success',
  WARNING: 'warning',
  ERROR: 'error',
  INFO: 'info'
}

// 持久化 key
const STORAGE_KEY = 'v_panel_notifications'

export const useNotificationStore = defineStore('notification', () => {
  // 状态
  const notifications = ref([])
  const maxNotifications = ref(100)

  // 从 sessionStorage 恢复
  const restoreFromStorage = () => {
    try {
      const cached = sessionStorage.getItem(STORAGE_KEY)
      if (cached) {
        notifications.value = JSON.parse(cached)
      }
    } catch (e) {
      console.error('Failed to restore notifications from storage:', e)
    }
  }

  // 保存到 sessionStorage
  const saveToStorage = () => {
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(notifications.value))
    } catch (e) {
      console.error('Failed to save notifications to storage:', e)
    }
  }

  // 初始化时恢复
  restoreFromStorage()

  // 计算属性
  const unreadCount = computed(() => 
    notifications.value.filter(n => !n.read).length
  )

  const hasUnread = computed(() => unreadCount.value > 0)

  const recentNotifications = computed(() => 
    notifications.value.slice(0, 10)
  )

  const errorNotifications = computed(() =>
    notifications.value.filter(n => n.type === NotificationType.ERROR)
  )

  const warningNotifications = computed(() =>
    notifications.value.filter(n => n.type === NotificationType.WARNING)
  )

  // 方法
  const addNotification = (notification) => {
    const newNotification = {
      id: Date.now() + Math.random().toString(36).substring(2),
      type: notification.type || NotificationType.INFO,
      title: notification.title || '',
      message: notification.message || '',
      read: false,
      timestamp: new Date().toISOString(),
      ...notification
    }

    notifications.value.unshift(newNotification)

    // 限制通知数量
    if (notifications.value.length > maxNotifications.value) {
      notifications.value = notifications.value.slice(0, maxNotifications.value)
    }

    saveToStorage()
    return newNotification
  }

  const success = (title, message = '') => {
    return addNotification({ type: NotificationType.SUCCESS, title, message })
  }

  const warning = (title, message = '') => {
    return addNotification({ type: NotificationType.WARNING, title, message })
  }

  const error = (title, message = '') => {
    return addNotification({ type: NotificationType.ERROR, title, message })
  }

  const info = (title, message = '') => {
    return addNotification({ type: NotificationType.INFO, title, message })
  }

  const markAsRead = (id) => {
    const notification = notifications.value.find(n => n.id === id)
    if (notification) {
      notification.read = true
      saveToStorage()
    }
  }

  const markAllAsRead = () => {
    notifications.value.forEach(n => {
      n.read = true
    })
    saveToStorage()
  }

  const removeNotification = (id) => {
    const index = notifications.value.findIndex(n => n.id === id)
    if (index >= 0) {
      notifications.value.splice(index, 1)
      saveToStorage()
    }
  }

  const clearAll = () => {
    notifications.value = []
    saveToStorage()
  }

  const clearRead = () => {
    notifications.value = notifications.value.filter(n => !n.read)
    saveToStorage()
  }

  const getById = (id) => {
    return notifications.value.find(n => n.id === id)
  }

  return {
    // 状态
    notifications,
    maxNotifications,
    
    // 计算属性
    unreadCount,
    hasUnread,
    recentNotifications,
    errorNotifications,
    warningNotifications,
    
    // 方法
    addNotification,
    success,
    warning,
    error,
    info,
    markAsRead,
    markAllAsRead,
    removeNotification,
    clearAll,
    clearRead,
    getById
  }
})
