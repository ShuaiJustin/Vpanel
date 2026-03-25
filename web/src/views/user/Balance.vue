<template>
  <div class="balance-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        我的余额
      </h1>
      <p class="page-subtitle">
        查看余额和交易记录
      </p>
    </div>

    <!-- 余额卡片 -->
    <el-card
      shadow="never"
      class="balance-card"
    >
      <div class="balance-info">
        <div class="balance-amount">
          <span class="label">可用余额</span>
          <span class="amount">¥{{ formattedBalance }}</span>
        </div>
        <el-button
          type="primary"
          @click="showRechargeDialog = true"
        >
          <el-icon><Plus /></el-icon>
          充值
        </el-button>
      </div>
    </el-card>

    <!-- 交易记录 -->
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
            <el-option
              label="充值"
              value="recharge"
            />
            <el-option
              label="消费"
              value="purchase"
            />
            <el-option
              label="退款"
              value="refund"
            />
            <el-option
              label="佣金"
              value="commission"
            />
            <el-option
              label="调整"
              value="adjustment"
            />
          </el-select>
        </div>
      </template>

      <!-- 加载状态 -->
      <div
        v-if="loading"
        class="loading-container"
      >
        <el-skeleton
          :rows="5"
          animated
        />
      </div>

      <!-- 交易列表 -->
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

      <!-- 空状态 -->
      <el-empty
        v-if="!loading && transactions.length === 0"
        description="暂无交易记录"
      />

      <!-- 分页 -->
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

    <!-- 充值对话框 -->
    <el-dialog
      v-model="showRechargeDialog"
      title="余额充值"
      width="400px"
    >
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
            @change="rechargeForm.amount = rechargeForm.customAmount * 100"
          />
        </el-form-item>
        <el-form-item label="支付方式">
          <el-radio-group v-model="rechargeForm.method">
            <el-radio value="alipay">
              支付宝
            </el-radio>
            <el-radio value="wechat">
              微信支付
            </el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRechargeDialog = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="recharging"
          @click="handleRecharge"
        >
          充值 ¥{{ (rechargeForm.amount / 100).toFixed(2) }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Coin, ShoppingCart, RefreshRight, Money, Edit } from '@element-plus/icons-vue'
import { useBalanceStore } from '@/stores/balance'

const balanceStore = useBalanceStore()

// 状态
const typeFilter = ref('')
const showRechargeDialog = ref(false)
const recharging = ref(false)
const rechargeAmounts = [1000, 5000, 10000, 20000, 50000] // 分

const rechargeForm = reactive({
  amount: 5000,
  customAmount: null,
  method: 'alipay'
})

// 计算属性
const loading = computed(() => balanceStore.loading)
const formattedBalance = computed(() => balanceStore.formattedBalance)
const transactions = computed(() => balanceStore.transactions)
const pagination = computed(() => balanceStore.pagination)

// 类型图标映射
const typeIcons = {
  recharge: Plus,
  purchase: ShoppingCart,
  refund: RefreshRight,
  commission: Coin,
  adjustment: Edit
}

// 方法
const getTypeInfo = (type) => {
  const info = balanceStore.getTypeInfo(type)
  return { ...info, icon: typeIcons[type] || Money }
}

const formatAmount = (amount) => {
  const yuan = (Math.abs(amount) / 100).toFixed(2)
  return amount >= 0 ? `+¥${yuan}` : `-¥${yuan}`
}

const fetchData = async () => {
  await Promise.all([
    balanceStore.fetchBalance(),
    balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
  ])
}

const handleFilterChange = () => {
  balanceStore.setPage(1)
  balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
}

const handlePageChange = (page) => {
  balanceStore.setPage(page)
  balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
}

const handleSizeChange = (size) => {
  balanceStore.setPageSize(size)
  balanceStore.fetchTransactions(typeFilter.value ? { type: typeFilter.value } : {})
}

const handleRecharge = async () => {
  if (rechargeForm.amount <= 0) {
    ElMessage.warning('请选择充值金额')
    return
  }

  recharging.value = true
  try {
    const result = await balanceStore.recharge(rechargeForm.amount, rechargeForm.method)
    if (result.payment_url) {
      // 跳转到支付页面
      window.open(result.payment_url, '_blank')
    }
    showRechargeDialog.value = false
    ElMessage.success('充值订单已创建')
  } catch (error) {
    ElMessage.error(error || '充值失败')
  } finally {
    recharging.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.balance-page {
  padding: 20px;
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
  border-radius: 8px;
  background: linear-gradient(135deg, #409eff, #66b1ff);
}

.balance-card :deep(.el-card__body) {
  padding: 32px;
}

.balance-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
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
  border-radius: 8px;
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
</style>
