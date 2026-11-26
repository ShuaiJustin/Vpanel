/**
 * 用户前台公告 Store
 * 管理公告列表和已读状态
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { announcements as announcementsApi } from '@/api/modules/portal'
import { toNormalizedError } from '@/utils/entitlement'

export const usePortalAnnouncementsStore = defineStore('portalAnnouncements', () => {
  // 状态
  const announcements = ref([])
  const currentAnnouncement = ref(null)
  const total = ref(0)
  const unreadCount = ref(0)
  const loading = ref(false)
  const error = ref(null)

  // 分页状态
  const pagination = ref({
    limit: 10,
    offset: 0
  })

  // 计算属性
  const pinnedAnnouncements = computed(() => 
    announcements.value.filter(a => a.is_pinned)
  )

  const regularAnnouncements = computed(() => 
    announcements.value.filter(a => !a.is_pinned)
  )

  const hasUnread = computed(() => unreadCount.value > 0)

  const hasMore = computed(() => 
    pagination.value.offset + announcements.value.length < total.value
  )

  // 方法
  async function fetchAnnouncements(params = {}) {
    loading.value = true
    error.value = null
    try {
      const response = await announcementsApi.getAnnouncements({
        ...pagination.value,
        ...params
      })
      announcements.value = response.announcements || []
      total.value = response.total || 0
      unreadCount.value = response.unread_count || 0
      return response
    } catch (err) {
      const normalizedError = toNormalizedError(err, '获取公告列表失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function fetchAnnouncement(id) {
    loading.value = true
    error.value = null
    try {
      const response = await announcementsApi.getAnnouncement(id)
      const announcement = {
        ...(response.announcement || response),
        is_read: response.is_read ?? response.announcement?.is_read ?? false
      }
      currentAnnouncement.value = announcement
      if (!announcement.is_read) {
        await markAsRead(id)
        announcement.is_read = true
      }
      return announcement
    } catch (err) {
      const normalizedError = toNormalizedError(err, '获取公告详情失败')
      error.value = normalizedError.message
      throw normalizedError
    } finally {
      loading.value = false
    }
  }

  async function fetchUnreadCount() {
    try {
      const response = await announcementsApi.getUnreadCount()
      unreadCount.value = response.unread_count || 0
      return unreadCount.value
    } catch (err) {
      console.error('Failed to fetch unread count:', err)
    }
  }

  async function markAsRead(id) {
    try {
      await announcementsApi.markAsRead(id)
      // 更新本地状态
      const announcement = announcements.value.find(a => a.id === id)
      if (announcement && !announcement.is_read) {
        announcement.is_read = true
        unreadCount.value = Math.max(0, unreadCount.value - 1)
      }
      if (currentAnnouncement.value && currentAnnouncement.value.id === id) {
        currentAnnouncement.value.is_read = true
      }
    } catch (err) {
      console.error('Failed to mark as read:', err)
    }
  }

  async function markAllAsRead() {
    try {
      const unreadIds = announcements.value
        .filter(item => !item.is_read)
        .map(item => item.id)

      if (unreadIds.length === 0) {
        unreadCount.value = 0
        return
      }

      await announcementsApi.markAllAsRead(unreadIds)
      // 更新本地状态
      announcements.value.forEach(a => {
        a.is_read = true
      })
      if (currentAnnouncement.value) {
        currentAnnouncement.value.is_read = true
      }
      unreadCount.value = 0
    } catch (err) {
      const normalizedError = toNormalizedError(err, '标记全部已读失败')
      error.value = normalizedError.message
      throw normalizedError
    }
  }

  function nextPage() {
    pagination.value.offset += pagination.value.limit
  }

  function prevPage() {
    pagination.value.offset = Math.max(0, pagination.value.offset - pagination.value.limit)
  }

  function resetPagination() {
    pagination.value.offset = 0
  }

  function clearCurrentAnnouncement() {
    currentAnnouncement.value = null
  }

  function getCategoryLabel(category) {
    const labels = {
      general: '通知',
      maintenance: '维护',
      update: '更新',
      promotion: '活动'
    }
    return labels[category] || category
  }

  function getCategoryType(category) {
    const types = {
      general: 'info',
      maintenance: 'warning',
      update: 'success',
      promotion: 'danger'
    }
    return types[category] || ''
  }

  return {
    // 状态
    announcements,
    currentAnnouncement,
    total,
    unreadCount,
    loading,
    error,
    pagination,
    // 计算属性
    pinnedAnnouncements,
    regularAnnouncements,
    hasUnread,
    hasMore,
    // 方法
    fetchAnnouncements,
    fetchAnnouncement,
    fetchUnreadCount,
    markAsRead,
    markAllAsRead,
    nextPage,
    prevPage,
    resetPagination,
    clearCurrentAnnouncement,
    getCategoryLabel,
    getCategoryType
  }
})
