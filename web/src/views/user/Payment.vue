<template>
  <div class="payment-page">
    <div class="page-header">
      <h1 class="page-title">
        订单支付
      </h1>
      <p class="page-subtitle">
        订单金额以已创建订单为准，在这里选择支付方式完成付款。
      </p>
    </div>

    <div
      v-if="loading"
      class="loading-container"
    >
      <el-skeleton
        :rows="5"
        animated
      />
    </div>

    <template v-else-if="order">
      <el-card
        shadow="never"
        class="order-card"
      >
        <template #header>
          <span>订单信息</span>
        </template>
        <el-descriptions
          :column="descriptionColumns"
          border
        >
          <el-descriptions-item label="订单号">
            {{ order.order_no }}
          </el-descriptions-item>
          <el-descriptions-item label="订单状态">
            <el-tag
              :type="getStatusInfo(order.status).type"
              size="small"
            >
              {{ getStatusInfo(order.status).label }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="套餐">
            {{ order.plan_name || `套餐 #${order.plan_id}` }}
          </el-descriptions-item>
          <el-descriptions-item label="原价">
            ¥{{ formatPrice(order.original_amount) }}
          </el-descriptions-item>
          <el-descriptions-item
            v-if="order.discount_amount > 0"
            label="优惠"
          >
            -¥{{ formatPrice(order.discount_amount) }}
          </el-descriptions-item>
          <el-descriptions-item
            v-if="order.balance_used > 0"
            label="余额抵扣"
          >
            -¥{{ formatPrice(order.balance_used) }}
          </el-descriptions-item>
          <el-descriptions-item label="应付金额">
            <span class="price-highlight">¥{{ formatPrice(order.pay_amount) }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">
            {{ order.created_at }}
          </el-descriptions-item>
          <el-descriptions-item label="过期时间">
            {{ order.expired_at || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="支付方式">
            {{ getMethodLabel(order.payment_method) || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="支付流水号">
            {{ order.payment_no || '-' }}
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <el-alert
        v-if="!canPay"
        type="info"
        show-icon
        :closable="false"
        class="status-alert"
        :title="statusAlertTitle"
      />

      <template v-else>
        <el-card
          shadow="never"
          class="payment-card"
        >
          <template #header>
            <span>支付方式</span>
          </template>
          <div
            v-if="paymentMethods.length > 0"
            class="payment-methods"
          >
            <button
              v-for="method in paymentMethods"
              :key="method.value"
              type="button"
              class="payment-method"
              :class="{ 'payment-method--active': selectedMethod === method.value }"
              @click="selectedMethod = method.value"
            >
              <el-icon :size="22">
                <component :is="method.icon" />
              </el-icon>
              <span>{{ method.label }}</span>
            </button>
          </div>
          <div
            v-if="showBalanceOverview"
            class="balance-overview"
            :class="{ 'balance-overview--warning': balanceInfoLoaded && !balanceInfoError && !isBalanceEnough }"
          >
            <div class="balance-overview__content">
              <span class="balance-overview__label">当前可用余额</span>
              <strong class="balance-overview__amount">¥{{ balanceStore.formattedBalance }}</strong>
              <p class="balance-overview__hint">
                {{ balanceOverviewHint }}
              </p>
            </div>
            <div class="balance-overview__actions">
              <el-button
                link
                @click="goToBalance"
              >
                查看余额
              </el-button>
              <el-button
                v-if="balanceInfoLoaded && (!isBalanceEnough || balanceInfoError)"
                type="primary"
                plain
                @click="goToRecharge"
              >
                去充值
              </el-button>
            </div>
          </div>
          <el-empty
            v-if="paymentMethods.length === 0"
            description="当前没有可用的支付方式"
          />
        </el-card>

        <el-card
          shadow="never"
          class="summary-card"
        >
          <div
            v-if="showRechargePrompt"
            class="payment-hint"
          >
            <el-alert
              type="warning"
              show-icon
              :closable="false"
              :title="paymentHintTitle"
              description="充值完成后，可返回当前订单继续完成余额支付。"
            />
            <el-button
              type="primary"
              plain
              class="payment-hint__action"
              @click="goToRecharge"
            >
              去充值
            </el-button>
          </div>
          <div class="summary-row">
            <span>订单实付</span>
            <span class="summary-value">¥{{ formatPrice(order.pay_amount) }}</span>
          </div>
          <div class="summary-row summary-row--muted">
            <span>说明</span>
            <span>如需修改优惠券或金额，请返回重新创建订单</span>
          </div>
          <el-button
            type="primary"
            size="large"
            class="pay-button"
            :loading="paying"
            :disabled="!selectedMethod || balancePaymentDisabled"
            @click="handlePay"
          >
            {{ selectedMethod === 'balance' ? '确认余额支付' : '立即支付' }}
          </el-button>
        </el-card>
      </template>
    </template>

    <el-empty
      v-else
      description="订单不存在或已失效"
    />

    <el-dialog
      v-model="showQRDialog"
      title="扫码支付"
      :width="qrDialogWidth"
      :close-on-click-modal="false"
    >
      <div class="qr-container">
        <div class="qr-code">
          <canvas ref="qrcodeCanvas" />
        </div>
        <p class="qr-tip">
          请使用 {{ selectedMethodLabel }} 扫描二维码完成支付
        </p>
        <p class="qr-amount">
          支付金额：<span>¥{{ formatPrice(order?.pay_amount || 0) }}</span>
        </p>
        <el-button
          v-if="paymentLink"
          type="primary"
          plain
          class="open-payment-button"
          @click="window.open(paymentLink, '_blank', 'noopener,noreferrer')"
        >
          打开支付页面
        </el-button>
        <el-progress
          v-if="polling"
          :percentage="pollProgress"
          :stroke-width="4"
          :show-text="false"
        />
        <p
          v-if="polling"
          class="poll-tip"
        >
          正在等待支付结果...
        </p>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { CreditCard, Wallet, Coin } from '@element-plus/icons-vue'
import QRCode from 'qrcode'
import { useOrderStore } from '@/stores/order'
import { useBalanceStore } from '@/stores/balance'
import { paymentsApi } from '@/api'
import { useViewport } from '@/composables/useViewport'
import { extractErrorMessage, getErrorCode } from '@/utils/entitlement'

const route = useRoute()
const router = useRouter()
const orderStore = useOrderStore()
const balanceStore = useBalanceStore()
const { isMobile } = useViewport()

const qrcodeCanvas = ref(null)

const loading = ref(true)
const selectedMethod = ref('')
const paying = ref(false)
const showQRDialog = ref(false)
const polling = ref(false)
const pollProgress = ref(0)
const paymentMethods = ref([])
const paymentLink = ref('')
const insufficientBalanceMessage = ref('')
const balanceInfoLoaded = ref(false)
const balanceInfoError = ref('')

const paymentMethodMeta = {
  alipay: { label: '支付宝', icon: CreditCard },
  wechat: { label: '微信支付', icon: Wallet },
  balance: { label: '余额支付', icon: Coin }
}

let pollProgressTimer = null

const order = computed(() => orderStore.currentOrder)
const descriptionColumns = computed(() => (isMobile.value ? 1 : 2))
const qrDialogWidth = computed(() => (isMobile.value ? '92%' : '420px'))
const canPay = computed(() => order.value?.status === 'pending')
const statusAlertTitle = computed(() => {
  const status = order.value?.status
  if (status === 'paid' || status === 'completed') {
    return '该订单已支付完成，无需重复付款。'
  }
  if (status === 'cancelled') {
    return '该订单已取消，不能继续支付。'
  }
  if (status === 'refunded') {
    return '该订单已退款，不能继续支付。'
  }
  return '当前订单无法支付。'
})
const selectedMethodLabel = computed(() => {
  const method = paymentMethods.value.find(item => item.value === selectedMethod.value)
  return method?.label || ''
})
const hasBalanceMethod = computed(() => paymentMethods.value.some(item => item.value === 'balance'))
const showBalanceOverview = computed(() => paymentMethods.value.length > 0 && hasBalanceMethod.value)
const isBalanceEnough = computed(() => Number(balanceStore.balance || 0) >= Number(order.value?.pay_amount || 0))
const balancePaymentDisabled = computed(() => {
  return selectedMethod.value === 'balance' && balanceInfoLoaded.value && !balanceInfoError.value && !isBalanceEnough.value
})
const balanceOverviewHint = computed(() => {
  if (balanceInfoError.value) {
    return balanceInfoError.value
  }

  if (!balanceInfoLoaded.value) {
    return '正在加载余额信息...'
  }

  if (isBalanceEnough.value) {
    return '当前余额可直接用于支付本订单。'
  }

  const shortage = Math.max(Number(order.value?.pay_amount || 0) - Number(balanceStore.balance || 0), 0)
  return `当前余额不足，还差 ¥${formatPrice(shortage)}，请先充值。`
})
const paymentHintTitle = computed(() => insufficientBalanceMessage.value || balanceOverviewHint.value)
const showRechargePrompt = computed(() => {
  return selectedMethod.value === 'balance' && balanceInfoLoaded.value && (!isBalanceEnough.value || Boolean(insufficientBalanceMessage.value) || Boolean(balanceInfoError.value))
})

const formatPrice = price => (Number(price || 0) / 100).toFixed(2)
const getStatusInfo = status => orderStore.getStatusInfo(status)
const getMethodLabel = method => paymentMethodMeta[method]?.label || method || '-'

const clearPaymentHint = () => {
  insufficientBalanceMessage.value = ''
}

const goToBalance = () => {
  router.push({
    name: 'user-balance',
    query: {
      redirect: route.fullPath
    }
  }).catch(error => {
    console.error('跳转到余额页面失败:', error)
  })
}

const isInsufficientBalanceError = error => {
  const code = getErrorCode(error)
  const message = extractErrorMessage(error)

  return code === 'INSUFFICIENT_BALANCE' || message.includes('余额不足')
}

const goToRecharge = () => {
  router.push({
    name: 'user-balance',
    query: {
      action: 'recharge',
      redirect: route.fullPath
    }
  }).catch(error => {
    console.error('跳转到余额充值页面失败:', error)
  })
}

const fetchBalanceInfo = async () => {
  if (!hasBalanceMethod.value) {
    balanceInfoLoaded.value = false
    balanceInfoError.value = ''
    return
  }

  balanceInfoLoaded.value = false
  balanceInfoError.value = ''

  try {
    await balanceStore.fetchBalance()
  } catch (error) {
    console.warn('加载余额信息失败:', error)
    balanceInfoError.value = '余额信息暂时无法加载，请点击“查看余额”确认。'
  } finally {
    balanceInfoLoaded.value = true
  }
}

const fetchPaymentMethods = async () => {
  try {
    const response = await paymentsApi.getMethods()
    const methods = (response.methods || []).map(method => ({
      value: method,
      label: paymentMethodMeta[method]?.label || method,
      icon: paymentMethodMeta[method]?.icon || CreditCard
    }))

    paymentMethods.value = methods

    if (!methods.some(item => item.value === selectedMethod.value)) {
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
  paymentLink.value = ''
  clearPaymentHint()
  try {
    const planId = Number(route.query.plan_id)
    const orderId = Number(route.query.order_id)
    const orderNo = String(route.query.order_no || '').trim()

    if (orderId) {
      await orderStore.fetchOrder(orderId)
    } else if (planId) {
      const response = await orderStore.createOrder({ plan_id: planId })
      const createdOrderId = response.order?.id
      if (createdOrderId) {
        router.replace({ name: 'user-payment', query: { order_id: createdOrderId } })
      }
    } else if (orderNo) {
      await orderStore.fetchOrderByOrderNo(orderNo)
    } else {
      ElMessage.error('缺少订单参数')
      router.replace({ name: 'user-plans' })
      return
    }

    await fetchPaymentMethods()
    await fetchBalanceInfo()
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载订单失败')
    router.replace({ name: 'user-plans' })
  } finally {
    loading.value = false
  }
}

const generateQRCode = async payload => {
  await nextTick()
  if (!qrcodeCanvas.value) {
    return
  }

  try {
    await QRCode.toCanvas(qrcodeCanvas.value, payload, {
      width: isMobile.value ? 180 : 220,
      margin: 2
    })
  } catch (error) {
    console.error('Failed to generate QR code:', error)
  }
}

const clearPollingTimer = () => {
  if (pollProgressTimer) {
    clearInterval(pollProgressTimer)
    pollProgressTimer = null
  }
}

const startPolling = async () => {
  polling.value = true
  pollProgress.value = 0
  clearPollingTimer()

  pollProgressTimer = setInterval(() => {
    pollProgress.value = Math.min(pollProgress.value + 3, 97)
  }, 3000)

  try {
    const result = await orderStore.pollPaymentStatus(order.value.order_no)
    if (result.status === 'paid') {
      pollProgress.value = 100
      ElMessage.success('支付成功')
      router.replace({
        name: 'user-orders',
        query: { payment: 'success', order_no: order.value.order_no }
      })
    }
  } catch (error) {
    ElMessage.error('支付超时，请返回订单页查看状态')
  } finally {
    clearPollingTimer()
    polling.value = false
    showQRDialog.value = false
  }
}

const handlePay = async () => {
  if (!selectedMethod.value) {
    ElMessage.warning('请选择支付方式')
    return
  }

  clearPaymentHint()
  paying.value = true
  try {
    const paymentData = await orderStore.createPayment(order.value.order_no, selectedMethod.value)
    paymentLink.value = paymentData.payment?.payment_url || ''
    const qrPayload = paymentData.payment?.qrcode_data || paymentData.payment?.payment_url

    if (qrPayload) {
      showQRDialog.value = true
      await generateQRCode(qrPayload)
      await startPolling()
      return
    }

    ElMessage.success('支付成功')
    router.replace({
      name: 'user-orders',
      query: { payment: 'success', order_no: order.value.order_no }
    })
  } catch (error) {
    const message = extractErrorMessage(error) || '创建支付失败'

    if (selectedMethod.value === 'balance' && isInsufficientBalanceError(error)) {
      insufficientBalanceMessage.value = message
    }

    ElMessage.error(message)
  } finally {
    paying.value = false
  }
}

watch(selectedMethod, () => {
  clearPaymentHint()
})

onMounted(() => {
  initOrder()
})

onUnmounted(() => {
  clearPollingTimer()
})
</script>

<style scoped>
.payment-page {
  padding: clamp(12px, 2vw, 20px);
  max-width: 720px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.loading-container {
  padding: 40px 0;
}

.order-card,
.payment-card,
.summary-card,
.status-alert {
  margin-bottom: 16px;
}

.price-highlight {
  font-size: 16px;
  font-weight: 700;
  color: #2563eb;
}

.payment-methods {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 14px;
}

.payment-method {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 18px 14px;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  background: var(--color-bg-card);
  cursor: pointer;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.payment-method:hover {
  border-color: var(--color-primary-light);
  transform: translateY(-1px);
}

.payment-method--active {
  border-color: #2563eb;
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.12);
  background: rgba(64, 158, 255, 0.12);
}

.balance-overview {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 16px;
  padding: 14px 16px;
  border-radius: 14px;
  background: rgba(37, 99, 235, 0.06);
  border: 1px solid rgba(37, 99, 235, 0.14);
}

.balance-overview--warning {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(245, 158, 11, 0.24);
}

.balance-overview__content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.balance-overview__label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.balance-overview__amount {
  font-size: 20px;
  color: var(--color-text-primary);
}

.balance-overview__hint {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.balance-overview__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.payment-hint {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
}

.payment-hint__action {
  align-self: flex-start;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--color-border);
}

.summary-row:last-of-type {
  border-bottom: none;
}

.summary-row--muted {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.summary-value {
  font-weight: 700;
  color: var(--color-text-primary);
}

.pay-button {
  width: 100%;
  margin-top: 18px;
}

.qr-container {
  text-align: center;
}

.qr-code {
  display: inline-flex;
  padding: 16px;
  background: var(--color-bg-card);
  border-radius: 16px;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
}

.qr-tip {
  margin-top: 16px;
  color: var(--color-text-regular);
}

.qr-amount span {
  font-size: 20px;
  color: #ef4444;
  font-weight: 700;
}

.open-payment-button {
  margin-top: 12px;
}

.poll-tip {
  margin-top: 12px;
  color: var(--color-text-secondary);
}

@media (max-width: 768px) {
  .page-title {
    font-size: 22px;
  }

  .payment-methods {
    grid-template-columns: 1fr;
  }

  .balance-overview {
    flex-direction: column;
    align-items: flex-start;
  }

  .balance-overview__actions,
  .payment-hint__action {
    width: 100%;
  }
}
</style>
