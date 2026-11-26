<template>
  <component
    :is="variant === 'dot' ? 'span' : 'el-tag'"
    :class="['status-indicator', `status-${normalizedStatus}`, { 'is-dot': variant === 'dot' }]"
    :type="tagType"
    :size="size"
    :effect="effect"
  >
    <span
      v-if="variant === 'dot'"
      class="status-dot"
      :style="dotStyle"
    />
    <span
      v-if="showIcon && icon"
      class="status-icon"
    >
      <el-icon><component :is="icon" /></el-icon>
    </span>
    <span
      v-if="showLabel"
      class="status-label"
    >{{ displayLabel }}</span>
  </component>
</template>

<script setup>
/**
 * 状态颜色指示器组件
 * 提供统一的状态显示，支持多种变体和自定义状态
 */
import { computed } from 'vue'
import {
  CircleCheck,
  CircleClose,
  Warning,
  Loading,
  Clock,
  QuestionFilled,
  Remove
} from '@element-plus/icons-vue'

const props = defineProps({
  // 状态值
  status: {
    type: [String, Boolean, Number],
    required: true
  },
  // 显示变体: tag, dot, badge
  variant: {
    type: String,
    default: 'tag',
    validator: (v) => ['tag', 'dot', 'badge'].includes(v)
  },
  // 尺寸
  size: {
    type: String,
    default: 'default',
    validator: (v) => ['small', 'default', 'large'].includes(v)
  },
  // 标签效果
  effect: {
    type: String,
    default: 'light',
    validator: (v) => ['dark', 'light', 'plain'].includes(v)
  },
  // 是否显示图标
  showIcon: {
    type: Boolean,
    default: false
  },
  // 是否显示标签文本
  showLabel: {
    type: Boolean,
    default: true
  },
  // 自定义标签文本
  label: {
    type: String,
    default: ''
  },
  // 状态类型 (用于预设映射)
  type: {
    type: String,
    default: 'default',
    validator: (v) => ['default', 'proxy', 'user', 'connection', 'task', 'health'].includes(v)
  }
})

// 状态映射配置
const STATUS_CONFIG = {
  // 默认状态映射
  default: {
    // 布尔值
    true: { type: 'success', label: '启用', icon: CircleCheck },
    false: { type: 'danger', label: '禁用', icon: CircleClose },
    // 字符串状态
    active: { type: 'success', label: '活跃', icon: CircleCheck },
    inactive: { type: 'info', label: '未激活', icon: Remove },
    enabled: { type: 'success', label: '启用', icon: CircleCheck },
    disabled: { type: 'danger', label: '禁用', icon: CircleClose },
    running: { type: 'success', label: '运行中', icon: CircleCheck },
    stopped: { type: 'danger', label: '已停止', icon: CircleClose },
    pending: { type: 'warning', label: '等待中', icon: Clock },
    loading: { type: 'primary', label: '加载中', icon: Loading },
    error: { type: 'danger', label: '错误', icon: CircleClose },
    warning: { type: 'warning', label: '警告', icon: Warning },
    success: { type: 'success', label: '成功', icon: CircleCheck },
    failed: { type: 'danger', label: '失败', icon: CircleClose },
    unknown: { type: 'info', label: '未知', icon: QuestionFilled }
  },
  // 代理状态
  proxy: {
    true: { type: 'success', label: '运行中', icon: CircleCheck },
    false: { type: 'danger', label: '已停止', icon: CircleClose },
    running: { type: 'success', label: '运行中', icon: CircleCheck },
    stopped: { type: 'danger', label: '已停止', icon: CircleClose },
    starting: { type: 'warning', label: '启动中', icon: Loading },
    stopping: { type: 'warning', label: '停止中', icon: Loading },
    error: { type: 'danger', label: '错误', icon: CircleClose },
    disabled: { type: 'info', label: '已禁用', icon: Remove }
  },
  // 用户状态
  user: {
    true: { type: 'success', label: '启用', icon: CircleCheck },
    false: { type: 'danger', label: '禁用', icon: CircleClose },
    active: { type: 'success', label: '活跃', icon: CircleCheck },
    inactive: { type: 'info', label: '未激活', icon: Remove },
    locked: { type: 'warning', label: '已锁定', icon: Warning },
    expired: { type: 'danger', label: '已过期', icon: CircleClose },
    pending: { type: 'warning', label: '待审核', icon: Clock }
  },
  // 连接状态
  connection: {
    connected: { type: 'success', label: '已连接', icon: CircleCheck },
    disconnected: { type: 'danger', label: '已断开', icon: CircleClose },
    connecting: { type: 'warning', label: '连接中', icon: Loading },
    reconnecting: { type: 'warning', label: '重连中', icon: Loading },
    error: { type: 'danger', label: '连接错误', icon: CircleClose }
  },
  // 任务状态
  task: {
    pending: { type: 'info', label: '待执行', icon: Clock },
    running: { type: 'primary', label: '执行中', icon: Loading },
    completed: { type: 'success', label: '已完成', icon: CircleCheck },
    failed: { type: 'danger', label: '失败', icon: CircleClose },
    cancelled: { type: 'warning', label: '已取消', icon: Remove }
  },
  // 健康状态
  health: {
    healthy: { type: 'success', label: '健康', icon: CircleCheck },
    unhealthy: { type: 'danger', label: '不健康', icon: CircleClose },
    degraded: { type: 'warning', label: '降级', icon: Warning },
    unknown: { type: 'info', label: '未知', icon: QuestionFilled }
  }
}

// 颜色映射
const COLOR_MAP = {
  success: '#67c23a',
  warning: '#e6a23c',
  danger: '#f56c6c',
  info: '#909399',
  primary: '#409eff'
}

// 标准化状态值
const normalizedStatus = computed(() => {
  const status = props.status
  if (typeof status === 'boolean') {
    return String(status)
  }
  if (typeof status === 'number') {
    return status > 0 ? 'true' : 'false'
  }
  return String(status).toLowerCase()
})

// 获取状态配置
const statusConfig = computed(() => {
  const typeConfig = STATUS_CONFIG[props.type] || STATUS_CONFIG.default
  const config = typeConfig[normalizedStatus.value]
  
  if (config) {
    return config
  }
  
  // 回退到默认配置
  const defaultConfig = STATUS_CONFIG.default[normalizedStatus.value]
  if (defaultConfig) {
    return defaultConfig
  }
  
  // 未知状态
  return { type: 'info', label: normalizedStatus.value, icon: QuestionFilled }
})

// 标签类型
const tagType = computed(() => statusConfig.value.type)

// 显示标签
const displayLabel = computed(() => props.label || statusConfig.value.label)

// 图标
const icon = computed(() => statusConfig.value.icon)

// 圆点样式
const dotStyle = computed(() => ({
  backgroundColor: COLOR_MAP[tagType.value] || COLOR_MAP.info
}))
</script>

<style scoped>
.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.status-indicator.is-dot {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
}

.status-icon {
  display: inline-flex;
  align-items: center;
}

.status-label {
  white-space: nowrap;
}

/* 动画效果 - 运行中状态 */
.status-running .status-dot,
.status-loading .status-dot,
.status-connecting .status-dot,
.status-starting .status-dot {
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.6;
    transform: scale(1.1);
  }
}

/* 尺寸变体 */
.status-indicator :deep(.el-tag--small) .status-dot {
  width: 6px;
  height: 6px;
}

.status-indicator :deep(.el-tag--large) .status-dot {
  width: 10px;
  height: 10px;
}
</style>
