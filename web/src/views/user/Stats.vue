<template>
  <div class="stats-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <div class="header-content">
        <h1 class="page-title">使用统计</h1>
        <p class="page-subtitle">查看您的流量使用情况和历史记录</p>
      </div>
      <el-button @click="exportData" :loading="exporting" :class="{ 'full-width-btn': isMobile }">
        <el-icon><Download /></el-icon>
        导出数据
      </el-button>
    </div>

    <!-- 时间范围选择 -->
    <div class="time-selector">
      <el-radio-group v-model="timeRange" @change="loadStats">
        <el-radio-button value="day">今日</el-radio-button>
        <el-radio-button value="week">本周</el-radio-button>
        <el-radio-button value="month">本月</el-radio-button>
        <el-radio-button value="year">本年</el-radio-button>
      </el-radio-group>

      <el-date-picker
        v-model="customRange"
        type="daterange"
        :style="{ width: customRangeWidth }"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        :shortcuts="dateShortcuts"
        @change="handleCustomRange"
      />
    </div>

    <!-- 统计概览 -->
    <el-row :gutter="20" class="stats-overview">
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-icon upload">
            <el-icon><Upload /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-label">上传流量</div>
            <div class="stat-value">{{ formatTraffic(stats.upload) }}</div>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-icon download">
            <el-icon><Download /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-label">下载流量</div>
            <div class="stat-value">{{ formatTraffic(stats.download) }}</div>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-icon total">
            <el-icon><DataLine /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-label">总流量</div>
            <div class="stat-value">{{ formatTraffic(stats.total) }}</div>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-icon connections">
            <el-icon><Connection /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-label">连接次数</div>
            <div class="stat-value">{{ stats.connections }}</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 流量图表 -->
    <el-card class="chart-card" shadow="never">
      <template #header>
        <div class="card-header">
          <span>流量趋势</span>
          <el-radio-group v-model="chartType" size="small">
            <el-radio-button value="line">折线图</el-radio-button>
            <el-radio-button value="bar">柱状图</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <div v-if="loading" class="chart-loading">
        <el-icon class="loading-icon"><Loading /></el-icon>
        <p>加载数据中...</p>
      </div>

      <div v-else class="chart-container">
        <canvas ref="trafficChart"></canvas>
      </div>
    </el-card>

    <!-- 节点使用统计 -->
    <el-row :gutter="20">
      <el-col :xs="24" :lg="12">
        <el-card class="usage-card" shadow="never">
          <template #header>
            <span>节点使用排行</span>
          </template>

          <div v-if="nodeUsage.length === 0" class="empty-state">
            暂无数据
          </div>

          <div v-else class="usage-list">
            <div 
              v-for="(item, index) in nodeUsage" 
              :key="item.node_id"
              class="usage-item"
            >
              <div class="usage-rank">{{ index + 1 }}</div>
              <div class="usage-info">
                <div class="usage-name">{{ item.node_name }}</div>
                <el-progress 
                  :percentage="item.percentage" 
                  :stroke-width="8"
                  :show-text="false"
                />
              </div>
              <div class="usage-value">{{ formatTraffic(item.traffic) }}</div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12">
        <el-card class="usage-card" shadow="never">
          <template #header>
            <span>协议使用分布</span>
          </template>

          <div v-if="protocolUsage.length === 0" class="empty-state">
            暂无数据
          </div>

          <div v-else class="protocol-chart">
            <canvas ref="protocolChart"></canvas>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 详细记录 -->
    <el-card class="records-card" shadow="never">
      <template #header>
        <span>详细记录</span>
      </template>

      <div class="table-wrap">
      <el-table :data="records" stripe>
        <el-table-column label="日期" prop="date" width="120" />
        <el-table-column label="上传" width="120">
          <template #default="{ row }">
            {{ formatTraffic(row.upload) }}
          </template>
        </el-table-column>
        <el-table-column label="下载" width="120">
          <template #default="{ row }">
            {{ formatTraffic(row.download) }}
          </template>
        </el-table-column>
        <el-table-column label="总计" width="120">
          <template #default="{ row }">
            {{ formatTraffic(row.total) }}
          </template>
        </el-table-column>
        <el-table-column label="连接数" prop="connections" width="100" />
      </el-table>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, nextTick, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { 
  Download, Upload, DataLine, Connection, Loading 
} from '@element-plus/icons-vue'
import { usePortalStatsStore } from '@/stores/portalStats'
import Chart from 'chart.js/auto'
import { useViewport } from '@/composables/useViewport'

const statsStore = usePortalStatsStore()
const { isMobile } = useViewport()

// 引用
const trafficChart = ref(null)
const protocolChart = ref(null)
let trafficChartInstance = null
let protocolChartInstance = null

// 状态
const loading = ref(false)
const exporting = ref(false)
const timeRange = ref('week')
const customRange = ref(null)
const chartType = ref('line')

// 数据
const stats = reactive({
  upload: 0,
  download: 0,
  total: 0,
  connections: 0
})

const nodeUsage = ref([])
const protocolUsage = ref([])
const records = ref([])
const chartData = ref({ labels: [], upload: [], download: [] })
const customRangeWidth = computed(() => (isMobile.value ? '100%' : '320px'))

// 日期快捷选项
const dateShortcuts = [
  {
    text: '最近一周',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 7)
      return [start, end]
    }
  },
  {
    text: '最近一月',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 30)
      return [start, end]
    }
  },
  {
    text: '最近三月',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 90)
      return [start, end]
    }
  }
]

// 方法
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

function handleCustomRange(range) {
  if (range) {
    timeRange.value = 'custom'
    loadStats()
  }
}

async function loadStats() {
  loading.value = true
  try {
    const params = { period: timeRange.value }
    if (timeRange.value === 'custom' && customRange.value) {
      params.start_date = customRange.value[0].toISOString().split('T')[0]
      params.end_date = customRange.value[1].toISOString().split('T')[0]
    }

    const data = await statsStore.fetchStats(params)
    
    // 安全地提取数据
    stats.upload = data?.summary?.upload || 0
    stats.download = data?.summary?.download || 0
    stats.total = data?.summary?.total || 0
    stats.connections = data?.summary?.connections || 0
    
    nodeUsage.value = Array.isArray(data?.node_usage) ? data.node_usage : []
    protocolUsage.value = Array.isArray(data?.protocol_usage) ? data.protocol_usage : []
    records.value = Array.isArray(data?.records) ? data.records : []
    chartData.value = data?.chart_data || { labels: [], upload: [], download: [] }

    await nextTick()
    renderTrafficChart()
    renderProtocolChart()
  } catch (error) {
    console.error('加载统计数据失败:', error)
    ElMessage.error(error?.message || '加载统计数据失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

function renderTrafficChart() {
  if (!trafficChart.value) return

  if (trafficChartInstance) {
    trafficChartInstance.destroy()
  }

  const ctx = trafficChart.value.getContext('2d')
  trafficChartInstance = new Chart(ctx, {
    type: chartType.value,
    data: {
      labels: chartData.value.labels,
      datasets: [
        {
          label: '上传',
          data: chartData.value.upload,
          borderColor: '#67c23a',
          backgroundColor: chartType.value === 'bar' ? 'rgba(103, 194, 58, 0.5)' : 'rgba(103, 194, 58, 0.1)',
          fill: chartType.value === 'line',
          tension: 0.4
        },
        {
          label: '下载',
          data: chartData.value.download,
          borderColor: '#409eff',
          backgroundColor: chartType.value === 'bar' ? 'rgba(64, 158, 255, 0.5)' : 'rgba(64, 158, 255, 0.1)',
          fill: chartType.value === 'line',
          tension: 0.4
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          position: 'top'
        },
        tooltip: {
          callbacks: {
            label: (context) => {
              return `${context.dataset.label}: ${formatTraffic(context.raw)}`
            }
          }
        }
      },
      scales: {
        y: {
          beginAtZero: true,
          ticks: {
            callback: (value) => formatTraffic(value)
          }
        }
      }
    }
  })
}

function renderProtocolChart() {
  if (!protocolChart.value || protocolUsage.value.length === 0) return

  if (protocolChartInstance) {
    protocolChartInstance.destroy()
  }

  const ctx = protocolChart.value.getContext('2d')
  protocolChartInstance = new Chart(ctx, {
    type: 'doughnut',
    data: {
      labels: protocolUsage.value.map(p => p.protocol),
      datasets: [{
        data: protocolUsage.value.map(p => p.traffic),
        backgroundColor: [
          '#409eff',
          '#67c23a',
          '#e6a23c',
          '#f56c6c',
          '#909399'
        ]
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          position: 'right'
        },
        tooltip: {
          callbacks: {
            label: (context) => {
              return `${context.label}: ${formatTraffic(context.raw)}`
            }
          }
        }
      }
    }
  })
}

async function exportData() {
  exporting.value = true
  try {
    await statsStore.exportStats({ range: timeRange.value })
    ElMessage.success('数据导出成功')
  } catch (error) {
    ElMessage.error('导出失败')
  } finally {
    exporting.value = false
  }
}

// 监听图表类型变化
watch(chartType, () => {
  renderTrafficChart()
})

onMounted(() => {
  loadStats()
})
</script>

<style scoped>
.stats-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

/* 时间选择器 */
.time-selector {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

/* 统计概览 */
.stats-overview {
  margin-bottom: 20px;
}

.stat-card {
  display: flex;
  align-items: center;
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  margin-bottom: 20px;
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  margin-right: 16px;
}

.stat-icon.upload {
  background: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.stat-icon.download {
  background: rgba(64, 158, 255, 0.1);
  color: #409eff;
}

.stat-icon.total {
  background: rgba(230, 162, 60, 0.1);
  color: #e6a23c;
}

.stat-icon.connections {
  background: rgba(144, 147, 153, 0.1);
  color: #909399;
}

.stat-label {
  font-size: 13px;
  color: #909399;
  margin-bottom: 4px;
}

.stat-value {
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}

/* 图表卡片 */
.chart-card,
.usage-card,
.records-card {
  margin-bottom: 20px;
  border-radius: 8px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.chart-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 300px;
  color: #909399;
}

.loading-icon {
  font-size: 32px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.chart-container {
  height: 300px;
}

.chart-container canvas,
.protocol-chart canvas {
  max-width: 100%;
}

/* 使用统计 */
.empty-state {
  text-align: center;
  padding: 40px 0;
  color: #909399;
}

.usage-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.usage-item {
  display: flex;
  align-items: center;
  gap: 12px;
}

.usage-rank {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #f5f7fa;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  color: #909399;
}

.usage-item:nth-child(1) .usage-rank {
  background: #ffd700;
  color: #fff;
}

.usage-item:nth-child(2) .usage-rank {
  background: #c0c0c0;
  color: #fff;
}

.usage-item:nth-child(3) .usage-rank {
  background: #cd7f32;
  color: #fff;
}

.usage-info {
  flex: 1;
}

.usage-name {
  font-size: 14px;
  color: #303133;
  margin-bottom: 4px;
}

.usage-value {
  font-size: 14px;
  font-weight: 500;
  color: #606266;
  min-width: 80px;
  text-align: right;
}

.protocol-chart {
  height: 250px;
}

.table-wrap {
  overflow-x: auto;
}

.table-wrap :deep(.el-table) {
  min-width: 580px;
}

.full-width-btn {
  width: 100%;
}

/* 响应式 */
@media (max-width: 768px) {
  .stats-page {
    padding: 12px;
  }

  .page-header {
    flex-direction: column;
    gap: 16px;
  }

  .time-selector {
    flex-direction: column;
    align-items: stretch;
  }

  .chart-container {
    height: 260px;
  }
}
</style>
