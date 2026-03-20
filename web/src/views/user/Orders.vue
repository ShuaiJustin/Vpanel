<template>
  <div class="orders-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">我的订单</h1>
      <p class="page-subtitle">查看和管理您的订单</p>
    </div>

    <!-- 订单列表 -->
    <el-card shadow="never" class="orders-card">
      <!-- 筛选 -->
      <div class="orders-filter">
        <el-radio-group v-model="statusFilter" @change="handleFilterChange">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="pending">待支付</el-radio-button>
          <el-radio-button value="paid">已支付</el-radio-button>
          <el-radio-button value="completed">已完成</el-radio-button>
        <el-radio-button value="cancelled">已取消</el-radio-button>
        <el-radio-button value="refunded">已退款</el-radio-button>
        </el-radio-group>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="loading-container">
        <el-skeleton :rows="5" animated />
      </div>

      <div v-else class="table-wrap">
      <el-table :data="orders" style="width: 100%">
        <el-table-column prop="order_no" label="订单号" width="180" />
        <el-table-column label="套餐" min-width="160">
          <template #default="{ row }">
            {{ row.plan_name || `套餐 #${row.plan_id}` }}
          </template>
        </el-table-column>
        <el-table-column label="金额" width="120">
          <template #default="{ row }">
            <span class="price">¥{{ formatPrice(row.pay_amount) }}</span>
            <span v-if="row.discount_amount > 0" class="discount">
              -¥{{ formatPrice(row.discount_amount) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusInfo(row.status).type" size="small">
              {{ getStatusInfo(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" min-width="160">
          <template #default="{ row }">
            <div class="row-actions">
            <el-button
              v-if="row.status === 'pending'"
              type="primary"
              link
              @click="goToPay(row)"
            >
              去支付
            </el-button>
            <el-button
              v-if="row.status === 'pending'"
              type="danger"
              link
              @click="handleCancel(row)"
            >
              取消
            </el-button>
            <el-button type="primary" link @click="viewDetail(row)">
              详情
            </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      </div>

      <!-- 空状态 -->
      <el-empty v-if="!loading && orders.length === 0" description="暂无订单" />

      <!-- 分页 -->
      <div v-if="pagination.total > 0" class="pagination-container">
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

    <!-- 订单详情对话框 -->
    <el-dialog v-model="showDetailDialog" title="订单详情" :width="detailDialogWidth">
      <el-descriptions v-if="currentOrder" :column="detailColumns" border>
        <el-descriptions-item label="订单号">{{ currentOrder.order_no }}</el-descriptions-item>
        <el-descriptions-item label="套餐">
          {{ currentOrder.plan_name || `套餐 #${currentOrder.plan_id}` }}
        </el-descriptions-item>
        <el-descriptions-item label="原价">¥{{ formatPrice(currentOrder.original_amount) }}</el-descriptions-item>
        <el-descriptions-item v-if="currentOrder.discount_amount > 0" label="优惠">
          -¥{{ formatPrice(currentOrder.discount_amount) }}
        </el-descriptions-item>
        <el-descriptions-item v-if="currentOrder.balance_used > 0" label="余额抵扣">
          -¥{{ formatPrice(currentOrder.balance_used) }}
        </el-descriptions-item>
        <el-descriptions-item label="实付金额">
          <span class="price-highlight">¥{{ formatPrice(currentOrder.pay_amount) }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="getStatusInfo(currentOrder.status).type" size="small">
            {{ getStatusInfo(currentOrder.status).label }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="支付方式">{{ getMethodLabel(currentOrder.payment_method) }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ currentOrder.created_at }}</el-descriptions-item>
        <el-descriptions-item v-if="currentOrder.paid_at" label="支付时间">
          {{ currentOrder.paid_at }}
        </el-descriptions-item>
        <el-descriptions-item label="过期时间">{{ currentOrder.expired_at || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <!-- 取消确认对话框 -->
    <el-dialog v-model="showCancelDialog" title="取消订单" width="400px">
      <p>确定要取消此订单吗？取消后无法恢复。</p>
      <template #footer>
        <el-button @click="showCancelDialog = false">取消</el-button>
        <el-button type="danger" :loading="cancelling" @click="confirmCancel">
          确认取消
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useOrderStore } from '@/stores/order'
import { useViewport } from '@/composables/useViewport'

const route = useRoute()
const router = useRouter()
const orderStore = useOrderStore()
const { isMobile } = useViewport({ mobileBreakpoint: 768, tabletBreakpoint: 1280 })

// 状态
const statusFilter = ref('')
const showDetailDialog = ref(false)
const showCancelDialog = ref(false)
const cancelling = ref(false)
const orderToCancel = ref(null)

// 计算属性
const loading = computed(() => orderStore.loading)
const orders = computed(() => orderStore.orders)
const currentOrder = computed(() => orderStore.currentOrder)
const pagination = computed(() => orderStore.pagination)
const detailColumns = computed(() => (isMobile.value ? 1 : 2))
const detailDialogWidth = computed(() => (isMobile.value ? '94%' : '620px'))

const methodLabels = {
  alipay: '支付宝',
  wechat: '微信支付',
  paypal: 'PayPal',
  crypto: '加密货币',
  balance: '余额支付'
}

// 方法
const formatPrice = (price) => (price / 100).toFixed(2)
const getStatusInfo = (status) => orderStore.getStatusInfo(status)
const getMethodLabel = (method) => methodLabels[method] || method || '-'

const fetchOrders = () => {
  const params = statusFilter.value ? { status: statusFilter.value } : {}
  orderStore.fetchOrders(params)
}

const handlePaymentResultNotice = async () => {
  if (route.query.payment !== 'success') {
    return
  }

  ElMessage.success('支付成功')

  const orderNo = String(route.query.order_no || '').trim()
  if (orderNo) {
    try {
      await orderStore.fetchOrderByOrderNo(orderNo)
      showDetailDialog.value = true
    } catch (error) {
      console.error('Failed to load paid order detail:', error)
    }
  }

  router.replace({ name: 'user-orders' })
}

const handleFilterChange = () => {
  orderStore.setPage(1)
  fetchOrders()
}

const handlePageChange = (page) => {
  orderStore.setPage(page)
  fetchOrders()
}

const handleSizeChange = (size) => {
  orderStore.setPageSize(size)
  fetchOrders()
}

const goToPay = (order) => {
  router.push({ name: 'user-payment', query: { order_id: order.id } })
}

const viewDetail = async (order) => {
  await orderStore.fetchOrder(order.id)
  showDetailDialog.value = true
}

const handleCancel = (order) => {
  orderToCancel.value = order
  showCancelDialog.value = true
}

const confirmCancel = async () => {
  if (!orderToCancel.value) return
  
  cancelling.value = true
  try {
    await orderStore.cancelOrder(orderToCancel.value.id)
    ElMessage.success('订单已取消')
    showCancelDialog.value = false
    fetchOrders()
  } catch (error) {
    ElMessage.error(error || '取消失败')
  } finally {
    cancelling.value = false
  }
}

onMounted(() => {
  fetchOrders()
  handlePaymentResultNotice()
})
</script>

<style scoped>
.orders-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.orders-card {
  border-radius: 8px;
}

.orders-filter {
  margin-bottom: 20px;
}

.table-wrap {
  overflow-x: auto;
}

.loading-container {
  padding: 20px;
}

.price {
  font-weight: 500;
  color: #303133;
}

.discount {
  display: block;
  font-size: 12px;
  color: #67c23a;
}

.price-highlight {
  font-size: 16px;
  font-weight: 600;
  color: #409eff;
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  .orders-page {
    padding: 12px;
  }

  .pagination-container {
    justify-content: center;
  }
}
</style>
