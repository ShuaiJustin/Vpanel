<template>
  <div class="form-validation">
    <el-form
      ref="formRef"
      :model="modelValue"
      :rules="computedRules"
      :label-width="labelWidth"
      :label-position="labelPosition"
      :inline="inline"
      :size="size"
      :disabled="disabled"
      :validate-on-rule-change="false"
      @validate="handleValidate"
    >
      <slot />
      
      <!-- 表单操作按钮 -->
      <el-form-item
        v-if="showActions"
        class="form-actions"
      >
        <slot name="actions">
          <el-button
            type="primary"
            :loading="submitting"
            :disabled="!isValid || disabled"
            @click="handleSubmit"
          >
            {{ submitText }}
          </el-button>
          <el-button
            v-if="showReset"
            @click="handleReset"
          >
            {{ resetText }}
          </el-button>
          <el-button
            v-if="showCancel"
            @click="$emit('cancel')"
          >
            {{ cancelText }}
          </el-button>
        </slot>
      </el-form-item>
    </el-form>
    
    <!-- 验证摘要 -->
    <div
      v-if="showSummary && hasErrors"
      class="validation-summary"
    >
      <el-alert
        type="error"
        :closable="false"
        show-icon
      >
        <template #title>
          请修正以下错误：
        </template>
        <ul class="error-list">
          <li
            v-for="(error, field) in errors"
            :key="field"
          >
            <span class="field-name">{{ getFieldLabel(field) }}:</span>
            <span class="error-message">{{ error }}</span>
          </li>
        </ul>
      </el-alert>
    </div>
  </div>
</template>

<script setup>
/**
 * 表单验证组件
 * 提供内联验证、实时反馈和验证摘要
 */
import { ref, computed, watch, provide } from 'vue'

const props = defineProps({
  // 表单数据 (v-model)
  modelValue: {
    type: Object,
    required: true
  },
  // 验证规则
  rules: {
    type: Object,
    default: () => ({})
  },
  // 标签宽度
  labelWidth: {
    type: String,
    default: '100px'
  },
  // 标签位置
  labelPosition: {
    type: String,
    default: 'right'
  },
  // 是否行内表单
  inline: {
    type: Boolean,
    default: false
  },
  // 表单尺寸
  size: {
    type: String,
    default: 'default'
  },
  // 是否禁用
  disabled: {
    type: Boolean,
    default: false
  },
  // 是否显示操作按钮
  showActions: {
    type: Boolean,
    default: true
  },
  // 提交按钮文本
  submitText: {
    type: String,
    default: '提交'
  },
  // 是否显示重置按钮
  showReset: {
    type: Boolean,
    default: true
  },
  // 重置按钮文本
  resetText: {
    type: String,
    default: '重置'
  },
  // 是否显示取消按钮
  showCancel: {
    type: Boolean,
    default: false
  },
  // 取消按钮文本
  cancelText: {
    type: String,
    default: '取消'
  },
  // 是否显示验证摘要
  showSummary: {
    type: Boolean,
    default: false
  },
  // 是否实时验证
  validateOnChange: {
    type: Boolean,
    default: true
  },
  // 字段标签映射
  fieldLabels: {
    type: Object,
    default: () => ({})
  },
  // 是否正在提交
  submitting: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue', 'submit', 'reset', 'cancel', 'validate'])

// 表单引用
const formRef = ref(null)

// 错误信息
const errors = ref({})

// 是否有错误
const hasErrors = computed(() => Object.keys(errors.value).length > 0)

// 是否有效
const isValid = ref(true)

// 计算规则（添加实时验证触发器）
const computedRules = computed(() => {
  if (!props.validateOnChange) {
    return props.rules
  }
  
  const rules = {}
  for (const [field, fieldRules] of Object.entries(props.rules)) {
    rules[field] = (Array.isArray(fieldRules) ? fieldRules : [fieldRules]).map(rule => ({
      ...rule,
      trigger: rule.trigger || ['blur', 'change']
    }))
  }
  return rules
})

// 获取字段标签
const getFieldLabel = (field) => {
  return props.fieldLabels[field] || field
}

// 处理验证事件
const handleValidate = (prop, isValid, message) => {
  if (isValid) {
    delete errors.value[prop]
  } else {
    errors.value[prop] = message
  }
  emit('validate', { field: prop, valid: isValid, message })
}

// 验证表单
const validate = async () => {
  try {
    await formRef.value?.validate()
    isValid.value = true
    errors.value = {}
    return true
  } catch (e) {
    isValid.value = false
    return false
  }
}

// 验证单个字段
const validateField = async (field) => {
  try {
    await formRef.value?.validateField(field)
    delete errors.value[field]
    return true
  } catch (e) {
    return false
  }
}

// 清除验证
const clearValidate = (fields) => {
  formRef.value?.clearValidate(fields)
  if (fields) {
    const fieldList = Array.isArray(fields) ? fields : [fields]
    fieldList.forEach(f => delete errors.value[f])
  } else {
    errors.value = {}
  }
  isValid.value = true
}

// 重置字段
const resetFields = () => {
  formRef.value?.resetFields()
  errors.value = {}
  isValid.value = true
}

// 处理提交
const handleSubmit = async () => {
  const valid = await validate()
  if (valid) {
    emit('submit', props.modelValue)
  }
}

// 处理重置
const handleReset = () => {
  resetFields()
  emit('reset')
}

// 监听数据变化
watch(
  () => props.modelValue,
  () => {
    if (props.validateOnChange && Object.keys(errors.value).length > 0) {
      // 重新验证有错误的字段
      Object.keys(errors.value).forEach(field => {
        validateField(field)
      })
    }
  },
  { deep: true }
)

// 提供给子组件
provide('formValidation', {
  errors,
  validateField,
  clearValidate
})

// 暴露方法
defineExpose({
  validate,
  validateField,
  clearValidate,
  resetFields,
  getFormRef: () => formRef.value
})
</script>

<style scoped>
.form-validation {
  width: 100%;
}

.form-actions {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.validation-summary {
  margin-top: 16px;
}

.error-list {
  margin: 8px 0 0 0;
  padding-left: 20px;
  list-style: disc;
}

.error-list li {
  margin-bottom: 4px;
  font-size: 13px;
}

.field-name {
  font-weight: 500;
  margin-right: 4px;
}

.error-message {
  color: var(--el-color-danger);
}
</style>
