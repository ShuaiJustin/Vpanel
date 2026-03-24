<template>
  <div class="plan-upgrade-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        套餐升降级
      </h1>
      <p class="page-subtitle">
        调整您的套餐以满足不同需求
      </p>
    </div>

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

    <template v-else>
      <!-- 当前套餐信息 -->
      <el-card
        v-if="currentPlan"
        class="current-plan-card"
      >
        <template #header>
          <div class="card-header">
            <span>当前套餐</span>
            <el-tag type="success">
              使用中
            </el-tag>
          </div>
        </template>
        <div class="current-plan-info">
          <div class="plan-name">
            {{ currentPlan.name }}
          </div>
          <div class="plan-details">
            <span>价格: ¥{{ formatPrice(currentPlan.price) }} / {{ currentPlan.duration }}天</span>
            <span>流量: {{ formatTraffic(currentPlan.traffic_limit) }}</span>
            <span v-if="expiresAt">到期时间: {{ formatDate(expiresAt) }}</span>
            <span v-if="remainingDays > 0">剩余: {{ remainingDays }} 天</span>
          </div>
        </div>
      </el-card>

      <!-- 待执行的降级提示 -->
      <el-alert
        v-if="pendingDowngrade"
        type="warning"
        :closable="false"
        class="pending-alert"
      >
        <template #title>
          <div class="pending-info">
            <span>您有一个待执行的降级计划</span>
            <el-button
              type="text"
              size="small"
              @click="cancelDowngrade"
            >
              取消降级
            </el-button>
          </div>
        </template>
        <template #default>
          将于 {{ formatDate(pendingDowngrade.effective_at) }} 降级至 {{ pendingDowngrade.new_plan?.name }}
        </template>
      </el-alert>

      <!-- 可选套餐列表 -->
      <div class="plans-section">
        <h2 class="section-title">
          选择新套餐
        </h2>
        <div class="plans-grid">
          <div
            v-for="plan in availablePlans"
            :key="plan.id"
            class="plan-card"
            :class="{
              'plan-card--selected': selectedPlan?.id === plan.id,
              'plan-card--upgrade': plan.price > (currentPlan?.price || 0),
              'plan-card--downgrade': plan.price < (currentPlan?.price || 0),
              'plan-card--current': plan.id === currentPlan?.id
            }"
            @click="selectPlan(plan)"
          >
            <!-- 标签 -->
            <div
              v-if="plan.id !== currentPlan?.id"
              class="plan-badge"
            >
              <el-tag
                v-if="plan.price > (currentPlan?.price || 0)"
                type="success"
                size="small"
              >
                升级
              </el-tag>
              <el-tag
                v-else
                type="warning"
                size="small"
              >
                降级
              </el-tag>
            </div>
            <div
              v-else
              class="plan-badge"
            >
              <el-tag
                type="info"
                size="small"
              >
                当前
              </el-tag>
            </div>

            <div class="plan-header">
              <h3 class="plan-name">
                {{ plan.name }}
              </h3>
            </div>

            <div class="plan-price">
              <span class="price-currency">¥</span>
              <span class="price-amount">{{ formatPrice(plan.price) }}</span>
              <span class="price-period">/ {{ plan.duration }}天</span>
            </div>

            <ul class="plan-features">
              <li>
                <el-icon><Check /></el-icon>
                流量 {{ formatTraffic(plan.traffic_limit) }}
              </li>
            </ul>
          </div>
        </div>
      </div>

      <!-- 价格计算结果 -->
      <el-card
        v-if="changeResult"
        class="result-card"
      >
        <template #header>
          <div class="card-header">
            <span>{{ changeResult.is_upgrade ? '升级' : '降级' }}详情</span>
          </div>
        </template>
        <div class="result-content">
          <div class="result-row">
            <span>当前套餐</span>
            <span>{{ changeResult.current_plan?.name }}</span>
          </div>
          <div class="result-row">
            <span>新套餐</span>
            <span>{{ changeResult.new_plan?.name }}</span>
          </div>
          <div class="result-row">
            <span>剩余天数</span>
            <span>{{ changeResult.remaining_days }} 天</span>
          </div>
          <div
            v-if="changeResult.is_upgrade"
            class="result-row highlight"
          >
            <span>需补差价</span>
            <span class="price">¥{{ formatPrice(changeResult.price_difference) }}</span>
          </div>
          <div
            v-else
            class="result-row"
          >
            <span>生效时间</span>
            <span>下个计费周期</span>
          </div>
        </div>

        <div class="result-actions">
          <el-button
            v-if="changeResult.is_upgrade"
            type="primary"
            size="large"
            :loading="submitting"
            @click="confirmUpgrade"
          >
            确认升级 (¥{{ formatPrice(changeResult.price_difference) }})
          </el-button>
          <el-button
            v-else
            type="warning"
            size="large"
            :loading="submitting"
            @click="confirmDowngrade"
          >
            确认降级
          </el-button>
          <el-button
            size="large"
            @click="cancelSelection"
          >
            取消
          </el-button>
        </div>
      </el-card>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check } from '@element-plus/icons-vue'
import { planChangeApi, plansApi } from '@/api'
import { useUserPortalStore } from '@/stores/userPortal'

const router = useRouter()
const userStore = useUserPortalStore()

// 状态
const loading = ref(true)
const submitting = ref(false)
const plans = ref([])
const currentPlan = ref(null)
const selectedPlan = ref(null)
const changeResult = ref(null)
const pendingDowngrade = ref(null)
const expiresAt = ref(null)

// 计算属性
const remainingDays = computed(() => {
  if (!expiresAt.value) return 0
  const now = new Date()
  const expires = new Date(expiresAt.value)
  const diff = expires - now
  return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)))
})

const availablePlans = computed(() => {
  return plans.value.filter(p => p.is_active)
})

// 方法
const formatPrice = (price) => {
  if (!price) return '0.00'
  return (price / 100).toFixed(2)
}

const formatTraffic = (bytes) => {
  if (!bytes || bytes === 0) return '无限制'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(0)} ${units[i]}`
}

const formatDate = (dateStr) => {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  })
}

const selectPlan = async (plan) => {
  if (plan.id === currentPlan.value?.id) {
    ElMessage.info('这是您当前的套餐')
    return
  }

  selectedPlan.value = plan
  changeResult.value = null

  try {
    const response = await planChangeApi.calculate({
      current_plan_id: currentPlan.value.id,
      new_plan_id: plan.id
    })
    changeResult.value = response.data
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '计算价格失败')
    selectedPlan.value = null
  }
}

const cancelSelection = () => {
  selectedPlan.value = null
  changeResult.value = null
}

const confirmUpgrade = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要升级到 ${selectedPlan.value.name} 吗？将从余额扣除 ¥${formatPrice(changeResult.value.price_difference)}`,
      '确认升级',
      { type: 'warning' }
    )

    submitting.value = true
    await planChangeApi.upgrade({
      current_plan_id: currentPlan.value.id,
      new_plan_id: selectedPlan.value.id
    })

    ElMessage.success('升级成功')
    router.push({ name: 'UserSubscription' })
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || '升级失败')
    }
  } finally {
    submitting.value = false
  }
}

const confirmDowngrade = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要降级到 ${selectedPlan.value.name} 吗？降级将在当前套餐到期后生效`,
      '确认降级',
      { type: 'warning' }
    )

    submitting.value = true
    await planChangeApi.downgrade({
      current_plan_id: currentPlan.value.id,
      new_plan_id: selectedPlan.value.id
    })

    ElMessage.success('降级已预约，将在当前套餐到期后生效')
    await fetchPendingDowngrade()
    cancelSelection()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || '降级预约失败')
    }
  } finally {
    submitting.value = false
  }
}

const cancelDowngrade = async () => {
  try {
    await ElMessageBox.confirm('确定要取消降级计划吗？', '取消降级', { type: 'warning' })
    await planChangeApi.cancelDowngrade()
    ElMessage.success('降级计划已取消')
    pendingDowngrade.value = null
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || '取消失败')
    }
  }
}

const fetchPendingDowngrade = async () => {
  try {
    const response = await planChangeApi.getPendingDowngrade()
    pendingDowngrade.value = response.data
  } catch {
    pendingDowngrade.value = null
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    // 获取套餐列表
    const plansResponse = await plansApi.list()
    plans.value = Array.isArray(plansResponse.plans) ? plansResponse.plans : []

    // 获取用户信息（包含当前套餐）
    // 这里假设用户信息中有 plan_id 和 expires_at
    const userInfo = userStore.userInfo
    if (userInfo) {
      expiresAt.value = userInfo.expires_at
      // 根据用户的 plan_id 找到当前套餐
      if (userInfo.plan_id) {
        currentPlan.value = plans.value.find(p => p.id === userInfo.plan_id)
      }
    }

    // 如果没有找到当前套餐，使用第一个套餐作为示例
    if (!currentPlan.value && plans.value.length > 0) {
      currentPlan.value = plans.value[0]
    }

    // 获取待执行的降级
    await fetchPendingDowngrade()
  } catch (error) {
    ElMessage.error('加载数据失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>


<style scoped>
.plan-upgrade-page {
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  text-align: center;
  margin-bottom: 32px;
}

.page-title {
  font-size: 28px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 16px;
  color: #909399;
  margin: 0;
}

.loading-container {
  padding: 40px;
}

.current-plan-card {
  margin-bottom: 24px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.current-plan-info {
  text-align: center;
}

.current-plan-info .plan-name {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 12px;
}

.current-plan-info .plan-details {
  display: flex;
  justify-content: center;
  gap: 24px;
  flex-wrap: wrap;
  color: #606266;
  font-size: 14px;
}

.pending-alert {
  margin-bottom: 24px;
}

.pending-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 16px 0;
}

.plans-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.plan-card {
  position: relative;
  background: var(--color-bg-card);
  border: 2px solid #ebeef5;
  border-radius: 12px;
  padding: 24px 16px;
  text-align: center;
  cursor: pointer;
  transition: all 0.3s;
}

.plan-card:hover {
  border-color: #409eff;
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.15);
}

.plan-card--selected {
  border-color: #409eff;
  background: #ecf5ff;
}

.plan-card--current {
  opacity: 0.6;
  cursor: not-allowed;
}

.plan-card--upgrade:not(.plan-card--current):hover {
  border-color: #67c23a;
}

.plan-card--downgrade:not(.plan-card--current):hover {
  border-color: #e6a23c;
}

.plan-badge {
  position: absolute;
  top: 8px;
  right: 8px;
}

.plan-card .plan-header {
  margin-bottom: 16px;
}

.plan-card .plan-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}

.plan-card .plan-price {
  margin-bottom: 16px;
}

.plan-card .price-currency {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.plan-card .price-amount {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text-primary);
}

.plan-card .price-period {
  font-size: 12px;
  color: #909399;
}

.plan-card .plan-features {
  list-style: none;
  padding: 0;
  margin: 0;
  text-align: left;
}

.plan-card .plan-features li {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #606266;
  padding: 4px 0;
}

.plan-card .plan-features .el-icon {
  color: #67c23a;
}

.result-card {
  margin-top: 24px;
}

.result-content {
  margin-bottom: 24px;
}

.result-row {
  display: flex;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid var(--color-border);
  font-size: 14px;
  color: #606266;
}

.result-row:last-child {
  border-bottom: none;
}

.result-row.highlight {
  font-weight: 600;
  color: var(--color-text-primary);
}

.result-row .price {
  color: #f56c6c;
  font-size: 18px;
}

.result-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}

@media (max-width: 768px) {
  .plans-grid {
    grid-template-columns: 1fr;
  }

  .current-plan-info .plan-details {
    flex-direction: column;
    gap: 8px;
  }

  .result-actions {
    flex-direction: column;
  }
}
</style>
