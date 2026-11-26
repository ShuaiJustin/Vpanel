<template>
  <el-card
    class="proxy-card"
    shadow="hover"
  >
    <template #header>
      <div class="card-header">
        <h3 class="name">
          {{ proxy.name }}
        </h3>
        <div class="actions">
          <el-tooltip
            content="二维码"
            placement="top"
          >
            <el-button
              type="primary"
              text
              :icon="QrCode"
              @click="showQRCode = true"
            />
          </el-tooltip>
          <el-tooltip
            content="编辑"
            placement="top"
          >
            <el-button
              type="primary" 
              text
              :icon="Edit"
              @click="$emit('edit', proxy.id)"
            />
          </el-tooltip>
          <el-tooltip
            content="删除"
            placement="top"
          >
            <el-button
              type="danger"
              text
              :icon="Delete"
              @click="handleDelete"
            />
          </el-tooltip>
        </div>
      </div>
    </template>
    
    <div class="card-content">
      <div class="info-row">
        <span class="label">协议:</span>
        <span class="value protocol">
          <el-tag
            size="small"
            :type="getProtocolTagType(proxy.protocol)"
          >
            {{ proxy.protocol.toUpperCase() }}
          </el-tag>
        </span>
      </div>
      
      <div class="info-row">
        <span class="label">地址:</span>
        <span class="value">{{ proxy.server }}:{{ proxy.port }}</span>
      </div>
      
      <div
        v-if="proxy.remark"
        class="info-row"
      >
        <span class="label">备注:</span>
        <span class="value">{{ proxy.remark }}</span>
      </div>
    </div>
    
    <!-- 二维码对话框 -->
    <el-dialog
      v-model="showQRCode"
      title="分享代理"
      width="400px"
      :destroy-on-close="true"
    >
      <QRCodeGenerator :proxy-id="proxy.id" />
    </el-dialog>
    
    <!-- 删除确认对话框 -->
    <el-dialog
      v-model="showDeleteConfirm"
      title="确认删除"
      width="360px"
    >
      <div class="delete-confirm">
        <p>确定要删除代理 "{{ proxy.name }}" 吗？此操作无法撤销。</p>
        <div class="confirm-buttons">
          <el-button @click="showDeleteConfirm = false">
            取消
          </el-button>
          <el-button
            type="danger"
            @click="confirmDelete"
          >
            删除
          </el-button>
        </div>
      </div>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Edit, Delete, QrCode } from '@element-plus/icons-vue'
import QRCodeGenerator from './QRCodeGenerator.vue'

const props = defineProps({
  proxy: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['edit', 'delete'])

const showQRCode = ref(false)
const showDeleteConfirm = ref(false)

// 获取协议标签的类型
const getProtocolTagType = (protocol) => {
  const types = {
    'shadowsocks': '',
    'trojan': 'success',
    'vmess': 'warning',
    'vless': 'danger',
    'socks': 'info'
  }
  return types[protocol.toLowerCase()] || ''
}

// 处理删除按钮点击
const handleDelete = () => {
  showDeleteConfirm.value = true
}

// 确认删除
const confirmDelete = () => {
  try {
    // 向父组件发送删除事件
    emit('delete', props.proxy.id)
    showDeleteConfirm.value = false
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
  }
}
</script>

<style scoped>
.proxy-card {
  margin-bottom: 20px;
  transition: all 0.3s;
}

.proxy-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.1);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 70%;
}

.actions {
  display: flex;
  gap: 5px;
}

.card-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  align-items: flex-start;
  line-height: 1.5;
}

.label {
  flex: 0 0 60px;
  color: #909399;
  font-size: 14px;
}

.value {
  flex: 1;
  word-break: break-all;
}

.protocol {
  text-transform: uppercase;
}

.delete-confirm {
  text-align: center;
}

.confirm-buttons {
  margin-top: 20px;
  display: flex;
  justify-content: center;
  gap: 20px;
}
</style> 
