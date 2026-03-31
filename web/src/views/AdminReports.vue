<template>
  <div class="admin-reports-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          商业化报表
        </h1>
        <p class="page-subtitle">
          按订单收款、在线充值、余额消费和人工调整拆分查看，后台口径更清楚。
        </p>
      </div>
      <div class="page-actions">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          :style="{ width: datePickerWidth }"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          range-separator="至"
          @change="fetchReports"
        />
        <el-button
          type="primary"
          :loading="loadingOverview || loadingOperations"
          @click="fetchReports"
        >
          <el-icon><Refresh /></el-icon>
          刷新报表
        </el-button>
      </div>
    </div>

    <el-card
      shadow="never"
      class="section-card"
    >
      <template #header>
        <div class="card-header">
          <div>
            <span>经营总览</span>
            <p class="card-subtitle">
              统计周期：{{ overviewPeriodLabel }}
            </p>
          </div>
          <el-tag type="info" effect="plain">
            仅统计真实业务数据
          </el-tag>
        </div>
      </template>

      <div
        v-loading="loadingOverview"
        class="overview-grid"
      >
        <div class="metric-card">
          <span class="metric-label">总现金流入</span>
          <strong class="metric-value is-primary">¥{{ formatPrice(totalCashIn) }}</strong>
          <span class="metric-hint">订单收款 + 在线充值到账</span>
        </div>
        <div class="metric-card">
          <span class="metric-label">订单收款</span>
          <strong class="metric-value">¥{{ formatPrice(overview.orders.revenue) }}</strong>
          <span class="metric-hint">{{ overview.orders.count }} 笔已收款订单</span>
        </div>
        <div class="metric-card">
          <span class="metric-label">在线充值到账</span>
          <strong class="metric-value is-success">¥{{ formatPrice(overview.recharges.amount) }}</strong>
          <span class="metric-hint">{{ overview.recharges.count }} 笔充值到账</span>
        </div>
        <div class="metric-card">
          <span class="metric-label">人工调整净额</span>
          <strong :class="['metric-value', overview.adjustments.net_amount >= 0 ? 'is-warning' : 'is-danger']">
            {{ formatSignedPrice(overview.adjustments.net_amount) }}
          </strong>
          <span class="metric-hint">{{ overview.adjustments.count }} 笔人工调整</span>
        </div>
      </div>
    </el-card>

    <div class="content-grid">
      <el-card
        shadow="never"
        class="section-card"
      >
        <template #header>
          <div class="card-header">
            <div>
              <span>订单收款</span>
              <p class="card-subtitle">
                只统计 `paid` / `completed` 状态的真实收款金额
              </p>
            </div>
            <el-button link @click="openAdminPage('/admin/orders')">
              查看订单管理
            </el-button>
          </div>
        </template>

        <div class="detail-list">
          <div class="detail-row">
            <span class="detail-label">已收款订单数</span>
            <strong class="detail-value">{{ overview.orders.count }}</strong>
          </div>
          <div class="detail-row">
            <span class="detail-label">订单收款金额</span>
            <strong class="detail-value">¥{{ formatPrice(overview.orders.revenue) }}</strong>
          </div>
          <div class="detail-row detail-row--note">
            <span class="detail-label">说明</span>
            <span class="detail-note">这里看的是套餐订单的外部实收金额，不和充值流水混算。</span>
          </div>
        </div>
      </el-card>

      <el-card
        shadow="never"
        class="section-card"
      >
        <template #header>
          <div class="card-header">
            <div>
              <span>余额商业化</span>
              <p class="card-subtitle">
                在线充值、余额消费、人工调整分开统计
              </p>
            </div>
            <div class="link-actions">
              <el-button link @click="openAdminPage('/admin/recharge-orders')">
                查看充值订单
              </el-button>
              <el-button link @click="openAdminPage('/admin/balances')">
                查看余额管理
              </el-button>
            </div>
          </div>
        </template>

        <div class="detail-list">
          <div class="detail-row">
            <span class="detail-label">在线充值到账</span>
            <div class="detail-pair">
              <strong class="detail-value is-success">¥{{ formatPrice(overview.recharges.amount) }}</strong>
              <span class="detail-meta">{{ overview.recharges.count }} 笔</span>
            </div>
          </div>
          <div class="detail-row">
            <span class="detail-label">余额支付消耗</span>
            <div class="detail-pair">
              <strong class="detail-value">¥{{ formatPrice(overview.balance_purchases.amount) }}</strong>
              <span class="detail-meta">{{ overview.balance_purchases.count }} 笔</span>
            </div>
          </div>
          <div class="detail-row">
            <span class="detail-label">人工增加余额</span>
            <div class="detail-pair">
              <strong class="detail-value is-warning">¥{{ formatPrice(overview.adjustments.increase_amount) }}</strong>
              <span class="detail-meta">{{ overview.adjustments.increase_count }} 笔</span>
            </div>
          </div>
          <div class="detail-row">
            <span class="detail-label">人工扣减余额</span>
            <div class="detail-pair">
              <strong class="detail-value is-danger">¥{{ formatPrice(overview.adjustments.decrease_amount) }}</strong>
              <span class="detail-meta">{{ overview.adjustments.decrease_count }} 笔</span>
            </div>
          </div>
          <div class="detail-row detail-row--highlight">
            <span class="detail-label">调整净额</span>
            <strong :class="['detail-value', overview.adjustments.net_amount >= 0 ? 'is-warning' : 'is-danger']">
              {{ formatSignedPrice(overview.adjustments.net_amount) }}
            </strong>
          </div>
        </div>
      </el-card>
    </div>

    <div class="content-grid">
      <el-card
        shadow="never"
        class="section-card"
      >
        <template #header>
          <div class="card-header">
            <div>
              <span>支付稳定性</span>
              <p class="card-subtitle">
                用于判断支付链路是否稳定、是否存在大面积失败
              </p>
            </div>
            <el-button link @click="openAdminPage('/admin/payment-settings')">
              查看支付配置
            </el-button>
          </div>
        </template>

        <div
          v-loading="loadingOperations"
          class="compact-grid"
        >
          <div class="compact-item">
            <span class="compact-label">失败总数</span>
            <strong class="compact-value is-danger">{{ failedPaymentStats.total_failed }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">待重试</span>
            <strong class="compact-value is-warning">{{ failedPaymentStats.pending_retry }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">重试成功</span>
            <strong class="compact-value is-success">{{ failedPaymentStats.recovered_by_retry }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">失败率</span>
            <strong class="compact-value">{{ formatPercent(failedPaymentStats.failure_rate) }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">恢复率</span>
            <strong class="compact-value is-success">{{ formatPercent(failedPaymentStats.recovery_rate) }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">平均重试次数</span>
            <strong class="compact-value">{{ Number(failedPaymentStats.avg_retry_attempts || 0).toFixed(1) }}</strong>
          </div>
        </div>

        <div
          v-if="failureReasonsList.length > 0"
          class="table-wrap"
        >
          <el-table
            :data="failureReasonsList"
            size="small"
          >
            <el-table-column prop="reason" label="失败原因" min-width="180" />
            <el-table-column prop="count" label="次数" width="90" />
            <el-table-column label="占比" min-width="140">
              <template #default="{ row }">
                <el-progress
                  :percentage="Math.round((row.count / Math.max(failedPaymentStats.total_failed || 1, 1)) * 100)"
                  :stroke-width="6"
                />
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-card>

      <el-card
        shadow="never"
        class="section-card"
      >
        <template #header>
          <div class="card-header">
            <div>
              <span>订阅暂停行为</span>
              <p class="card-subtitle">
                这是商业化运营数据，不再和财务数据混在一起理解
              </p>
            </div>
            <el-button link @click="openAdminPage('/admin/subscriptions')">
              查看订阅管理
            </el-button>
          </div>
        </template>

        <div
          v-loading="loadingOperations"
          class="compact-grid"
        >
          <div class="compact-item">
            <span class="compact-label">总暂停次数</span>
            <strong class="compact-value">{{ pauseStats.total_pauses }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">当前暂停中</span>
            <strong class="compact-value is-warning">{{ pauseStats.active_pauses }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">已恢复</span>
            <strong class="compact-value is-success">{{ pauseStats.resumed_pauses }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">自动恢复</span>
            <strong class="compact-value">{{ pauseStats.auto_resumed }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">平均暂停天数</span>
            <strong class="compact-value">{{ Number(pauseStats.avg_pause_days || 0).toFixed(1) }}</strong>
          </div>
          <div class="compact-item">
            <span class="compact-label">暂停率</span>
            <strong class="compact-value">{{ formatPercent(pauseStats.pause_rate) }}</strong>
          </div>
        </div>

        <div
          v-if="pauseStats.abuse_patterns?.length > 0"
          class="table-wrap"
        >
          <el-table
            :data="pauseStats.abuse_patterns"
            size="small"
          >
            <el-table-column prop="user_id" label="用户 ID" width="100" />
            <el-table-column prop="pause_count" label="暂停次数" width="100" />
            <el-table-column prop="total_pause_days" label="总暂停天数" width="120" />
            <el-table-column label="操作" width="100">
              <template #default="{ row }">
                <el-button
                  type="primary"
                  link
                  size="small"
                  @click="viewUser(row.user_id)"
                >
                  查看
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import api from '@/api'
import { useViewport } from '@/composables/useViewport'
import { extractErrorMessage } from '@/utils/entitlement'

const { isMobile, isTablet } = useViewport()

const pad = value => String(value).padStart(2, '0')
const formatDate = value => `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`
const createDefaultDateRange = () => {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - 29)
  return [formatDate(start), formatDate(end)]
}

const loadingOverview = ref(false)
const loadingOperations = ref(false)
const dateRange = ref(createDefaultDateRange())

const overview = reactive({
  start: '',
  end: '',
  orders: {
    revenue: 0,
    count: 0
  },
  recharges: {
    amount: 0,
    count: 0
  },
  balance_purchases: {
    amount: 0,
    count: 0
  },
  adjustments: {
    increase_amount: 0,
    decrease_amount: 0,
    net_amount: 0,
    increase_count: 0,
    decrease_count: 0,
    count: 0
  }
})

const failedPaymentStats = reactive({
  total_failed: 0,
  pending_retry: 0,
  retry_exhausted: 0,
  recovered_by_retry: 0,
  failure_rate: 0,
  recovery_rate: 0,
  avg_retry_attempts: 0,
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
const overviewPeriodLabel = computed(() => `${overview.start || dateRange.value?.[0] || '-'} 至 ${overview.end || dateRange.value?.[1] || '-'}`)
const totalCashIn = computed(() => Number(overview.orders.revenue || 0) + Number(overview.recharges.amount || 0))

const failureReasonsList = computed(() => {
  const reasons = failedPaymentStats.failures_by_reason || {}
  return Object.entries(reasons)
    .map(([reason, count]) => ({
      reason: reason || '未知原因',
      count
    }))
    .sort((first, second) => second.count - first.count)
})

const formatPrice = value => (Number(value || 0) / 100).toFixed(2)
const formatSignedPrice = value => `${Number(value || 0) >= 0 ? '+' : '-'}¥${formatPrice(Math.abs(Number(value || 0)))}`
const formatPercent = value => `${Number(value || 0).toFixed(1)}%`

const resetOverview = () => {
  overview.start = dateRange.value?.[0] || ''
  overview.end = dateRange.value?.[1] || ''
  Object.assign(overview.orders, { revenue: 0, count: 0 })
  Object.assign(overview.recharges, { amount: 0, count: 0 })
  Object.assign(overview.balance_purchases, { amount: 0, count: 0 })
  Object.assign(overview.adjustments, {
    increase_amount: 0,
    decrease_amount: 0,
    net_amount: 0,
    increase_count: 0,
    decrease_count: 0,
    count: 0
  })
}

const resetFailedPaymentStats = () => {
  Object.assign(failedPaymentStats, {
    total_failed: 0,
    pending_retry: 0,
    retry_exhausted: 0,
    recovered_by_retry: 0,
    failure_rate: 0,
    recovery_rate: 0,
    avg_retry_attempts: 0,
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

const fetchCommercialOverview = async () => {
  loadingOverview.value = true
  try {
    const response = await api.get('/admin/reports/overview', {
      params: {
        start: dateRange.value?.[0],
        end: dateRange.value?.[1]
      }
    })

    const data = response?.data || {}
    overview.start = data.start || dateRange.value?.[0] || ''
    overview.end = data.end || dateRange.value?.[1] || ''
    Object.assign(overview.orders, data.orders || {})
    Object.assign(overview.recharges, data.recharges || {})
    Object.assign(overview.balance_purchases, data.balance_purchases || {})
    Object.assign(overview.adjustments, data.adjustments || {})
  } catch (error) {
    resetOverview()
    throw error
  } finally {
    loadingOverview.value = false
  }
}

const fetchFailedPaymentStats = async () => {
  try {
    const response = await api.get('/admin/reports/failed-payments')
    Object.assign(failedPaymentStats, response?.stats || response?.data?.stats || {})
  } catch (error) {
    resetFailedPaymentStats()
  }
}

const fetchPauseStats = async () => {
  try {
    const response = await api.get('/admin/reports/pause-stats')
    Object.assign(pauseStats, response?.stats || response?.data?.stats || response || {})
  } catch (error) {
    resetPauseStats()
  }
}

const fetchReports = async () => {
  loadingOperations.value = true
  try {
    await Promise.all([
      fetchCommercialOverview(),
      fetchFailedPaymentStats(),
      fetchPauseStats()
    ])
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '获取商业化报表失败')
  } finally {
    loadingOperations.value = false
  }
}

const openAdminPage = path => {
  window.open(path, '_blank')
}

const viewUser = userId => {
  window.open(`/admin/subscriptions?user_id=${userId}`, '_blank')
}

onMounted(fetchReports)
</script>

<style scoped>
.admin-reports-page {
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
  margin: 0;
  font-size: 28px;
  font-weight: 700;
}

.page-subtitle,
.card-subtitle,
.metric-hint,
.detail-meta {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.page-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.section-card {
  border-radius: 16px;
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.link-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.overview-grid,
.compact-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 16px;
}

.metric-card,
.compact-item {
  border: 1px solid var(--el-border-color-light);
  border-radius: 14px;
  padding: 16px;
  background: var(--el-bg-color-page);
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.metric-label,
.compact-label,
.detail-label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.metric-value,
.detail-value,
.compact-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.metric-value.is-primary {
  color: var(--el-color-primary);
}

.metric-value.is-success,
.detail-value.is-success,
.compact-value.is-success {
  color: var(--el-color-success);
}

.metric-value.is-warning,
.detail-value.is-warning,
.compact-value.is-warning {
  color: var(--el-color-warning);
}

.metric-value.is-danger,
.detail-value.is-danger,
.compact-value.is-danger {
  color: var(--el-color-danger);
}

.content-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.detail-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.detail-row:last-child {
  border-bottom: none;
}

.detail-row--note {
  align-items: flex-start;
}

.detail-row--highlight {
  padding-top: 18px;
}

.detail-note {
  max-width: 320px;
  text-align: right;
  color: var(--el-text-color-regular);
  line-height: 1.6;
}

.detail-pair {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.table-wrap {
  margin-top: 16px;
  overflow-x: auto;
}

@media (max-width: 1080px) {
  .content-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .admin-reports-page {
    padding: 12px;
  }

  .page-header,
  .page-actions,
  .card-header,
  .link-actions,
  .detail-row {
    flex-direction: column;
    align-items: flex-start;
  }

  .page-actions {
    width: 100%;
  }

  .detail-note {
    max-width: none;
    text-align: left;
  }
}
</style>
