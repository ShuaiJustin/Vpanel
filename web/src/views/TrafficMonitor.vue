<template>
  <div class="traffic-monitor">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">流量监控</h1>
        <p class="page-subtitle">查看实时流量曲线、历史流量统计和连接明细</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="refreshData">刷新数据</el-button>
      </div>
    </div>

    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>流量概览</span>
          <span class="toolbar-summary">共 {{ trafficData.length }} 条记录</span>
        </div>
      </template>
      
      <div class="charts-container">
        <el-card class="chart-card">
          <template #header>
            <div class="chart-header">实时流量</div>
          </template>
          <div class="chart" ref="realtimeChartRef"></div>
        </el-card>
        
        <el-card class="chart-card">
          <template #header>
            <div class="chart-header">历史流量统计</div>
          </template>
          <div class="chart" ref="historyChartRef"></div>
        </el-card>
      </div>
      
      <div class="table-shell">
        <el-table :data="trafficData" style="width: 100%" v-loading="loading">
          <el-table-column prop="timestamp" label="时间" width="180">
            <template #default="{ row }">
              {{ formatDate(row.timestamp) }}
            </template>
          </el-table-column>
          <el-table-column prop="inbound" label="入站流量" width="150">
            <template #default="{ row }">
              {{ formatTraffic(row.inbound) }}
            </template>
          </el-table-column>
          <el-table-column prop="outbound" label="出站流量" width="150">
            <template #default="{ row }">
              {{ formatTraffic(row.outbound) }}
            </template>
          </el-table-column>
          <el-table-column prop="total" label="总流量" width="150">
            <template #default="{ row }">
              {{ formatTraffic(row.total) }}
            </template>
          </el-table-column>
          <el-table-column prop="protocol" label="协议" width="120" />
          <el-table-column prop="client" label="客户端" />
        </el-table>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts'

import { ElMessage } from 'element-plus'
import api from '@/api/index'

// 图表引用
const realtimeChartRef = ref(null)
const historyChartRef = ref(null)
let realtimeChart = null
let historyChart = null

// 数据
const loading = ref(false)
const trafficData = ref([])


// 初始化图表
const initCharts = () => {
  // 实时流量图表
  realtimeChart = echarts.init(realtimeChartRef.value)
  realtimeChart.setOption({
    title: {
      text: '实时流量监控'
    },
    tooltip: {
      trigger: 'axis'
    },
    legend: {
      data: ['入站流量', '出站流量']
    },
    xAxis: {
      type: 'category',
      data: []
    },
    yAxis: {
      type: 'value',
      name: '流量 (MB/s)'
    },
    series: [
      {
        name: '入站流量',
        type: 'line',
        data: []
      },
      {
        name: '出站流量',
        type: 'line',
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
  const data = trafficData.value
  
  // 更新实时流量图表
  realtimeChart.setOption({
    xAxis: {
      data: data.map(item => formatTime(item.timestamp))
    },
    series: [
      {
        data: data.map(item => (item.inbound / 1024 / 1024).toFixed(2))
      },
      {
        data: data.map(item => (item.outbound / 1024 / 1024).toFixed(2))
      }
    ]
  })

  // 更新历史流量图表
  historyChart.setOption({
    xAxis: {
      data: data.map(item => formatDate(item.timestamp, 'MM-DD'))
    },
    series: [
      {
        data: data.map(item => (item.inbound / 1024 / 1024 / 1024).toFixed(2))
      },
      {
        data: data.map(item => (item.outbound / 1024 / 1024 / 1024).toFixed(2))
      },
      {
        data: data.map(item => ((item.inbound + item.outbound) / 1024 / 1024 / 1024).toFixed(2))
      }
    ]
  })
}

// 刷新数据
const refreshData = async () => {
  loading.value = true
  try {
    const response = await api.get('/stats/traffic')
    const data = response.data || response
    trafficData.value = data.list || []
    
    // 更新图表
    updateCharts()
  } catch (error) {
    console.error('获取流量数据失败:', error)
    ElMessage.error('获取流量数据失败')
    trafficData.value = []
  } finally {
    loading.value = false
  }
}

// 格式化流量数据
const formatTraffic = (bytes) => {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(2) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

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

// 格式化时间
const formatTime = (date) => {
  const d = new Date(date)
  const hours = String(d.getHours()).padStart(2, '0')
  const minutes = String(d.getMinutes()).padStart(2, '0')
  return `${hours}:${minutes}`
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
