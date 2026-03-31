<template>
  <div class="admin-balances-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          余额管理
        </h1>
        <p class="page-subtitle">
          查看用户余额、交易流水，并执行人工余额调整
        </p>
      </div>
      <div class="page-actions">
        <el-button
          :loading="loadingUsers"
          @click="loadUsers"
        >
          刷新用户
        </el-button>
        <el-button
          :loading="loadingBalance || loadingTransactions"
          :disabled="!selectedUserId"
          @click="refreshCurrentUserData"
        >
          刷新余额
        </el-button>
      </div>
    </div>

    <el-card
      shadow="never"
      class="selector-card"
    >
      <div class="selector-grid">
        <el-select
          v-model="selectedUserId"
          filterable
          clearable
          placeholder="选择用户（支持搜索用户名、邮箱、ID）"
          class="selector-control"
        >
          <el-option
            v-for="user in users"
            :key="user.id"
            :label="getUserOptionLabel(user)"
            :value="user.id"
          />
        </el-select>
        <div class="selector-user-meta">
          <template v-if="selectedUser">
            <span class="selector-user-name">{{ selectedUser.username }}</span>
            <span class="selector-user-hint">UID {{ selectedUser.id }} · {{ selectedUser.email || '未填写邮箱' }}</span>
          </template>
          <span v-else class="selector-user-hint">请选择一个用户后查看余额详情</span>
        </div>
      </div>
    </el-card>

    <template v-if="selectedUserId">
      <div class="overview-strip">
        <div class="overview-card">
          <span class="overview-label">当前可用余额</span>
          <strong class="overview-value is-primary">¥{{ formattedBalance }}</strong>
        </div>
        <div class="overview-card">
          <span class="overview-label">流水总条数</span>
          <strong class="overview-value">{{ transactionPagination.total }}</strong>
        </div>
        <div class="overview-card">
          <span class="overview-label">当前页收入</span>
          <strong class="overview-value is-success">¥{{ currentPageIncome }}</strong>
        </div>
        <div class="overview-card">
          <span class="overview-label">当前页支出</span>
          <strong class="overview-value is-danger">¥{{ currentPageExpense }}</strong>
        </div>
      </div>

      <div class="detail-grid">
        <el-card shadow="never" class="adjust-card">
          <template #header>
            <div class="card-header">
              <span>余额调整</span>
              <span class="toolbar-summary">仅用于补偿、修正或客服处理</span>
            </div>
          </template>
          <el-form label-position="top">
            <el-form-item label="调整方向">
              <el-radio-group v-model="adjustForm.direction">
                <el-radio-button value="increase">
                  增加余额
                </el-radio-button>
                <el-radio-button value="decrease">
                  扣减余额
                </el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="调整金额（元）">
              <el-input-number
                v-model="adjustForm.amount"
                :min="0.01"
                :precision="2"
                :step="10"
                controls-position="right"
                class="adjust-amount"
              />
            </el-form-item>
            <el-form-item label="调整原因">
              <el-input
                v-model="adjustForm.reason"
                maxlength="120"
                show-word-limit
                placeholder="例如：工单补偿、充值异常修正、管理员手动扣减"
              />
            </el-form-item>
            <div class="adjust-actions">
              <el-button @click="resetAdjustForm">
                重置
              </el-button>
              <el-button
                type="primary"
                :loading="submittingAdjust"
                @click="submitAdjustment"
              >
                确认调整
              </el-button>
            </div>
          </el-form>
        </el-card>

        <el-card shadow="never" class="tips-card">
          <template #header>
            <div class="card-header">
              <span>操作说明</span>
            </div>
          </template>
          <ul class="tips-list">
            <li>正向调整会增加用户余额，负向调整会直接扣减余额。</li>
            <li>每次调整都会写入余额流水，建议原因填写清晰可追踪。</li>
            <li>若用户余额不足，扣减会失败，避免出现负数余额。</li>
            <li>用户在线充值产生的入账会出现在“充值订单”和“余额流水”两处。</li>
          </ul>
        </el-card>
      </div>

      <el-card shadow="never" class="transactions-card">
        <template #header>
          <div class="card-header">
            <span>余额流水</span>
            <span class="toolbar-summary">当前页 {{ transactions.length }} 条</span>
          </div>
        </template>

        <div class="toolbar-card toolbar-card--inner">
          <div class="toolbar-grid">
            <el-select
              v-model="transactionFilter.type"
              clearable
              placeholder="流水类型"
            >
              <el-option label="充值" value="recharge" />
              <el-option label="消费" value="purchase" />
              <el-option label="退款" value="refund" />
              <el-option label="佣金" value="commission" />
              <el-option label="调整" value="adjustment" />
            </el-select>
            <el-date-picker
              v-model="transactionFilter.dateRange"
              type="daterange"
              value-format="YYYY-MM-DD"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              range-separator="至"
            />
          </div>
          <div class="toolbar-actions">
            <el-button @click="resetTransactionFilters">
              重置
            </el-button>
            <el-button
              type="primary"
              @click="applyTransactionFilters"
            >
              筛选
            </el-button>
          </div>
        </div>

        <el-table
          v-loading="loadingTransactions"
          :data="transactions"
          border
        >
          <el-table-column label="类型" min-width="110">
            <template #default="{ row }">
              <el-tag :type="getTypeTagType(row.type)">
                {{ getTypeLabel(row.type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="变动金额" min-width="120" align="right">
            <template #default="{ row }">
              <span :class="['amount-text', row.amount > 0 ? 'is-success' : 'is-danger']">
                {{ formatAmount(row.amount) }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="变动后余额" min-width="120" align="right">
            <template #default="{ row }">
              ¥{{ formatPrice(row.balance) }}
            </template>
          </el-table-column>
          <el-table-column prop="description" label="说明" min-width="220" show-overflow-tooltip />
          <el-table-column prop="operator" label="操作人" min-width="120">
            <template #default="{ row }">
              {{ row.operator || '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="时间" min-width="170" />
        </el-table>

        <el-empty
          v-if="!loadingTransactions && transactions.length === 0"
          description="暂无余额流水"
        />

        <div
          v-if="transactionPagination.total > 0"
          class="pagination-container"
        >
          <el-pagination
            v-model:current-page="transactionPagination.page"
            v-model:page-size="transactionPagination.pageSize"
            :total="transactionPagination.total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next"
            @size-change="handleTransactionSizeChange"
            @current-change="handleTransactionPageChange"
          />
        </div>
      </el-card>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { balanceApi, usersApi } from '@/api'
import { extractErrorMessage } from '@/utils/entitlement'

const users = ref([])
const selectedUserId = ref(null)
const currentBalance = ref(0)
const transactions = ref([])
const loadingUsers = ref(false)
const loadingBalance = ref(false)
const loadingTransactions = ref(false)
const submittingAdjust = ref(false)

const transactionPagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const transactionFilter = reactive({
  type: '',
  dateRange: []
})

const adjustForm = reactive({
  direction: 'increase',
  amount: 10,
  reason: ''
})

const typeMap = {
  recharge: { label: '充值', tag: 'success' },
  purchase: { label: '消费', tag: 'danger' },
  refund: { label: '退款', tag: 'warning' },
  commission: { label: '佣金', tag: 'success' },
  adjustment: { label: '调整', tag: 'info' }
}

const selectedUser = computed(() => users.value.find(user => user.id === selectedUserId.value) || null)
const formattedBalance = computed(() => formatPrice(currentBalance.value))
const currentPageIncome = computed(() => formatPrice(transactions.value.filter(tx => tx.amount > 0).reduce((sum, tx) => sum + Number(tx.amount || 0), 0)))
const currentPageExpense = computed(() => formatPrice(Math.abs(transactions.value.filter(tx => tx.amount < 0).reduce((sum, tx) => sum + Number(tx.amount || 0), 0))))

const formatPrice = amount => (Number(amount || 0) / 100).toFixed(2)
const formatAmount = amount => `${amount >= 0 ? '+' : '-'}¥${formatPrice(Math.abs(amount || 0))}`
const getTypeLabel = type => typeMap[type]?.label || type || '-'
const getTypeTagType = type => typeMap[type]?.tag || 'info'
const getUserOptionLabel = user => `${user.username}（ID ${user.id}${user.email ? ` · ${user.email}` : ''}）`

const loadUsers = async () => {
  loadingUsers.value = true
  try {
    const response = await usersApi.list()
    users.value = response.users || []
    if (!selectedUserId.value && users.value.length > 0) {
      selectedUserId.value = users.value[0].id
    }
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载用户列表失败')
  } finally {
    loadingUsers.value = false
  }
}

const fetchBalance = async () => {
  if (!selectedUserId.value) {
    currentBalance.value = 0
    return
  }

  loadingBalance.value = true
  try {
    const response = await balanceApi.admin.getUserBalance(selectedUserId.value)
    currentBalance.value = response.balance || 0
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载用户余额失败')
  } finally {
    loadingBalance.value = false
  }
}

const buildTransactionParams = () => {
  const params = {
    page: transactionPagination.page,
    page_size: transactionPagination.pageSize,
    type: transactionFilter.type || undefined
  }

  if (transactionFilter.dateRange?.length === 2) {
    params.start_date = `${transactionFilter.dateRange[0]} 00:00:00`
    params.end_date = `${transactionFilter.dateRange[1]} 23:59:59`
  }

  return params
}

const fetchTransactions = async () => {
  if (!selectedUserId.value) {
    transactions.value = []
    transactionPagination.total = 0
    return
  }

  loadingTransactions.value = true
  try {
    const response = await balanceApi.admin.getTransactions(selectedUserId.value, buildTransactionParams())
    transactions.value = response.transactions || []
    transactionPagination.total = response.total || 0
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载余额流水失败')
  } finally {
    loadingTransactions.value = false
  }
}

const refreshCurrentUserData = async () => {
  await Promise.all([fetchBalance(), fetchTransactions()])
}

const resetAdjustForm = () => {
  adjustForm.direction = 'increase'
  adjustForm.amount = 10
  adjustForm.reason = ''
}

const submitAdjustment = async () => {
  if (!selectedUserId.value) {
    ElMessage.warning('请先选择用户')
    return
  }
  if (!adjustForm.amount || Number(adjustForm.amount) <= 0) {
    ElMessage.warning('请输入有效的调整金额')
    return
  }
  if (!adjustForm.reason.trim()) {
    ElMessage.warning('请填写调整原因')
    return
  }

  const cents = Math.round(Number(adjustForm.amount) * 100)
  const signedAmount = adjustForm.direction === 'decrease' ? -cents : cents

  submittingAdjust.value = true
  try {
    await balanceApi.admin.adjust({
      user_id: selectedUserId.value,
      amount: signedAmount,
      reason: adjustForm.reason.trim()
    })
    ElMessage.success('余额调整成功')
    resetAdjustForm()
    await refreshCurrentUserData()
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '余额调整失败')
  } finally {
    submittingAdjust.value = false
  }
}

const applyTransactionFilters = async () => {
  transactionPagination.page = 1
  await fetchTransactions()
}

const resetTransactionFilters = async () => {
  transactionFilter.type = ''
  transactionFilter.dateRange = []
  transactionPagination.page = 1
  await fetchTransactions()
}

const handleTransactionPageChange = async page => {
  transactionPagination.page = page
  await fetchTransactions()
}

const handleTransactionSizeChange = async size => {
  transactionPagination.pageSize = size
  transactionPagination.page = 1
  await fetchTransactions()
}

watch(selectedUserId, async () => {
  transactionPagination.page = 1
  await refreshCurrentUserData()
})

onMounted(loadUsers)
</script>

<style scoped>
.admin-balances-page {
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

.selector-card,
.overview-card,
.toolbar-card {
  border-radius: 16px;
}

.selector-card {
  margin-bottom: 20px;
}

.selector-grid {
  display: grid;
  grid-template-columns: minmax(280px, 420px) 1fr;
  gap: 16px;
  align-items: center;
}

.selector-control {
  width: 100%;
}

.selector-user-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.selector-user-name {
  font-size: 16px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.selector-user-hint,
.toolbar-summary {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.overview-card {
  border: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color-overlay);
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.overview-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.overview-value {
  font-size: 26px;
  font-weight: 700;
}

.overview-value.is-primary {
  color: var(--el-color-primary);
}

.overview-value.is-success {
  color: var(--el-color-success);
}

.overview-value.is-danger {
  color: var(--el-color-danger);
}

.detail-grid {
  display: grid;
  grid-template-columns: minmax(320px, 420px) 1fr;
  gap: 20px;
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.adjust-amount {
  width: 100%;
}

.adjust-actions,
.toolbar-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.tips-list {
  margin: 0;
  padding-left: 18px;
  color: var(--el-text-color-regular);
  line-height: 1.8;
}

.transactions-card {
  border-radius: 16px;
}

.toolbar-card--inner {
  margin-bottom: 16px;
}

.toolbar-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.amount-text {
  font-weight: 700;
}

.amount-text.is-success {
  color: var(--el-color-success);
}

.amount-text.is-danger {
  color: var(--el-color-danger);
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 960px) {
  .admin-balances-page {
    padding: 12px;
  }

  .page-header,
  .selector-grid,
  .detail-grid,
  .card-header {
    grid-template-columns: 1fr;
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
