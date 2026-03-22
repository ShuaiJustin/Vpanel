<template>
  <div class="node-comparison-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          节点性能对比
        </h1>
        <p class="page-subtitle">
          对比节点延迟、负载、同步状态和基础运行指标
        </p>
      </div>
      <div class="header-actions">
        <el-button
          :loading="loading"
          @click="fetchNodes"
        >
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <!-- 节点选择 -->
    <el-card
      shadow="never"
      class="selection-card"
    >
      <div class="selection-header">
        <span>选择要对比的节点（最多选择 5 个）</span>
        <el-button
          v-if="selectedNodes.length > 0"
          link
          @click="clearSelection"
        >
          清除选择
        </el-button>
      </div>
      <el-checkbox-group
        v-model="selectedNodeIds"
        :max="5"
      >
        <el-checkbox
          v-for="node in nodeStore.nodes"
          :key="node.id"
          :label="node.id"
          :disabled="selectedNodeIds.length >= 5 && !selectedNodeIds.includes(node.id)"
        >
          <span class="node-checkbox-label">
            <el-tag
              :type="getStatusType(node.status)"
              size="small"
            >
              {{ getStatusText(node.status) }}
            </el-tag>
            {{ node.name }}
            <span class="node-region">({{ node.region || '未知' }})</span>
          </span>
        </el-checkbox>
      </el-checkbox-group>
    </el-card>

    <!-- 对比表格 -->
    <el-card
      v-if="selectedNodes.length > 0"
      shadow="never"
      class="comparison-card"
    >
      <template #header>
        <span>性能对比</span>
      </template>
      <div class="table-shell">
        <el-table
          :data="comparisonData"
          border
          style="width: 100%"
        >
          <el-table-column
            prop="metric"
            label="指标"
            width="150"
            fixed
          />
          <el-table-column
            v-for="node in selectedNodes"
            :key="node.id"
            :label="node.name"
            min-width="150"
          >
            <template #default="{ row }">
              <div
                class="metric-cell"
                :class="getCellClass(row, node)"
              >
                <span class="metric-value">{{ getMetricValue(row, node) }}</span>
                <el-icon
                  v-if="row.key !== 'name' && row.key !== 'region'"
                  class="rank-icon"
                >
                  <Trophy v-if="isTopPerformer(row, node)" />
                </el-icon>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>

    <!-- 可视化对比 -->
    <el-row
      v-if="selectedNodes.length > 0"
      :gutter="isMobile ? 12 : 20"
    >
      <el-col :span="chartSpan">
        <el-card
          shadow="never"
          class="chart-card"
        >
          <template #header>
            <span>延迟对比</span>
          </template>
          <div class="bar-chart">
            <div
              v-for="node in selectedNodes"
              :key="node.id"
              class="bar-item"
            >
              <span class="bar-label">{{ node.name }}</span>
              <div class="bar-container">
                <div
                  class="bar-fill"
                  :class="getLatencyClass(node.latency)"
                  :style="{ width: getLatencyBarWidth(node.latency) + '%' }"
                />
              </div>
              <span class="bar-value">{{ node.latency }}ms</span>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="chartSpan">
        <el-card
          shadow="never"
          class="chart-card"
        >
          <template #header>
            <span>负载对比</span>
          </template>
          <div class="bar-chart">
            <div
              v-for="node in selectedNodes"
              :key="node.id"
              class="bar-item"
            >
              <span class="bar-label">{{ node.name }}</span>
              <div class="bar-container">
                <div
                  class="bar-fill"
                  :class="getLoadClass(node)"
                  :style="{ width: getLoadPercentage(node) + '%' }"
                />
              </div>
              <span class="bar-value">{{ node.current_users }}/{{ node.max_users || '∞' }}</span>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 详细数据表 -->
    <el-card
      v-if="selectedNodes.length > 0"
      shadow="never"
      class="detail-card"
    >
      <template #header>
        <span>详细数据</span>
      </template>
      <div class="table-shell">
        <el-table
          :data="selectedNodes"
          border
          style="width: 100%"
        >
          <el-table-column
            prop="name"
            label="名称"
            width="150"
          />
          <el-table-column
            label="状态"
            width="100"
          >
            <template #default="{ row }">
              <el-tag
                :type="getStatusType(row.status)"
                size="small"
              >
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column
            prop="region"
            label="地区"
            width="100"
          />
          <el-table-column
            prop="address"
            label="地址"
            min-width="150"
          />
          <el-table-column
            label="延迟"
            width="100"
            sortable
            :sort-method="sortByLatency"
          >
            <template #default="{ row }">
              <span :class="getLatencyClass(row.latency)">{{ row.latency }}ms</span>
            </template>
          </el-table-column>
          <el-table-column
            label="负载"
            width="120"
            sortable
            :sort-method="sortByLoad"
          >
            <template #default="{ row }">
              {{ row.current_users }}/{{ row.max_users || '∞' }}
            </template>
          </el-table-column>
          <el-table-column
            prop="weight"
            label="权重"
            width="80"
            sortable
          />
          <el-table-column
            label="同步状态"
            width="100"
          >
            <template #default="{ row }">
              <el-tag
                :type="getSyncStatusType(row.sync_status)"
                size="small"
              >
                {{ getSyncStatusText(row.sync_status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column
            label="最后在线"
            width="180"
          >
            <template #default="{ row }">
              {{ formatTime(row.last_seen_at) }}
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>

    <el-empty
      v-if="selectedNodes.length === 0"
      description="请选择要对比的节点"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Trophy } from '@element-plus/icons-vue'
import { useNodeStore } from '@/stores/node'
import { useViewport } from '@/composables/useViewport'

const nodeStore = useNodeStore()
const { isMobile } = useViewport()

const loading = ref(false)
const selectedNodeIds = ref([])
const chartSpan = computed(() => (isMobile.value ? 24 : 12))

const selectedNodes = computed(() => {
  return nodeStore.nodes.filter(n => selectedNodeIds.value.includes(n.id))
})

const comparisonData = computed(() => [
  { key: 'name', metric: '名称' },
  { key: 'region', metric: '地区' },
  { key: 'status', metric: '状态' },
  { key: 'latency', metric: '延迟', unit: 'ms', lowerBetter: true },
  { key: 'current_users', metric: '当前用户' },
  { key: 'max_users', metric: '最大用户' },
  { key: 'load', metric: '负载率', unit: '%', lowerBetter: true },
  { key: 'weight', metric: '权重' },
  { key: 'sync_status', metric: '同步状态' }
])

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
  if (!latency) return 'good'
  if (latency < 100) return 'good'
  if (latency < 300) return 'medium'
  return 'bad'
}

const getLoadClass = (node) => {
  if (!node.max_users) return 'good'
  const ratio = node.current_users / node.max_users
  if (ratio < 0.5) return 'good'
  if (ratio < 0.8) return 'medium'
  return 'bad'
}

const getLoadPercentage = (node) => {
  if (!node.max_users) return 30 // 无限制时显示 30%
  return Math.min(100, Math.round((node.current_users / node.max_users) * 100))
}

const getLatencyBarWidth = (latency) => {
  const maxLatency = Math.max(...selectedNodes.value.map(n => n.latency || 0), 500)
  return Math.min(100, Math.round((latency / maxLatency) * 100))
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const getMetricValue = (row, node) => {
  const key = row.key
  
  if (key === 'status') {
    return getStatusText(node.status)
  }
  if (key === 'sync_status') {
    return getSyncStatusText(node.sync_status)
  }
  if (key === 'load') {
    if (!node.max_users) return '-'
    return Math.round((node.current_users / node.max_users) * 100) + '%'
  }
  if (key === 'max_users') {
    return node.max_users || '无限制'
  }
  
  const value = node[key]
  if (value === undefined || value === null) return '-'
  if (row.unit) return value + row.unit
  return value
}

const getCellClass = (row, node) => {
  if (row.key === 'latency') {
    return getLatencyClass(node.latency)
  }
  if (row.key === 'load') {
    return getLoadClass(node)
  }
  if (row.key === 'status') {
    return node.status
  }
  return ''
}

const isTopPerformer = (row, node) => {
  if (selectedNodes.value.length < 2) return false
  
  const key = row.key
  if (!['latency', 'load', 'current_users', 'weight'].includes(key)) return false
  
  const values = selectedNodes.value.map(n => {
    if (key === 'load') {
      return n.max_users ? n.current_users / n.max_users : 0
    }
    return n[key] || 0
  })
  
  const nodeValue = key === 'load' 
    ? (node.max_users ? node.current_users / node.max_users : 0)
    : (node[key] || 0)
  
  if (row.lowerBetter) {
    return nodeValue === Math.min(...values) && nodeValue > 0
  }
  return nodeValue === Math.max(...values)
}

const sortByLatency = (a, b) => (a.latency || 0) - (b.latency || 0)

const sortByLoad = (a, b) => {
  const loadA = a.max_users ? a.current_users / a.max_users : 0
  const loadB = b.max_users ? b.current_users / b.max_users : 0
  return loadA - loadB
}

const clearSelection = () => {
  selectedNodeIds.value = []
}

const fetchNodes = async () => {
  loading.value = true
  try {
    await nodeStore.fetchNodes()
  } catch (e) {
    ElMessage.error(e.message || '获取节点列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(fetchNodes)
</script>

<style scoped>
.node-comparison-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
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

.selection-card {
  margin-bottom: 20px;
}

.selection-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.node-checkbox-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.node-region {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.comparison-card, .chart-card, .detail-card {
  margin-bottom: 20px;
}

.metric-cell {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.metric-cell.good { color: var(--el-color-success); }
.metric-cell.medium { color: var(--el-color-warning); }
.metric-cell.bad { color: var(--el-color-danger); }
.metric-cell.online { color: var(--el-color-success); }
.metric-cell.offline { color: var(--el-color-info); }
.metric-cell.unhealthy { color: var(--el-color-danger); }

.rank-icon {
  color: var(--el-color-warning);
  font-size: 16px;
}

.bar-chart {
  padding: 10px 0;
}

.bar-item {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}

.bar-item:last-child {
  margin-bottom: 0;
}

.bar-label {
  width: 100px;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bar-container {
  flex: 1;
  height: 20px;
  background: var(--el-fill-color-light);
  border-radius: 10px;
  overflow: hidden;
  margin: 0 12px;
}

.bar-fill {
  height: 100%;
  border-radius: 10px;
  transition: width 0.3s;
}

.bar-fill.good { background: var(--el-color-success); }
.bar-fill.medium { background: var(--el-color-warning); }
.bar-fill.bad { background: var(--el-color-danger); }

.bar-value {
  width: 80px;
  text-align: right;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.table-shell {
  overflow-x: auto;
}

.table-shell :deep(.el-table) {
  min-width: 760px;
}

@media (max-width: 768px) {
  .node-comparison-page {
    padding: 12px;
  }

  .page-header,
  .header-actions,
  .selection-header,
  .bar-item,
  .metric-cell {
    flex-direction: column;
    align-items: flex-start;
  }

  .header-actions {
    width: 100%;
  }

  .header-actions .el-button {
    width: 100%;
  }

  .node-checkbox-label {
    display: flex;
    flex-wrap: wrap;
    line-height: 1.6;
  }

  .bar-label,
  .bar-value {
    width: 100%;
    text-align: left;
  }

  .bar-container {
    width: 100%;
    margin: 8px 0;
  }
}
</style>
