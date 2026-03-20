<template>
  <div class="admin-trials-page">
    <div class="page-header">
      <h1 class="page-title">试用管理</h1>
      <el-button type="primary" @click="showGrantDialog">
        <el-icon><Plus /></el-icon>
        授予试用
      </el-button>
    </div>
    <div class="stats-row">
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ stats.total_trials || 0 }}</div>
        <div class="stat-label">总试用数</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--active">
        <div class="stat-value">{{ stats.active_trials || 0 }}</div>
        <div class="stat-label">活跃试用</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--expired">
        <div class="stat-value">{{ stats.expired_trials || 0 }}</div>
        <div class="stat-label">已过期</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--converted">
        <div class="stat-value">{{ stats.converted_trials || 0 }}</div>
        <div class="stat-label">已转化</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--rate">
        <div class="stat-value">{{ (stats.conversion_rate || 0).toFixed(1) }}%</div>
        <div class="stat-label">转化率</div>
      </el-card>
    </div>
    <el-card shadow="never" class="actions-card">
      <el-button type="warning" @click="expireTrials" :loading="expiring">
        <el-icon><Timer /></el-icon>
        手动过期检查
      </el-button>
      <el-button @click="fetchStats">
        <el-icon><Refresh /></el-icon>
        刷新统计
      </el-button>
    </el-card>
    <el-card shadow="never" class="config-card">
      <template #header><span>试用配置</span></template>
      <el-descriptions :column="4" border size="small">
        <el-descriptions-item label="试用功能">
          <el-tag :type="trialConfig.enabled ? 'success' : 'danger'" size="small">
            {{ trialConfig.enabled ? '已启用' : '已禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="试用时长">{{ trialConfig.duration || 0 }} 天</el-descriptions-item>
        <el-descriptions-item label="流量限制">{{ formatTraffic(trialConfig.traffic_limit) }}</el-descriptions-item>
        <el-descriptions-item label="邮箱验证">
          <el-tag :type="trialConfig.require_email_verify ? 'warning' : 'info'" size="small">
            {{ trialConfig.require_email_verify ? '需要' : '不需要' }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
    <el-card shadow="never" class="search-card">
      <template #header><span>查询用户试用</span></template>
      <el-form :inline="true">
        <el-form-item label="用户ID">
          <el-input v-model="searchUserId" placeholder="输入用户ID" clearable />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="searchUserTrial" :loading="searching">查询</el-button>
        </el-form-item>
      </el-form>
      <div v-if="searchResult" class="search-result">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="用户ID">{{ searchResult.user_id }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusType(searchResult.status)" size="small">{{ getStatusLabel(searchResult.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ formatTime(searchResult.start_at) }}</el-descriptions-item>
          <el-descriptions-item label="到期时间">{{ formatTime(searchResult.expire_at) }}</el-descriptions-item>
          <el-descriptions-item label="剩余天数">{{ searchResult.remaining_days }} 天</el-descriptions-item>
          <el-descriptions-item label="流量使用">{{ formatTraffic(searchResult.traffic_used) }} / {{ formatTraffic(searchResult.traffic_limit) }}</el-descriptions-item>
        </el-descriptions>
      </div>
      <el-empty v-else-if="searchResult === null && searchUserId" description="该用户没有试用记录" />
    </el-card>
    <el-dialog v-model="grantDialogVisible" title="授予试用" width="400px">
      <el-form :model="grantForm" :rules="grantRules" ref="grantFormRef" label-width="80px">
        <el-form-item label="用户ID" prop="user_id">
          <el-input-number v-model="grantForm.user_id" :min="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="试用天数" prop="duration">
          <el-input-number v-model="grantForm.duration" :min="1" :max="365" />
          <span style="margin-left: 8px; color: #909399;">天</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="grantDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="granting" @click="submitGrant">授予</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Timer, Refresh } from '@element-plus/icons-vue'
import api from '@/api'

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
  while (size >= 1024 && i < units.length - 1) { size /= 1024; i++ }
  return size.toFixed(0) + ' ' + units[i]
}
const formatTime = (time) => time ? new Date(time).toLocaleString('zh-CN') : '-'
const getStatusType = (status) => ({ active: 'success', expired: 'warning', converted: 'primary' }[status] || 'info')
const getStatusLabel = (status) => ({ active: '活跃', expired: '已过期', converted: '已转化' }[status] || status)

const fetchStats = async () => {
  try {
    const response = await api.get('/admin/trials/stats')
    if (response?.stats) Object.assign(stats, response.stats)
  } catch (error) { console.error('Failed to fetch trial stats:', error) }
}
const fetchConfig = async () => {
  try {
    const response = await api.get('/trial')
    if (response?.trial_config) Object.assign(trialConfig, response.trial_config)
  } catch (error) { console.error('Failed to fetch trial config:', error) }
}
const searchUserTrial = async () => {
  if (!searchUserId.value) { ElMessage.warning('请输入用户ID'); return }
  searching.value = true
  try {
    const response = await api.get('/admin/trials/user/' + searchUserId.value)
    searchResult.value = response?.trial || null
  } catch (error) {
    if (error.status === 404) searchResult.value = null
    else ElMessage.error(error.message || '查询失败')
  } finally { searching.value = false }
}
const expireTrials = async () => {
  try {
    await ElMessageBox.confirm('确定要执行试用过期检查吗？', '确认', { type: 'warning' })
    expiring.value = true
    const response = await api.post('/admin/trials/expire')
    ElMessage.success('已过期 ' + (response?.expired_count || 0) + ' 个试用')
    fetchStats()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(error.message || '操作失败')
  } finally { expiring.value = false }
}
const showGrantDialog = () => { grantForm.user_id = null; grantForm.duration = trialConfig.duration || 7; grantDialogVisible.value = true }
const submitGrant = async () => {
  await grantFormRef.value.validate()
  granting.value = true
  try {
    await api.post('/admin/trials/grant', { user_id: grantForm.user_id, duration: grantForm.duration })
    ElMessage.success('试用已授予')
    grantDialogVisible.value = false
    fetchStats()
  } catch (error) { ElMessage.error(error.message || '授予失败') }
  finally { granting.value = false }
}
onMounted(() => { fetchStats(); fetchConfig() })
</script>

<style scoped>
.admin-trials-page { padding: 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-title { font-size: 24px; font-weight: 600; margin: 0; }
.stats-row { display: grid; grid-template-columns: repeat(5, 1fr); gap: 16px; margin-bottom: 20px; }
.stat-card { text-align: center; }
.stat-card :deep(.el-card__body) { padding: 20px; }
.stat-value { font-size: 28px; font-weight: 600; color: #303133; }
.stat-label { font-size: 14px; color: #909399; margin-top: 8px; }
.stat-card--active .stat-value { color: #67c23a; }
.stat-card--expired .stat-value { color: #e6a23c; }
.stat-card--converted .stat-value { color: #409eff; }
.stat-card--rate .stat-value { color: #67c23a; }
.actions-card { margin-bottom: 20px; }
.actions-card :deep(.el-card__body) { display: flex; gap: 12px; }
.config-card { margin-bottom: 20px; }
.search-card { margin-bottom: 20px; }
.search-result { margin-top: 16px; }
@media (max-width: 1200px) { .stats-row { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 768px) { .stats-row { grid-template-columns: repeat(2, 1fr); } }
</style>
