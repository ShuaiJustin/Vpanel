<template>
  <div class="dokodemo-qrcode-container">
    <div v-if="loading" class="qrcode-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>生成二维码中...</span>
    </div>
    <div v-else-if="error" class="qrcode-error">
      <el-icon><Warning /></el-icon>
      <span>{{ error }}</span>
    </div>
    <div v-else class="qrcode-content">
      <div class="protocol-header">
        <span class="protocol-badge">Dokodemo-Door</span>
        <span class="protocol-name">{{ connectionName }}</span>
      </div>
      
      <div class="qrcode-display">
        <img v-if="qrCodeImage" :src="qrCodeImage" alt="Dokodemo QR Code" />
        <div v-else ref="qrcodeTarget" class="qrcode-element"></div>
      </div>
      
      <div class="connection-details">
        <div class="connection-item">
          <span class="item-label">目标地址</span>
          <span class="item-value">{{ address }}</span>
        </div>
        <div class="connection-item">
          <span class="item-label">目标端口</span>
          <span class="item-value">{{ port }}</span>
        </div>
        <div class="connection-item">
          <span class="item-label">网络类型</span>
          <span class="item-value">{{ network.toUpperCase() }}</span>
        </div>
        <div class="connection-item">
          <span class="item-label">转发模式</span>
          <span class="item-value">{{ followRedirect ? '开启重定向' : '直接转发' }}</span>
        </div>
      </div>
      
      <div class="share-link">
        <el-input 
          v-model="shareLink" 
          readonly 
          size="small"
          class="share-link-input"
        >
          <template #append>
            <el-button type="primary" @click="copyLink">
              <el-icon><DocumentCopy /></el-icon>
            </el-button>
          </template>
        </el-input>
      </div>
      
      <div class="actions">
        <el-button type="primary" @click="downloadQRCode">
          <el-icon class="el-icon--left"><Download /></el-icon>
          下载二维码
        </el-button>
        <el-button @click="regenerateQRCode">
          <el-icon class="el-icon--left"><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading, Warning, DocumentCopy, Download, Refresh } from '@element-plus/icons-vue'
import QRCode from 'qrcode'
import { copyText } from '@/utils/clipboard'

const props = defineProps({
  // 连接名称
  connectionName: {
    type: String,
    default: 'Dokodemo-Door 连接'
  },
  // 目标地址
  address: {
    type: String,
    required: true
  },
  // 端口
  port: {
    type: [Number, String],
    default: 80
  },
  // 网络类型 (tcp/udp/tcp,udp)
  network: {
    type: String,
    default: 'tcp'
  },
  // 跟随重定向
  followRedirect: {
    type: Boolean,
    default: false
  },
  // 超时时间(秒)
  timeout: {
    type: [Number, String],
    default: 300
  },
  // 自定义备注
  remark: {
    type: String,
    default: ''
  }
})

// 状态
const loading = ref(false)
const error = ref('')
const qrCodeImage = ref('')
const qrcodeTarget = ref(null)

// 计算分享链接
const shareLink = computed(() => {
  // dokodemo://address:port?network=tcp&timeout=300&followRedirect=true#remark
  let link = `dokodemo://${props.address}:${props.port}`
  
  // 添加参数
  const params = new URLSearchParams()
  
  // 添加网络类型
  if (props.network && props.network !== 'tcp') {
    params.append('network', props.network)
  }
  
  // 添加超时
  if (props.timeout && props.timeout !== 300) {
    params.append('timeout', props.timeout)
  }
  
  // 添加重定向
  if (props.followRedirect) {
    params.append('followRedirect', 'true')
  }
  
  // 添加参数
  const queryString = params.toString()
  if (queryString) {
    link += `?${queryString}`
  }
  
  // 添加备注
  const commentText = props.remark || props.connectionName
  if (commentText) {
    link += `#${encodeURIComponent(commentText)}`
  }
  
  return link
})

// 生成二维码
const generateQRCode = async () => {
  if (!shareLink.value) {
    error.value = '无法生成Dokodemo-Door链接，请检查必填参数'
    return
  }
  
  loading.value = true
  error.value = ''
  
  try {
    // 生成二维码
    const dataUrl = await QRCode.toDataURL(shareLink.value, {
      errorCorrectionLevel: 'M',
      margin: 4,
      width: 256,
      color: {
        dark: '#000000',
        light: '#ffffff'
      }
    })
    
    qrCodeImage.value = dataUrl
  } catch (err) {
    console.error('生成二维码失败:', err)
    error.value = '生成二维码失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

// 复制链接
const copyLink = async () => {
  if (!shareLink.value) {
    ElMessage.error('无可用的分享链接')
    return
  }
  
  try {
    await copyText(shareLink.value)
    ElMessage.success('链接已复制到剪贴板')
  } catch (err) {
    console.error('复制失败:', err)
    ElMessage.error('复制失败，请手动复制')
  }
}

// 下载二维码
const downloadQRCode = () => {
  if (!qrCodeImage.value) {
    ElMessage.error('二维码图片未生成')
    return
  }
  
  const link = document.createElement('a')
  link.href = qrCodeImage.value
  link.download = `dokodemo_${props.address}_${props.port}.png`
  link.click()
}

// 重新生成二维码
const regenerateQRCode = () => {
  generateQRCode()
}

// 监听属性变化
watch(
  () => [
    props.address,
    props.port,
    props.network,
    props.followRedirect,
    props.timeout,
    props.remark,
    props.connectionName
  ],
  () => {
    generateQRCode()
  }
)

// 组件挂载时生成二维码
onMounted(() => {
  generateQRCode()
})
</script>

<style scoped>
.dokodemo-qrcode-container {
  width: 100%;
  padding: 16px;
  border-radius: 8px;
  background-color: #fff;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.qrcode-loading,
.qrcode-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 250px;
  gap: 16px;
  color: #909399;
}

.qrcode-error {
  color: #F56C6C;
}

.qrcode-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.protocol-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  width: 100%;
  justify-content: center;
}

.protocol-badge {
  background-color: #5e35b1;
  color: white;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

.protocol-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.qrcode-display {
  padding: 16px;
  border: 1px solid #EBEEF5;
  border-radius: 4px;
  background-color: #fff;
}

.qrcode-display img {
  max-width: 100%;
  height: auto;
}

.qrcode-element {
  min-width: 240px;
  min-height: 240px;
}

.connection-details {
  width: 100%;
  border: 1px solid #EBEEF5;
  border-radius: 4px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.connection-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.item-label {
  font-weight: 500;
  color: #606266;
}

.item-value {
  font-family: monospace;
  color: #303133;
}

.share-link {
  width: 100%;
}

.share-link-input {
  font-family: monospace;
  font-size: 12px;
}

.actions {
  display: flex;
  justify-content: center;
  gap: 16px;
  margin-top: 8px;
}

@media (max-width: 480px) {
  .connection-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  
  .item-value {
    width: 100%;
  }
  
  .actions {
    flex-direction: column;
    gap: 8px;
    width: 100%;
  }
  
  .actions .el-button {
    width: 100%;
  }
}
</style> 
