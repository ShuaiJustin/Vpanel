<template>
  <el-card
    v-if="showCard"
    class="trial-card"
    shadow="never"
  >
    <template #header>
      <div class="card-header">
        <span>
          <el-icon><Timer /></el-icon>
          试用状态
        </span>
      </div>
    </template>

    <div
      v-loading="loading"
      class="trial-content"
    >
      <!-- 试用中状态 -->
      <template v-if="hasTrial && trialStatus === 'active'">
        <div class="trial-status active">
          <el-icon class="status-icon">
            <Clock />
          </el-icon>
          <div class="status-info">
            <h4>试用中</h4>
            <p>剩余 {{ trial?.remaining_days || 0 }} 天</p>
          </div>
        </div>
        
        <div class="trial-details">
          <div class="detail-item">
            <span class="label">开始时间</span>
            <span class="value">{{ formatDate(trial?.start_at) }}</span>
          </div>
          <div class="detail-item">
            <span class="label">到期时间</span>
            <span class="value">{{ formatDate(trial?.expire_at) }}</span>
          </div>
          <div class="detail-item">
            <span class="label">流量使用</span>
            <span class="value">
              {{ formatTraffic(trial?.traffic_used) }} / {{ formatTraffic(trial?.traffic_limit) }}
            </span>
          </div>
        </div>

        <el-progress 
          :percentage="trafficPercent" 
          :color="trafficColor"
          :stroke-width="8"
          class="traffic-progress"
        />

        <el-button
          type="primary"
          class="action-btn"
          @click="goToPlans"
        >
          <el-icon><ShoppingCart /></el-icon>
          升级为正式套餐
        </el-button>
      </template>

      <!-- 试用已过期 -->
      <template v-else-if="hasTrial && trialStatus === 'expired'">
        <div class="trial-status expired">
          <el-icon class="status-icon">
            <WarningFilled />
          </el-icon>
          <div class="status-info">
            <h4>试用已过期</h4>
            <p>您的试用期已结束</p>
          </div>
        </div>

        <el-button
          type="primary"
          class="action-btn"
          @click="goToPlans"
        >
          <el-icon><ShoppingCart /></el-icon>
          购买套餐
        </el-button>
      </template>

      <!-- 试用已转化 -->
      <template v-else-if="hasTrial && trialStatus === 'converted'">
        <div class="trial-status converted">
          <el-icon class="status-icon">
            <CircleCheck />
          </el-icon>
          <div class="status-info">
            <h4>已升级为正式用户</h4>
            <p>感谢您的支持！</p>
          </div>
        </div>
      </template>

      <!-- 可以激活试用 -->
      <template v-else-if="canActivate">
        <div class="trial-status available">
          <el-icon class="status-icon">
            <Present />
          </el-icon>
          <div class="status-info">
            <h4>免费试用</h4>
            <p>{{ trialConfig?.duration || 7 }} 天免费体验</p>
          </div>
        </div>

        <div class="trial-benefits">
          <div class="benefit-item">
            <el-icon><Check /></el-icon>
            <span>{{ formatTraffic(trialConfig?.traffic_limit) }} 流量</span>
          </div>
          <div class="benefit-item">
            <el-icon><Check /></el-icon>
            <span>全部节点访问</span>
          </div>
          <div class="benefit-item">
            <el-icon><Check /></el-icon>
            <span>无需绑定支付方式</span>
          </div>
        </div>

        <el-button 
          type="success" 
          class="action-btn"
          :loading="activating"
          @click="handleActivate"
        >
          <el-icon><VideoPlay /></el-icon>
          立即开始试用
        </el-button>
      </template>

      <!-- 不能激活试用 -->
      <template v-else-if="!canActivate && message">
        <div class="trial-status unavailable">
          <el-icon class="status-icon">
            <InfoFilled />
          </el-icon>
          <div class="status-info">
            <h4>试用不可用</h4>
            <p>{{ message }}</p>
          </div>
        </div>
      </template>
    </div>
  </el-card>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { 
  Timer, Clock, WarningFilled, CircleCheck, Present, 
  Check, VideoPlay, ShoppingCart, InfoFilled 
} from '@element-plus/icons-vue'
import { getTrialStatus, activateTrial } from '@/api/modules/portal/trial'

const router = useRouter()

// State
const loading = ref(false)
const activating = ref(false)
const hasTrial = ref(false)
const canActivate = ref(false)
const message = ref('')
const trial = ref(null)
const trialConfig = ref(null)

// Computed
const showCard = computed(() => {
  // Show card if trial is enabled and user either has trial or can activate
  return trialConfig.value?.enabled && (hasTrial.value || canActivate.value)
})

const trialStatus = computed(() => trial.value?.status || '')

const trafficPercent = computed(() => {
  if (!trial.value?.traffic_limit) return 0
  return Math.min(100, Math.round((trial.value.traffic_used / trial.value.traffic_limit) * 100))
})

const trafficColor = computed(() => {
  if (trafficPercent.value >= 90) return '#f56c6c'
  if (trafficPercent.value >= 70) return '#e6a23c'
  return '#67c23a'
})

// Methods
function formatDate(dateStr) {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
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

function goToPlans() {
  router.push('/user/plans')
}

async function fetchTrialStatus() {
  loading.value = true
  try {
    const response = await getTrialStatus()
    const data = response.data || response
    hasTrial.value = data.has_trial || false
    canActivate.value = data.can_activate || false
    message.value = data.message || ''
    trial.value = data.trial || null
    trialConfig.value = data.trial_config || null
  } catch (error) {
    console.error('Failed to fetch trial status:', error)
  } finally {
    loading.value = false
  }
}

async function handleActivate() {
  activating.value = true
  try {
    const response = await activateTrial()
    const data = response.data || response
    trial.value = data.trial
    hasTrial.value = true
    canActivate.value = false
    ElMessage.success('试用已激活！')
  } catch (error) {
    ElMessage.error(error.message || '激活失败')
  } finally {
    activating.value = false
  }
}

onMounted(() => {
  fetchTrialStatus()
})
</script>

<style scoped>
.trial-card {
  border-radius: 8px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.trial-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.trial-status {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px;
  border-radius: 8px;
}

.trial-status.active {
  background: rgba(64, 158, 255, 0.1);
}

.trial-status.expired {
  background: rgba(245, 108, 108, 0.1);
}

.trial-status.converted {
  background: rgba(103, 194, 58, 0.1);
}

.trial-status.available {
  background: rgba(103, 194, 58, 0.1);
}

.trial-status.unavailable {
  background: rgba(144, 147, 153, 0.1);
}

.status-icon {
  font-size: 32px;
  flex-shrink: 0;
}

.trial-status.active .status-icon {
  color: #409eff;
}

.trial-status.expired .status-icon {
  color: #f56c6c;
}

.trial-status.converted .status-icon {
  color: #67c23a;
}

.trial-status.available .status-icon {
  color: #67c23a;
}

.trial-status.unavailable .status-icon {
  color: #909399;
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

.trial-details {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px 16px;
  background: var(--color-bg-page);
  border-radius: 8px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 120px;
}

.detail-item .label {
  font-size: 12px;
  color: #909399;
}

.detail-item .value {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.traffic-progress {
  margin: 0;
}

.trial-benefits {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 16px;
  background: var(--color-bg-page);
  border-radius: 8px;
}

.benefit-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #606266;
}

.benefit-item .el-icon {
  color: #67c23a;
}

.action-btn {
  width: 100%;
}

/* 深色模式适配 */
.dark .status-info h4 {
  color: var(--color-text-primary);
}

.dark .detail-item .value {
  color: var(--color-text-primary);
}

.dark .trial-details {
  background: var(--color-border-light);
}

.dark .trial-benefits {
  background: var(--color-border-light);
}
</style>
