<template>
  <div class="traffic-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">流量统计</h1>
        <p class="page-subtitle">查看后台总流量趋势和用户流量排行</p>
      </div>
      <div class="header-actions">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          :style="{ width: datePickerWidth }"
        />
        <el-button type="primary" :loading="loading" @click="fetchTrafficData">查询</el-button>
        <el-button :disabled="filteredUsers.length === 0" @click="exportTrafficData">导出 CSV</el-button>
      </div>
    </div>

    <el-row :gutter="isMobile ? 12 : 20" class="summary-row">
      <el-col :span="statCardSpan">
        <el-card class="summary-card" shadow="never">
          <div class="summary-label">总流量</div>
          <div class="summary-value">{{ formatTraffic(totalStats.totalTraffic) }}</div>
        </el-card>
      </el-col>
      <el-col :span="statCardSpan">
        <el-card class="summary-card" shadow="never">
          <div class="summary-label">总上传</div>
          <div class="summary-value">{{ formatTraffic(totalStats.uploadTraffic) }}</div>
        </el-card>
      </el-col>
      <el-col :span="statCardSpan">
        <el-card class="summary-card" shadow="never">
          <div class="summary-label">总下载</div>
          <div class="summary-value">{{ formatTraffic(totalStats.downloadTraffic) }}</div>
        </el-card>
      </el-col>
      <el-col :span="statCardSpan">
        <el-card class="summary-card" shadow="never">
          <div class="summary-label">活跃用户</div>
          <div class="summary-value">{{ totalStats.activeUsers }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="chart-card">
      <template #header>
        <div class="card-header">
          <span>流量趋势</span>
          <span class="card-meta">{{ rangeLabel }}</span>
        </div>
      </template>
      <div ref="trafficChartRef" class="traffic-chart"></div>
    </el-card>

    <el-card shadow="never" class="table-card">
      <template #header>
        <div class="card-header">
          <span>用户流量排行</span>
          <div class="table-actions">
            <el-input
              v-model="searchQuery"
              clearable
              placeholder="搜索用户名/邮箱"
              :style="{ width: searchWidth }"
            />
            <el-select v-model="sortBy" :style="{ width: sortWidth }">
              <el-option label="总流量降序" value="total_desc" />
              <el-option label="总流量升序" value="total_asc" />
              <el-option label="用户名" value="username" />
            </el-select>
          </div>
        </div>
      </template>

      <div class="table-shell">
        <el-table :data="filteredUsers" v-loading="loading" style="width: 100%">
          <el-table-column prop="username" label="用户名" min-width="140" />
          <el-table-column prop="email" label="邮箱" min-width="180" />
          <el-table-column prop="proxy_count" label="代理数" width="100" />
          <el-table-column label="总流量" width="150">
            <template #default="{ row }">
              {{ formatTraffic(row.total) }}
            </template>
          </el-table-column>
          <el-table-column label="上传" width="140">
            <template #default="{ row }">
              {{ formatTraffic(row.upload) }}
            </template>
          </el-table-column>
          <el-table-column label="下载" width="140">
            <template #default="{ row }">
              {{ formatTraffic(row.download) }}
            </template>
          </el-table-column>
          <el-table-column label="使用占比" min-width="220">
            <template #default="{ row }">
              <template v-if="row.traffic_limit > 0">
                <el-progress
                  :percentage="getTrafficPercentage(row)"
                  :status="getTrafficStatus(row)"
                />
                <div class="progress-note">
                  {{ formatTraffic(row.total) }} / {{ formatTraffic(row.traffic_limit) }}
                </div>
              </template>
              <span v-else class="limit-note">不限额</span>
            </template>
          </el-table-column>
          <el-table-column label="最后活跃" min-width="180">
            <template #default="{ row }">
              {{ formatDateTime(row.last_active) }}
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import { statsApi } from '@/api'
import { useViewport } from '@/composables/useViewport'

const { isMobile, isTablet } = useViewport({ mobileBreakpoint: 768, tabletBreakpoint: 1200 })

const loading = ref(false)
const dateRange = ref([])
const searchQuery = ref('')
const sortBy = ref('total_desc')
const users = ref([])
const timeline = ref([])
const trafficChartRef = ref(null)

const totalStats = ref({
  totalTraffic: 0,
  uploadTraffic: 0,
  downloadTraffic: 0,
  activeUsers: 0
})

const statCardSpan = computed(() => (isMobile.value ? 24 : isTablet.value ? 12 : 6))
const datePickerWidth = computed(() => (isMobile.value ? '100%' : isTablet.value ? '320px' : '360px'))
const searchWidth = computed(() => (isMobile.value ? '100%' : '220px'))
const sortWidth = computed(() => (isMobile.value ? '100%' : '160px'))
const rangeLabel = computed(() => {
  if (!Array.isArray(dateRange.value) || dateRange.value.length !== 2) {
    return '最近 30 天'
  }
  return `${formatDate(dateRange.value[0])} 至 ${formatDate(dateRange.value[1])}`
})

const filteredUsers = computed(() => {
  const keyword = searchQuery.value.trim().toLowerCase()
  let result = users.value

  if (keyword) {
    result = result.filter((item) => {
      const username = (item.username || '').toLowerCase()
      const email = (item.email || '').toLowerCase()
      return username.includes(keyword) || email.includes(keyword)
    })
  }

  const sorted = [...result]
  switch (sortBy.value) {
    case 'total_asc':
      sorted.sort((a, b) => a.total - b.total)
      break
    case 'username':
      sorted.sort((a, b) => (a.username || '').localeCompare(b.username || ''))
      break
    default:
      sorted.sort((a, b) => b.total - a.total)
      break
  }
  return sorted
})

let trafficChart = null

function createDefaultRange() {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - 30)
  return [start, end]
}

function getRangeParams() {
  const [start, end] = Array.isArray(dateRange.value) && dateRange.value.length === 2
    ? dateRange.value
    : createDefaultRange()

  return {
    period: 'custom',
    start: start.toISOString(),
    end: end.toISOString()
  }
}

async function fetchTrafficData() {
  loading.value = true
  try {
    const params = getRangeParams()
    const [trafficResponse, detailResponse, userResponse, dashboardResponse] = await Promise.all([
      statsApi.getTrafficStats(params),
      statsApi.getDetailedStats(params),
      statsApi.getUserStats(params),
      statsApi.getDashboardStats()
    ])

    const trafficData = trafficResponse?.data || {}
    totalStats.value = {
      totalTraffic: trafficData.total || 0,
      uploadTraffic: trafficData.up || 0,
      downloadTraffic: trafficData.down || 0,
      activeUsers: dashboardResponse?.data?.active_users || 0
    }

    timeline.value = detailResponse?.data?.timeline || []
    users.value = (userResponse?.data || []).map((item) => ({
      user_id: item.user_id,
      username: item.username || '-',
      email: item.email || '-',
      proxy_count: item.proxy_count || 0,
      upload: item.upload || 0,
      download: item.download || 0,
      total: item.total || 0,
      traffic_limit: item.traffic_limit || 0,
      last_active: item.last_active || ''
    }))

    renderTrafficChart()
  } catch (error) {
    users.value = []
    timeline.value = []
    totalStats.value = {
      totalTraffic: 0,
      uploadTraffic: 0,
      downloadTraffic: 0,
      activeUsers: 0
    }
    renderTrafficChart()
    ElMessage.error(error?.message || '获取流量统计失败')
  } finally {
    loading.value = false
  }
}

function renderTrafficChart() {
  if (!trafficChartRef.value) {
    return
  }

  if (!trafficChart) {
    trafficChart = echarts.init(trafficChartRef.value)
  }

  trafficChart.setOption({
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        const title = params?.[0]?.axisValue || ''
        const lines = params.map((item) => `${item.seriesName}: ${formatTraffic(item.value)}`)
        return [title, ...lines].join('<br>')
      }
    },
    legend: {
      data: ['上传', '下载']
    },
    grid: {
      left: 16,
      right: 16,
      top: 48,
      bottom: 16,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: timeline.value.map((item) => formatTimelineLabel(item.time))
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        formatter: (value) => formatTraffic(value)
      }
    },
    series: [
      {
        name: '上传',
        type: 'line',
        smooth: true,
        areaStyle: { opacity: 0.12 },
        data: timeline.value.map((item) => item.upload || 0),
        color: '#67c23a'
      },
      {
        name: '下载',
        type: 'line',
        smooth: true,
        areaStyle: { opacity: 0.12 },
        data: timeline.value.map((item) => item.download || 0),
        color: '#409eff'
      }
    ]
  })
}

function exportTrafficData() {
  const rows = [
    ['用户名', '邮箱', '代理数', '上传', '下载', '总流量', '流量限制', '最后活跃'],
    ...filteredUsers.value.map((item) => [
      item.username,
      item.email,
      item.proxy_count,
      item.upload,
      item.download,
      item.total,
      item.traffic_limit,
      item.last_active
    ])
  ]

  const csv = rows
    .map((row) => row.map((value) => `"${String(value ?? '').replace(/"/g, '""')}"`).join(','))
    .join('\n')

  const blob = new Blob([`\ufeff${csv}`], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `traffic-report-${Date.now()}.csv`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function getTrafficPercentage(user) {
  if (!user?.traffic_limit) {
    return 0
  }
  return Math.min(100, Math.round((user.total / user.traffic_limit) * 100))
}

function getTrafficStatus(user) {
  const percentage = getTrafficPercentage(user)
  if (percentage >= 90) return 'exception'
  if (percentage >= 70) return 'warning'
  return 'success'
}

function formatTraffic(bytes) {
  const value = Number(bytes) || 0
  if (value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(2)} ${units[index]}`
}

function formatDateTime(value) {
  if (!value) return '暂无'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '暂无'
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function formatDate(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function formatTimelineLabel(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value || ''
  }
  return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function handleResize() {
  trafficChart?.resize()
}

onMounted(() => {
  dateRange.value = createDefaultRange()
  fetchTrafficData()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  trafficChart?.dispose()
  trafficChart = null
})
</script>

<style scoped>
.traffic-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 24px;
  font-weight: 600;
}

.page-subtitle {
  margin: 0;
  color: #909399;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.summary-row {
  margin-bottom: 20px;
}

.summary-card {
  min-height: 128px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.summary-label {
  color: #909399;
  font-size: 13px;
  margin-bottom: 12px;
}

.summary-value {
  font-size: 28px;
  font-weight: 600;
  line-height: 1.2;
}

.chart-card,
.table-card {
  margin-bottom: 20px;
}

.traffic-chart {
  width: 100%;
  height: 340px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.card-meta {
  color: #909399;
  font-size: 13px;
}

.table-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.table-shell {
  overflow-x: auto;
}

.progress-note,
.limit-note {
  margin-top: 8px;
  color: #909399;
  font-size: 12px;
}

@media (max-width: 768px) {
  .traffic-page {
    padding: 16px;
  }

  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .header-actions,
  .table-actions {
    width: 100%;
  }

  .summary-value {
    font-size: 24px;
  }
}
</style>
