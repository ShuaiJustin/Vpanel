<template>
  <div class="admin-trials-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          试用管理
        </h1>
        <p class="page-subtitle">
          查看试用数据、执行过期检查并为指定用户授予试用
        </p>
      </div>
      <div class="page-actions">
        <el-button
          type="primary"
          @click="showGrantDialog"
        >
          <el-icon class="el-icon--left">
            <Plus />
          </el-icon>
          授予试用
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">总试用数</span>
        <strong class="overview-value">{{ stats.total_trials || 0 }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">活跃试用</span>
        <strong class="overview-value is-success">{{ stats.active_trials || 0 }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">已过期</span>
        <strong class="overview-value is-warning">{{ stats.expired_trials || 0 }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">已转化</span>
        <strong class="overview-value is-primary">{{ stats.converted_trials || 0 }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">转化率</span>
        <strong class="overview-value is-success">{{ (stats.conversion_rate || 0).toFixed(1) }}%</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-actions">
        <span class="toolbar-summary">
          当前试用功能{{ trialConfig.enabled ? '已启用' : '已禁用' }}，默认时长 {{ trialConfig.duration || 0 }} 天
        </span>
        <el-button
          type="warning"
          :loading="expiring"
          @click="expireTrials"
        >
          <el-icon class="el-icon--left">
            <Timer />
          </el-icon>
          手动过期检查
        </el-button>
        <el-button @click="refreshAll">
          <el-icon class="el-icon--left">
            <Refresh />
          </el-icon>
          刷新统计
        </el-button>
      </div>
    </div>

    <div class="detail-grid">
      <el-card
        shadow="never"
        class="config-card"
      >
        <template #header>
          <div class="card-header">
            <span>试用配置</span>
          </div>
        </template>
        <el-descriptions
          :column="isMobile ? 1 : 2"
          border
        >
          <el-descriptions-item label="试用功能">
            <span :class="['metric-pill', trialConfig.enabled ? 'is-success' : 'is-danger']">
              {{ trialConfig.enabled ? '已启用' : '已禁用' }}
            </span>
          </el-descriptions-item>
          <el-descriptions-item label="试用时长">
            {{ trialConfig.duration || 0 }} 天
          </el-descriptions-item>
          <el-descriptions-item label="流量限制">
            {{ formatTraffic(trialConfig.traffic_limit) }}
          </el-descriptions-item>
          <el-descriptions-item label="邮箱验证">
            <span :class="['metric-pill', trialConfig.require_email_verify ? 'is-warning' : 'is-muted']">
              {{ trialConfig.require_email_verify ? '需要验证' : '无需验证' }}
            </span>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <el-card
        shadow="never"
        class="search-card"
      >
        <template #header>
          <div class="card-header">
            <span>查询用户试用</span>
          </div>
        </template>
        <el-form
          :inline="!isMobile"
          class="trial-search-form"
        >
          <el-form-item label="用户ID">
            <el-input
              v-model="searchUserId"
              placeholder="输入用户ID"
              clearable
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              :loading="searching"
              @click="searchUserTrial"
            >
              查询
            </el-button>
          </el-form-item>
        </el-form>
        <div
          v-if="searchResult"
          class="search-result"
        >
          <el-descriptions
            :column="isMobile ? 1 : 2"
            border
          >
            <el-descriptions-item label="用户ID">
              {{ searchResult.user_id }}
            </el-descriptions-item>
            <el-descriptions-item label="状态">
              <span :class="['metric-pill', getStatusPillClass(searchResult.status)]">{{ getStatusLabel(searchResult.status) }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="开始时间">
              {{ formatTime(searchResult.start_at) }}
            </el-descriptions-item>
            <el-descriptions-item label="到期时间">
              {{ formatTime(searchResult.expire_at) }}
            </el-descriptions-item>
            <el-descriptions-item label="剩余天数">
              {{ searchResult.remaining_days }} 天
            </el-descriptions-item>
            <el-descriptions-item label="流量使用">
              {{ formatTraffic(searchResult.traffic_used) }} / {{ formatTraffic(searchResult.traffic_limit) }}
            </el-descriptions-item>
          </el-descriptions>
        </div>
        <el-empty
          v-else-if="searchResult === null && searchUserId"
          description="该用户没有试用记录"
        />
      </el-card>
    </div>

    <el-dialog
      v-model="grantDialogVisible"
      title="授予试用"
      :width="isMobile ? 'calc(100vw - 24px)' : '420px'"
    >
      <el-form
        ref="grantFormRef"
        :model="grantForm"
        :rules="grantRules"
        :label-width="isMobile ? '76px' : '80px'"
      >
        <el-form-item
          label="用户ID"
          prop="user_id"
        >
          <el-input-number
            v-model="grantForm.user_id"
            :min="1"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item
          label="试用天数"
          prop="duration"
        >
          <el-input-number
            v-model="grantForm.duration"
            :min="1"
            :max="365"
          />
          <span class="form-unit">天</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="grantDialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="granting"
          @click="submitGrant"
        >
          授予
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Timer, Refresh } from '@element-plus/icons-vue'
import api from '@/api'
import { useViewport } from '@/composables/useViewport'

const { isMobile } = useViewport()

const stats = reactive({ total_trials: 0, active_trials: 0, expired_trials: 0, converted_trials: 0, conversion_rate: 0 })
const trialConfig = reactive({ enabled: false, duration: 7, traffic_limit: 0, require_email_verify: false })
const searchUserId = ref('')
const searchResult = ref(undefined)
const searching = ref(false)
const expiring = ref(false)
const grantDialogVisible = ref(false)
const granting = ref(false)
const grantFormRef = ref(null)
const grantForm = reactive({ user_id: null, duration: 7 })
const grantRules = {
  user_id: [{ required: true, message: '请输入用户ID', trigger: 'blur' }],
  duration: [{ required: true, message: '请输入试用天数', trigger: 'blur' }]
}

const formatTraffic = (bytes) => {
  if (!bytes || bytes === 0) return '无限制'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i += 1
  }
  return `${size.toFixed(0)} ${units[i]}`
}

const formatTime = (time) => time ? new Date(time).toLocaleString('zh-CN') : '-'
const getStatusType = (status) => ({ active: 'success', expired: 'warning', converted: 'primary' }[status] || 'info')
const getStatusLabel = (status) => ({ active: '活跃', expired: '已过期', converted: '已转化' }[status] || status)
const getStatusPillClass = (status) => {
  const type = getStatusType(status)
  if (type === 'success') return 'is-success'
  if (type === 'warning') return 'is-warning'
  if (type === 'primary') return 'is-primary'
  return 'is-muted'
}

const fetchStats = async () => {
  try {
    const response = await api.get('/admin/trials/stats')
    if (response?.stats) Object.assign(stats, response.stats)
  } catch (error) {
    console.error('Failed to fetch trial stats:', error)
  }
}

const fetchConfig = async () => {
  try {
    const response = await api.get('/trial')
    if (response?.trial_config) Object.assign(trialConfig, response.trial_config)
  } catch (error) {
    console.error('Failed to fetch trial config:', error)
  }
}

const refreshAll = async () => {
  await Promise.all([fetchStats(), fetchConfig()])
}

const searchUserTrial = async () => {
  if (!searchUserId.value) {
    ElMessage.warning('请输入用户ID')
    return
  }
  searching.value = true
  try {
    const response = await api.get(`/admin/trials/user/${searchUserId.value}`)
    searchResult.value = response?.trial || null
  } catch (error) {
    if (error.status === 404) searchResult.value = null
    else ElMessage.error(error.message || '查询失败')
  } finally {
    searching.value = false
  }
}

const expireTrials = async () => {
  try {
    await ElMessageBox.confirm('确定要执行试用过期检查吗？', '确认', { type: 'warning' })
    expiring.value = true
    const response = await api.post('/admin/trials/expire')
    ElMessage.success(`已过期 ${response?.expired_count || 0} 个试用`)
    fetchStats()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(error.message || '操作失败')
  } finally {
    expiring.value = false
  }
}

const showGrantDialog = () => {
  grantForm.user_id = null
  grantForm.duration = trialConfig.duration || 7
  grantDialogVisible.value = true
}

const submitGrant = async () => {
  await grantFormRef.value.validate()
  granting.value = true
  try {
    await api.post('/admin/trials/grant', { user_id: grantForm.user_id, duration: grantForm.duration })
    ElMessage.success('试用已授予')
    grantDialogVisible.value = false
    fetchStats()
  } catch (error) {
    ElMessage.error(error.message || '授予失败')
  } finally {
    granting.value = false
  }
}

onMounted(() => {
  fetchStats()
  fetchConfig()
})
</script>

<style scoped>
.admin-trials-page {
  padding: 20px;
}

.trial-search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.trial-search-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.form-unit {
  margin-left: 8px;
  font-size: 12px;
  color: #64748b;
}

@media (max-width: 768px) {
  .admin-trials-page {
    padding: 12px;
  }
}
</style>
