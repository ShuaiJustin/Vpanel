<template>
  <div class="payment-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">订单支付</h1>
      <p class="page-subtitle">请选择支付方式完成支付</p>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <el-skeleton :rows="5" animated />
    </div>

    <template v-else-if="order">
      <!-- 订单信息 -->
      <el-card shadow="never" class="order-card">
        <template #header>
          <span>订单信息</span>
        </template>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="订单号">{{ order.order_no }}</el-descriptions-item>
          <el-descriptions-item label="套餐">{{ order.plan_name }}</el-descriptions-item>
          <el-descriptions-item label="原价">¥{{ formatPrice(order.original_amount) }}</el-descriptions-item>
          <el-descriptions-item label="时长">{{ order.duration }}天</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- 优惠券 -->
      <el-card shadow="never" class="coupon-card">
        <template #header>
          <span>优惠券</span>
        </template>
        <div class="coupon-input">
          <el-input
            v-model="couponCode"
            placeholder="请输入优惠券码"
            :disabled="couponApplied"
          >
            <template #append>
              <el-button
                v-if="!couponApplied"
                :loading="validatingCoupon"
                @click="applyCoupon"
              >
                使用
              </el-button>
              <el-button v-else type="danger" @click="removeCoupon">
                移除
              </el-button>
            </template>
          </el-input>
          <div v-if="couponInfo" class="coupon-info">
            <el-tag type="success" size="small">
              {{ couponInfo.type === 'fixed' ? `减 ¥${formatPrice(couponInfo.value)}` : `${couponInfo.value}% 折扣` }}
            </el-tag>
            <span class="coupon-discount">-¥{{ formatPrice(discountAmount) }}</span>
          </div>
        </div>
      </el-card>

      <!-- 余额抵扣 -->
      <el-card v-if="userBalance > 0" shadow="never" class="balance-card">
        <template #header>
          <span>余额抵扣</span>
        </template>
        <div class="balance-option">
          <el-checkbox v-model="useBalance">
            使用余额抵扣（可用余额：¥{{ formatPrice(userBalance) }}）
          </el-checkbox>
          <span v-if="useBalance" class="balance-deduct">
            -¥{{ formatPrice(balanceDeduct) }}
          </span>
        </div>
      </el-card>

      <!-- 支付方式 -->
      <el-card shadow="never" class="payment-card">
        <template #header>
          <span>支付方式</span>
        </template>
        <div v-if="paymentMethods.length > 0" class="payment-methods">
          <div
            v-for="method in paymentMethods"
            :key="method.value"
            class="payment-method"
            :class="{ 'payment-method--active': selectedMethod === method.value }"
            @click="selectedMethod = method.value"
          >
            <el-icon :size="24"><component :is="method.icon" /></el-icon>
            <span>{{ method.label }}</span>
          </div>
        </div>
        <el-empty v-else description="当前未配置可用的在线支付方式" />
      </el-card>

      <!-- 支付金额 -->
      <el-card shadow="never" class="summary-card">
        <div class="summary-row">
          <span>商品金额</span>
          <span>¥{{ formatPrice(order.original_amount) }}</span>
        </div>
        <div v-if="discountAmount > 0" class="summary-row discount">
          <span>优惠券</span>
          <span>-¥{{ formatPrice(discountAmount) }}</span>
        </div>
        <div v-if="useBalance && balanceDeduct > 0" class="summary-row discount">
          <span>余额抵扣</span>
          <span>-¥{{ formatPrice(balanceDeduct) }}</span>
        </div>
        <div class="summary-row total">
          <span>应付金额</span>
          <span class="total-amount">¥{{ formatPrice(finalAmount) }}</span>
        </div>
        <el-button
          type="primary"
          size="large"
          class="pay-button"
          :loading="paying"
          :disabled="finalAmount > 0 && !selectedMethod"
          @click="handlePay"
        >
          {{ finalAmount > 0 ? '立即支付' : '确认订单' }}
        </el-button>
      </el-card>
    </template>

    <!-- 支付二维码对话框 -->
    <el-dialog
      v-model="showQRDialog"
      title="扫码支付"
      width="400px"
      :close-on-click-modal="false"
    >
      <div class="qr-container">
        <div class="qr-code">
          <canvas ref="qrcodeCanvas"></canvas>
        </div>
        <p class="qr-tip">请使用{{ selectedMethodLabel }}扫描二维码完成支付</p>
        <p class="qr-amount">支付金额：<span>¥{{ formatPrice(finalAmount) }}</span></p>
        <el-progress
          v-if="polling"
          :percentage="pollProgress"
          :stroke-width="4"
          :show-text="false"
        />
        <p v-if="polling" class="poll-tip">正在等待支付结果...</p>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { CreditCard, Wallet } from '@element-plus/icons-vue'
import { useOrderStore } from '@/stores/order'
import { useBalanceStore } from '@/stores/balance'
import { paymentsApi } from '@/api'
import QRCode from 'qrcode'

const route = useRoute()
const router = useRouter()
const orderStore = useOrderStore()
const balanceStore = useBalanceStore()

// 引用
const qrcodeCanvas = ref(null)

// 状态
const loading = ref(true)
const couponCode = ref('')
const couponApplied = ref(false)
const validatingCoupon = ref(false)
const useBalance = ref(false)
const selectedMethod = ref('')
const paying = ref(false)
const showQRDialog = ref(false)
const polling = ref(false)
const pollProgress = ref(0)

const paymentMethods = ref([])

const paymentMethodMeta = {
  alipay: { label: '支付宝', icon: CreditCard },
  wechat: { label: '微信支付', icon: Wallet }
}

// 计算属性
const order = computed(() => orderStore.currentOrder)
const couponInfo = computed(() => orderStore.couponInfo)
const userBalance = computed(() => balanceStore.balance)

const discountAmount = computed(() => {
  if (!couponInfo.value || !order.value) return 0
  if (couponInfo.value.type === 'fixed') {
    return Math.min(couponInfo.value.value, order.value.original_amount)
  }
  return Math.round(order.value.original_amount * couponInfo.value.value / 100)
})

const balanceDeduct = computed(() => {
  if (!useBalance.value || !order.value) return 0
  const remaining = order.value.original_amount - discountAmount.value
  return Math.min(userBalance.value, remaining)
})

const finalAmount = computed(() => {
  if (!order.value) return 0
  return Math.max(0, order.value.original_amount - discountAmount.value - balanceDeduct.value)
})

const selectedMethodLabel = computed(() => {
  const method = paymentMethods.value.find(m => m.value === selectedMethod.value)
  return method?.label || ''
})

// 方法
const formatPrice = (price) => (price / 100).toFixed(2)

const fetchPaymentMethods = async () => {
  try {
    const response = await paymentsApi.getMethods()
    const methods = (response.methods || [])
      .filter(method => method !== 'balance')
      .map(method => ({
        value: method,
        label: paymentMethodMeta[method]?.label || method,
        icon: paymentMethodMeta[method]?.icon || CreditCard
      }))

    paymentMethods.value = methods

    if (!methods.some(method => method.value === selectedMethod.value)) {
      selectedMethod.value = methods[0]?.value || ''
    }
  } catch (error) {
    paymentMethods.value = []
    selectedMethod.value = ''
    console.error('Failed to fetch payment methods:', error)
  }
}

const initOrder = async () => {
  loading.value = true
  try {
    const planId = route.query.plan_id
    const orderNo = route.query.order_no

    if (orderNo) {
      // 继续支付已有订单
      await orderStore.fetchOrder(orderNo)
    } else if (planId) {
      // 创建新订单
      await orderStore.createOrder({ plan_id: parseInt(planId) })
    } else {
      ElMessage.error('缺少订单参数')
      router.push({ name: 'user-plans' })
      return
    }

    // 获取用户余额
    await Promise.all([
      balanceStore.fetchBalance(),
      fetchPaymentMethods()
    ])
  } catch (error) {
    ElMessage.error(error || '加载订单失败')
    router.push({ name: 'user-plans' })
  } finally {
    loading.value = false
  }
}

const applyCoupon = async () => {
  if (!couponCode.value.trim()) {
    ElMessage.warning('请输入优惠券码')
    return
  }

  validatingCoupon.value = true
  try {
    await orderStore.validateCoupon(
      couponCode.value,
      order.value.plan_id,
      order.value.original_amount
    )
    couponApplied.value = true
    ElMessage.success('优惠券已应用')
  } catch (error) {
    ElMessage.error(error || '优惠券无效')
  } finally {
    validatingCoupon.value = false
  }
}

const removeCoupon = () => {
  couponCode.value = ''
  couponApplied.value = false
  orderStore.couponInfo = null
}

const generateQRCode = async (url) => {
  await nextTick()
  if (qrcodeCanvas.value) {
    try {
      await QRCode.toCanvas(qrcodeCanvas.value, url, {
        width: 200,
        margin: 2
      })
    } catch (error) {
      console.error('Failed to generate QR code:', error)
    }
  }
}

const handlePay = async () => {
  if (finalAmount.value > 0 && !selectedMethod.value) {
    ElMessage.warning('请选择支付方式')
    return
  }

  paying.value = true
  try {
    const paymentData = await orderStore.createPayment(
      order.value.order_no,
      finalAmount.value > 0 ? selectedMethod.value : 'balance'
    )

    const qrPayload = paymentData.payment?.qrcode_data || paymentData.payment?.payment_url

    if (qrPayload) {
      // 显示支付二维码
      showQRDialog.value = true
      await generateQRCode(qrPayload)
      startPolling()
    } else {
      // 余额支付成功
      ElMessage.success('支付成功')
      router.push({ name: 'user-orders' })
    }
  } catch (error) {
    ElMessage.error(error || '创建支付失败')
  } finally {
    paying.value = false
  }
}

const startPolling = async () => {
  polling.value = true
  pollProgress.value = 0

  const interval = setInterval(() => {
    pollProgress.value = Math.min(pollProgress.value + 1, 100)
  }, 3000)

  try {
    const result = await orderStore.pollPaymentStatus(order.value.order_no)
    if (result.status === 'paid') {
      ElMessage.success('支付成功')
      router.push({ name: 'user-orders' })
    }
  } catch (error) {
    ElMessage.error('支付超时，请查看订单状态')
  } finally {
    clearInterval(interval)
    polling.value = false
    showQRDialog.value = false
  }
}

onMounted(() => {
  initOrder()
})
</script>

<style scoped>
.payment-page {
  padding: 20px;
  max-width: 600px;
  margin: 0 auto;
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

.loading-container {
  padding: 40px;
}

.order-card,
.coupon-card,
.balance-card,
.payment-card,
.summary-card {
  margin-bottom: 16px;
  border-radius: 8px;
}

.coupon-input {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.coupon-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.coupon-discount {
  color: #67c23a;
  font-weight: 500;
}

.balance-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.balance-deduct {
  color: #67c23a;
  font-weight: 500;
}

.payment-methods {
  display: flex;
  gap: 16px;
}

.payment-method {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 20px;
  border: 2px solid #ebeef5;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
}

.payment-method:hover {
  border-color: #409eff;
}

.payment-method--active {
  border-color: #409eff;
  background: #ecf5ff;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid #f5f5f5;
}

.summary-row.discount {
  color: #67c23a;
}

.summary-row.total {
  border-bottom: none;
  font-weight: 500;
}

.total-amount {
  font-size: 24px;
  color: #f56c6c;
  font-weight: 600;
}

.pay-button {
  width: 100%;
  margin-top: 16px;
}

.qr-container {
  text-align: center;
}

.qr-code {
  display: inline-block;
  padding: 16px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.qr-tip {
  margin-top: 16px;
  color: #606266;
}

.qr-amount span {
  font-size: 20px;
  color: #f56c6c;
  font-weight: 600;
}

.poll-tip {
  margin-top: 12px;
  color: #909399;
  font-size: 13px;
}
</style>
