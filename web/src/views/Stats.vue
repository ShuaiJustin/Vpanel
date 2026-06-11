<template>
  <div class="stats-page">
    <!-- 概览卡片 -->
    <el-row
      :gutter="isMobile ? 12 : 20"
      class="overview-row"
    >
      <el-col :span="statCardSpan">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon users">
              <el-icon size="32">
                <User />
              </el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">
                {{ dashboardStats.total_users }}
              </div>
              <div class="stat-label">
                总用户数
              </div>
            </div>
          </div>
          <div class="stat-footer">
            <span>活跃用户: {{ dashboardStats.active_users }}</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="statCardSpan">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon proxies">
              <el-icon size="32">
                <Connection />
              </el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">
                {{ dashboardStats.total_proxies }}
              </div>
              <div class="stat-label">
                总代理数
              </div>
            </div>
          </div>
          <div class="stat-footer">
            <span>活跃代理: {{ dashboardStats.active_proxies }}</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="statCardSpan">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon traffic">
              <el-icon size="32">
                <DataLine />
              </el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">
                {{ formatBytes(periodTrafficStats.total) }}
              </div>
              <div class="stat-label">
                {{ trafficPeriodLabel }}流量
              </div>
            </div>
          </div>
          <div class="stat-footer">
            <span>↑ {{ formatBytes(periodTrafficStats.upload) }} / ↓ {{ formatBytes(periodTrafficStats.download) }}</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="statCardSpan">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon online">
              <el-icon size="32">
                <Monitor />
              </el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">
                {{ dashboardStats.online_count }}
              </div>
              <div class="stat-label">
                在线节点
              </div>
            </div>
          </div>
          <div class="stat-footer">
            <span>当前在线节点数</span>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 协议统计 -->
    <el-row
      :gutter="isMobile ? 12 : 20"
      class="charts-row"
    >
      <el-col :span="chartSpan">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>协议分布</span>
            </div>
          </template>
          <div class="chart-shell">
            <div
              ref="protocolChartRef"
              class="chart-container"
              :class="{ 'chart-container--muted': !hasProtocolChartData }"
            />
            <el-empty
              v-if="!hasProtocolChartData"
              class="chart-empty"
              description="暂无协议分布数据"
              :image-size="54"
            />
          </div>
        </el-card>
      </el-col>
      <el-col :span="chartSpan">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>流量趋势</span>
              <el-radio-group
                v-model="trafficPeriod"
                class="traffic-period-group"
                size="small"
                @change="changeTrafficPeriod"
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
            </div>
          </template>
          <div class="chart-shell">
            <div
              ref="trafficChartRef"
              class="chart-container"
              :class="{ 'chart-container--muted': !hasTrafficTimelineData }"
            />
            <el-empty
              v-if="!hasTrafficTimelineData"
              class="chart-empty"
              description="当前周期暂无流量趋势数据"
              :image-size="54"
            />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 协议详情表格 -->
    <el-card class="protocol-table">
      <template #header>
        <div class="card-header">
          <span>协议统计详情</span>
          <el-button
            type="primary"
            size="small"
            @click="refreshData"
          >
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>
      <div
        v-if="!isMobile"
        class="table-shell"
      >
        <el-table
          v-loading="loading"
          :data="protocolStats"
          style="width: 100%"
        >
          <el-table-column
            prop="protocol"
            label="协议"
            width="150"
          >
            <template #default="{ row }">
              <el-tag :type="getProtocolTagType(row.protocol)">
                {{ row.protocol.toUpperCase() }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column
            prop="count"
            label="代理数量"
            width="120"
          />
          <el-table-column
            prop="traffic"
            label="流量使用"
            width="150"
          >
            <template #default="{ row }">
              {{ formatBytes(row.traffic) }}
            </template>
          </el-table-column>
          <el-table-column
            prop="status"
            label="状态"
            width="100"
          >
            <template #default="{ row }">
              <el-tag
                :type="row.status === 'active' ? 'success' : 'danger'"
                size="small"
              >
                {{ row.status === 'active' ? '正常' : '异常' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="流量占比">
            <template #default="{ row }">
              <el-progress
                :percentage="getTrafficPercentage(row.traffic)"
                :color="getProtocolColor(row.protocol)"
              />
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div
        v-else
        v-loading="loading"
        class="mobile-card-list"
      >
        <el-empty
          v-if="!loading && !protocolStats.length"
          description="暂无协议统计数据"
          :image-size="64"
        />
        <article
          v-for="row in protocolStats"
          :key="row.protocol"
          class="mobile-stat-card"
        >
          <div class="mobile-stat-card__header">
            <el-tag :type="getProtocolTagType(row.protocol)">
              {{ row.protocol.toUpperCase() }}
            </el-tag>
            <el-tag
              :type="row.status === 'active' ? 'success' : 'danger'"
              size="small"
            >
              {{ row.status === 'active' ? '正常' : '异常' }}
            </el-tag>
          </div>
          <div class="mobile-stat-grid">
            <div class="mobile-stat-item">
              <span>代理数量</span>
              <strong>{{ row.count }}</strong>
            </div>
            <div class="mobile-stat-item">
              <span>流量使用</span>
              <strong>{{ formatBytes(row.traffic) }}</strong>
            </div>
          </div>
          <el-progress
            :percentage="getTrafficPercentage(row.traffic)"
            :color="getProtocolColor(row.protocol)"
          />
        </article>
      </div>
    </el-card>

    <!-- 用户统计 -->
    <el-card class="user-stats">
      <template #header>
        <div class="card-header">
          <span>用户流量排行</span>
        </div>
      </template>
      <div
        v-if="!isMobile"
        class="table-shell"
      >
        <el-table
          v-loading="loading"
          :data="userStats"
          style="width: 100%"
        >
          <el-table-column
            prop="username"
            label="用户名"
            width="150"
          />
          <el-table-column
            prop="proxy_count"
            label="代理数"
            width="100"
          />
          <el-table-column
            prop="upload"
            label="上传流量"
            width="120"
          >
            <template #default="{ row }">
              {{ formatBytes(row.upload) }}
            </template>
          </el-table-column>
          <el-table-column
            prop="download"
            label="下载流量"
            width="120"
          >
            <template #default="{ row }">
              {{ formatBytes(row.download) }}
            </template>
          </el-table-column>
          <el-table-column
            prop="total"
            label="总流量"
            width="120"
          >
            <template #default="{ row }">
              {{ formatBytes(row.total) }}
            </template>
          </el-table-column>
          <el-table-column
            prop="last_active"
            label="最后活跃"
          />
        </el-table>
      </div>
      <div
        v-else
        v-loading="loading"
        class="mobile-card-list"
      >
        <article
          v-for="row in userStats"
          :key="row.username"
          class="mobile-stat-card"
        >
          <div class="mobile-stat-card__header">
            <span class="mobile-user-name">{{ row.username }}</span>
            <span class="mobile-total-traffic">{{ formatBytes(row.total) }}</span>
          </div>
          <div class="mobile-stat-grid">
            <div class="mobile-stat-item">
              <span>代理数</span>
              <strong>{{ row.proxy_count }}</strong>
            </div>
            <div class="mobile-stat-item">
              <span>上传流量</span>
              <strong>{{ formatBytes(row.upload) }}</strong>
            </div>
            <div class="mobile-stat-item">
              <span>下载流量</span>
              <strong>{{ formatBytes(row.download) }}</strong>
            </div>
            <div class="mobile-stat-item">
              <span>最后活跃</span>
              <strong>{{ row.last_active || '-' }}</strong>
            </div>
          </div>
        </article>
      </div>
      <div
        v-if="userStats.length === 0"
        class="empty-data"
      >
        暂无用户统计数据
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { User, Connection, DataLine, Monitor, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import echarts from '@/utils/charts'
import { statsApi } from '@/api/index'
import { useViewport } from '@/composables/useViewport'
import { formatTrafficBytes } from '@/utils/traffic'

const loading = ref(false)
const trafficPeriod = ref('today')
const lastSuccessfulTrafficPeriod = ref(trafficPeriod.value)
const { isMobile, isTablet } = useViewport()
const statCardSpan = computed(() => (isMobile.value ? 24 : isTablet.value ? 12 : 6))
const chartSpan = computed(() => (isMobile.value ? 24 : 12))

const protocolChartRef = ref(null)
const trafficChartRef = ref(null)
let protocolChart = null
let trafficChart = null

const trafficPeriodLabel = computed(() => {
  if (trafficPeriod.value === 'week') return '本周'
  if (trafficPeriod.value === 'month') return '本月'
  return '今日'
})

const dashboardStats = ref({
  total_users: 0,
  active_users: 0,
  total_proxies: 0,
  active_proxies: 0,
  total_traffic: 0,
  upload_traffic: 0,
  download_traffic: 0,
  online_count: 0
})

const periodTrafficStats = ref({
  total: 0,
  upload: 0,
  download: 0
})

const protocolStats = ref([])
const userStats = ref([])
const totalTraffic = ref(0)
const trafficTimeline = ref([])
const hasProtocolChartData = computed(() =>
  protocolStats.value.some(item => Number(item.count || 0) > 0 || Number(item.traffic || 0) > 0)
)
const hasTrafficTimelineData = computed(() =>
  trafficTimeline.value.some(item => Number(item.upload || 0) > 0 || Number(item.download || 0) > 0)
)

const formatTimelineLabel = (rawValue) => {
  const date = new Date(rawValue)
  if (Number.isNaN(date.getTime())) {
    return rawValue || ''
  }

  if (trafficPeriod.value === 'today') {
    return `${String(date.getHours()).padStart(2, '0')}:00`
  }

  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${month}-${day}`
}

// 格式化字节
const formatBytes = (bytes) => {
  return formatTrafficBytes(bytes)
}

// 获取协议标签类型
const getProtocolTagType = (protocol) => {
  const types = {
    vmess: 'primary',
    vless: 'success',
    trojan: 'warning',
    shadowsocks: 'danger'
  }
  return types[protocol.toLowerCase()] || 'info'
}

// 获取协议颜色
const getProtocolColor = (protocol) => {
  const colors = {
    vmess: '#409EFF',
    vless: '#67C23A',
    trojan: '#E6A23C',
    shadowsocks: '#F56C6C'
  }
  return colors[protocol.toLowerCase()] || '#909399'
}

// 获取流量百分比
const getTrafficPercentage = (traffic) => {
  if (totalTraffic.value === 0) return 0
  return Math.round((traffic / totalTraffic.value) * 100)
}

const getAxisLabelOptions = () => ({
  fontSize: isMobile.value ? 11 : 12,
  color: '#606266',
  hideOverlap: true
})

const getProtocolChartOption = () => {
  const mobile = isMobile.value
  const data = protocolStats.value.map(item => ({
    name: item.protocol.toUpperCase(),
    value: item.count,
    itemStyle: {
      color: getProtocolColor(item.protocol)
    }
  }))

  return {
    tooltip: {
      trigger: 'item',
      confine: true,
      formatter: '{b}: {c} ({d}%)'
    },
    legend: {
      type: mobile ? 'scroll' : 'plain',
      orient: mobile ? 'horizontal' : 'vertical',
      left: mobile ? 8 : 'left',
      right: mobile ? 8 : undefined,
      top: mobile ? 4 : 'middle',
      itemWidth: mobile ? 16 : 22,
      itemHeight: mobile ? 10 : 14,
      itemGap: mobile ? 10 : 12,
      textStyle: {
        fontSize: mobile ? 11 : 12,
        color: '#303133'
      }
    },
    series: [
      {
        name: '协议分布',
        type: 'pie',
        radius: mobile ? ['32%', '56%'] : ['40%', '70%'],
        center: mobile ? ['50%', '64%'] : ['58%', '50%'],
        avoidLabelOverlap: true,
        itemStyle: {
          borderRadius: 10,
          borderColor: '#fff',
          borderWidth: 2
        },
        label: {
          show: false,
          position: 'center'
        },
        emphasis: {
          label: {
            show: !mobile,
            fontSize: 20,
            fontWeight: 'bold'
          }
        },
        labelLine: {
          show: false
        },
        data
      }
    ]
  }
}

const getTrafficChartOption = (timeline = trafficTimeline.value) => {
  const mobile = isMobile.value
  const times = timeline.map(item => formatTimelineLabel(item.time))
  const uploads = timeline.map(item => item.upload)
  const downloads = timeline.map(item => item.download)

  return {
    tooltip: {
      trigger: 'axis',
      confine: true,
      axisPointer: {
        type: 'cross',
        label: {
          backgroundColor: '#6a7985'
        }
      }
    },
    legend: {
      top: mobile ? 2 : 4,
      left: 'center',
      data: ['上传', '下载'],
      itemWidth: mobile ? 16 : 22,
      itemHeight: mobile ? 10 : 12,
      textStyle: {
        fontSize: mobile ? 11 : 12,
        color: '#606266'
      }
    },
    grid: {
      top: mobile ? 42 : 48,
      left: mobile ? 8 : 14,
      right: mobile ? 8 : 18,
      bottom: mobile ? 8 : 16,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: times,
      axisLabel: getAxisLabelOptions()
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        ...getAxisLabelOptions(),
        formatter: (value) => formatBytes(value)
      }
    },
    series: [
      {
        name: '上传',
        type: 'line',
        stack: 'Total',
        areaStyle: {},
        emphasis: {
          focus: 'series'
        },
        data: uploads
      },
      {
        name: '下载',
        type: 'line',
        stack: 'Total',
        areaStyle: {},
        emphasis: {
          focus: 'series'
        },
        data: downloads
      }
    ]
  }
}

// 初始化协议分布图表
const initProtocolChart = () => {
  if (!protocolChartRef.value) return
  
  protocolChart = echarts.init(protocolChartRef.value)
  protocolChart.setOption(getProtocolChartOption(), true)
}

// 初始化流量趋势图表
const initTrafficChart = () => {
  if (!trafficChartRef.value) return
  
  trafficChart = echarts.init(trafficChartRef.value)
  trafficChart.setOption(getTrafficChartOption(), true)
}

// 更新协议图表
const updateProtocolChart = () => {
  if (!protocolChart) return

  protocolChart.setOption(getProtocolChartOption(), true)
}

// 更新流量图表
const updateTrafficChart = (timeline) => {
  if (!trafficChart || !timeline) return

  trafficChart.setOption(getTrafficChartOption(timeline), true)
}

// 加载仪表盘统计
const loadDashboardStats = async () => {
  try {
    const response = await statsApi.getDashboardStats()
    if (response.code === 200) {
      dashboardStats.value = response.data || dashboardStats.value
      return true
    }
  } catch (error) {
    console.error('加载仪表盘统计失败:', error)
  }
  return false
}

// 加载详细统计
const loadDetailedStats = async () => {
  try {
    const response = await statsApi.getDetailedStats({ period: trafficPeriod.value })
    if (response.code === 200 && response.data) {
      const data = response.data || {}
      protocolStats.value = Array.isArray(data.by_protocol) ? data.by_protocol : []
      userStats.value = Array.isArray(data.by_user) ? data.by_user : []
      periodTrafficStats.value = {
        total: data.total_traffic || 0,
        upload: data.upload || 0,
        download: data.download || 0
      }
      totalTraffic.value = Number(data.total_traffic || 0)
      trafficTimeline.value = Array.isArray(data.timeline) ? data.timeline : []
      updateProtocolChart()
      updateTrafficChart(trafficTimeline.value)
      lastSuccessfulTrafficPeriod.value = trafficPeriod.value
      return true
    }
  } catch (error) {
    console.error('加载详细统计失败:', error)
  }
  return false
}

// 刷新所有数据
const refreshData = async () => {
  loading.value = true
  try {
    const [dashboardOk, detailedOk] = await Promise.all([
      loadDashboardStats(),
      loadDetailedStats()
    ])

    if (!dashboardOk && !detailedOk) {
      ElMessage.error('统计数据加载失败')
    } else if (!dashboardOk || !detailedOk) {
      ElMessage.warning('部分统计数据刷新失败')
    }
  } finally {
    loading.value = false
  }
}

const changeTrafficPeriod = async () => {
  const previousPeriod = lastSuccessfulTrafficPeriod.value
  const previousProtocolStats = [...protocolStats.value]
  const previousUserStats = [...userStats.value]
  const previousPeriodTrafficStats = { ...periodTrafficStats.value }
  const previousTotalTraffic = totalTraffic.value
  const previousTimeline = [...trafficTimeline.value]

  loading.value = true
  try {
    const detailedOk = await loadDetailedStats()
    if (!detailedOk) {
      trafficPeriod.value = previousPeriod
      protocolStats.value = previousProtocolStats
      userStats.value = previousUserStats
      periodTrafficStats.value = previousPeriodTrafficStats
      totalTraffic.value = previousTotalTraffic
      trafficTimeline.value = previousTimeline
      updateProtocolChart()
      updateTrafficChart(trafficTimeline.value)
      ElMessage.warning('统计周期切换失败，已保留原统计数据')
    }
  } finally {
    loading.value = false
  }
}

// 窗口大小变化时重新调整图表大小
const handleResize = () => {
  protocolChart?.resize()
  trafficChart?.resize()
}

watch(isMobile, () => {
  updateProtocolChart()
  updateTrafficChart(trafficTimeline.value)
  requestAnimationFrame(() => {
    protocolChart?.resize()
    trafficChart?.resize()
  })
})

onMounted(() => {
  initProtocolChart()
  initTrafficChart()
  refreshData()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  protocolChart?.dispose()
  trafficChart?.dispose()
})
</script>

<style scoped>
.stats-page {
  padding: 20px;
}

.overview-row {
  margin-bottom: 20px;
}

.stat-card {
  min-height: 140px;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 15px;
  padding: 10px 0;
}

.stat-icon {
  width: 60px;
  height: 60px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 15px;
  color: white;
}

.stat-icon.users {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.stat-icon.proxies {
  background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%);
}

.stat-icon.traffic {
  background: linear-gradient(135deg, #ee0979 0%, #ff6a00 100%);
}

.stat-icon.online {
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  color: var(--color-text-primary);
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 5px;
}

.stat-footer {
  border-top: 1px solid #ebeef5;
  padding-top: 10px;
  font-size: 12px;
  color: #909399;
}

.charts-row {
  margin-bottom: 20px;
}

.chart-container {
  height: 300px;
}

.chart-shell {
  position: relative;
}

.chart-container--muted {
  opacity: 0.28;
}

.chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(255, 255, 255, 0.95));
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.protocol-table {
  margin-bottom: 20px;
}

.user-stats {
  margin-bottom: 20px;
}

.empty-data {
  text-align: center;
  padding: 40px;
  color: #909399;
}

.table-shell {
  overflow-x: auto;
}

.table-shell :deep(.el-table) {
  min-width: 760px;
}

.mobile-card-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.mobile-stat-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  padding: 14px;
  border: 1px solid var(--color-border, #dcdfe6);
  border-radius: 8px;
  background: var(--color-bg-card, #ffffff);
}

.mobile-stat-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
}

.mobile-stat-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.mobile-stat-item {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--color-border-light, #ebeef5);
  border-radius: 8px;
  background: var(--color-bg-soft, #f8fafc);
  color: #909399;
  font-size: 12px;
}

.mobile-stat-item strong {
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--color-text-primary);
  font-size: 14px;
}

.mobile-user-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 700;
  color: var(--color-text-primary);
}

.mobile-total-traffic {
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 700;
  color: var(--color-primary);
}

@media (max-width: 768px) {
  .stats-page {
    padding: 12px;
  }

  .stat-card {
    min-height: 0;
  }

  :deep(.stat-card .el-card__body) {
    padding: 14px;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .stat-content {
    align-items: center;
    flex-direction: row;
    gap: 14px;
    padding: 4px 0;
    min-width: 0;
  }

  .stat-icon {
    margin-right: 0;
    width: 52px;
    height: 52px;
    flex-shrink: 0;
  }

  .stat-value {
    font-size: 24px;
    overflow-wrap: anywhere;
  }

  .stat-footer {
    overflow-wrap: anywhere;
  }

  .traffic-period-group {
    width: 100%;
  }

  .traffic-period-group :deep(.el-radio-button) {
    flex: 1;
  }

  .traffic-period-group :deep(.el-radio-button__inner) {
    width: 100%;
    padding-inline: 0;
  }

  :deep(.charts-row .el-card__header),
  :deep(.protocol-table .el-card__header),
  :deep(.user-stats .el-card__header) {
    padding: 14px 16px;
  }

  :deep(.charts-row .el-card__body) {
    padding: 12px 10px 10px;
  }

  .chart-container {
    height: 280px;
    min-width: 0;
  }

  .mobile-stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>
