<template>
  <el-dialog
    v-model="visible"
    title="生成配置预览（Xray）"
    width="760px"
    destroy-on-close
  >
    <div class="config-preview-toolbar">
      <span>节点</span>
      <el-select
        v-model="selectedNodeId"
        placeholder="选择节点"
        filterable
        style="width: 260px"
      >
        <el-option
          v-for="node in nodes"
          :key="node.id"
          :label="`${node.name || '节点'} #${node.id}`"
          :value="node.id"
        />
      </el-select>
      <el-button
        type="primary"
        :loading="loading"
        :disabled="!selectedNodeId"
        @click="loadPreview"
      >
        生成预览
      </el-button>
    </div>

    <el-alert
      v-if="summary"
      type="info"
      :closable="false"
      show-icon
      title="客户端数才是用户数"
      description="合并后 inbound 数会下降，但 client_count 不变。每个 client 保留原 email（user-&lt;uid&gt;-proxy-&lt;pid&gt;），流量统计口径不变。"
    />

    <el-descriptions
      v-if="summary"
      :column="3"
      border
      size="small"
      class="config-preview-summary"
    >
      <el-descriptions-item label="入站数">
        {{ summary.inbound_count }}
      </el-descriptions-item>
      <el-descriptions-item label="客户端数">
        {{ summary.client_count }}
      </el-descriptions-item>
      <el-descriptions-item label="合并入站数">
        {{ summary.merged_inbound_count }}
      </el-descriptions-item>
      <el-descriptions-item label="合并开关">
        <el-tag :type="summary.merge_enabled ? 'success' : 'info'" size="small">
          {{ summary.merge_enabled ? '已开启' : '未开启（兼容模式）' }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="节点 ID">
        {{ summary.node_id }}
      </el-descriptions-item>
      <el-descriptions-item label="配置 JSON">
        {{ configValid ? '可解析' : '-' }}
      </el-descriptions-item>
    </el-descriptions>

    <div v-if="configText" class="config-preview-json">
      <div class="config-preview-json-head">生成的 config.json</div>
      <pre>{{ configText }}</pre>
    </div>

    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/index'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  nodes: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:modelValue'])

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const selectedNodeId = ref('')
const loading = ref(false)
const summary = ref(null)
const configText = ref('')

watch(
  () => props.nodes,
  (list) => {
    if (!selectedNodeId.value && list.length) {
      selectedNodeId.value = list[0].id
    }
  },
  { immediate: true }
)

const configValid = computed(() => {
  if (!configText.value) return false
  try {
    JSON.parse(configText.value)
    return true
  } catch {
    return false
  }
})

const loadPreview = async () => {
  if (!selectedNodeId.value) return
  loading.value = true
  try {
    const payload = await api.get(`/admin/nodes/${selectedNodeId.value}/config/preview`)
    const data =
      payload?.data && typeof payload.data === 'object' && !Array.isArray(payload.data)
        ? payload.data
        : payload
    if (!data || data.success === false) {
      throw new Error(data?.message || '生成失败')
    }

    summary.value = {
      node_id: data.node_id,
      inbound_count: data.inbound_count,
      client_count: data.client_count,
      merged_inbound_count: data.merged_inbound_count,
      merge_enabled: data.merge_enabled
    }
    const raw = data.config || ''
    try {
      configText.value = JSON.stringify(JSON.parse(raw), null, 2)
    } catch {
      configText.value = raw
    }
  } catch (error) {
    ElMessage.error(`生成配置预览失败：${error?.message || error}`)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.config-preview-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}
.config-preview-summary {
  margin-top: 12px;
}
.config-preview-json {
  margin-top: 16px;
}
.config-preview-json-head {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
}
.config-preview-json pre {
  max-height: 320px;
  overflow: auto;
  background: var(--el-fill-color-light, #f5f7fa);
  padding: 12px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
