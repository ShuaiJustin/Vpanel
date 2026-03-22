<!-- eslint-disable vue/no-v-html -->
<template>
  <div class="announcement-detail-page">
    <!-- 返回按钮 -->
    <div class="back-bar">
      <el-button
        link
        @click="goBack"
      >
        <el-icon><ArrowLeft /></el-icon>
        返回公告列表
      </el-button>
    </div>

    <!-- 加载状态 -->
    <div
      v-if="loading"
      class="loading-state"
    >
      <el-icon class="loading-icon">
        <Loading />
      </el-icon>
      <p>加载公告内容...</p>
    </div>

    <!-- 公告不存在 -->
    <el-empty
      v-else-if="!announcement"
      description="公告不存在或已删除"
    />

    <!-- 公告内容 -->
    <el-card
      v-else
      class="announcement-card"
      shadow="never"
    >
      <div class="announcement-header">
        <div class="header-meta">
          <el-tag
            :type="getCategoryType(announcement.category)"
            size="small"
          >
            {{ getCategoryLabel(announcement.category) }}
          </el-tag>
          <span class="publish-date">
            <el-icon><Calendar /></el-icon>
            {{ formatDate(announcement.published_at) }}
          </span>
        </div>
        <h1 class="announcement-title">
          {{ announcement.title }}
        </h1>
      </div>

      <el-divider />

      <!-- eslint-disable-next-line vue/no-v-html -->
      <div
        class="announcement-content"
        v-html="safeAnnouncementContent"
      />

      <el-divider />

      <div class="announcement-footer">
        <div class="footer-info">
          <span
            v-if="announcement.is_pinned"
            class="pinned-tag"
          >
            <el-icon><Top /></el-icon>
            置顶公告
          </span>
        </div>
        <div class="footer-actions">
          <el-button
            v-if="prevAnnouncement"
            @click="viewAnnouncement(prevAnnouncement.id)"
          >
            <el-icon><ArrowLeft /></el-icon>
            上一篇
          </el-button>
          <el-button
            v-if="nextAnnouncement"
            @click="viewAnnouncement(nextAnnouncement.id)"
          >
            下一篇
            <el-icon><ArrowRight /></el-icon>
          </el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, ArrowRight, Loading, Calendar, Top } from '@element-plus/icons-vue'
import { usePortalAnnouncementsStore } from '@/stores/portalAnnouncements'
import { sanitizeHtml } from '@/utils/htmlSanitizer'

const router = useRouter()
const route = useRoute()
const announcementsStore = usePortalAnnouncementsStore()

// 状态
const loading = ref(false)
const announcement = ref(null)
const prevAnnouncement = ref(null)
const nextAnnouncement = ref(null)
const safeAnnouncementContent = computed(() => sanitizeHtml(announcement.value?.content || ''))

// 方法
function getCategoryType(category) {
  const types = {
    general: 'info',
    maintenance: 'warning',
    update: 'success',
    promotion: 'danger'
  }
  return types[category] || 'info'
}

function getCategoryLabel(category) {
  const labels = {
    general: '公告',
    maintenance: '维护',
    update: '更新',
    promotion: '活动'
  }
  return labels[category] || '公告'
}

function formatDate(dateStr) {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function goBack() {
  router.push('/user/announcements')
}

function viewAnnouncement(id) {
  router.push(`/user/announcements/${id}`)
}

async function loadAnnouncement() {
  const id = route.params.id
  if (!id) {
    router.push('/user/announcements')
    return
  }

  loading.value = true
  try {
    announcement.value = await announcementsStore.fetchAnnouncement(id)
    
    // 标记为已读
    if (announcement.value && !announcement.value.is_read) {
      await announcementsStore.markAsRead(id)
    }

    // 获取上一篇和下一篇
    const allAnnouncements = announcementsStore.announcements
    const currentIndex = allAnnouncements.findIndex(a => a.id === parseInt(id))
    if (currentIndex > 0) {
      nextAnnouncement.value = allAnnouncements[currentIndex - 1]
    } else {
      nextAnnouncement.value = null
    }
    if (currentIndex < allAnnouncements.length - 1) {
      prevAnnouncement.value = allAnnouncements[currentIndex + 1]
    } else {
      prevAnnouncement.value = null
    }
  } catch (error) {
    ElMessage.error('加载公告失败')
    announcement.value = null
  } finally {
    loading.value = false
  }
}

// 监听路由变化
watch(() => route.params.id, () => {
  if (route.params.id) {
    loadAnnouncement()
  }
})

onMounted(() => {
  loadAnnouncement()
})
</script>

<style scoped>
.announcement-detail-page {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.back-bar {
  margin-bottom: 20px;
}

/* 加载状态 */
.loading-state {
  text-align: center;
  padding: 60px 0;
  color: #909399;
}

.loading-icon {
  font-size: 32px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 公告卡片 */
.announcement-card {
  border-radius: 12px;
}

.announcement-header {
  margin-bottom: 16px;
}

.header-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.publish-date {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 14px;
  color: #909399;
}

.announcement-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0;
  line-height: 1.4;
}

/* 公告内容 */
.announcement-content {
  font-size: 15px;
  color: #606266;
  line-height: 1.8;
}

.announcement-content :deep(h1),
.announcement-content :deep(h2),
.announcement-content :deep(h3) {
  color: #303133;
  margin: 24px 0 12px 0;
}

.announcement-content :deep(p) {
  margin: 12px 0;
}

.announcement-content :deep(ul),
.announcement-content :deep(ol) {
  padding-left: 24px;
  margin: 12px 0;
}

.announcement-content :deep(li) {
  margin: 8px 0;
}

.announcement-content :deep(a) {
  color: #409eff;
  text-decoration: none;
}

.announcement-content :deep(a:hover) {
  text-decoration: underline;
}

.announcement-content :deep(code) {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
}

.announcement-content :deep(pre) {
  background: #f5f7fa;
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
}

.announcement-content :deep(blockquote) {
  border-left: 4px solid #409eff;
  padding-left: 16px;
  margin: 16px 0;
  color: #909399;
}

/* 公告底部 */
.announcement-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pinned-tag {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #e6a23c;
  font-size: 14px;
}

.footer-actions {
  display: flex;
  gap: 12px;
}

/* 响应式 */
@media (max-width: 768px) {
  .announcement-title {
    font-size: 20px;
  }

  .announcement-footer {
    flex-direction: column;
    gap: 16px;
  }
}
</style>
