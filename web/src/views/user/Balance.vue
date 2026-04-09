<template>
  <div class="balance-page">
    <div class="page-header">
      <h1 class="page-title">
        我的余额
      </h1>
      <p class="page-subtitle">
        查看余额和交易记录
      </p>
    </div>

    <el-card
      shadow="never"
      class="balance-card"
    >
      <div class="balance-info">
        <div class="balance-amount">
          <span class="label">可用余额</span>
          <span class="amount">¥{{ formattedBalance }}</span>
        </div>
        <div class="balance-actions">
          <el-button
            type="primary"
            :loading="loadingRechargeMethods"
            @click="openRechargeDialog"
          >
            <el-icon><Plus /></el-icon>
            充值
          </el-button>
          <span
            v-if="!loadingRechargeMethods && !hasRechargeMethods"
            class="balance-action-tip"
          >
            当前未开启在线充值
          </span>
        </div>
      </div>
    </el-card>

    <el-card
      shadow="never"
      class="transactions-card"
    >
      <template #header>
        <div class="card-header">
          <span>交易记录</span>
          <el-select
            v-model="typeFilter"
            placeholder="全部类型"
            clearable
            @change="handleFilterChange"
          >
            <el-option label="充值" value="recharge" />
            <el-option label="消费" value="purchase" />
            <el-option label="退款" value="refund" />
            <el-option label="佣金" value="commission" />
            <el-option label="调整" value="adjustment" />
          </el-select>
        </div>
      </template>

      <div
        v-if="loading"
        class="loading-container"
      >
        <el-skeleton :rows="5" animated />
      </div>

      <div
        v-else
        class="transactions-list"
      >
        <div
          v-for="tx in transactions"
          :key="tx.id"
          class="transaction-item"
        >
          <div
            class="tx-icon"
            :class="`tx-icon--${tx.type}`"
          >
            <el-icon><component :is="getTypeInfo(tx.type).icon" /></el-icon>
          </div>
          <div class="tx-info">
            <div class="tx-title">
              {{ getTypeInfo(tx.type).label }}
            </div>
            <div class="tx-desc">
              {{ tx.description || '-' }}
            </div>
            <div class="tx-time">
              {{ tx.created_at }}
            </div>
          </div>
          <div
            class="tx-amount"
            :class="{ 'tx-amount--positive': tx.amount > 0 }"
          >
            {{ formatAmount(tx.amount) }}
          </div>
        </div>
      </div>

      <el-empty
        v-if="!loading && transactions.length === 0"
        description="暂无交易记录"
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

    <el-dialog
      v-model="showRechargeDialog"
      title="余额充值"
      width="460px"
    >
      <template v-if="hasRechargeMethods">
        <el-form
          :model="rechargeForm"
          label-width="80px"
        >
          <el-form-item label="充值金额">
            <div class="amount-options">
              <div
                v-for="amount in rechargeAmounts"
                :key="amount"
                class="amount-option"
                :class="{ 'amount-option--active': rechargeForm.amount === amount }"
                @click="rechargeForm.amount = amount"
              >
                ¥{{ amount / 100 }}
              </div>
            </div>
            <el-input-number
              v-model="rechargeForm.customAmount"
              :min="1"
              :max="10000"
              placeholder="自定义金额"
              style="width: 100%; margin-top: 12px;"
              @change="handleCustomAmountChange"
            />
          </el-form-item>
          <el-form-item label="支付方式">
            <el-radio-group v-model="rechargeForm.method">
              <el-radio
                v-for="method in rechargeMethods"
                :key="method.value"
                :value="method.value"
              >
                {{ method.label }}
              </el-radio>
            </el-radio-group>
          </el-form-item>
        </el-form>
      </template>
      <template v-else>
        <el-alert
          type="warning"
          show-icon
          :closable="false"
          title="当前未开启在线充值"
          description="请联系管理员开通支付网关，或使用礼品卡为账户充值。"
        />
      </template>
      <template #footer>
        <el-button @click="showRechargeDialog = false">
          取消
        </el-button>
        <el-button
          v-if="!hasRechargeMethods"
          type="primary"
          plain
          @click="goToGiftCard"
        >
          去礼品卡充值
        </el-button>
        <el-button
          v-else
          type="primary"
          :loading="recharging"
          :disabled="!rechargeForm.method"
          @click="handleRecharge"
        >
          充值 ¥{{ (rechargeForm.amount / 100).toFixed(2) }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showQRDialog"
      title="扫码充值"
      width="360px"
      :close-on-click-modal="false"
    >
      <div class="qr-container">
        <div class="qr-code">
          <canvas ref="qrcodeCanvas" />
        </div>
        <p class="qr-title">
          请使用{{ currentRechargeMethodLabel }}扫码完成充值
        </p>
        <p class="qr-amount">
          充值金额：¥{{ (rechargeForm.amount / 100).toFixed(2) }}
        </p>
        <el-progress
          v-if="polling"
          :percentage="pollProgress"
          :stroke-width="4"
          :show-text="false"
        />
        <p
          v-if="polling"
          class="qr-tip"
        >
          正在等待充值结果...
        </p>
      </div>
      <template #footer>
        <el-button @click="showQRDialog = false">
          关闭
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, Coin, ShoppingCart, RefreshRight, Money, Edit } from '@element-plus/icons-vue'
import QRCode from 'qrcode'
import { balanceApi, paymentsApi } from '@/api'
import { useBalanceStore } from '@/stores/balance'
import { extractErrorMessage } from '@/utils/entitlement'

const balanceStore = useBalanceStore()
const route = useRoute()
const router = useRouter()
const qrcodeCanvas = ref(null)

const typeFilter = ref('')
const showRechargeDialog = ref(false)
const showQRDialog = ref(false)
const recharging = ref(false)
const loadingRechargeMethods = ref(false)
const polling = ref(false)
const pollProgress = ref(0)
const rechargeMethods = ref([])
const rechargeOrderNo = ref('')
const rechargeAmounts = [1000, 5000, 10000, 20000, 50000]

const rechargeForm = reactive({
  amount: 5000,
  customAmount: null,
  method: ''
})

const loading = computed(() => balanceStore.loading)
const formattedBalance = computed(() => balanceStore.formattedBalance)
const transactions = computed(() => balanceStore.transactions)
const pagination = computed(() => balanceStore.pagination)
const hasRechargeMethods = computed(() => rechargeMethods.value.length > 0)
const currentRechargeMethodLabel = computed(() => {
  return rechargeMethods.value.find(method => method.value === rechargeForm.method)?.label || '微信'
})

const typeIcons = {
  recharge: Plus,
  purchase: ShoppingCart,
  refund: RefreshRight,
  commission: Coin,
  adjustment: Edit
}

const rechargeMethodMeta = {
  alipay: '支付宝',
  wechat: '微信支付'
}

let pollTimer = null
let pollProgressTimer = null

const getTypeInfo = (type) => {
  const info = balanceStore.getTypeInfo(type)
  return { ...info, icon: typeIcons[type] || Money }
}

const formatAmount = (amount) => {
  const yuan = (Math.abs(amount) / 100).toFixed(2)
  return amount >= 0 ? `+¥${yuan}` : `-¥${yuan}`
}

const fetchData = async () => {
  try {
    await Promise.all([
      balanceStore.fetchBalance(),
      balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
    ])
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载余额信息失败')
  }
}

const fetchRechargeMethods = async () => {
  loadingRechargeMethods.value = true
  try {
    const response = await paymentsApi.getMethods()
    rechargeMethods.value = (response.methods || [])
      .filter(method => method !== 'balance')
      .map(method => ({
        value: method,
        label: rechargeMethodMeta[method] || method
      }))

    if (!rechargeMethods.value.some(method => method.value === rechargeForm.method)) {
      rechargeForm.method = rechargeMethods.value[0]?.value || ''
    }
  } catch (error) {
    rechargeMethods.value = []
    rechargeForm.method = ''
    console.warn('加载充值方式失败:', error)
  } finally {
    loadingRechargeMethods.value = false
  }
}

const handleFilterChange = async () => {
  balanceStore.setPage(1)
  try {
    await balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载交易记录失败')
  }
}

const handlePageChange = async (page) => {
  balanceStore.setPage(page)
  try {
    await balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载交易记录失败')
  }
}

const handleSizeChange = async (size) => {
  balanceStore.setPageSize(size)
  try {
    await balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载交易记录失败')
  }
}

const handleCustomAmountChange = (value) => {
  rechargeForm.amount = Number(value || 0) * 100
}

const openRechargeDialog = () => {
  showRechargeDialog.value = true
}

const openRechargeDialogIfNeeded = () => {
  if (route.query.action === 'recharge') {
    showRechargeDialog.value = true
  }
}

const getSafeRedirectTarget = () => {
  const redirect = String(route.query.redirect || '').trim()
  return redirect.startsWith('/user/') ? redirect : ''
}

const returnToRedirectTarget = (message) => {
  const redirectTarget = getSafeRedirectTarget()
  if (!redirectTarget) {
    return false
  }

  if (message) {
    ElMessage.success(message)
  }

  router.replace(redirectTarget).catch(error => {
    console.error('返回原页面失败:', error)
  })
  return true
}

const goToGiftCard = () => {
  const redirectTarget = getSafeRedirectTarget()
  router.push({
    name: 'user-gift-card',
    query: redirectTarget ? { redirect: redirectTarget } : undefined
  }).catch(error => {
    console.error('跳转礼品卡页面失败:', error)
  })
}

const clearPollingState = () => {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
  if (pollProgressTimer) {
    clearInterval(pollProgressTimer)
    pollProgressTimer = null
  }
}

const generateQRCode = async (payload) => {
  await nextTick()
  if (!qrcodeCanvas.value || !payload) {
    return
  }

  try {
    await QRCode.toCanvas(qrcodeCanvas.value, payload, {
      width: 220,
      margin: 2
    })
  } catch (error) {
    console.error('生成充值二维码失败:', error)
  }
}

const pollRechargeStatus = async (orderNo, attempts = 100) => {
  for (let index = 0; index < attempts; index += 1) {
    const result = await balanceApi.getRechargeStatus(orderNo)

    if (result.status === 'paid') {
      return result
    }

    if (result.status === 'cancelled' || result.status === 'expired') {
      throw new Error('充值订单已失效，请重新发起充值。')
    }

    await new Promise(resolve => {
      pollTimer = setTimeout(resolve, 3000)
    })
  }

  throw new Error('充值结果确认超时，请稍后刷新余额页面查看。')
}

const startPolling = async (orderNo) => {
  polling.value = true
  pollProgress.value = 0
  clearPollingState()

  pollProgressTimer = setInterval(() => {
    pollProgress.value = Math.min(pollProgress.value + 3, 97)
  }, 3000)

  try {
    await pollRechargeStatus(orderNo)
    pollProgress.value = 100
    showQRDialog.value = false
    await fetchData()

    const redirected = returnToRedirectTarget('充值成功，正在返回原订单继续支付')
    if (!redirected) {
      ElMessage.success('充值成功')
    }
  } catch (error) {
    ElMessage.info(extractErrorMessage(error) || '充值结果确认超时，请稍后刷新余额页面查看。')
  } finally {
    clearPollingState()
    polling.value = false
  }
}

const handleRecharge = async () => {
  if (rechargeForm.amount <= 0) {
    ElMessage.warning('请选择充值金额')
    return
  }

  if (!rechargeForm.method) {
    ElMessage.warning('当前没有可用的在线充值方式')
    return
  }

  recharging.value = true
  try {
    const result = await balanceStore.recharge(rechargeForm.amount, rechargeForm.method)
    const payment = result.payment || {}
    rechargeOrderNo.value = result.order_no || ''

    showRechargeDialog.value = false

    if (payment.payment_url) {
      window.open(payment.payment_url, '_blank', 'noopener,noreferrer')
    }

    if (payment.qrcode_data || payment.qrcode_url) {
      showQRDialog.value = true
      await generateQRCode(payment.qrcode_data || payment.qrcode_url)
    }

    if (rechargeOrderNo.value) {
      startPolling(rechargeOrderNo.value)
    }

    ElMessage.success(getSafeRedirectTarget() ? '充值订单已创建，支付完成后会自动返回原订单继续支付' : '充值订单已创建')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '充值失败')
  } finally {
    recharging.value = false
  }
}

watch(
  () => route.query.action,
  () => {
    openRechargeDialogIfNeeded()
  },
  { immediate: true }
)

onMounted(() => {
  fetchData()
  fetchRechargeMethods()
})

onUnmounted(() => {
  clearPollingState()
})
</script>

<style scoped>
.balance-page {
  padding: 20px;
  max-width: 1100px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.balance-card {
  margin-bottom: 20px;
  border-radius: 16px;
  background: linear-gradient(135deg, #409eff, #66b1ff);
}

.balance-card :deep(.el-card__body) {
  padding: 32px;
}

.balance-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.balance-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.balance-action-tip {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.88);
}

.balance-amount .label {
  display: block;
  font-size: 14px;
  color: rgba(255, 255, 255, 0.8);
  margin-bottom: 8px;
}

.balance-amount .amount {
  font-size: 36px;
  font-weight: 600;
  color: #fff;
}

.transactions-card {
  border-radius: 16px;
}

.transactions-card :deep(.el-card__body) {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.loading-container {
  padding: 20px;
}

.transactions-list {
  display: flex;
  flex-direction: column;
}

.transaction-item {
  display: flex;
  align-items: center;
  padding: 16px 0;
  border-bottom: 1px solid var(--color-border);
}

.transaction-item:last-child {
  border-bottom: none;
}

.tx-icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 16px;
  font-size: 18px;
}

.tx-icon--recharge {
  background: #e6f7e6;
  color: #67c23a;
}

.tx-icon--purchase {
  background: #fef0f0;
  color: #f56c6c;
}

.tx-icon--refund {
  background: #fdf6ec;
  color: #e6a23c;
}

.tx-icon--commission {
  background: #e6f7e6;
  color: #67c23a;
}

.tx-icon--adjustment {
  background: #f4f4f5;
  color: #909399;
}

.tx-info {
  flex: 1;
}

.tx-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.tx-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.tx-time {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 4px;
}

.tx-amount {
  font-size: 16px;
  font-weight: 500;
  color: #f56c6c;
}

.tx-amount--positive {
  color: #67c23a;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.amount-options {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.amount-option {
  padding: 12px 24px;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.3s;
}

.amount-option:hover {
  border-color: #409eff;
}

.amount-option--active {
  border-color: #409eff;
  background: rgba(64, 158, 255, 0.12);
  color: #409eff;
}

.qr-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.qr-code {
  padding: 16px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
}

.qr-title {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-primary);
}

.qr-amount,
.qr-tip {
  margin: 0;
  font-size: 13px;
  color: #909399;
}

@media (max-width: 640px) {
  .balance-page {
    padding: 12px 12px 96px;
  }

  .balance-info,
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .balance-actions {
    width: 100%;
    align-items: stretch;
  }
}
</style>
