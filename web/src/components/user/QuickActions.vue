<template>
  <el-card
    class="quick-actions-card"
    shadow="never"
  >
    <template #header>
      <span>{{ title }}</span>
    </template>

    <div
      class="actions-grid"
      :style="gridStyle"
    >
      <div 
        v-for="action in actions" 
        :key="action.key"
        class="action-item"
        :class="{ disabled: action.disabled }"
        @click="handleClick(action)"
      >
        <div
          class="action-icon"
          :style="{ color: action.color || '#409eff' }"
        >
          <el-icon><component :is="action.icon" /></el-icon>
        </div>
        <span class="action-label">{{ action.label }}</span>
        <el-badge 
          v-if="action.badge" 
          :value="action.badge" 
          :type="action.badgeType || 'danger'"
          class="action-badge"
        />
      </div>
    </div>
  </el-card>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  title: {
    type: String,
    default: '快捷操作'
  },
  actions: {
    type: Array,
    required: true,
    // 每个 action 对象包含: key, label, icon, color, badge, badgeType, disabled, route, handler
  },
  columns: {
    type: Number,
    default: 3
  }
})

const emit = defineEmits(['action'])

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${props.columns}, 1fr)`
}))

function handleClick(action) {
  if (action.disabled) return
  emit('action', action)
}
</script>

<style scoped>
.quick-actions-card {
  border-radius: 8px;
}

.actions-grid {
  display: grid;
  gap: 16px;
}

.action-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
}

.action-item:hover {
  background: #f5f7fa;
}

.action-item.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.action-item.disabled:hover {
  background: transparent;
}

.action-icon {
  font-size: 28px;
  margin-bottom: 8px;
}

.action-label {
  font-size: 13px;
  color: #606266;
  text-align: center;
}

.action-badge {
  position: absolute;
  top: 8px;
  right: 8px;
}

/* 响应式 */
@media (max-width: 576px) {
  .actions-grid {
    grid-template-columns: repeat(2, 1fr) !important;
  }
}
</style>
