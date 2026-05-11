<template>
  <div class="node-card" :class="{ offline: node.status !== 'online' }">
    <div class="card-header">
      <div class="endpoint-badge" :title="endpoint">
        <span class="endpoint-label">节点地址</span>
        <span class="endpoint-value">{{ endpoint }}</span>
      </div>
      <el-tag :type="statusType" size="small" effect="light">
        {{ statusLabel }}
      </el-tag>
    </div>

    <div class="card-main">
      <div class="node-heading">
        <span class="node-flag">{{ regionFlag }}</span>
        <div class="node-details">
          <h3 class="node-name">
            {{ displayName }}
          </h3>
          <p v-if="showSubtitle" class="node-subtitle">
            {{ subtitle }}
          </p>
        </div>
      </div>

      <div class="node-meta">
        <div class="meta-chip">
          <span class="meta-chip-label">地区</span>
          <strong>{{ regionLabel }}</strong>
        </div>
        <div class="meta-chip">
          <span class="meta-chip-label">协议</span>
          <strong>{{ protocolLabel }}</strong>
        </div>
        <div v-if="portLabel" class="meta-chip">
          <span class="meta-chip-label">端口</span>
          <strong>{{ portLabel }}</strong>
        </div>
      </div>

      <div class="metric-grid">
        <div class="metric-card">
          <span class="metric-label">延迟</span>
          <div class="metric-value">
            <span v-if="testing" class="latency-testing">
              <el-icon class="is-loading"><Loading /></el-icon>
              测速中...
            </span>
            <span v-else-if="hasLatency" :class="latencyClass">
              {{ latency }}ms
            </span>
            <span v-else class="latency-unknown"> 未测试 </span>
          </div>
        </div>

        <div class="metric-card metric-card-load">
          <span class="metric-label">负载</span>
          <div class="load-bar">
            <el-progress
              :percentage="normalizedLoad"
              :color="loadColor"
              :stroke-width="6"
              :show-text="false"
            />
            <span class="load-text">{{ normalizedLoad }}%</span>
          </div>
        </div>
      </div>
    </div>

    <div class="card-footer">
      <el-button
        size="small"
        :loading="testing"
        :disabled="node.status !== 'online'"
        @click="$emit('test')"
      >
        <el-icon><Timer /></el-icon>
        测速
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue";
import { Timer, Loading } from "@element-plus/icons-vue";
import {
  getNodeLatencyClass,
  getNodeRegionFlag,
  getNodeRegionLabel,
  getNodeStatusText,
  getNodeStatusType,
  getProtocolDisplayName,
} from "@/composables/useNodePresentation";

const props = defineProps({
  node: {
    type: Object,
    required: true,
  },
  latency: {
    type: Number,
    default: null,
  },
  testing: {
    type: Boolean,
    default: false,
  },
});

defineEmits(["test"]);

const displayName = computed(
  () => props.node.display_name || props.node.name || "未命名节点",
);
const regionLabel = computed(
  () => props.node.region_label || getNodeRegionLabel(props.node.region),
);
const regionFlag = computed(() =>
  getNodeRegionFlag(props.node.region_label || props.node.region),
);
const protocolLabel = computed(
  () =>
    props.node.protocol_label || getProtocolDisplayName(props.node.protocol),
);
const endpoint = computed(() => {
  if (!props.node.host) return "-";
  return props.node.port
    ? `${props.node.host}:${props.node.port}`
    : props.node.host;
});
const fallbackSubtitle = computed(() =>
  [protocolLabel.value, endpoint.value !== "-" ? endpoint.value : ""]
    .filter(Boolean)
    .join(" · "),
);
const subtitle = computed(() => props.node.subtitle || "");
const showSubtitle = computed(() => {
  const value = String(subtitle.value || "").trim();
  return Boolean(value) && value !== fallbackSubtitle.value;
});
const statusType = computed(() => {
  if (props.node.status === "maintenance") return "warning";
  return getNodeStatusType(props.node.status);
});
const statusLabel = computed(() => {
  if (props.node.status === "maintenance") return "维护中";
  return getNodeStatusText(props.node.status);
});
const hasLatency = computed(
  () => typeof props.latency === "number" && props.latency >= 0,
);
const latencyClass = computed(() => getNodeLatencyClass(props.latency));
const normalizedLoad = computed(() => {
  const load = Number(props.node.load);
  if (!Number.isFinite(load) || load < 0) return 0;
  if (load > 100) return 100;
  return Math.round(load);
});
const portLabel = computed(() =>
  props.node.port ? String(props.node.port) : "",
);
const loadColor = computed(() => {
  if (normalizedLoad.value >= 80) return "#f56c6c";
  if (normalizedLoad.value >= 60) return "#e6a23c";
  return "#67c23a";
});
</script>

<style scoped>
.node-card {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  background: var(--color-bg-card);
  border-radius: 16px;
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  transition:
    transform 0.25s ease,
    box-shadow 0.25s ease,
    border-color 0.25s ease;
  border: 1px solid var(--color-border-light);
}

.node-card:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-3px);
  border-color: color-mix(
    in srgb,
    var(--color-primary) 20%,
    var(--color-border-light)
  );
}

.node-card.offline {
  opacity: 0.8;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  padding: 18px 18px 0;
}

.endpoint-badge {
  display: inline-flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  max-width: 100%;
  padding: 10px 12px;
  border-radius: 12px;
  background: var(--color-border-light);
  border: 1px solid var(--color-border);
}

.endpoint-label {
  font-size: 12px;
  line-height: 1;
  color: var(--color-text-secondary);
}

.endpoint-value {
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text-primary);
  word-break: break-all;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.card-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 18px;
  padding: 16px 18px 18px;
}

.node-heading {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}

.node-flag {
  font-size: 28px;
  line-height: 1;
  flex-shrink: 0;
}

.node-details {
  min-width: 0;
  flex: 1;
}

.node-name {
  margin: 0;
  font-size: 18px;
  line-height: 1.45;
  font-weight: 700;
  color: var(--color-text-primary);
  word-break: break-word;
}

.node-subtitle {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-secondary);
  word-break: break-word;
}

.node-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 8px 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--color-primary) 8%, var(--color-bg-card));
  border: 1px solid
    color-mix(in srgb, var(--color-primary) 16%, var(--color-border));
  color: var(--color-text-primary);
}

.meta-chip-label {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.meta-chip strong {
  font-size: 13px;
  line-height: 1.4;
  word-break: break-word;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 14px;
  background: color-mix(
    in srgb,
    var(--color-bg-card) 86%,
    var(--color-border-light)
  );
  border: 1px solid var(--color-border-light);
}

.metric-label {
  font-size: 12px;
  line-height: 1;
  color: var(--color-text-secondary);
}

.metric-value {
  min-height: 24px;
  display: flex;
  align-items: center;
  font-size: 15px;
  color: var(--color-text-primary);
}

.metric-card-load {
  min-width: 0;
}

.load-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.load-bar .el-progress {
  flex: 1;
  min-width: 0;
}

.load-text {
  font-size: 12px;
  color: var(--color-text-regular);
  min-width: 38px;
  text-align: right;
}

.latency-testing {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--color-primary);
  font-size: 14px;
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
  font-size: 14px;
}

.card-footer {
  display: flex;
  gap: 10px;
  padding: 14px 18px 18px;
  border-top: 1px solid var(--color-border-light);
  background: color-mix(
    in srgb,
    var(--color-bg-card) 94%,
    var(--color-border-light)
  );
}

.card-footer .el-button {
  flex: 1;
}

@media (max-width: 640px) {
  .card-header,
  .card-main,
  .card-footer {
    padding-left: 14px;
    padding-right: 14px;
  }

  .card-header {
    flex-direction: column;
    align-items: stretch;
  }

  .metric-grid {
    grid-template-columns: 1fr;
  }

  .card-footer {
    flex-direction: column;
  }
}
</style>
