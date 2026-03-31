<template>
  <div class="nodes-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">节点列表</h1>
        <p class="page-subtitle">
          优先选择在线且延迟更低的节点，连接会更稳定。
        </p>
      </div>

      <div class="header-stats">
        <div class="stat-chip">
          <span class="stat-chip-label">全部节点</span>
          <strong>{{ totalCount }}</strong>
        </div>
        <div class="stat-chip stat-chip-success">
          <span class="stat-chip-label">在线节点</span>
          <strong>{{ onlineCount }}</strong>
        </div>
        <div class="stat-chip">
          <span class="stat-chip-label">当前显示</span>
          <strong>{{ filteredNodes.length }}</strong>
        </div>
      </div>
    </div>

    <div v-if="!accessRestricted" class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="filters.keyword"
          clearable
          class="keyword-filter"
          placeholder="搜索名称 / 地址 / 地区 / 协议"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <el-select
          v-model="filters.region"
          placeholder="全部地区"
          clearable
          style="width: 140px"
        >
          <el-option
            v-for="region in regionOptions"
            :key="region.value"
            :label="region.label"
            :value="region.value"
          />
        </el-select>

        <el-select
          v-model="filters.protocol"
          placeholder="全部协议"
          clearable
          style="width: 140px"
        >
          <el-option
            v-for="protocol in protocolOptions"
            :key="protocol.value"
            :label="protocol.label"
            :value="protocol.value"
          />
        </el-select>

        <el-select
          v-model="filters.status"
          placeholder="全部状态"
          clearable
          style="width: 120px"
        >
          <el-option
            v-for="status in statusOptions"
            :key="status.value"
            :label="status.label"
            :value="status.value"
          />
        </el-select>
      </div>

      <div class="filter-right">
        <el-select v-model="sortBy" style="width: 150px">
          <el-option label="默认排序" value="default" />
          <el-option label="按名称" value="name" />
          <el-option label="按地区" value="region" />
          <el-option label="按延迟" value="latency" />
          <el-option label="按负载" value="load" />
        </el-select>

        <el-button v-if="hasActiveFilters" @click="resetFilters">
          重置筛选
        </el-button>

        <el-button @click="refreshNodes">
          <el-icon><RefreshRight /></el-icon>
          刷新
        </el-button>

        <el-button-group>
          <el-button
            :type="viewMode === 'card' ? 'primary' : 'default'"
            @click="viewMode = 'card'"
          >
            <el-icon><Grid /></el-icon>
          </el-button>
          <el-button
            :type="viewMode === 'list' ? 'primary' : 'default'"
            @click="viewMode = 'list'"
          >
            <el-icon><List /></el-icon>
          </el-button>
        </el-button-group>

        <el-button
          type="primary"
          :loading="testingAll"
          :disabled="!hasOnlineNodes"
          @click="testAllLatency"
        >
          <el-icon><Timer /></el-icon>
          测速全部
        </el-button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <el-icon class="loading-icon">
        <Loading />
      </el-icon>
      <p>加载节点列表...</p>
    </div>

    <el-card
      v-else-if="accessRestricted"
      class="access-card"
      shadow="never"
    >
      <el-empty description="当前暂无可用节点">
        <template #description>
          <p class="access-card__description">
            {{ accessRestrictedHint }}
          </p>
        </template>
        <div class="access-card__actions">
          <el-button
            type="primary"
            @click="goToPlans"
          >
            {{ accessPlanActionLabel }}
          </el-button>
          <el-button
            v-if="hasCurrentPlan"
            @click="goToSubscription"
          >
            查看订阅状态
          </el-button>
        </div>
      </el-empty>
    </el-card>

    <el-empty
      v-else-if="filteredNodes.length === 0"
      :description="emptyNodesDescription"
    >
      <el-button
        v-if="hasActiveFilters"
        @click="resetFilters"
      >
        清空筛选
      </el-button>
    </el-empty>

    <div v-else-if="viewMode === 'card'" class="nodes-grid">
      <NodeCard
        v-for="node in filteredNodes"
        :key="node.id"
        :node="node"
        :latency="latencyResults[node.id]"
        :testing="testingNodes[node.id]"
        @test="testLatency(node)"
        @copy="copyNodeConfig(node)"
      />
    </div>

    <el-table
      v-else
      :data="filteredNodes"
      class="nodes-table"
      table-layout="auto"
    >
      <el-table-column label="节点信息" min-width="320">
        <template #default="{ row }">
          <div class="node-info-cell">
            <div class="node-endpoint" :title="getNodeEndpoint(row)">
              {{ getNodeEndpoint(row) }}
            </div>
            <div class="node-name-row">
              <span class="node-flag">{{
                getRegionFlag(row.region_label || row.region)
              }}</span>
              <div class="node-name-meta">
                <span class="node-name">{{ getNodeDisplayName(row) }}</span>
                <span class="node-subtitle">{{ getNodeMetaLine(row) }}</span>
              </div>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="地区" prop="region" width="110">
        <template #default="{ row }">
          {{ row.region_label || getRegionLabel(row.region) }}
        </template>
      </el-table-column>

      <el-table-column label="协议" prop="protocol" width="120">
        <template #default="{ row }">
          <el-tag size="small" effect="light">
            {{ row.protocol_label || getProtocolLabel(row.protocol) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small" effect="light">
            {{ getStatusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column label="负载" width="150">
        <template #default="{ row }">
          <div class="table-load-cell">
            <el-progress
              :percentage="getNormalizedLoad(row.load)"
              :color="getLoadColor(getNormalizedLoad(row.load))"
              :stroke-width="6"
              :show-text="false"
            />
            <span class="load-text">{{ getNormalizedLoad(row.load) }}%</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="延迟" width="120">
        <template #default="{ row }">
          <span v-if="testingNodes[row.id]" class="latency-testing">
            <el-icon class="is-loading"><Loading /></el-icon>
          </span>
          <span
            v-else-if="hasLatency(row.id)"
            :class="getLatencyClass(latencyResults[row.id])"
          >
            {{ latencyResults[row.id] }}ms
          </span>
          <span v-else class="latency-unknown"> 未测试 </span>
        </template>
      </el-table-column>

      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button
            link
            type="primary"
            :loading="testingNodes[row.id]"
            :disabled="row.status !== 'online'"
            @click="testLatency(row)"
          >
            测速
          </el-button>
          <el-button
            link
            type="primary"
            :disabled="row.status !== 'online'"
            @click="copyNodeConfig(row)"
          >
            复制
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import {
  Grid,
  List,
  Timer,
  Loading,
  Search,
  RefreshRight,
} from "@element-plus/icons-vue";
import { usePortalNodesStore } from "@/stores/portalNodes";
import { useUserPortalStore } from "@/stores/userPortal";
import { proxiesApi } from "@/api/modules/proxies";
import { copyText } from "@/utils/clipboard";
import NodeCard from "@/components/user/NodeCard.vue";
import {
  getNodeLatencyClass,
  getNodeRegionFlag,
  getNodeRegionLabel,
  getNodeStatusText,
  getNodeStatusType,
  getProtocolDisplayName,
} from "@/composables/useNodePresentation";
import { extractErrorMessage, getNoEntitlementMessage, isNoEntitlementError } from "@/utils/entitlement";

const router = useRouter();
const nodesStore = usePortalNodesStore();
const userStore = useUserPortalStore();

const loading = ref(false);
const accessRestricted = ref(false);
const accessRestrictedMessage = ref(getNoEntitlementMessage("nodes"));
const viewMode = ref("card");
const sortBy = ref("default");
const testingAll = ref(false);
const testingNodes = reactive({});
const latencyResults = reactive({});

const filters = reactive({
  keyword: "",
  region: "",
  protocol: "",
  status: "",
});

const statusOptions = [
  { value: "online", label: "在线" },
  { value: "offline", label: "离线" },
  { value: "unhealthy", label: "不健康" },
  { value: "maintenance", label: "维护中" },
];

const regionOptions = computed(() =>
  (nodesStore.regions || []).map((region) => ({
    value: region,
    label: getNodeRegionLabel(region),
  })),
);

const protocolOptions = computed(() =>
  (nodesStore.protocols || []).map((protocol) => ({
    value: protocol,
    label: getProtocolDisplayName(protocol),
  })),
);

const totalCount = computed(() => nodesStore.nodes.length);
const onlineCount = computed(
  () => nodesStore.nodes.filter((node) => node.status === "online").length,
);
const hasOnlineNodes = computed(() =>
  filteredNodes.value.some((node) => node.status === "online"),
);
const hasActiveFilters = computed(() =>
  Boolean(
    filters.keyword || filters.region || filters.protocol || filters.status,
  ),
);
const hasCurrentPlan = computed(() => Boolean(userStore.user?.plan_id));
const hasExpiredEntitlement = computed(() => hasCurrentPlan.value && userStore.status === "expired");
const accessPlanActionLabel = computed(() => {
  if (!hasCurrentPlan.value) return "购买套餐";
  if (hasExpiredEntitlement.value) return "续费套餐";
  return "查看套餐列表";
});
const accessRestrictedHint = computed(() => {
  if (!accessRestricted.value) return "";
  if (!hasCurrentPlan.value) {
    return "当前还没有有效套餐，购买套餐后即可查看和使用节点。";
  }
  if (hasExpiredEntitlement.value) {
    return "当前套餐已过期，续费后即可继续查看和使用节点。";
  }
  return accessRestrictedMessage.value || getNoEntitlementMessage("nodes");
});
const emptyNodesDescription = computed(() =>
  hasActiveFilters.value ? "没有符合条件的节点" : "暂无可用节点",
);

const getNodeDisplayName = (node) =>
  node.display_name || node.name || "未命名节点";

const getNodeEndpoint = (node) => {
  if (!node.host) return "-";
  return node.port ? `${node.host}:${node.port}` : node.host;
};

const getFallbackSubtitle = (node) =>
  [
    node.protocol_label || getProtocolLabel(node.protocol),
    getNodeEndpoint(node) !== "-" ? getNodeEndpoint(node) : "",
  ]
    .filter(Boolean)
    .join(" · ");

const getNodeSubtitle = (node) => {
  const subtitle = String(node.subtitle || "").trim();
  if (subtitle && subtitle !== getFallbackSubtitle(node)) {
    return subtitle;
  }
  return [
    node.region_label || getRegionLabel(node.region),
    node.protocol_label || getProtocolLabel(node.protocol),
  ]
    .filter(Boolean)
    .join(" · ");
};

const getNodeMetaLine = (node) => getNodeSubtitle(node);

const filteredNodes = computed(() => {
  let nodes = [...nodesStore.nodes];

  if (filters.keyword) {
    const keyword = filters.keyword.trim().toLowerCase();
    nodes = nodes.filter((node) => {
      const searchText = [
        getNodeDisplayName(node),
        getNodeEndpoint(node),
        node.region_label || getRegionLabel(node.region),
        node.protocol_label || getProtocolLabel(node.protocol),
        node.subtitle || "",
      ]
        .join(" ")
        .toLowerCase();
      return searchText.includes(keyword);
    });
  }

  if (filters.region) {
    nodes = nodes.filter((node) => node.region === filters.region);
  }
  if (filters.protocol) {
    nodes = nodes.filter((node) => node.protocol === filters.protocol);
  }
  if (filters.status) {
    nodes = nodes.filter((node) => node.status === filters.status);
  }

  if (sortBy.value === "name") {
    nodes.sort((a, b) =>
      getNodeDisplayName(a).localeCompare(getNodeDisplayName(b), "zh-CN"),
    );
  } else if (sortBy.value === "region") {
    nodes.sort((a, b) =>
      getRegionLabel(a.region_label || a.region).localeCompare(
        getRegionLabel(b.region_label || b.region),
        "zh-CN",
      ),
    );
  } else if (sortBy.value === "latency") {
    nodes.sort((a, b) => {
      const la = hasLatencyValue(latencyResults[a.id])
        ? latencyResults[a.id]
        : Number.MAX_SAFE_INTEGER;
      const lb = hasLatencyValue(latencyResults[b.id])
        ? latencyResults[b.id]
        : Number.MAX_SAFE_INTEGER;
      return la - lb;
    });
  } else if (sortBy.value === "load") {
    nodes.sort((a, b) => getNormalizedLoad(a.load) - getNormalizedLoad(b.load));
  }

  return nodes;
});

function getRegionFlag(region) {
  return getNodeRegionFlag(region);
}

function getRegionLabel(region) {
  return getNodeRegionLabel(region);
}

function getProtocolLabel(protocol) {
  return getProtocolDisplayName(protocol);
}

function getStatusType(status) {
  if (status === "maintenance") return "warning";
  return getNodeStatusType(status);
}

function getStatusLabel(status) {
  if (status === "maintenance") return "维护中";
  return getNodeStatusText(status);
}

function getNormalizedLoad(load) {
  const value = Number(load);
  if (!Number.isFinite(value) || value < 0) return 0;
  if (value > 100) return 100;
  return Math.round(value);
}

function getLoadColor(load) {
  if (load >= 80) return "#f56c6c";
  if (load >= 60) return "#e6a23c";
  return "#67c23a";
}

function hasLatencyValue(value) {
  return typeof value === "number" && value >= 0;
}

function hasLatency(nodeId) {
  return hasLatencyValue(latencyResults[nodeId]);
}

function getLatencyClass(latency) {
  return getNodeLatencyClass(latency);
}

function resetFilters() {
  filters.keyword = "";
  filters.region = "";
  filters.protocol = "";
  filters.status = "";
  sortBy.value = "default";
}

async function testLatency(node) {
  testingNodes[node.id] = true;
  try {
    const latency = await nodesStore.testNodeLatency(node.id);
    latencyResults[node.id] = latency;
  } catch (error) {
    latencyResults[node.id] = null;
    ElMessage.error(`测速失败: ${getNodeDisplayName(node)}`);
  } finally {
    testingNodes[node.id] = false;
  }
}

async function testAllLatency() {
  testingAll.value = true;
  const onlineNodes = filteredNodes.value.filter(
    (node) => node.status === "online",
  );

  for (const node of onlineNodes) {
    await testLatency(node);
  }

  testingAll.value = false;
  ElMessage.success("测速完成");
}

async function copyNodeConfig(node) {
  try {
    const response = await proxiesApi.generateLink(node.id);
    const link = response?.link;
    await copyText(link);
    ElMessage.success(`已复制 ${getNodeDisplayName(node)} 配置`);
  } catch (error) {
    ElMessage.error(`复制失败: ${getNodeDisplayName(node)}`);
  }
}

async function loadNodes() {
  loading.value = true;
  try {
    await nodesStore.fetchNodes();
    accessRestricted.value = false;
    return true;
  } catch (error) {
    if (isNoEntitlementError(error)) {
      accessRestricted.value = true;
      accessRestrictedMessage.value =
        extractErrorMessage(error) || getNoEntitlementMessage("nodes");
      return false;
    }

    accessRestricted.value = false;
    const message = extractErrorMessage(error) || "加载节点列表失败";
    ElMessage.error(message);
    return false;
  } finally {
    loading.value = false;
  }
}

async function refreshNodes() {
  const ok = await loadNodes();
  if (ok) {
    ElMessage.success("节点列表已刷新");
  }
}

function goToPlans() {
  router.push("/user/plans").catch((error) => {
    console.error("跳转到套餐页面失败:", error);
  });
}

function goToSubscription() {
  router.push("/user/subscription").catch((error) => {
    console.error("跳转到订阅页面失败:", error);
  });
}

onMounted(() => {
  loadNodes();
});
</script>

<style scoped>
.nodes-page {
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin: 0;
  line-height: 1.6;
}

.header-stats {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
}

.stat-chip {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 100px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid var(--color-border-light);
  background: var(--color-bg-card);
  box-shadow: var(--shadow-sm);
}

.stat-chip-label {
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1;
}

.stat-chip strong {
  font-size: 20px;
  line-height: 1.1;
  color: var(--color-text-primary);
}

.stat-chip-success {
  border-color: color-mix(
    in srgb,
    var(--color-success) 22%,
    var(--color-border-light)
  );
}

.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px;
  border-radius: 16px;
  border: 1px solid var(--color-border-light);
  background: var(--color-bg-card);
  box-shadow: var(--shadow-sm);
}

.filter-left,
.filter-right {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.keyword-filter {
  width: 280px;
}

.loading-state {
  text-align: center;
  padding: 60px 0;
  color: var(--color-text-secondary);
}

.access-card {
  border-radius: 16px;
}

.access-card :deep(.el-card__body) {
  padding: 28px 20px;
}

.access-card__description {
  margin: 0;
  color: var(--color-text-secondary);
  line-height: 1.8;
}

.access-card__actions {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 12px;
}

.loading-icon {
  font-size: 32px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 18px;
  align-items: stretch;
}

.nodes-table {
  border-radius: 16px;
  overflow: hidden;
}

.node-info-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.node-endpoint {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  padding: 5px 10px;
  border-radius: 999px;
  background: var(--color-border-light);
  border: 1px solid var(--color-border);
  color: var(--color-text-primary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-all;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.node-name-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.node-name-meta {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.node-flag {
  font-size: 18px;
  line-height: 1.2;
}

.node-name {
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.45;
  word-break: break-word;
}

.node-subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1.5;
  word-break: break-word;
}

.table-load-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.table-load-cell .el-progress {
  flex: 1;
  min-width: 0;
}

.load-text {
  font-size: 12px;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.latency-testing {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--color-primary);
}

.latency-good {
  color: var(--color-success);
  font-weight: 600;
}

.latency-medium,
.latency-fair {
  color: var(--color-warning);
  font-weight: 600;
}

.latency-bad,
.latency-poor {
  color: var(--color-danger);
  font-weight: 600;
}

.latency-unknown {
  color: var(--color-text-placeholder);
}

@media (max-width: 992px) {
  .page-header {
    flex-direction: column;
  }

  .header-stats {
    justify-content: flex-start;
  }
}

@media (max-width: 768px) {
  .nodes-page {
    padding: 16px;
  }

  .filter-bar {
    padding: 14px;
  }

  .filter-left,
  .filter-right {
    display: grid;
    grid-template-columns: 1fr;
    width: 100%;
  }

  .keyword-filter {
    width: 100%;
  }

  .filter-left :deep(.el-select),
  .filter-right :deep(.el-select),
  .filter-right :deep(.el-button-group),
  .filter-right :deep(.el-button) {
    width: 100% !important;
  }

  .nodes-grid {
    grid-template-columns: 1fr;
  }

  .access-card__actions {
    flex-direction: column;
  }
}
</style>
