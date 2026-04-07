<template>
  <div class="dashboard-container">
    <!-- 系统概览 -->
    <div class="panel-box">
      <div class="panel-header">
        <span class="panel-title">系统概览</span>
        <el-button type="primary" size="small" @click="refreshStats">
          刷新
        </el-button>
      </div>
      <div class="stats-cards">
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12" :lg="8">
            <el-card shadow="hover" class="stats-card cpu-card">
              <template #header>
                <div class="card-header">
                  <span>CPU 使用率</span>
                  <el-tag>{{ systemStats.cpu.toFixed(1) }}%</el-tag>
                </div>
              </template>
              <div class="stats-progress">
                <el-progress
                  type="dashboard"
                  :width="isMobile ? 108 : 126"
                  :percentage="Math.min(Math.round(systemStats.cpu), 100)"
                  :color="getCpuColor"
                />
              </div>
              <div v-if="cpuInfo.model" class="stats-details">
                <p>核心数: {{ cpuInfo.cores }}</p>
                <p>型号: {{ cpuInfo.model }}</p>
              </div>
            </el-card>
          </el-col>

          <el-col :xs="24" :sm="12" :lg="8">
            <el-card shadow="hover" class="stats-card memory-card">
              <template #header>
                <div class="card-header">
                  <span>内存使用率</span>
                  <el-tag>{{ systemStats.memory.toFixed(1) }}%</el-tag>
                </div>
              </template>
              <div class="stats-progress">
                <el-progress
                  type="dashboard"
                  :width="isMobile ? 108 : 126"
                  :percentage="Math.min(Math.round(systemStats.memory), 100)"
                  :color="getMemoryColor"
                />
              </div>
              <div v-if="memoryInfo.total" class="stats-details">
                <p>已用: {{ formatBytes(memoryInfo.used) }}</p>
                <p>总计: {{ formatBytes(memoryInfo.total) }}</p>
              </div>
            </el-card>
          </el-col>

          <el-col :xs="24" :sm="12" :lg="8">
            <el-card shadow="hover" class="stats-card disk-card">
              <template #header>
                <div class="card-header">
                  <span>磁盘使用率</span>
                  <el-tag>{{ systemStats.disk.toFixed(1) }}%</el-tag>
                </div>
              </template>
              <div class="stats-progress">
                <el-progress
                  type="dashboard"
                  :width="isMobile ? 108 : 126"
                  :percentage="Math.min(Math.round(systemStats.disk), 100)"
                  :color="getDiskColor"
                />
              </div>
              <div v-if="diskInfo.total" class="stats-details">
                <p>已用: {{ formatBytes(diskInfo.used) }}</p>
                <p>总计: {{ formatBytes(diskInfo.total) }}</p>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </div>

    <!-- 系统信息 -->
    <div v-if="systemInfo.os" class="panel-box">
      <div class="panel-header">
        <span class="panel-title">系统信息</span>
      </div>
      <div class="system-info-content">
        <el-descriptions border :column="isMobile ? 1 : 3">
          <el-descriptions-item label="操作系统">
            {{ systemInfo.os }}
          </el-descriptions-item>
          <el-descriptions-item label="主机名">
            {{ systemInfo.hostname }}
          </el-descriptions-item>
          <el-descriptions-item label="运行时间">
            {{ systemInfo.uptime }}
          </el-descriptions-item>
          <el-descriptions-item label="内核版本">
            {{ systemInfo.kernel }}
          </el-descriptions-item>
          <el-descriptions-item label="负载均衡">
            {{ systemInfo.load ? systemInfo.load.join(" / ") : "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="IP 地址">
            {{ systemInfo.ipAddress }}
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </div>

    <!-- 流量统计 -->
    <div class="panel-box">
      <div class="panel-header">
        <span class="panel-title">流量统计</span>
        <el-radio-group
          v-model="trafficPeriod"
          class="period-switch"
          size="small"
          @change="changeTrafficPeriod"
        >
          <el-radio-button value="today"> 今日 </el-radio-button>
          <el-radio-button value="week"> 本周 </el-radio-button>
          <el-radio-button value="month"> 本月 </el-radio-button>
        </el-radio-group>
      </div>
      <div class="traffic-stats">
        <el-row :gutter="20">
          <el-col :xs="24" :lg="12">
            <el-card shadow="hover" class="traffic-card">
              <template #header>
                <div class="card-header">
                  <span>总流量</span>
                </div>
              </template>
              <div class="traffic-info">
                <div class="traffic-data">
                  <div class="traffic-value">
                    {{ formatTraffic(trafficStats.total) }}
                  </div>
                  <div class="traffic-label">
                    {{ trafficPeriodLabel }}总流量
                  </div>
                  <div class="traffic-meta-list">
                    <div class="traffic-meta-hint">
                      {{ trafficPeriodSummaryHint }}
                    </div>
                    <div class="traffic-meta-item">
                      <span>用户总额度</span>
                      <strong>{{
                        formatTrafficLimit(trafficStats.userLimit)
                      }}</strong>
                    </div>
                    <div class="traffic-meta-item">
                      <span>在线节点总额度</span>
                      <strong>{{
                        formatTrafficLimit(trafficStats.nodeLimit)
                      }}</strong>
                    </div>
                    <div class="traffic-meta-item">
                      <span>有效总额度</span>
                      <strong>{{
                        formatTrafficLimit(trafficStats.limit)
                      }}</strong>
                    </div>
                    <div class="traffic-meta-hint">
                      {{ trafficLimitHint }}
                    </div>
                  </div>
                </div>
                <div class="traffic-chart">
                  <el-progress
                    type="circle"
                    :percentage="trafficProgressPercentage"
                    :width="isMobile ? 96 : 120"
                    :format="formatTrafficProgress"
                  />
                </div>
              </div>
            </el-card>
          </el-col>

          <el-col :xs="24" :lg="12">
            <el-card shadow="hover" class="traffic-card">
              <template #header>
                <div class="card-header">
                  <span>上/下行流量</span>
                </div>
              </template>
              <div class="traffic-details">
                <div class="traffic-item">
                  <span class="traffic-item-label">上行流量</span>
                  <span class="traffic-item-value">{{
                    formatTraffic(trafficStats.up)
                  }}</span>
                </div>
                <div class="traffic-item">
                  <span class="traffic-item-label">下行流量</span>
                  <span class="traffic-item-value">{{
                    formatTraffic(trafficStats.down)
                  }}</span>
                </div>
                <div class="traffic-chart-small">
                  <div class="up-down-ratio">
                    <div
                      class="up-bar"
                      :style="{ width: getUpPercentage + '%' }"
                    />
                    <div
                      class="down-bar"
                      :style="{ width: getDownPercentage + '%' }"
                    />
                  </div>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </div>

    <!-- 协议概览 -->
    <div class="panel-box">
      <div class="panel-header">
        <span class="panel-title">协议概览</span>
      </div>
      <div class="protocols-stats">
        <el-row :gutter="20">
          <el-col :xs="24" :lg="12">
            <el-card shadow="hover" class="protocol-card">
              <template #header>
                <div class="card-header">
                  <span>活跃协议</span>
                </div>
              </template>
              <div class="protocol-list">
                <el-table :data="protocolStats" border style="width: 100%">
                  <el-table-column prop="protocol" label="协议类型">
                    <template #default="scope">
                      <span class="protocol-tag" :class="scope.row.protocol">{{
                        scope.row.protocol
                      }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="count" label="数量" width="80" />
                  <el-table-column label="状态" width="100">
                    <template #default="scope">
                      <el-tag
                        :type="
                          scope.row.status === 'active' ? 'success' : 'danger'
                        "
                      >
                        {{
                          scope.row.status === "active" ? "运行中" : "已停止"
                        }}
                      </el-tag>
                    </template>
                  </el-table-column>
                </el-table>
                <el-empty
                  v-if="protocolStats.length === 0"
                  description="暂无协议数据"
                />
              </div>
            </el-card>
          </el-col>

          <el-col :xs="24" :lg="12">
            <el-card shadow="hover" class="protocol-card">
              <template #header>
                <div class="card-header">
                  <span>流量分布</span>
                </div>
              </template>
              <div class="protocol-chart">
                <div
                  v-if="protocolTraffic.length > 0"
                  class="traffic-distribution"
                >
                  <div
                    v-for="(item, index) in protocolTraffic"
                    :key="index"
                    class="traffic-bar"
                  >
                    <div class="bar-label">
                      {{ item.protocol }}
                    </div>
                    <div class="bar-container">
                      <div
                        class="bar-fill"
                        :style="{
                          width: item.percentage + '%',
                          backgroundColor: getProtocolColor(item.protocol),
                        }"
                      />
                    </div>
                    <div class="bar-value">
                      {{ formatTraffic(item.traffic) }}
                    </div>
                  </div>
                </div>
                <el-empty v-else description="暂无流量数据" />
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { ElMessage } from "element-plus";
import { statsApi, systemApi } from "@/api";
import { useViewport } from "@/composables/useViewport";

const { isMobile } = useViewport();
const DASHBOARD_REFRESH_INTERVAL = 60000;

// 系统状态数据
const systemStats = ref({
  cpu: 0,
  memory: 0,
  disk: 0,
});

// 格式化百分比，保留1位小数
const formatPercent = (value) => {
  if (typeof value !== "number" || isNaN(value)) return 0;
  return Math.round(value * 10) / 10;
};

// 详细系统信息
const cpuInfo = ref({ cores: 0, model: "" });
const memoryInfo = ref({ used: 0, total: 0 });
const diskInfo = ref({ used: 0, total: 0 });
const systemInfo = ref({
  os: "",
  kernel: "",
  hostname: "",
  uptime: "",
  load: null,
  ipAddress: "",
});

// 流量统计数据
const trafficPeriod = ref("week");
const trafficStats = ref({
  total: 0,
  up: 0,
  down: 0,
  limit: 0,
  userLimit: 0,
  nodeLimit: 0,
  percentage: 0,
});

// 协议统计数据
const protocolStats = ref([]);

// 协议流量分布
const protocolTraffic = ref([]);

const unwrapApiData = (response) => {
  if (response && response.code === 200 && response.data !== undefined) {
    return response.data;
  }
  return response;
};

const normalizeTrafficStats = (payload, fallback = trafficStats.value) => {
  const data = payload || {};
  return {
    total: Number(data.total || 0),
    up: Number(data.up ?? data.upload ?? fallback?.up ?? 0),
    down: Number(data.down ?? data.download ?? fallback?.down ?? 0),
    limit: Number(data.limit || 0),
    userLimit: Number(data.user_limit ?? data.userLimit ?? 0),
    nodeLimit: Number(data.node_limit ?? data.nodeLimit ?? 0),
    percentage: Number(data.percentage || 0),
  };
};

const buildProtocolTraffic = (stats) => {
  const items = Array.isArray(stats) ? stats : [];
  const totalTraffic = items.reduce(
    (sum, item) => sum + (item?.traffic || 0),
    0,
  );

  return items
    .filter((item) => (item?.traffic || 0) > 0)
    .map((item) => ({
      protocol: item.protocol,
      traffic: item.traffic || 0,
      percentage:
        totalTraffic > 0
          ? Math.round(((item.traffic || 0) / totalTraffic) * 1000) / 10
          : 0,
    }));
};

// 计算上传流量百分比
const getUpPercentage = computed(() => {
  const total = trafficStats.value.up + trafficStats.value.down;
  return total > 0 ? Math.round((trafficStats.value.up / total) * 100) : 50;
});

// 计算下载流量百分比
const getDownPercentage = computed(() => {
  const total = trafficStats.value.up + trafficStats.value.down;
  return total > 0 ? Math.round((trafficStats.value.down / total) * 100) : 50;
});

const hasTrafficLimit = computed(
  () => Number(trafficStats.value.limit || 0) > 0,
);

const trafficLimitHint = computed(() => {
  const hasUserLimit = Number(trafficStats.value.userLimit || 0) > 0;
  const hasNodeLimit = Number(trafficStats.value.nodeLimit || 0) > 0;

  if (hasUserLimit && hasNodeLimit) {
    return "有效总额度取用户总额度与在线节点总额度中的较小值";
  }
  if (hasUserLimit) {
    return "当前按用户总额度计算流量进度";
  }
  if (hasNodeLimit) {
    return "当前按在线节点总额度计算流量进度";
  }
  return "当前未配置可用于进度计算的总额度";
});

const trafficPeriodLabel = computed(() => {
  switch (trafficPeriod.value) {
    case "today":
      return "今日";
    case "week":
      return "本周";
    case "month":
      return "本月";
    default:
      return "当前";
  }
});

const trafficPeriodSummaryHint = computed(() => {
  if (Number(trafficStats.value.total || 0) > 0) {
    return `当前展示${trafficPeriodLabel.value.toLowerCase()}聚合流量`;
  }

  if (trafficPeriod.value === "today") {
    return "今日暂无新流量，可切换到本周或本月查看历史聚合";
  }

  return `当前所选${trafficPeriodLabel.value}暂无流量记录`;
});

const trafficProgressPercentage = computed(() => {
  const rawValue = Number(trafficStats.value.percentage || 0);
  if (!Number.isFinite(rawValue) || rawValue <= 0) {
    return 0;
  }

  return Math.min(rawValue, 100);
});

// CPU 颜色
const getCpuColor = computed(() => {
  const cpu = systemStats.value.cpu;
  if (cpu < 50) return "#67c23a";
  if (cpu < 80) return "#e6a23c";
  return "#f56c6c";
});

// 内存颜色
const getMemoryColor = computed(() => {
  const memory = systemStats.value.memory;
  if (memory < 50) return "#67c23a";
  if (memory < 80) return "#e6a23c";
  return "#f56c6c";
});

// 磁盘颜色
const getDiskColor = computed(() => {
  const disk = systemStats.value.disk;
  if (disk < 50) return "#67c23a";
  if (disk < 80) return "#e6a23c";
  return "#f56c6c";
});

// 格式化流量
const formatTraffic = (bytes) => {
  if (!bytes || bytes === 0) return "0 B";

  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};
const formatTrafficLimit = (bytes) => {
  const value = Number(bytes || 0);
  if (!Number.isFinite(value) || value <= 0) {
    return "不限额";
  }
  return formatTraffic(value);
};

const formatTrafficProgress = (percentage) => {
  if (!hasTrafficLimit.value) {
    return "不限";
  }

  const normalized = Number(percentage || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return "0%";
  }

  const clamped = Math.min(normalized, 100);
  if (clamped < 0.1) {
    return "<0.1%";
  }

  const rounded = Math.round(clamped * 10) / 10;
  if (rounded >= 100) {
    return "100%";
  }

  return Number.isInteger(rounded) ? `${rounded}%` : `${rounded.toFixed(1)}%`;
};

// 格式化字节
const formatBytes = (bytes) => {
  return formatTraffic(bytes);
};

// 获取协议颜色
const getProtocolColor = (protocol) => {
  switch (protocol) {
    case "vmess":
      return "#409eff";
    case "vless":
      return "#67c23a";
    case "trojan":
      return "#e6a23c";
    case "shadowsocks":
      return "#f56c6c";
    default:
      return "#909399";
  }
};

// 加载系统状态
const loadSystemStatus = async () => {
  try {
    const response = await systemApi.getSystemStatus({
      include_processes: false,
    });

    // 后端直接返回数据，不是 {code, data} 格式
    // 检查响应格式
    let data = response;
    if (response && response.code === 200 && response.data) {
      data = response.data;
    }

    if (data) {
      // 更新系统信息
      if (data.systemInfo) {
        systemInfo.value = data.systemInfo;
        if (!systemInfo.value.load) {
          systemInfo.value.load = [0, 0, 0];
        }
      }

      // 更新CPU信息
      if (data.cpuInfo) {
        cpuInfo.value = data.cpuInfo;
      }
      systemStats.value.cpu = formatPercent(
        data.cpuUsage || data.CPU?.usage || 0,
      );

      // 更新内存信息
      if (data.memoryInfo) {
        memoryInfo.value = data.memoryInfo;
      }
      systemStats.value.memory = formatPercent(
        data.memoryUsage || data.Memory?.usage_percent || 0,
      );

      // 更新磁盘信息
      if (data.diskInfo) {
        diskInfo.value = data.diskInfo;
      }
      systemStats.value.disk = formatPercent(data.diskUsage || 0);
    }
    return true;
  } catch (error) {
    console.error("Failed to load system status:", error);
    return false;
  }
};

// 加载统计数据
const loadStats = async () => {
  try {
    const response = await statsApi.getDetailedStats({
      period: trafficPeriod.value,
    });
    const data = unwrapApiData(response) || {};

    trafficStats.value = normalizeTrafficStats(
      {
        total: data.total_traffic ?? data.totalTraffic,
        upload: data.upload,
        download: data.download,
        up: data.upload,
        down: data.download,
        limit: data.limit,
        user_limit: data.user_limit ?? data.userLimit,
        node_limit: data.node_limit ?? data.nodeLimit,
        percentage: data.percentage,
      },
      trafficStats.value,
    );

    const protocols = Array.isArray(data.by_protocol ?? data.byProtocol)
      ? data.by_protocol ?? data.byProtocol
      : [];
    protocolStats.value = protocols;
    protocolTraffic.value = buildProtocolTraffic(protocols);

    return {
      success: true,
      partial: false,
    };
  } catch (error) {
    console.error("Failed to load detailed stats:", error);
    return {
      success: false,
      partial: false,
    };
  }
};

// 加载所有数据
const loadData = async () => {
  const [systemOk, statsResult] = await Promise.all([
    loadSystemStatus(),
    loadStats(),
  ]);

  const successCount = Number(systemOk) + Number(statsResult?.success);
  return {
    success: successCount > 0,
    partial: successCount > 0 && (systemOk === false || statsResult?.partial),
  };
};

// 刷新统计数据
const refreshStats = async () => {
  const result = await loadData();
  if (result.success && !result.partial) {
    ElMessage.success("数据已刷新");
    return;
  }
  if (result.success) {
    ElMessage.warning("部分数据刷新成功");
    return;
  }
  ElMessage.error("刷新失败");
};

// 切换流量统计周期
const changeTrafficPeriod = async (period) => {
  const previousPeriod = trafficPeriod.value;
  const previousTrafficStats = { ...trafficStats.value };
  const previousProtocolStats = [...protocolStats.value];
  const previousProtocolTraffic = [...protocolTraffic.value];
  trafficPeriod.value = period;
  const result = await loadStats();

  if (result.success && !result.partial) {
    return;
  }
  if (result.success) {
    trafficPeriod.value = previousPeriod;
    trafficStats.value = previousTrafficStats;
    protocolStats.value = previousProtocolStats;
    protocolTraffic.value = previousProtocolTraffic;
    ElMessage.warning("部分请求失败，已保留原统计周期数据");
    return;
  }
  trafficPeriod.value = previousPeriod;
  trafficStats.value = previousTrafficStats;
  protocolStats.value = previousProtocolStats;
  protocolTraffic.value = previousProtocolTraffic;
  ElMessage.error("统计数据更新失败");
};

// 定时刷新
let refreshTimer = null;

const startAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
  }

  refreshTimer = setInterval(() => {
    if (document.visibilityState === "hidden") {
      return;
    }
    loadData();
  }, DASHBOARD_REFRESH_INTERVAL);
};

const handleVisibilityChange = () => {
  if (document.visibilityState === "visible") {
    loadData();
  }
};

// 初始化
onMounted(() => {
  loadData();
  startAutoRefresh();
  document.addEventListener("visibilitychange", handleVisibilityChange);
});

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
  }
  document.removeEventListener("visibilitychange", handleVisibilityChange);
});
</script>

<style scoped>
.dashboard-container {
  padding-bottom: 20px;
}

.panel-box {
  background-color: var(--el-bg-color, #fff);
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  margin-bottom: 20px;
  border: 1px solid var(--el-border-color, #eee);
}

.panel-header {
  padding: 15px 20px;
  border-bottom: 1px solid var(--el-border-color, #eee);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.panel-title {
  font-size: 16px;
  font-weight: bold;
  color: var(--el-text-color-primary, #333);
}

.stats-cards,
.traffic-stats,
.protocols-stats,
.system-info-content {
  padding: 20px;
}

.system-info-content,
.protocol-list,
.protocol-chart {
  overflow-x: auto;
}

.period-switch {
  flex-shrink: 0;
}

.protocol-list :deep(.el-table) {
  min-width: 320px;
}

.stats-card {
  height: auto;
  min-height: 240px;
  margin-bottom: 10px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stats-progress {
  display: flex;
  justify-content: center;
  padding: 20px 0;
  /* macOS 显示模式兼容性修复 */
  -webkit-transform: translateZ(0);
  transform: translateZ(0);
}

.stats-progress :deep(.el-progress) {
  -webkit-transform: translateZ(0);
  transform: translateZ(0);
}

.stats-progress :deep(.el-progress svg) {
  -webkit-transform: translateZ(0);
  transform: translateZ(0);
  shape-rendering: geometricPrecision;
}

.stats-progress :deep(.el-progress__text) {
  font-weight: bold;
}

.stats-details {
  text-align: center;
  padding: 10px 0;
  border-top: 1px solid var(--el-border-color, #eee);
  margin-top: 10px;
}

.stats-details p {
  margin: 5px 0;
  color: var(--el-text-color-regular, #606266);
  font-size: 12px;
}

.traffic-card {
  height: auto;
  min-height: 260px;
  margin-bottom: 10px;
}

.protocol-card {
  height: 320px;
  margin-bottom: 10px;
}

.traffic-info {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  padding: 8px 10px 4px;
  min-height: 180px;
}

.traffic-data {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
  padding: 0 20px 0 10px;
}

.traffic-value {
  font-size: 42px;
  line-height: 1.05;
  font-weight: bold;
  color: #409eff;
  margin: 0;
}

.traffic-label {
  font-size: 14px;
  color: var(--el-text-color-regular, #666);
}
.traffic-meta-list {
  display: grid;
  gap: 8px;
  width: 100%;
  max-width: 420px;
}

.traffic-meta-item {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  font-size: 13px;
  color: var(--el-text-color-regular, #666);
}

.traffic-meta-item strong {
  color: var(--el-text-color-primary, #303133);
}

.traffic-meta-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  line-height: 1.5;
}

.traffic-chart {
  flex-shrink: 0;
  width: 140px;
  display: flex;
  justify-content: center;
  align-items: center;
  padding-top: 4px;
}

.traffic-details {
  padding: 20px;
  height: 140px;
}

.traffic-item {
  display: flex;
  justify-content: space-between;
  margin-bottom: 15px;
}

.traffic-item-label {
  color: var(--el-text-color-regular, #666);
}

.traffic-item-value {
  font-weight: bold;
}

.traffic-chart-small {
  margin-top: 20px;
}

.up-down-ratio {
  height: 20px;
  background-color: var(--el-fill-color, #f5f7fa);
  border-radius: 10px;
  overflow: hidden;
  display: flex;
}

.up-bar {
  background-color: #409eff;
  height: 100%;
}

.down-bar {
  background-color: #67c23a;
  height: 100%;
}

.protocol-tag {
  display: inline-block;
  padding: 2px 8px;
  font-size: 12px;
  border-radius: 3px;
  color: #fff;
  background-color: #909399;
}

.protocol-tag.vmess {
  background-color: #409eff;
}

.protocol-tag.vless {
  background-color: #67c23a;
}

.protocol-tag.trojan {
  background-color: #e6a23c;
}

.protocol-tag.shadowsocks {
  background-color: #f56c6c;
}

.traffic-distribution {
  padding: 10px 0;
}

.traffic-bar {
  margin-bottom: 15px;
}

.bar-label {
  font-size: 14px;
  margin-bottom: 5px;
}

.bar-container {
  height: 20px;
  background-color: var(--el-fill-color, #f5f7fa);
  border-radius: 10px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: 10px;
}

.bar-value {
  text-align: right;
  font-size: 12px;
  margin-top: 2px;
  color: var(--el-text-color-regular, #666);
}

@media (max-width: 768px) {
  .panel-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }

  .stats-cards,
  .traffic-stats,
  .protocols-stats,
  .system-info-content {
    padding: 14px;
  }

  .stats-card,
  .traffic-card,
  .protocol-card {
    height: auto;
    min-height: auto;
  }

  .traffic-info {
    flex-direction: column;
    gap: 16px;
    height: auto;
    padding: 8px 0;
  }

  .traffic-data {
    padding: 0;
  }

  .traffic-details {
    height: auto;
    padding: 4px 0;
  }

  .traffic-value {
    font-size: 20px;
  }

  .period-switch {
    width: 100%;
  }

  .period-switch :deep(.el-radio-button) {
    flex: 1;
  }

  .period-switch :deep(.el-radio-button__inner) {
    width: 100%;
  }
}
</style>
