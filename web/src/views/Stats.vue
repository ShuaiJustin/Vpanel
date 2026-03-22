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
                {{ formatBytes(dashboardStats.total_traffic) }}
              </div>
              <div class="stat-label">
                总流量
              </div>
            </div>
          </div>
          <div class="stat-footer">
            <span>↑ {{ formatBytes(dashboardStats.upload_traffic) }} / ↓ {{ formatBytes(dashboardStats.download_traffic) }}</span>
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
                在线连接
              </div>
            </div>
          </div>
          <div class="stat-footer">
            <span>实时连接数</span>
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
          <div
            ref="protocolChartRef"
            class="chart-container"
          />
        </el-card>
      </el-col>
      <el-col :span="chartSpan">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>流量趋势</span>
              <el-radio-group
                v-model="trafficPeriod"
                size="small"
                @change="loadTrafficStats"
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
          <div
            ref="trafficChartRef"
            class="chart-container"
          />
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
      <div class="table-shell">
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
    </el-card>

    <!-- 用户统计 -->
    <el-card class="user-stats">
      <template #header>
        <div class="card-header">
          <span>用户流量排行</span>
        </div>
      </template>
      <div class="table-shell">
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
        v-if="userStats.length === 0"
        class="empty-data"
      >
        暂无用户统计数据
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { User, Connection, DataLine, Monitor, Refresh } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { statsApi } from '@/api/index'
import { useViewport } from '@/composables/useViewport'

const loading = ref(false)
const trafficPeriod = ref('today')
const { isMobile, isTablet } = useViewport()
const statCardSpan = computed(() => (isMobile.value ? 24 : isTablet.value ? 12 : 6))
const chartSpan = computed(() => (isMobile.value ? 24 : 12))

const protocolChartRef = ref(null)
const trafficChartRef = ref(null)
let protocolChart = null
let trafficChart = null

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

const protocolStats = ref([])
const userStats = ref([])
const totalTraffic = ref(0)

// 格式化字节
const formatBytes = (bytes) => {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
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

// 初始化协议分布图表
const initProtocolChart = () => {
  if (!protocolChartRef.value) return
  
  protocolChart = echarts.init(protocolChartRef.value)
  protocolChart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)'
    },
    legend: {
      orient: 'vertical',
      left: 'left'
    },
    series: [
      {
        name: '协议分布',
        type: 'pie',
        radius: ['40%', '70%'],
        avoidLabelOverlap: false,
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
            show: true,
            fontSize: 20,
            fontWeight: 'bold'
          }
        },
        labelLine: {
          show: false
        },
        data: []
      }
    ]
  })
}

// 初始化流量趋势图表
const initTrafficChart = () => {
  if (!trafficChartRef.value) return
  
  trafficChart = echarts.init(trafficChartRef.value)
  trafficChart.setOption({
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross',
        label: {
          backgroundColor: '#6a7985'
        }
      }
    },
    legend: {
      data: ['上传', '下载']
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: []
    },
    yAxis: {
      type: 'value',
      axisLabel: {
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
        data: []
      },
      {
        name: '下载',
        type: 'line',
        stack: 'Total',
        areaStyle: {},
        emphasis: {
          focus: 'series'
        },
        data: []
      }
    ]
  })
}

// 更新协议图表
const updateProtocolChart = () => {
  if (!protocolChart) return
  
  const data = protocolStats.value.map(item => ({
    name: item.protocol.toUpperCase(),
    value: item.count,
    itemStyle: {
      color: getProtocolColor(item.protocol)
    }
  }))
  
  protocolChart.setOption({
    series: [{
      data: data
    }]
  })
}

// 更新流量图表
const updateTrafficChart = (timeline) => {
  if (!trafficChart || !timeline) return
  
  const times = timeline.map(item => item.time)
  const uploads = timeline.map(item => item.upload)
  const downloads = timeline.map(item => item.download)
  
  trafficChart.setOption({
    xAxis: {
      data: times
    },
    series: [
      { data: uploads },
      { data: downloads }
    ]
  })
}

// 加载仪表盘统计
const loadDashboardStats = async () => {
  try {
    const response = await statsApi.getDashboardStats()
    if (response.code === 200) {
      dashboardStats.value = response.data || dashboardStats.value
    }
  } catch (error) {
    console.error('加载仪表盘统计失败:', error)
  }
}

// 加载协议统计
const loadProtocolStats = async () => {
  try {
    const response = await statsApi.getProtocolStats()
    if (response.code === 200) {
      protocolStats.value = response.data || []
      totalTraffic.value = protocolStats.value.reduce((sum, item) => sum + item.traffic, 0)
      updateProtocolChart()
    }
  } catch (error) {
    console.error('加载协议统计失败:', error)
  }
}

// 加载流量统计
const loadTrafficStats = async () => {
  try {
    const response = await statsApi.getDetailedStats({ period: trafficPeriod.value })
    if (response.code === 200 && response.data) {
      updateTrafficChart(response.data.timeline || [])
    }
  } catch (error) {
    console.error('加载流量统计失败:', error)
  }
}

// 加载用户统计
const loadUserStats = async () => {
  try {
    const response = await statsApi.getUserStats()
    if (response.code === 200) {
      userStats.value = response.data || []
    }
  } catch (error) {
    console.error('加载用户统计失败:', error)
  }
}

// 刷新所有数据
const refreshData = async () => {
  loading.value = true
  try {
    await Promise.all([
      loadDashboardStats(),
      loadProtocolStats(),
      loadTrafficStats(),
      loadUserStats()
    ])
  } finally {
    loading.value = false
  }
}

// 窗口大小变化时重新调整图表大小
const handleResize = () => {
  protocolChart?.resize()
  trafficChart?.resize()
}

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
  height: 140px;
}

.stat-content {
  display: flex;
  align-items: center;
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
  color: #303133;
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

@media (max-width: 768px) {
  .stats-page {
    padding: 12px;
  }

  .stat-card {
    height: auto;
  }

  .stat-content,
  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .stat-icon {
    margin-right: 0;
  }

  .chart-container {
    height: 260px;
  }
}
</style>
