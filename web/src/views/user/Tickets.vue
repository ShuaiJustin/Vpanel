<template>
  <div class="tickets-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <div class="header-content">
        <h1 class="page-title">
          工单列表
        </h1>
        <p class="page-subtitle">
          提交工单获取技术支持
        </p>
      </div>
      <el-button
        type="primary"
        @click="createTicket"
      >
        <el-icon><Plus /></el-icon>
        创建工单
      </el-button>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-radio-group
        v-model="selectedStatus"
        size="default"
      >
        <el-radio-button value="">
          全部
        </el-radio-button>
        <el-radio-button value="open">
          待处理
        </el-radio-button>
        <el-radio-button value="pending">
          处理中
        </el-radio-button>
        <el-radio-button value="resolved">
          已解决
        </el-radio-button>
        <el-radio-button value="closed">
          已关闭
        </el-radio-button>
      </el-radio-group>
    </div>

    <!-- 加载状态 -->
    <div
      v-if="loading"
      class="loading-state"
    >
      <el-icon class="loading-icon">
        <Loading />
      </el-icon>
      <p>加载工单列表...</p>
    </div>

    <!-- 空状态 -->
    <el-empty
      v-else-if="filteredTickets.length === 0"
      description="暂无工单"
    >
      <el-button
        type="primary"
        @click="createTicket"
      >
        创建工单
      </el-button>
    </el-empty>

    <!-- 工单列表 -->
    <div
      v-else
      class="tickets-list"
    >
      <div 
        v-for="ticket in filteredTickets" 
        :key="ticket.id"
        class="ticket-card"
        @click="viewTicket(ticket)"
      >
        <div class="ticket-header">
          <span class="ticket-id">#{{ ticket.id }}</span>
          <el-tag
            :type="getStatusType(ticket.status)"
            size="small"
          >
            {{ getStatusLabel(ticket.status) }}
          </el-tag>
        </div>

        <h3 class="ticket-subject">
          {{ ticket.subject }}
        </h3>

        <div class="ticket-meta">
          <span class="meta-item">
            <el-icon><Calendar /></el-icon>
            {{ formatDate(ticket.created_at) }}
          </span>
          <span class="meta-item">
            <el-icon><ChatDotRound /></el-icon>
            {{ ticket.message_count || 0 }} 条回复
          </span>
          <el-tag 
            v-if="ticket.priority !== 'normal'" 
            :type="getPriorityType(ticket.priority)"
            size="small"
          >
            {{ getPriorityLabel(ticket.priority) }}
          </el-tag>
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
        @current-change="loadTickets"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, Loading, Calendar, ChatDotRound, ArrowRight } from '@element-plus/icons-vue'
import { usePortalTicketsStore } from '@/stores/portalTickets'

const router = useRouter()
const ticketsStore = usePortalTicketsStore()

// 状态
const loading = ref(false)
const selectedStatus = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

// 计算属性
const filteredTickets = computed(() => {
  let list = ticketsStore.tickets
  if (selectedStatus.value) {
    list = list.filter(t => t.status === selectedStatus.value)
  }
  return list
})

// 方法
function getStatusType(status) {
  const types = {
    open: 'warning',
    pending: 'primary',
    resolved: 'success',
    closed: 'info'
  }
  return types[status] || 'info'
}

function getStatusLabel(status) {
  const labels = {
    open: '待处理',
    pending: '处理中',
    resolved: '已解决',
    closed: '已关闭'
  }
  return labels[status] || status
}

function getPriorityType(priority) {
  const types = {
    low: 'info',
    normal: '',
    high: 'warning',
    urgent: 'danger'
  }
  return types[priority] || ''
}

function getPriorityLabel(priority) {
  const labels = {
    low: '低',
    normal: '普通',
    high: '高',
    urgent: '紧急'
  }
  return labels[priority] || priority
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

function createTicket() {
  router.push('/user/tickets/create')
}

function viewTicket(ticket) {
  router.push(`/user/tickets/${ticket.id}`)
}

async function loadTickets() {
  loading.value = true
  try {
    await ticketsStore.fetchTickets({
      page: currentPage.value,
      page_size: pageSize.value,
      status: selectedStatus.value || undefined
    })
    total.value = ticketsStore.total
  } catch (error) {
    ElMessage.error('加载工单列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadTickets()
})
</script>

<style scoped>
.tickets-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

/* 筛选栏 */
.filter-bar {
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

/* 工单列表 */
.tickets-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ticket-card {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 20px;
  background: var(--color-bg-card);
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: all 0.3s;
}

.ticket-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  transform: translateX(4px);
}

.ticket-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.ticket-id {
  font-size: 13px;
  color: #909399;
  font-family: monospace;
}

.ticket-subject {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px 0;
  padding-right: 40px;
}

.ticket-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #909399;
}

.arrow-icon {
  position: absolute;
  top: 50%;
  right: 20px;
  transform: translateY(-50%);
  font-size: 16px;
  color: #c0c4cc;
}

/* 分页 */
.pagination {
  display: flex;
  justify-content: center;
  margin-top: 24px;
}

/* 响应式 */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    gap: 16px;
  }
}
</style>
