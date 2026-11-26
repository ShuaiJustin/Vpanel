<template>
  <div :class="['page-section-header', `page-section-header--${align}`]">
    <div class="page-section-header__copy">
      <div class="page-section-header__title">
        {{ title }}
      </div>
      <div
        v-if="subtitle"
        class="page-section-header__subtitle"
      >
        {{ subtitle }}
      </div>
    </div>
    <slot />
  </div>
</template>

<script setup>
defineProps({
  title: {
    type: String,
    required: true
  },
  subtitle: {
    type: String,
    default: ''
  },
  align: {
    type: String,
    default: 'start',
    validator: (value) => ['start', 'center'].includes(value)
  }
})
</script>

<style scoped>
.page-section-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.page-section-header--start {
  align-items: flex-start;
}

.page-section-header--center {
  align-items: center;
}

.page-section-header__copy {
  min-width: 0;
}

.page-section-header__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-section-header__subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
