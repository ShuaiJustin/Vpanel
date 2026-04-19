<template>
  <div class="subscription-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        订阅管理
      </h1>
      <p class="page-subtitle">
        获取订阅链接，导入到您的客户端使用
      </p>
      <p class="page-hint">
        {{ subscriptionRefreshHint }}
      </p>
    </div>

    <!-- 订阅状态和操作卡片 -->
    <el-row
      :gutter="20"
      class="subscription-actions"
    >
      <!-- 订阅状态卡片 -->
      <el-col
        :xs="24"
        :lg="12"
      >
        <el-card
          class="status-card"
          shadow="never"
        >
          <template #header>
            <div class="card-header">
              <span>订阅状态</span>
              <el-tag
                :type="subscriptionStatus.type"
                size="small"
              >
                {{ subscriptionStatus.label }}
              </el-tag>
            </div>
          </template>
          <div class="status-content">
            <el-descriptions
              :column="1"
              border
              size="small"
            >
              <el-descriptions-item label="到期时间">
                {{ expiryDisplayText }}
                <span
                  v-if="daysUntilExpiry !== null"
                  class="days-hint"
                >
                  ({{ daysUntilExpiry > 0 ? `剩余 ${daysUntilExpiry} 天` : '已过期' }})
                </span>
              </el-descriptions-item>
              <el-descriptions-item label="当前周期已用流量">
                {{ trafficUsageDisplay }}
              </el-descriptions-item>
              <el-descriptions-item label="可用节点">
                {{ availableNodes }} 个
              </el-descriptions-item>
            </el-descriptions>
            
            <!-- 升级/降级按钮 -->
            <div class="action-buttons">
              <el-button
                type="primary"
                @click="handlePrimaryPlanAction"
              >
                <el-icon><TrendCharts /></el-icon>
                {{ hasActiveSubscription ? '升级/降级套餐' : '升级为正式套餐' }}
              </el-button>
              <el-button
                type="success"
                @click="goToPlans"
              >
                <el-icon><ShoppingCart /></el-icon>
                {{ secondaryPlanActionLabel }}
              </el-button>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 暂停/恢复卡片 -->
      <el-col
        :xs="24"
        :lg="12"
      >
        <PauseCard />
      </el-col>
    </el-row>

    <!-- 订阅链接卡片 -->
    <el-card
      v-if="!noEntitlement"
      class="subscription-card"
      shadow="never"
    >
      <template #header>
        <div class="card-header">
          <span>订阅链接</span>
          <el-button
            link
            type="primary"
            @click="resetSubscription"
          >
            <el-icon><Refresh /></el-icon>
            重置链接
          </el-button>
        </div>
      </template>

      <div class="subscription-content">
        <!-- 订阅格式选择 -->
        <div class="format-selector">
          <span class="selector-label">订阅格式：</span>
          <el-radio-group
            v-model="selectedFormat"
            size="small"
          >
            <el-radio-button 
              v-for="format in formats" 
              :key="format.value" 
              :value="format.value"
            >
              {{ format.label }}
            </el-radio-button>
          </el-radio-group>
        </div>

        <!-- 订阅链接 -->
        <div class="subscription-url">
          <el-input
            v-model="subscriptionUrl"
            readonly
            size="large"
          >
            <template #append>
              <el-button @click="copyUrl">
                <el-icon><CopyDocument /></el-icon>
                复制
              </el-button>
            </template>
          </el-input>
        </div>

        <!-- QR 码 -->
        <div class="qrcode-section">
          <div class="qrcode-wrapper">
            <canvas ref="qrcodeCanvas" />
          </div>
          <div class="qrcode-actions">
            <el-button
              size="small"
              @click="downloadQRCode"
            >
              <el-icon><Download /></el-icon>
              下载二维码
            </el-button>
          </div>
        </div>

        <!-- 使用说明 -->
        <el-alert
          type="info"
          :closable="false"
          show-icon
          class="usage-tip"
        >
          <template #title>
            <span>使用说明</span>
          </template>
          <template #default>
            <p>1. 复制上方订阅链接或扫描二维码</p>
            <p>2. 在客户端中添加订阅</p>
            <p>3. 更新订阅获取最新节点</p>
          </template>
        </el-alert>
      </div>
    </el-card>

    <el-card
      v-else
      class="subscription-card subscription-card--empty"
      shadow="never"
    >
      <el-empty description="当前暂无可用订阅链接">
        <template #description>
          <p class="subscription-empty__description">
            {{ noEntitlementMessage }}
          </p>
        </template>
        <el-button
          type="primary"
          @click="goToPlans"
        >
          <el-icon><ShoppingCart /></el-icon>
          {{ secondaryPlanActionLabel }}
        </el-button>
      </el-empty>
    </el-card>

    <!-- 客户端推荐 -->
    <el-card
      class="clients-card"
      shadow="never"
    >
      <template #header>
        <div class="card-header">
          <span>推荐客户端</span>
          <el-button
            link
            type="primary"
            @click="goToDownload"
          >
            查看全部
            <el-icon><ArrowRight /></el-icon>
          </el-button>
        </div>
      </template>

      <div class="clients-grid">
        <div 
          v-for="client in recommendedClients" 
          :key="client.name"
          class="client-item"
          @click="openClientLink(client)"
        >
          <div class="client-icon">
            <el-icon><component :is="client.icon" /></el-icon>
          </div>
          <div class="client-info">
            <h4 class="client-name">
              {{ client.name }}
            </h4>
            <span class="client-platform">{{ client.platform }}</span>
          </div>
          <el-icon class="client-arrow">
            <ArrowRight />
          </el-icon>
        </div>
      </div>
    </el-card>

    <!-- 重置确认对话框 -->
    <el-dialog
      v-model="showResetDialog"
      title="重置订阅链接"
      :width="isMobile ? 'calc(100vw - 24px)' : '400px'"
    >
      <el-alert
        type="warning"
        :closable="false"
        show-icon
      >
        <template #title>
          重置后，旧的订阅链接将失效，您需要在所有客户端中更新订阅链接。
        </template>
      </el-alert>
      <template #footer>
        <el-button @click="showResetDialog = false">
          取消
        </el-button>
        <el-button
          type="danger"
          :loading="resetting"
          @click="confirmReset"
        >
          确认重置
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { 
  Refresh, CopyDocument, Download, ArrowRight,
  Monitor, Iphone, Apple, TrendCharts, ShoppingCart
} from '@element-plus/icons-vue'
import { useUserPortalStore } from '@/stores/userPortal'
import { useSubscriptionStore } from '@/stores/subscription'
import { usePauseStore } from '@/stores/pause'
import PauseCard from '@/components/user/PauseCard.vue'
import QRCode from 'qrcode'
import { copyText } from '@/utils/clipboard'
import { useViewport } from '@/composables/useViewport'
import { extractErrorMessage, getNoEntitlementMessage, isNoEntitlementError } from '@/utils/entitlement'
import { formatTrafficBytes, formatTrafficLimit } from '@/utils/traffic'

const router = useRouter()
const userStore = useUserPortalStore()
const subscriptionStore = useSubscriptionStore()
const pauseStore = usePauseStore()
const { isMobile } = useViewport()

// 引用
const qrcodeCanvas = ref(null)

// 状态
const selectedFormat = ref('clashmeta')
const showResetDialog = ref(false)
const resetting = ref(false)
const loading = ref(false)
const noEntitlement = ref(false)
const subscriptionUpdatedAt = ref(null)
const subscriptionRefreshInFlight = ref(false)
const SUBSCRIPTION_REFRESH_INTERVAL = 30 * 1000
let subscriptionRefreshTimer = null

// 订阅格式
// Clash Meta/Mihomo 放第一位，覆盖 Clash Verge/Stash 等现代 Clash 内核；
// 基础 Clash 只支持 VMess/Trojan/SS，VLESS 与 Reality 节点会被丢弃
const formats = [
  { value: 'clashmeta', label: 'Clash Meta' },
  { value: 'clash', label: 'Clash' },
  { value: 'singbox', label: 'Sing-box' },
  { value: 'v2rayn', label: 'V2Ray' },
  { value: 'shadowrocket', label: 'Shadowrocket' },
  { value: 'surge', label: 'Surge' },
  { value: 'quantumultx', label: 'Quantumult X' }
]

// 推荐客户端
const recommendedClients = [
  { name: 'Clash Verge', platform: 'Windows / macOS / Linux', icon: Monitor, url: 'https://github.com/clash-verge-rev/clash-verge-rev' },
  { name: 'Shadowrocket', platform: 'iOS', icon: Iphone, url: 'https://apps.apple.com/app/shadowrocket/id932747118' },
  { name: 'ClashX Pro', platform: 'macOS', icon: Apple, url: 'https://install.appcenter.ms/users/clashx/apps/clashx-pro/distribution_groups/public' }
]

// 计算属性
const subscriptionUrl = computed(() => {
  if (!subscriptionStore.link) return ''
  const url = new URL(subscriptionStore.link)
  url.searchParams.set('format', selectedFormat.value)
  return url.toString()
})

const subscriptionStatus = computed(() => {
  if (pauseStore.cannotPauseReason === '当前无有效订阅') {
    return { type: 'info', label: '无有效订阅' }
  }
  if (userStore.hasActiveTrial && !userStore.hasActiveSubscription) {
    return { type: 'success', label: '试用中' }
  }
  const status = userStore.status
  if (status === 'active') return { type: 'success', label: '正常' }
  if (status === 'expired') return { type: 'warning', label: '已过期' }
  return { type: 'danger', label: '已禁用' }
})

const hasCurrentPlan = computed(() => userStore.hasActiveSubscription)
const hasTrialEntitlement = computed(() => userStore.hasActiveTrial)
const hasAnyEntitlement = computed(() => userStore.hasEntitlement)
const isExpiredEntitlement = computed(() => hasAnyEntitlement.value && userStore.status === 'expired')
const expiryDisplayText = computed(() => {
  if (!hasAnyEntitlement.value) return '未开通套餐'
  if (!userStore.expiresAt) return '永久有效'
  return new Date(userStore.expiresAt).toLocaleDateString('zh-CN')
})
const daysUntilExpiry = computed(() => userStore.daysUntilExpiry)
const trafficUsed = computed(() => userStore.trafficUsed)
const trafficLimit = computed(() => userStore.trafficLimit)
const trafficUsageDisplay = computed(() => `${formatTrafficBytes(trafficUsed.value)} / ${formatTrafficLimit(trafficLimit.value)}`)
const availableNodes = computed(() => userStore.availableNodes || 0)
const secondaryPlanActionLabel = computed(() => {
  if (hasCurrentPlan.value) return '续费套餐'
  if (hasTrialEntitlement.value) return '升级为正式套餐'
  return '购买套餐'
})
const noEntitlementMessage = computed(() => {
  if (!hasAnyEntitlement.value) return '当前暂无可用订阅链接，请先购买套餐。'
  if (isExpiredEntitlement.value) return '当前套餐已过期，续费后即可恢复订阅链接和节点使用。'
  return getNoEntitlementMessage('subscription')
})

const subscriptionRefreshHint = computed(() => {
  const baseHint = '约每 30 秒自动刷新'
  if (!subscriptionUpdatedAt.value) return baseHint

  const updatedAt = subscriptionUpdatedAt.value instanceof Date
    ? subscriptionUpdatedAt.value
    : new Date(subscriptionUpdatedAt.value)

  if (Number.isNaN(updatedAt.getTime())) {
    return baseHint
  }

  return `${updatedAt.toLocaleTimeString('zh-CN', { hour12: false })} 更新 · ${baseHint}`
})

// 方法
async function loadSubscription(options = {}) {
  if (subscriptionRefreshInFlight.value) return

  const { silent = false, includePauseStatus = !silent } = options
  if (!silent) {
    loading.value = true
  }

  try {
    subscriptionRefreshInFlight.value = true
    await userStore.fetchProfile({ silent })
    if (includePauseStatus) {
      try {
        await pauseStore.fetchPauseStatus()
      } catch (pauseError) {
        console.warn('加载暂停状态失败:', pauseError)
      }
    }
    await subscriptionStore.fetchLink()
    noEntitlement.value = false
    subscriptionUpdatedAt.value = new Date()
    await generateQRCode()
  } catch (error) {
    console.error('加载订阅失败:', error)
    if (isNoEntitlementError(error)) {
      noEntitlement.value = true
      subscriptionUpdatedAt.value = null
      subscriptionStore.clearSubscription()
    } else if (!silent) {
      noEntitlement.value = false
      ElMessage.error(extractErrorMessage(error) || '加载订阅信息失败')
    }
  } finally {
    subscriptionRefreshInFlight.value = false
    if (!silent) {
      loading.value = false
    }
  }
}

async function generateQRCode() {
  await nextTick()
  if (qrcodeCanvas.value && subscriptionUrl.value) {
    try {
      await QRCode.toCanvas(qrcodeCanvas.value, subscriptionUrl.value, {
        width: 200,
        margin: 2,
        color: {
          dark: '#303133',
          light: '#ffffff'
        }
      })
    } catch (error) {
      console.error('生成二维码失败:', error)
    }
  }
}

async function copyUrl() {
  if (!subscriptionUrl.value) {
    ElMessage.warning('订阅链接未加载')
    return
  }

  try {
    await copyText(subscriptionUrl.value)
    ElMessage.success('订阅链接已复制')
  } catch (error) {
    console.error('复制订阅链接失败:', error)
    ElMessage.error('复制失败，请手动复制链接')
  }
}

function downloadQRCode() {
  if (!qrcodeCanvas.value) return
  
  const link = document.createElement('a')
  link.download = `subscription-${selectedFormat.value}.png`
  link.href = qrcodeCanvas.value.toDataURL('image/png')
  link.click()
  
  ElMessage.success('二维码已下载')
}

function resetSubscription() {
  showResetDialog.value = true
}

async function confirmReset() {
  resetting.value = true
  try {
    await subscriptionStore.regenerate()
    showResetDialog.value = false
    ElMessage.success('订阅链接已重置')
    await generateQRCode()
  } catch (error) {
    console.error('重置订阅失败:', error)
    ElMessage.error(extractErrorMessage(error) || '重置失败')
  } finally {
    resetting.value = false
  }
}

function goToDownload() {
  router.push('/user/download')
}

function handlePrimaryPlanAction() {
  if (hasCurrentPlan.value) {
    goToPlanUpgrade()
    return
  }

  goToPlans()
}

function goToPlanUpgrade() {
  router.push({ name: 'user-plan-upgrade' }).catch(error => {
    console.error('跳转到套餐升降级页面失败:', error)
  })
}

function goToPlans() {
  router.push('/user/plans').catch(error => {
    console.error('跳转到套餐页面失败:', error)
  })
}

function openClientLink(client) {
  if (client.url && client.url !== '#') {
    window.open(client.url, '_blank')
  } else {
    router.push('/user/download')
  }
}

function startSubscriptionAutoRefresh() {
  stopSubscriptionAutoRefresh()
  subscriptionRefreshTimer = window.setInterval(() => {
    if (document.visibilityState === 'hidden') return
    loadSubscription({ silent: true, includePauseStatus: false })
  }, SUBSCRIPTION_REFRESH_INTERVAL)
}

function stopSubscriptionAutoRefresh() {
  if (subscriptionRefreshTimer !== null) {
    clearInterval(subscriptionRefreshTimer)
    subscriptionRefreshTimer = null
  }
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') {
    loadSubscription({ silent: true, includePauseStatus: false })
  }
}

// 监听格式变化重新生成二维码
watch(selectedFormat, () => {
  generateQRCode()
})

onMounted(() => {
  loadSubscription()
  startSubscriptionAutoRefresh()
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  stopSubscriptionAutoRefresh()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<style scoped>
.subscription-page {
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

.page-hint {
  font-size: 12px;
  color: #909399;
  margin: 6px 0 0;
}

/* 订阅操作区域 */
.subscription-actions {
  margin-bottom: 20px;
}

.status-card {
  height: 100%;
  border-radius: 8px;
}

.status-card :deep(.el-card__body) {
  display: flex;
  min-height: 260px;
}

.status-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  justify-content: space-between;
  gap: 16px;
}

.days-hint {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-left: 8px;
}

.action-buttons {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

/* 卡片样式 */
.subscription-card,
.clients-card {
  margin-bottom: 20px;
  border-radius: 8px;
}

.subscription-card--empty :deep(.el-card__body) {
  padding: 36px 24px;
}

.subscription-empty__description {
  margin: 0;
  color: var(--color-text-secondary);
  line-height: 1.8;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

/* 订阅内容 */
.subscription-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.format-selector {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.selector-label {
  font-size: 14px;
  color: var(--color-text-regular);
}

.subscription-url {
  width: 100%;
}

/* 二维码 */
.qrcode-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: var(--color-border-light);
  border: 1px solid var(--color-border);
  border-radius: 8px;
}

.qrcode-wrapper {
  padding: 16px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.qrcode-wrapper canvas {
  display: block;
  max-width: min(100%, 220px);
  height: auto;
}

/* 使用说明 */
.usage-tip p {
  margin: 4px 0;
  font-size: 13px;
}

/* 客户端网格 */
.clients-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}

.client-item {
  display: flex;
  align-items: center;
  padding: 16px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
}

.client-item:hover {
  border-color: #409eff;
  background: var(--color-border-light);
}

.client-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(64, 158, 255, 0.12);
  border-radius: 8px;
  color: #409eff;
  font-size: 20px;
  margin-right: 12px;
}

.client-info {
  flex: 1;
}

.client-name {
  font-size: 15px;
  font-weight: 500;
  color: var(--color-text-primary);
  margin: 0 0 4px 0;
}

.client-platform {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.client-arrow {
  color: var(--color-text-placeholder);
}

/* 响应式 */
@media (max-width: 768px) {
  .subscription-page {
    padding: 12px;
  }

  .format-selector {
    flex-direction: column;
    align-items: flex-start;
  }

  .action-buttons > .el-button {
    width: 100%;
  }

  .selector-label {
    width: 100%;
  }

  .subscription-url :deep(.el-input-group__append .el-button) {
    padding-left: 12px;
    padding-right: 12px;
  }

  .clients-grid {
    grid-template-columns: 1fr;
  }

  .client-item {
    align-items: flex-start;
  }
}
</style>
