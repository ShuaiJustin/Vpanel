<template>
  <el-card
    class="pause-card"
    shadow="never"
  >
    <template #header>
      <div class="card-header">
        <span>
          <el-icon><VideoPause /></el-icon>
          订阅暂停
        </span>
      </div>
    </template>

    <div
      v-loading="pauseStore.loading"
      class="pause-content"
    >
      <!-- 已暂停状态 -->
      <template v-if="pauseStore.isPaused">
        <div class="pause-status paused">
          <el-icon class="status-icon">
            <Clock />
          </el-icon>
          <div class="status-info">
            <h4>订阅已暂停</h4>
            <p>暂停于 {{ formatDate(pauseStore.activePause?.paused_at) }}</p>
            <p class="auto-resume">
              将于 {{ formatDate(pauseStore.autoResumeAt) }} 自动恢复
            </p>
          </div>
        </div>
        
        <div class="pause-details">
          <div class="detail-item">
            <span class="label">剩余天数</span>
            <span class="value">{{ formatRemainingDays(pauseStore.activePause?.remaining_days) }}</span>
          </div>
          <div class="detail-item">
            <span class="label">剩余流量</span>
            <span class="value">{{ formatTraffic(pauseStore.activePause?.remaining_traffic) }}</span>
          </div>
        </div>

        <el-button 
          type="primary" 
          class="action-btn"
          :loading="resuming"
          @click="handleResume"
        >
          <el-icon><VideoPlay /></el-icon>
          恢复订阅
        </el-button>
      </template>

      <!-- 未暂停状态 -->
      <template v-else>
        <div
          class="pause-status"
          :class="noSubscription ? 'inactive' : 'active'"
        >
          <el-icon class="status-icon">
            <CircleCloseFilled v-if="noSubscription" />
            <CircleCheck v-else />
          </el-icon>
          <div class="status-info">
            <h4>{{ noSubscription ? '无有效订阅' : '订阅正常' }}</h4>
            <p v-if="pauseStore.canPause">
              本周期剩余 {{ pauseStore.remainingPauses }} 次暂停机会
            </p>
            <p
              v-else
              class="cannot-pause"
            >
              {{ pauseStore.cannotPauseReason }}
            </p>
          </div>
        </div>

        <el-alert
          v-if="pauseStore.canPause"
          type="info"
          :closable="false"
          show-icon
          class="pause-tip"
        >
          <template #title>
            暂停后，您的订阅将冻结，剩余时间和流量将保留。
            最长可暂停 {{ pauseStore.maxDuration }} 天。
          </template>
        </el-alert>

        <el-button 
          type="warning" 
          class="action-btn"
          :disabled="!pauseStore.canPause"
          @click="showPauseDialog = true"
        >
          <el-icon><VideoPause /></el-icon>
          暂停订阅
        </el-button>
      </template>
    </div>

    <!-- 暂停确认对话框 -->
    <el-dialog
      v-model="showPauseDialog"
      title="确认暂停订阅"
      width="400px"
    >
      <el-alert
        type="warning"
        :closable="false"
        show-icon
      >
        <template #title>
          暂停后：
        </template>
        <template #default>
          <ul class="pause-warning-list">
            <li>您将无法使用代理服务</li>
            <li>剩余时间和流量将被保留</li>
            <li>最长暂停 {{ pauseStore.maxDuration }} 天后自动恢复</li>
            <li>本周期剩余 {{ pauseStore.remainingPauses }} 次暂停机会</li>
          </ul>
        </template>
      </el-alert>
      <template #footer>
        <el-button @click="showPauseDialog = false">
          取消
        </el-button>
        <el-button
          type="warning"
          :loading="pausing"
          @click="handlePause"
        >
          确认暂停
        </el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { 
  VideoPause, VideoPlay, Clock, CircleCheck, CircleCloseFilled
} from '@element-plus/icons-vue'
import { usePauseStore } from '@/stores/pause'
import { extractErrorMessage } from '@/utils/entitlement'

const pauseStore = usePauseStore()

// State
const showPauseDialog = ref(false)
const pausing = ref(false)
const resuming = ref(false)
const noSubscriptionReason = '当前无有效订阅'
const noSubscription = computed(() => !pauseStore.isPaused && pauseStore.cannotPauseReason === noSubscriptionReason)

// Methods
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

function formatTraffic(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(2)} ${units[i]}`
}

function formatRemainingDays(days) {
  if (typeof days === 'number' && days < 0) return '永久'
  return `${days || 0} 天`
}

async function handlePause() {
  pausing.value = true
  try {
    await pauseStore.pause()
    showPauseDialog.value = false
    ElMessage.success('订阅已暂停')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '暂停失败')
  } finally {
    pausing.value = false
  }
}

async function handleResume() {
  resuming.value = true
  try {
    await pauseStore.resume()
    ElMessage.success('订阅已恢复')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '恢复失败')
  } finally {
    resuming.value = false
  }
}

onMounted(() => {
  pauseStore.fetchPauseStatus().catch(error => {
    console.warn('加载暂停状态失败:', error)
  })
})
</script>

<style scoped>
.pause-card {
  border-radius: 8px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pause-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.pause-status {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px;
  border-radius: 8px;
}

.pause-status.paused {
  background: rgba(230, 162, 60, 0.1);
}

.pause-status.active {
  background: rgba(103, 194, 58, 0.1);
}

.pause-status.inactive {
  background: rgba(245, 108, 108, 0.1);
}

.status-icon {
  font-size: 32px;
  flex-shrink: 0;
}

.pause-status.paused .status-icon {
  color: #e6a23c;
}

.pause-status.active .status-icon {
  color: #67c23a;
}

.pause-status.inactive .status-icon {
  color: #f56c6c;
}

.status-info h4 {
  margin: 0 0 4px 0;
  font-size: 16px;
  color: var(--color-text-primary);
}

.status-info p {
  margin: 0;
  font-size: 13px;
  color: #909399;
}

.auto-resume {
  margin-top: 4px !important;
  color: #e6a23c !important;
  font-weight: 500;
}

.cannot-pause {
  color: #f56c6c !important;
}

.pause-details {
  display: flex;
  gap: 24px;
  padding: 12px 16px;
  background: var(--color-bg-page);
  border-radius: 8px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-item .label {
  font-size: 12px;
  color: #909399;
}

.detail-item .value {
  font-size: 16px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.pause-tip {
  margin: 0;
}

.action-btn {
  width: 100%;
}

.pause-warning-list {
  margin: 8px 0 0 0;
  padding-left: 20px;
  font-size: 13px;
  color: #606266;
}

.pause-warning-list li {
  margin: 4px 0;
}

/* 深色模式适配 */
:global(.dark) .status-info h4 {
  color: var(--color-text-primary);
}

:global(.dark) .detail-item .value {
  color: var(--color-text-primary);
}

:global(.dark) .pause-details {
  background: var(--color-border-light);
}
</style>
