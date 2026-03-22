<template>
  <el-form-item
    :label="label"
    :prop="prop"
    :required="isRequired"
    :class="['form-field', { 'has-error': hasError, 'is-valid': isValid && showValidIcon }]"
  >
    <template
      v-if="$slots.label || tooltip"
      #label
    >
      <span class="field-label">
        <slot name="label">{{ label }}</slot>
        <el-tooltip
          v-if="tooltip"
          :content="tooltip"
          placement="top"
        >
          <el-icon class="label-tooltip"><QuestionFilled /></el-icon>
        </el-tooltip>
      </span>
    </template>
    
    <div class="field-content">
      <slot />
      
      <!-- 验证状态图标 -->
      <span
        v-if="showValidIcon && isValid && !hasError"
        class="valid-icon"
      >
        <el-icon color="var(--el-color-success)"><CircleCheck /></el-icon>
      </span>
    </div>
    
    <!-- 帮助文本 -->
    <div
      v-if="helpText && !hasError"
      class="field-help"
    >
      {{ helpText }}
    </div>
    
    <!-- 字符计数 -->
    <div
      v-if="showCount && maxLength"
      class="field-count"
      :class="{ 'count-warning': isNearLimit }"
    >
      {{ currentLength }} / {{ maxLength }}
    </div>
  </el-form-item>
</template>

<script setup>
/**
 * 表单字段组件
 * 提供增强的表单字段功能，包括帮助文本、字符计数、验证状态图标
 */
import { computed, inject } from 'vue'
import { QuestionFilled, CircleCheck } from '@element-plus/icons-vue'

const props = defineProps({
  // 字段标签
  label: {
    type: String,
    default: ''
  },
  // 字段属性名
  prop: {
    type: String,
    required: true
  },
  // 是否必填
  required: {
    type: Boolean,
    default: false
  },
  // 帮助文本
  helpText: {
    type: String,
    default: ''
  },
  // 提示信息
  tooltip: {
    type: String,
    default: ''
  },
  // 是否显示字符计数
  showCount: {
    type: Boolean,
    default: false
  },
  // 最大长度
  maxLength: {
    type: Number,
    default: 0
  },
  // 当前值（用于字符计数）
  modelValue: {
    type: [String, Number],
    default: ''
  },
  // 是否显示验证成功图标
  showValidIcon: {
    type: Boolean,
    default: false
  }
})

// 注入表单验证上下文
const formValidation = inject('formValidation', null)

// 是否必填
const isRequired = computed(() => props.required)

// 是否有错误
const hasError = computed(() => {
  if (!formValidation) return false
  return !!formValidation.errors.value[props.prop]
})

// 是否验证通过
const isValid = computed(() => {
  if (!formValidation) return false
  return !hasError.value && props.modelValue
})

// 当前长度
const currentLength = computed(() => {
  if (!props.modelValue) return 0
  return String(props.modelValue).length
})

// 是否接近限制
const isNearLimit = computed(() => {
  if (!props.maxLength) return false
  return currentLength.value >= props.maxLength * 0.9
})
</script>

<style scoped>
.form-field {
  position: relative;
}

.form-field.has-error :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--el-color-danger) inset;
}

.form-field.is-valid :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--el-color-success) inset;
}

.field-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.label-tooltip {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  cursor: help;
}

.field-content {
  position: relative;
  width: 100%;
}

.valid-icon {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 16px;
}

.field-help {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
  line-height: 1.4;
}

.field-count {
  position: absolute;
  right: 0;
  bottom: -20px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.field-count.count-warning {
  color: var(--el-color-warning);
}
</style>
