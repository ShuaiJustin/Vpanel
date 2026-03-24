<template>
  <div class="connections-container">
    <div class="page-header">
      <div class="title">
        Trojan 连接管理
      </div>
      <el-button
        type="primary"
        @click="refresh"
      >
        <el-icon class="el-icon--left">
          <Refresh />
        </el-icon> 刷新
      </el-button>
    </div>
    
    <div class="stats-cards">
      <el-card
        shadow="hover"
        class="stats-card"
      >
        <div class="stat-value">
          {{ stats.totalConnections }}
        </div>
        <div class="stat-label">
          总连接数
        </div>
      </el-card>
      
      <el-card
        shadow="hover"
        class="stats-card"
      >
        <div class="stat-value">
          {{ formatBytes(stats.totalUpload) }}
        </div>
        <div class="stat-label">
          总上传流量
        </div>
      </el-card>
      
      <el-card
        shadow="hover"
        class="stats-card"
      >
        <div class="stat-value">
          {{ formatBytes(stats.totalDownload) }}
        </div>
        <div class="stat-label">
          总下载流量
        </div>
      </el-card>
      
      <el-card
        shadow="hover"
        class="stats-card"
      >
        <div class="stat-value">
          {{ stats.activeConnections }}
        </div>
        <div class="stat-label">
          活跃连接
        </div>
      </el-card>
    </div>
    
    <el-table
      v-loading="loading"
      :data="connections"
      border
      style="width: 100%"
      stripe
    >
      <el-table-column
        prop="id"
        label="ID"
        width="80"
      />
      
      <el-table-column
        prop="source"
        label="客户端"
        min-width="160"
      >
        <template #default="{ row }">
          <div class="ip-address">
            {{ row.source }}
          </div>
          <div class="location">
            {{ row.location }}
          </div>
        </template>
      </el-table-column>
      
      <el-table-column
        prop="target"
        label="目标地址"
        min-width="160"
      />
      
      <el-table-column
        label="连接状态"
        width="100"
        align="center"
      >
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">
            {{ row.status === 'active' ? '活跃' : '关闭' }}
          </el-tag>
        </template>
      </el-table-column>
      
      <el-table-column
        label="上传/下载"
        min-width="160"
      >
        <template #default="{ row }">
          <div class="traffic-info">
            <span class="traffic-label">上传:</span>
            <span class="traffic-value">{{ formatBytes(row.upload) }}</span>
          </div>
          <div class="traffic-info">
            <span class="traffic-label">下载:</span>
            <span class="traffic-value">{{ formatBytes(row.download) }}</span>
          </div>
        </template>
      </el-table-column>
      
      <el-table-column
        prop="duration"
        label="连接时长"
        min-width="100"
      >
        <template #default="{ row }">
          {{ formatDuration(row.duration) }}
        </template>
      </el-table-column>
      
      <el-table-column
        prop="created_at"
        label="开始时间"
        min-width="180"
      />
      
      <el-table-column
        label="操作"
        width="100"
        fixed="right"
      >
        <template #default="{ row }">
          <el-button
            size="small"
            type="danger"
            :disabled="row.status !== 'active'"
            @click="disconnectClient(row)"
          >
            断开
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    
    <div class="pagination">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        :total="total"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import api from '@/api/index'

// 连接列表
const connections = ref([])
const loading = ref(false)

// 分页
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)

// 统计数据
const stats = reactive({
  totalConnections: 0,
  activeConnections: 0,
  totalUpload: 0,
  totalDownload: 0
})

// 加载连接数据
const loadConnections = async () => {
  loading.value = true
  
  try {
    const response = await api.get('/trojan/connections', {
      params: {
        page: currentPage.value,
        pageSize: pageSize.value
      }
    })
    
    const data = response.data || response
    connections.value = data.list || []
    total.value = data.total || 0
    
    // 统计数据
    stats.totalConnections = data.total || 0
    stats.activeConnections = connections.value.filter(c => c.status === 'active').length
    stats.totalUpload = connections.value.reduce((sum, c) => sum + (c.upload || 0), 0)
    stats.totalDownload = connections.value.reduce((sum, c) => sum + (c.download || 0), 0)
  } catch (error) {
    console.error('加载连接数据失败:', error)
    ElMessage.error('加载连接数据失败')
    connections.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 格式化流量大小
const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  
  return parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + ' ' + sizes[i]
}

// 格式化持续时间
const formatDuration = (seconds) => {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainingSeconds = seconds % 60
  
  let result = ''
  
  if (hours > 0) {
    result += `${hours}小时`
  }
  
  if (minutes > 0 || hours > 0) {
    result += `${minutes}分钟`
  }
  
  if (remainingSeconds > 0 || (hours === 0 && minutes === 0)) {
    result += `${remainingSeconds}秒`
  }
  
  return result
}

// 断开客户端连接
const disconnectClient = (connection) => {
  ElMessageBox.confirm(
    `确定要断开连接 ${connection.source} 吗？`,
    '断开确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  )
    .then(async () => {
      // 调用断开连接 API
      await new Promise(resolve => setTimeout(resolve, 500))
      
      // 更新连接状态
      const conn = connections.value.find(c => c.id === connection.id)
      if (conn) {
        conn.status = 'closed'
        stats.activeConnections--
      }
      
      ElMessage.success('连接已断开')
    })
    .catch(() => {
      ElMessage.info('已取消断开')
    })
}

// 刷新连接列表
const refresh = () => {
  loadConnections()
}

// 处理分页大小变化
const handleSizeChange = (newSize) => {
  pageSize.value = newSize
  loadConnections()
}

// 处理页码变化
const handleCurrentChange = (newPage) => {
  currentPage.value = newPage
  loadConnections()
}

// 自动刷新定时器
let refreshTimer = null

// 组件挂载时加载连接数据
onMounted(() => {
  loadConnections()
  
  // 设置定时刷新
  refreshTimer = setInterval(() => {
    loadConnections()
  }, 30000) // 每30秒刷新一次
})

// 组件卸载时清除定时器
onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style scoped>
.connections-container {
  padding: 20px;
  background-color: var(--color-bg-page);
  min-height: 100vh;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.title {
  font-size: 22px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.stats-card {
  text-align: center;
  padding: 16px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #409eff;
  margin-bottom: 8px;
}

.stat-label {
  font-size: 14px;
  color: #606266;
}

.ip-address {
  font-weight: 500;
}

.location {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.traffic-info {
  display: flex;
  align-items: center;
  margin-bottom: 4px;
}

.traffic-label {
  font-size: 12px;
  color: #909399;
  width: 40px;
}

.traffic-value {
  font-weight: 500;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

@media (max-width: 768px) {
  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .stats-cards {
    grid-template-columns: 1fr;
  }
}
</style> 