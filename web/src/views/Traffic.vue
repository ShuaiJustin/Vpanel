<template>
  <div class="traffic-container">
    <div class="header">
      <div class="page-heading">
        <h1>流量统计</h1>
        <p class="page-subtitle">按时间维度查看整体流量趋势和用户流量明细</p>
      </div>
      <div class="actions">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          :shortcuts="dateShortcuts"
          @change="handleDateChange"
        />
        <el-button type="primary" @click="fetchTrafficData">查询</el-button>
        <el-button type="success" @click="exportTrafficData">导出报表</el-button>
      </div>
    </div>

    <!-- 总流量统计 -->
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ formatTraffic(totalStats.totalTraffic) }}</div>
          <div class="stat-title">总流量</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ formatTraffic(totalStats.uploadTraffic) }}</div>
          <div class="stat-title">总上传</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ formatTraffic(totalStats.downloadTraffic) }}</div>
          <div class="stat-title">总下载</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ totalStats.activeUsers }}</div>
          <div class="stat-title">活跃用户</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 流量趋势图 -->
    <el-card class="chart-card">
      <template #header>
        <div class="card-header">
          <span>流量趋势</span>
          <el-radio-group v-model="trafficChartType" size="small">
            <el-radio-button value="day">日</el-radio-button>
            <el-radio-button value="week">周</el-radio-button>
            <el-radio-button value="month">月</el-radio-button>
          </el-radio-group>
        </div>
      </template>
      <div id="traffic-chart" style="width: 100%; height: 300px;"></div>
    </el-card>

    <!-- 用户流量表格 -->
    <el-card class="user-traffic-card">
      <template #header>
        <div class="card-header">
          <span>用户流量明细</span>
          <div class="header-actions">
            <el-input
              v-model="searchQuery"
              placeholder="搜索用户"
              style="width: 200px; margin-right: 10px;"
              clearable
            />
            <el-select v-model="sortBy" placeholder="排序方式" style="width: 150px;">
              <el-option label="流量降序" value="traffic_desc" />
              <el-option label="流量升序" value="traffic_asc" />
              <el-option label="用户名" value="username" />
            </el-select>
          </div>
        </div>
      </template>

      <el-table :data="filteredUserTraffic" v-loading="loading" style="width: 100%">
        <el-table-column prop="username" label="用户名" width="150" />
        <el-table-column prop="email" label="邮箱" width="200" />
        <el-table-column label="总流量" width="150">
          <template #default="scope">
            {{ formatTraffic(scope.row.totalTraffic) }}
          </template>
        </el-table-column>
        <el-table-column label="上传流量" width="150">
          <template #default="scope">
            {{ formatTraffic(scope.row.uploadTraffic) }}
          </template>
        </el-table-column>
        <el-table-column label="下载流量" width="150">
          <template #default="scope">
            {{ formatTraffic(scope.row.downloadTraffic) }}
          </template>
        </el-table-column>
        <el-table-column label="流量使用" width="250">
          <template #default="scope">
            <el-progress 
              :percentage="calculateTrafficPercentage(scope.row)" 
              :status="getTrafficStatus(scope.row)"
            />
            <div>{{ formatTraffic(scope.row.totalTraffic) }} / {{ formatTraffic(scope.row.trafficLimit) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="最后在线" width="180">
          <template #default="scope">
            {{ formatDateTime(scope.row.lastOnline) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="120">
          <template #default="scope">
            <el-button size="small" type="primary" @click="showUserDetail(scope.row)">详情</el-button>
            <el-button size="small" type="success" @click="resetUserTraffic(scope.row)">重置</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 用户流量详情对话框 -->
    <el-dialog title="用户流量详情" v-model="userDetailVisible" width="800px">
      <template v-if="selectedUser">
        <el-descriptions title="用户信息" :column="2" border>
          <el-descriptions-item label="用户名">{{ selectedUser.username }}</el-descriptions-item>
          <el-descriptions-item label="邮箱">{{ selectedUser.email }}</el-descriptions-item>
          <el-descriptions-item label="流量限制">{{ formatTraffic(selectedUser.trafficLimit) }}</el-descriptions-item>
          <el-descriptions-item label="流量使用">{{ formatTraffic(selectedUser.totalTraffic) }}</el-descriptions-item>
          <el-descriptions-item label="上传流量">{{ formatTraffic(selectedUser.uploadTraffic) }}</el-descriptions-item>
          <el-descriptions-item label="下载流量">{{ formatTraffic(selectedUser.downloadTraffic) }}</el-descriptions-item>
          <el-descriptions-item label="注册时间">{{ formatDateTime(selectedUser.createdAt) }}</el-descriptions-item>
          <el-descriptions-item label="最后在线">{{ formatDateTime(selectedUser.lastOnline) }}</el-descriptions-item>
        </el-descriptions>

        <div class="user-traffic-chart" id="user-traffic-chart" style="width: 100%; height: 300px; margin-top: 20px;"></div>

        <el-table :data="selectedUser.trafficLog" style="width: 100%; margin-top: 20px;">
          <el-table-column prop="date" label="日期" width="180" />
          <el-table-column label="总流量" width="150">
            <template #default="scope">
              {{ formatTraffic(scope.row.total) }}
            </template>
          </el-table-column>
          <el-table-column label="上传" width="150">
            <template #default="scope">
              {{ formatTraffic(scope.row.upload) }}
            </template>
          </el-table-column>
          <el-table-column label="下载" width="150">
            <template #default="scope">
              {{ formatTraffic(scope.row.download) }}
            </template>
          </el-table-column>
        </el-table>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { ref, reactive, computed, onMounted, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as echarts from 'echarts/core'
import { LineChart, BarChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
  DataZoomComponent
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import api from '@/api/index'

// 注册必要的 ECharts 组件
echarts.use([
  TitleComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
  DataZoomComponent,
  LineChart,
  BarChart,
  CanvasRenderer
])

export default {
  name: 'Traffic',
  setup() {
    // 状态
    const loading = ref(false)
    const userTraffic = ref([])
    const dateRange = ref([])
    const searchQuery = ref('')
    const sortBy = ref('traffic_desc')
    const trafficChartType = ref('day')
    const userDetailVisible = ref(false)
    const selectedUser = ref(null)
    
    // 总流量统计
    const totalStats = reactive({
      totalTraffic: 0,
      uploadTraffic: 0,
      downloadTraffic: 0,
      activeUsers: 0
    })

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
        text: '最近一个月',
        value: () => {
          const end = new Date()
          const start = new Date()
          start.setTime(start.getTime() - 3600 * 1000 * 24 * 30)
          return [start, end]
        }
      },
      {
        text: '最近三个月',
        value: () => {
          const end = new Date()
          const start = new Date()
          start.setTime(start.getTime() - 3600 * 1000 * 24 * 90)
          return [start, end]
        }
      }
    ]

    // 过滤后的用户流量数据
    const filteredUserTraffic = computed(() => {
      let result = userTraffic.value

      // 搜索过滤
      if (searchQuery.value) {
        const query = searchQuery.value.toLowerCase()
        result = result.filter(user => 
          user.username.toLowerCase().includes(query) ||
          user.email.toLowerCase().includes(query)
        )
      }

      // 排序
      switch (sortBy.value) {
        case 'traffic_desc':
          result = result.slice().sort((a, b) => b.totalTraffic - a.totalTraffic)
          break
        case 'traffic_asc':
          result = result.slice().sort((a, b) => a.totalTraffic - b.totalTraffic)
          break
        case 'username':
          result = result.slice().sort((a, b) => a.username.localeCompare(b.username))
          break
      }

      return result
    })

    // 获取流量数据
    const fetchTrafficData = async () => {
      loading.value = true
      try {
        const params = {
          startTime: dateRange.value && dateRange.value[0] ? dateRange.value[0].toISOString() : undefined,
          endTime: dateRange.value && dateRange.value[1] ? dateRange.value[1].toISOString() : undefined
        }
        
        const response = await api.get('/stats/user', { params })
        const data = response.data || response
        userTraffic.value = data.users || []
        
        // 计算总流量统计
        totalStats.totalTraffic = userTraffic.value.reduce((sum, user) => sum + (user.totalTraffic || 0), 0)
        totalStats.uploadTraffic = userTraffic.value.reduce((sum, user) => sum + (user.uploadTraffic || 0), 0)
        totalStats.downloadTraffic = userTraffic.value.reduce((sum, user) => sum + (user.downloadTraffic || 0), 0)
        totalStats.activeUsers = userTraffic.value.filter(user => {
          const lastOnline = new Date(user.lastOnline)
          const now = new Date()
          return (now - lastOnline) < 24 * 60 * 60 * 1000
        }).length
        
        // 初始化图表
        initTrafficChart()
      } catch (error) {
        ElMessage.error('获取流量数据失败')
        console.error(error)
        userTraffic.value = []
      } finally {
        loading.value = false
      }
    }

    // 导出流量数据
    const exportTrafficData = () => {
      ElMessage.success('流量报表导出成功')
    }

    // 显示用户详情
    const showUserDetail = (user) => {
      selectedUser.value = { ...user }
      userDetailVisible.value = true
      
      // 在弹窗显示后初始化用户流量图表
      nextTick(() => {
        initUserTrafficChart()
      })
    }

    // 重置用户流量
    const resetUserTraffic = (user) => {
      ElMessageBox.confirm(
        `确定要重置 ${user.username} 的流量统计吗？`,
        '警告',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }
      ).then(async () => {
        try {
          // TODO: 替换为实际的 API 调用
          // TODO: 替换为实际 API 调用
          // await trafficService.resetUserTraffic(user.id)
          
          // 重置本地数据
          const index = userTraffic.value.findIndex(u => u.id === user.id)
          if (index !== -1) {
            userTraffic.value[index].totalTraffic = 0
            userTraffic.value[index].uploadTraffic = 0
            userTraffic.value[index].downloadTraffic = 0
          }
          
          ElMessage.success('流量重置成功')
          
          // 重新计算总流量统计
          totalStats.totalTraffic = userTraffic.value.reduce((sum, user) => sum + user.totalTraffic, 0)
          totalStats.uploadTraffic = userTraffic.value.reduce((sum, user) => sum + user.uploadTraffic, 0)
          totalStats.downloadTraffic = userTraffic.value.reduce((sum, user) => sum + user.downloadTraffic, 0)
        } catch (error) {
          ElMessage.error('流量重置失败')
          console.error(error)
        }
      }).catch(() => {})
    }

    // 日期范围变化处理
    const handleDateChange = () => {
      // 这里可以立即触发查询，也可以等待用户点击查询按钮
    }

    // 初始化流量趋势图表
    const initTrafficChart = () => {
      const chartElement = document.getElementById('traffic-chart')
      if (!chartElement) return
      
      const chart = echarts.init(chartElement)
      
      // 准备数据
      const dates = []
      const uploadData = []
      const downloadData = []
      
      // 获取过去30天的日期
      for (let i = 29; i >= 0; i--) {
        const date = new Date()
        date.setDate(date.getDate() - i)
        dates.push(`${date.getMonth() + 1}-${date.getDate()}`)
        
        // 生成每天的流量数据 (TODO: 替换为实际 API 数据)
        const uploadValue = Math.floor(Math.random() * 10 * 1024 * 1024 * 1024) // 0-10GB
        const downloadValue = Math.floor(Math.random() * 20 * 1024 * 1024 * 1024) // 0-20GB
        
        uploadData.push(uploadValue)
        downloadData.push(downloadValue)
      }
      
      // 配置图表选项
      const option = {
        title: {
          text: '流量趋势'
        },
        tooltip: {
          trigger: 'axis',
          formatter: function(params) {
            let result = params[0].axisValue + '<br/>'
            params.forEach(param => {
              result += `${param.seriesName}: ${formatTraffic(param.value)}<br/>`
            })
            return result
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
          data: dates
        },
        yAxis: {
          type: 'value',
          axisLabel: {
            formatter: function(value) {
              return formatTraffic(value)
            }
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
            data: uploadData
          },
          {
            name: '下载',
            type: 'line',
            stack: 'Total',
            areaStyle: {},
            emphasis: {
              focus: 'series'
            },
            data: downloadData
          }
        ]
      }
      
      chart.setOption(option)
      
      // 监听窗口大小变化，调整图表大小
      window.addEventListener('resize', () => {
        chart.resize()
      })
    }

    // 初始化用户流量图表
    const initUserTrafficChart = () => {
      if (!selectedUser.value) return
      
      const chartElement = document.getElementById('user-traffic-chart')
      if (!chartElement) return
      
      const chart = echarts.init(chartElement)
      
      // 准备数据
      const dates = []
      const uploadData = []
      const downloadData = []
      
      // 使用用户的流量日志数据
      selectedUser.value.trafficLog.slice().reverse().forEach(log => {
        dates.push(log.date)
        uploadData.push(log.upload)
        downloadData.push(log.download)
      })
      
      // 配置图表选项
      const option = {
        title: {
          text: '用户流量趋势'
        },
        tooltip: {
          trigger: 'axis',
          formatter: function(params) {
            let result = params[0].axisValue + '<br/>'
            params.forEach(param => {
              result += `${param.seriesName}: ${formatTraffic(param.value)}<br/>`
            })
            return result
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
          data: dates
        },
        yAxis: {
          type: 'value',
          axisLabel: {
            formatter: function(value) {
              return formatTraffic(value)
            }
          }
        },
        series: [
          {
            name: '上传',
            type: 'bar',
            stack: 'total',
            emphasis: {
              focus: 'series'
            },
            data: uploadData
          },
          {
            name: '下载',
            type: 'bar',
            stack: 'total',
            emphasis: {
              focus: 'series'
            },
            data: downloadData
          }
        ]
      }
      
      chart.setOption(option)
    }

    // 计算流量使用百分比
    const calculateTrafficPercentage = (user) => {
      if (!user.trafficLimit) return 0
      return Math.min(100, Math.round((user.totalTraffic / user.trafficLimit) * 100))
    }

    // 获取流量状态
    const getTrafficStatus = (user) => {
      const percentage = calculateTrafficPercentage(user)
      if (percentage >= 90) return 'exception'
      if (percentage >= 70) return 'warning'
      return 'success'
    }

    // 格式化流量显示
    const formatTraffic = (bytes) => {
      if (bytes === 0) return '0 B'
      const k = 1024
      const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
      const i = Math.floor(Math.log(bytes) / Math.log(k))
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
    }

    // 格式化日期时间
    const formatDateTime = (date) => {
      if (!date) return '未知'
      const d = new Date(date)
      return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
    }

    // 监听图表类型变化
    watch(trafficChartType, () => {
      initTrafficChart()
    })

    // 初始化
    onMounted(() => {
      // 设置默认日期范围为最近30天
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 30)
      dateRange.value = [start, end]
      
      // 获取流量数据
      fetchTrafficData()
    })

    return {
      loading,
      userTraffic,
      filteredUserTraffic,
      dateRange,
      dateShortcuts,
      searchQuery,
      sortBy,
      totalStats,
      trafficChartType,
      userDetailVisible,
      selectedUser,
      fetchTrafficData,
      exportTrafficData,
      showUserDetail,
      resetUserTraffic,
      handleDateChange,
      calculateTrafficPercentage,
      getTrafficStatus,
      formatTraffic,
      formatDateTime
    }
  }
}
</script>

<style scoped>
.traffic-container {
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
  align-items: center;
}

.stat-card {
  text-align: center;
  padding: 20px;
  margin-bottom: 20px;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #409EFF;
  margin-bottom: 10px;
}

.stat-title {
  font-size: 16px;
  color: #606266;
}

.chart-card {
  margin-bottom: 20px;
}

.user-traffic-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style> 
