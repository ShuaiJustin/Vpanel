<template>
  <div class="admin-orders-page">
    <div class="page-header">
      <h1 class="page-title">订单管理</h1>
    </div>

    <el-card shadow="never">
      <!-- 筛选 -->
      <div class="filter-bar">
        <el-select v-model="filter.status" placeholder="订单状态" clearable @change="fetchOrders">
          <el-option label="待支付" value="pending" />
          <el-option label="已支付" value="paid" />
          <el-option label="已完成" value="completed" />
          <el-option label="已取消" value="cancelled" />
          <el-option label="已退款" value="refunded" />
        </el-select>
        <el-input v-model="filter.search" placeholder="搜索订单号/用户ID" clearable style="width: 200px;" @keyup.enter="fetchOrders" />
        <el-date-picker v-model="filter.dateRange" type="daterange" start-placeholder="开始日期" end-placeholder="结束日期" @change="fetchOrders" />
        <el-button type="primary" @click="fetchOrders">搜索</el-button>
      </div>

      <el-table :data="orders" v-loading="loading" style="width: 100%">
        <el-table-column prop="order_no" label="订单号" width="180" />
        <el-table-column prop="user_id" label="用户ID" width="100" />
        <el-table-column prop="plan_name" label="套餐" />
        <el-table-column label="金额" width="120">
          <template #default="{ row }">
            <span>¥{{ formatPrice(row.pay_amount) }}</span>
            <span v-if="row.discount_amount > 0" class="discount">-¥{{ formatPrice(row.discount_amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">{{ getStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="payment_method" label="支付方式" width="100" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="viewDetail(row)">详情</el-button>
            <el-button v-if="row.status === 'pending'" type="success" link @click="handleRetry(row)">重试支付</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="pagination.total > 0" class="pagination-container">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @size-change="fetchOrders"
          @current-change="fetchOrders"
        />
      </div>
    </el-card>

    <!-- 详情对话框 -->
    <el-dialog v-model="detailVisible" title="订单详情" width="600px">
      <el-descriptions v-if="currentOrder" :column="2" border>
        <el-descriptions-item label="订单号">{{ currentOrder.order_no }}</el-descriptions-item>
        <el-descriptions-item label="用户ID">{{ currentOrder.user_id }}</el-descriptions-item>
        <el-descriptions-item label="套餐">{{ currentOrder.plan_name }}</el-descriptions-item>
        <el-descriptions-item label="原价">¥{{ formatPrice(currentOrder.original_amount) }}</el-descriptions-item>
        <el-descriptions-item label="优惠">-¥{{ formatPrice(currentOrder.discount_amount) }}</el-descriptions-item>
        <el-descriptions-item label="余额抵扣">-¥{{ formatPrice(currentOrder.balance_used) }}</el-descriptions-item>
        <el-descriptions-item label="实付">¥{{ formatPrice(currentOrder.pay_amount) }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="getStatusType(currentOrder.status)" size="small">{{ getStatusLabel(currentOrder.status) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="支付方式">{{ currentOrder.payment_method || '-' }}</el-descriptions-item>
        <el-descriptions-item label="支付流水号">{{ currentOrder.payment_no || '-' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ currentOrder.created_at }}</el-descriptions-item>
        <el-descriptions-item label="支付时间">{{ currentOrder.paid_at || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <!-- 重试支付对话框 -->
    <el-dialog v-model="retryVisible" title="重试支付" width="450px">
      <div v-if="retryInfo" class="retry-info">
        <el-alert v-if="retryInfo.retry_info" :type="retryInfo.can_retry ? 'warning' : 'error'" :closable="false">
          <template #title>
            <span v-if="retryInfo.can_retry">
              已重试 {{ retryInfo.retry_info.attempt_count }} 次，可继续重试
            </span>
            <span v-else>
              已达到最大重试次数 ({{ retryInfo.retry_info.attempt_count }} 次)
            </span>
          </template>
          <template v-if="retryInfo.retry_info.last_error" #default>
            <div class="last-error">上次失败原因: {{ retryInfo.retry_info.last_error }}</div>
          </template>
        </el-alert>
      </div>
      <el-form :model="retryForm" label-width="100px" style="margin-top: 16px;">
        <el-form-item label="支付方式">
          <el-select v-model="retryForm.method" placeholder="选择支付方式" style="width: 100%;">
            <el-option v-for="method in paymentMethods" :key="method" :label="getMethodLabel(method)" :value="method" />
          </el-select>
        </el-form-item>
        <el-form-item label="切换方式">
          <el-checkbox v-model="retryForm.switchMethod">切换到新支付方式</el-checkbox>
          <div class="form-tip">勾选后将更新订单的支付方式</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="retryVisible = false">取消</el-button>
        <el-button type="primary" :loading="retrying" :disabled="!retryInfo?.can_retry" @click="confirmRetry">
          {{ retryForm.switchMethod ? '切换并重试' : '重试支付' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ordersApi, paymentsApi } from '@/api/index'

const loading = ref(false)
const orders = ref([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const filter = reactive({ status: '', search: '', dateRange: null })
const detailVisible = ref(false)
const retryVisible = ref(false)
const currentOrder = ref(null)
const retrying = ref(false)
const retryForm = reactive({ method: '', switchMethod: false })
const retryInfo = ref(null)
const paymentMethods = ref(['alipay', 'wechat'])

const formatPrice = (price) => (price / 100).toFixed(2)

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

const getStatusType = (status) => statusMap[status]?.type || 'info'
const getStatusLabel = (status) => statusMap[status]?.label || status
const getMethodLabel = (method) => methodLabels[method] || method

const fetchOrders = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      status: filter.status || undefined,
      search: filter.search || undefined
    }
    if (filter.dateRange) {
      params.start_date = filter.dateRange[0].toISOString()
      params.end_date = filter.dateRange[1].toISOString()
    }
    const res = await ordersApi.admin.list(params)
    orders.value = res.orders || []
    pagination.total = res.total || 0
  } catch (e) {
    ElMessage.error(e.message || '获取订单列表失败')
  } finally {
    loading.value = false
  }
}

const fetchPaymentMethods = async () => {
  try {
    const res = await paymentsApi.getMethods()
    if (res.methods && res.methods.length > 0) {
      paymentMethods.value = res.methods
    }
  } catch (e) {
    console.error('Failed to fetch payment methods:', e)
  }
}

const viewDetail = async (order) => {
  try {
    const res = await ordersApi.admin.get(order.id)
    currentOrder.value = res.order
    detailVisible.value = true
  } catch (e) {
    ElMessage.error(e.message || '获取订单详情失败')
  }
}

const handleRetry = async (order) => {
  currentOrder.value = order
  retryForm.method = order.payment_method || paymentMethods.value[0] || 'alipay'
  retryForm.switchMethod = false
  retryInfo.value = null
  retryVisible.value = true

  // Fetch retry info
  try {
    const res = await paymentsApi.getRetryInfo(order.id)
    retryInfo.value = res
  } catch (e) {
    retryInfo.value = { can_retry: true, retry_info: null }
  }
}

const confirmRetry = async () => {
  if (!retryForm.method) {
    ElMessage.warning('请选择支付方式')
    return
  }

  retrying.value = true
  try {
    // Switch payment method if requested
    if (retryForm.switchMethod) {
      await paymentsApi.switchMethod({
        order_id: currentOrder.value.id,
        method: retryForm.method
      })
    }

    // Retry payment
    await paymentsApi.retry({
      order_id: currentOrder.value.id,
      method: retryForm.method
    })

    ElMessage.success('支付重试已发起')
    retryVisible.value = false
    fetchOrders()
  } catch (e) {
    ElMessage.error(e.message || '重试失败')
  } finally {
    retrying.value = false
  }
}

onMounted(() => {
  fetchOrders()
  fetchPaymentMethods()
})
</script>

<style scoped>
.admin-orders-page { padding: 20px; }
.page-header { margin-bottom: 20px; }
.page-title { font-size: 24px; font-weight: 600; margin: 0; }
.filter-bar { display: flex; gap: 12px; margin-bottom: 20px; flex-wrap: wrap; }
.discount { display: block; font-size: 12px; color: #67c23a; }
.pagination-container { margin-top: 20px; display: flex; justify-content: flex-end; }
.retry-info { margin-bottom: 16px; }
.last-error { font-size: 12px; color: #909399; margin-top: 4px; }
.form-tip { font-size: 12px; color: #909399; margin-top: 4px; }
</style>
