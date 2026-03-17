<template>
  <div class="node-detail-page">
    <div class="page-header">
      <div class="header-left">
        <el-button link @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h1 class="page-title">{{ node?.name || '节点详情' }}</h1>
        <el-tag v-if="node" :type="getStatusType(node.status)" size="large">
          {{ getStatusText(node.status) }}
        </el-tag>
      </div>
      <div class="header-actions">
        <el-button @click="refreshData">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" @click="editNode">编辑</el-button>
      </div>
    </div>

    <el-row :gutter="20" v-loading="loading">
      <!-- 基本信息 -->
      <el-col :span="16">
        <el-card shadow="never" class="info-card">
          <template #header>
            <span>基本信息</span>
          </template>
          <el-descriptions :column="2" border v-if="node">
            <el-descriptions-item label="ID">{{ node.id }}</el-descriptions-item>
            <el-descriptions-item label="名称">{{ node.name }}</el-descriptions-item>
            <el-descriptions-item label="地址">{{ node.address }}</el-descriptions-item>
            <el-descriptions-item label="端口">{{ node.port }}</el-descriptions-item>
            <el-descriptions-item label="地区">{{ node.region || '-' }}</el-descriptions-item>
            <el-descriptions-item label="权重">{{ node.weight }}</el-descriptions-item>
            <el-descriptions-item label="最大用户数">{{ node.max_users || '无限制' }}</el-descriptions-item>
            <el-descriptions-item label="当前用户数">{{ node.current_users }}</el-descriptions-item>
            <el-descriptions-item label="延迟">
              <span :class="getLatencyClass(node.latency)">{{ node.latency }}ms</span>
            </el-descriptions-item>
            <el-descriptions-item label="同步状态">
              <el-tag :type="getSyncStatusType(node.sync_status)" size="small">
                {{ getSyncStatusText(node.sync_status) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="最后在线">{{ formatTime(node.last_seen_at) }}</el-descriptions-item>
            <el-descriptions-item label="最后同步">{{ formatTime(node.synced_at) }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ formatTime(node.created_at) }}</el-descriptions-item>
            <el-descriptions-item label="更新时间">{{ formatTime(node.updated_at) }}</el-descriptions-item>
          </el-descriptions>
          <div v-if="node?.tags && parseTags(node.tags).length" class="tags-section">
            <span class="tags-label">标签：</span>
            <el-tag v-for="tag in parseTags(node.tags)" :key="tag" size="small" style="margin-right: 8px;">
              {{ tag }}
            </el-tag>
          </div>
        </el-card>

        <!-- 流量统计 -->
        <el-card shadow="never" class="info-card">
          <template #header>
            <div class="card-header">
              <span>流量统计</span>
              <el-radio-group v-model="trafficPeriod" size="small" @change="fetchTraffic">
                <el-radio-button label="today">今日</el-radio-button>
                <el-radio-button label="week">本周</el-radio-button>
                <el-radio-button label="month">本月</el-radio-button>
              </el-radio-group>
            </div>
          </template>
          <el-row :gutter="20">
            <el-col :span="8">
              <div class="traffic-stat">
                <div class="traffic-value">{{ formatBytes(trafficStats.upload) }}</div>
                <div class="traffic-label">上传流量</div>
              </div>
            </el-col>
            <el-col :span="8">
              <div class="traffic-stat">
                <div class="traffic-value">{{ formatBytes(trafficStats.download) }}</div>
                <div class="traffic-label">下载流量</div>
              </div>
            </el-col>
            <el-col :span="8">
              <div class="traffic-stat">
                <div class="traffic-value">{{ formatBytes(trafficStats.upload + trafficStats.download) }}</div>
                <div class="traffic-label">总流量</div>
              </div>
            </el-col>
          </el-row>
        </el-card>

        <!-- Top 用户 -->
        <el-card shadow="never" class="info-card">
          <template #header>
            <span>流量 Top 用户</span>
          </template>
          <el-table :data="topUsers" size="small" style="width: 100%">
            <el-table-column prop="user_id" label="用户 ID" width="100" />
            <el-table-column prop="username" label="用户名" />
            <el-table-column label="上传" width="120">
              <template #default="{ row }">{{ formatBytes(row.upload) }}</template>
            </el-table-column>
            <el-table-column label="下载" width="120">
              <template #default="{ row }">{{ formatBytes(row.download) }}</template>
            </el-table-column>
            <el-table-column label="总流量" width="120">
              <template #default="{ row }">{{ formatBytes(row.upload + row.download) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <!-- 右侧面板 -->
      <el-col :span="8">
        <!-- 实时状态 -->
        <el-card shadow="never" class="info-card">
          <template #header>
            <span>实时状态</span>
          </template>
          <div class="status-item">
            <span class="status-label">连接状态</span>
            <el-tag :type="node?.status === 'online' ? 'success' : 'danger'" size="small">
              {{ node?.status === 'online' ? '已连接' : '未连接' }}
            </el-tag>
          </div>
          <div class="status-item">
            <span class="status-label">负载</span>
            <div class="load-progress">
              <el-progress
                :percentage="loadPercentage"
                :status="loadStatus"
                :stroke-width="8"
              />
            </div>
          </div>
          <div class="status-item">
            <span class="status-label">延迟</span>
            <span :class="getLatencyClass(node?.latency)">{{ node?.latency || 0 }}ms</span>
          </div>
        </el-card>

        <!-- 内核管理 -->
        <el-card shadow="never" class="info-card">
          <template #header>
            <span>内核管理</span>
          </template>
          <div class="status-item">
            <span class="status-label">内核类型</span>
            <el-tag size="small">Xray</el-tag>
          </div>
          <div class="status-item">
            <span class="status-label">运行状态</span>
            <el-tag :type="node?.xray_running ? 'success' : 'danger'" size="small">
              {{ node?.xray_running ? '运行中' : '已停止' }}
            </el-tag>
          </div>
          <div class="status-item status-item-top">
            <span class="status-label">当前版本</span>
            <div class="core-version">{{ formatCoreVersion(node?.xray_version) }}</div>
          </div>
          <div class="status-item">
            <span class="status-label">最后心跳</span>
            <span>{{ formatTime(node?.last_seen_at) }}</span>
          </div>
          <div class="core-actions">
            <el-button plain @click="refreshData">
              刷新状态
            </el-button>
            <el-button
              v-if="!node?.xray_running"
              type="success"
              @click="startCore"
              :loading="coreActionLoading === 'start'"
            >
              启动内核
            </el-button>
            <el-button
              v-else
              type="warning"
              @click="restartCore"
              :loading="coreActionLoading === 'restart'"
            >
              重启内核
            </el-button>
            <el-button type="primary" @click="syncConfig" :loading="syncing">
              同步配置
            </el-button>
          </div>
          <div class="core-tip">节点命令会进入队列，并在节点下一次心跳时执行。</div>
        </el-card>

        <!-- 内核操作记录 -->
        <el-card shadow="never" class="info-card">
          <template #header>
            <span>内核操作记录</span>
          </template>
          <div v-if="recentRecoveryEvents.length" class="recovery-events">
            <div
              v-for="event in recentRecoveryEvents"
              :key="event.command_id"
              class="recovery-event"
            >
              <div class="recovery-event-header">
                <el-tag :type="getRecoveryStatusType(event.status)" size="small">
                  {{ getRecoveryStatusText(event.status) }}
                </el-tag>
                <span class="recovery-time">{{ formatTime(event.updated_at || event.created_at) }}</span>
              </div>
              <div class="recovery-command">{{ getRecoveryCommandText(event.command_type) }}</div>
              <div class="recovery-reason">{{ event.reason || '未提供原因' }}</div>
              <div class="recovery-meta">
                来源：{{ getRecoverySourceText(event.source) }}
                <span v-if="event.message"> · {{ event.message }}</span>
              </div>
            </div>
          </div>
          <el-empty v-else description="暂无恢复记录" :image-size="60" />
        </el-card>

        <!-- 所属分组 -->
        <el-card shadow="never" class="info-card">
          <template #header>
            <span>所属分组</span>
          </template>
          <div v-if="nodeGroups.length" class="groups-list">
            <el-tag
              v-for="group in nodeGroups"
              :key="group.id"
              style="margin-right: 8px; margin-bottom: 8px;"
            >
              {{ group.name }}
            </el-tag>
          </div>
          <el-empty v-else description="暂无分组" :image-size="60" />
        </el-card>

        <!-- 快捷操作 -->
        <el-card shadow="never" class="info-card">
          <template #header>
            <span>快捷操作</span>
          </template>
          <div class="quick-actions">
            <el-button type="warning" plain @click="showTokenDialog">
              管理 Token
            </el-button>
            <el-button type="danger" plain @click="deleteNode">
              删除节点
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Token 管理对话框 -->
    <el-dialog v-model="tokenDialogVisible" title="Token 管理" width="500px">
      <div class="token-dialog-content">
        <div class="token-info">
          <div class="token-label">当前 Token</div>
          <div class="token-value token-text">
            <span v-if="showToken">{{ currentToken || '未生成' }}</span>
            <span v-else>{{ currentToken ? '••••••••••••••••' : '未生成' }}</span>
            <el-button v-if="currentToken" link @click="showToken = !showToken">
              <el-icon><View v-if="!showToken" /><Hide v-else /></el-icon>
            </el-button>
            <el-button v-if="currentToken" link @click="copyToken">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </div>
        </div>
        <div class="token-actions">
          <el-button type="primary" @click="handleGenerateToken" :loading="tokenLoading">
            {{ currentToken ? '重新生成' : '生成 Token' }}
          </el-button>
          <el-button v-if="currentToken" type="warning" @click="handleRotateToken" :loading="tokenLoading">
            轮换 Token
          </el-button>
          <el-button v-if="currentToken" type="danger" @click="handleRevokeToken" :loading="tokenLoading">
            撤销 Token
          </el-button>
        </div>
        <el-alert
          v-if="newToken"
          type="success"
          :closable="false"
          show-icon
          class="new-token-alert"
        >
          <template #title>新 Token 已生成，请妥善保存：</template>
          <div class="new-token-text">{{ newToken }}</div>
        </el-alert>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Refresh, View, Hide, CopyDocument } from '@element-plus/icons-vue'
import { useNodeStore } from '@/stores/node'
import { nodeGroupsApi } from '@/api'

const route = useRoute()
const router = useRouter()
const nodeStore = useNodeStore()

const loading = ref(false)
const syncing = ref(false)
const coreActionLoading = ref('')
const tokenDialogVisible = ref(false)
const tokenLoading = ref(false)
const currentToken = ref('')
const newToken = ref('')
const showToken = ref(false)
const trafficPeriod = ref('today')
const trafficStats = ref({ upload: 0, download: 0 })
const topUsers = ref([])
const nodeGroups = ref([])

const node = computed(() => nodeStore.currentNode)
const recentRecoveryEvents = computed(() => Array.isArray(node.value?.recent_recovery_events) ? node.value.recent_recovery_events : [])

const loadPercentage = computed(() => {
  if (!node.value?.max_users) return 0
  return Math.round((node.value.current_users / node.value.max_users) * 100)
})

const loadStatus = computed(() => {
  if (loadPercentage.value >= 90) return 'exception'
  if (loadPercentage.value >= 70) return 'warning'
  return 'success'
})

const getStatusType = (status) => {
  const types = { online: 'success', offline: 'info', unhealthy: 'danger' }
  return types[status] || 'info'
}

const getStatusText = (status) => {
  const texts = { online: '在线', offline: '离线', unhealthy: '不健康' }
  return texts[status] || status
}

const getSyncStatusType = (status) => {
  const types = { synced: 'success', pending: 'warning', failed: 'danger' }
  return types[status] || 'info'
}

const getSyncStatusText = (status) => {
  const texts = { synced: '已同步', pending: '待同步', failed: '同步失败' }
  return texts[status] || status
}

const getLatencyClass = (latency) => {
  if (!latency) return ''
  if (latency < 100) return 'latency-good'
  if (latency < 300) return 'latency-medium'
  return 'latency-bad'
}

const getRecoveryStatusType = (status) => {
  const types = { success: 'success', failed: 'danger', dispatched: 'warning', queued: 'info' }
  return types[status] || 'info'
}

const getRecoveryStatusText = (status) => {
  const texts = { success: '已恢复', failed: '恢复失败', dispatched: '已下发', queued: '已排队' }
  return texts[status] || status || '未知'
}

const getRecoverySourceText = (source) => {
  const texts = { heartbeat: '节点心跳', health_checker: '健康检查器', admin: '管理员', portal_ping: '用户入口探测' }
  return texts[source] || source || '系统'
}

const getRecoveryCommandText = (commandType) => {
  const texts = {
    xray_start: '启动 Xray',
    xray_restart: '重启 Xray',
    xray_status: '刷新 Xray 状态',
    config_sync: '同步节点配置'
  }
  return texts[commandType] || commandType || '未知命令'
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const formatBytes = (bytes) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return `${bytes.toFixed(2)} ${units[i]}`
}

const formatCoreVersion = (version) => {
  if (!version) return '-'
  return String(version).split('\n')[0]
}

const parseTags = (tags) => {
  if (Array.isArray(tags)) return tags
  if (typeof tags === 'string') {
    try { return JSON.parse(tags) } catch { return [] }
  }
  return []
}

const getTimeRange = () => {
  const now = new Date()
  let start
  if (trafficPeriod.value === 'today') {
    start = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  } else if (trafficPeriod.value === 'week') {
    start = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
  } else {
    start = new Date(now.getFullYear(), now.getMonth(), 1)
  }
  return { start: start.toISOString(), end: now.toISOString() }
}

const fetchNode = async () => {
  loading.value = true
  try {
    await nodeStore.fetchNode(route.params.id)
  } catch (e) {
    ElMessage.error(e.message || '获取节点详情失败')
  } finally {
    loading.value = false
  }
}

const fetchTraffic = async () => {
  if (!node.value) return
  try {
    const { start, end } = getTimeRange()
    const res = await nodeStore.getNodeTraffic(node.value.id, { start, end })
    trafficStats.value = res || { upload: 0, download: 0 }
  } catch (e) {
    console.error('获取流量统计失败:', e)
  }
}

const fetchTopUsers = async () => {
  if (!node.value) return
  try {
    const { start, end } = getTimeRange()
    const res = await nodeStore.getTopUsers(node.value.id, { limit: 10, start, end })
    topUsers.value = res?.users || []
  } catch (e) {
    console.error('获取 Top 用户失败:', e)
  }
}

const fetchNodeGroups = async () => {
  if (!node.value) return
  try {
    const res = await nodeGroupsApi.list()
    const allGroups = res?.groups || res || []
    nodeGroups.value = allGroups.filter(g => {
      // 检查节点是否在该分组中
      return g.nodes?.some(n => n.id === node.value.id)
    })
  } catch (e) {
    console.error('获取分组失败:', e)
  }
}

const refreshData = async () => {
  await fetchNode()
  await Promise.all([fetchTraffic(), fetchTopUsers(), fetchNodeGroups()])
}

const goBack = () => {
  router.push('/admin/nodes')
}

const editNode = () => {
  router.push(`/admin/nodes/${node.value.id}/edit`)
}

const syncConfig = async () => {
  if (!node.value) return
  syncing.value = true
  try {
    const response = await nodeStore.syncNodeCoreConfig(node.value.id)
    ElMessage.success(response.message || '配置同步已加入队列')
    await fetchNode()
  } catch (e) {
    ElMessage.error(e.message || '同步失败')
  } finally {
    syncing.value = false
  }
}

const startCore = async () => {
  if (!node.value) return
  coreActionLoading.value = 'start'
  try {
    const response = await nodeStore.startNodeCore(node.value.id)
    ElMessage.success(response.message || '启动命令已加入队列')
    await fetchNode()
  } catch (e) {
    ElMessage.error(e.message || '启动节点内核失败')
  } finally {
    coreActionLoading.value = ''
  }
}

const restartCore = async () => {
  if (!node.value) return
  try {
    await ElMessageBox.confirm(
      `确定要重启节点 "${node.value.name}" 的 Xray 内核吗？`,
      '重启确认',
      { type: 'warning' }
    )
  } catch {
    return
  }
  coreActionLoading.value = 'restart'
  try {
    const response = await nodeStore.restartNodeCore(node.value.id)
    ElMessage.success(response.message || '重启命令已加入队列')
    await fetchNode()
  } catch (e) {
    ElMessage.error(e.message || '重启节点内核失败')
  } finally {
    coreActionLoading.value = ''
  }
}

const deleteNode = async () => {
  await ElMessageBox.confirm(
    `确定要删除节点 "${node.value.name}" 吗？该节点上的用户将被重新分配到其他节点。`,
    '删除确认',
    { type: 'warning' }
  )
  try {
    await nodeStore.deleteNode(node.value.id)
    ElMessage.success('删除成功')
    router.push('/admin/nodes')
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

const showTokenDialog = () => {
  currentToken.value = ''
  newToken.value = ''
  showToken.value = false
  tokenDialogVisible.value = true
}

const handleGenerateToken = async () => {
  tokenLoading.value = true
  try {
    const res = await nodeStore.generateToken(node.value.id)
    newToken.value = res.token
    currentToken.value = res.token
    ElMessage.success('Token 生成成功')
  } catch (e) {
    ElMessage.error(e.message || '生成 Token 失败')
  } finally {
    tokenLoading.value = false
  }
}

const handleRotateToken = async () => {
  await ElMessageBox.confirm('轮换 Token 后，旧 Token 将立即失效，确定继续？', '确认', { type: 'warning' })
  tokenLoading.value = true
  try {
    const res = await nodeStore.rotateToken(node.value.id)
    newToken.value = res.token
    currentToken.value = res.token
    ElMessage.success('Token 轮换成功')
  } catch (e) {
    ElMessage.error(e.message || '轮换 Token 失败')
  } finally {
    tokenLoading.value = false
  }
}

const handleRevokeToken = async () => {
  await ElMessageBox.confirm('撤销 Token 后，节点将无法连接，确定继续？', '确认', { type: 'warning' })
  tokenLoading.value = true
  try {
    await nodeStore.revokeToken(node.value.id)
    currentToken.value = ''
    newToken.value = ''
    ElMessage.success('Token 已撤销')
  } catch (e) {
    ElMessage.error(e.message || '撤销 Token 失败')
  } finally {
    tokenLoading.value = false
  }
}

const copyToken = async () => {
  try {
    await navigator.clipboard.writeText(currentToken.value)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败')
  }
}

onMounted(async () => {
  await fetchNode()
  await Promise.all([fetchTraffic(), fetchTopUsers(), fetchNodeGroups()])
})
</script>

<style scoped>
.node-detail-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.info-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tags-section {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.tags-label {
  color: var(--el-text-color-secondary);
  margin-right: 12px;
}

.traffic-stat {
  text-align: center;
  padding: 20px 0;
}

.traffic-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--el-color-primary);
}

.traffic-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-top: 8px;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.status-item-top {
  align-items: flex-start;
}

.status-item:last-child {
  border-bottom: none;
}

.status-label {
  color: var(--el-text-color-secondary);
}

.load-progress {
  width: 120px;
}

.groups-list {
  padding: 10px 0;
}

.quick-actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.core-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding-top: 16px;
}

.core-tip {
  margin-top: 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.5;
}

.core-version {
  max-width: 60%;
  color: var(--el-text-color-primary);
  font-size: 13px;
  line-height: 1.5;
  text-align: right;
  word-break: break-word;
}

.quick-actions .el-button {
  width: 100%;
}

.latency-good { color: var(--el-color-success); }
.latency-medium { color: var(--el-color-warning); }
.latency-bad { color: var(--el-color-danger); }

.token-dialog-content {
  padding: 10px 0;
}

.token-info {
  display: flex;
  margin-bottom: 16px;
}

.token-label {
  width: 100px;
  color: var(--el-text-color-secondary);
}

.token-value {
  flex: 1;
}

.token-text {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: monospace;
}

.token-actions {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.new-token-alert {
  margin-top: 20px;
}

.new-token-text {
  font-family: monospace;
  word-break: break-all;
  margin-top: 8px;
  padding: 8px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}


.recovery-events {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.recovery-event {
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-fill-color-blank);
}

.recovery-event-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.recovery-time {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.recovery-reason {
  font-size: 13px;
  color: var(--el-text-color-primary);
  margin-bottom: 6px;
  word-break: break-word;
}

.recovery-command {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-color-primary);
  margin-bottom: 6px;
}

.recovery-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  word-break: break-word;
}
</style>
