<template>
  <div class="devices-container">
    <h1>我的设备</h1>
    
    <!-- 设备槽位信息 -->
    <el-card class="device-quota-card">
      <div class="quota-info">
        <div class="quota-item">
          <span class="quota-label">在线设备</span>
          <span class="quota-value">{{ devices.length }}</span>
        </div>
        <div class="quota-divider">
          /
        </div>
        <div class="quota-item">
          <span class="quota-label">最大设备数</span>
          <span
            class="quota-value"
            :class="{ 'text-warning': isNearLimit }"
          >
            {{ maxDevices === 0 ? '无限制' : maxDevices }}
          </span>
        </div>
      </div>
      <el-progress 
        v-if="maxDevices > 0"
        :percentage="deviceUsagePercent" 
        :status="deviceUsageStatus"
        :stroke-width="10"
        style="margin-top: 15px;"
      />
      <div
        v-if="isNearLimit"
        class="quota-tips"
      >
        <el-icon><Warning /></el-icon>
        您的设备数量即将达到上限，如需更多设备请联系管理员
      </div>
    </el-card>
    
    <!-- 设备列表 -->
    <el-card style="margin-top: 20px;">
      <template #header>
        <div class="card-header">
          <span>在线设备列表</span>
          <el-button
            size="small"
            :loading="loading"
            @click="fetchDevices"
          >
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>
      
      <el-empty
        v-if="devices.length === 0 && !loading"
        description="暂无在线设备"
      />
      
      <div
        v-else
        class="device-list"
      >
        <el-card 
          v-for="device in devices" 
          :key="device.ip" 
          class="device-card"
          :class="{ 'current-device': device.isCurrent }"
          shadow="hover"
        >
          <div class="device-info">
            <div class="device-icon">
              <el-icon :size="32">
                <Monitor />
              </el-icon>
            </div>
            <div class="device-details">
              <div class="device-ip">
                {{ device.ip }}
                <el-tag
                  v-if="device.isCurrent"
                  size="small"
                  type="success"
                >
                  当前设备
                </el-tag>
              </div>
              <div class="device-location">
                <el-icon><Location /></el-icon>
                {{ device.locationText }}
              </div>
              <div class="device-time">
                <el-icon><Clock /></el-icon>
                最后活动: {{ device.lastActivity }}
              </div>
              <div
                v-if="device.userAgent"
                class="device-agent"
              >
                <el-icon><Platform /></el-icon>
                {{ device.userAgent }}
              </div>
            </div>
            <div class="device-actions">
              <el-button 
                type="danger" 
                size="small" 
                :disabled="device.isCurrent"
                :loading="device.kicking"
                @click="kickDevice(device)"
              >
                <el-icon><Close /></el-icon>
                踢出
              </el-button>
            </div>
          </div>
        </el-card>
      </div>
    </el-card>
    
    <!-- IP 访问历史 -->
    <el-card style="margin-top: 20px;">
      <template #header>
        <div class="card-header">
          <span>IP 访问历史</span>
        </div>
      </template>
      
      <el-table
        v-loading="historyLoading"
        :data="ipHistory"
        border
      >
        <el-table-column
          prop="ip"
          label="IP 地址"
          width="150"
        />
        <el-table-column
          prop="country"
          label="国家/地区"
          width="120"
        >
          <template #default="scope">
            {{ scope.row.countryFlag }} {{ scope.row.country }}
          </template>
        </el-table-column>
        <el-table-column
          prop="city"
          label="城市"
          width="120"
        />
        <el-table-column
          prop="firstSeen"
          label="首次访问"
          width="180"
        />
        <el-table-column
          prop="lastSeen"
          label="最后访问"
          width="180"
        />
        <el-table-column
          prop="accessCount"
          label="访问次数"
          width="100"
        />
      </el-table>
      
      <div class="pagination-container">
        <el-pagination
          v-model:current-page="historyPage"
          v-model:page-size="historyPageSize"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          :total="historyTotal"
          @size-change="fetchIPHistory"
          @current-change="fetchIPHistory"
        />
      </div>
    </el-card>
    
    <!-- 订阅链接访问 IP -->
    <el-card
      v-if="false"
      style="margin-top: 20px;"
    >
      <template #header>
        <div class="card-header">
          <span>订阅链接访问 IP</span>
          <el-tooltip content="这些 IP 曾访问过您的订阅链接">
            <el-icon><QuestionFilled /></el-icon>
          </el-tooltip>
        </div>
      </template>
      
      <el-table
        :data="[]"
        border
      >
        <el-table-column
          prop="ip"
          label="IP 地址"
          width="150"
        />
        <el-table-column
          prop="country"
          label="国家/地区"
          width="120"
        />
        <el-table-column
          prop="lastAccess"
          label="最后访问"
          width="180"
        />
        <el-table-column
          prop="accessCount"
          label="访问次数"
          width="100"
        />
        <el-table-column
          prop="userAgent"
          label="客户端"
        />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Monitor, Location, Clock, Platform, Refresh, Close, Warning, QuestionFilled } from '@element-plus/icons-vue'
import api from '@/api/index'

// 设备数据
const loading = ref(false)
const devices = ref([])
const maxDevices = ref(0)

// IP 历史
const historyLoading = ref(false)
const ipHistory = ref([])
const historyPage = ref(1)
const historyPageSize = ref(10)
const historyTotal = ref(0)

// 计算属性
const deviceUsagePercent = computed(() => {
  if (maxDevices.value === 0) return 0
  return Math.min(100, Math.round((devices.value.length / maxDevices.value) * 100))
})

const deviceUsageStatus = computed(() => {
  if (deviceUsagePercent.value >= 100) return 'exception'
  if (deviceUsagePercent.value >= 80) return 'warning'
  return 'success'
})

const isNearLimit = computed(() => {
  return maxDevices.value > 0 && deviceUsagePercent.value >= 80
})

const formatDateTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN')
}

const toCountryFlag = (countryCode) => {
  const code = String(countryCode || '').trim().toUpperCase()
  if (!/^[A-Z]{2}$/.test(code)) return ''
  return String.fromCodePoint(...[...code].map((char) => char.charCodeAt(0) + 127397))
}

const buildLocationText = (country, city, countryFlag) => {
  const countryLabel = country || '位置未知'
  const cityLabel = city ? ` - ${city}` : ''
  return `${countryFlag ? `${countryFlag} ` : ''}${countryLabel}${cityLabel}`
}

const unwrapApiPayload = (response) => {
  const payload = response?.data ?? response ?? {}
  if (payload && typeof payload === 'object' && !Array.isArray(payload) && 'code' in payload && 'data' in payload) {
    return payload.data ?? {}
  }
  return payload
}

const normalizeDevice = (device, currentIP = '') => {
  const ip = device.ip || '-'
  const country = device.country || ''
  const city = device.city || ''
  const countryFlag = toCountryFlag(device.country_code || device.countryCode)
  const isCurrent = Boolean(device.is_current ?? device.isCurrent ?? (currentIP && ip === currentIP))

  return {
    ...device,
    ip,
    country,
    city,
    countryFlag,
    isCurrent,
    kicking: false,
    userAgent: device.user_agent || device.userAgent || '',
    lastActivity: formatDateTime(device.last_active || device.lastActivity),
    locationText: buildLocationText(country, city, countryFlag)
  }
}

const normalizeHistoryItem = (item) => {
  const country = item.country || '-'
  const city = item.city || '-'
  const countryFlag = toCountryFlag(item.country_code || item.countryCode)

  return {
    ...item,
    country,
    city,
    countryFlag,
    firstSeen: formatDateTime(item.first_seen || item.firstSeen || item.created_at || item.createdAt),
    lastSeen: formatDateTime(item.last_seen || item.lastSeen || item.created_at || item.createdAt),
    accessCount: Number(item.access_count ?? item.accessCount ?? 1),
    userAgent: item.user_agent || item.userAgent || ''
  }
}

// 获取设备列表
const fetchDevices = async () => {
  loading.value = true
  try {
    const response = await api.get('/user/devices')
    const payload = unwrapApiPayload(response)
    const currentIP = payload.current_ip ?? payload.currentIp ?? ''
    devices.value = (payload.devices || []).map((device) => normalizeDevice(device, currentIP))
    maxDevices.value = Number(payload.max_devices ?? payload.maxDevices ?? 0)
  } catch (error) {
    console.error('Failed to fetch devices:', error)
    ElMessage.error('获取设备列表失败')
  } finally {
    loading.value = false
  }
}

// 踢出设备
const kickDevice = (device) => {
  ElMessageBox.confirm(
    `确定要踢出设备 ${device.ip} 吗？该设备将需要重新连接。`,
    '踢出设备',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  ).then(async () => {
    device.kicking = true
    try {
      await api.post(`/user/devices/${encodeURIComponent(device.ip)}/kick`)
      ElMessage.success('设备已踢出')
      fetchDevices()
    } catch (error) {
      console.error('Failed to kick device:', error)
      ElMessage.error('踢出设备失败')
    } finally {
      device.kicking = false
    }
  }).catch(() => {})
}

// 获取 IP 历史
const fetchIPHistory = async () => {
  historyLoading.value = true
  try {
    const response = await api.get('/user/ip-history', {
      params: {
        limit: historyPageSize.value,
        offset: (historyPage.value - 1) * historyPageSize.value
      }
    })
    const payload = unwrapApiPayload(response)
    const list = Array.isArray(payload)
      ? payload
      : Array.isArray(payload.data)
        ? payload.data
        : Array.isArray(payload.list)
          ? payload.list
          : []

    ipHistory.value = list.map(normalizeHistoryItem)
    historyTotal.value = Number(payload.total ?? list.length)
  } catch (error) {
    console.error('Failed to fetch IP history:', error)
    ElMessage.error('获取 IP 历史失败')
  } finally {
    historyLoading.value = false
  }
}

// 自动刷新
let refreshInterval = null

onMounted(() => {
  fetchDevices()
  fetchIPHistory()
  
  // 每30秒自动刷新设备列表
  refreshInterval = setInterval(fetchDevices, 30000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<style scoped>
.devices-container {
  padding: 20px;
}

.device-quota-card {
  background: linear-gradient(135deg, var(--el-color-primary-light-9), var(--el-color-primary-light-7));
}

.quota-info {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
}

.quota-item {
  text-align: center;
}

.quota-label {
  display: block;
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.quota-value {
  display: block;
  font-size: 36px;
  font-weight: bold;
  color: var(--el-color-primary);
}

.quota-divider {
  font-size: 36px;
  color: var(--el-text-color-secondary);
}

.quota-tips {
  margin-top: 15px;
  padding: 10px;
  background: var(--el-color-warning-light-9);
  border-radius: 4px;
  color: var(--el-color-warning);
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.device-list {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.device-card {
  border-left: 4px solid var(--el-color-primary);
}

.device-card.current-device {
  border-left-color: var(--el-color-success);
  background: var(--el-color-success-light-9);
}

.device-info {
  display: flex;
  align-items: center;
  gap: 20px;
}

.device-icon {
  color: var(--el-color-primary);
}

.device-details {
  flex: 1;
}

.device-ip {
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.device-location,
.device-time,
.device-agent {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 4px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

.text-warning {
  color: var(--el-color-warning) !important;
}
</style>
