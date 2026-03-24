<template>
  <div class="monitor-container">
    <div class="header">
      <div class="page-heading">
        <h1>系统监控</h1>
        <p class="page-subtitle">
          实时查看 CPU、内存、网络和磁盘运行状态
        </p>
      </div>
      <div class="actions">
        <el-button
          type="primary"
          @click="refreshStats"
        >
          刷新
        </el-button>
        <el-button
          v-if="!autoRefresh"
          type="success"
          @click="startAutoRefresh"
        >
          自动刷新
        </el-button>
        <el-button
          v-else
          type="danger"
          @click="stopAutoRefresh"
        >
          停止刷新
        </el-button>
      </div>
    </div>

    <el-row :gutter="isMobile ? 12 : 20">
      <el-col :span="panelSpan">
        <el-card class="box-card">
          <template #header>
            <div class="card-header">
              <span>CPU 使用率</span>
              <el-tag :type="getCpuStatusType">
                {{ getCpuStatus }}
              </el-tag>
            </div>
          </template>
          <div class="chart-container">
            <div class="gauge-chart">
              <el-progress
                type="dashboard"
                :percentage="Math.round(stats.cpu)"
                :color="cpuColors"
              />
              <div class="chart-value">
                {{ stats.cpu.toFixed(2) }}%
              </div>
            </div>
            <div class="chart-info">
              <p><strong>核心数:</strong> {{ systemInfo.cpuCores }}</p>
              <p><strong>负载平均值:</strong> {{ systemInfo.loadAvg.join(' / ') }}</p>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="panelSpan">
        <el-card class="box-card">
          <template #header>
            <div class="card-header">
              <span>内存使用率</span>
              <el-tag :type="getMemoryStatusType">
                {{ getMemoryStatus }}
              </el-tag>
            </div>
          </template>
          <div class="chart-container">
            <div class="gauge-chart">
              <el-progress
                type="dashboard"
                :percentage="Math.round(stats.memory)"
                :color="memoryColors"
              />
              <div class="chart-value">
                {{ stats.memory.toFixed(2) }}%
              </div>
            </div>
            <div class="chart-info">
              <p><strong>总内存:</strong> {{ formatBytes(systemInfo.memoryTotal) }}</p>
              <p><strong>已用内存:</strong> {{ formatBytes(systemInfo.memoryUsed) }}</p>
              <p><strong>可用内存:</strong> {{ formatBytes(systemInfo.memoryFree) }}</p>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row
      :gutter="isMobile ? 12 : 20"
      style="margin-top: 20px;"
    >
      <el-col :span="panelSpan">
        <el-card class="box-card">
          <template #header>
            <div class="card-header">
              <span>网络流量</span>
            </div>
          </template>
          <div class="chart-container">
            <div class="network-stats">
              <div class="network-item">
                <div class="network-title">
                  <i class="el-icon-upload" /> 上传
                </div>
                <div class="network-value">
                  {{ formatBytes(stats.network.upload) }}/s
                </div>
              </div>
              <div class="network-item">
                <div class="network-title">
                  <i class="el-icon-download" /> 下载
                </div>
                <div class="network-value">
                  {{ formatBytes(stats.network.download) }}/s
                </div>
              </div>
            </div>
            <div class="chart-info">
              <p><strong>总上传:</strong> {{ formatBytes(networkTotal.upload) }}</p>
              <p><strong>总下载:</strong> {{ formatBytes(networkTotal.download) }}</p>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="panelSpan">
        <el-card class="box-card">
          <template #header>
            <div class="card-header">
              <span>磁盘使用率</span>
              <el-tag :type="getDiskStatusType">
                {{ getDiskStatus }}
              </el-tag>
            </div>
          </template>
          <div class="chart-container">
            <div class="gauge-chart">
              <el-progress
                type="dashboard"
                :percentage="diskUsagePercentage"
                :color="diskColors"
              />
              <div class="chart-value">
                {{ diskUsagePercentage.toFixed(2) }}%
              </div>
            </div>
            <div class="chart-info">
              <p><strong>总容量:</strong> {{ formatBytes(stats.disk.total) }}</p>
              <p><strong>已用空间:</strong> {{ formatBytes(stats.disk.used) }}</p>
              <p><strong>可用空间:</strong> {{ formatBytes(stats.disk.total - stats.disk.used) }}</p>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row
      :gutter="isMobile ? 12 : 20"
      style="margin-top: 20px;"
    >
      <el-col :span="24">
        <el-card class="box-card">
          <template #header>
            <div class="card-header">
              <span>系统信息</span>
            </div>
          </template>
          <div class="system-info">
            <el-descriptions
              :column="infoColumns"
              border
            >
              <el-descriptions-item label="主机名">
                {{ systemInfo.hostname }}
              </el-descriptions-item>
              <el-descriptions-item label="操作系统">
                {{ systemInfo.os }}
              </el-descriptions-item>
              <el-descriptions-item label="系统版本">
                {{ systemInfo.platform }} {{ systemInfo.release }}
              </el-descriptions-item>
              <el-descriptions-item label="架构">
                {{ systemInfo.arch }}
              </el-descriptions-item>
              <el-descriptions-item label="运行时间">
                {{ formatUptime(stats.uptime) }}
              </el-descriptions-item>
              <el-descriptions-item label="系统时间">
                {{ currentTime }}
              </el-descriptions-item>
              <el-descriptions-item label="Goroutines">
                {{ systemInfo.goroutines }}
              </el-descriptions-item>
              <el-descriptions-item label="进程内存">
                {{ formatBytes(systemInfo.processMemory) }}
              </el-descriptions-item>
              <el-descriptions-item label="IP地址">
                {{ systemInfo.ip }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script>
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import api from '@/api/index'
import { useViewport } from '@/composables/useViewport'

export default {
  name: 'Monitor',
  setup() {
    const { isMobile, isTablet } = useViewport()

    // 状态
    const stats = reactive({
      cpu: 0,
      memory: 0,
      network: {
        upload: 0,
        download: 0
      },
      disk: {
        used: 0,
        total: 1
      },
      uptime: 0
    })

    const systemInfo = reactive({
      hostname: '',
      os: '',
      platform: '',
      release: '',
      arch: '',
      cpuCores: 0,
      loadAvg: [0, 0, 0],
      memoryTotal: 0,
      memoryUsed: 0,
      memoryFree: 0,
      goroutines: 0,
      processMemory: 0,
      ip: ''
    })

    const networkTotal = reactive({
      upload: 0,
      download: 0
    })

    const autoRefresh = ref(false)
    const refreshTimer = ref(null)
    const currentTime = ref(new Date().toLocaleString())
    const panelSpan = computed(() => (isMobile.value ? 24 : 12))
    const infoColumns = computed(() => (isMobile.value ? 1 : isTablet.value ? 2 : 3))

    // 计算属性
    const diskUsagePercentage = computed(() => {
      if (stats.disk.total === 0) return 0
      return (stats.disk.used / stats.disk.total) * 100
    })

    const getCpuStatus = computed(() => {
      if (stats.cpu >= 90) return '严重'
      if (stats.cpu >= 70) return '警告'
      return '正常'
    })

    const getCpuStatusType = computed(() => {
      if (stats.cpu >= 90) return 'danger'
      if (stats.cpu >= 70) return 'warning'
      return 'success'
    })

    const getMemoryStatus = computed(() => {
      if (stats.memory >= 90) return '严重'
      if (stats.memory >= 70) return '警告'
      return '正常'
    })

    const getMemoryStatusType = computed(() => {
      if (stats.memory >= 90) return 'danger'
      if (stats.memory >= 70) return 'warning'
      return 'success'
    })

    const getDiskStatus = computed(() => {
      if (diskUsagePercentage.value >= 90) return '严重'
      if (diskUsagePercentage.value >= 70) return '警告'
      return '正常'
    })

    const getDiskStatusType = computed(() => {
      if (diskUsagePercentage.value >= 90) return 'danger'
      if (diskUsagePercentage.value >= 70) return 'warning'
      return 'success'
    })

    // 颜色配置
    const cpuColors = [
      { color: '#67C23A', percentage: 70 },
      { color: '#E6A23C', percentage: 90 },
      { color: '#F56C6C', percentage: 100 }
    ]

    const memoryColors = [
      { color: '#67C23A', percentage: 70 },
      { color: '#E6A23C', percentage: 90 },
      { color: '#F56C6C', percentage: 100 }
    ]

    const diskColors = [
      { color: '#67C23A', percentage: 70 },
      { color: '#E6A23C', percentage: 90 },
      { color: '#F56C6C', percentage: 100 }
    ]

    // 方法
    const fetchStats = async () => {
      try {
        const response = await api.get('/system/stats')
        const data = response.data || response
        
        // 更新统计数据
        if (data.cpu !== undefined) stats.cpu = data.cpu
        if (data.memory !== undefined) stats.memory = data.memory
        if (data.network) {
          stats.network.upload = data.network.upload || 0
          stats.network.download = data.network.download || 0
        }
        if (data.disk) {
          stats.disk.used = data.disk.used || 0
          stats.disk.total = data.disk.total || 1
        }
        if (data.uptime !== undefined) stats.uptime = data.uptime
        
        // 更新系统信息
        if (data.systemInfo) {
          Object.assign(systemInfo, data.systemInfo)
        }
        
        // 更新总网络流量
        networkTotal.upload += stats.network.upload
        networkTotal.download += stats.network.download
        
        // 更新当前时间
        currentTime.value = new Date().toLocaleString()
      } catch (error) {
        console.error('获取系统状态失败:', error)
      }
    }

    const refreshStats = async () => {
      await fetchStats()
    }

    const startAutoRefresh = () => {
      autoRefresh.value = true
      refreshStats()
      refreshTimer.value = setInterval(refreshStats, 3000)
    }

    const stopAutoRefresh = () => {
      autoRefresh.value = false
      if (refreshTimer.value) {
        clearInterval(refreshTimer.value)
        refreshTimer.value = null
      }
    }

    // 工具函数
    const formatBytes = (bytes) => {
      if (bytes === 0) return '0 B'
      const k = 1024
      const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
      const i = Math.floor(Math.log(bytes) / Math.log(k))
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
    }

    const formatUptime = (seconds) => {
      const days = Math.floor(seconds / (3600 * 24))
      const hours = Math.floor((seconds % (3600 * 24)) / 3600)
      const minutes = Math.floor((seconds % 3600) / 60)
      const secs = Math.floor(seconds % 60)

      let result = ''
      if (days > 0) {
        result += `${days}天 `
      }
      if (hours > 0 || days > 0) {
        result += `${hours}小时 `
      }
      if (minutes > 0 || hours > 0 || days > 0) {
        result += `${minutes}分钟 `
      }
      result += `${secs}秒`
      
      return result
    }

    // 生命周期
    onMounted(() => {
      refreshStats()
      // 启动自动刷新
      startAutoRefresh()
    })

    onBeforeUnmount(() => {
      // 清理定时器
      stopAutoRefresh()
    })

    return {
      stats,
      systemInfo,
      networkTotal,
      autoRefresh,
      isMobile,
      currentTime,
      panelSpan,
      infoColumns,
      diskUsagePercentage,
      getCpuStatus,
      getCpuStatusType,
      getMemoryStatus,
      getMemoryStatusType,
      getDiskStatus,
      getDiskStatusType,
      cpuColors,
      memoryColors,
      diskColors,
      refreshStats,
      startAutoRefresh,
      stopAutoRefresh,
      formatBytes,
      formatUptime
    }
  }
}
</script>

<style scoped>
.monitor-container {
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.actions {
  display: flex;
  gap: 10px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chart-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px;
}

.gauge-chart {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 50%;
}

.chart-value {
  position: absolute;
  bottom: 30px;
  font-size: 18px;
  font-weight: bold;
}

.chart-info {
  width: 50%;
  padding-left: 20px;
}

.chart-info p {
  margin: 8px 0;
  font-size: 14px;
}

.network-stats {
  display: flex;
  justify-content: space-around;
  width: 100%;
  margin-bottom: 20px;
}

.network-item {
  text-align: center;
  padding: 10px;
  border-radius: 5px;
  background-color: var(--color-bg-page);
  width: 45%;
}

.network-title {
  font-size: 16px;
  margin-bottom: 10px;
  color: #606266;
}

.network-value {
  font-size: 24px;
  font-weight: bold;
  color: #409EFF;
}

.system-info {
  padding: 10px;
}

@media (max-width: 768px) {
  .monitor-container {
    padding: 12px;
  }

  .header,
  .actions,
  .chart-container,
  .network-stats {
    flex-direction: column;
    align-items: stretch;
  }

  .actions {
    width: 100%;
  }

  .actions > * {
    width: 100%;
  }

  .gauge-chart,
  .chart-info,
  .network-item {
    width: 100%;
  }

  .chart-info {
    padding-left: 0;
  }
}
</style> 
