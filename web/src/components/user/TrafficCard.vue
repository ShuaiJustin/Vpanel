<template>
  <el-card class="traffic-card" shadow="never">
    <template #header>
      <div class="card-header">
        <span>{{ title }}</span>
        <slot name="extra" />
      </div>
    </template>

    <div class="traffic-content">
      <!-- 进度环 -->
      <div class="traffic-progress">
        <el-progress
          type="dashboard"
          :percentage="percentage"
          :width="size"
          :stroke-width="strokeWidth"
          :color="progressColor"
        >
          <template #default>
            <div class="progress-inner">
              <div class="progress-value">{{ percentage }}%</div>
              <div class="progress-label">
                {{ progressLabel }}
              </div>
            </div>
          </template>
        </el-progress>
      </div>

      <!-- 详情列表 -->
      <div v-if="showDetails" class="traffic-details">
        <div class="detail-item">
          <span class="detail-label">已使用</span>
          <span class="detail-value">{{ formatTraffic(used) }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">总流量</span>
          <span class="detail-value">{{ formatTraffic(total) }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">剩余</span>
          <span class="detail-value remaining">{{
            formatTraffic(remaining)
          }}</span>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({
  title: {
    type: String,
    default: "流量使用",
  },
  used: {
    type: Number,
    default: 0,
  },
  total: {
    type: Number,
    default: 0,
  },
  size: {
    type: Number,
    default: 140,
  },
  strokeWidth: {
    type: Number,
    default: 10,
  },
  showDetails: {
    type: Boolean,
    default: true,
  },
  progressLabel: {
    type: String,
    default: "已使用",
  },
});

// 计算属性
const percentage = computed(() => {
  if (!props.total) return 0;
  return Math.min(100, Math.round((props.used / props.total) * 100));
});

const remaining = computed(() => {
  return Math.max(0, props.total - props.used);
});

const progressColor = computed(() => {
  const percent = percentage.value;
  if (percent >= 90) return "#f56c6c";
  if (percent >= 70) return "#e6a23c";
  return "#409eff";
});

// 方法
function formatTraffic(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let size = bytes;
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024;
    i++;
  }
  return `${size.toFixed(2)} ${units[i]}`;
}
</script>

<style scoped>
.traffic-card {
  border-radius: 8px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.traffic-content {
  display: flex;
  align-items: center;
  gap: 32px;
}

.traffic-progress {
  flex-shrink: 0;
}

.progress-inner {
  text-align: center;
}

.progress-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.progress-label {
  font-size: 12px;
  color: #909399;
}

.traffic-details {
  flex: 1;
}

.detail-item {
  display: flex;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px solid #ebeef5;
}

.detail-item:last-child {
  border-bottom: none;
}

.detail-label {
  color: #909399;
  font-size: 14px;
}

.detail-value {
  font-weight: 500;
  color: var(--color-text-primary);
  font-size: 14px;
}

.detail-value.remaining {
  color: #67c23a;
}

/* 响应式 */
@media (max-width: 576px) {
  .traffic-content {
    flex-direction: column;
    gap: 20px;
  }
}

/* 深色模式适配 */
:global(.dark) .progress-value {
  color: var(--color-text-primary);
}

:global(.dark) .detail-value {
  color: var(--color-text-primary);
}

:global(.dark) .detail-item {
  border-bottom: 1px solid var(--color-border);
}
</style>
