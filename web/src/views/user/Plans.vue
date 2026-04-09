<template>
  <div class="plans-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <div class="header-content">
        <div>
          <h1 class="page-title">
            选择套餐
          </h1>
          <p class="page-subtitle">
            选择适合您的套餐，享受高速稳定的服务
          </p>
          <p class="page-hint">
            当前下单与支付统一以人民币结算，订单金额请以支付页展示为准。
          </p>
        </div>
      </div>
    </div>

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

    <!-- 套餐列表 -->
    <div
      v-else
      class="plans-grid"
    >
      <div
        v-for="plan in sortedPlans"
        :key="plan.id"
        class="plan-card"
        :class="{ 'plan-card--popular': plan.is_recommended }"
      >
        <!-- 热门标签 -->
        <div
          v-if="plan.is_recommended"
          class="popular-badge"
        >
          <el-icon><Star /></el-icon>
          热门
        </div>

        <!-- 套餐名称 -->
        <div class="plan-header">
          <h3 class="plan-name">
            {{ plan.name }}
          </h3>
          <p class="plan-description">
            {{ plan.description }}
          </p>
        </div>

        <!-- 价格 -->
        <div class="plan-price">
          <span class="price-currency">¥</span>
          <span class="price-amount">{{ formatPlanPrice(plan) }}</span>
          <span class="price-period">/ {{ plan.duration }}天</span>
        </div>

        <!-- 月均价格 -->
        <div class="plan-monthly">
          月均 ¥{{ formatMonthlyPrice(plan) }}
        </div>

        <!-- 功能列表 -->
        <ul class="plan-features">
          <li>
            <el-icon><Check /></el-icon>
            流量 {{ formatTraffic(plan.traffic_limit) }}
          </li>
          <li v-if="plan.ip_limit">
            <el-icon><Check /></el-icon>
            {{ plan.ip_limit }} 台设备同时在线
          </li>
          <li
            v-for="feature in plan.features"
            :key="feature"
          >
            <el-icon><Check /></el-icon>
            {{ feature }}
          </li>
        </ul>

        <!-- 购买按钮 -->
        <el-button
          type="primary"
          size="large"
          class="plan-button"
          :class="{ 'plan-button--popular': plan.is_recommended }"
          @click="selectPlan(plan)"
        >
          立即购买
        </el-button>
      </div>
    </div>

    <!-- 空状态 -->
    <el-empty
      v-if="!loading && sortedPlans.length === 0"
      description="暂无可用套餐"
    />
  </div>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Star, Check } from '@element-plus/icons-vue'
import { usePlanStore } from '@/stores/plan'
import { extractErrorMessage } from '@/utils/entitlement'
import { formatTrafficLimit } from '@/utils/traffic'

const router = useRouter()
const planStore = usePlanStore()
const loading = computed(() => planStore.loading)
const sortedPlans = computed(() => planStore.sortedPlans)

// 方法
const formatPrice = (price) => (price / 100).toFixed(2)

const formatPlanPrice = (plan) => formatPrice(plan.price)

const formatMonthlyPrice = (plan) => {
  const price = plan?.price || 0
  if (!plan || plan.duration <= 0) return '0.00'
  return formatPrice(Math.round(price / (plan.duration / 30)))
}

const formatTraffic = (bytes) => formatTrafficLimit(bytes, 0)

const selectPlan = (plan) => {
  router.push({
    name: 'user-payment',
    query: {
      plan_id: plan.id
    }
  })
}

const loadPlans = async () => {
  try {
    await planStore.fetchPlans()
  } catch (error) {
    console.error('加载套餐失败:', error)
    ElMessage.error(extractErrorMessage(error) || '加载套餐列表失败')
  }
}

onMounted(async () => {
  await loadPlans()
})
</script>

<style scoped>
.plans-page {
  padding: clamp(12px, 2vw, 20px);
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  text-align: center;
  margin-bottom: 40px;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  max-width: 800px;
  margin: 0 auto;
}

.page-title {
  font-size: 28px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
  text-align: left;
}

.page-subtitle {
  font-size: 16px;
  color: #909399;
  margin: 0;
  text-align: left;
}

.page-hint {
  font-size: 13px;
  color: #909399;
  margin: 8px 0 0;
  text-align: left;
}

.loading-container {
  padding: 40px;
}

.plans-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 360px));
  justify-content: center;
  gap: 24px;
  align-items: stretch;
}

.plan-card {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 100%;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 32px 24px;
  text-align: center;
  transition: all 0.3s;
}

.plan-card:hover {
  border-color: var(--color-primary);
  box-shadow: 0 4px 20px rgba(64, 158, 255, 0.15);
  transform: translateY(-4px);
}

.plan-card--popular {
  border-color: var(--color-primary);
  box-shadow: 0 4px 20px rgba(64, 158, 255, 0.2);
}

.popular-badge {
  position: absolute;
  top: -12px;
  left: 50%;
  transform: translateX(-50%);
  background: linear-gradient(135deg, #409eff, #66b1ff);
  color: #fff;
  padding: 4px 16px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 4px;
}

.plan-header {
  margin-bottom: 24px;
}

.plan-name {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.plan-description {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.plan-price {
  margin-bottom: 8px;
}

.price-currency {
  font-size: 20px;
  font-weight: 500;
  color: var(--color-text-primary);
  vertical-align: top;
}

.price-amount {
  font-size: 48px;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1;
}

.price-period {
  font-size: 14px;
  color: #909399;
}

.plan-monthly {
  font-size: 13px;
  color: #67c23a;
  margin-bottom: 24px;
}

.plan-features {
  list-style: none;
  flex: 1;
  padding: 0;
  margin: 0 0 24px 0;
  text-align: left;
}

.plan-features li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  font-size: 14px;
  color: #606266;
  border-bottom: 1px solid var(--color-border);
}

.plan-features li:last-child {
  border-bottom: none;
}

.plan-features .el-icon {
  color: #67c23a;
}

.plan-button {
  margin-top: auto;
  width: 100%;
}

.plan-button--popular {
  background: linear-gradient(135deg, #409eff, #66b1ff);
  border: none;
}

@media (max-width: 768px) {
  .page-header {
    margin-bottom: 24px;
  }

  .header-content {
    align-items: stretch;
    flex-direction: column;
  }

  .page-title,
  .page-subtitle {
    text-align: center;
  }

  .plans-grid {
    grid-template-columns: 1fr;
  }

  .plan-card {
    padding: 28px 18px 22px;
  }

  .price-amount {
    font-size: 40px;
  }
}
</style>
