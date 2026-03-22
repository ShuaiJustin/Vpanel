<template>
  <div class="announcements-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        公告中心
      </h1>
      <p class="page-subtitle">
        查看系统公告和重要通知
      </p>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-radio-group
        v-model="selectedCategory"
        size="default"
      >
        <el-radio-button value="">
          全部
        </el-radio-button>
        <el-radio-button value="general">
          公告
        </el-radio-button>
        <el-radio-button value="maintenance">
          维护
        </el-radio-button>
        <el-radio-button value="update">
          更新
        </el-radio-button>
        <el-radio-button value="promotion">
          活动
        </el-radio-button>
      </el-radio-group>

      <div class="filter-right">
        <el-badge
          :value="unreadCount"
          :hidden="unreadCount === 0"
        >
          <el-button
            :disabled="unreadCount === 0"
            @click="markAllAsRead"
          >
            全部已读
          </el-button>
        </el-badge>
      </div>
    </div>

    <!-- 加载状态 -->
    <div
      v-if="loading"
      class="loading-state"
    >
      <el-icon class="loading-icon">
        <Loading />
      </el-icon>
      <p>加载公告列表...</p>
    </div>

    <!-- 空状态 -->
    <el-empty
      v-else-if="filteredAnnouncements.length === 0"
      description="暂无公告"
    />

    <!-- 公告列表 -->
    <div
      v-else
      class="announcements-list"
    >
      <div 
        v-for="item in filteredAnnouncements" 
        :key="item.id"
        class="announcement-card"
        :class="{ unread: !item.is_read, pinned: item.is_pinned }"
        @click="viewAnnouncement(item)"
      >
        <!-- 置顶标记 -->
        <div
          v-if="item.is_pinned"
          class="pinned-badge"
        >
          <el-icon><Top /></el-icon>
          置顶
        </div>

        <!-- 未读标记 -->
        <div
          v-if="!item.is_read"
          class="unread-dot"
        />

        <div class="card-content">
          <div class="card-header">
            <el-tag
              :type="getCategoryType(item.category)"
              size="small"
            >
              {{ getCategoryLabel(item.category) }}
            </el-tag>
            <span class="publish-date">{{ formatDate(item.published_at) }}</span>
          </div>

          <h3 class="announcement-title">
            {{ item.title }}
          </h3>
          
          <p class="announcement-summary">
            {{ getSummary(item.content) }}
          </p>
        </div>

        <el-icon class="arrow-icon">
          <ArrowRight />
        </el-icon>
      </div>
    </div>

    <!-- 分页 -->
    <div
      v-if="total > pageSize"
      class="pagination"
    >
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadAnnouncements"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Loading, Top, ArrowRight } from '@element-plus/icons-vue'
import { usePortalAnnouncementsStore } from '@/stores/portalAnnouncements'

const router = useRouter()
const announcementsStore = usePortalAnnouncementsStore()

// 状态
const loading = ref(false)
const selectedCategory = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

// 计算属性
const filteredAnnouncements = computed(() => {
  let list = announcementsStore.announcements
  if (selectedCategory.value) {
    list = list.filter(a => a.category === selectedCategory.value)
  }
  return list
})

const unreadCount = computed(() => {
  return announcementsStore.unreadCount
})

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
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function getSummary(content) {
  if (!content) return ''
  // 移除 HTML 标签并截取
  const text = content.replace(/<[^>]+>/g, '')
  return text.length > 100 ? text.substring(0, 100) + '...' : text
}

function viewAnnouncement(item) {
  router.push(`/user/announcements/${item.id}`)
}

async function markAllAsRead() {
  try {
    await announcementsStore.markAllAsRead()
    ElMessage.success('已全部标记为已读')
  } catch (error) {
    ElMessage.error('操作失败')
  }
}

async function loadAnnouncements() {
  loading.value = true
  try {
    await announcementsStore.fetchAnnouncements({
      page: currentPage.value,
      page_size: pageSize.value,
      category: selectedCategory.value || undefined
    })
    total.value = announcementsStore.total
  } catch (error) {
    ElMessage.error('加载公告失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadAnnouncements()
})
</script>

<style scoped>
.announcements-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

/* 筛选栏 */
.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
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

/* 公告列表 */
.announcements-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.announcement-card {
  position: relative;
  display: flex;
  align-items: center;
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: all 0.3s;
}

.announcement-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  transform: translateX(4px);
}

.announcement-card.unread {
  border-left: 3px solid #409eff;
}

.announcement-card.pinned {
  background: linear-gradient(135deg, #fff 0%, #f5f7fa 100%);
}

/* 置顶标记 */
.pinned-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  background: #e6a23c;
  color: #fff;
  font-size: 12px;
  border-radius: 4px;
}

/* 未读标记 */
.unread-dot {
  position: absolute;
  top: 50%;
  left: 8px;
  transform: translateY(-50%);
  width: 8px;
  height: 8px;
  background: #409eff;
  border-radius: 50%;
}

.card-content {
  flex: 1;
  min-width: 0;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.publish-date {
  font-size: 13px;
  color: #909399;
}

.announcement-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.announcement-summary {
  font-size: 14px;
  color: #606266;
  margin: 0;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.arrow-icon {
  flex-shrink: 0;
  font-size: 16px;
  color: #c0c4cc;
  margin-left: 16px;
}

/* 分页 */
.pagination {
  display: flex;
  justify-content: center;
  margin-top: 24px;
}

/* 响应式 */
@media (max-width: 768px) {
  .filter-bar {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
