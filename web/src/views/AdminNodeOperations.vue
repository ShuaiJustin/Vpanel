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
          class="nodes-table"
          :empty-text="hasActiveFilters ? '暂无匹配节点' : '暂无节点'"
        >
          <el-table-column
            label="节点对象"
            min-width="280"
          >
            <template #default="{ row }">
              <div class="entity-cell">
                <div
                  class="node-address"
                  :title="`${row.address}:${row.port}`"
                >
                  {{ row.address }}:{{ row.port }}
                </div>
                <div class="entity-cell__header">
                  <span class="entity-cell__title">{{ row.name }}</span>
                  <span :class="['metric-pill', getStatusPillClass(row.status)]">
                    {{ getStatusText(row.status) }}
                  </span>
                </div>
                <div class="entity-cell__meta">
                  <span>ID：{{ row.id }}</span>
                  <span>地区：{{ row.region || '未设置' }}</span>
                  <span>权重：{{ row.weight }}</span>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column
            label="运行状态"
            min-width="240"
          >
            <template #default="{ row }">
              <div class="stack-cell">
                <div class="stack-item stack-item--inline">
                  <span class="stack-label">运行状态</span>
                  <span :class="['metric-pill', getStatusPillClass(row.status)]">
                    {{ getStatusText(row.status) }}
                  </span>
                </div>
                <div class="stack-item stack-item--inline">
                  <span class="stack-label">同步状态</span>
                  <span :class="['metric-pill', getSyncPillClass(row.sync_status)]">
                    {{ getSyncStatusText(row.sync_status) }}
                  </span>
                </div>
                <div class="stack-item">
                  <span class="stack-label">Xray 运行</span>
                  <span :class="['stack-value', 'is-strong', row.xray_running ? 'is-success' : 'is-danger']">
                    {{ row.xray_running ? '运行中' : '已停止' }}
                  </span>
                </div>
                <div class="stack-item">
                  <span class="stack-label">最后心跳</span>
                  <span
                    class="stack-value"
                    :title="formatTime(row.last_seen_at)"
                  >
                    {{ formatCompactTime(row.last_seen_at) }}
                  </span>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column
            label="运维概况"
            min-width="240"
          >
            <template #default="{ row }">
              <div class="stack-cell">
                <div class="stack-item">
                  <span class="stack-label">用户负载</span>
                  <span class="stack-value is-strong">
                    {{ row.current_users }}/{{ row.max_users || '∞' }}
                  </span>
                </div>
                <el-progress
                  v-if="row.max_users > 0"
                  :percentage="getLoadPercent(row)"
                  :stroke-width="6"
                  :show-text="false"
                  :status="getLoadStatus(row)"
                />
                <div class="stack-item">
                  <span class="stack-label">节点延迟</span>
                  <span :class="['stack-value', getLatencyValueClass(row.latency)]">
                    {{ row.latency }}ms
                  </span>
                </div>
                <div class="stack-item">
                  <span class="stack-label">内核版本</span>
                  <span
                    class="stack-value"
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
            width="220"
            fixed="right"
            align="right"
          >
            <template #default="{ row }">
              <div class="operation-btns">
                <el-button
                  size="small"
                  type="primary"
                  class="row-action"
                  @click="openOperations(row)"
                >
                  进入运维
                </el-button>
                <el-button
                  size="small"
                  class="row-action row-action--primary"
                  @click="viewNodeDetail(row)"
                >
                  详情
                </el-button>
                <el-button
                  size="small"
                  plain
                  class="row-action"
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
import { computed, onMounted, onUnmounted, reactive, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useNodeStore } from '@/stores/node'
import { debounce } from '@/utils/debounce'
import { extractErrorMessage } from '@/utils/entitlement'

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

const getStatusText = (status) => {
  const map = { online: '在线', offline: '离线', unhealthy: '不健康' }
  return map[status] || status || '未知'
}

const getStatusPillClass = (status) => {
  const classes = {
    online: 'is-success',
    offline: 'is-muted',
    unhealthy: 'is-danger'
  }
  return classes[status] || 'is-muted'
}

const getSyncStatusText = (status) => {
  const map = { synced: '已同步', pending: '待同步', failed: '同步失败' }
  return map[status] || status || '未知'
}

const getSyncPillClass = (status) => {
  const classes = {
    synced: 'is-success',
    pending: 'is-warning',
    failed: 'is-danger'
  }
  return classes[status] || 'is-muted'
}

const getLatencyValueClass = (latency) => {
  const value = Number(latency) || 0
  if (value <= 0) return ''
  if (value < 100) return 'is-success'
  if (value < 300) return 'is-warning'
  return 'is-danger'
}

const getLoadPercent = (node) => {
  if (!node.max_users) return 0
  return Math.min(100, Math.round((node.current_users / node.max_users) * 100))
}

const getLoadStatus = (node) => {
  if (!node.max_users) return ''
  const ratio = node.current_users / node.max_users
  if (ratio >= 0.9) return 'exception'
  if (ratio >= 0.7) return 'warning'
  return 'success'
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
      region: filters.region || undefined,
      search: filters.search.trim() || undefined
    })
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '获取节点运维列表失败')
  }
}

const debouncedSearchNodes = debounce(async () => {
  pagination.page = 1
  await fetchNodes()
}, 300)

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

watch(
  () => filters.search,
  () => {
    debouncedSearchNodes()
  }
)

onUnmounted(() => {
  debouncedSearchNodes.cancel?.()
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

.nodes-table {
  width: 100%;
  min-width: 980px;
}

.node-address {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  padding: 6px 10px;
  border-radius: 12px;
  background: rgba(37, 99, 235, 0.08);
  color: #1d4ed8;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
  word-break: break-all;
}

.node-operations-index-page :deep(.nodes-table td.el-table__cell),
.node-operations-index-page :deep(.nodes-table th.el-table__cell) {
  vertical-align: middle !important;
}

.node-operations-index-page :deep(.nodes-table .el-table__header-wrapper .cell) {
  display: flex;
  align-items: center;
  min-height: 52px;
}

.node-operations-index-page
  :deep(.nodes-table .el-table__body td.el-table__cell > .cell) {
  display: flex;
  align-items: center;
  min-height: 100%;
  padding-top: 14px;
  padding-bottom: 14px;
}

.node-operations-index-page :deep(.nodes-table .entity-cell),
.node-operations-index-page :deep(.nodes-table .stack-cell),
.node-operations-index-page :deep(.nodes-table .operation-btns) {
  width: 100%;
}

.node-operations-index-page :deep(.nodes-table .entity-cell),
.node-operations-index-page :deep(.nodes-table .stack-cell) {
  justify-content: center;
}

.node-operations-index-page :deep(.nodes-table .operation-btns) {
  align-items: center;
  gap: 8px;
  min-height: 100%;
}

.node-operations-index-page :deep(.row-action) {
  min-width: 54px;
}

.node-operations-index-page :deep(.row-action--primary) {
  background: #eff6ff;
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

/* ── dark mode ── */
:global(.dark) .node-address {
  color: #93c5fd;
  background: rgba(59, 130, 246, 0.14);
}

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

:global(.dark) .section-title {
  color: #f1f5f9;
}

:global(.dark) .section-subtitle {
  color: #94a3b8;
}

:global(.dark) .node-operations-index-page :deep(.row-action--primary) {
  background: rgba(59, 130, 246, 0.14);
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

  .operation-btns {
    justify-content: flex-start;
  }
}
</style>
