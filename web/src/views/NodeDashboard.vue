<template>
  <div class="node-dashboard-page">
    <div class="page-header">
      <div class="header-copy">
        <p class="page-eyebrow">
          Node Cluster
        </p>
        <div class="title-row">
          <h1 class="page-title">
            节点集群概览
          </h1>
          <el-tag
            size="small"
            effect="plain"
            class="refresh-tag"
          >
            自动刷新 {{ autoRefreshSeconds }}s
          </el-tag>
        </div>
        <p class="page-subtitle">
          集中查看节点健康率、负载分布、分组覆盖和待处理风险。
          <span v-if="lastUpdatedText">上次更新 {{ lastUpdatedText }}</span>
        </p>
      </div>

      <div class="header-actions">
        <el-button
          :loading="loading"
          @click="refreshData"
        >
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button @click="router.push('/admin/node-groups')">
          节点分组
        </el-button>
        <el-button
          type="primary"
          @click="router.push('/admin/nodes')"
        >
          管理节点
        </el-button>
      </div>
    </div>

    <el-card
      shadow="never"
      class="hero-card"
    >
      <div class="hero-layout">
        <div class="hero-main">
          <div class="hero-status">
            <div
              class="status-orb"
              :class="healthTone"
            >
              <el-icon class="hero-status-icon">
                <CircleCheck v-if="healthStatus === 'healthy'" />
                <Warning v-else-if="healthStatus === 'warning'" />
                <CircleClose v-else />
              </el-icon>
            </div>
            <div class="hero-copy">
              <div class="hero-kicker">
                集群状态
              </div>
              <div class="hero-title">
                {{ healthStatusText }}
              </div>
              <div class="hero-note">
                {{ healthSummaryText }}
              </div>
            </div>
          </div>

          <div class="hero-highlights">
            <div class="highlight-chip">
              <span class="chip-label">在线节点</span>
              <strong>{{ onlineCount }}/{{ totalNodes }}</strong>
            </div>
            <div class="highlight-chip">
              <span class="chip-label">需处理</span>
              <strong>{{ attentionCount }}</strong>
            </div>
            <div class="highlight-chip">
              <span class="chip-label">活跃用户</span>
              <strong>{{ nodeStore.totalUsers }}</strong>
            </div>
          </div>
        </div>

        <div class="hero-side">
          <div class="hero-percent">
            {{ healthPercentage }}%
          </div>
          <div class="hero-percent-label">
            健康率
          </div>
          <el-progress
            :percentage="healthPercentage"
            :show-text="false"
            :stroke-width="8"
            :color="progressColor"
            class="hero-progress"
          />
        </div>
      </div>
    </el-card>

    <div class="metrics-grid">
      <el-card
        shadow="never"
        class="metric-card metric-card--good"
      >
        <div class="metric-top">
          <div>
            <div class="metric-label">
              健康节点
            </div>
            <div class="metric-value">
              {{ onlineCount }}
            </div>
          </div>
          <div class="metric-icon-shell">
            <el-icon><CircleCheck /></el-icon>
          </div>
        </div>
        <div class="metric-foot">
          占总节点 {{ formatPercent(totalNodes ? onlineCount / totalNodes : 0) }}
        </div>
      </el-card>

      <el-card
        shadow="never"
        class="metric-card"
      >
        <div class="metric-top">
          <div>
            <div class="metric-label">
              总节点数
            </div>
            <div class="metric-value">
              {{ totalNodes }}
            </div>
          </div>
          <div class="metric-icon-shell">
            <el-icon><Monitor /></el-icon>
          </div>
        </div>
        <div class="metric-foot">
          在线 {{ onlineCount }} / 离线 {{ offlineCount }} / 异常 {{ unhealthyCount }}
        </div>
      </el-card>

      <el-card
        shadow="never"
        class="metric-card"
      >
        <div class="metric-top">
          <div>
            <div class="metric-label">
              活跃用户
            </div>
            <div class="metric-value">
              {{ nodeStore.totalUsers }}
            </div>
          </div>
          <div class="metric-icon-shell">
            <el-icon><User /></el-icon>
          </div>
        </div>
        <div class="metric-foot">
          高负载节点 {{ highLoadNodes.length }} 个
        </div>
      </el-card>

      <el-card
        shadow="never"
        class="metric-card metric-card--latency"
      >
        <div class="metric-top">
          <div>
            <div class="metric-label">
              平均延迟
            </div>
            <div class="metric-value">
              {{ nodeStore.averageLatency }}ms
            </div>
          </div>
          <div class="metric-icon-shell">
            <el-icon><Timer /></el-icon>
          </div>
        </div>
        <div class="metric-foot">
          高延迟节点 {{ highLatencyNodes.length }} 个
        </div>
      </el-card>
    </div>

    <div class="dashboard-grid">
      <div class="dashboard-main">
        <el-card
          shadow="never"
          class="panel-card"
        >
          <template #header>
            <div class="card-header">
              <div>
                <div class="card-title">
                  运行分布
                </div>
                <div class="card-subtitle">
                  按节点运行状态快速判断集群稳定性
                </div>
              </div>
            </div>
          </template>

          <div class="status-track">
            <div
              v-for="segment in statusSegments"
              :key="segment.key"
              class="status-track-segment"
              :class="`status-track-segment--${segment.key}`"
              :style="{ width: `${segment.percentage || 0}%` }"
            />
          </div>

          <div class="status-grid">
            <article
              v-for="segment in statusSegments"
              :key="segment.key"
              class="status-stat"
              :class="`status-stat--${segment.key}`"
            >
              <div class="status-stat-head">
                <span class="status-dot" />
                <span>{{ segment.label }}</span>
              </div>
              <div class="status-stat-value">
                {{ segment.count }}
              </div>
              <div class="status-stat-meta">
                {{ segment.percentage }}%
              </div>
            </article>
          </div>
        </el-card>

        <el-card
          shadow="never"
          class="panel-card"
        >
          <template #header>
            <div class="card-header">
              <div>
                <div class="card-title">
                  重点节点
                </div>
                <div class="card-subtitle">
                  优先显示存在风险或当前负载更高的节点
                </div>
              </div>

              <el-radio-group
                v-model="nodeFilter"
                size="small"
              >
                <el-radio-button label="focus">
                  重点
                </el-radio-button>
                <el-radio-button label="online">
                  在线
                </el-radio-button>
                <el-radio-button label="all">
                  全部
                </el-radio-button>
              </el-radio-group>
            </div>
          </template>

          <div
            v-if="focusNodes.length"
            class="focus-list"
          >
            <article
              v-for="node in focusNodes"
              :key="node.id"
              class="focus-node"
              :class="{ 'focus-node--issue': node.issues.length > 0 }"
            >
              <div class="focus-node-head">
                <div>
                  <el-link
                    type="primary"
                    class="focus-node-link"
                    @click="router.push(`/admin/nodes/${node.id}`)"
                  >
                    {{ node.name }}
                  </el-link>
                  <div class="focus-node-meta">
                    {{ node.region || "未标记地区" }} · {{ node.address }}:{{ node.port }}
                  </div>
                </div>

                <div class="focus-node-badges">
                  <el-tag
                    :type="getStatusType(node.status)"
                    size="small"
                  >
                    {{ getStatusText(node.status) }}
                  </el-tag>
                  <el-tag
                    v-if="node.sync_status && node.sync_status !== 'synced'"
                    type="warning"
                    size="small"
                  >
                    {{ getSyncStatusText(node.sync_status) }}
                  </el-tag>
                </div>
              </div>

              <div
                v-if="node.issues.length"
                class="issue-list"
              >
                <span
                  v-for="issue in node.issues"
                  :key="`${node.id}-${issue.label}`"
                  class="issue-pill"
                  :class="`issue-pill--${issue.level}`"
                >
                  {{ issue.label }}
                </span>
              </div>

              <div class="focus-node-load">
                <div class="metric-row">
                  <span>负载</span>
                  <strong>{{ formatLoadLabel(node) }}</strong>
                </div>
                <el-progress
                  :percentage="node.capacityLimited ? node.loadPercentage : 0"
                  :show-text="false"
                  :stroke-width="8"
                  :color="node.loadPercentage >= 85 ? '#f56c6c' : node.loadPercentage >= 65 ? '#e6a23c' : '#3b82f6'"
                />
              </div>

              <div class="focus-node-stats">
                <div class="focus-stat">
                  <span class="focus-stat-label">延迟</span>
                  <span
                    class="focus-stat-value"
                    :class="getLatencyClass(node.latency)"
                  >
                    {{ node.latency ? `${node.latency}ms` : "未上报" }}
                  </span>
                </div>
                <div class="focus-stat">
                  <span class="focus-stat-label">内核</span>
                  <span
                    class="focus-stat-value"
                    :title="node.xray_version || '未上报'"
                  >
                    {{ formatCoreVersionCompact(node.xray_version) }}
                  </span>
                </div>
                <div class="focus-stat">
                  <span class="focus-stat-label">最近心跳</span>
                  <span class="focus-stat-value">
                    {{ formatRelativeTime(node.last_seen_at || node.updated_at) }}
                  </span>
                </div>
              </div>
            </article>
          </div>

          <el-empty
            v-else
            description="当前没有可展示的节点"
            :image-size="72"
          />
        </el-card>
      </div>

      <div class="dashboard-side">
        <el-card
          shadow="never"
          class="panel-card"
        >
          <template #header>
            <div class="card-header">
              <div>
                <div class="card-title">
                  运维提醒
                </div>
                <div class="card-subtitle">
                  按风险优先级整理当前需要处理的事项
                </div>
              </div>
            </div>
          </template>

          <div
            v-if="alertItems.length"
            class="alert-list"
          >
            <article
              v-for="alert in alertItems"
              :key="alert.id"
              class="alert-card"
              :class="`alert-card--${alert.level}`"
            >
              <div class="alert-icon-shell">
                <el-icon>
                  <CircleClose v-if="alert.level === 'error'" />
                  <Warning v-else />
                </el-icon>
              </div>

              <div class="alert-copy">
                <div class="alert-title">
                  {{ alert.message }}
                </div>
                <div class="alert-meta">
                  {{ alert.nodeName }}
                  <span v-if="alert.time">· {{ formatRelativeTime(alert.time) }}</span>
                </div>
              </div>
            </article>
          </div>

          <el-empty
            v-else
            description="暂无需要处理的告警"
            :image-size="72"
          />
        </el-card>

        <el-card
          shadow="never"
          class="panel-card"
        >
          <template #header>
            <div class="card-header">
              <div>
                <div class="card-title">
                  今日流量
                </div>
                <div class="card-subtitle">
                  按自然日聚合当前全部节点上报流量
                </div>
              </div>
            </div>
          </template>

          <div class="traffic-summary">
            <div class="traffic-pill">
              <span class="traffic-label">上传</span>
              <strong>{{ formatBytes(trafficSummary.upload) }}</strong>
            </div>
            <div class="traffic-pill">
              <span class="traffic-label">下载</span>
              <strong>{{ formatBytes(trafficSummary.download) }}</strong>
            </div>
            <div class="traffic-pill traffic-pill--total">
              <span class="traffic-label">总计</span>
              <strong>{{ formatBytes(trafficSummary.total) }}</strong>
            </div>
          </div>

          <div class="traffic-stack">
            <div
              class="traffic-stack-upload"
              :style="{ width: `${uploadShare}%` }"
            />
            <div
              class="traffic-stack-download"
              :style="{ width: `${downloadShare}%` }"
            />
          </div>

          <div class="traffic-foot">
            <span>上传占比 {{ uploadShare }}%</span>
            <span>下载占比 {{ downloadShare }}%</span>
          </div>
        </el-card>

        <el-card
          shadow="never"
          :class="['panel-card', { 'panel-card--compact-empty': !groupHighlights.length }]"
        >
          <template #header>
            <div class="card-header">
              <div>
                <div class="card-title">
                  分组覆盖
                </div>
                <div class="card-subtitle">
                  查看核心分组承载的节点与用户规模
                </div>
              </div>
              <el-button
                link
                @click="router.push('/admin/node-groups')"
              >
                查看全部
              </el-button>
            </div>
          </template>

          <div
            v-if="groupHighlights.length"
            class="group-list"
          >
            <article
              v-for="group in groupHighlights"
              :key="group.id"
              class="group-card"
              @click="router.push('/admin/node-groups')"
            >
              <div class="group-card-head">
                <div>
                  <div class="group-name">
                    {{ group.name }}
                  </div>
                  <div class="group-meta">
                    {{ group.region || "未标记地区" }} · {{ formatStrategy(group.strategy) }}
                  </div>
                </div>
                <div class="group-pill">
                  {{ group.nodeCount }} 节点
                </div>
              </div>

              <div class="group-metric">
                <div class="metric-row">
                  <span>节点覆盖</span>
                  <strong>{{ formatPercent(totalNodes ? group.nodeCount / totalNodes : 0) }}</strong>
                </div>
                <el-progress
                  :percentage="totalNodes ? Math.round((group.nodeCount / totalNodes) * 100) : 0"
                  :show-text="false"
                  :stroke-width="6"
                  color="#3b82f6"
                />
              </div>

              <div class="group-card-foot">
                <span>健康 {{ group.healthyCount }}</span>
                <span>用户 {{ group.userCount }}</span>
              </div>
            </article>
          </div>

          <div
            v-else
            class="compact-empty-state"
          >
            <el-empty
              description="暂无分组数据"
              :image-size="56"
            />
            <el-button
              plain
              @click="router.push('/admin/node-groups')"
            >
              前往节点分组
            </el-button>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import {
  Refresh,
  CircleCheck,
  Warning,
  CircleClose,
  User,
  Timer,
  Monitor,
} from "@element-plus/icons-vue";
import { useNodeStore } from "@/stores/node";
import { useNodeGroupStore } from "@/stores/nodeGroup";

const router = useRouter();
const nodeStore = useNodeStore();
const groupStore = useNodeGroupStore();

const autoRefreshSeconds = 30;
const loading = ref(false);
const nodeFilter = ref("focus");
const trafficStats = ref({ upload: 0, download: 0, total: 0 });
const lastUpdatedAt = ref(null);

let refreshInterval = null;

const clusterHealth = computed(() => nodeStore.clusterHealth || {});
const totalNodes = computed(() =>
  Number(clusterHealth.value.total_nodes ?? nodeStore.nodeCount ?? 0),
);
const onlineCount = computed(() =>
  Number(clusterHealth.value.online_nodes ?? nodeStore.onlineCount ?? 0),
);
const offlineCount = computed(() =>
  Number(clusterHealth.value.offline_nodes ?? nodeStore.offlineNodes.length ?? 0),
);
const unhealthyCount = computed(() =>
  Number(
    clusterHealth.value.unhealthy_nodes ?? nodeStore.unhealthyNodes.length ?? 0,
  ),
);
const attentionCount = computed(() => offlineCount.value + unhealthyCount.value);
const healthPercentage = computed(() =>
  Math.round(
    Number(clusterHealth.value.health_percentage ?? nodeStore.healthyPercentage ?? 0),
  ),
);

const trafficSummary = computed(() => {
  const raw = trafficStats.value?.stats || trafficStats.value || {};
  const upload = Number(raw.upload || 0);
  const download = Number(raw.download || 0);
  const total = Number(raw.total || upload + download);

  return { upload, download, total };
});

const uploadShare = computed(() => {
  if (!trafficSummary.value.total) return 0;
  return Math.round((trafficSummary.value.upload / trafficSummary.value.total) * 100);
});

const downloadShare = computed(() => {
  if (!trafficSummary.value.total) return 0;
  return 100 - uploadShare.value;
});

const lastUpdatedText = computed(() =>
  lastUpdatedAt.value ? formatRelativeTime(lastUpdatedAt.value) : "",
);

const healthStatus = computed(() => {
  const overallStatus = clusterHealth.value.overall_status;

  if (overallStatus === "healthy") return "healthy";
  if (overallStatus === "partial" || overallStatus === "degraded") return "warning";
  if (overallStatus === "critical") return "error";
  if (overallStatus === "no_nodes") return "error";

  if (totalNodes.value === 0) return "error";
  if (unhealthyCount.value > 0) return "warning";
  if (offlineCount.value > 0) return "warning";
  return "healthy";
});

const healthTone = computed(() => {
  if (healthStatus.value === "healthy") return "good";
  if (healthStatus.value === "warning") return "warning";
  return "danger";
});

const healthStatusText = computed(() => {
  const textMap = {
    healthy: "运行正常",
    warning: "存在风险",
    error: totalNodes.value === 0 ? "尚未接入节点" : "需要立即处理",
  };
  return textMap[healthStatus.value] || "未知";
});

const healthSummaryText = computed(() => {
  if (totalNodes.value === 0) {
    return "当前还没有节点接入，无法生成集群健康评估。";
  }

  if (healthStatus.value === "healthy") {
    return `全部 ${totalNodes.value} 个节点处于在线状态，当前没有明显风险。`;
  }

  if (unhealthyCount.value > 0) {
    return `有 ${unhealthyCount.value} 个节点处于不健康状态，建议优先检查内核和配置同步。`;
  }

  return `有 ${offlineCount.value} 个节点离线，建议核对 Agent 心跳、网络连通性和服务状态。`;
});

const progressColor = computed(() => {
  if (healthPercentage.value >= 90) return "#22c55e";
  if (healthPercentage.value >= 70) return "#f59e0b";
  return "#ef4444";
});

const statusSegments = computed(() => {
  const items = [
    { key: "online", label: "在线", count: onlineCount.value },
    { key: "offline", label: "离线", count: offlineCount.value },
    { key: "unhealthy", label: "异常", count: unhealthyCount.value },
  ];

  return items.map((item) => ({
    ...item,
    percentage: totalNodes.value ? Math.round((item.count / totalNodes.value) * 100) : 0,
  }));
});

const rankedNodes = computed(() =>
  nodeStore.nodes
    .map((node) => {
      const maxUsers = Number(node.max_users || 0);
      const currentUsers = Number(node.current_users || 0);
      const capacityLimited = maxUsers > 0;
      const loadRatio = capacityLimited ? currentUsers / maxUsers : 0;
      const loadPercentage = capacityLimited ? Math.min(Math.round(loadRatio * 100), 100) : 0;
      const issues = buildNodeIssues(node, loadRatio);
      const severityScore = issues.reduce(
        (score, issue) => score + (issue.level === "error" ? 10 : 5),
        0,
      );

      return {
        ...node,
        capacityLimited,
        loadRatio,
        loadPercentage,
        issues,
        severityScore,
      };
    })
    .sort((a, b) => {
      if (b.severityScore !== a.severityScore) return b.severityScore - a.severityScore;
      if (b.loadPercentage !== a.loadPercentage) return b.loadPercentage - a.loadPercentage;
      return Number(b.latency || 0) - Number(a.latency || 0);
    }),
);

const highLoadNodes = computed(() =>
  rankedNodes.value.filter((node) => node.capacityLimited && node.loadPercentage >= 85),
);

const highLatencyNodes = computed(() =>
  rankedNodes.value.filter((node) => Number(node.latency || 0) >= 300),
);

const focusNodes = computed(() => {
  if (nodeFilter.value === "online") {
    return rankedNodes.value.filter((node) => node.status === "online").slice(0, 8);
  }

  if (nodeFilter.value === "all") {
    return rankedNodes.value.slice(0, 8);
  }

  const critical = rankedNodes.value.filter((node) => node.issues.length > 0);
  return (critical.length ? critical : rankedNodes.value).slice(0, 8);
});

const groupHighlights = computed(() =>
  groupStore.groups
    .map((group) => ({
      ...group,
      nodeCount: Number(group.total_nodes || group.node_count || 0),
      healthyCount: Number(group.healthy_nodes || 0),
      userCount: Number(group.total_users || group.user_count || 0),
    }))
    .sort((a, b) => {
      if (b.nodeCount !== a.nodeCount) return b.nodeCount - a.nodeCount;
      return b.userCount - a.userCount;
    })
    .slice(0, 5),
);

const alertItems = computed(() =>
  rankedNodes.value
    .flatMap((node) =>
      node.issues.map((issue, index) => ({
        id: `${node.id}-${index}-${issue.label}`,
        level: issue.level,
        message: issue.label,
        nodeName: node.name,
        time: node.last_seen_at || node.updated_at || null,
      })),
    )
    .slice(0, 6),
);

function buildNodeIssues(node, loadRatio) {
  const issues = [];

  if (node.status === "offline") {
    issues.push({ level: "error", label: "节点离线" });
  } else if (node.status === "unhealthy") {
    issues.push({ level: "error", label: "健康检查异常" });
  }

  if (node.status === "online" && node.xray_running === false) {
    issues.push({ level: "error", label: "Xray 未运行" });
  }

  if (node.install_status === "failed") {
    issues.push({ level: "warning", label: "自动安装失败" });
  }

  if (node.sync_status === "failed") {
    issues.push({ level: "warning", label: "配置同步失败" });
  } else if (node.sync_status === "pending") {
    issues.push({ level: "warning", label: "配置待同步" });
  }

  if (node.max_users > 0 && loadRatio >= 0.85) {
    issues.push({ level: "warning", label: "负载偏高" });
  }

  if (Number(node.latency || 0) >= 300) {
    issues.push({ level: "warning", label: "延迟偏高" });
  }

  return issues.slice(0, 3);
}

function getStatusType(status) {
  const types = { online: "success", offline: "info", unhealthy: "danger" };
  return types[status] || "info";
}

function getStatusText(status) {
  const texts = { online: "在线", offline: "离线", unhealthy: "异常" };
  return texts[status] || status || "未知";
}

function getSyncStatusText(status) {
  const texts = {
    synced: "已同步",
    pending: "待同步",
    failed: "同步失败",
  };
  return texts[status] || status || "未知";
}

function getLatencyClass(latency) {
  if (!latency) return "";
  if (latency < 120) return "latency-good";
  if (latency < 300) return "latency-medium";
  return "latency-bad";
}

function formatPercent(value) {
  return `${Math.round(Number(value || 0) * 100)}%`;
}

function formatLoadLabel(node) {
  if (!node.capacityLimited) {
    return `${node.current_users || 0} / 无上限`;
  }
  return `${node.current_users || 0} / ${node.max_users}`;
}

function formatBytes(bytes) {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = Number(bytes);
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 2)} ${units[unitIndex]}`;
}

function formatStrategy(strategy) {
  const texts = {
    "round-robin": "轮询",
    weighted: "权重",
    geographic: "地域",
    "least-connections": "最少连接",
  };
  return texts[strategy] || strategy || "未设置";
}

function formatCoreVersionCompact(version) {
  if (!version) return "未上报";
  const normalized = String(version).split("\n")[0];
  const matched = normalized.match(/(Xray\s+\d+(?:\.\d+)+)/i);
  return matched?.[1] || normalized;
}

function formatRelativeTime(value) {
  if (!value) return "未上报";

  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return "未上报";

  const diff = Date.now() - date.getTime();
  const seconds = Math.floor(diff / 1000);

  if (seconds < 60) return "刚刚";

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟前`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;

  const days = Math.floor(hours / 24);
  if (days < 7) return `${days} 天前`;

  return date.toLocaleString("zh-CN");
}

async function fetchData() {
  try {
    const now = new Date();
    const start = new Date(now.getFullYear(), now.getMonth(), now.getDate());

    const results = await Promise.allSettled([
      nodeStore.fetchNodes({ limit: 500 }),
      groupStore.fetchGroupsWithStats(),
      nodeStore.fetchClusterHealth(),
    ]);

    // Log individual failures without breaking the whole dashboard
    results.forEach((result, idx) => {
      if (result.status === 'rejected') {
        console.warn(`Dashboard fetch [${idx}] failed:`, result.reason);
      }
    });

    try {
      const totalTraffic = await nodeStore.getTotalTraffic({
        start: start.toISOString(),
        end: now.toISOString(),
      });
      trafficStats.value =
        totalTraffic?.stats || totalTraffic || { upload: 0, download: 0, total: 0 };
    } catch {
      trafficStats.value = { upload: 0, download: 0, total: 0 };
    }

    lastUpdatedAt.value = new Date();
  } catch (error) {
    ElMessage.error(error.message || "获取节点概览失败");
  }
}

async function refreshData() {
  loading.value = true;
  await fetchData();
  loading.value = false;
}

onMounted(() => {
  refreshData();
  refreshInterval = setInterval(fetchData, autoRefreshSeconds * 1000);
});

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval);
  }
});
</script>

<style scoped>
.node-dashboard-page {
  padding: 24px;
  --panel-border: rgba(148, 163, 184, 0.18);
  --panel-shadow: 0 20px 40px rgba(15, 23, 42, 0.06);
  --panel-radius: 22px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  margin-bottom: 20px;
}

.header-copy {
  max-width: 760px;
}

.page-eyebrow {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #64748b;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.page-title {
  margin: 0;
  font-size: 32px;
  font-weight: 700;
  line-height: 1.1;
  color: #0f172a;
}

.refresh-tag {
  border-color: rgba(59, 130, 246, 0.18);
  color: #2563eb;
  background: rgba(59, 130, 246, 0.08);
}

.page-subtitle {
  margin: 10px 0 0;
  font-size: 14px;
  line-height: 1.7;
  color: #64748b;
}

.header-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

:deep(.hero-card.el-card),
:deep(.metric-card.el-card),
:deep(.panel-card.el-card) {
  border: 1px solid var(--panel-border);
  border-radius: var(--panel-radius);
  box-shadow: var(--panel-shadow);
}

.hero-card {
  margin-bottom: 20px;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.15), transparent 32%),
    linear-gradient(135deg, #f8fbff 0%, #ffffff 48%, #f4f8ff 100%);
}

.hero-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 220px;
  gap: 24px;
  align-items: center;
}

.hero-main {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.hero-status {
  display: flex;
  align-items: center;
  gap: 18px;
}

.status-orb {
  width: 78px;
  height: 78px;
  border-radius: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.6);
}

.status-orb.good {
  background: linear-gradient(180deg, rgba(34, 197, 94, 0.18), rgba(34, 197, 94, 0.08));
  color: #15803d;
}

.status-orb.warning {
  background: linear-gradient(180deg, rgba(245, 158, 11, 0.18), rgba(245, 158, 11, 0.08));
  color: #b45309;
}

.status-orb.danger {
  background: linear-gradient(180deg, rgba(239, 68, 68, 0.18), rgba(239, 68, 68, 0.08));
  color: #b91c1c;
}

.hero-status-icon {
  font-size: 34px;
}

.hero-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-kicker {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #64748b;
}

.hero-title {
  font-size: 28px;
  font-weight: 700;
  color: #0f172a;
}

.hero-note {
  font-size: 14px;
  line-height: 1.7;
  color: #64748b;
}

.hero-highlights {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.highlight-chip {
  min-width: 132px;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.76);
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.chip-label {
  display: block;
  margin-bottom: 6px;
  font-size: 12px;
  color: #64748b;
}

.highlight-chip strong {
  font-size: 20px;
  color: #0f172a;
}

.hero-side {
  padding: 24px 20px;
  border-radius: 22px;
  background: rgba(15, 23, 42, 0.04);
  border: 1px solid rgba(148, 163, 184, 0.14);
  text-align: center;
}

.hero-percent {
  font-size: 40px;
  font-weight: 700;
  line-height: 1;
  color: #0f172a;
}

.hero-percent-label {
  margin-top: 8px;
  font-size: 13px;
  color: #64748b;
}

.hero-progress {
  margin-top: 18px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.metric-card {
  background: var(--color-bg-card);
}

.metric-card--good {
  background: linear-gradient(180deg, rgba(34, 197, 94, 0.08), rgba(255, 255, 255, 1));
}

.metric-card--latency {
  background: linear-gradient(180deg, rgba(59, 130, 246, 0.08), rgba(255, 255, 255, 1));
}

.metric-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.metric-label {
  font-size: 13px;
  color: #64748b;
}

.metric-value {
  margin-top: 8px;
  font-size: 30px;
  font-weight: 700;
  color: #0f172a;
}

.metric-icon-shell {
  width: 52px;
  height: 52px;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: #2563eb;
  background: rgba(59, 130, 246, 0.1);
}

.metric-foot {
  margin-top: 18px;
  font-size: 13px;
  line-height: 1.6;
  color: #64748b;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) minmax(320px, 0.95fr);
  gap: 20px;
}

.dashboard-main,
.dashboard-side {
  min-width: 0;
}

.panel-card {
  margin-bottom: 20px;
}

.panel-card--compact-empty :deep(.el-card__body) {
  padding-top: 18px;
  padding-bottom: 18px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.card-title {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
}

.card-subtitle {
  margin-top: 4px;
  font-size: 13px;
  color: #64748b;
}

.status-track {
  display: flex;
  height: 14px;
  overflow: hidden;
  border-radius: 999px;
  background: #e2e8f0;
}

.status-track-segment {
  height: 100%;
  transition: width 0.3s ease;
}

.status-track-segment--online {
  background: linear-gradient(90deg, #22c55e, #4ade80);
}

.status-track-segment--offline {
  background: linear-gradient(90deg, #94a3b8, #cbd5e1);
}

.status-track-segment--unhealthy {
  background: linear-gradient(90deg, #ef4444, #f87171);
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 18px;
}

.status-stat {
  padding: 16px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: #f8fafc;
}

.status-stat-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #475569;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
}

.status-stat--online .status-dot {
  background: #22c55e;
}

.status-stat--offline .status-dot {
  background: #94a3b8;
}

.status-stat--unhealthy .status-dot {
  background: #ef4444;
}

.status-stat-value {
  margin-top: 14px;
  font-size: 30px;
  font-weight: 700;
  color: #0f172a;
}

.status-stat-meta {
  margin-top: 6px;
  font-size: 13px;
  color: #64748b;
}

.focus-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.focus-node {
  padding: 18px;
  border-radius: 20px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: var(--color-bg-card);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.focus-node:hover {
  transform: translateY(-1px);
  border-color: rgba(59, 130, 246, 0.24);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.06);
}

.focus-node--issue {
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(255, 247, 237, 0.92));
}

.focus-node-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.focus-node-link {
  font-size: 16px;
  font-weight: 700;
}

.focus-node-meta {
  margin-top: 6px;
  font-size: 13px;
  color: #64748b;
  word-break: break-all;
}

.focus-node-badges {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.issue-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
}

.issue-pill {
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.issue-pill--warning {
  color: #b45309;
  background: rgba(245, 158, 11, 0.12);
}

.issue-pill--error {
  color: #b91c1c;
  background: rgba(239, 68, 68, 0.12);
}

.focus-node-load {
  margin-top: 16px;
}

.metric-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
  font-size: 13px;
  color: #64748b;
}

.metric-row strong {
  color: #0f172a;
}

.focus-node-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.focus-stat {
  padding: 12px 14px;
  border-radius: 16px;
  background: #f8fafc;
  min-width: 0;
}

.focus-stat-label {
  display: block;
  margin-bottom: 6px;
  font-size: 12px;
  color: #64748b;
}

.focus-stat-value {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #0f172a;
  word-break: break-word;
}

.group-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.compact-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.group-card {
  padding: 16px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: var(--color-bg-card);
  cursor: pointer;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.group-card:hover {
  transform: translateY(-1px);
  border-color: rgba(59, 130, 246, 0.22);
  box-shadow: 0 18px 32px rgba(15, 23, 42, 0.05);
}

.group-card-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.group-name {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.group-meta {
  margin-top: 6px;
  font-size: 12px;
  color: #64748b;
}

.group-pill {
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  color: #2563eb;
  background: rgba(59, 130, 246, 0.1);
}

.group-metric {
  margin-top: 14px;
}

.group-card-foot {
  display: flex;
  gap: 14px;
  margin-top: 12px;
  font-size: 12px;
  color: #64748b;
}

.alert-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.alert-card {
  display: flex;
  gap: 12px;
  padding: 14px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.alert-card--warning {
  background: rgba(255, 247, 237, 0.9);
}

.alert-card--error {
  background: rgba(254, 242, 242, 0.95);
}

.alert-icon-shell {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.alert-card--warning .alert-icon-shell {
  color: #b45309;
  background: rgba(245, 158, 11, 0.14);
}

.alert-card--error .alert-icon-shell {
  color: #b91c1c;
  background: rgba(239, 68, 68, 0.14);
}

.alert-title {
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
}

.alert-meta {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.6;
  color: #64748b;
}

.traffic-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.traffic-pill {
  padding: 14px 12px;
  border-radius: 18px;
  text-align: center;
  background: #f8fafc;
}

.traffic-pill--total {
  background: linear-gradient(180deg, rgba(34, 197, 94, 0.12), rgba(255, 255, 255, 1));
}

.traffic-label {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: #64748b;
}

.traffic-pill strong {
  font-size: 18px;
  color: #0f172a;
  word-break: break-word;
}

.traffic-stack {
  display: flex;
  height: 14px;
  margin-top: 18px;
  overflow: hidden;
  border-radius: 999px;
  background: #e2e8f0;
}

.traffic-stack-upload {
  background: linear-gradient(90deg, #3b82f6, #60a5fa);
}

.traffic-stack-download {
  background: linear-gradient(90deg, #22c55e, #4ade80);
}

.traffic-foot {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
  font-size: 12px;
  color: #64748b;
}

.latency-good {
  color: #16a34a;
}

.latency-medium {
  color: #d97706;
}

.latency-bad {
  color: #dc2626;
}

@media (max-width: 1280px) {
  .metrics-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 1024px) {
  .node-dashboard-page {
    padding: 16px;
  }

  .page-header,
  .hero-layout,
  .card-header,
  .focus-node-head {
    grid-template-columns: 1fr;
    flex-direction: column;
  }

  .header-actions,
  .hero-highlights,
  .focus-node-badges {
    width: 100%;
  }

  .header-actions > * {
    flex: 1;
  }

  .hero-side {
    text-align: left;
  }

  .status-grid,
  .focus-node-stats,
  .traffic-summary {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .node-dashboard-page {
    padding: 12px;
    --panel-radius: 18px;
  }

  :deep(.hero-card .el-card__body),
  :deep(.metric-card .el-card__body),
  :deep(.panel-card .el-card__body) {
    padding: 16px;
  }

  .metrics-grid {
    grid-template-columns: 1fr;
  }

  .title-row {
    align-items: flex-start;
  }

  .page-header {
    gap: 14px;
    margin-bottom: 16px;
  }

  .page-title {
    font-size: 26px;
  }

  .page-subtitle {
    font-size: 13px;
    line-height: 1.6;
  }

  .node-dashboard-page .header-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .node-dashboard-page .header-actions > * {
    min-width: 0;
    width: 100%;
    margin-left: 0;
    flex: initial;
  }

  .node-dashboard-page .header-actions > :last-child {
    grid-column: 1 / -1;
  }

  .hero-layout,
  .hero-main {
    gap: 16px;
  }

  .hero-status {
    align-items: flex-start;
    gap: 14px;
  }

  .status-orb {
    width: 60px;
    height: 60px;
    border-radius: 18px;
  }

  .hero-status-icon {
    font-size: 28px;
  }

  .hero-title {
    font-size: 24px;
  }

  .hero-note {
    font-size: 13px;
    line-height: 1.6;
  }

  .hero-highlights {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .highlight-chip {
    min-width: 0;
    padding: 12px;
    border-radius: 16px;
  }

  .highlight-chip strong {
    font-size: 18px;
  }

  .hero-side {
    padding: 16px;
    border-radius: 18px;
  }

  .hero-percent {
    font-size: 34px;
  }

  .metric-value,
  .status-stat-value {
    font-size: 26px;
  }

  .metric-icon-shell {
    width: 44px;
    height: 44px;
    border-radius: 14px;
    font-size: 20px;
  }

  .panel-card {
    margin-bottom: 16px;
  }

  .card-header {
    gap: 10px;
  }

  .status-stat,
  .focus-node,
  .alert-card,
  .group-card {
    padding: 14px;
    border-radius: 16px;
  }

  .focus-node-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .focus-node-stats .focus-stat:last-child {
    grid-column: 1 / -1;
  }

  .focus-stat {
    padding: 10px 12px;
    border-radius: 14px;
  }

  .traffic-summary {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .traffic-pill {
    padding: 12px 8px;
    border-radius: 16px;
  }

  .traffic-pill strong {
    font-size: 16px;
  }

  .traffic-foot {
    flex-direction: column;
    gap: 6px;
  }

  .group-card-head {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
