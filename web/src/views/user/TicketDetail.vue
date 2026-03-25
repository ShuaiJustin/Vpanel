<template>
  <div class="ticket-detail-page">
    <!-- 返回按钮 -->
    <div class="back-bar">
      <el-button
        link
        @click="goBack"
      >
        <el-icon><ArrowLeft /></el-icon>
        返回工单列表
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
      <p>加载工单详情...</p>
    </div>

    <!-- 工单不存在 -->
    <el-empty
      v-else-if="!ticket"
      description="工单不存在或已删除"
    />

    <!-- 工单内容 -->
    <template v-else>
      <!-- 工单信息 -->
      <el-card
        class="ticket-info-card"
        shadow="never"
      >
        <div class="ticket-header">
          <div class="header-left">
            <span class="ticket-id">#{{ ticket.id }}</span>
            <el-tag
              :type="getStatusType(ticket.status)"
              size="default"
            >
              {{ getStatusLabel(ticket.status) }}
            </el-tag>
            <el-tag 
              v-if="ticket.priority !== 'normal'" 
              :type="getPriorityType(ticket.priority)"
              size="small"
            >
              {{ getPriorityLabel(ticket.priority) }}
            </el-tag>
          </div>
          <div class="header-right">
            <el-button 
              v-if="canClose" 
              type="danger" 
              plain 
              size="small"
              @click="closeTicket"
            >
              关闭工单
            </el-button>
          </div>
        </div>

        <h1 class="ticket-subject">
          {{ ticket.subject }}
        </h1>

        <div class="ticket-meta">
          <span class="meta-item">
            <el-icon><Calendar /></el-icon>
            创建于 {{ formatDate(ticket.created_at) }}
          </span>
          <span class="meta-item">
            <el-icon><Clock /></el-icon>
            更新于 {{ formatDate(ticket.updated_at) }}
          </span>
        </div>
      </el-card>

      <!-- 消息列表 -->
      <div class="messages-section">
        <div 
          v-for="message in ticket.messages" 
          :key="message.id"
          class="message-item"
          :class="{ admin: message.is_admin }"
        >
          <div class="message-avatar">
            <el-avatar :size="40">
              {{ message.is_admin ? '客服' : '我' }}
            </el-avatar>
          </div>
          <div class="message-content">
            <div class="message-header">
              <span class="message-author">
                {{ message.is_admin ? '客服' : '我' }}
              </span>
              <span class="message-time">{{ formatDate(message.created_at) }}</span>
            </div>
            <div
              class="message-body"
            >
              {{ message.content }}
            </div>
            <div
              v-if="message.attachments && message.attachments.length"
              class="message-attachments"
            >
              <a 
                v-for="(attachment, index) in message.attachments" 
                :key="index"
                :href="attachment.url"
                target="_blank"
                class="attachment-link"
              >
                <el-icon><Document /></el-icon>
                {{ attachment.name }}
              </a>
            </div>
          </div>
        </div>
      </div>

      <!-- 回复表单 -->
      <el-card
        v-if="canReply"
        class="reply-card"
        shadow="never"
      >
        <h3 class="reply-title">
          回复工单
        </h3>
        <el-form
          ref="replyFormRef"
          :model="replyForm"
          :rules="replyRules"
        >
          <el-form-item prop="content">
            <el-input
              v-model="replyForm.content"
              type="textarea"
              :rows="4"
              placeholder="请输入回复内容..."
              maxlength="2000"
              show-word-limit
            />
          </el-form-item>

          <el-form-item>
            <el-upload
              v-model:file-list="replyForm.attachments"
              :auto-upload="false"
              :limit="3"
              accept=".jpg,.jpeg,.png,.gif,.pdf,.doc,.docx"
            >
              <el-button size="small">
                <el-icon><Upload /></el-icon>
                添加附件
              </el-button>
              <template #tip>
                <div class="upload-tip">
                  支持 jpg/png/gif/pdf/doc 格式，最多 3 个文件
                </div>
              </template>
            </el-upload>
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              :loading="submitting"
              @click="submitReply"
            >
              提交回复
            </el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 已关闭提示 -->
      <el-alert
        v-else
        type="info"
        :closable="false"
        show-icon
        class="closed-alert"
      >
        该工单已关闭，无法继续回复。如需帮助，请创建新工单。
      </el-alert>
    </template>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  ArrowLeft, Loading, Calendar, Clock, Document, Upload 
} from '@element-plus/icons-vue'
import { usePortalTicketsStore } from '@/stores/portalTickets'

const router = useRouter()
const route = useRoute()
const ticketsStore = usePortalTicketsStore()

// 表单引用
const replyFormRef = ref(null)

// 状态
const loading = ref(false)
const submitting = ref(false)
const ticket = ref(null)

// 回复表单
const replyForm = reactive({
  content: '',
  attachments: []
})

const replyRules = {
  content: [
    { required: true, message: '请输入回复内容', trigger: 'blur' },
    { min: 10, message: '回复内容至少 10 个字符', trigger: 'blur' }
  ]
}

// 计算属性
const canReply = computed(() => {
  return ticket.value && ticket.value.status !== 'closed'
})

const canClose = computed(() => {
  return ticket.value && ['open', 'pending', 'resolved'].includes(ticket.value.status)
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

function goBack() {
  router.push('/user/tickets')
}

async function loadTicket() {
  const id = route.params.id
  if (!id) {
    router.push('/user/tickets')
    return
  }

  loading.value = true
  try {
    ticket.value = await ticketsStore.fetchTicket(id)
  } catch (error) {
    ElMessage.error('加载工单失败')
    ticket.value = null
  } finally {
    loading.value = false
  }
}

async function submitReply() {
  if (!replyFormRef.value) return

  try {
    await replyFormRef.value.validate()
    submitting.value = true

    await ticketsStore.replyTicket(ticket.value.id, {
      content: replyForm.content
    })

    ElMessage.success('回复已提交')
    replyForm.content = ''
    replyForm.attachments = []
    
    // 重新加载工单
    await loadTicket()
  } catch (error) {
    if (error !== false) {
      ElMessage.error(error.message || '提交失败')
    }
  } finally {
    submitting.value = false
  }
}

async function closeTicket() {
  try {
    await ElMessageBox.confirm(
      '确定要关闭此工单吗？关闭后将无法继续回复。',
      '关闭工单',
      {
        confirmButtonText: '确定关闭',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await ticketsStore.closeTicket(ticket.value.id)
    ElMessage.success('工单已关闭')
    await loadTicket()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('操作失败')
    }
  }
}

onMounted(() => {
  loadTicket()
})
</script>

<style scoped>
.ticket-detail-page {
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
  color: var(--color-text-secondary);
}

.loading-icon {
  font-size: 32px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 工单信息卡片 */
.ticket-info-card {
  margin-bottom: 20px;
  border-radius: 8px;
}

.ticket-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ticket-id {
  font-size: 14px;
  color: var(--color-text-secondary);
  font-family: monospace;
}

.ticket-subject {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px 0;
}

.ticket-meta {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* 消息列表 */
.messages-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 20px;
}

.message-item {
  display: flex;
  gap: 12px;
}

.message-item.admin {
  flex-direction: row-reverse;
}

.message-avatar {
  flex-shrink: 0;
}

.message-content {
  flex: 1;
  max-width: 80%;
  background: var(--color-bg-card);
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.message-item.admin .message-content {
  background: rgba(64, 158, 255, 0.12);
}

.message-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.message-author {
  font-weight: 500;
  color: var(--color-text-primary);
}

.message-time {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.message-body {
  font-size: 14px;
  color: var(--color-text-regular);
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.message-attachments {
  margin-top: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.attachment-link {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  background: var(--color-bg-page);
  border-radius: 4px;
  font-size: 13px;
  color: #409eff;
  text-decoration: none;
}

.attachment-link:hover {
  background: rgba(64, 158, 255, 0.12);
}

/* 回复表单 */
.reply-card {
  border-radius: 8px;
}

.reply-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 16px 0;
}

.upload-tip {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-top: 4px;
}

/* 已关闭提示 */
.closed-alert {
  border-radius: 8px;
}

/* 响应式 */
@media (max-width: 768px) {
  .message-content {
    max-width: 90%;
  }

  .ticket-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
}
</style>
