<template>
  <div class="admin-orders-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          订单管理
        </h1>
        <p class="page-subtitle">
          查看订单、处理状态流转和执行人工退款
        </p>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">订单总数</span>
        <strong class="overview-value">{{ pagination.total }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页待支付</span>
        <strong class="overview-value is-warning">{{ pendingOrderCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页可退款</span>
        <strong class="overview-value is-danger">{{ refundableOrderCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页实付总额</span>
        <strong class="overview-value is-primary">¥{{ currentPageRevenue }}</strong>
      </div>
    </div>

    <div class="toolbar-card orders-toolbar-card">
      <div class="orders-toolbar">
        <el-select
          v-model="filter.status"
          class="toolbar-field toolbar-field--status"
          placeholder="订单状态"
          clearable
        >
          <el-option
            label="待支付"
            value="pending"
          />
          <el-option
            label="已支付"
            value="paid"
          />
          <el-option
            label="已完成"
            value="completed"
          />
          <el-option
            label="已取消"
            value="cancelled"
          />
          <el-option
            label="已退款"
            value="refunded"
          />
        </el-select>
        <el-select
          v-model="filter.paymentMethod"
          class="toolbar-field toolbar-field--method"
          placeholder="支付方式"
          clearable
        >
          <el-option
            v-for="method in paymentMethods"
            :key="method"
            :label="getMethodLabel(method)"
            :value="method"
          />
        </el-select>
        <el-input
          v-model="filter.search"
          class="search-input toolbar-field toolbar-field--search"
          placeholder="搜索订单号或用户 ID"
          clearable
          @keyup.enter="applyFilters"
        />
        <el-date-picker
          v-model="filter.dateRange"
          class="toolbar-field toolbar-field--date"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          unlink-panels
          value-format="YYYY-MM-DD"
        />
        <div class="toolbar-field toolbar-field--amount amount-range-field">
          <el-input-number
            v-model="filter.minAmount"
            class="amount-input"
            :min="0"
            :precision="2"
            :step="10"
            controls-position="right"
            placeholder="最低实付"
          />
          <span class="amount-range-separator">-</span>
          <el-input-number
            v-model="filter.maxAmount"
            class="amount-input"
            :min="0"
            :precision="2"
            :step="10"
            controls-position="right"
            placeholder="最高实付"
          />
        </div>
        <div class="filter-actions">
          <el-button
            type="primary"
            @click="applyFilters"
          >
            搜索
          </el-button>
          <el-button @click="resetFilters">
            重置
          </el-button>
        </div>
      </div>
      <div class="toolbar-actions toolbar-actions--summary">
        <span class="toolbar-summary">当前页 {{ orders.length }} 笔订单，共 {{ pagination.total }} 笔</span>
      </div>
    </div>

    <div class="table-shell">
      <el-table
        v-loading="loading"
        :data="orders"
        border
        stripe
        class="orders-table"
        row-key="id"
      >
        <el-table-column
          label="订单对象"
          min-width="300"
        >
          <template #default="{ row }">
            <div class="entity-cell">
              <div class="entity-cell__header">
                <span class="entity-cell__title">{{ row.order_no }}</span>
                <span :class="['metric-pill', getStatusPillClass(row.status)]">{{ getStatusLabel(row.status) }}</span>
              </div>
              <div class="entity-cell__meta">
                <span>用户 ID：{{ row.user_id }}</span>
                <span>套餐：{{ row.plan_name || `套餐 #${row.plan_id}` }}</span>
              </div>
              <div class="entity-cell__hint">
                创建于 {{ row.created_at }}{{ row.expired_at ? `，超时截止 ${row.expired_at}` : '' }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="金额明细"
          min-width="230"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">实付金额</span>
                <span class="stack-value is-strong">¥{{ formatPrice(row.pay_amount) }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">原价</span>
                <span class="stack-value">¥{{ formatPrice(row.original_amount) }}</span>
              </div>
              <div
                v-if="row.discount_amount > 0"
                class="stack-item"
              >
                <span class="stack-label">优惠抵扣</span>
                <span class="stack-value is-success">-¥{{ formatPrice(row.discount_amount) }}</span>
              </div>
              <div
                v-if="row.balance_used > 0"
                class="stack-item"
              >
                <span class="stack-label">余额抵扣</span>
                <span class="stack-value is-success">-¥{{ formatPrice(row.balance_used) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="支付与状态"
          min-width="230"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">支付方式</span>
                <span class="stack-value">{{ getMethodLabel(row.payment_method) || '-' }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">支付流水号</span>
                <span class="stack-value">{{ row.payment_no || '-' }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">支付时间</span>
                <span class="stack-value">{{ row.paid_at || '-' }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="操作"
          min-width="200"
          align="right"
          fixed="right"
        >
          <template #default="{ row }">
            <div class="operation-btns">
              <el-button
                size="small"
                class="row-action row-action--primary"
                @click="viewDetail(row)"
              >
                详情
              </el-button>
              <el-button
                v-if="getStatusAction(row)"
                size="small"
                class="row-action"
                :class="row.status === 'pending' ? 'row-action--warning' : 'row-action--success'"
                @click="updateStatus(row, getStatusAction(row).status)"
              >
                {{ getStatusAction(row).label }}
              </el-button>
              <el-button
                v-if="canRefund(row)"
                size="small"
                class="row-action row-action--danger"
                @click="openRefundDialog(row)"
              >
                退款
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div
      v-if="pagination.total > 0"
      class="pagination-container"
    >
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50]"
        :layout="isMobile ? 'total, prev, next' : 'total, sizes, prev, pager, next'"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
      />
    </div>

    <el-dialog
      v-model="detailVisible"
      title="订单详情"
      :width="detailDialogWidth"
    >
      <el-descriptions
        v-if="currentOrder"
        :column="detailColumns"
        border
      >
        <el-descriptions-item label="订单号">
          {{ currentOrder.order_no }}
        </el-descriptions-item>
        <el-descriptions-item label="用户 ID">
          {{ currentOrder.user_id }}
        </el-descriptions-item>
        <el-descriptions-item label="套餐">
          {{ currentOrder.plan_name || `套餐 #${currentOrder.plan_id}` }}
        </el-descriptions-item>
        <el-descriptions-item label="订单状态">
          <el-tag
            :type="getStatusType(currentOrder.status)"
            size="small"
          >
            {{ getStatusLabel(currentOrder.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="原价">
          ¥{{ formatPrice(currentOrder.original_amount) }}
        </el-descriptions-item>
        <el-descriptions-item label="优惠">
          -¥{{ formatPrice(currentOrder.discount_amount) }}
        </el-descriptions-item>
        <el-descriptions-item label="余额抵扣">
          -¥{{ formatPrice(currentOrder.balance_used) }}
        </el-descriptions-item>
        <el-descriptions-item label="实付">
          ¥{{ formatPrice(currentOrder.pay_amount) }}
        </el-descriptions-item>
        <el-descriptions-item label="支付方式">
          {{ getMethodLabel(currentOrder.payment_method) || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="支付流水号">
          {{ currentOrder.payment_no || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">
          {{ currentOrder.created_at }}
        </el-descriptions-item>
        <el-descriptions-item label="支付时间">
          {{ currentOrder.paid_at || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="过期时间">
          {{ currentOrder.expired_at || '-' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <el-dialog
      v-model="refundVisible"
      title="订单退款"
      :width="refundDialogWidth"
    >
      <div
        v-if="currentOrder"
        class="refund-summary"
      >
        <p>订单号：{{ currentOrder.order_no }}</p>
        <p>实付金额：¥{{ formatPrice(currentOrder.pay_amount + currentOrder.balance_used) }}</p>
        <p>当前状态：{{ getStatusLabel(currentOrder.status) }}</p>
      </div>
      <el-form label-position="top">
        <el-form-item label="退款金额">
          <el-input-number
            v-model="refundForm.amount"
            :min="0"
            :max="refundMaxAmount"
            :precision="2"
            :step="10"
            controls-position="right"
            style="width: 100%"
          />
          <div class="form-tip">
            填 `0` 表示全额退款，最大可退 ¥{{ refundMaxAmount.toFixed(2) }}
          </div>
        </el-form-item>
        <el-form-item label="退款原因">
          <el-input
            v-model="refundForm.reason"
            type="textarea"
            :rows="3"
            maxlength="200"
            show-word-limit
            placeholder="可选，填写退款原因"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="refundVisible = false">
          取消
        </el-button>
        <el-button
          type="danger"
          :loading="submittingRefund"
          @click="submitRefund"
        >
          确认退款
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ordersApi, paymentsApi } from '@/api/index'
import { useViewport } from '@/composables/useViewport'
import { extractErrorMessage } from '@/utils/entitlement'

const { isMobile } = useViewport()

const loading = ref(false)
const orders = ref([])
const paymentMethods = ref(['alipay', 'wechat'])
const currentOrder = ref(null)
const detailVisible = ref(false)
const refundVisible = ref(false)
const submittingRefund = ref(false)

const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const filter = reactive({
  status: '',
  paymentMethod: '',
  search: '',
  dateRange: [],
  minAmount: null,
  maxAmount: null
})
const refundForm = reactive({
  amount: 0,
  reason: ''
})

const detailColumns = computed(() => (isMobile.value ? 1 : 2))
const detailDialogWidth = computed(() => (isMobile.value ? '94%' : '720px'))
const refundDialogWidth = computed(() => (isMobile.value ? '94%' : '520px'))
const refundMaxAmount = computed(() => {
  if (!currentOrder.value) {
    return 0
  }

  return (currentOrder.value.pay_amount + currentOrder.value.balance_used) / 100
})
const pendingOrderCount = computed(() => orders.value.filter((order) => order.status === 'pending').length)
const refundableOrderCount = computed(() => orders.value.filter((order) => canRefund(order)).length)
const currentPageRevenue = computed(() => (orders.value.reduce((sum, order) => sum + Number(order.pay_amount || 0), 0) / 100).toFixed(2))

const statusMap = {
  pending: { label: '待支付', type: 'warning' },
  paid: { label: '已支付', type: 'success' },
  completed: { label: '已完成', type: 'success' },
  cancelled: { label: '已取消', type: 'info' },
  refunded: { label: '已退款', type: 'danger' }
}

const methodLabels = {
  alipay: '支付宝',
  wechat: '微信支付',
  paypal: 'PayPal',
  crypto: '加密货币',
  balance: '余额支付'
}

const formatPrice = (price) => (Number(price || 0) / 100).toFixed(2)
const getStatusType = (status) => statusMap[status]?.type || 'info'
const getStatusLabel = (status) => statusMap[status]?.label || status || '-'
const getMethodLabel = (method) => methodLabels[method] || method || '-'
const getStatusPillClass = (status) => {
  const type = getStatusType(status)
  if (type === 'success') return 'is-success'
  if (type === 'warning') return 'is-warning'
  if (type === 'danger') return 'is-danger'
  return 'is-muted'
}
const canRefund = (order) => ['paid', 'completed'].includes(order.status)
const getStatusAction = (order) => {
  if (order.status === 'pending') {
    return { label: '取消', status: 'cancelled' }
  }

  if (order.status === 'paid') {
    return { label: '完成', status: 'completed' }
  }

  return null
}

const toAmountInCents = (amount) => {
  if (amount === null || amount === undefined || amount === '') {
    return undefined
  }

  return Math.round(Number(amount) * 100)
}

const buildParams = () => {
  const params = {
    page: pagination.page,
    page_size: pagination.pageSize,
    status: filter.status || undefined,
    payment_method: filter.paymentMethod || undefined,
    search: filter.search.trim() || undefined,
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
    const res = await ordersApi.admin.list(buildParams())
    orders.value = res.orders || []
    pagination.total = res.total || 0
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '获取订单列表失败')
  } finally {
    loading.value = false
  }
}

const fetchPaymentMethods = async () => {
  try {
    const res = await paymentsApi.getMethods()
    if (Array.isArray(res.methods) && res.methods.length > 0) {
      paymentMethods.value = res.methods
    }
  } catch (error) {
    console.error('Failed to fetch payment methods:', error)
  }
}

const applyFilters = async () => {
  pagination.page = 1
  await fetchOrders()
}

const resetFilters = async () => {
  filter.status = ''
  filter.paymentMethod = ''
  filter.search = ''
  filter.dateRange = []
  filter.minAmount = null
  filter.maxAmount = null
  pagination.page = 1
  await fetchOrders()
}

const handlePageChange = async (page) => {
  pagination.page = page
  await fetchOrders()
}

const handleSizeChange = async (pageSize) => {
  pagination.page = 1
  pagination.pageSize = pageSize
  await fetchOrders()
}

const viewDetail = async (order) => {
  try {
    const res = await ordersApi.admin.get(order.id)
    currentOrder.value = res.order
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '获取订单详情失败')
  }
}

const updateStatus = async (order, status) => {
  const actionLabel = status === 'cancelled' ? '取消订单' : '标记完成'

  try {
    await ElMessageBox.confirm(
      `确认要对订单 ${order.order_no} 执行“${actionLabel}”吗？`,
      '确认操作',
      { type: 'warning' }
    )

    await ordersApi.admin.updateStatus(order.id, status)
    ElMessage.success(`${actionLabel}成功`)
    await fetchOrders()
  } catch (error) {
    if (error === 'cancel') {
      return
    }
    ElMessage.error(extractErrorMessage(error) || `${actionLabel}失败`)
  }
}

const openRefundDialog = (order) => {
  currentOrder.value = order
  refundForm.amount = 0
  refundForm.reason = ''
  refundVisible.value = true
}

const submitRefund = async () => {
  if (!currentOrder.value) {
    return
  }

  const amountInCents = toAmountInCents(refundForm.amount) || 0
  if (amountInCents < 0) {
    ElMessage.warning('退款金额不能小于 0')
    return
  }

  if (amountInCents > currentOrder.value.pay_amount + currentOrder.value.balance_used) {
    ElMessage.warning('退款金额超过订单可退金额')
    return
  }

  submittingRefund.value = true
  try {
    await ordersApi.admin.refund(currentOrder.value.id, {
      amount: amountInCents,
      reason: refundForm.reason.trim()
    })
    ElMessage.success('退款已处理')
    refundVisible.value = false
    await fetchOrders()
    if (detailVisible.value) {
      await viewDetail(currentOrder.value)
    }
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '退款失败')
  } finally {
    submittingRefund.value = false
  }
}

onMounted(() => {
  fetchOrders()
  fetchPaymentMethods()
})
</script>

<style scoped>
.admin-orders-page {
  padding: 20px;
}

.orders-toolbar-card {
  align-items: stretch;
}

.orders-toolbar {
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

.search-input {
  min-width: 0;
}

.amount-range-field {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.amount-range-separator {
  color: var(--admin-text-muted);
  font-size: 14px;
  text-align: center;
}

.amount-input {
  min-width: 0;
}

.orders-toolbar :deep(.el-date-editor.el-input__wrapper) {
  min-height: 44px;
}

.orders-toolbar :deep(.el-date-editor .el-range-input) {
  min-width: 0;
  font-size: 13px;
}

.orders-toolbar :deep(.el-select),
.orders-toolbar :deep(.el-input),
.orders-toolbar :deep(.el-date-editor),
.orders-toolbar :deep(.el-input-number) {
  width: 100%;
  max-width: 100%;
}

.orders-toolbar :deep(.amount-input .el-input__wrapper) {
  padding-right: 34px;
}

.orders-toolbar :deep(.el-input-number__increase),
.orders-toolbar :deep(.el-input-number__decrease) {
  background: transparent;
  width: 28px;
}

.filter-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  margin-left: auto;
}

.toolbar-actions--summary {
  width: 100%;
  justify-content: flex-start;
}

.orders-table {
  width: 100%;
  min-width: 980px;
}

.refund-summary {
  margin-bottom: 16px;
  padding: 14px 16px;
  border-radius: 14px;
  background: rgba(248, 250, 252, 0.92);
  color: #334155;
}

.refund-summary p {
  margin: 0 0 8px;
}

.refund-summary p:last-child {
  margin-bottom: 0;
}

.form-tip {
  margin-top: 6px;
  font-size: 12px;
  color: #64748b;
}

@media (max-width: 1280px) {
  .filter-actions {
    width: 100%;
    margin-left: 0;
  }

  .toolbar-field--status,
  .toolbar-field--method {
    flex-basis: 180px;
  }

  .toolbar-field--date {
    min-width: 280px;
  }

  .toolbar-field--amount {
    min-width: 260px;
  }
}

@media (max-width: 768px) {
  .admin-orders-page {
    padding: 12px;
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

  .orders-table {
    min-width: 760px;
  }

  .filter-actions {
    width: 100%;
    display: grid;
    grid-template-columns: 1fr 1fr;
    margin-left: 0;
  }
}
</style>
