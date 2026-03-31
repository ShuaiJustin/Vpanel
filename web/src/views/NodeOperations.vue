<template>
  <div class="node-operations-page">
    <div class="page-header">
      <div class="header-left">
        <el-button
          link
          @click="goBack"
        >
          <el-icon><ArrowLeft /></el-icon>
          返回运维列表
        </el-button>
        <div class="header-copy">
          <h1 class="page-title">
            {{ node?.name || '节点运维' }}
          </h1>
          <p class="page-subtitle">
            统一处理内核管理、网络优化和操作记录
          </p>
        </div>
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
        <el-button @click="goToDetail">
          查看详情
        </el-button>
        <el-button
          type="primary"
          @click="editNode"
        >
          编辑节点
        </el-button>
      </div>
    </div>

    <div
      v-if="node"
      class="summary-grid"
    >
      <div
        v-for="card in summaryCards"
        :key="card.key"
        :class="['summary-card', { 'summary-card-primary': card.primary }]"
      >
        <span class="summary-label">{{ card.label }}</span>
        <strong :class="['summary-value', card.valueClass]">{{ card.value }}</strong>
        <small
          class="summary-meta"
          :title="card.metaTitle"
        >
          {{ card.meta }}
        </small>
      </div>
    </div>

    <div class="workspace-toolbar">
      <div class="workspace-toolbar__copy">
        <div class="workspace-toolbar__title">
          运维工作区
        </div>
        <div class="workspace-toolbar__description">
          {{ activeWorkspaceDescription }}
        </div>
      </div>
      <el-radio-group
        v-model="activeWorkspace"
        size="small"
        class="workspace-toolbar__switcher"
      >
        <el-radio-button
          v-for="workspace in workspaceOptions"
          :key="workspace.value"
          :label="workspace.value"
        >
          {{ workspace.label }}
        </el-radio-button>
      </el-radio-group>
    </div>

    <el-row
      v-loading="loading"
      :gutter="isMobile ? 12 : 20"
    >
      <el-col :span="24">
        <el-card
          v-if="activeWorkspace === 'core'"
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="内核管理"
              subtitle="Xray 进程控制、状态刷新与配置同步"
            />
          </template>
          <div class="status-item">
            <span class="status-label">内核类型</span>
            <el-tag size="small">
              Xray
            </el-tag>
          </div>
          <div class="status-item">
            <span class="status-label">运行状态</span>
            <el-tag
              :type="node?.xray_running ? 'success' : 'danger'"
              size="small"
            >
              {{ node?.xray_running ? '运行中' : '已停止' }}
            </el-tag>
          </div>
          <div class="status-item status-item-top">
            <span class="status-label">当前版本</span>
            <div class="core-version">
              {{ formatCoreVersion(node?.xray_version) }}
            </div>
          </div>
          <div class="status-item">
            <span class="status-label">最后心跳</span>
            <span>{{ formatTime(node?.last_seen_at) }}</span>
          </div>
          <div class="status-item">
            <span class="status-label">同步状态</span>
            <el-tag
              :type="getSyncStatusType(node?.sync_status)"
              size="small"
            >
              {{ getSyncStatusText(node?.sync_status) }}
            </el-tag>
          </div>
          <div class="core-actions">
            <el-button
              plain
              @click="refreshData"
            >
              刷新状态
            </el-button>
            <el-button
              v-if="!node?.xray_running"
              type="success"
              :loading="coreActionLoading === 'start'"
              @click="startCore"
            >
              启动内核
            </el-button>
            <el-button
              v-else
              type="warning"
              :loading="coreActionLoading === 'restart'"
              @click="restartCore"
            >
              重启内核
            </el-button>
            <el-button
              type="primary"
              :loading="syncing"
              @click="syncConfig"
            >
              同步配置
            </el-button>
          </div>
          <div class="core-tip">
            节点命令会进入队列，并在节点下一次心跳时执行。
          </div>
        </el-card>

        <el-card
          v-if="activeWorkspace === 'network'"
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="网络优化"
              subtitle="管理 Linux 网络栈与 Xray Sockopt 优化项"
            >
              <el-tag
                size="small"
                type="warning"
              >
                BBR / fq / TFO
              </el-tag>
            </PageSectionHeader>
          </template>
          <div class="profile-grid">
            <div class="profile-card">
              <div class="profile-card__header">
                <div>
                  <div class="profile-card__title">
                    推荐配置
                  </div>
                  <div class="profile-card__meta">
                    面向大多数 Linux VPS 的默认优化组合
                  </div>
                </div>
                <el-button
                  size="small"
                  @click="loadRecommendedOptimization"
                >
                  载入推荐
                </el-button>
              </div>
              <div class="profile-card__tags">
                <el-tag
                  v-for="tag in recommendedOptimizationTags"
                  :key="`recommended-${tag}`"
                  size="small"
                  effect="plain"
                >
                  {{ tag }}
                </el-tag>
              </div>
            </div>
            <div class="profile-card">
              <div class="profile-card__header">
                <div>
                  <div class="profile-card__title">
                    已保存配置
                  </div>
                  <div class="profile-card__meta">
                    当前节点上次落库的网络优化策略
                  </div>
                </div>
                <el-button
                  size="small"
                  :disabled="!hasSavedOptimizationSettings"
                  @click="loadSavedOptimization"
                >
                  载入已保存
                </el-button>
              </div>
              <div
                v-if="savedOptimizationTags.length"
                class="profile-card__tags"
              >
                <el-tag
                  v-for="tag in savedOptimizationTags"
                  :key="`saved-${tag}`"
                  size="small"
                  effect="plain"
                  type="success"
                >
                  {{ tag }}
                </el-tag>
              </div>
              <div
                v-else
                class="profile-card__empty"
              >
                该节点暂未保存专属优化配置。
              </div>
            </div>
          </div>
          <div class="optimization-tags">
            <el-tag
              v-for="tag in activeOptimizationTags"
              :key="tag"
              size="small"
              effect="plain"
            >
              {{ tag }}
            </el-tag>
          </div>
          <div class="status-item status-item-top">
            <span class="status-label">SSH 目标</span>
            <div class="network-endpoint">
              {{ sshEndpoint }}
            </div>
          </div>
          <div class="optimization-options">
            <el-checkbox v-model="networkOptimizationForm.enable_bbr">
              启用 BBR
            </el-checkbox>
            <el-checkbox v-model="networkOptimizationForm.enable_fq">
              启用 fq 队列
            </el-checkbox>
            <el-checkbox v-model="networkOptimizationForm.enable_tcp_fastopen">
              启用 TCP Fast Open
            </el-checkbox>
            <el-checkbox v-model="networkOptimizationForm.enable_xray_sockopt">
              同步 Xray Sockopt
            </el-checkbox>
            <el-checkbox
              v-model="networkOptimizationForm.xray_tcp_fastopen"
              :disabled="!networkOptimizationForm.enable_xray_sockopt"
            >
              Xray 开启 TCP Fast Open
            </el-checkbox>
          </div>
          <div class="status-item">
            <span class="status-label">Xray TCP 拥塞控制</span>
            <el-select
              v-model="networkOptimizationForm.xray_tcp_congestion"
              class="optimization-select"
              :disabled="!networkOptimizationForm.enable_xray_sockopt"
            >
              <el-option
                label="bbr"
                value="bbr"
              />
              <el-option
                label="cubic"
                value="cubic"
              />
              <el-option
                label="不设置"
                value=""
              />
            </el-select>
          </div>
          <div
            v-if="networkOptimizationState"
            class="optimization-state-grid"
          >
            <div
              v-for="item in optimizationStateItems"
              :key="item.label"
              class="optimization-state-item"
            >
              <span class="optimization-state-label">{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
          <el-empty
            v-else
            description="尚未检测远端网络优化状态"
            :image-size="56"
          />
          <div
            v-if="networkOptimizationState?.available_congestion_controls?.length"
            class="core-tip"
          >
            可用拥塞控制：{{ networkOptimizationState.available_congestion_controls.join(', ') }}
          </div>
          <div
            v-if="networkOptimizationState?.xray_config_path"
            class="core-tip"
          >
            Xray 配置：{{ networkOptimizationState.xray_config_path }}
          </div>
          <div class="core-actions">
            <el-button @click="networkOptimizationDialogVisible = true">
              SSH 配置
            </el-button>
            <el-button
              :loading="networkOptimizationAction === 'inspect'"
              @click="inspectNetworkOptimization"
            >
              检测
            </el-button>
            <el-button
              type="primary"
              :loading="networkOptimizationAction === 'apply'"
              @click="applyNetworkOptimization"
            >
              应用优化
            </el-button>
            <el-button
              type="danger"
              plain
              :loading="networkOptimizationAction === 'rollback'"
              @click="rollbackNetworkOptimization"
            >
              回滚
            </el-button>
          </div>
          <div class="core-tip">
            系统层修改会立即通过 SSH 生效，Xray Sockopt 会加入配置同步队列。没有保存 SSH 凭据时，请先填写密码或私钥。
          </div>
          <el-collapse
            v-if="networkOptimizationLogs"
            v-model="networkLogPanels"
            class="operation-collapse"
          >
            <el-collapse-item
              name="network-log"
              title="执行日志"
            >
              <pre class="optimization-log">{{ networkOptimizationLogs }}</pre>
            </el-collapse-item>
          </el-collapse>
        </el-card>

        <el-card
          v-if="activeWorkspace === 'events'"
          shadow="never"
          class="info-card"
        >
          <template #header>
            <PageSectionHeader
              title="操作记录"
              subtitle="最近的恢复、同步和节点调度记录"
            />
          </template>
          <div
            v-if="recentRecoveryEvents.length"
            class="recovery-events"
          >
            <el-timeline class="operations-timeline">
              <el-timeline-item
                v-for="event in recentRecoveryEvents"
                :key="event.command_id"
                :timestamp="formatTime(event.updated_at || event.created_at)"
                :color="getRecoveryStatusColor(event.status)"
                placement="top"
              >
                <div class="timeline-card">
                  <div class="recovery-event-header">
                    <el-tag
                      :type="getRecoveryStatusType(event.status)"
                      size="small"
                    >
                      {{ getRecoveryStatusText(event.status) }}
                    </el-tag>
                  </div>
                  <div class="recovery-command">
                    {{ getRecoveryCommandText(event.command_type) }}
                  </div>
                  <div class="recovery-reason">
                    {{ event.reason || '未提供原因' }}
                  </div>
                  <div class="recovery-meta">
                    来源：{{ getRecoverySourceText(event.source) }}
                    <span v-if="event.message"> · {{ event.message }}</span>
                  </div>
                </div>
              </el-timeline-item>
            </el-timeline>
          </div>
          <el-empty
            v-else
            description="暂无操作记录"
            :image-size="60"
          />
        </el-card>
      </el-col>
    </el-row>

    <el-dialog
      v-model="networkOptimizationDialogVisible"
      title="SSH 连接配置"
      :width="networkDialogWidth"
    >
      <div class="network-dialog-content">
        <el-alert
          type="info"
          :closable="false"
          show-icon
        >
          <template #title>
            网络优化会直接修改节点的 Linux `sysctl` 参数。请使用具备 root 或 sudo 权限的 SSH 账户。
          </template>
        </el-alert>
        <el-form
          label-width="110px"
          class="network-dialog-form"
        >
          <el-form-item label="SSH 主机">
            <el-input
              v-model="sshForm.host"
              placeholder="默认使用节点地址"
            />
          </el-form-item>
          <el-form-item label="SSH 端口">
            <el-input-number
              v-model="sshForm.port"
              :min="1"
              :max="65535"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="SSH 用户名">
            <el-input
              v-model="sshForm.username"
              placeholder="root"
            />
          </el-form-item>
          <el-form-item label="SSH 密码">
            <el-input
              v-model="sshForm.password"
              type="password"
              show-password
              placeholder="留空则尝试节点已保存密码"
            />
          </el-form-item>
          <el-form-item label="SSH 私钥">
            <el-input
              v-model="sshForm.private_key"
              type="textarea"
              :rows="7"
              placeholder="留空则尝试节点已保存私钥路径"
            />
          </el-form-item>
        </el-form>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Refresh } from '@element-plus/icons-vue'
import PageSectionHeader from '@/components/PageSectionHeader.vue'
import { nodesApi } from '@/api'
import {
  formatCoreVersion,
  formatCoreVersionCompact,
  formatNodeTime as formatTime,
  formatUsersLimitDisplay,
  getNodeStatusText as getStatusText,
  getNodeStatusType as getStatusType,
  getNodeSyncStatusText as getSyncStatusText,
  getNodeSyncStatusType as getSyncStatusType,
  getRecoveryCommandText,
  getRecoverySourceText,
  getRecoveryStatusColor,
  getRecoveryStatusText,
  getRecoveryStatusType
} from '@/composables/useNodePresentation'
import { useViewport } from '@/composables/useViewport'
import { useNodeStore } from '@/stores/node'
import { extractErrorMessage } from '@/utils/entitlement'

const route = useRoute()
const router = useRouter()
const nodeStore = useNodeStore()
const { isMobile } = useViewport()

const loading = ref(false)
const syncing = ref(false)
const coreActionLoading = ref('')
const activeWorkspace = ref('core')
const networkOptimizationDialogVisible = ref(false)
const networkOptimizationAction = ref('')
const networkLogPanels = ref([])
const networkOptimizationLogs = ref('')
const networkOptimizationState = ref(null)
const networkOptimizationMeta = ref({
  has_saved_settings: false,
  saved_settings: {},
  recommended_settings: {
    enable_bbr: true,
    enable_fq: true,
    enable_tcp_fastopen: true,
    enable_xray_sockopt: true,
    xray_tcp_fastopen: true,
    xray_tcp_congestion: 'bbr'
  },
  ssh_defaults: {
    host: '',
    port: 22,
    username: 'root',
    has_saved_password: false,
    has_saved_private_key: false
  },
  backup_path: ''
})

const sshForm = reactive({
  host: '',
  port: 22,
  username: 'root',
  password: '',
  private_key: ''
})

const networkOptimizationForm = reactive({
  enable_bbr: true,
  enable_fq: true,
  enable_tcp_fastopen: true,
  enable_xray_sockopt: true,
  xray_tcp_fastopen: true,
  xray_tcp_congestion: 'bbr'
})

const node = computed(() => nodeStore.currentNode)
const recentRecoveryEvents = computed(() => Array.isArray(node.value?.recent_recovery_events) ? node.value.recent_recovery_events : [])
const workspaceOptions = Object.freeze([
  { value: 'core', label: '内核管理' },
  { value: 'network', label: '网络优化' },
  { value: 'events', label: '操作记录' }
])
const networkDialogWidth = computed(() => (isMobile.value ? 'calc(100vw - 24px)' : '720px'))
const activeWorkspaceDescription = computed(() => {
  const descriptions = {
    core: '集中处理 Xray 状态、进程控制与配置同步，避免在详情页分散操作。',
    network: '先在这里整理草稿，再决定检测、应用或回滚网络优化策略。',
    events: '查看节点最近的恢复、同步和自动调度记录，便于回溯变更。'
  }
  return descriptions[activeWorkspace.value] || descriptions.core
})
const currentUsersLimitDisplay = computed(() => formatUsersLimitDisplay(
  node.value?.current_users,
  node.value?.max_users
))
const loadPercentage = computed(() => {
  if (!node.value?.max_users) return 0
  return Math.round((node.value.current_users / node.value.max_users) * 100)
})
const summaryCards = computed(() => {
  if (!node.value) return []

  return [
    {
      key: 'address',
      label: '节点地址',
      value: node.value.address || '-',
      valueClass: 'summary-value-address',
      meta: `Agent 端口 ${node.value.port} · ${node.value.region || '未设置地区'}`,
      metaTitle: '',
      primary: true
    },
    {
      key: 'core',
      label: '内核状态',
      value: node.value.xray_running ? '运行中' : '已停止',
      valueClass: '',
      meta: formatCoreVersionCompact(node.value.xray_version),
      metaTitle: formatCoreVersion(node.value.xray_version)
    },
    {
      key: 'sync',
      label: '同步状态',
      value: getSyncStatusText(node.value.sync_status),
      valueClass: '',
      meta: `最后同步 ${formatTime(node.value.synced_at)}`,
      metaTitle: ''
    },
    {
      key: 'users',
      label: '用户负载',
      value: currentUsersLimitDisplay.value,
      valueClass: '',
      meta: `负载 ${loadPercentage.value}%`,
      metaTitle: ''
    }
  ]
})
const hasSavedSSHCredentials = computed(() => Boolean(
  networkOptimizationMeta.value?.ssh_defaults?.has_saved_password ||
  networkOptimizationMeta.value?.ssh_defaults?.has_saved_private_key
))
const sshEndpoint = computed(() => {
  const host = sshForm.host || networkOptimizationMeta.value?.ssh_defaults?.host || node.value?.address || '-'
  const port = sshForm.port || networkOptimizationMeta.value?.ssh_defaults?.port || 22
  const username = sshForm.username || networkOptimizationMeta.value?.ssh_defaults?.username || 'root'
  return `${host}:${port} / ${username}`
})

const buildOptimizationTags = (settings) => {
  const source = settings || {}
  const tags = []
  if (source.enable_bbr) tags.push('BBR')
  if (source.enable_fq) tags.push('fq')
  if (source.enable_tcp_fastopen) tags.push('TCP Fast Open')
  if (source.enable_xray_sockopt) tags.push('Xray Sockopt')
  if (source.enable_xray_sockopt && source.xray_tcp_congestion) {
    tags.push(`Xray ${source.xray_tcp_congestion}`)
  }
  return tags
}

const activeOptimizationTags = computed(() => {
  const tags = buildOptimizationTags(networkOptimizationForm)
  return tags.length ? tags : ['未启用优化项']
})

const recommendedOptimizationTags = computed(() => {
  const tags = buildOptimizationTags(networkOptimizationMeta.value?.recommended_settings)
  return tags.length ? tags : ['未提供推荐配置']
})

const savedOptimizationTags = computed(() => buildOptimizationTags(networkOptimizationMeta.value?.saved_settings))
const hasSavedOptimizationSettings = computed(() => Boolean(
  networkOptimizationMeta.value?.has_saved_settings &&
  savedOptimizationTags.value.length
))
const optimizationStateItems = computed(() => {
  if (!networkOptimizationState.value) return []

  return [
    { label: '内核', value: networkOptimizationState.value.kernel_version || '-' },
    { label: '当前拥塞', value: networkOptimizationState.value.current_congestion_control || '-' },
    { label: '默认队列', value: networkOptimizationState.value.default_qdisc || '-' },
    { label: 'TCP Fast Open', value: networkOptimizationState.value.tcp_fastopen || '-' },
    { label: 'BBR 可用', value: networkOptimizationState.value.bbr_available ? '是' : '否' },
    { label: '备份状态', value: networkOptimizationState.value.backup_exists ? '已创建' : '未创建' }
  ]
})

const applyNetworkOptimizationForm = (settings) => {
  const source = settings || networkOptimizationMeta.value?.recommended_settings || {}
  networkOptimizationForm.enable_bbr = source.enable_bbr ?? true
  networkOptimizationForm.enable_fq = source.enable_fq ?? true
  networkOptimizationForm.enable_tcp_fastopen = source.enable_tcp_fastopen ?? true
  networkOptimizationForm.enable_xray_sockopt = source.enable_xray_sockopt ?? true
  networkOptimizationForm.xray_tcp_fastopen = source.xray_tcp_fastopen ?? true
  networkOptimizationForm.xray_tcp_congestion = source.xray_tcp_congestion ?? 'bbr'
}

const updateNetworkLogs = (logs) => {
  networkOptimizationLogs.value = logs || ''
  networkLogPanels.value = networkOptimizationLogs.value ? ['network-log'] : []
}

const loadRecommendedOptimization = () => {
  applyNetworkOptimizationForm(networkOptimizationMeta.value?.recommended_settings)
  ElMessage.success('已载入推荐优化配置')
}

const loadSavedOptimization = () => {
  if (!hasSavedOptimizationSettings.value) {
    ElMessage.warning('当前节点暂无已保存优化配置')
    return
  }
  applyNetworkOptimizationForm(networkOptimizationMeta.value?.saved_settings)
  ElMessage.success('已载入已保存优化配置')
}

const ensureSSHDefaults = (force = false) => {
  const defaults = networkOptimizationMeta.value?.ssh_defaults || {}
  if (force || !sshForm.host) {
    sshForm.host = defaults.host || node.value?.address || ''
  }
  if (force || !sshForm.port) {
    sshForm.port = defaults.port || 22
  }
  if (force || !sshForm.username) {
    sshForm.username = defaults.username || 'root'
  }
}

const fetchNode = async () => {
  loading.value = true
  try {
    await nodeStore.fetchNode(route.params.id)
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '获取节点运维信息失败')
  } finally {
    loading.value = false
  }
}

const fetchNetworkOptimizationProfile = async (forceSSHDefaults = false) => {
  if (!node.value) return

  try {
    const response = await nodesApi.getNetworkOptimizationProfile(node.value.id)
    networkOptimizationMeta.value = {
      ...networkOptimizationMeta.value,
      ...response
    }
    ensureSSHDefaults(forceSSHDefaults)
    if (response?.has_saved_settings) {
      applyNetworkOptimizationForm(response.saved_settings)
    } else {
      applyNetworkOptimizationForm(response?.recommended_settings)
    }
  } catch (error) {
    console.error('获取网络优化配置失败:', error)
  }
}

const refreshData = async () => {
  await fetchNode()
  await fetchNetworkOptimizationProfile()
}

const goBack = () => {
  router.push('/admin/node-operations')
}

const goToDetail = () => {
  if (!node.value) return
  router.push(`/admin/nodes/${node.value.id}`)
}

const editNode = () => {
  if (!node.value) return
  router.push(`/admin/nodes/${node.value.id}/edit`)
}

const syncConfig = async () => {
  if (!node.value) return
  syncing.value = true
  try {
    const response = await nodeStore.syncNodeCoreConfig(node.value.id)
    ElMessage.success(response.message || '配置同步已加入队列')
    await fetchNode()
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '同步失败')
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
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '启动节点内核失败')
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
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '重启节点内核失败')
  } finally {
    coreActionLoading.value = ''
  }
}

const getNetworkOptimizationSSHPayload = () => ({
  host: sshForm.host,
  port: sshForm.port,
  username: sshForm.username,
  password: sshForm.password,
  private_key: sshForm.private_key
})

const validateNetworkOptimizationSSH = () => {
  if (!sshForm.host || !sshForm.username) {
    ElMessage.warning('请先填写 SSH 主机和用户名')
    networkOptimizationDialogVisible.value = true
    return false
  }

  if (!sshForm.password && !sshForm.private_key && !hasSavedSSHCredentials.value) {
    ElMessage.warning('请提供 SSH 密码或私钥')
    networkOptimizationDialogVisible.value = true
    return false
  }

  return true
}

const inspectNetworkOptimization = async () => {
  if (!node.value || !validateNetworkOptimizationSSH()) return
  networkOptimizationAction.value = 'inspect'
  try {
    const response = await nodesApi.inspectNetworkOptimization(node.value.id, {
      ssh: getNetworkOptimizationSSHPayload()
    })
    networkOptimizationState.value = response?.state || null
    updateNetworkLogs(response?.logs)
    if (response?.saved_settings) {
      networkOptimizationMeta.value.saved_settings = response.saved_settings
    }
    ElMessage.success('节点网络状态检测完成')
  } catch (error) {
    updateNetworkLogs(error?.logs || error?.response?.data?.logs)
    ElMessage.error(extractErrorMessage(error) || '检测节点网络优化状态失败')
  } finally {
    networkOptimizationAction.value = ''
  }
}

const applyNetworkOptimization = async () => {
  if (!node.value || !validateNetworkOptimizationSSH()) return

  try {
    await ElMessageBox.confirm(
      `确定要为节点 "${node.value.name}" 应用网络优化吗？这会修改节点 sysctl，并触发一次 Xray 配置同步。`,
      '应用网络优化',
      { type: 'warning' }
    )
  } catch {
    return
  }

  networkOptimizationAction.value = 'apply'
  try {
    const response = await nodesApi.applyNetworkOptimization(node.value.id, {
      ssh: getNetworkOptimizationSSHPayload(),
      settings: { ...networkOptimizationForm }
    })
    networkOptimizationState.value = response?.result?.state || null
    updateNetworkLogs(response?.result?.log)
    networkOptimizationMeta.value.has_saved_settings = true
    networkOptimizationMeta.value.saved_settings = { ...networkOptimizationForm }
    ElMessage.success(response?.message || '节点网络优化已应用')
    await refreshData()
  } catch (error) {
    updateNetworkLogs(error?.logs || error?.response?.data?.logs)
    ElMessage.error(extractErrorMessage(error) || '应用节点网络优化失败')
  } finally {
    networkOptimizationAction.value = ''
  }
}

const rollbackNetworkOptimization = async () => {
  if (!node.value || !validateNetworkOptimizationSSH()) return

  try {
    await ElMessageBox.confirm(
      `确定要回滚节点 "${node.value.name}" 的网络优化吗？这会恢复原始 sysctl，并清除节点上的 Xray 优化设置。`,
      '回滚网络优化',
      { type: 'warning' }
    )
  } catch {
    return
  }

  networkOptimizationAction.value = 'rollback'
  try {
    const response = await nodesApi.rollbackNetworkOptimization(node.value.id, {
      ssh: getNetworkOptimizationSSHPayload()
    })
    networkOptimizationState.value = response?.result?.state || null
    updateNetworkLogs(response?.result?.log)
    networkOptimizationMeta.value.has_saved_settings = false
    networkOptimizationMeta.value.saved_settings = {}
    applyNetworkOptimizationForm(networkOptimizationMeta.value.recommended_settings)
    ElMessage.success(response?.message || '节点网络优化已回滚')
    await refreshData()
  } catch (error) {
    networkOptimizationLogs.value = error?.logs || error?.response?.data?.logs || ''
    ElMessage.error(extractErrorMessage(error) || '回滚节点网络优化失败')
  } finally {
    networkOptimizationAction.value = ''
  }
}

onMounted(async () => {
  await refreshData()
})

watch(
  () => route.params.id,
  async (newId, oldId) => {
    if (!newId || newId === oldId) return
    activeWorkspace.value = 'core'
    updateNetworkLogs('')
    networkOptimizationState.value = null
    networkOptimizationMeta.value.has_saved_settings = false
    networkOptimizationMeta.value.saved_settings = {}
    sshForm.host = ''
    sshForm.port = 22
    sshForm.username = 'root'
    sshForm.password = ''
    sshForm.private_key = ''
    applyNetworkOptimizationForm(networkOptimizationMeta.value.recommended_settings)
    await refreshData()
  }
)
</script>

<style scoped>
.node-operations-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.page-title {
  margin: 0;
  font-size: 28px;
  font-weight: 600;
}

.page-subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
}

.header-actions {
  display: flex;
  gap: 12px;
}

.node-operations-page .summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 20px;
}

.workspace-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
  padding: 16px 18px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.workspace-toolbar__copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
}

.workspace-toolbar__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.workspace-toolbar__description {
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}

.workspace-toolbar__switcher {
  flex-shrink: 0;
}

.summary-card {
  display: flex;
  min-height: 100px;
  flex-direction: column;
  gap: 8px;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.summary-card-primary {
  background: linear-gradient(140deg, var(--el-color-primary-light-9) 0%, var(--el-bg-color) 100%);
}

.summary-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.summary-value {
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--el-text-color-primary);
}

.summary-value-address {
  word-break: break-word;
}

.summary-meta {
  margin-top: auto;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
}

.info-card {
  margin-bottom: 18px;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.status-item:last-child {
  border-bottom: none;
}

.status-item-top {
  align-items: flex-start;
}

.status-label {
  color: var(--el-text-color-secondary);
}

.core-version,
.network-endpoint {
  max-width: 62%;
  color: var(--el-text-color-primary);
  font-size: 13px;
  line-height: 1.5;
  text-align: right;
  word-break: break-word;
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

.profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.profile-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-fill-color-light);
}

.profile-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.profile-card__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.profile-card__meta {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}

.profile-card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.profile-card__empty {
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
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

.optimization-log {
  max-height: 220px;
  margin: 0;
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

.operation-collapse {
  margin-top: 16px;
}

.operation-collapse :deep(.el-collapse-item__header) {
  font-weight: 600;
}

.recovery-events {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.operations-timeline {
  padding-top: 4px;
}

.recovery-event-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.timeline-card {
  padding: 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-fill-color-light);
}

.recovery-command {
  margin-top: 10px;
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-color-primary);
}

.recovery-reason {
  margin-bottom: 6px;
  color: var(--el-text-color-primary);
  line-height: 1.6;
  word-break: break-word;
}

.recovery-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
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

@media (max-width: 1280px) {
  .node-operations-page .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .node-operations-page {
    padding: 12px;
  }

  .page-header,
  .header-left,
  .header-actions,
  .workspace-toolbar,
  .status-item,
  .recovery-event-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .node-operations-page .summary-grid,
  .workspace-toolbar {
    grid-template-columns: 1fr;
  }

  .header-actions {
    width: 100%;
  }

  .header-actions :deep(.el-button) {
    width: 100%;
  }

  .core-version,
  .network-endpoint,
  .optimization-select {
    width: 100%;
    max-width: none;
    text-align: left;
  }

  .optimization-options,
  .optimization-state-grid,
  .profile-grid {
    grid-template-columns: 1fr;
  }

  .workspace-toolbar__switcher {
    width: 100%;
  }

  .workspace-toolbar__switcher :deep(.el-radio-button),
  .workspace-toolbar__switcher :deep(.el-radio-button__inner) {
    width: 100%;
  }
}
</style>
