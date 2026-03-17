<template>
  <div class="invite-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">邀请推广</h1>
      <p class="page-subtitle">邀请好友注册，获得佣金奖励</p>
    </div>

    <!-- 邀请码卡片 -->
    <el-card shadow="never" class="invite-card">
      <div class="invite-content">
        <div class="invite-code-section">
          <div class="invite-label">我的邀请码</div>
          <div class="invite-code">{{ code || '加载中...' }}</div>
          <el-button type="primary" @click="copyCode">
            <el-icon><CopyDocument /></el-icon>
            复制邀请码
          </el-button>
        </div>
        <div class="invite-link-section">
          <div class="invite-label">邀请链接</div>
          <el-input v-model="inviteLink" readonly>
            <template #append>
              <el-button @click="copyLink">复制</el-button>
            </template>
          </el-input>
        </div>
        <div class="invite-qr-section">
          <canvas ref="qrcodeCanvas"></canvas>
          <el-button size="small" @click="downloadQR">
            <el-icon><Download /></el-icon>
            下载二维码
          </el-button>
        </div>
      </div>
    </el-card>

    <!-- 统计卡片 -->
    <div class="stats-grid">
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ totalInvites }}</div>
        <div class="stat-label">邀请人数</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ convertedInvites }}</div>
        <div class="stat-label">已转化</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ conversionRate.toFixed(1) }}%</div>
        <div class="stat-label">转化率</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--highlight">
        <div class="stat-value">¥{{ formattedTotalEarnings }}</div>
        <div class="stat-label">累计收益</div>
      </el-card>
    </div>

    <!-- 收益信息 -->
    <el-card shadow="never" class="earnings-card">
      <template #header>
        <span>收益明细</span>
      </template>
      <div class="earnings-info">
        <div class="earnings-item">
          <span class="earnings-label">待确认佣金</span>
          <span class="earnings-value pending">¥{{ formattedPendingEarnings }}</span>
        </div>
        <div class="earnings-item">
          <span class="earnings-label">已确认佣金</span>
          <span class="earnings-value confirmed">¥{{ formattedTotalEarnings }}</span>
        </div>
      </div>
      <el-alert type="info" :closable="false" show-icon>
        <template #title>
          佣金将在订单完成后7天自动确认并转入您的余额
        </template>
      </el-alert>
    </el-card>

    <!-- 推荐列表 -->
    <el-card shadow="never" class="referrals-card">
      <template #header>
        <span>推荐记录</span>
      </template>

      <el-table :data="referrals" style="width: 100%">
        <el-table-column prop="invitee_id" label="用户ID" width="100" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getReferralStatusInfo(row.status).type" size="small">
              {{ getReferralStatusInfo(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" />
        <el-table-column prop="converted_at" label="转化时间">
          <template #default="{ row }">
            {{ row.converted_at || '-' }}
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="referrals.length === 0" description="暂无推荐记录" />

      <div v-if="referralPagination.total > 0" class="pagination-container">
        <el-pagination
          v-model:current-page="referralPagination.page"
          :total="referralPagination.total"
          :page-size="referralPagination.pageSize"
          layout="prev, pager, next"
          @current-change="handleReferralPageChange"
        />
      </div>
    </el-card>

    <!-- 佣金列表 -->
    <el-card shadow="never" class="commissions-card">
      <template #header>
        <span>佣金记录</span>
      </template>

      <el-table :data="commissions" style="width: 100%">
        <el-table-column prop="from_user_id" label="来源用户" width="100" />
        <el-table-column label="金额" width="120">
          <template #default="{ row }">
            <span class="commission-amount">¥{{ formatPrice(row.amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="比例" width="80">
          <template #default="{ row }">
            {{ (row.rate * 100).toFixed(0) }}%
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getCommissionStatusInfo(row.status).type" size="small">
              {{ getCommissionStatusInfo(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" />
        <el-table-column prop="confirm_at" label="确认时间">
          <template #default="{ row }">
            {{ row.confirm_at || '-' }}
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="commissions.length === 0" description="暂无佣金记录" />

      <div v-if="commissionPagination.total > 0" class="pagination-container">
        <el-pagination
          v-model:current-page="commissionPagination.page"
          :total="commissionPagination.total"
          :page-size="commissionPagination.pageSize"
          layout="prev, pager, next"
          @current-change="handleCommissionPageChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { CopyDocument, Download } from '@element-plus/icons-vue'
import { useInviteStore } from '@/stores/invite'
import QRCode from 'qrcode'
import { copyText } from '@/utils/clipboard'

const inviteStore = useInviteStore()

// 引用
const qrcodeCanvas = ref(null)

// 计算属性
const code = computed(() => inviteStore.code)
const inviteLink = computed(() => inviteStore.inviteLink)
const totalInvites = computed(() => inviteStore.totalInvites)
const convertedInvites = computed(() => inviteStore.convertedInvites)
const conversionRate = computed(() => inviteStore.conversionRate)
const formattedTotalEarnings = computed(() => inviteStore.formattedTotalEarnings)
const formattedPendingEarnings = computed(() => inviteStore.formattedPendingEarnings)
const referrals = computed(() => inviteStore.referrals)
const commissions = computed(() => inviteStore.commissions)
const referralPagination = computed(() => inviteStore.referralPagination)
const commissionPagination = computed(() => inviteStore.commissionPagination)

// 方法
const formatPrice = (price) => (price / 100).toFixed(2)
const getReferralStatusInfo = (status) => inviteStore.getReferralStatusInfo(status)
const getCommissionStatusInfo = (status) => inviteStore.getCommissionStatusInfo(status)

const generateQRCode = async () => {
  await nextTick()
  if (qrcodeCanvas.value && inviteLink.value) {
    try {
      await QRCode.toCanvas(qrcodeCanvas.value, inviteLink.value, {
        width: 150,
        margin: 2
      })
    } catch (error) {
      console.error('Failed to generate QR code:', error)
    }
  }
}

const copyCode = async () => {
  try {
    await copyText(code.value)
    ElMessage.success('邀请码已复制')
  } catch (error) {
    console.error('复制邀请码失败:', error)
    ElMessage.error('复制失败')
  }
}

const copyLink = async () => {
  try {
    await copyText(inviteLink.value)
    ElMessage.success('邀请链接已复制')
  } catch (error) {
    console.error('复制邀请链接失败:', error)
    ElMessage.error('复制失败')
  }
}

const downloadQR = () => {
  if (!qrcodeCanvas.value) return
  const link = document.createElement('a')
  link.download = 'invite-qrcode.png'
  link.href = qrcodeCanvas.value.toDataURL('image/png')
  link.click()
  ElMessage.success('二维码已下载')
}

const handleReferralPageChange = (page) => {
  inviteStore.setReferralPage(page)
  inviteStore.fetchReferrals()
}

const handleCommissionPageChange = (page) => {
  inviteStore.setCommissionPage(page)
  inviteStore.fetchCommissions()
}

onMounted(async () => {
  await inviteStore.fetchAll()
  await inviteStore.fetchReferrals()
  await inviteStore.fetchCommissions()
  generateQRCode()
})
</script>

<style scoped>
.invite-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.invite-card {
  margin-bottom: 20px;
  border-radius: 8px;
}

.invite-content {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 24px;
  align-items: center;
}

.invite-label {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}

.invite-code {
  font-size: 32px;
  font-weight: 700;
  color: #409eff;
  letter-spacing: 4px;
  margin-bottom: 12px;
}

.invite-qr-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  text-align: center;
  border-radius: 8px;
}

.stat-card--highlight {
  background: linear-gradient(135deg, #67c23a, #85ce61);
}

.stat-card--highlight .stat-value,
.stat-card--highlight .stat-label {
  color: #fff;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 8px;
}

.earnings-card,
.referrals-card,
.commissions-card {
  margin-bottom: 20px;
  border-radius: 8px;
}

.earnings-info {
  display: flex;
  gap: 40px;
  margin-bottom: 16px;
}

.earnings-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.earnings-label {
  font-size: 14px;
  color: #909399;
}

.earnings-value {
  font-size: 24px;
  font-weight: 600;
}

.earnings-value.pending {
  color: #e6a23c;
}

.earnings-value.confirmed {
  color: #67c23a;
}

.commission-amount {
  font-weight: 500;
  color: #67c23a;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  .invite-content {
    grid-template-columns: 1fr;
  }

  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
