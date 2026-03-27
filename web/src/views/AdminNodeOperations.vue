<template>
  <div class="node-operations-index-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          节点运维
        </h1>
        <p class="page-subtitle">
          集中处理节点内核控制、网络优化和运维入口分发
        </p>
      </div>
      <div class="page-actions">
        <el-button @click="fetchNodes">
          <el-icon class="el-icon--left">
            <Refresh />
          </el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前页节点</span>
        <strong class="overview-value">{{ filteredNodes.length }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">在线节点</span>
        <strong class="overview-value is-success">{{ onlineCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">内核运行中</span>
        <strong class="overview-value is-primary">{{ xrayRunningCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">待同步</span>
        <strong class="overview-value is-warning">{{ pendingSyncCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">平均延迟</span>
        <strong class="overview-value">{{ averageLatency }}ms</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-main">
        <div class="toolbar-copy">
          <div class="toolbar-title">
            运维筛选区
          </div>
          <div class="toolbar-description">
            先筛出需要处理的节点，再从列表直接进入单节点运维工作台。
          </div>
        </div>
        <div class="toolbar-filters">
          <el-input
            v-model="filters.search"
            class="toolbar-search"
            placeholder="搜索节点名称、地址或地区"
            clearable
          />
          <el-select
            v-model="filters.status"
            placeholder="状态"
            clearable
            @change="handleFilterChange"
          >
            <el-option
              label="在线"
              value="online"
            />
            <el-option
              label="离线"
              value="offline"
            />
            <el-option
              label="不健康"
              value="unhealthy"
            />
          </el-select>
          <el-select
            v-model="filters.region"
            placeholder="地区"
            clearable
            @change="handleFilterChange"
          >
            <el-option
              v-for="region in regions"
              :key="region"
              :label="region"
              :value="region"
            />
          </el-select>
          <el-button @click="resetFilters">
            重置
          </el-button>
        </div>
      </div>
      <div class="toolbar-side">
        <div class="toolbar-summary">
          当前页共 {{ nodeStore.total }} 条，筛选后 {{ filteredNodes.length }} 条
        </div>
        <div class="toolbar-chip-row">
          <span class="toolbar-chip">
            {{ activeFilterCount ? `已启用 ${activeFilterCount} 个筛选` : "当前查看全部节点" }}
          </span>
          <span class="toolbar-chip toolbar-chip--primary">
            待同步 {{ pendingSyncCount }}
          </span>
        </div>
      </div>
    </div>

    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <div>
            <div class="section-title">
              运维入口
            </div>
            <div class="section-subtitle">
              从节点列表直接进入单节点运维工作台
            </div>
          </div>
        </div>
      </template>

      <div class="table-shell">
        <el-table
          v-loading="nodeStore.loading"
          :data="filteredNodes"
          border
          stripe
          row-key="id"
          :empty-text="hasActiveFilters ? '暂无匹配节点' : '暂无节点'"
        >
          <el-table-column
            label="节点对象"
            min-width="280"
          >
            <template #default="{ row }">
              <div class="entity-cell">
                <div class="entity-title">
                  {{ row.name }}
                </div>
                <div class="entity-meta">
                  {{ row.address }}:{{ row.port }}
                </div>
                <div class="entity-meta">
                  地区 {{ row.region || '未设置' }} · 权重 {{ row.weight }}
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column
            label="运行状态"
            min-width="220"
          >
            <template #default="{ row }">
              <div class="status-stack">
                <div class="status-chip-row">
                  <el-tag :type="getStatusType(row.status)">
                    {{ getStatusText(row.status) }}
                  </el-tag>
                  <el-tag
                    :type="getSyncStatusType(row.sync_status)"
                    effect="plain"
                  >
                    {{ getSyncStatusText(row.sync_status) }}
                  </el-tag>
                </div>
                <div class="status-metric-grid">
                  <div class="status-metric-card">
                    <span class="metric-label">Xray</span>
                    <strong :class="['status-metric-value', row.xray_running ? 'is-success-text' : 'is-danger-text']">
                      {{ row.xray_running ? '运行中' : '已停止' }}
                    </strong>
                  </div>
                  <div class="status-metric-card">
                    <span class="metric-label">最后心跳</span>
                    <strong
                      class="status-metric-value"
                      :title="formatTime(row.last_seen_at)"
                    >
                      {{ formatCompactTime(row.last_seen_at) }}
                    </strong>
                  </div>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column
            label="运维概况"
            min-width="260"
          >
            <template #default="{ row }">
              <div class="status-stack">
                <div class="status-metric-grid">
                  <div class="status-metric-card">
                    <span class="metric-label">用户负载</span>
                    <strong class="status-metric-value">{{ row.current_users }}/{{ row.max_users || '∞' }}</strong>
                  </div>
                  <div class="status-metric-card">
                    <span class="metric-label">节点延迟</span>
                    <strong :class="['status-metric-value', getLatencyClass(row.latency)]">{{ row.latency }}ms</strong>
                  </div>
                </div>
                <div class="status-inline">
                  <span class="metric-label">内核版本</span>
                  <span
                    class="metric-value metric-value-wrap"
                    :title="formatCoreVersion(row.xray_version)"
                  >
                    {{ formatCoreVersionCompact(row.xray_version) }}
                  </span>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column
            label="操作"
            width="240"
            fixed="right"
            align="right"
          >
            <template #default="{ row }">
              <div class="operation-btns">
                <el-button
                  size="small"
                  type="primary"
                  @click="openOperations(row)"
                >
                  进入运维
                </el-button>
                <el-button
                  size="small"
                  @click="viewNodeDetail(row)"
                >
                  详情
                </el-button>
                <el-button
                  size="small"
                  plain
                  @click="editNode(row)"
                >
                  编辑
                </el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div
        v-if="nodeStore.total > 0"
        class="pagination-container"
      >
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          layout="total, sizes, prev, pager, next"
          :page-sizes="[10, 20, 50, 100]"
          :total="nodeStore.total"
          @current-change="fetchNodes"
          @size-change="handleSizeChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useNodeStore } from '@/stores/node'

const router = useRouter()
const nodeStore = useNodeStore()

const filters = reactive({
  search: '',
  status: '',
  region: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 20
})

const regions = computed(() => {
  const values = new Set()
  for (const node of nodeStore.nodes || []) {
    if (node?.region) {
      values.add(node.region)
    }
  }
  return [...values]
})

const filteredNodes = computed(() => {
  const search = filters.search.trim().toLowerCase()

  return (nodeStore.nodes || []).filter((node) => {
    if (filters.status && node.status !== filters.status) {
      return false
    }

    if (filters.region && node.region !== filters.region) {
      return false
    }

    if (!search) {
      return true
    }

    return [
      node.name,
      node.address,
      node.region
    ].some((field) => String(field || '').toLowerCase().includes(search))
  })
})

const hasActiveFilters = computed(() => Boolean(
  filters.search ||
  filters.status ||
  filters.region
))
const activeFilterCount = computed(() => [
  filters.search.trim(),
  filters.status,
  filters.region
].filter(Boolean).length)

const onlineCount = computed(() => filteredNodes.value.filter((node) => node.status === 'online').length)
const xrayRunningCount = computed(() => filteredNodes.value.filter((node) => node.xray_running).length)
const pendingSyncCount = computed(() => filteredNodes.value.filter((node) => node.sync_status !== 'synced').length)
const averageLatency = computed(() => {
  const available = filteredNodes.value.filter((node) => Number(node.latency) > 0)
  if (!available.length) {
    return 0
  }

  const total = available.reduce((sum, node) => sum + (Number(node.latency) || 0), 0)
  return Math.round(total / available.length)
})

const getStatusType = (status) => {
  const map = { online: 'success', offline: 'info', unhealthy: 'danger' }
  return map[status] || 'info'
}

const getStatusText = (status) => {
  const map = { online: '在线', offline: '离线', unhealthy: '不健康' }
  return map[status] || status || '未知'
}

const getSyncStatusType = (status) => {
  const map = { synced: 'success', pending: 'warning', failed: 'danger' }
  return map[status] || 'info'
}

const getSyncStatusText = (status) => {
  const map = { synced: '已同步', pending: '待同步', failed: '同步失败' }
  return map[status] || status || '未知'
}

const getLatencyClass = (latency) => {
  const value = Number(latency) || 0
  if (value <= 0) return ''
  if (value < 100) return 'latency-good'
  if (value < 300) return 'latency-medium'
  return 'latency-bad'
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const formatCompactTime = (time) => {
  if (!time) return '-'
  const date = new Date(time)
  if (Number.isNaN(date.getTime())) return '-'

  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

const formatCoreVersion = (version) => {
  if (!version) return '-'
  return String(version).split('\n')[0]
}

const formatCoreVersionCompact = (version) => {
  const normalized = formatCoreVersion(version)
  if (normalized === '-') return normalized

  const matched = normalized.match(/(Xray\s+\d+(?:\.\d+)+)/i)
  return matched?.[1] || normalized
}

const fetchNodes = async () => {
  try {
    await nodeStore.fetchNodes({
      limit: pagination.pageSize,
      offset: (pagination.page - 1) * pagination.pageSize,
      status: filters.status || undefined,
      region: filters.region || undefined
    })
  } catch (error) {
    ElMessage.error(error.message || '获取节点运维列表失败')
  }
}

const handleFilterChange = async () => {
  pagination.page = 1
  await fetchNodes()
}

const handleSizeChange = async (pageSize) => {
  pagination.page = 1
  pagination.pageSize = pageSize
  await fetchNodes()
}

const resetFilters = async () => {
  filters.search = ''
  filters.status = ''
  filters.region = ''
  pagination.page = 1
  await fetchNodes()
}

const openOperations = (node) => {
  router.push(`/admin/nodes/${node.id}/operations`)
}

const viewNodeDetail = (node) => {
  router.push(`/admin/nodes/${node.id}`)
}

const editNode = (node) => {
  router.push(`/admin/nodes/${node.id}/edit`)
}

onMounted(async () => {
  await fetchNodes()
})
</script>

<style scoped>
.node-operations-index-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 28px;
  font-weight: 600;
}

.page-subtitle {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
}

.page-actions {
  display: flex;
  gap: 12px;
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.overview-card {
  padding: 16px 18px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.overview-label {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.overview-value {
  display: block;
  margin-top: 8px;
  font-size: 28px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.overview-value.is-success {
  color: var(--el-color-success);
}

.overview-value.is-primary {
  color: var(--el-color-primary);
}

.overview-value.is-warning {
  color: var(--el-color-warning);
}

.toolbar-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px;
  margin-bottom: 20px;
  padding: 16px 18px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.toolbar-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 14px;
}

.toolbar-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toolbar-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.toolbar-description {
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}

.toolbar-filters {
  display: grid;
  grid-template-columns: minmax(260px, 2fr) repeat(2, minmax(150px, 1fr)) auto;
  gap: 12px;
  align-items: center;
}

.toolbar-search {
  width: 100%;
}

.toolbar-filters :deep(.el-select) {
  width: 100%;
}

.toolbar-side {
  display: flex;
  min-width: 260px;
  flex-direction: column;
  align-items: flex-end;
  gap: 12px;
}

.toolbar-summary {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
  text-align: right;
}

.toolbar-chip-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.toolbar-chip {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 12px;
  border-radius: 999px;
  background: var(--el-fill-color-light);
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-regular);
}

.toolbar-chip--primary {
  color: var(--el-color-primary-dark-2);
  background: var(--el-color-primary-light-9);
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.section-subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.table-shell {
  overflow-x: auto;
}

.entity-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.entity-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.entity-meta {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.5;
  word-break: break-word;
}

.status-stack {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 10px;
}

.status-row,
.status-inline {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.status-chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.status-metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.status-metric-card {
  display: flex;
  min-height: 72px;
  flex-direction: column;
  justify-content: space-between;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-fill-color-light);
}

.status-metric-value {
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  color: var(--el-text-color-primary);
}

.metric-label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.metric-value {
  min-width: 0;
  color: var(--el-text-color-primary);
  font-size: 13px;
  text-align: right;
  line-height: 1.5;
}

.metric-value-wrap {
  display: inline-block;
  max-width: 168px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.is-success-text {
  color: var(--el-color-success);
}

.is-danger-text {
  color: var(--el-color-danger);
}

.operation-btns {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}

.latency-good {
  color: var(--el-color-success);
}

.latency-medium {
  color: var(--el-color-warning);
}

.latency-bad {
  color: var(--el-color-danger);
}

/* ── dark mode ── */
:global(.dark) .overview-card {
  background: rgba(30, 41, 59, 0.7);
  border-color: rgba(100, 116, 139, 0.22);
}

:global(.dark) .toolbar-card {
  background: rgba(30, 41, 59, 0.7);
  border-color: rgba(100, 116, 139, 0.22);
}

:global(.dark) .toolbar-chip {
  background: rgba(100, 116, 139, 0.18);
  color: #94a3b8;
}

:global(.dark) .toolbar-chip--primary {
  color: #93c5fd;
  background: rgba(59, 130, 246, 0.16);
}

:global(.dark) .status-metric-card {
  background: rgba(51, 65, 85, 0.5);
  border-color: rgba(100, 116, 139, 0.2);
}

:global(.dark) .status-metric-value {
  color: #e2e8f0;
}

:global(.dark) .metric-label {
  color: #94a3b8;
}

:global(.dark) .metric-value {
  color: #cbd5e1;
}

:global(.dark) .entity-title {
  color: #f1f5f9;
}

:global(.dark) .entity-meta {
  color: #94a3b8;
}

:global(.dark) .section-title {
  color: #f1f5f9;
}

:global(.dark) .section-subtitle {
  color: #94a3b8;
}

@media (max-width: 768px) {
  .node-operations-index-page {
    padding: 12px;
  }

  .page-header,
  .toolbar-card {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-card {
    grid-template-columns: 1fr;
  }

  .page-actions,
  .toolbar-filters {
    width: 100%;
  }

  .toolbar-filters {
    grid-template-columns: 1fr;
  }

  .toolbar-side {
    min-width: 0;
    align-items: flex-start;
  }

  .toolbar-summary {
    text-align: left;
  }

  .toolbar-chip-row {
    justify-content: flex-start;
  }

  .page-actions :deep(.el-button),
  .toolbar-filters :deep(.el-button),
  .toolbar-filters :deep(.el-select),
  .toolbar-search {
    width: 100%;
  }

  .status-metric-grid {
    grid-template-columns: 1fr;
  }

  .status-row {
    flex-direction: column;
    gap: 4px;
  }

  .metric-value,
  .metric-value-wrap {
    max-width: none;
    text-align: left;
  }

  .operation-btns {
    justify-content: flex-start;
  }
}
</style>
