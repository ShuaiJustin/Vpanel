<template>
  <div class="node-detail-page">
    <div class="page-header">
      <div class="header-left">
        <el-button
          link
          @click="goBack"
        >
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h1 class="page-title">
          {{ node?.name || '节点详情' }}
        </h1>
        <el-tag
          v-if="node"
          :type="getStatusType(node.status)"
          size="large"
        >
          {{ getStatusText(node.status) }}
        </el-tag>
      </div>
      <div class="header-actions">
        <el-button @click="refreshData">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button @click="openOperationsPage">
          节点运维
        </el-button>
        <el-button
          type="primary"
          @click="editNode"
        >
          编辑
        </el-button>
      </div>
    </div>

    <div
      v-if="node"
      class="overview-grid"
    >
      <div
        v-for="card in overviewCards"
        :key="card.key"
        :class="['overview-card', { 'overview-card-primary': card.primary }]"
      >
        <div class="overview-label">
          {{ card.label }}
        </div>
        <div
          v-if="card.tags?.length"
          class="overview-tag-row"
        >
          <el-tag
            v-for="tag in card.tags"
            :key="`${card.key}-${tag.label}`"
            :type="tag.type"
            :effect="tag.effect || 'light'"
          >
            {{ tag.label }}
          </el-tag>
        </div>
        <div
          v-else
          :class="['overview-value', card.valueClass]"
        >
          {{ card.value }}
        </div>
        <div class="overview-meta">
          {{ card.meta }}
        </div>
      </div>
    </div>

    <el-row
      v-loading="loading"
      :gutter="isMobile ? 12 : 20"
    >
      <!-- 基本信息 -->
      <el-col :span="mainColumnSpan">
        <el-card
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="基本信息"
              subtitle="节点识别、接入参数与容量配置"
            >
              <el-tag
                v-if="node?.region"
                size="small"
                effect="plain"
              >
                {{ node.region }}
              </el-tag>
            </PageSectionHeader>
          </template>
          <el-descriptions
            v-if="node"
            :column="detailColumns"
            border
          >
            <el-descriptions-item
              v-for="item in basicInfoItems"
              :key="item.label"
              :label="item.label"
            >
              <el-tag
                v-if="item.type === 'tag'"
                :type="item.tagType"
                size="small"
              >
                {{ item.value }}
              </el-tag>
              <span
                v-else
                :class="item.valueClass"
              >
                {{ item.value }}
              </span>
            </el-descriptions-item>
          </el-descriptions>
          <div
            v-if="parsedTags.length"
            class="tags-section"
          >
            <span class="tags-label">标签：</span>
            <el-tag
              v-for="tag in parsedTags"
              :key="tag"
              size="small"
              style="margin-right: 8px;"
            >
              {{ tag }}
            </el-tag>
          </div>
          <div
            v-if="node"
            class="detail-panel-grid"
          >
            <div class="detail-panel">
              <span class="detail-panel-label">支持协议</span>
              <div class="detail-panel-content">
                <el-tag
                  v-for="protocol in supportedProtocols"
                  :key="protocol"
                  size="small"
                  effect="plain"
                >
                  {{ String(protocol).toUpperCase() }}
                </el-tag>
                <span
                  v-if="!supportedProtocols.length"
                  class="detail-inline-text"
                >
                  未配置
                </span>
              </div>
            </div>
            <div class="detail-panel">
              <span class="detail-panel-label">TLS 配置</span>
              <div class="detail-panel-content">
                <el-tag
                  :type="node.tls_enabled ? 'success' : 'info'"
                  size="small"
                >
                  {{ node.tls_enabled ? '已启用' : '未启用' }}
                </el-tag>
                <span
                  v-if="node.tls_domain"
                  class="detail-inline-text"
                >
                  {{ node.tls_domain }}
                </span>
              </div>
            </div>
            <div class="detail-panel">
              <span class="detail-panel-label">流量限制</span>
              <div class="detail-panel-content detail-panel-content-text">
                {{ formatLimitDisplay(node.traffic_limit) }}
              </div>
            </div>
            <div class="detail-panel">
              <span class="detail-panel-label">速率限制</span>
              <div class="detail-panel-content detail-panel-content-text">
                {{ formatSpeedLimitDisplay(node.speed_limit) }}
              </div>
            </div>
          </div>
        </el-card>

        <el-card
          v-if="hasNodeNotes"
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="备注信息"
              subtitle="节点说明、管理员备注与访问约束"
            />
          </template>
          <div class="notes-grid">
            <div
              v-if="node?.description"
              class="note-panel"
            >
              <div class="detail-panel-label">
                节点描述
              </div>
              <div class="note-text">
                {{ node.description }}
              </div>
            </div>
            <div
              v-if="node?.remarks"
              class="note-panel"
            >
              <div class="detail-panel-label">
                管理员备注
              </div>
              <div class="note-text">
                {{ node.remarks }}
              </div>
            </div>
            <div
              v-if="ipWhitelistEntries.length"
              class="note-panel note-panel-wide"
            >
              <div class="detail-panel-label">
                IP 白名单
              </div>
              <div class="tag-cloud">
                <el-tag
                  v-for="ip in ipWhitelistEntries"
                  :key="ip"
                  size="small"
                  effect="plain"
                >
                  {{ ip }}
                </el-tag>
              </div>
            </div>
          </div>
        </el-card>

        <!-- 流量统计 -->
        <el-card
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="流量统计"
              subtitle="查看节点在不同周期内的总流量消耗"
              align="center"
            >
              <el-radio-group
                v-model="trafficPeriod"
                size="small"
                @change="fetchTraffic"
              >
                <el-radio-button label="today">
                  今日
                </el-radio-button>
                <el-radio-button label="week">
                  本周
                </el-radio-button>
                <el-radio-button label="month">
                  本月
                </el-radio-button>
              </el-radio-group>
            </PageSectionHeader>
          </template>
          <el-row :gutter="isMobile ? 12 : 20">
            <el-col
              v-for="item in trafficSummaryCards"
              :key="item.key"
              :span="trafficStatSpan"
            >
              <div class="traffic-stat">
                <div class="traffic-value">
                  {{ item.value }}
                </div>
                <div class="traffic-label">
                  {{ item.label }}
                </div>
              </div>
            </el-col>
          </el-row>
        </el-card>

        <!-- Top 用户 -->
        <el-card
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="流量 Top 用户"
              subtitle="当前周期内消耗流量最高的用户列表"
            />
          </template>
          <div
            v-if="topUsers.length"
            class="table-shell"
          >
            <el-table
              :data="topUsers"
              size="small"
              style="width: 100%"
            >
              <el-table-column
                prop="user_id"
                label="用户 ID"
                width="100"
              />
              <el-table-column
                prop="username"
                label="用户名"
              />
              <el-table-column
                label="上传"
                width="120"
              >
                <template #default="{ row }">
                  {{ formatBytes(row.upload) }}
                </template>
              </el-table-column>
              <el-table-column
                label="下载"
                width="120"
              >
                <template #default="{ row }">
                  {{ formatBytes(row.download) }}
                </template>
              </el-table-column>
              <el-table-column
                label="总流量"
                width="120"
              >
                <template #default="{ row }">
                  {{ formatBytes(row.upload + row.download) }}
                </template>
              </el-table-column>
            </el-table>
          </div>
          <el-empty
            v-else
            description="当前周期暂无流量记录"
            :image-size="56"
          />
        </el-card>
      </el-col>

      <!-- 右侧面板 -->
      <el-col
        :span="sideColumnSpan"
        class="side-column"
      >
        <el-card
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="节点运维"
              subtitle="将内核管理、网络优化和运维记录集中到独立工作台"
            />
          </template>
          <div class="operation-handoff">
            <div class="operation-handoff__meta">
              <el-tag
                :type="node?.xray_running ? 'success' : 'danger'"
                size="small"
              >
                {{ node?.xray_running ? '内核运行中' : '内核已停止' }}
              </el-tag>
              <el-tag
                :type="getSyncStatusType(node?.sync_status)"
                size="small"
                effect="plain"
              >
                {{ getSyncStatusText(node?.sync_status) }}
              </el-tag>
            </div>
            <div class="operation-handoff__body">
              节点详情页只保留查看信息。内核管理、网络优化和运维记录已经移到独立工作台，避免详情页继续堆叠。
            </div>
            <div class="quick-actions operation-handoff__actions">
              <el-button
                type="primary"
                @click="openOperationsPage"
              >
                进入节点运维
              </el-button>
              <el-button
                plain
                @click="editNode"
              >
                编辑节点
              </el-button>
            </div>
          </div>
        </el-card>

        <!-- 所属分组 -->
        <el-card
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="所属分组"
              subtitle="当前节点归属的节点分组"
            />
          </template>
          <div
            v-if="nodeGroups.length"
            class="groups-list"
          >
            <el-tag
              v-for="group in nodeGroups"
              :key="group.id"
              style="margin-right: 8px; margin-bottom: 8px;"
            >
              {{ group.name }}
            </el-tag>
          </div>
          <el-empty
            v-else
            description="暂无分组"
            :image-size="60"
          />
        </el-card>

        <!-- 快捷操作 -->
        <el-card
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="快捷操作"
              subtitle="节点 Token 与删除等高风险操作入口"
            />
          </template>
          <div class="quick-actions">
            <el-button
              type="primary"
              @click="openOperationsPage"
            >
              进入节点运维
            </el-button>
            <el-button
              type="warning"
              plain
              @click="showTokenDialog"
            >
              管理 Token
            </el-button>
            <el-button
              type="danger"
              plain
              @click="deleteNode"
            >
              删除节点
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Token 管理对话框 -->
    <el-dialog
      v-model="tokenDialogVisible"
      title="Token 管理"
      :width="tokenDialogWidth"
    >
      <div class="token-dialog-content">
        <div class="token-info">
          <div class="token-label">
            当前 Token
          </div>
          <div class="token-value token-text">
            <span v-if="showToken">{{ currentToken || '未生成' }}</span>
            <span v-else>{{ currentToken ? '••••••••••••••••' : '未生成' }}</span>
            <el-button
              v-if="currentToken"
              link
              @click="showToken = !showToken"
            >
              <el-icon><View v-if="!showToken" /><Hide v-else /></el-icon>
            </el-button>
            <el-button
              v-if="currentToken"
              link
              @click="copyToken"
            >
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </div>
        </div>
        <div class="token-actions">
          <el-button
            type="primary"
            :loading="tokenLoading"
            @click="handleGenerateToken"
          >
            {{ currentToken ? '重新生成' : '生成 Token' }}
          </el-button>
          <el-button
            v-if="currentToken"
            type="warning"
            :loading="tokenLoading"
            @click="handleRotateToken"
          >
            轮换 Token
          </el-button>
          <el-button
            v-if="currentToken"
            type="danger"
            :loading="tokenLoading"
            @click="handleRevokeToken"
          >
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
          <template #title>
            新 Token 已生成，请妥善保存：
          </template>
          <div class="new-token-text">
            {{ newToken }}
          </div>
        </el-alert>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Refresh, View, Hide, CopyDocument } from '@element-plus/icons-vue'
import PageSectionHeader from '@/components/PageSectionHeader.vue'
import { useNodeStore } from '@/stores/node'
import { nodeGroupsApi, usersApi } from '@/api'
import {
  formatCoreVersion,
  formatNodeTime as formatTime,
  formatUsersLimitDisplay,
  getNodeLatencyClass as getLatencyClass,
  getNodeStatusText as getStatusText,
  getNodeStatusType as getStatusType,
  getNodeSyncStatusText as getSyncStatusText,
  getNodeSyncStatusType as getSyncStatusType,
  parseNodeTags as parseTags
} from '@/composables/useNodePresentation'
import { useViewport } from '@/composables/useViewport'

const route = useRoute()
const router = useRouter()
const nodeStore = useNodeStore()
const { isMobile } = useViewport()

const loading = ref(false)
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
const supportedProtocols = computed(() => Array.isArray(node.value?.protocols) ? node.value.protocols : [])
const supportedProtocolsDisplay = computed(() => (
  supportedProtocols.value.length
    ? supportedProtocols.value.map((protocol) => String(protocol).toUpperCase()).join(' / ')
    : '未配置协议'
))
const parsedTags = computed(() => parseTags(node.value?.tags))
const ipWhitelistEntries = computed(() => Array.isArray(node.value?.ip_whitelist) ? node.value.ip_whitelist : [])
const hasNodeNotes = computed(() => Boolean(
  node.value?.description ||
  node.value?.remarks ||
  ipWhitelistEntries.value.length
))
const mainColumnSpan = computed(() => (isMobile.value ? 24 : 16))
const sideColumnSpan = computed(() => (isMobile.value ? 24 : 8))
const detailColumns = computed(() => (isMobile.value ? 1 : 2))
const trafficStatSpan = computed(() => (isMobile.value ? 24 : 8))
const tokenDialogWidth = computed(() => (isMobile.value ? 'calc(100vw - 24px)' : '500px'))
const trafficPeriodText = computed(() => {
  const map = {
    today: '今日',
    week: '近 7 天',
    month: '本月'
  }
  return map[trafficPeriod.value] || '当前周期'
})
const currentUsersLimitDisplay = computed(() => formatUsersLimitDisplay(
  node.value?.current_users,
  node.value?.max_users
))
const overviewCards = computed(() => {
  if (!node.value) return []

  return [
    {
      key: 'address',
      label: '节点地址',
      value: node.value.address || '-',
      valueClass: 'overview-address',
      meta: `Agent 端口 ${node.value.port} · ${node.value.region || '未设置地区'}`,
      primary: true
    },
    {
      key: 'status',
      label: '连接与同步',
      tags: [
        { label: getStatusText(node.value.status), type: getStatusType(node.value.status) },
        { label: getSyncStatusText(node.value.sync_status), type: getSyncStatusType(node.value.sync_status), effect: 'plain' }
      ],
      meta: `最后在线 ${formatTime(node.value.last_seen_at)}`
    },
    {
      key: 'users',
      label: '用户负载',
      value: currentUsersLimitDisplay.value,
      meta: `权重 ${node.value.weight} · 优先级 ${node.value.priority || 0}`
    },
    {
      key: 'traffic',
      label: `${trafficPeriodText.value}流量`,
      value: formatBytes(trafficStats.value.upload + trafficStats.value.download),
      meta: `上行 ${formatBytes(trafficStats.value.upload)} · 下行 ${formatBytes(trafficStats.value.download)}`
    },
    {
      key: 'core',
      label: 'Xray 内核',
      value: node.value.xray_running ? '运行中' : '已停止',
      meta: formatCoreVersion(node.value.xray_version)
    },
    {
      key: 'access',
      label: '接入策略',
      value: node.value.tls_enabled ? 'TLS 已启用' : '未启用 TLS',
      meta: supportedProtocolsDisplay.value
    }
  ]
})
const basicInfoItems = computed(() => {
  if (!node.value) return []

  return [
    { label: 'ID', value: node.value.id },
    { label: '名称', value: node.value.name },
    { label: '地址', value: node.value.address },
    { label: 'Agent 端口', value: node.value.port },
    { label: '地区', value: node.value.region || '-' },
    { label: '权重', value: node.value.weight },
    { label: '最大用户数', value: node.value.max_users || '无限制' },
    { label: '当前用户数', value: node.value.current_users },
    { label: '延迟', value: `${node.value.latency || 0}ms`, valueClass: getLatencyClass(node.value.latency) },
    {
      label: '同步状态',
      value: getSyncStatusText(node.value.sync_status),
      type: 'tag',
      tagType: getSyncStatusType(node.value.sync_status)
    },
    { label: '最后在线', value: formatTime(node.value.last_seen_at) },
    { label: '最后同步', value: formatTime(node.value.synced_at) },
    { label: '创建时间', value: formatTime(node.value.created_at) },
    { label: '更新时间', value: formatTime(node.value.updated_at) }
  ]
})
const trafficSummaryCards = computed(() => [
  { key: 'upload', label: '上传流量', value: formatBytes(trafficStats.value.upload) },
  { key: 'download', label: '下载流量', value: formatBytes(trafficStats.value.download) },
  { key: 'total', label: '总流量', value: formatBytes(trafficStats.value.upload + trafficStats.value.download) }
])

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

const formatLimitDisplay = (bytes) => (Number(bytes) > 0 ? formatBytes(Number(bytes)) : '无限制')

const formatSpeedLimitDisplay = (bytes) => (Number(bytes) > 0 ? `${formatBytes(Number(bytes))}/s` : '无限制')

const normalizeTrafficStats = (response) => {
  const stats = response?.stats || response?.data?.stats || response?.data || response || {}
  return {
    upload: Number(stats.upload) || 0,
    download: Number(stats.download) || 0
  }
}

const normalizeTopUsers = (response) => {
  const rows = response?.top_users || response?.data?.top_users || response?.users || response?.data?.users || []
  if (!Array.isArray(rows)) return []

  return rows.map((row) => ({
    user_id: row.user_id,
    username: row.username || '',
    upload: Number(row.upload) || 0,
    download: Number(row.download) || 0
  }))
}

const fillTopUsernames = async (rows) => {
  const userIds = [...new Set(
    rows
      .filter((row) => row.user_id && !row.username)
      .map((row) => row.user_id)
  )]

  if (!userIds.length) {
    return rows.map((row) => ({
      ...row,
      username: row.username || `用户 #${row.user_id}`
    }))
  }

  const usernames = new Map()
  await Promise.all(userIds.map(async (userId) => {
    try {
      const user = await usersApi.get(userId)
      usernames.set(userId, user?.username || `用户 #${userId}`)
    } catch {
      usernames.set(userId, `用户 #${userId}`)
    }
  }))

  return rows.map((row) => ({
    ...row,
    username: row.username || usernames.get(row.user_id) || `用户 #${row.user_id}`
  }))
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
    trafficStats.value = normalizeTrafficStats(res)
  } catch (e) {
    console.error('获取流量统计失败:', e)
    trafficStats.value = { upload: 0, download: 0 }
  }
}

const fetchTopUsers = async () => {
  if (!node.value) return
  try {
    const { start, end } = getTimeRange()
    const res = await nodeStore.getTopUsers(node.value.id, { limit: 10, start, end })
    topUsers.value = await fillTopUsernames(normalizeTopUsers(res))
  } catch (e) {
    console.error('获取 Top 用户失败:', e)
    topUsers.value = []
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

const openOperationsPage = () => {
  if (!node.value?.id) return
  router.push(`/admin/nodes/${node.value.id}/operations`)
}

const editNode = () => {
  router.push(`/admin/nodes/${node.value.id}/edit`)
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
  await refreshData()
})

watch(
  () => route.params.id,
  async (newId, oldId) => {
    if (!newId || newId === oldId) return
    currentToken.value = ''
    newToken.value = ''
    showToken.value = false
    trafficStats.value = { upload: 0, download: 0 }
    topUsers.value = []
    nodeGroups.value = []
    await refreshData()
  }
)
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

.overview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.overview-card {
  display: flex;
  min-height: 132px;
  flex-direction: column;
  gap: 10px;
  padding: 18px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 16px;
  background: linear-gradient(180deg, var(--el-fill-color-light) 0%, var(--el-bg-color) 100%);
}

.overview-card-primary {
  background: linear-gradient(140deg, var(--el-color-primary-light-9) 0%, var(--el-bg-color) 100%);
}

.overview-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  letter-spacing: 0.04em;
}

.overview-value {
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--el-text-color-primary);
}

.overview-address {
  font-size: 20px;
  word-break: break-word;
}

.overview-meta {
  margin-top: auto;
  font-size: 13px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
  word-break: break-word;
}

.overview-tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.info-card {
  margin-bottom: 20px;
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

.detail-panel-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  margin-top: 18px;
}

.detail-panel {
  padding: 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-fill-color-light);
}

.detail-panel-label {
  display: block;
  margin-bottom: 10px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.detail-panel-content {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.detail-panel-content-text {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.detail-inline-text {
  font-size: 13px;
  color: var(--el-text-color-primary);
  line-height: 1.5;
  word-break: break-word;
}

.notes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.note-panel {
  min-height: 118px;
  padding: 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-fill-color-light);
}

.note-panel-wide {
  grid-column: 1 / -1;
}

.note-text {
  color: var(--el-text-color-primary);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
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

.side-column {
  min-width: 0;
}

.runtime-pill-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.runtime-pill {
  display: flex;
  min-height: 98px;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-fill-color-light);
}

.runtime-pill-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.runtime-pill small {
  color: var(--el-text-color-secondary);
  line-height: 1.4;
}

.metric-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.metric-row {
  padding: 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-bg-color);
}

.metric-row-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
  font-size: 13px;
}

.metric-row-header strong {
  font-size: 14px;
  color: var(--el-text-color-primary);
}

.metric-row-meta {
  margin-bottom: 10px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.runtime-footer {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.runtime-footer-item {
  display: flex;
  min-height: 74px;
  flex-direction: column;
  justify-content: space-between;
  gap: 8px;
  padding: 12px;
  border-radius: 12px;
  background: var(--el-fill-color-light);
}

.runtime-footer-item strong {
  color: var(--el-text-color-primary);
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
}

.table-shell {
  overflow-x: auto;
}

.table-shell :deep(.el-table) {
  min-width: 560px;
}

.groups-list {
  padding: 10px 0;
}

.quick-actions {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.operation-handoff {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.operation-handoff__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.operation-handoff__body {
  display: flex;
  align-items: center;
  min-height: 132px;
  padding: 14px 16px;
  border-radius: 12px;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
  line-height: 1.7;
}

.operation-handoff__actions {
  gap: 14px;
}

.operation-handoff__actions :deep(.el-button) {
  width: 100%;
  min-height: 50px;
  margin-left: 0;
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

.optimization-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.optimization-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 16px;
  padding: 16px 0;
}

.optimization-select {
  width: 180px;
}

.optimization-state-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.optimization-state-item {
  display: flex;
  min-height: 84px;
  flex-direction: column;
  justify-content: space-between;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-fill-color-light);
}

.optimization-state-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.network-endpoint {
  max-width: 62%;
  color: var(--el-text-color-primary);
  font-size: 13px;
  line-height: 1.5;
  text-align: right;
  word-break: break-word;
}

.network-endpoint-user {
  color: var(--el-text-color-secondary);
}

.optimization-log {
  max-height: 220px;
  margin: 16px 0 0;
  padding: 12px;
  overflow: auto;
  border-radius: 8px;
  background: #0f172a;
  color: #dbeafe;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.network-dialog-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.network-dialog-form {
  margin-top: 4px;
}

.quick-actions :deep(.el-button) {
  width: 100%;
  min-height: 44px;
  border-radius: 10px;
  margin-left: 0;
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

@media (max-width: 768px) {
  .node-detail-page {
    padding: 12px;
  }

  .page-header,
  .header-left,
  .header-actions,
  .status-item,
  .token-info,
  .token-actions,
  .recovery-event-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .header-actions,
  .token-actions,
  .core-actions {
    width: 100%;
  }

  .header-actions .el-button,
  .token-actions .el-button,
  .core-actions .el-button {
    width: 100%;
  }

  .load-progress,
  .core-version,
  .network-endpoint,
  .optimization-select {
    width: 100%;
    max-width: none;
    text-align: left;
  }

  .optimization-options,
  .optimization-state-grid,
  .runtime-pill-grid,
  .runtime-footer,
  .detail-panel-grid,
  .notes-grid {
    grid-template-columns: 1fr;
  }

  .operation-handoff {
    gap: 16px;
  }

  .operation-handoff__body {
    min-height: 0;
  }

  .token-label {
    width: auto;
  }

  .token-text {
    width: 100%;
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .traffic-stat {
    padding: 14px 0;
  }
}
</style>
