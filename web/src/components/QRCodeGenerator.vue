<template>
  <div class="qrcode-container">
    <div v-if="loading" class="qrcode-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>生成中...</span>
    </div>
    <div v-else-if="error" class="qrcode-error">
      <el-icon><WarningFilled /></el-icon>
      <span>{{ error }}</span>
    </div>
    <div v-else class="qrcode-content">
      <div class="qrcode-image">
        <img :src="qrCodeImage" alt="二维码" />
      </div>
      <div class="qrcode-info">
        <div class="link-display">
          <el-input 
            v-model="shareLink" 
            readonly 
            class="share-link-input"
            :disabled="!shareLink"
          >
            <template #append>
              <el-button type="primary" @click="copyLink" :disabled="!shareLink">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </template>
          </el-input>
        </div>
        <div class="actions">
          <el-button type="primary" @click="downloadQRCode" :disabled="!qrCodeImage">下载二维码</el-button>
          <el-button @click="refreshQRCode" :disabled="loading">刷新</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading, WarningFilled, CopyDocument } from '@element-plus/icons-vue'
import QRCode from 'qrcode'
import api from '@/api/index'
import { copyText } from '@/utils/clipboard'

const props = defineProps({
  proxyId: {
    type: [String, Number],
    required: true
  },
  size: {
    type: Number,
    default: 256
  }
})

const shareLink = ref('')
const qrCodeImage = ref('')
const loading = ref(false)
const error = ref('')

// 获取分享链接
const fetchShareLink = async () => {
  loading.value = true
  error.value = ''
  
  try {
    const response = await api.get(`/proxies/${props.proxyId}/link`)
    const data = response.data || response
    shareLink.value = data.link
    
    // 生成二维码
    generateQRCode(data.link)
  } catch (err) {
    console.error('获取分享链接失败:', err)
    error.value = '获取分享链接失败'
    loading.value = false
  }
}

// 生成二维码
const generateQRCode = async (link) => {
  try {
    const dataUrl = await QRCode.toDataURL(link, {
      width: props.size,
      margin: 2,
      color: {
        dark: '#000000',
        light: '#ffffff'
      }
    })
    
    qrCodeImage.value = dataUrl
  } catch (err) {
    console.error('生成二维码失败:', err)
    error.value = '生成二维码失败'
  } finally {
    loading.value = false
  }
}

// 复制链接到剪贴板
const copyLink = async () => {
  if (!shareLink.value) return
  
  try {
    await copyText(shareLink.value)
    ElMessage.success('已复制到剪贴板')
  } catch (err) {
    console.error('复制链接失败:', err)
    ElMessage.error('复制失败，请手动复制链接')
  }
}

// 下载二维码图片
const downloadQRCode = () => {
  if (!qrCodeImage.value) return
  
  const link = document.createElement('a')
  link.href = qrCodeImage.value
  link.download = `proxy_${props.proxyId}_qrcode.png`
  link.click()
}

// 刷新二维码
const refreshQRCode = () => {
  fetchShareLink()
}

// 组件加载时获取分享链接
onMounted(() => {
  fetchShareLink()
})
</script>

<style scoped>
.qrcode-container {
  border-radius: 8px;
  overflow: hidden;
  width: 100%;
  padding: 16px;
  background-color: #fff;
}

.qrcode-loading,
.qrcode-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 300px;
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

.qrcode-image {
  padding: 16px;
  border: 1px solid #EBEEF5;
  border-radius: 4px;
  background-color: #fff;
}

.qrcode-info {
  width: 100%;
}

.link-display {
  margin-bottom: 16px;
}

.share-link-input {
  font-family: monospace;
}

.actions {
  display: flex;
  justify-content: center;
  gap: 16px;
}

@media (max-width: 768px) {
  .qrcode-image img {
    max-width: 100%;
    height: auto;
  }
}
</style> 
