<template>
  <div class="traffic-monitor">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          流量监控
        </h1>
        <p class="page-subtitle">
          查看最近 5 分钟节点流量和历史流量趋势
        </p>
      </div>
      <div class="page-actions">
        <el-select
          v-model="historyPeriod"
          size="small"
          style="width: 120px"
          @change="refreshData"
        >
          <el-option
            label="今日"
            value="today"
          />
          <el-option
            label="本周"
            value="week"
          />
          <el-option
            label="本月"
            value="month"
          />
        </el-select>
        <el-button
          type="primary"
          @click="refreshData"
        >
          刷新数据
        </el-button>
      </div>
    </div>

    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>流量概览</span>
          <span class="toolbar-summary">历史时间点 {{ trafficData.length }} 条</span>
        </div>
      </template>
      
      <div class="charts-container">
        <el-card class="chart-card">
          <template #header>
            <div class="chart-header">
              最近 {{ realtimeWindowLabel }}
            </div>
          </template>
          <div
            ref="realtimeChartRef"
            class="chart"
          />
        </el-card>
        
        <el-card class="chart-card">
          <template #header>
            <div class="chart-header">
              历史流量统计
            </div>
          </template>
          <div
            ref="historyChartRef"
            class="chart"
          />
        </el-card>
      </div>
      
      <div class="table-shell">
        <el-table
          v-loading="loading"
          :data="trafficData"
          style="width: 100%"
        >
          <el-table-column
            prop="timestamp"
            label="时间"
            width="180"
          >
            <template #default="{ row }">
              {{ formatDate(row.timestamp) }}
            </template>
          </el-table-column>
          <el-table-column
            prop="inbound"
            label="入站流量"
            width="150"
          >
            <template #default="{ row }">
              {{ formatTraffic(row.inbound) }}
            </template>
          </el-table-column>
          <el-table-column
            prop="outbound"
            label="出站流量"
            width="150"
          >
            <template #default="{ row }">
              {{ formatTraffic(row.outbound) }}
            </template>
          </el-table-column>
          <el-table-column
            prop="total"
            label="总流量"
            width="150"
          >
            <template #default="{ row }">
              {{ formatTraffic(row.total) }}
            </template>
          </el-table-column>
          <el-table-column
            label="上行占比"
            width="120"
          >
            <template #default="{ row }">
              {{ formatPercentage(row.upPercentage) }}
            </template>
          </el-table-column>
          <el-table-column
            label="下行占比"
            width="120"
          >
            <template #default="{ row }">
              {{ formatPercentage(row.downPercentage) }}
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts'

import { ElMessage } from 'element-plus'
import { nodesApi, statsApi } from '@/api'

// 图表引用
const realtimeChartRef = ref(null)
const historyChartRef = ref(null)
let realtimeChart = null
let historyChart = null

// 数据
const loading = ref(false)
const trafficData = ref([])
const realtimeNodeTraffic = ref([])
const realtimeWindow = ref('5m')
const realtimeTimestamp = ref('')
const historyPeriod = ref('week')

const realtimeWindowLabel = computed(() => {
  if (!realtimeTimestamp.value) {
    return '5 分钟'
  }
  return realtimeWindow.value || '5m'
})

const unwrapApiData = (response) => {
  if (response && response.code === 200 && response.data !== undefined) {
    return response.data
  }
  return response
}

const mapRealtimeRows = (response) => {
  const rows = Array.isArray(response?.traffic_by_node) ? response.traffic_by_node : []
  return rows.map((item) => ({
    nodeId: item.node_id,
    label: `节点 ${item.node_id}`,
    inbound: item.upload || 0,
    outbound: item.download || 0,
    total: item.total || 0
  }))
}

const mapHistoryRows = (response) => {
  const timeline = Array.isArray(response?.timeline) ? response.timeline : []
  return [...timeline]
    .map((item) => {
      const inbound = item.upload || 0
      const outbound = item.download || 0
      const total = inbound + outbound
      return {
        timestamp: item.time,
        inbound,
        outbound,
        total,
        upPercentage: total > 0 ? inbound / total : 0,
        downPercentage: total > 0 ? outbound / total : 0
      }
    })
    .sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp))
}


// 初始化图表
const initCharts = () => {
  // 实时流量图表
  realtimeChart = echarts.init(realtimeChartRef.value)
  realtimeChart.setOption({
    title: {
      text: '最近 5 分钟节点流量'
    },
    tooltip: {
      trigger: 'axis'
    },
    legend: {
      data: ['上行流量', '下行流量']
    },
    xAxis: {
      type: 'category',
      data: []
    },
    yAxis: {
      type: 'value',
      name: '流量 (MB)'
    },
    series: [
      {
        name: '上行流量',
        type: 'bar',
        data: []
      },
      {
        name: '下行流量',
        type: 'bar',
        data: []
      }
    ]
  })

  // 历史流量图表
  historyChart = echarts.init(historyChartRef.value)
  historyChart.setOption({
    title: {
      text: '流量历史统计'
    },
    tooltip: {
      trigger: 'axis'
    },
    legend: {
      data: ['入站流量', '出站流量', '总流量']
    },
    xAxis: {
      type: 'category',
      data: []
    },
    yAxis: {
      type: 'value',
      name: '流量 (GB)'
    },
    series: [
      {
        name: '入站流量',
        type: 'bar',
        data: []
      },
      {
        name: '出站流量',
        type: 'bar',
        data: []
      },
      {
        name: '总流量',
        type: 'line',
        data: []
      }
    ]
  })
}

// 更新图表数据
const updateCharts = () => {
  const realtimeData = realtimeNodeTraffic.value
  const historyData = [...trafficData.value].sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp))
  
  // 更新实时流量图表
  realtimeChart.setOption({
    title: {
      text: `最近 ${realtimeWindowLabel.value} 节点流量`
    },
    xAxis: {
      data: realtimeData.map(item => item.label)
    },
    series: [
      {
        data: realtimeData.map(item => Number((item.inbound / 1024 / 1024).toFixed(2)))
      },
      {
        data: realtimeData.map(item => Number((item.outbound / 1024 / 1024).toFixed(2)))
      }
    ]
  })

  // 更新历史流量图表
  historyChart.setOption({
    xAxis: {
      data: historyData.map(item => formatDate(item.timestamp, historyPeriod.value === 'today' ? 'HH:mm' : 'MM-DD'))
    },
    series: [
      {
        data: historyData.map(item => Number((item.inbound / 1024 / 1024 / 1024).toFixed(2)))
      },
      {
        data: historyData.map(item => Number((item.outbound / 1024 / 1024 / 1024).toFixed(2)))
      },
      {
        data: historyData.map(item => Number((item.total / 1024 / 1024 / 1024).toFixed(2)))
      }
    ]
  })
}

// 刷新数据
const refreshData = async () => {
  loading.value = true
  try {
    const [realtimeResult, historyResult] = await Promise.allSettled([
      nodesApi.getRealTimeStats(),
      statsApi.getDetailedStats({ period: historyPeriod.value })
    ])

    if (realtimeResult.status === 'fulfilled') {
      const realtimeData = unwrapApiData(realtimeResult.value)
      realtimeWindow.value = realtimeData?.window || '5m'
      realtimeTimestamp.value = realtimeData?.timestamp || ''
      realtimeNodeTraffic.value = mapRealtimeRows(realtimeData)
    } else {
      console.error('获取实时流量数据失败:', realtimeResult.reason)
    }

    if (historyResult.status === 'fulfilled') {
      const historyData = unwrapApiData(historyResult.value)
      trafficData.value = mapHistoryRows(historyData)
    } else {
      console.error('获取历史流量数据失败:', historyResult.reason)
    }

    if (realtimeResult.status === 'rejected' && historyResult.status === 'rejected') {
      ElMessage.error('获取流量数据失败')
    }

    updateCharts()
  } catch (error) {
    console.error('获取流量数据失败:', error)
    ElMessage.error('获取流量数据失败')
  } finally {
    loading.value = false
  }
}

// 格式化流量数据
const formatTraffic = (bytes) => {
  if (!bytes) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(2) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

const formatPercentage = (value) => `${Math.round((value || 0) * 100)}%`

// 格式化日期
const formatDate = (date, format = 'YYYY-MM-DD HH:mm:ss') => {
  const d = new Date(date)
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hours = String(d.getHours()).padStart(2, '0')
  const minutes = String(d.getMinutes()).padStart(2, '0')
  const seconds = String(d.getSeconds()).padStart(2, '0')
  
  return format
    .replace('YYYY', year)
    .replace('MM', month)
    .replace('DD', day)
    .replace('HH', hours)
    .replace('mm', minutes)
    .replace('ss', seconds)
}

// 窗口大小变化时重新调整图表大小
const handleResize = () => {
  realtimeChart?.resize()
  historyChart?.resize()
}

onMounted(() => {
  // 初始化图表
  initCharts()
  
  // 加载初始数据
  refreshData()
  
  // 监听窗口大小变化
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  // 移除事件监听
  window.removeEventListener('resize', handleResize)
  
  // 销毁图表实例
  realtimeChart?.dispose()
  historyChart?.dispose()
})
</script>

<style scoped>
.traffic-monitor {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.charts-container {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}

.chart-card {
  flex: 1;
  min-width: 300px;
}

.chart-header {
  font-weight: bold;
}

.chart {
  height: 300px;
  margin-top: 10px;
}

.table-shell {
  overflow-x: auto;
}

.table-shell :deep(.el-table) {
  min-width: 880px;
}

@media (max-width: 768px) {
  .traffic-monitor {
    padding: 12px;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .chart-card {
    min-width: 0;
    width: 100%;
  }

  .chart {
    height: 260px;
  }
}
</style> 
