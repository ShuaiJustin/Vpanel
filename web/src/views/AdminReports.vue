<template>
  <div class="admin-reports-page">
    <div class="page-header">
      <h1 class="page-title">财务报表</h1>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        :style="{ width: datePickerWidth }"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        @change="fetchReports"
      />
    </div>

    <!-- 统计卡片 -->
    <div class="stats-grid">
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">¥{{ formatPrice(stats.total_revenue) }}</div>
        <div class="stat-label">总收入</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ stats.total_orders }}</div>
        <div class="stat-label">总订单数</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ stats.paid_orders }}</div>
        <div class="stat-label">已支付订单</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ stats.refunded_orders }}</div>
        <div class="stat-label">退款订单数</div>
      </el-card>
    </div>

    <!-- 支付失败统计 -->
    <el-card shadow="never" class="failed-payments-card">
      <template #header>
        <div class="card-header">
          <span>支付失败统计</span>
          <el-button type="primary" size="small" @click="fetchFailedPaymentStats">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>
      <div class="failed-stats-grid">
        <div class="failed-stat-item">
          <div class="failed-stat-value error">{{ failedPaymentStats.total_failed }}</div>
          <div class="failed-stat-label">失败总数</div>
        </div>
        <div class="failed-stat-item">
          <div class="failed-stat-value warning">{{ failedPaymentStats.pending_retry }}</div>
          <div class="failed-stat-label">待重试</div>
        </div>
        <div class="failed-stat-item">
          <div class="failed-stat-value danger">{{ failedPaymentStats.retry_exhausted }}</div>
          <div class="failed-stat-label">重试耗尽</div>
        </div>
        <div class="failed-stat-item">
          <div class="failed-stat-value success">{{ failedPaymentStats.recovered_by_retry }}</div>
          <div class="failed-stat-label">重试成功</div>
        </div>
        <div class="failed-stat-item">
          <div class="failed-stat-value">{{ failedPaymentStats.failure_rate?.toFixed(2) || 0 }}%</div>
          <div class="failed-stat-label">失败率</div>
        </div>
        <div class="failed-stat-item">
          <div class="failed-stat-value success">{{ failedPaymentStats.recovery_rate?.toFixed(2) || 0 }}%</div>
          <div class="failed-stat-label">恢复率</div>
        </div>
        <div class="failed-stat-item">
          <div class="failed-stat-value">{{ failedPaymentStats.avg_retry_attempts?.toFixed(1) || 0 }}</div>
          <div class="failed-stat-label">平均重试次数</div>
        </div>
      </div>
      
      <!-- 失败原因分布 -->
      <div v-if="Object.keys(failedPaymentStats.failures_by_reason || {}).length > 0" class="failure-reasons">
        <h4>失败原因分布</h4>
        <div class="table-wrap">
        <el-table :data="failureReasonsList" size="small" style="width: 100%">
          <el-table-column prop="reason" label="失败原因" />
          <el-table-column prop="count" label="次数" width="100" />
          <el-table-column label="占比" width="150">
            <template #default="{ row }">
              <el-progress 
                :percentage="Math.round(row.count / failedPaymentStats.total_failed * 100)" 
                :stroke-width="6" 
              />
            </template>
          </el-table-column>
        </el-table>
        </div>
      </div>
    </el-card>

    <!-- 订阅暂停统计 -->
    <el-card shadow="never" class="pause-stats-card">
      <template #header>
        <div class="card-header">
          <span>订阅暂停统计</span>
          <el-button type="primary" size="small" @click="fetchPauseStats">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>
      <div class="pause-stats-grid">
        <div class="pause-stat-item">
          <div class="pause-stat-value">{{ pauseStats.total_pauses }}</div>
          <div class="pause-stat-label">总暂停次数</div>
        </div>
        <div class="pause-stat-item">
          <div class="pause-stat-value warning">{{ pauseStats.active_pauses }}</div>
          <div class="pause-stat-label">当前暂停中</div>
        </div>
        <div class="pause-stat-item">
          <div class="pause-stat-value success">{{ pauseStats.resumed_pauses }}</div>
          <div class="pause-stat-label">已恢复</div>
        </div>
        <div class="pause-stat-item">
          <div class="pause-stat-value">{{ pauseStats.auto_resumed }}</div>
          <div class="pause-stat-label">自动恢复</div>
        </div>
        <div class="pause-stat-item">
          <div class="pause-stat-value">{{ pauseStats.avg_pause_days?.toFixed(1) || 0 }}</div>
          <div class="pause-stat-label">平均暂停天数</div>
        </div>
        <div class="pause-stat-item">
          <div class="pause-stat-value">{{ pauseStats.pause_rate?.toFixed(1) || 0 }}%</div>
          <div class="pause-stat-label">暂停率</div>
        </div>
      </div>
      
      <!-- 暂停滥用检测 -->
      <div v-if="pauseStats.abuse_patterns?.length > 0" class="abuse-patterns">
        <h4>潜在滥用用户</h4>
        <div class="table-wrap">
        <el-table :data="pauseStats.abuse_patterns" size="small" style="width: 100%">
          <el-table-column prop="user_id" label="用户ID" width="100" />
          <el-table-column prop="pause_count" label="暂停次数" width="100" />
          <el-table-column prop="total_pause_days" label="总暂停天数" width="120" />
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button type="primary" link size="small" @click="viewUser(row.user_id)">
                查看
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="chart-card">
      <template #header>
        <span>趋势与排行</span>
      </template>
      <el-alert
        title="当前后端只提供汇总类报表接口"
        description="收入趋势、订单趋势和套餐销售排行仍缺少真实的按日/按套餐统计接口，页面不再展示伪造图表数据。"
        type="info"
        :closable="false"
        show-icon
      />
    </el-card>

    <!-- 套餐销售排行 -->
    <el-card shadow="never">
      <template #header>
        <span>套餐销售排行</span>
      </template>
      <el-empty description="缺少真实套餐排行接口，暂不展示模拟数据" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import api from '@/api'
import { useViewport } from '@/composables/useViewport'

const { isMobile, isTablet } = useViewport({ mobileBreakpoint: 768, tabletBreakpoint: 1280 })

const dateRange = ref(null)

const stats = reactive({
  total_revenue: 0,
  total_orders: 0,
  paid_orders: 0,
  refunded_orders: 0
})

const failedPaymentStats = reactive({
  total_failed: 0,
  pending_retry: 0,
  retry_exhausted: 0,
  recovered_by_retry: 0,
  failure_rate: 0,
  recovery_rate: 0,
  avg_retry_attempts: 0,
  failures_by_method: {},
  failures_by_reason: {}
})

const pauseStats = reactive({
  total_pauses: 0,
  active_pauses: 0,
  resumed_pauses: 0,
  auto_resumed: 0,
  avg_pause_days: 0,
  pause_rate: 0,
  abuse_patterns: []
})
const datePickerWidth = computed(() => (isMobile.value ? '100%' : isTablet.value ? '320px' : '360px'))

const formatPrice = (price) => ((price || 0) / 100).toFixed(2)
const resetFailedPaymentStats = () => {
  Object.assign(failedPaymentStats, {
    total_failed: 0,
    pending_retry: 0,
    retry_exhausted: 0,
    recovered_by_retry: 0,
    failure_rate: 0,
    recovery_rate: 0,
    avg_retry_attempts: 0,
    failures_by_method: {},
    failures_by_reason: {}
  })
}
const resetPauseStats = () => {
  Object.assign(pauseStats, {
    total_pauses: 0,
    active_pauses: 0,
    resumed_pauses: 0,
    auto_resumed: 0,
    avg_pause_days: 0,
    pause_rate: 0,
    abuse_patterns: []
  })
}

// Convert failures_by_reason object to array for table display
const failureReasonsList = computed(() => {
  const reasons = failedPaymentStats.failures_by_reason || {}
  return Object.entries(reasons).map(([reason, count]) => ({
    reason: reason || '未知原因',
    count
  })).sort((a, b) => b.count - a.count)
})

const fetchFailedPaymentStats = async () => {
  try {
    const response = await api.get('/admin/reports/failed-payments')
    Object.assign(failedPaymentStats, response?.stats || response?.data?.stats || {})
  } catch (error) {
    console.error('Failed to fetch failed payment stats:', error)
    resetFailedPaymentStats()
  }
}

const fetchPauseStats = async () => {
  try {
    const response = await api.get('/admin/reports/pause-stats')
    Object.assign(pauseStats, response?.stats || response?.data?.stats || response || {})
  } catch (error) {
    console.error('Failed to fetch pause stats:', error)
    resetPauseStats()
  }
}

const viewUser = (userId) => {
  window.open(`/admin/subscriptions?user_id=${userId}`, '_blank')
}

const fetchReports = async () => {
  try {
    // 获取日期范围
    let startDate = ''
    let endDate = ''
    
    if (dateRange.value && dateRange.value.length === 2) {
      startDate = dateRange.value[0].toISOString().split('T')[0]
      endDate = dateRange.value[1].toISOString().split('T')[0]
    } else {
      // 默认最近30天
      const end = new Date()
      const start = new Date()
      start.setDate(start.getDate() - 30)
      startDate = start.toISOString().split('T')[0]
      endDate = end.toISOString().split('T')[0]
    }

    // 获取收入报表
    const revenueResponse = await api.get('/admin/reports/revenue', {
      params: { start: startDate, end: endDate }
    })
    
    if (revenueResponse.code === 200 && revenueResponse.data) {
      stats.total_revenue = revenueResponse.data.revenue || 0
      stats.total_orders = revenueResponse.data.order_count || 0
    }

    // 获取订单统计
    const orderStatsResponse = await api.get('/admin/reports/orders')
    
    if (orderStatsResponse.code === 200 && orderStatsResponse.data) {
      stats.paid_orders = orderStatsResponse.data.paid || 0
      stats.refunded_orders = orderStatsResponse.data.refunded || 0
    }

    await Promise.all([
      fetchFailedPaymentStats(),
      fetchPauseStats()
    ])
  } catch (error) {
    console.error('Failed to fetch reports:', error)
    ElMessage.error(`获取报表数据失败: ${error.message || '未知错误'}`)
  }
}

onMounted(() => {
  fetchReports()
})
</script>

<style scoped>
.admin-reports-page { padding: 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; margin-bottom: 20px; }
.page-title { font-size: 24px; font-weight: 600; margin: 0; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; margin-bottom: 20px; }
.stat-card { text-align: center; }
.stat-value { font-size: 28px; font-weight: 600; color: #409eff; }
.stat-label { font-size: 14px; color: #909399; margin-top: 8px; }
.chart-card { margin-bottom: 20px; }
.chart-container { height: 300px; }

/* Failed payment stats styles */
.failed-payments-card { margin-bottom: 20px; }
.card-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; }
.failed-stats-grid {
  display: grid; 
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 16px; 
  margin-bottom: 20px;
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
}
.failed-stat-item { text-align: center; }
.failed-stat-value { 
  font-size: 24px; 
  font-weight: 600; 
  color: #606266;
}
.failed-stat-value.error { color: #f56c6c; }
.failed-stat-value.warning { color: #e6a23c; }
.failed-stat-value.danger { color: #f56c6c; }
.failed-stat-value.success { color: #67c23a; }
.failed-stat-label { 
  font-size: 12px; 
  color: #909399; 
  margin-top: 4px; 
}
.failure-reasons { margin-top: 16px; }
.table-wrap { overflow-x: auto; }
.table-wrap :deep(.el-table) { min-width: 480px; }
.failure-reasons h4 { 
  font-size: 14px; 
  font-weight: 600; 
  margin-bottom: 12px; 
  color: #303133;
}

/* Pause stats styles */
.pause-stats-card { margin-bottom: 20px; }
.pause-stats-grid {
  display: grid; 
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 16px; 
  margin-bottom: 20px;
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
}
.pause-stat-item { text-align: center; }
.pause-stat-value { 
  font-size: 24px; 
  font-weight: 600; 
  color: #606266;
}
.pause-stat-value.warning { color: #e6a23c; }
.pause-stat-value.success { color: #67c23a; }
.pause-stat-label { 
  font-size: 12px; 
  color: #909399; 
  margin-top: 4px; 
}
.abuse-patterns { margin-top: 16px; }
.abuse-patterns h4 { 
  font-size: 14px; 
  font-weight: 600; 
  margin-bottom: 12px; 
  color: #303133;
}

@media (max-width: 1200px) {
  .admin-reports-page { padding: 16px; }
}
@media (max-width: 768px) {
  .admin-reports-page { padding: 12px; }
  .page-header { align-items: stretch; }
  .stats-grid { grid-template-columns: 1fr 1fr; }
  .failed-stats-grid { grid-template-columns: 1fr 1fr; }
  .pause-stats-grid { grid-template-columns: 1fr 1fr; }
}

@media (max-width: 520px) {
  .stats-grid,
  .failed-stats-grid,
  .pause-stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
