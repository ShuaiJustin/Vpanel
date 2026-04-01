<template>
  <div class="admin-recharge-orders-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          充值订单
        </h1>
        <p class="page-subtitle">
          查看用户余额充值订单与支付到账情况
        </p>
      </div>
      <div class="page-actions">
        <el-button
          :loading="loading"
          @click="fetchOrders"
        >
          刷新
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">充值订单总数</span>
        <strong class="overview-value">{{ pagination.total }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页待支付</span>
        <strong class="overview-value is-warning">{{ pendingCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页已到账</span>
        <strong class="overview-value is-success">{{ paidCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页到账金额</span>
        <strong class="overview-value is-primary">¥{{ currentPagePaidAmount }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-grid">
        <el-input
          v-model="filter.search"
          class="toolbar-field toolbar-field--search"
          clearable
          placeholder="搜索订单号 / 流水号 / 用户名 / 用户ID"
          @keyup.enter="applyFilters"
        />
        <el-select
          v-model="filter.status"
          class="toolbar-field toolbar-field--status"
          clearable
          placeholder="订单状态"
        >
          <el-option label="待支付" value="pending" />
          <el-option label="已到账" value="paid" />
          <el-option label="已取消" value="cancelled" />
          <el-option label="已过期" value="expired" />
        </el-select>
        <el-select
          v-model="filter.method"
          class="toolbar-field toolbar-field--method"
          clearable
          placeholder="支付方式"
        >
          <el-option label="支付宝" value="alipay" />
          <el-option label="微信支付" value="wechat" />
        </el-select>
        <el-date-picker
          v-model="filter.dateRange"
          class="toolbar-field toolbar-field--date"
          type="daterange"
          value-format="YYYY-MM-DD"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          range-separator="至"
        />
        <div class="toolbar-field toolbar-field--amount amount-range-field">
          <el-input-number
            v-model="filter.minAmount"
            class="amount-input"
            :min="0"
            :precision="2"
            controls-position="right"
            placeholder="最低金额"
          />
          <span class="amount-range-separator">-</span>
          <el-input-number
            v-model="filter.maxAmount"
            class="amount-input"
            :min="0"
            :precision="2"
            controls-position="right"
            placeholder="最高金额"
          />
        </div>
      </div>
      <div class="toolbar-actions">
        <el-button @click="resetFilters">
          重置
        </el-button>
        <el-button
          type="primary"
          @click="applyFilters"
        >
          筛选
        </el-button>
      </div>
    </div>

    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>充值订单列表</span>
          <span class="toolbar-summary">当前页 {{ orders.length }} 条，共 {{ pagination.total }} 条</span>
        </div>
      </template>

      <el-table
        v-loading="loading"
        :data="orders"
        border
      >
        <el-table-column
          prop="order_no"
          label="充值订单号"
          min-width="200"
        />
        <el-table-column
          label="用户"
          min-width="180"
        >
          <template #default="{ row }">
            <div class="user-cell">
              <span class="user-cell__name">{{ row.username || '-' }}</span>
              <span class="user-cell__meta">UID {{ row.user_id }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          label="充值金额"
          min-width="120"
          align="right"
        >
          <template #default="{ row }">
            <span class="amount-text">¥{{ formatPrice(row.amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="支付方式"
          min-width="120"
        >
          <template #default="{ row }">
            {{ getMethodLabel(row.method) }}
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          min-width="110"
        >
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">
              {{ getStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="payment_no"
          label="外部流水号"
          min-width="180"
          show-overflow-tooltip
        />
        <el-table-column
          prop="created_at"
          label="创建时间"
          min-width="170"
        />
        <el-table-column
          prop="paid_at"
          label="到账时间"
          min-width="170"
        >
          <template #default="{ row }">
            {{ row.paid_at || '-' }}
          </template>
        </el-table-column>
      </el-table>

      <el-empty
        v-if="!loading && orders.length === 0"
        description="暂无充值订单"
      />

      <div
        v-if="pagination.total > 0"
        class="pagination-container"
      >
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { balanceApi } from '@/api'
import { extractErrorMessage } from '@/utils/entitlement'

const loading = ref(false)
const orders = ref([])

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const filter = reactive({
  search: '',
  status: '',
  method: '',
  dateRange: [],
  minAmount: null,
  maxAmount: null
})

const statusMap = {
  pending: { label: '待支付', type: 'warning' },
  paid: { label: '已到账', type: 'success' },
  cancelled: { label: '已取消', type: 'info' },
  expired: { label: '已过期', type: 'danger' }
}

const methodLabels = {
  alipay: '支付宝',
  wechat: '微信支付'
}

const pendingCount = computed(() => orders.value.filter(order => order.status === 'pending').length)
const paidCount = computed(() => orders.value.filter(order => order.status === 'paid').length)
const currentPagePaidAmount = computed(() => {
  const total = orders.value
    .filter(order => order.status === 'paid')
    .reduce((sum, order) => sum + Number(order.amount || 0), 0)
  return formatPrice(total)
})

const formatPrice = price => (Number(price || 0) / 100).toFixed(2)
const getMethodLabel = method => methodLabels[method] || method || '-'
const getStatusLabel = status => statusMap[status]?.label || status || '-'
const getStatusType = status => statusMap[status]?.type || 'info'

const toAmountInCents = amount => {
  if (amount === null || amount === undefined || amount === '') {
    return undefined
  }

  return Math.round(Number(amount) * 100)
}

const buildParams = () => {
  const params = {
    page: pagination.page,
    page_size: pagination.pageSize,
    search: filter.search.trim() || undefined,
    status: filter.status || undefined,
    method: filter.method || undefined,
    min_amount: toAmountInCents(filter.minAmount),
    max_amount: toAmountInCents(filter.maxAmount)
  }

  if (filter.dateRange?.length === 2) {
    params.start_date = `${filter.dateRange[0]} 00:00:00`
    params.end_date = `${filter.dateRange[1]} 23:59:59`
  }

  return params
}

const fetchOrders = async () => {
  loading.value = true
  try {
    const response = await balanceApi.admin.listRechargeOrders(buildParams())
    orders.value = response.orders || []
    pagination.total = response.total || 0
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '获取充值订单失败')
  } finally {
    loading.value = false
  }
}

const applyFilters = async () => {
  pagination.page = 1
  await fetchOrders()
}

const resetFilters = async () => {
  filter.search = ''
  filter.status = ''
  filter.method = ''
  filter.dateRange = []
  filter.minAmount = null
  filter.maxAmount = null
  pagination.page = 1
  await fetchOrders()
}

const handlePageChange = async page => {
  pagination.page = page
  await fetchOrders()
}

const handleSizeChange = async size => {
  pagination.pageSize = size
  pagination.page = 1
  await fetchOrders()
}

onMounted(fetchOrders)
</script>

<style scoped>
.admin-recharge-orders-page {
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

.page-subtitle {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.overview-card,
.toolbar-card {
  border: 1px solid var(--el-border-color-light);
  border-radius: 16px;
  background: var(--el-bg-color-overlay);
  padding: 16px 18px;
}

.overview-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.overview-label,
.toolbar-summary,
.user-cell__meta {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.overview-value {
  font-size: 26px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.overview-value.is-primary {
  color: var(--el-color-primary);
}

.overview-value.is-success {
  color: var(--el-color-success);
}

.overview-value.is-warning {
  color: var(--el-color-warning);
}

.toolbar-card {
  margin-bottom: 20px;
}

.toolbar-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  width: 100%;
  align-items: stretch;
}

.toolbar-field {
  min-width: 0;
  flex: 1 1 180px;
}

.toolbar-field--status,
.toolbar-field--method {
  flex: 0 1 168px;
}

.toolbar-field--date {
  flex: 1.35 1 340px;
  min-width: 320px;
}

.toolbar-field--search {
  flex: 1.15 1 260px;
}

.toolbar-field--amount {
  flex: 1.15 1 300px;
  min-width: 280px;
}

.amount-range-field {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.amount-range-separator {
  color: var(--admin-text-muted, var(--el-text-color-secondary));
  font-size: 14px;
  text-align: center;
}

.amount-input {
  min-width: 0;
}

.toolbar-grid :deep(.el-date-editor.el-input__wrapper) {
  min-height: 44px;
}

.toolbar-grid :deep(.el-date-editor .el-range-input) {
  min-width: 0;
  font-size: 13px;
}

.toolbar-grid :deep(.el-input),
.toolbar-grid :deep(.el-select),
.toolbar-grid :deep(.el-date-editor),
.toolbar-grid :deep(.el-input-number) {
  width: 100%;
  max-width: 100%;
}

.toolbar-grid :deep(.amount-input .el-input__wrapper) {
  padding-right: 34px;
}

.toolbar-grid :deep(.el-input-number__increase),
.toolbar-grid :deep(.el-input-number__decrease) {
  background: transparent;
  width: 28px;
}

.toolbar-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.user-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.user-cell__name,
.amount-text {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 1280px) {
  .toolbar-field--date {
    min-width: 280px;
  }

  .toolbar-field--amount {
    min-width: 260px;
  }
}

@media (max-width: 768px) {
  .admin-recharge-orders-page {
    padding: 12px;
  }

  .page-header,
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-actions {
    justify-content: stretch;
    display: grid;
    grid-template-columns: 1fr 1fr;
  }

  .toolbar-field--status,
  .toolbar-field--method,
  .toolbar-field--search,
  .toolbar-field--date,
  .toolbar-field--amount {
    flex: 1 1 100%;
    min-width: 0;
  }

  .amount-range-field {
    grid-template-columns: 1fr;
  }

  .amount-range-separator {
    display: none;
  }
}
</style>
