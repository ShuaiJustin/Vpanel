<template>
  <div
    class="empty-state"
    :class="[size, { 'with-border': bordered }]"
  >
    <!-- 图标或图片 -->
    <div class="empty-state-icon">
      <slot name="icon">
        <component
          :is="iconComponent"
          v-if="iconComponent"
        />
        <svg
          v-else
          viewBox="0 0 200 200"
          class="default-icon"
        >
          <circle
            cx="100"
            cy="80"
            r="40"
            fill="#e8e8e8"
          />
          <rect
            x="40"
            y="130"
            width="120"
            height="10"
            rx="5"
            fill="#e8e8e8"
          />
          <rect
            x="60"
            y="150"
            width="80"
            height="10"
            rx="5"
            fill="#f0f0f0"
          />
        </svg>
      </slot>
    </div>

    <!-- 标题 -->
    <div
      v-if="displayTitle || $slots.title"
      class="empty-state-title"
    >
      <slot name="title">
        {{ displayTitle }}
      </slot>
    </div>

    <!-- 描述 -->
    <div
      v-if="displayDescription || $slots.description"
      class="empty-state-description"
    >
      <slot name="description">
        {{ displayDescription }}
      </slot>
    </div>

    <!-- 操作按钮 -->
    <div
      v-if="$slots.actions || actionText"
      class="empty-state-actions"
    >
      <slot name="actions">
        <el-button
          type="primary"
          @click="$emit('action')"
        >
          {{ actionText }}
        </el-button>
      </slot>
    </div>
  </div>
</template>

<script setup>
/**
 * 空状态组件
 * 在没有数据时显示友好的提示
 */
import { computed } from 'vue'
import { 
  Document, 
  User, 
  Connection, 
  Setting, 
  Search,
  Warning,
  CircleClose
} from '@element-plus/icons-vue'

const props = defineProps({
  // 预设类型
  type: {
    type: String,
    default: 'default',
    validator: (value) => [
      'default', 'no-data', 'no-result', 'no-permission', 
      'error', 'no-network', 'no-user', 'no-proxy'
    ].includes(value)
  },
  // 标题
  title: {
    type: String,
    default: ''
  },
  // 描述
  description: {
    type: String,
    default: ''
  },
  // 操作按钮文字
  actionText: {
    type: String,
    default: ''
  },
  // 尺寸
  size: {
    type: String,
    default: 'default',
    validator: (value) => ['small', 'default', 'large'].includes(value)
  },
  // 是否显示边框
  bordered: {
    type: Boolean,
    default: false
  }
})

defineEmits(['action'])

// 预设配置
const presets = {
  'default': {
    title: '暂无数据',
    description: '当前没有可显示的内容',
    icon: null
  },
  'no-data': {
    title: '暂无数据',
    description: '还没有任何数据，快去添加吧',
    icon: Document
  },
  'no-result': {
    title: '未找到结果',
    description: '没有找到匹配的结果，请尝试其他搜索条件',
    icon: Search
  },
  'no-permission': {
    title: '无权限访问',
    description: '您没有权限查看此内容，请联系管理员',
    icon: Warning
  },
  'error': {
    title: '加载失败',
    description: '数据加载出错，请稍后重试',
    icon: CircleClose
  },
  'no-network': {
    title: '网络异常',
    description: '网络连接失败，请检查网络设置',
    icon: Connection
  },
  'no-user': {
    title: '暂无用户',
    description: '还没有任何用户，快去添加吧',
    icon: User
  },
  'no-proxy': {
    title: '暂无代理',
    description: '还没有配置任何代理，快去添加吧',
    icon: Setting
  }
}

// 获取预设配置
const preset = computed(() => presets[props.type] || presets.default)

// 图标组件
const iconComponent = computed(() => {
  return preset.value.icon
})

// 实际显示的标题
const displayTitle = computed(() => props.title || preset.value.title)

// 实际显示的描述
const displayDescription = computed(() => props.description || preset.value.description)
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  text-align: center;
}

.empty-state.with-border {
  border: 1px dashed #e0e0e0;
  border-radius: 8px;
  background: #fafafa;
}

/* 尺寸 */
.empty-state.small {
  padding: 20px 16px;
}

.empty-state.small .empty-state-icon {
  width: 60px;
  height: 60px;
}

.empty-state.small .empty-state-title {
  font-size: 14px;
}

.empty-state.small .empty-state-description {
  font-size: 12px;
}

.empty-state.large {
  padding: 60px 40px;
}

.empty-state.large .empty-state-icon {
  width: 120px;
  height: 120px;
}

.empty-state.large .empty-state-title {
  font-size: 20px;
}

/* 图标 */
.empty-state-icon {
  width: 80px;
  height: 80px;
  margin-bottom: 16px;
  color: #c0c4cc;
}

.empty-state-icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.default-icon {
  width: 100%;
  height: 100%;
}

/* 标题 */
.empty-state-title {
  font-size: 16px;
  font-weight: 500;
  color: var(--color-text-primary);
  margin-bottom: 8px;
}

/* 描述 */
.empty-state-description {
  font-size: 14px;
  color: #909399;
  max-width: 300px;
  line-height: 1.5;
}

/* 操作按钮 */
.empty-state-actions {
  margin-top: 20px;
}
</style>
