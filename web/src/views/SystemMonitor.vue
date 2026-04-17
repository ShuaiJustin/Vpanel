<template>
  <div class="system-monitor">
    <!-- 错误提示条，添加条件控制只在错误时显示 -->
    <el-alert
      v-if="apiError"
      title="获取系统状态失败"
      type="error"
      show-icon
      :closable="false"
      style="margin-bottom: 20px"
    />
    
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>系统监控</span>
          <div class="card-actions">
            <el-button
              :loading="repairingRuntime"
              type="warning"
              plain
              @click="triggerRuntimeReconcile"
            >
              修复脏代理
            </el-button>
            <el-button
              :loading="refreshing"
              type="primary"
              @click="refreshData({ silent: false })"
            >
              刷新数据
            </el-button>
          </div>
        </div>
      </template>
      
      <div
        class="monitor-stats-grid"
        :style="{ gap: `${gridGutter}px` }"
      >
        <div class="monitor-stats-item">
          <el-card class="stats-card">
            <template #header>
              <div class="stats-header">
                CPU 使用率
              </div>
            </template>
            <div class="stats-value">
              <el-progress
                type="dashboard"
                :width="progressWidth"
                :percentage="cpuUsage"
                :color="getColorByPercentage"
              />
              <div class="stats-details">
                <p>核心数: {{ cpuInfo.cores }}</p>
                <p>型号: {{ cpuInfo.model }}</p>
              </div>
            </div>
          </el-card>
        </div>
        <div class="monitor-stats-item">
          <el-card class="stats-card">
            <template #header>
              <div class="stats-header">
                内存使用率
              </div>
            </template>
            <div class="stats-value">
              <el-progress
                type="dashboard"
                :width="progressWidth"
                :percentage="memoryUsage"
                :color="getColorByPercentage"
              />
              <div class="stats-details">
                <p>已用: {{ formatBytes(memoryInfo.used) }}</p>
                <p>总计: {{ formatBytes(memoryInfo.total) }}</p>
              </div>
            </div>
          </el-card>
        </div>
        <div class="monitor-stats-item">
          <el-card class="stats-card">
            <template #header>
              <div class="stats-header">
                磁盘使用率
              </div>
            </template>
            <div class="stats-value">
              <el-progress
                type="dashboard"
                :width="progressWidth"
                :percentage="diskUsage"
                :color="getColorByPercentage"
              />
              <div class="stats-details">
                <p>已用: {{ formatBytes(diskInfo.used) }}</p>
                <p>总计: {{ formatBytes(diskInfo.total) }}</p>
              </div>
            </div>
          </el-card>
        </div>
      </div>
      
      <el-row :gutter="gridGutter">
        <el-col
          :xs="24"
          :lg="12"
        >
          <el-card class="chart-card">
            <template #header>
              <div class="chart-header">
                CPU/内存历史趋势
              </div>
            </template>
            <div
              ref="resourceChartRef"
              class="chart"
            />
          </el-card>
        </el-col>
        <el-col
          :xs="24"
          :lg="12"
        >
          <el-card class="chart-card">
            <template #header>
              <div class="chart-header">
                磁盘 I/O
              </div>
            </template>
            <div
              ref="diskChartRef"
              class="chart"
            />
          </el-card>
        </el-col>
      </el-row>
      
      <el-card class="system-info">
        <template #header>
          <div class="card-header">
            <span>系统信息</span>
          </div>
        </template>
        <el-descriptions
          border
          :column="descriptionColumns"
        >
          <el-descriptions-item label="操作系统">
            {{ systemInfo.os }}
          </el-descriptions-item>
          <el-descriptions-item label="内核版本">
            {{ systemInfo.kernel }}
          </el-descriptions-item>
          <el-descriptions-item label="主机名">
            {{ systemInfo.hostname }}
          </el-descriptions-item>
          <el-descriptions-item label="运行时间">
            {{ systemInfo.uptime }}
          </el-descriptions-item>
          <el-descriptions-item label="负载均衡">
            {{ systemInfo.load ? systemInfo.load.join(' / ') : '0 / 0 / 0' }}
          </el-descriptions-item>
          <el-descriptions-item label="IP 地址">
            {{ systemInfo.ipAddress }}
          </el-descriptions-item>
        </el-descriptions>
      </el-card>
      
      <el-card class="process-table">
        <template #header>
          <div class="card-header">
            <span>进程列表</span>
            <el-input
              v-model="processSearch"
              placeholder="搜索进程"
              :style="{ width: processSearchWidth }"
              clearable
            />
          </div>
        </template>
        <div class="process-table-wrap">
          <el-table
            v-loading="loading"
            :data="filteredProcesses"
            style="width: 100%"
          >
            <el-table-column
              prop="pid"
              label="PID"
              width="80"
            />
            <el-table-column
              prop="name"
              label="名称"
              min-width="150"
            />
            <el-table-column
              prop="user"
              label="用户"
              width="100"
            />
            <el-table-column
              prop="cpu"
              label="CPU %"
              width="90"
            />
            <el-table-column
              prop="memory"
              label="内存 %"
              width="90"
            />
            <el-table-column
              prop="memoryUsed"
              label="内存使用"
              width="130"
            >
              <template #default="{ row }">
                {{ formatBytes(row.memoryUsed) }}
              </template>
            </el-table-column>
            <el-table-column
              v-if="!isMobile"
              prop="started"
              label="开始时间"
              width="150"
            />
            <el-table-column
              prop="state"
              label="状态"
              width="100"
            >
              <template #default="{ row }">
                <el-tag :type="getStatusType(row.state)">
                  {{ row.state }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-card>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import echarts from '@/utils/charts'
import { systemApi } from '@/api'
import { ElMessage } from 'element-plus'
import { useViewport } from '@/composables/useViewport'

const { isMobile, isTablet } = useViewport()
const SYSTEM_MONITOR_REFRESH_INTERVAL = 60000

// 图表引用
const resourceChartRef = ref(null)
const diskChartRef = ref(null)
let resourceChart = null
let diskChart = null

// 数据状态
const loading = ref(false)
const refreshing = ref(false)
const apiError = ref(false)
const repairingRuntime = ref(false)
const processSearch = ref('')
const cpuUsage = ref(0)
const memoryUsage = ref(0)
const diskUsage = ref(0)
const cpuInfo = ref({ cores: 0, model: 'Unknown' })
const memoryInfo = ref({ used: 0, total: 1 })
const diskInfo = ref({ used: 0, total: 1 })
const systemInfo = ref({
  os: 'Unknown',
  kernel: 'Unknown',
  hostname: 'Unknown',
  uptime: '0 days, 0 hours, 0 minutes',
  load: [0, 0, 0],
  ipAddress: '0.0.0.0'
})
const processes = ref([])
const gridGutter = computed(() => (isMobile.value ? 12 : 20))
const progressWidth = computed(() => (isMobile.value ? 132 : isTablet.value ? 150 : 168))
const descriptionColumns = computed(() => (isMobile.value ? 1 : 2))
const processSearchWidth = computed(() => (isMobile.value ? '100%' : isTablet.value ? '180px' : '220px'))

// 计算属性
const filteredProcesses = computed(() => {
  if (!processSearch.value) return processes.value
  const search = processSearch.value.toLowerCase()
  return processes.value.filter(p => 
    p.name.toLowerCase().includes(search) || 
    p.user.toLowerCase().includes(search) ||
    p.pid.toString().includes(search)
  )
})

// 根据百分比获取颜色
const getColorByPercentage = (percentage) => {
  if (percentage < 60) return '#67C23A'
  if (percentage < 80) return '#E6A23C'
  return '#F56C6C'
}

const normalizePercentage = value => {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) {
    return 0
  }

  return Math.max(0, Math.min(100, Math.round(parsed)))
}

// 获取进程状态类型
const getStatusType = (state) => {
  const types = {
    running: 'success',
    sleeping: 'info',
    stopped: 'warning',
    zombie: 'danger',
    idle: 'info'
  }
  return types[state.toLowerCase()] || 'info'
}

// 格式化字节大小
const formatBytes = (bytes) => {
  const normalized = Number(bytes)
  if (!Number.isFinite(normalized) || normalized <= 0) return '0 B'
  if (normalized < 1) return `${normalized.toFixed(2)} B`
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(normalized) / Math.log(k))
  return parseFloat((normalized / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 初始化图表
const initCharts = () => {
  if (!resourceChartRef.value || !diskChartRef.value) {
    return
  }

  // CPU/内存历史趋势图表
  resourceChart = echarts.init(resourceChartRef.value)
  resourceChart.setOption({
    title: {
      text: 'CPU/内存使用率趋势'
    },
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
      data: ['CPU', '内存']
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: generateTimePoints(10)
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 100,
      name: '使用率 (%)'
    },
    series: [
      {
        name: 'CPU',
        type: 'line',
        data: generateRandomData(10, 10, 70),
        areaStyle: {}
      },
      {
        name: '内存',
        type: 'line',
        data: generateRandomData(10, 30, 90),
        areaStyle: {}
      }
    ]
  })
  
  // 磁盘 I/O 图表
  diskChart = echarts.init(diskChartRef.value)
  diskChart.setOption({
    title: {
      text: '磁盘 I/O 活动'
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      }
    },
    legend: {
      data: ['读取', '写入']
    },
    xAxis: {
      type: 'category',
      data: generateTimePoints(7)
    },
    yAxis: {
      type: 'value',
      name: '速率 (MB/s)'
    },
    series: [
      {
        name: '读取',
        type: 'bar',
        data: generateRandomData(7, 0, 50)
      },
      {
        name: '写入',
        type: 'bar',
        data: generateRandomData(7, 5, 70)
      }
    ]
  })
}

// 生成随机数据
const generateRandomData = (count, min, max) => {
  return Array(count).fill(0).map(() => Math.floor(Math.random() * (max - min + 1)) + min)
}

// 生成时间点
const generateTimePoints = (count) => {
  const now = new Date()
  return Array(count).fill(0).map((_, i) => {
    const d = new Date(now - (count - i - 1) * 60 * 1000)
    return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
  })
}



// 更新图表数据
const updateCharts = () => {
  if (!resourceChart || !diskChart) {
    return
  }

  // 更新CPU/内存图表
  resourceChart.setOption({
    xAxis: {
      data: generateTimePoints(10)
    },
    series: [
      {
        data: generateRandomData(10, 10, 70)
      },
      {
        data: generateRandomData(10, 30, 90)
      }
    ]
  })
  
  // 更新磁盘I/O图表
  diskChart.setOption({
    xAxis: {
      data: generateTimePoints(7)
    },
    series: [
      {
        data: generateRandomData(7, 0, 50)
      },
      {
        data: generateRandomData(7, 5, 70)
      }
    ]
  })
}

// 刷新数据
const buildRuntimeReconcileSummary = (stats = {}) => {
  const scannedProxies = Number(stats.scanned_proxies ?? stats.scannedProxies ?? 0)
  const deletedMissingNode = Number(stats.deleted_missing_node ?? stats.deletedMissingNode ?? 0)
  const evaluatedUsers = Number(stats.evaluated_users ?? stats.evaluatedUsers ?? 0)
  const forbiddenUsersDetected = Number(stats.forbidden_users_detected ?? stats.forbiddenUsersDetected ?? 0)

  return `扫描代理 ${scannedProxies} 条，清理缺失节点代理 ${deletedMissingNode} 条，校验用户 ${evaluatedUsers} 个，发现失效用户 ${forbiddenUsersDetected} 个`
}

const triggerRuntimeReconcile = async () => {
  if (repairingRuntime.value) return

  repairingRuntime.value = true
  try {
    const response = await systemApi.triggerRuntimeReconcile()
    const data = response?.code === 200 && response?.data ? response.data : response
    const stats = data?.stats || {}

    ElMessage.success(data?.message ? `${data.message}：${buildRuntimeReconcileSummary(stats)}` : buildRuntimeReconcileSummary(stats))
    await refreshData({ silent: false })
  } catch (error) {
    console.error('触发运行时巡检失败:', error)
    ElMessage.error(error?.message || '触发运行时巡检失败')
  } finally {
    repairingRuntime.value = false
  }
}

const refreshData = async ({ silent = false } = {}) => {
  const shouldShowBlockingLoading = !silent && processes.value.length === 0
  if (shouldShowBlockingLoading) {
    loading.value = true
  } else if (!silent) {
    refreshing.value = true
  }

  apiError.value = false
  
  try {
    const response = await systemApi.getSystemStatus({
      include_processes: true
    })

    const data = response?.code === 200 && response?.data ? response.data : response

    if (!data) {
      throw new Error('API返回数据格式不正确')
    }

    // 更新系统信息
    if (data.systemInfo) {
      systemInfo.value = data.systemInfo

      if (!systemInfo.value.load || systemInfo.value.load === null) {
        systemInfo.value.load = [0, 0, 0]
      }
    }

    // 更新CPU信息
    if (data.cpuInfo) {
      cpuInfo.value = data.cpuInfo
      cpuUsage.value = normalizePercentage(data.cpuUsage)
    }

    // 更新内存信息
    if (data.memoryInfo) {
      memoryInfo.value = data.memoryInfo
      memoryUsage.value = normalizePercentage(data.memoryUsage)
    }

    // 更新磁盘信息
    if (data.diskInfo) {
      diskInfo.value = data.diskInfo
      diskUsage.value = normalizePercentage(data.diskUsage)
    }

    // 更新进程列表
    if (data.processes) {
      processes.value = data.processes
    }

    apiError.value = false
  } catch (error) {
    console.error('获取系统状态失败:', error)
    apiError.value = true
    if (!silent) {
      ElMessage.error('获取系统状态失败')
    }
  } finally {
    loading.value = false
    refreshing.value = false
    // 更新图表
    updateCharts()
  }
}

// 窗口大小变化时重新调整图表大小
const handleResize = () => {
  resourceChart?.resize()
  diskChart?.resize()
}

// 定时器引用
let timer = null

const handleVisibilityChange = () => {
  if (document.visibilityState === 'visible') {
    refreshData({ silent: true })
  }
}

onMounted(() => {
  // 初始化图表
  initCharts()
  
  // 加载初始数据
  refreshData({ silent: false })
  
  // 开始定时更新
  timer = setInterval(() => {
    if (document.visibilityState === 'hidden') {
      return
    }
    refreshData({ silent: true })
  }, SYSTEM_MONITOR_REFRESH_INTERVAL)
  
  // 监听窗口大小变化
  window.addEventListener('resize', handleResize)
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onUnmounted(() => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  window.removeEventListener('resize', handleResize)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  resourceChart?.dispose()
  diskChart?.dispose()
})
</script>

<style scoped>
.system-monitor {
  padding: 20px;
  z-index: 1;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.monitor-stats-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-bottom: 20px;
}

.monitor-stats-item {
  min-width: 0;
}

.stats-card {
  height: 100%;
  width: 100%;
}

:deep(.stats-card .el-card__body) {
  height: 100%;
}

.stats-header {
  font-weight: bold;
  text-align: center;
}

.stats-value {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  text-align: center;
  padding: 12px 0;
}

.stats-details {
  text-align: center;
  max-width: 100%;
}

.stats-details p {
  margin: 5px 0;
  color: #606266;
  word-break: break-word;
}

.chart-card {
  margin-bottom: 20px;
}

.chart-header {
  font-weight: bold;
}

.chart {
  height: 300px;
  width: 100%;
}

.system-info {
  margin-bottom: 20px;
}

.process-table {
  margin-top: 20px;
}

.process-table-wrap {
  overflow-x: auto;
}

.el-card {
  z-index: 1;
}

@media (max-width: 768px) {
  .system-monitor {
    padding: 12px;
  }

  .card-header {
    align-items: stretch;
    flex-direction: column;
  }

  .card-actions {
    width: 100%;
  }

  .card-actions :deep(.el-button) {
    flex: 1 1 100%;
  }

  .monitor-stats-grid {
    grid-template-columns: 1fr;
  }

  .stats-value {
    gap: 12px;
    padding: 8px 0;
  }

  .chart {
    height: 260px;
  }
}

@media (min-width: 769px) and (max-width: 1280px) {
  .monitor-stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

:deep(.monitor-stats-item > .el-card) {
  height: 100%;
}
</style> 
