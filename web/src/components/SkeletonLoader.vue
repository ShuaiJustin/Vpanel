<template>
  <div
    class="skeleton-wrapper"
    :class="{ animated }"
  >
    <!-- 卡片骨架 -->
    <template v-if="type === 'card'">
      <div
        v-for="i in count"
        :key="i"
        class="skeleton-card"
      >
        <div class="skeleton-card-header">
          <div class="skeleton-avatar" />
          <div class="skeleton-text-group">
            <div class="skeleton-text skeleton-text-title" />
            <div class="skeleton-text skeleton-text-subtitle" />
          </div>
        </div>
        <div class="skeleton-card-body">
          <div
            v-for="j in 3"
            :key="j"
            class="skeleton-text"
          />
        </div>
      </div>
    </template>

    <!-- 表格骨架 -->
    <template v-else-if="type === 'table'">
      <div class="skeleton-table">
        <div class="skeleton-table-header">
          <div
            v-for="i in columns"
            :key="i"
            class="skeleton-cell"
          />
        </div>
        <div
          v-for="i in rows"
          :key="i"
          class="skeleton-table-row"
        >
          <div
            v-for="j in columns"
            :key="j"
            class="skeleton-cell"
          />
        </div>
      </div>
    </template>

    <!-- 列表骨架 -->
    <template v-else-if="type === 'list'">
      <div
        v-for="i in count"
        :key="i"
        class="skeleton-list-item"
      >
        <div
          v-if="showAvatar"
          class="skeleton-avatar"
        />
        <div class="skeleton-text-group">
          <div class="skeleton-text skeleton-text-title" />
          <div class="skeleton-text skeleton-text-subtitle" />
        </div>
      </div>
    </template>

    <!-- 统计卡片骨架 -->
    <template v-else-if="type === 'stats'">
      <div
        v-for="i in count"
        :key="i"
        class="skeleton-stats"
      >
        <div class="skeleton-stats-icon" />
        <div class="skeleton-text-group">
          <div class="skeleton-text skeleton-text-number" />
          <div class="skeleton-text skeleton-text-label" />
        </div>
      </div>
    </template>

    <!-- 图表骨架 -->
    <template v-else-if="type === 'chart'">
      <div class="skeleton-chart">
        <div class="skeleton-chart-title" />
        <div class="skeleton-chart-area" />
      </div>
    </template>

    <!-- 表单骨架 -->
    <template v-else-if="type === 'form'">
      <div
        v-for="i in count"
        :key="i"
        class="skeleton-form-item"
      >
        <div class="skeleton-text skeleton-text-label" />
        <div class="skeleton-input" />
      </div>
    </template>

    <!-- 默认文本骨架 -->
    <template v-else>
      <div
        v-for="i in count"
        :key="i"
        class="skeleton-text"
        :style="{ width: getRandomWidth() }"
      />
    </template>
  </div>
</template>

<script setup>
/**
 * 骨架屏加载组件
 * 在数据加载时显示占位内容
 */
defineProps({
  // 骨架类型
  type: {
    type: String,
    default: 'text',
    validator: (value) => ['text', 'card', 'table', 'list', 'stats', 'chart', 'form'].includes(value)
  },
  // 数量
  count: {
    type: Number,
    default: 3
  },
  // 表格行数
  rows: {
    type: Number,
    default: 5
  },
  // 表格列数
  columns: {
    type: Number,
    default: 4
  },
  // 是否显示头像
  showAvatar: {
    type: Boolean,
    default: true
  },
  // 是否启用动画
  animated: {
    type: Boolean,
    default: true
  }
})

// 生成随机宽度
const getRandomWidth = () => {
  const widths = ['60%', '70%', '80%', '90%', '100%']
  return widths[Math.floor(Math.random() * widths.length)]
}
</script>

<style scoped>
.skeleton-wrapper {
  width: 100%;
}

.skeleton-wrapper.animated .skeleton-text,
.skeleton-wrapper.animated .skeleton-avatar,
.skeleton-wrapper.animated .skeleton-cell,
.skeleton-wrapper.animated .skeleton-input,
.skeleton-wrapper.animated .skeleton-stats-icon,
.skeleton-wrapper.animated .skeleton-chart-area,
.skeleton-wrapper.animated .skeleton-chart-title {
  background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
  background-size: 200% 100%;
  animation: skeleton-loading 1.5s infinite;
}

@keyframes skeleton-loading {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

/* 基础元素 */
.skeleton-text {
  height: 16px;
  background-color: #f0f0f0;
  border-radius: 4px;
  margin-bottom: 8px;
}

.skeleton-text-title {
  width: 40%;
  height: 20px;
}

.skeleton-text-subtitle {
  width: 60%;
  height: 14px;
}

.skeleton-text-number {
  width: 80px;
  height: 28px;
}

.skeleton-text-label {
  width: 100px;
  height: 14px;
}

.skeleton-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background-color: #f0f0f0;
  flex-shrink: 0;
}

.skeleton-text-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 卡片骨架 */
.skeleton-card {
  background: var(--color-bg-card);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.skeleton-card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.skeleton-card-body .skeleton-text:last-child {
  width: 70%;
}

/* 表格骨架 */
.skeleton-table {
  width: 100%;
  border-radius: 8px;
  overflow: hidden;
  background: var(--color-bg-card);
}

.skeleton-table-header {
  display: flex;
  background: #fafafa;
  padding: 12px 16px;
  gap: 16px;
}

.skeleton-table-row {
  display: flex;
  padding: 16px;
  gap: 16px;
  border-bottom: 1px solid #f0f0f0;
}

.skeleton-cell {
  flex: 1;
  height: 16px;
  background-color: #f0f0f0;
  border-radius: 4px;
}

/* 列表骨架 */
.skeleton-list-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}

.skeleton-list-item:last-child {
  border-bottom: none;
}

/* 统计卡片骨架 */
.skeleton-stats {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: var(--color-bg-card);
  border-radius: 8px;
  margin-bottom: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.skeleton-stats-icon {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  background-color: #f0f0f0;
  flex-shrink: 0;
}

/* 图表骨架 */
.skeleton-chart {
  background: var(--color-bg-card);
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.skeleton-chart-title {
  width: 120px;
  height: 20px;
  background-color: #f0f0f0;
  border-radius: 4px;
  margin-bottom: 16px;
}

.skeleton-chart-area {
  width: 100%;
  height: 200px;
  background-color: #f0f0f0;
  border-radius: 4px;
}

/* 表单骨架 */
.skeleton-form-item {
  margin-bottom: 20px;
}

.skeleton-form-item .skeleton-text-label {
  margin-bottom: 8px;
}

.skeleton-input {
  width: 100%;
  height: 40px;
  background-color: #f0f0f0;
  border-radius: 4px;
}
</style>
