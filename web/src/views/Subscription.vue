<template>
  <div class="subscription-container">
    <div class="subscription-header">
      <h2>订阅管理</h2>
      <p>获取和管理您的订阅链接</p>
    </div>

    <div class="subscription-content">
      <!-- 订阅链接卡片 -->
      <el-card class="subscription-card">
        <template #header>
          <div class="card-header">
            <span>订阅链接</span>
            <el-button 
              type="danger" 
              size="small" 
              :loading="loading"
              @click="showRegenerateDialog"
            >
              重新生成
            </el-button>
          </div>
        </template>

        <div
          v-if="loading"
          class="loading-container"
        >
          <el-skeleton
            :rows="3"
            animated
          />
        </div>

        <div
          v-else-if="hasSubscription"
          class="link-section"
        >
          <!-- 主链接 -->
          <div class="link-item">
            <label>订阅链接</label>
            <div class="link-input-group">
              <el-input 
                v-model="link" 
                readonly 
                class="link-input"
              >
                <template #append>
                  <el-button @click="copyLink(link)">
                    <el-icon><DocumentCopy /></el-icon>
                  </el-button>
                </template>
              </el-input>
              <el-button
                class="qr-button"
                aria-label="显示二维码"
                @click="showQRCode(link)"
              >
                <el-icon><Grid /></el-icon>
              </el-button>
            </div>
          </div>

          <!-- 短链接 -->
          <div
            v-if="shortLink"
            class="link-item"
          >
            <label>短链接</label>
            <div class="link-input-group">
              <el-input 
                v-model="shortLink" 
                readonly 
                class="link-input"
              >
                <template #append>
                  <el-button @click="copyLink(shortLink)">
                    <el-icon><DocumentCopy /></el-icon>
                  </el-button>
                </template>
              </el-input>
              <el-button
                class="qr-button"
                @click="showQRCode(shortLink)"
              >
                <el-icon><Grid /></el-icon>
              </el-button>
            </div>
          </div>
        </div>

        <div
          v-else
          class="empty-state"
        >
          <el-empty description="暂无订阅链接">
            <el-button
              type="primary"
              @click="fetchSubscription"
            >
              获取订阅链接
            </el-button>
          </el-empty>
        </div>
      </el-card>

      <!-- 格式选择卡片 -->
      <el-card
        v-if="hasSubscription"
        class="formats-card"
      >
        <template #header>
          <span>客户端格式</span>
        </template>

        <div class="formats-grid">
          <div 
            v-for="format in formats" 
            :key="format.name" 
            class="format-item"
            @click="copyLink(format.link)"
          >
            <div class="format-icon">
              <el-icon :size="24">
                <Link />
              </el-icon>
            </div>
            <div class="format-info">
              <div class="format-name">
                {{ format.display_name }}
              </div>
              <div class="format-desc">
                点击复制链接
              </div>
            </div>
          </div>
        </div>
      </el-card>

      <!-- 访问统计卡片 -->
      <el-card
        v-if="hasSubscription"
        class="stats-card"
      >
        <template #header>
          <span>访问统计</span>
        </template>

        <div class="stats-grid">
          <div class="stat-item">
            <div class="stat-value">
              {{ accessCount }}
            </div>
            <div class="stat-label">
              总访问次数
            </div>
          </div>
          <div class="stat-item">
            <div class="stat-value">
              {{ formatDate(lastAccessAt) }}
            </div>
            <div class="stat-label">
              最后访问时间
            </div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- QR码对话框 -->
    <el-dialog 
      v-model="qrDialogVisible"
      title="扫描二维码"
      width="min(350px, 90vw)"
      center
    >
      <div class="qr-container">
        <canvas ref="qrCanvas" />
      </div>
      <template #footer>
        <el-button @click="qrDialogVisible = false">
          关闭
        </el-button>
      </template>
    </el-dialog>

    <!-- 重新生成确认对话框 -->
    <el-dialog 
      v-model="regenerateDialogVisible" 
      title="确认重新生成" 
      width="400px"
    >
      <div class="regenerate-warning">
        <el-icon
          :size="48"
          color="#E6A23C"
        >
          <Warning />
        </el-icon>
        <p>重新生成订阅链接后，旧链接将立即失效。</p>
        <p>所有使用旧链接的客户端都需要重新配置。</p>
        <p><strong>确定要继续吗？</strong></p>
      </div>
      <template #footer>
        <el-button @click="regenerateDialogVisible = false">
          取消
        </el-button>
        <el-button
          type="danger"
          :loading="loading"
          @click="confirmRegenerate"
        >
          确认重新生成
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { DocumentCopy, Grid, Link, Warning } from '@element-plus/icons-vue'
import { useSubscriptionStore } from '@/stores/subscription'
import QRCode from 'qrcode'
import { copyText } from '@/utils/clipboard'

const subscriptionStore = useSubscriptionStore()

// 响应式状态
const qrDialogVisible = ref(false)
const regenerateDialogVisible = ref(false)
const qrCanvas = ref(null)
const currentQRLink = ref('')

// 计算属性
const loading = computed(() => subscriptionStore.loading)
const link = computed(() => subscriptionStore.link)
const shortLink = computed(() => subscriptionStore.shortLink)
const formats = computed(() => subscriptionStore.formats)
const accessCount = computed(() => subscriptionStore.accessCount)
const lastAccessAt = computed(() => subscriptionStore.lastAccessAt)
const hasSubscription = computed(() => subscriptionStore.hasSubscription)

// 方法
const fetchSubscription = async () => {
  try {
    await subscriptionStore.fetchLink()
  } catch (error) {
    ElMessage.error(error || '获取订阅链接失败')
  }
}

const copyLink = async (linkToCopy) => {
  try {
    await copyText(linkToCopy)
    ElMessage.success('链接已复制到剪贴板')
  } catch (error) {
    console.error('复制订阅链接失败:', error)
    ElMessage.error('复制失败，请手动复制链接')
  }
}

const showQRCode = async (linkForQR) => {
  currentQRLink.value = linkForQR
  qrDialogVisible.value = true
  
  await nextTick()
  
  if (qrCanvas.value) {
    try {
      await QRCode.toCanvas(qrCanvas.value, linkForQR, {
        width: 280,
        margin: 2,
        color: {
          dark: '#000000',
          light: '#ffffff'
        }
      })
    } catch (error) {
      console.error('生成二维码失败:', error)
      ElMessage.error('生成二维码失败')
    }
  }
}

const showRegenerateDialog = () => {
  regenerateDialogVisible.value = true
}

const confirmRegenerate = async () => {
  try {
    await subscriptionStore.regenerate()
    regenerateDialogVisible.value = false
    ElMessage.success('订阅链接已重新生成')
  } catch (error) {
    ElMessage.error(error || '重新生成订阅链接失败')
  }
}

const formatDate = (dateStr) => {
  if (!dateStr) return '--'
  const date = new Date(dateStr)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

// 生命周期
onMounted(() => {
  fetchSubscription()
})
</script>


<style scoped>
.subscription-container {
  padding: 20px;
}

.subscription-header {
  margin-bottom: 20px;
}

.subscription-header h2 {
  font-size: 24px;
  margin: 0 0 8px;
  color: #303133;
}

.subscription-header p {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.subscription-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 900px;
}

.subscription-card,
.formats-card,
.stats-card {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.loading-container {
  padding: 20px;
}

.link-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.link-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.link-item label {
  font-size: 14px;
  color: #606266;
  font-weight: 500;
}

.link-input-group {
  display: flex;
  gap: 8px;
}

.link-input {
  flex: 1;
}

.qr-button {
  flex-shrink: 0;
}

.empty-state {
  padding: 40px 0;
}

.formats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.format-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
}

.format-item:hover {
  border-color: #409eff;
  background-color: #f5f7fa;
}

.format-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #ecf5ff;
  border-radius: 8px;
  color: #409eff;
}

.format-info {
  flex: 1;
}

.format-name {
  font-size: 14px;
  font-weight: 500;
  color: #303133;
}

.format-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 20px;
}

.stat-item {
  text-align: center;
  padding: 16px;
  background-color: #f5f7fa;
  border-radius: 8px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 8px;
}

.qr-container {
  display: flex;
  justify-content: center;
  padding: 20px;
}

.regenerate-warning {
  text-align: center;
  padding: 20px;
}

.regenerate-warning p {
  margin: 12px 0;
  color: #606266;
}

.regenerate-warning .el-icon {
  margin-bottom: 16px;
}
</style>
