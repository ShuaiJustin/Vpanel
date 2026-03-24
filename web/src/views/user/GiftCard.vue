<template>
  <div class="giftcard-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        礼品卡
      </h1>
      <p class="page-subtitle">
        兑换礼品卡充值余额
      </p>
    </div>

    <!-- 兑换卡片 -->
    <el-card
      shadow="never"
      class="redeem-card"
    >
      <div class="redeem-content">
        <div class="redeem-icon">
          <el-icon :size="48">
            <Present />
          </el-icon>
        </div>
        <h2 class="redeem-title">
          兑换礼品卡
        </h2>
        <p class="redeem-desc">
          输入礼品卡码，将面值充入您的账户余额
        </p>
        
        <div class="redeem-form">
          <el-input
            v-model="redeemCode"
            placeholder="请输入礼品卡码"
            size="large"
            clearable
            :disabled="redeeming"
            @keyup.enter="handleRedeem"
          >
            <template #prefix>
              <el-icon><Ticket /></el-icon>
            </template>
          </el-input>
          <el-button
            type="primary"
            size="large"
            :loading="redeeming"
            :disabled="!redeemCode.trim()"
            @click="handleRedeem"
          >
            兑换
          </el-button>
        </div>

        <!-- 验证结果 -->
        <div
          v-if="validationResult"
          class="validation-result"
          :class="{ 'validation-result--valid': validationResult.valid }"
        >
          <el-icon v-if="validationResult.valid">
            <CircleCheck />
          </el-icon>
          <el-icon v-else>
            <CircleClose />
          </el-icon>
          <span v-if="validationResult.valid">
            礼品卡有效，面值 ¥{{ (validationResult.value / 100).toFixed(2) }}
          </span>
          <span v-else>礼品卡无效或已使用</span>
        </div>
      </div>
    </el-card>

    <!-- 兑换记录 -->
    <el-card
      shadow="never"
      class="history-card"
    >
      <template #header>
        <span>兑换记录</span>
      </template>

      <!-- 加载状态 -->
      <div
        v-if="loading"
        class="loading-container"
      >
        <el-skeleton
          :rows="3"
          animated
        />
      </div>

      <!-- 记录列表 -->
      <div
        v-else
        class="history-list"
      >
        <div
          v-for="gc in giftCards"
          :key="gc.id"
          class="history-item"
        >
          <div class="gc-icon">
            <el-icon><Present /></el-icon>
          </div>
          <div class="gc-info">
            <div class="gc-code">
              {{ maskCode(gc.code) }}
            </div>
            <div class="gc-time">
              {{ formatTime(gc.redeemed_at) }}
            </div>
          </div>
          <div class="gc-value">
            +¥{{ (gc.value / 100).toFixed(2) }}
          </div>
        </div>
      </div>

      <!-- 空状态 -->
      <el-empty
        v-if="!loading && giftCards.length === 0"
        description="暂无兑换记录"
      />

      <!-- 分页 -->
      <div
        v-if="pagination.total > pagination.pageSize"
        class="pagination-container"
      >
        <el-pagination
          v-model:current-page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.pageSize"
          layout="prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 兑换成功对话框 -->
    <el-dialog
      v-model="showSuccessDialog"
      title="兑换成功"
      width="400px"
      center
    >
      <div class="success-content">
        <el-icon
          class="success-icon"
          :size="64"
        >
          <CircleCheck />
        </el-icon>
        <p class="success-amount">
          +¥{{ (redeemResult?.credited / 100 || 0).toFixed(2) }}
        </p>
        <p class="success-text">
          礼品卡已成功兑换，金额已充入您的账户余额
        </p>
      </div>
      <template #footer>
        <el-button
          type="primary"
          @click="showSuccessDialog = false"
        >
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Present, Ticket, CircleCheck, CircleClose } from '@element-plus/icons-vue'
import { giftCardsApi } from '@/api'

// 状态
const loading = ref(false)
const redeeming = ref(false)
const redeemCode = ref('')
const validationResult = ref(null)
const showSuccessDialog = ref(false)
const redeemResult = ref(null)
const giftCards = ref([])
const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

// 方法
const fetchGiftCards = async () => {
  loading.value = true
  try {
    const res = await giftCardsApi.list({
      page: pagination.page,
      page_size: pagination.pageSize
    })
    giftCards.value = res.gift_cards || []
    pagination.total = res.total || 0
  } catch (error) {
    console.error('Failed to fetch gift cards:', error)
  } finally {
    loading.value = false
  }
}

const handleRedeem = async () => {
  const code = redeemCode.value.trim()
  if (!code) {
    ElMessage.warning('请输入礼品卡码')
    return
  }

  redeeming.value = true
  validationResult.value = null

  try {
    const res = await giftCardsApi.redeem({ code })
    redeemResult.value = res
    showSuccessDialog.value = true
    redeemCode.value = ''
    // 刷新列表
    fetchGiftCards()
  } catch (error) {
    const msg = error.response?.data?.error || '兑换失败'
    ElMessage.error(msg)
  } finally {
    redeeming.value = false
  }
}

const handlePageChange = (page) => {
  pagination.page = page
  fetchGiftCards()
}

const maskCode = (code) => {
  if (!code || code.length < 8) return code
  return code.substring(0, 4) + '****' + code.substring(code.length - 4)
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

onMounted(() => {
  fetchGiftCards()
})
</script>


<style scoped>
.giftcard-page {
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

.redeem-card {
  margin-bottom: 20px;
  border-radius: 8px;
}

.redeem-card :deep(.el-card__body) {
  padding: 40px;
}

.redeem-content {
  text-align: center;
  max-width: 500px;
  margin: 0 auto;
}

.redeem-icon {
  color: #409eff;
  margin-bottom: 16px;
}

.redeem-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.redeem-desc {
  font-size: 14px;
  color: #909399;
  margin: 0 0 24px 0;
}

.redeem-form {
  display: flex;
  gap: 12px;
}

.redeem-form .el-input {
  flex: 1;
}

.validation-result {
  margin-top: 16px;
  padding: 12px;
  border-radius: 4px;
  background: #fef0f0;
  color: #f56c6c;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.validation-result--valid {
  background: #f0f9eb;
  color: #67c23a;
}

.history-card {
  border-radius: 8px;
}

.loading-container {
  padding: 20px;
}

.history-list {
  display: flex;
  flex-direction: column;
}

.history-item {
  display: flex;
  align-items: center;
  padding: 16px 0;
  border-bottom: 1px solid var(--color-border);
}

.history-item:last-child {
  border-bottom: none;
}

.gc-icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: #e6f7e6;
  color: #67c23a;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 16px;
  font-size: 18px;
}

.gc-info {
  flex: 1;
}

.gc-code {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
  font-family: monospace;
}

.gc-time {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.gc-value {
  font-size: 16px;
  font-weight: 500;
  color: #67c23a;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

.success-content {
  text-align: center;
  padding: 20px 0;
}

.success-icon {
  color: #67c23a;
}

.success-amount {
  font-size: 32px;
  font-weight: 600;
  color: #67c23a;
  margin: 16px 0 8px 0;
}

.success-text {
  font-size: 14px;
  color: #909399;
  margin: 0;
}
</style>
