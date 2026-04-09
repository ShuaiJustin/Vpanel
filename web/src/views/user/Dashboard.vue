<template>
  <div class="dashboard-page">
    <!-- 欢迎区域 -->
    <div class="welcome-section">
      <div class="welcome-content">
        <h1 class="welcome-title">
          {{ greeting }}，{{ userStore.username || "用户" }}
        </h1>
        <p class="welcome-subtitle">{{ welcomeSubtitle }}</p>
      </div>
      <div class="welcome-actions">
        <el-button type="primary" @click="handlePrimaryAction">
          <el-icon><component :is="primaryActionIcon" /></el-icon>
          {{ primaryActionLabel }}
        </el-button>
      </div>
    </div>

    <!-- 状态卡片 -->
    <el-row :gutter="20" class="status-cards">
      <!-- 账户状态 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="status-card">
          <div class="card-icon" :class="accountStatusClass">
            <el-icon><User /></el-icon>
          </div>
          <div class="card-content">
            <div class="card-label">账户状态</div>
            <div class="card-value">
              <el-tag :type="accountStatusType" size="small">
                {{ accountStatusText }}
              </el-tag>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 到期时间 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="status-card">
          <div class="card-icon expiry">
            <el-icon><Calendar /></el-icon>
          </div>
          <div class="card-content">
            <div class="card-label">到期时间</div>
            <div class="card-value">
{{ expiryDisplayText }}
                <span
                  v-if="expiryHintText"
                  class="days-hint"
                >
                  ({{ expiryHintText }})
                </span>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 在线设备 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="status-card">
          <div class="card-icon devices">
            <el-icon><Monitor /></el-icon>
          </div>
          <div class="card-content">
            <div class="card-label">在线设备</div>
            <div class="card-value">
              {{ onlineDevices }} / {{ maxDevicesDisplay }}
            </div>
          </div>
        </div>
      </el-col>

      <!-- 可用节点 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="status-card">
          <div class="card-icon nodes">
            <el-icon><Connection /></el-icon>
          </div>
          <div class="card-content">
            <div class="card-label">可用节点</div>
            <div class="card-value">{{ availableNodes }} 个</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 流量使用 -->
    <el-row :gutter="20" class="main-content">
      <el-col :xs="24" :lg="16">
        <el-card class="traffic-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="card-header-info">
                <span>当前周期流量使用</span>
                <span class="card-header-hint">{{ trafficRefreshHint }}</span>
              </div>
              <el-button link type="primary" @click="goToStats">
                查看详情
                <el-icon><ArrowRight /></el-icon>
              </el-button>
            </div>
          </template>

          <div class="traffic-overview">
            <div class="traffic-progress">
              <el-progress
                type="dashboard"
                :percentage="trafficPercentValue"
                :width="160"
                :stroke-width="12"
                :color="trafficProgressColor"
              >
                <template #default>
                  <div class="progress-content">
                    <div class="progress-value">
                      {{ trafficPercentDisplay }}
                    </div>
                    <div class="progress-label">当前周期已用</div>
                  </div>
                </template>
              </el-progress>
            </div>

            <div class="traffic-details">
              <div class="traffic-item">
                <span class="item-label">当前周期已用</span>
                <span class="item-value">{{
                  formatTraffic(userStore.trafficUsed)
                }}</span>
              </div>
              <div class="traffic-item">
                <span class="item-label">当前周期总流量</span>
                <span class="item-value">{{ totalTrafficDisplay }}</span>
              </div>
              <div class="traffic-item">
                <span class="item-label">当前周期剩余流量</span>
                <span class="item-value">{{ remainingTrafficDisplay }}</span>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 快捷操作 -->
      <el-col :xs="24" :lg="8">
        <el-card class="quick-actions-card" shadow="never">
          <template #header>
            <span>快捷操作</span>
          </template>

          <div class="quick-actions">
            <div class="action-item" @click="goToNodes">
              <el-icon class="action-icon">
                <Connection />
              </el-icon>
              <span class="action-label">节点列表</span>
            </div>
            <div class="action-item" @click="handlePrimaryAction">
              <el-icon class="action-icon">
                <component :is="primaryActionIcon" />
              </el-icon>
              <span class="action-label">{{ primaryActionQuickLabel }}</span>
            </div>
            <div class="action-item" @click="goToDownload">
              <el-icon class="action-icon">
                <Download />
              </el-icon>
              <span class="action-label">客户端下载</span>
            </div>
            <div class="action-item" @click="goToDevices">
              <el-icon class="action-icon">
                <Monitor />
              </el-icon>
              <span class="action-label">在线设备</span>
            </div>
            <div class="action-item" @click="goToTickets">
              <el-icon class="action-icon">
                <ChatDotRound />
              </el-icon>
              <span class="action-label">工单支持</span>
            </div>
            <div class="action-item" @click="goToSettings">
              <el-icon class="action-icon">
                <Setting />
              </el-icon>
              <span class="action-label">个人设置</span>
            </div>
            <div class="action-item" @click="goToHelp">
              <el-icon class="action-icon">
                <QuestionFilled />
              </el-icon>
              <span class="action-label">帮助中心</span>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 公告区域 -->
    <el-card
      v-if="announcements.length > 0"
      class="announcements-card"
      shadow="never"
    >
      <template #header>
        <div class="card-header">
          <span>
            <el-icon><Bell /></el-icon>
            最新公告
          </span>
          <el-button link type="primary" @click="goToAnnouncements">
            查看全部
            <el-icon><ArrowRight /></el-icon>
          </el-button>
        </div>
      </template>

      <div class="announcement-list">
        <div
          v-for="item in announcements"
          :key="item.id"
          class="announcement-item"
          @click="viewAnnouncement(item.id)"
        >
          <el-tag
            :type="getCategoryType(item.category)"
            size="small"
            class="announcement-tag"
          >
            {{ getCategoryLabel(item.category) }}
          </el-tag>
          <span class="announcement-title">{{ item.title }}</span>
          <span class="announcement-date">{{
            formatDate(item.published_at)
          }}</span>
        </div>
      </div>
    </el-card>

    <!-- 试用和暂停状态卡片 -->
    <el-row :gutter="20" class="status-section">
      <el-col :xs="24" :lg="12">
        <TrialCard />
      </el-col>
      <el-col :xs="24" :lg="12">
        <PauseCard />
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { useRouter } from "vue-router";
import {
  User,
  Calendar,
  Monitor,
  Connection,
  Link,
  Download,
  ChatDotRound,
  Setting,
  QuestionFilled,
  Bell,
  ArrowRight,
  ShoppingCart,
} from "@element-plus/icons-vue";
import { useUserPortalStore } from "@/stores/userPortal";
import { usePortalAnnouncementsStore } from "@/stores/portalAnnouncements";
import PauseCard from "@/components/user/PauseCard.vue";
import TrialCard from "@/components/user/TrialCard.vue";
import api from "@/api/base";
import {
  formatRemainingTraffic,
  formatTrafficBytes,
  formatTrafficLimit,
  isUnlimitedTrafficLimit,
} from "@/utils/traffic";

const router = useRouter();
const userStore = useUserPortalStore();
const announcementsStore = usePortalAnnouncementsStore();

// 数据
const onlineDevices = ref(0);
const maxDevices = ref(0);
const trafficUpdatedAt = ref(null);
const trafficRefreshInFlight = ref(false);
const TRAFFIC_REFRESH_INTERVAL = 30 * 1000;
let trafficRefreshTimer = null;

// 计算属性
const greeting = computed(() => {
  const hour = new Date().getHours();
  if (hour < 6) return "夜深了";
  if (hour < 12) return "早上好";
  if (hour < 14) return "中午好";
  if (hour < 18) return "下午好";
  return "晚上好";
});

const hasCurrentPlan = computed(() => userStore.hasActiveSubscription);
const hasTrialEntitlement = computed(() => userStore.hasActiveTrial);
const hasAnyEntitlement = computed(() => userStore.hasEntitlement);
const isExpiredEntitlement = computed(() => hasAnyEntitlement.value && userStore.status === "expired");

const welcomeSubtitle = computed(() => {
  if (!hasAnyEntitlement.value) return "先购买套餐，即可获取订阅并开始使用节点";
  if (hasTrialEntitlement.value && !hasCurrentPlan.value) return "您正在使用试用订阅，可升级为正式套餐";
  if (isExpiredEntitlement.value) return "当前套餐已过期，续费后即可恢复订阅和节点使用";
  return "欢迎回来，这是您的账户概览";
});

const accountStatusClass = computed(() => {
  if (!hasAnyEntitlement.value) return "inactive";
  const status = userStore.status;
  if (status === "active") return "active";
  if (status === "expired") return "expired";
  return "disabled";
});

const accountStatusType = computed(() => {
  if (!hasAnyEntitlement.value) return "info";
  const status = userStore.status;
  if (status === "active") return "success";
  if (status === "expired") return "warning";
  return "danger";
});

const accountStatusText = computed(() => {
  if (!hasAnyEntitlement.value) return "无有效订阅";
  if (hasTrialEntitlement.value && !hasCurrentPlan.value) return "试用中";
  const status = userStore.status;
  if (status === "active") return "正常";
  if (status === "expired") return "已过期";
  if (status === "disabled") return "已禁用";
  return "未知";
});

const expiryDisplayText = computed(() => {
  if (!hasAnyEntitlement.value) return "未开通套餐";
  if (!userStore.expiresAt) return "永久有效";
  return formatDate(userStore.expiresAt);
});

const expiryHintText = computed(() => {
  if (!hasAnyEntitlement.value || userStore.daysUntilExpiry === null) return "";
  return userStore.daysUntilExpiry > 0 ? `剩余 ${userStore.daysUntilExpiry} 天` : "已过期";
});

const primaryActionLabel = computed(() => {
  if (!hasAnyEntitlement.value) return "购买套餐";
  if (hasTrialEntitlement.value && !hasCurrentPlan.value) return "升级套餐";
  if (isExpiredEntitlement.value) return "续费套餐";
  return "获取订阅";
});

const primaryActionQuickLabel = computed(() => {
  if (!hasAnyEntitlement.value) return "购买套餐";
  if (hasTrialEntitlement.value && !hasCurrentPlan.value) return "升级套餐";
  if (isExpiredEntitlement.value) return "续费套餐";
  return "订阅管理";
});

const primaryActionIcon = computed(() => {
  return !hasAnyEntitlement.value || isExpiredEntitlement.value || (hasTrialEntitlement.value && !hasCurrentPlan.value) ? ShoppingCart : Link;
});

const isUnlimitedTraffic = computed(() => isUnlimitedTrafficLimit(userStore.trafficLimit));

const totalTrafficDisplay = computed(() => {
  return formatTrafficLimit(userStore.trafficLimit);
});

const remainingTrafficDisplay = computed(() => {
  return formatRemainingTraffic(userStore.trafficLimit, userStore.trafficUsed);
});

const rawTrafficPercent = computed(() => {
  if (isUnlimitedTraffic.value || !userStore.trafficLimit) return 0;
  return Math.min(100, (userStore.trafficUsed / userStore.trafficLimit) * 100);
});

const trafficPercentValue = computed(() => {
  const percent = rawTrafficPercent.value;
  if (percent > 0 && percent < 0.1) return 0.1;
  return Number(percent.toFixed(percent < 10 ? 1 : 0));
});

const trafficPercentDisplay = computed(() => {
  if (isUnlimitedTraffic.value) return "不限";

  const percent = rawTrafficPercent.value;
  if (percent <= 0) return "0%";
  if (percent < 0.1) return "<0.1%";
  return `${percent < 10 ? percent.toFixed(1) : percent.toFixed(0)}%`;
});

const trafficRefreshHint = computed(() => {
  const baseHint = "约每 30 秒自动刷新";
  if (!trafficUpdatedAt.value) return baseHint;

  const updatedAt =
    trafficUpdatedAt.value instanceof Date
      ? trafficUpdatedAt.value
      : new Date(trafficUpdatedAt.value);

  if (Number.isNaN(updatedAt.getTime())) {
    return baseHint;
  }

  const updatedTime = updatedAt.toLocaleTimeString("zh-CN", { hour12: false });
  return `${updatedTime} 更新 · ${baseHint}`;
});

const trafficProgressColor = computed(() => {
  if (isUnlimitedTraffic.value) return "#67c23a";

  const percent = rawTrafficPercent.value;
  if (percent >= 90) return "#f56c6c";
  if (percent >= 70) return "#e6a23c";
  return "#409eff";
});

const announcements = computed(() => {
  return announcementsStore.announcements.slice(0, 5);
});

const maxDevicesDisplay = computed(() => {
  return maxDevices.value === 0 ? "无限制" : String(maxDevices.value);
});

const availableNodes = computed(() => userStore.availableNodes || 0);

// 方法
function formatDate(dateStr) {
  if (!dateStr) return "-";
  const date = new Date(dateStr);
  return date.toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
}

const formatTraffic = formatTrafficBytes;

function getCategoryType(category) {
  const types = {
    general: "info",
    maintenance: "warning",
    update: "success",
    promotion: "danger",
  };
  return types[category] || "info";
}

function getCategoryLabel(category) {
  const labels = {
    general: "公告",
    maintenance: "维护",
    update: "更新",
    promotion: "活动",
  };
  return labels[category] || "公告";
}

// 导航方法
function goToNodes() {
  router.push("/user/nodes");
}

function handlePrimaryAction() {
  if (!hasAnyEntitlement.value || isExpiredEntitlement.value || (hasTrialEntitlement.value && !hasCurrentPlan.value)) {
    goToPlans();
    return;
  }

  goToSubscription();
}

function goToPlans() {
  router.push("/user/plans");
}

function goToSubscription() {
  router.push("/user/subscription");
}

function goToDownload() {
  router.push("/user/download");
}

function goToDevices() {
  router.push("/user/devices");
}

function goToTickets() {
  router.push("/user/tickets");
}

function goToSettings() {
  router.push("/user/settings");
}

function goToHelp() {
  router.push("/user/help");
}

function goToStats() {
  router.push("/user/stats");
}

function goToAnnouncements() {
  router.push("/user/announcements");
}

function viewAnnouncement(id) {
  router.push(`/user/announcements/${id}`);
}

async function refreshTrafficData({ silent = true } = {}) {
  if (trafficRefreshInFlight.value) return;

  try {
    trafficRefreshInFlight.value = true;
    await userStore.fetchProfile({ silent });
    trafficUpdatedAt.value = new Date();
    return true;
  } catch (error) {
    console.error("Failed to refresh traffic data:", error);
    return false;
  } finally {
    trafficRefreshInFlight.value = false;
  }
}

function startTrafficAutoRefresh() {
  stopTrafficAutoRefresh();
  trafficRefreshTimer = window.setInterval(() => {
    if (document.visibilityState === "hidden") return;
    refreshTrafficData({ silent: true });
  }, TRAFFIC_REFRESH_INTERVAL);
}

function stopTrafficAutoRefresh() {
  if (trafficRefreshTimer !== null) {
    clearInterval(trafficRefreshTimer);
    trafficRefreshTimer = null;
  }
}

function handleVisibilityChange() {
  if (document.visibilityState === "visible") {
    refreshTrafficData({ silent: true });
  }
}

// 加载数据
async function loadDashboardData() {
  const [trafficResult, announcementsResult, devicesResult] =
    await Promise.allSettled([
      refreshTrafficData({ silent: false }),
      announcementsStore.fetchAnnouncements(),
      api.get("/user/devices"),
    ]);

  if (trafficResult.status === "rejected") {
    console.error("Failed to load dashboard traffic data:", trafficResult.reason);
  } else if (trafficResult.value === false) {
    console.error("Failed to load dashboard traffic data: refresh returned false");
  }

  if (announcementsResult.status === "rejected") {
    console.error(
      "Failed to load dashboard announcements:",
      announcementsResult.reason,
    );
  }

  if (devicesResult.status === "fulfilled") {
    const devicesResp = devicesResult.value;
    const devicesData = devicesResp?.data ?? devicesResp ?? {};
    const devices = Array.isArray(devicesData.devices)
      ? devicesData.devices
      : [];
    onlineDevices.value = Number(
      devicesData.current_count ?? devices.length ?? 0,
    );
    maxDevices.value = Number(
      devicesData.max_devices ?? devicesData.maxDevices ?? 0,
    );
  } else {
    console.error("Failed to load dashboard devices:", devicesResult.reason);
  }
}

onMounted(() => {
  loadDashboardData();
  startTrafficAutoRefresh();
  document.addEventListener("visibilitychange", handleVisibilityChange);
});

onBeforeUnmount(() => {
  stopTrafficAutoRefresh();
  document.removeEventListener("visibilitychange", handleVisibilityChange);
});
</script>

<style scoped>
.dashboard-page {
  padding: 20px;
}

/* 欢迎区域 */
.welcome-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  padding: 24px;
  background: linear-gradient(135deg, #409eff 0%, #66b1ff 100%);
  border-radius: 12px;
  color: #fff;
}

.welcome-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 8px 0;
}

.welcome-subtitle {
  font-size: 14px;
  opacity: 0.9;
  margin: 0;
}

.welcome-actions .el-button {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.3);
  color: #fff;
}

.welcome-actions .el-button:hover {
  background: rgba(255, 255, 255, 0.3);
}

/* 状态卡片 */
.status-cards {
  margin-bottom: 20px;
}

.status-card {
  display: flex;
  align-items: center;
  min-height: 108px;
  padding: 20px;
  background: var(--color-bg-card);
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  margin-bottom: 20px;
}

.card-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  margin-right: 16px;
}

.card-icon.inactive {
  background: rgba(144, 147, 153, 0.1);
  color: #909399;
}

.card-icon.active {
  background: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.card-icon.expired {
  background: rgba(230, 162, 60, 0.1);
  color: #e6a23c;
}

.card-icon.disabled {
  background: rgba(245, 108, 108, 0.1);
  color: #f56c6c;
}

.card-icon.expiry {
  background: rgba(64, 158, 255, 0.1);
  color: #409eff;
}

.card-icon.devices {
  background: rgba(144, 147, 153, 0.1);
  color: #909399;
}

.card-icon.nodes {
  background: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.card-label {
  font-size: 13px;
  color: #909399;
  margin-bottom: 4px;
}

.card-value {
  font-size: 16px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.days-hint {
  font-size: 12px;
  color: #909399;
  font-weight: normal;
}

/* 主内容区 */
.main-content {
  margin-bottom: 20px;
}

.traffic-card,
.quick-actions-card,
.announcements-card {
  height: 100%;
  border-radius: 8px;
}

.traffic-card :deep(.el-card__body),
.quick-actions-card :deep(.el-card__body) {
  display: flex;
  min-height: 280px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.card-header-hint {
  font-size: 12px;
  color: #909399;
  font-weight: 400;
}

/* 流量卡片 */
.traffic-overview {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 40px;
}

.traffic-progress {
  flex-shrink: 0;
}

.progress-content {
  text-align: center;
}

.progress-value {
  font-size: 28px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.progress-label {
  font-size: 13px;
  color: #909399;
}

.traffic-details {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.traffic-item {
  display: flex;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid #ebeef5;
}

.traffic-item:last-child {
  border-bottom: none;
}

.item-label {
  color: #909399;
}

.item-value {
  font-weight: 500;
  color: var(--color-text-primary);
}

/* 快捷操作 */
.quick-actions {
  display: grid;
  flex: 1;
  align-content: start;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-auto-rows: minmax(88px, 1fr);
  gap: 16px;
}

.action-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 88px;
  padding: 16px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
}

.action-item:hover {
  background: var(--color-bg-page);
}

.action-icon {
  font-size: 28px;
  color: #409eff;
  margin-bottom: 8px;
}

.action-label {
  font-size: 13px;
  color: #606266;
}

/* 公告列表 */
.announcements-card {
  margin-top: 20px;
}

.announcements-card .card-header span {
  display: flex;
  align-items: center;
  gap: 8px;
}

.announcement-list {
  display: flex;
  flex-direction: column;
}

.announcement-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #ebeef5;
  cursor: pointer;
  transition: background 0.3s;
}

.announcement-item:last-child {
  border-bottom: none;
}

.announcement-item:hover {
  background: var(--color-bg-page);
  margin: 0 -20px;
  padding: 12px 20px;
}

.announcement-tag {
  flex-shrink: 0;
  margin-right: 12px;
}

.announcement-title {
  flex: 1;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.announcement-date {
  flex-shrink: 0;
  font-size: 13px;
  color: #909399;
  margin-left: 12px;
}

/* 试用和暂停状态区域 */
.status-section {
  margin-top: 20px;
}

/* 响应式 */
@media (max-width: 768px) {
  .welcome-section {
    flex-direction: column;
    text-align: center;
    gap: 16px;
  }

  .traffic-overview {
    flex-direction: column;
    gap: 24px;
  }

  .traffic-card :deep(.el-card__body),
  .quick-actions-card :deep(.el-card__body) {
    min-height: auto;
  }

  .quick-actions {
    grid-template-columns: repeat(2, 1fr);
    grid-auto-rows: minmax(80px, auto);
  }
}

/* 深色模式适配 */
:global(.dark) .status-card {
  background: var(--color-bg-card);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.3);
}

:global(.dark) .card-value {
  color: var(--color-text-primary);
}

:global(.dark) .item-value {
  color: var(--color-text-primary);
}

:global(.dark) .progress-value {
  color: var(--color-text-primary);
}

:global(.dark) .announcement-title {
  color: var(--color-text-primary);
}

:global(.dark) .action-item:hover {
  background: var(--color-border-light);
}

:global(.dark) .announcement-item:hover {
  background: var(--color-border-light);
}

:global(.dark) .traffic-item {
  border-bottom: 1px solid var(--color-border);
}

:global(.dark) .announcement-item {
  border-bottom: 1px solid var(--color-border);
}
</style>
