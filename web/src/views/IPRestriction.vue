<template>
  <div class="ip-restriction-container">
    <h1>IP 限制管理</h1>
    
    <el-tabs type="border-card" v-model="activeTab">
      <!-- 统计仪表板 -->
      <el-tab-pane label="统计概览" name="stats">
        <div class="stats-grid">
          <el-card class="stat-card">
            <template #header>
              <div class="card-header">
                <span>在线设备</span>
                <el-icon><Monitor /></el-icon>
              </div>
            </template>
            <div class="stat-value">{{ stats.activeDevices }}</div>
            <div class="stat-label">当前活跃连接</div>
          </el-card>
          
          <el-card class="stat-card">
            <template #header>
              <div class="card-header">
                <span>在线用户</span>
                <el-icon><User /></el-icon>
              </div>
            </template>
            <div class="stat-value">{{ stats.activeUsers }}</div>
            <div class="stat-label">有活跃连接的用户</div>
          </el-card>
          
          <el-card class="stat-card">
            <template #header>
              <div class="card-header">
                <span>白名单</span>
                <el-icon><CircleCheck /></el-icon>
              </div>
            </template>
            <div class="stat-value">{{ stats.whitelistCount }}</div>
            <div class="stat-label">IP/CIDR 条目</div>
          </el-card>
          
          <el-card class="stat-card">
            <template #header>
              <div class="card-header">
                <span>黑名单</span>
                <el-icon><CircleClose /></el-icon>
              </div>
            </template>
            <div class="stat-value">{{ stats.blacklistCount }}</div>
            <div class="stat-label">IP/CIDR 条目</div>
          </el-card>
          
          <el-card class="stat-card">
            <template #header>
              <div class="card-header">
                <span>今日拦截</span>
                <el-icon><Warning /></el-icon>
              </div>
            </template>
            <div class="stat-value">{{ stats.blockedToday }}</div>
            <div class="stat-label">被拦截的请求</div>
          </el-card>
          
          <el-card class="stat-card">
            <template #header>
              <div class="card-header">
                <span>可疑活动</span>
                <el-icon><QuestionFilled /></el-icon>
              </div>
            </template>
            <div class="stat-value">{{ stats.suspiciousCount }}</div>
            <div class="stat-label">检测到的可疑模式</div>
          </el-card>
        </div>
        
        <!-- 国家访问统计 -->
        <el-card class="country-stats" style="margin-top: 20px;">
          <template #header>
            <div class="card-header">
              <span>按国家/地区统计</span>
              <el-button size="small" @click="refreshStats" :loading="statsLoading">
                <el-icon><Refresh /></el-icon>
                刷新
              </el-button>
            </div>
          </template>
          <el-table :data="stats.countryStats" border v-loading="statsLoading">
            <el-table-column prop="country" label="国家/地区" width="150"></el-table-column>
            <el-table-column prop="countryCode" label="代码" width="80"></el-table-column>
            <el-table-column prop="activeCount" label="活跃连接" width="100"></el-table-column>
            <el-table-column prop="totalCount" label="总访问次数" width="120"></el-table-column>
            <el-table-column prop="percentage" label="占比">
              <template #default="scope">
                <el-progress :percentage="scope.row.percentage" :stroke-width="10" />
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
      
      <!-- 全局设置 -->
      <el-tab-pane label="全局设置" name="settings">
        <el-form :model="settingsForm" label-width="180px" class="settings-form">
          <el-divider content-position="left">并发 IP 限制</el-divider>
          
          <el-form-item label="启用 IP 限制">
            <el-switch v-model="settingsForm.enabled"></el-switch>
            <div class="form-tips">启用后将限制用户同时在线的设备数量</div>
          </el-form-item>
          
          <el-form-item label="默认最大并发 IP">
            <el-input-number v-model="settingsForm.defaultMaxConcurrentIPs" :min="0" :max="100"></el-input-number>
            <div class="form-tips">0 表示无限制，新用户将使用此默认值</div>
          </el-form-item>
          
          <el-form-item label="IP 不活跃超时(分钟)">
            <el-input-number v-model="settingsForm.inactiveTimeout" :min="1" :max="1440"></el-input-number>
            <div class="form-tips">超过此时间未活动的 IP 将被自动清理</div>
          </el-form-item>
          
          <el-divider content-position="left">自动黑名单</el-divider>
          
          <el-form-item label="启用自动黑名单">
            <el-switch v-model="settingsForm.autoBlacklistEnabled"></el-switch>
            <div class="form-tips">自动将多次失败尝试的 IP 加入黑名单</div>
          </el-form-item>
          
          <el-form-item label="失败尝试阈值" v-if="settingsForm.autoBlacklistEnabled">
            <el-input-number v-model="settingsForm.failedAttemptThreshold" :min="3" :max="100"></el-input-number>
            <div class="form-tips">达到此次数后自动加入黑名单</div>
          </el-form-item>
          
          <el-form-item label="自动黑名单时长(小时)" v-if="settingsForm.autoBlacklistEnabled">
            <el-input-number v-model="settingsForm.autoBlacklistDuration" :min="1" :max="8760"></el-input-number>
            <div class="form-tips">自动黑名单的持续时间，0 表示永久</div>
          </el-form-item>
          
          <el-divider content-position="left">地理位置限制</el-divider>
          
          <el-form-item label="启用地理位置限制">
            <el-switch v-model="settingsForm.geoRestrictionEnabled"></el-switch>
          </el-form-item>
          
          <el-form-item label="限制模式" v-if="settingsForm.geoRestrictionEnabled">
            <el-radio-group v-model="settingsForm.geoRestrictionMode">
              <el-radio value="whitelist">仅允许指定国家</el-radio>
              <el-radio value="blacklist">禁止指定国家</el-radio>
            </el-radio-group>
          </el-form-item>
          
          <el-form-item label="国家列表" v-if="settingsForm.geoRestrictionEnabled">
            <el-select v-model="settingsForm.geoCountries" multiple placeholder="选择国家" style="width: 100%">
              <el-option v-for="country in countryOptions" :key="country.code" :label="country.name" :value="country.code"></el-option>
            </el-select>
          </el-form-item>
          
          <el-divider content-position="left">可疑活动检测（暂未接线）</el-divider>

          <el-alert
            title="后端暂未提供可疑活动检测配置接口，以下选项仅保留展示。"
            type="info"
            :closable="false"
            style="margin-bottom: 16px;"
          />
          
          <el-form-item label="启用可疑活动检测">
            <el-switch v-model="settingsForm.suspiciousDetectionEnabled" disabled></el-switch>
          </el-form-item>
          
          <el-form-item label="多国家检测时间窗口(分钟)" v-if="settingsForm.suspiciousDetectionEnabled">
            <el-input-number v-model="settingsForm.suspiciousTimeWindow" :min="1" :max="60" disabled></el-input-number>
            <div class="form-tips">在此时间内从多个国家访问将被标记为可疑</div>
          </el-form-item>
          
          <el-form-item label="多国家阈值" v-if="settingsForm.suspiciousDetectionEnabled">
            <el-input-number v-model="settingsForm.suspiciousCountryThreshold" :min="2" :max="10" disabled></el-input-number>
            <div class="form-tips">触发可疑标记的国家数量</div>
          </el-form-item>
          
          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" @click="saveSettings" :loading="settingsSaving">保存设置</el-button>
            <el-button @click="resetSettings">重置</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 白名单管理 -->
      <el-tab-pane label="白名单" name="whitelist">
        <div class="actions">
          <el-button type="primary" @click="showAddWhitelistDialog">
            <el-icon><Plus /></el-icon>
            添加白名单
          </el-button>
          <el-button type="success" @click="showImportWhitelistDialog">
            <el-icon><Upload /></el-icon>
            批量导入
          </el-button>
          <el-input 
            v-model="whitelistSearch" 
            placeholder="搜索 IP 或备注" 
            clearable 
            style="width: 200px; margin-left: 10px;"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>
        
        <el-table :data="paginatedWhitelist" border v-loading="whitelistLoading" style="width: 100%">
          <el-table-column prop="displayIp" label="IP/CIDR" width="180"></el-table-column>
          <el-table-column prop="type" label="类型" width="100">
            <template #default="scope">
              <el-tag :type="scope.row.type === 'global' ? 'primary' : 'success'">
                {{ scope.row.type === 'global' ? '全局' : '用户级' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="username" label="关联用户" width="120">
            <template #default="scope">
              {{ scope.row.username || '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="description" label="备注"></el-table-column>
          <el-table-column prop="createdAt" label="创建时间" width="180"></el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="scope">
              <el-button size="small" type="danger" @click="deleteWhitelist(scope.row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        
        <div class="pagination-container">
          <el-pagination
            v-model:current-page="whitelistPage"
            v-model:page-size="whitelistPageSize"
            :page-sizes="[10, 20, 50, 100]"
            layout="total, sizes, prev, pager, next"
            :total="whitelistDisplayTotal"
          />
        </div>
      </el-tab-pane>
      
      <!-- 黑名单管理 -->
      <el-tab-pane label="黑名单" name="blacklist">
        <div class="actions">
          <el-button type="primary" @click="showAddBlacklistDialog">
            <el-icon><Plus /></el-icon>
            添加黑名单
          </el-button>
          <el-input 
            v-model="blacklistSearch" 
            placeholder="搜索 IP 或原因" 
            clearable 
            style="width: 200px; margin-left: 10px;"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>
        
        <el-table :data="paginatedBlacklist" border v-loading="blacklistLoading" style="width: 100%">
          <el-table-column prop="displayIp" label="IP/CIDR" width="180"></el-table-column>
          <el-table-column prop="reason" label="原因"></el-table-column>
          <el-table-column prop="source" label="来源" width="100">
            <template #default="scope">
              <el-tag :type="scope.row.source === 'auto' ? 'warning' : 'info'">
                {{ scope.row.source === 'auto' ? '自动' : '手动' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="expiresAt" label="过期时间" width="180">
            <template #default="scope">
              <span :class="{ 'text-warning': isExpiringSoon(scope.row.expiresAtRaw) }">
                {{ scope.row.expiresAt || '永久' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="createdAt" label="创建时间" width="180"></el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="scope">
              <el-button size="small" type="danger" @click="deleteBlacklist(scope.row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        
        <div class="pagination-container">
          <el-pagination
            v-model:current-page="blacklistPage"
            v-model:page-size="blacklistPageSize"
            :page-sizes="[10, 20, 50, 100]"
            layout="total, sizes, prev, pager, next"
            :total="blacklistDisplayTotal"
          />
        </div>
      </el-tab-pane>
      
      <!-- 用户在线 IP -->
      <el-tab-pane label="用户在线 IP" name="online">
        <div class="actions">
          <el-input 
            v-model="onlineSearch" 
            placeholder="搜索用户名或 IP" 
            clearable 
            style="width: 200px;"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          <el-button @click="fetchOnlineIPs" :loading="onlineLoading" style="margin-left: 10px;">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
        
        <el-table :data="filteredOnlineIPs" border v-loading="onlineLoading" style="width: 100%">
          <el-table-column prop="username" label="用户名" width="150"></el-table-column>
          <el-table-column prop="ip" label="IP 地址" width="150"></el-table-column>
          <el-table-column prop="country" label="国家/地区" width="120">
            <template #default="scope">
              <span>{{ scope.row.countryFlag }} {{ scope.row.country }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="city" label="城市" width="120"></el-table-column>
          <el-table-column prop="lastActivity" label="最后活动" width="180"></el-table-column>
          <el-table-column prop="deviceInfo" label="设备信息"></el-table-column>
          <el-table-column label="操作" width="120">
            <template #default="scope">
              <el-button size="small" type="danger" @click="kickUserIP(scope.row)">
                <el-icon><Close /></el-icon>
                踢出
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
      
      <!-- IP 历史记录 -->
      <el-tab-pane label="IP 历史" name="history">
        <div class="actions">
          <el-select v-model="historyUserId" placeholder="选择用户" clearable style="width: 200px;">
            <el-option v-for="user in userOptions" :key="user.id" :label="user.username" :value="user.id"></el-option>
          </el-select>
          <el-date-picker
            v-model="historyDateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            style="margin-left: 10px;"
          />
          <el-button @click="fetchIPHistory" :loading="historyLoading" style="margin-left: 10px;">
            <el-icon><Search /></el-icon>
            查询
          </el-button>
        </div>
        
        <el-table :data="ipHistory" border v-loading="historyLoading" style="width: 100%">
          <el-table-column prop="username" label="用户名" width="150"></el-table-column>
          <el-table-column prop="ip" label="IP 地址" width="150"></el-table-column>
          <el-table-column prop="country" label="国家/地区" width="120"></el-table-column>
          <el-table-column prop="city" label="城市" width="120"></el-table-column>
          <el-table-column prop="action" label="操作类型" width="100">
            <template #default="scope">
              <el-tag :type="getActionTagType(scope.row.action)">
                {{ getActionLabel(scope.row.action) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="createdAt" label="时间" width="180"></el-table-column>
          <el-table-column prop="details" label="详情"></el-table-column>
        </el-table>
        
        <div class="pagination-container">
          <el-pagination
            v-model:current-page="historyPage"
            v-model:page-size="historyPageSize"
            :page-sizes="[20, 50, 100, 200]"
            layout="total, sizes, prev, pager, next"
            :total="historyTotal"
            @size-change="fetchIPHistory"
            @current-change="fetchIPHistory"
          />
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 添加/编辑白名单对话框 -->
    <el-dialog 
      title="添加白名单"
      v-model="whitelistDialogVisible"
      width="500px"
    >
      <el-form :model="whitelistForm" :rules="whitelistRules" ref="whitelistFormRef" label-width="100px">
        <el-form-item label="IP/CIDR" prop="ip">
          <el-input v-model="whitelistForm.ip" placeholder="例如: 192.168.1.1 或 10.0.0.0/8"></el-input>
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="whitelistForm.type">
            <el-radio value="global">全局白名单</el-radio>
            <el-radio value="user">用户级白名单</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="关联用户" prop="userId" v-if="whitelistForm.type === 'user'">
          <el-select v-model="whitelistForm.userId" placeholder="选择用户" style="width: 100%">
            <el-option v-for="user in userOptions" :key="user.id" :label="user.username" :value="user.id"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="备注" prop="description">
          <el-input v-model="whitelistForm.description" type="textarea" :rows="2" placeholder="可选备注"></el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="whitelistDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveWhitelist" :loading="whitelistSaving">保存</el-button>
      </template>
    </el-dialog>
    
    <!-- 批量导入白名单对话框 -->
    <el-dialog title="批量导入白名单" v-model="importWhitelistDialogVisible" width="500px">
      <el-form label-width="100px">
        <el-form-item label="IP 列表">
          <el-input 
            v-model="importWhitelistText" 
            type="textarea" 
            :rows="10" 
            placeholder="每行一个 IP 或 CIDR，例如:
192.168.1.1
10.0.0.0/8
172.16.0.0/12"
          ></el-input>
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="importWhitelistType">
            <el-radio value="global">全局白名单</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importWhitelistDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="importWhitelist" :loading="importingWhitelist">导入</el-button>
      </template>
    </el-dialog>
    
    <!-- 添加/编辑黑名单对话框 -->
    <el-dialog 
      title="添加黑名单"
      v-model="blacklistDialogVisible"
      width="500px"
    >
      <el-form :model="blacklistForm" :rules="blacklistRules" ref="blacklistFormRef" label-width="100px">
        <el-form-item label="IP/CIDR" prop="ip">
          <el-input v-model="blacklistForm.ip" placeholder="例如: 192.168.1.1 或 10.0.0.0/8"></el-input>
        </el-form-item>
        <el-form-item label="原因" prop="reason">
          <el-input v-model="blacklistForm.reason" placeholder="封禁原因"></el-input>
        </el-form-item>
        <el-form-item label="过期时间" prop="expiresAt">
          <el-date-picker
            v-model="blacklistForm.expiresAt"
            type="datetime"
            placeholder="留空表示永久"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="blacklistDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveBlacklist" :loading="blacklistSaving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Monitor, User, CircleCheck, CircleClose, Warning, QuestionFilled,
  Refresh, Plus, Upload, Search, Close
} from '@element-plus/icons-vue'
import api from '@/api/index'

// 当前标签页
const activeTab = ref('stats')

// ==================== 统计数据 ====================
const statsLoading = ref(false)
const stats = reactive({
  activeDevices: 0,
  activeUsers: 0,
  whitelistCount: 0,
  blacklistCount: 0,
  blockedToday: 0,
  suspiciousCount: 0,
  countryStats: []
})

const formatDateTime = (value) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

const parseIPOrCIDR = (value) => {
  const normalized = value?.trim() || ''
  if (normalized.includes('/')) {
    return { ip: '', cidr: normalized }
  }
  return { ip: normalized, cidr: '' }
}

const refreshStats = async () => {
  statsLoading.value = true
  try {
    const response = await api.get('/admin/ip-restrictions/stats')
    const data = response?.data || response
    stats.activeDevices = data?.total_active_ips || 0
    stats.activeUsers = data?.active_users || 0
    stats.whitelistCount = data?.total_whitelisted || 0
    stats.blacklistCount = data?.total_blacklisted || 0
    stats.blockedToday = data?.blocked_today || 0
    stats.suspiciousCount = data?.suspicious_count || 0
    stats.countryStats = data?.country_stats || []
  } catch (error) {
    console.error('Failed to fetch stats:', error)
    ElMessage.error(`获取统计数据失败: ${error.message || '未知错误'}`)
  } finally {
    statsLoading.value = false
  }
}

// ==================== 设置 ====================
const settingsSaving = ref(false)
const settingsForm = reactive({
  enabled: true,
  defaultMaxConcurrentIPs: 3,
  inactiveTimeout: 30,
  autoBlacklistEnabled: true,
  failedAttemptThreshold: 10,
  failedAttemptWindow: 5,
  autoBlacklistDuration: 24,
  geoRestrictionEnabled: false,
  geoRestrictionMode: 'blacklist',
  geoCountries: [],
  suspiciousDetectionEnabled: true,
  suspiciousTimeWindow: 5,
  suspiciousCountryThreshold: 3
})

const countryOptions = [
  { code: 'CN', name: '中国' },
  { code: 'US', name: '美国' },
  { code: 'JP', name: '日本' },
  { code: 'KR', name: '韩国' },
  { code: 'SG', name: '新加坡' },
  { code: 'HK', name: '香港' },
  { code: 'TW', name: '台湾' },
  { code: 'DE', name: '德国' },
  { code: 'GB', name: '英国' },
  { code: 'FR', name: '法国' },
  { code: 'RU', name: '俄罗斯' },
  { code: 'AU', name: '澳大利亚' },
  { code: 'CA', name: '加拿大' },
  { code: 'IN', name: '印度' },
  { code: 'BR', name: '巴西' }
]

const fetchSettings = async () => {
  try {
    const response = await api.get('/admin/settings/ip-restriction')
    const settings = response?.data || response || {}
    const allowedCountries = settings.allowed_countries || []
    const blockedCountries = settings.blocked_countries || []

    Object.assign(settingsForm, {
      enabled: settings.enabled ?? settingsForm.enabled,
      defaultMaxConcurrentIPs: settings.default_max_concurrent_ips ?? settingsForm.defaultMaxConcurrentIPs,
      inactiveTimeout: settings.inactive_timeout ?? settingsForm.inactiveTimeout,
      autoBlacklistEnabled: settings.auto_blacklist_enabled ?? settingsForm.autoBlacklistEnabled,
      failedAttemptThreshold: settings.max_failed_attempts ?? settingsForm.failedAttemptThreshold,
      failedAttemptWindow: settings.failed_attempt_window ?? settingsForm.failedAttemptWindow,
      autoBlacklistDuration: settings.auto_blacklist_duration
        ? Math.max(1, Math.round(settings.auto_blacklist_duration / 60))
        : settingsForm.autoBlacklistDuration,
      geoRestrictionEnabled: settings.geo_restriction_enabled ?? settingsForm.geoRestrictionEnabled,
      geoRestrictionMode: allowedCountries.length > 0 ? 'whitelist' : 'blacklist',
      geoCountries: allowedCountries.length > 0 ? allowedCountries : blockedCountries
    })
  } catch (error) {
    console.error('Failed to fetch settings:', error)
  }
}

const saveSettings = async () => {
  settingsSaving.value = true
  try {
    await api.put('/admin/settings/ip-restriction', {
      enabled: settingsForm.enabled,
      default_max_concurrent_ips: settingsForm.defaultMaxConcurrentIPs,
      inactive_timeout: settingsForm.inactiveTimeout,
      subscription_ip_limit_enabled: false,
      default_subscription_ip_limit: 0,
      geo_restriction_enabled: settingsForm.geoRestrictionEnabled,
      allowed_countries: settingsForm.geoRestrictionEnabled && settingsForm.geoRestrictionMode === 'whitelist'
        ? settingsForm.geoCountries
        : [],
      blocked_countries: settingsForm.geoRestrictionEnabled && settingsForm.geoRestrictionMode === 'blacklist'
        ? settingsForm.geoCountries
        : [],
      auto_blacklist_enabled: settingsForm.autoBlacklistEnabled,
      max_failed_attempts: settingsForm.failedAttemptThreshold,
      failed_attempt_window: settingsForm.failedAttemptWindow,
      auto_blacklist_duration: settingsForm.autoBlacklistDuration * 60
    })
    ElMessage.success('设置已保存')
  } catch (error) {
    console.error('Failed to save settings:', error)
    ElMessage.error('保存设置失败')
  } finally {
    settingsSaving.value = false
  }
}

const resetSettings = () => {
  fetchSettings()
}

// ==================== 白名单 ====================
const whitelistLoading = ref(false)
const whitelistSaving = ref(false)
const whitelist = ref([])
const whitelistSearch = ref('')
const whitelistPage = ref(1)
const whitelistPageSize = ref(10)
const whitelistTotal = ref(0)
const whitelistDialogVisible = ref(false)
const whitelistFormRef = ref(null)
const importWhitelistDialogVisible = ref(false)
const importWhitelistText = ref('')
const importWhitelistType = ref('global')
const importingWhitelist = ref(false)

const whitelistForm = reactive({
  id: null,
  ip: '',
  type: 'global',
  userId: null,
  description: ''
})

const whitelistRules = {
  ip: [
    { required: true, message: '请输入 IP 或 CIDR', trigger: 'blur' },
    { pattern: /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$|^([0-9a-fA-F:]+)(\/\d{1,3})?$/, message: '请输入有效的 IP 或 CIDR', trigger: 'blur' }
  ],
  userId: [
    { required: true, message: '请选择用户', trigger: 'change' }
  ]
}

const filteredWhitelist = computed(() => {
  if (!whitelistSearch.value) return whitelist.value
  const query = whitelistSearch.value.toLowerCase()
  return whitelist.value.filter(item => 
    item.displayIp.toLowerCase().includes(query) || 
    (item.description && item.description.toLowerCase().includes(query))
  )
})

const whitelistDisplayTotal = computed(() => (
  whitelistSearch.value ? filteredWhitelist.value.length : whitelistTotal.value
))

const paginatedWhitelist = computed(() => {
  const start = (whitelistPage.value - 1) * whitelistPageSize.value
  return filteredWhitelist.value.slice(start, start + whitelistPageSize.value)
})

const fetchWhitelist = async () => {
  whitelistLoading.value = true
  try {
    const response = await api.get('/admin/ip-whitelist')
    const items = Array.isArray(response?.data)
      ? response.data
      : (Array.isArray(response) ? response : (response?.list || []))
    whitelist.value = items.map(item => ({
      ...item,
      type: item.user_id ? 'user' : 'global',
      userId: item.user_id || null,
      username: getUsernameById(item.user_id),
      displayIp: item.cidr || item.ip,
      createdAt: formatDateTime(item.created_at)
    }))
    whitelistTotal.value = whitelist.value.length
  } catch (error) {
    console.error('Failed to fetch whitelist:', error)
    ElMessage.error('获取白名单失败')
  } finally {
    whitelistLoading.value = false
  }
}

const showAddWhitelistDialog = () => {
  Object.assign(whitelistForm, { id: null, ip: '', type: 'global', userId: null, description: '' })
  whitelistDialogVisible.value = true
}

const showImportWhitelistDialog = () => {
  importWhitelistText.value = ''
  importWhitelistType.value = 'global'
  importWhitelistDialogVisible.value = true
}

const saveWhitelist = async () => {
  if (!whitelistFormRef.value) return
  
  whitelistFormRef.value.validate(async (valid) => {
    if (!valid) return
    
    whitelistSaving.value = true
    try {
      const target = parseIPOrCIDR(whitelistForm.ip)
      await api.post('/admin/ip-whitelist', {
        ...target,
        user_id: whitelistForm.type === 'user' ? whitelistForm.userId : null,
        description: whitelistForm.description
      })
      ElMessage.success('添加成功')
      whitelistDialogVisible.value = false
      fetchWhitelist()
      refreshStats()
    } catch (error) {
      console.error('Failed to save whitelist:', error)
      ElMessage.error('保存失败')
    } finally {
      whitelistSaving.value = false
    }
  })
}

const deleteWhitelist = (row) => {
  ElMessageBox.confirm(`确定要删除白名单 ${row.displayIp} 吗?`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await api.delete(`/admin/ip-whitelist/${row.id}`)
      ElMessage.success('删除成功')
      fetchWhitelist()
      refreshStats()
    } catch (error) {
      console.error('Failed to delete whitelist:', error)
      ElMessage.error('删除失败')
    }
  }).catch(() => {})
}

const importWhitelist = async () => {
  const ips = importWhitelistText.value.split('\n').map(ip => ip.trim()).filter(ip => ip)
  if (ips.length === 0) {
    ElMessage.warning('请输入至少一个 IP')
    return
  }
  
  importingWhitelist.value = true
  try {
    await api.post('/admin/ip-whitelist/import', {
      ips,
      type: importWhitelistType.value
    })
    ElMessage.success(`成功导入 ${ips.length} 条记录`)
    importWhitelistDialogVisible.value = false
    fetchWhitelist()
    refreshStats()
  } catch (error) {
    console.error('Failed to import whitelist:', error)
    ElMessage.error('导入失败')
  } finally {
    importingWhitelist.value = false
  }
}

// ==================== 黑名单 ====================
const blacklistLoading = ref(false)
const blacklistSaving = ref(false)
const blacklist = ref([])
const blacklistSearch = ref('')
const blacklistPage = ref(1)
const blacklistPageSize = ref(10)
const blacklistTotal = ref(0)
const blacklistDialogVisible = ref(false)
const blacklistFormRef = ref(null)

const blacklistForm = reactive({
  id: null,
  ip: '',
  reason: '',
  expiresAt: null
})

const blacklistRules = {
  ip: [
    { required: true, message: '请输入 IP 或 CIDR', trigger: 'blur' },
    { pattern: /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$|^([0-9a-fA-F:]+)(\/\d{1,3})?$/, message: '请输入有效的 IP 或 CIDR', trigger: 'blur' }
  ],
  reason: [
    { required: true, message: '请输入封禁原因', trigger: 'blur' }
  ]
}

const filteredBlacklist = computed(() => {
  if (!blacklistSearch.value) return blacklist.value
  const query = blacklistSearch.value.toLowerCase()
  return blacklist.value.filter(item => 
    item.displayIp.toLowerCase().includes(query) || 
    (item.reason && item.reason.toLowerCase().includes(query))
  )
})

const blacklistDisplayTotal = computed(() => (
  blacklistSearch.value ? filteredBlacklist.value.length : blacklistTotal.value
))

const paginatedBlacklist = computed(() => {
  const start = (blacklistPage.value - 1) * blacklistPageSize.value
  return filteredBlacklist.value.slice(start, start + blacklistPageSize.value)
})

const isExpiringSoon = (expiresAt) => {
  if (!expiresAt) return false
  const expires = new Date(expiresAt)
  const now = new Date()
  const diff = expires - now
  return diff > 0 && diff < 24 * 60 * 60 * 1000 // 24小时内过期
}

const fetchBlacklist = async () => {
  blacklistLoading.value = true
  try {
    const response = await api.get('/admin/ip-blacklist')
    const items = Array.isArray(response?.data)
      ? response.data
      : (Array.isArray(response) ? response : (response?.list || []))
    blacklist.value = items.map(item => ({
      ...item,
      source: item.is_automatic ? 'auto' : 'manual',
      displayIp: item.cidr || item.ip,
      expiresAtRaw: item.expires_at,
      expiresAt: formatDateTime(item.expires_at),
      createdAt: formatDateTime(item.created_at)
    }))
    blacklistTotal.value = blacklist.value.length
  } catch (error) {
    console.error('Failed to fetch blacklist:', error)
    ElMessage.error('获取黑名单失败')
  } finally {
    blacklistLoading.value = false
  }
}

const showAddBlacklistDialog = () => {
  Object.assign(blacklistForm, { id: null, ip: '', reason: '', expiresAt: null })
  blacklistDialogVisible.value = true
}

const saveBlacklist = async () => {
  if (!blacklistFormRef.value) return
  
  blacklistFormRef.value.validate(async (valid) => {
    if (!valid) return
    
    blacklistSaving.value = true
    try {
      const target = parseIPOrCIDR(blacklistForm.ip)
      const expiresIn = blacklistForm.expiresAt
        ? Math.ceil((blacklistForm.expiresAt.getTime() - Date.now()) / 60000)
        : 0

      if (expiresIn < 0) {
        ElMessage.warning('过期时间必须晚于当前时间')
        return
      }

      await api.post('/admin/ip-blacklist', {
        ...target,
        reason: blacklistForm.reason,
        expires_in: expiresIn
      })
      ElMessage.success('添加成功')
      blacklistDialogVisible.value = false
      fetchBlacklist()
      refreshStats()
    } catch (error) {
      console.error('Failed to save blacklist:', error)
      ElMessage.error('保存失败')
    } finally {
      blacklistSaving.value = false
    }
  })
}

const deleteBlacklist = (row) => {
  ElMessageBox.confirm(`确定要删除黑名单 ${row.displayIp} 吗?`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await api.delete(`/admin/ip-blacklist/${row.id}`)
      ElMessage.success('删除成功')
      fetchBlacklist()
      refreshStats()
    } catch (error) {
      console.error('Failed to delete blacklist:', error)
      ElMessage.error('删除失败')
    }
  }).catch(() => {})
}

// ==================== 在线 IP ====================
const onlineLoading = ref(false)
const onlineIPs = ref([])
const onlineSearch = ref('')

const filteredOnlineIPs = computed(() => {
  if (!onlineSearch.value) return onlineIPs.value
  const query = onlineSearch.value.toLowerCase()
  return onlineIPs.value.filter(item => 
    item.username.toLowerCase().includes(query) || 
    item.ip.toLowerCase().includes(query)
  )
})

const fetchOnlineIPs = async () => {
  onlineLoading.value = true
  try {
    const response = await api.get('/admin/ip-restrictions/online')
    const items = Array.isArray(response?.data)
      ? response.data
      : (Array.isArray(response) ? response : [])
    onlineIPs.value = items.map(item => ({
      ...item,
      userId: item.user_id,
      username: getUsernameById(item.user_id),
      countryFlag: '',
      lastActivity: formatDateTime(item.last_active),
      deviceInfo: item.user_agent || item.device_type || '-'
    }))
  } catch (error) {
    console.error('Failed to fetch online IPs:', error)
    ElMessage.error('获取在线 IP 失败')
  } finally {
    onlineLoading.value = false
  }
}

const kickUserIP = (row) => {
  ElMessageBox.confirm(`确定要踢出用户 ${row.username} 的设备 ${row.ip} 吗?`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      await api.post(`/admin/users/${row.userId}/kick-ip`, { ip: row.ip })
      ElMessage.success('踢出成功')
      fetchOnlineIPs()
      refreshStats()
    } catch (error) {
      console.error('Failed to kick IP:', error)
      ElMessage.error('踢出失败')
    }
  }).catch(() => {})
}

// ==================== IP 历史 ====================
const historyLoading = ref(false)
const ipHistory = ref([])
const historyUserId = ref(null)
const historyDateRange = ref(null)
const historyPage = ref(1)
const historyPageSize = ref(20)
const historyTotal = ref(0)
const userOptions = ref([])

const fetchIPHistory = async () => {
  historyLoading.value = true
  try {
    const params = {
      limit: historyPageSize.value,
      offset: (historyPage.value - 1) * historyPageSize.value
    }
    if (historyUserId.value) {
      params.user_id = historyUserId.value
    }
    
    const response = await api.get('/admin/ip-restrictions/history', { params })
    const items = Array.isArray(response?.data)
      ? response.data
      : (Array.isArray(response) ? response : (response?.list || []))
    let normalized = items.map(item => ({
      ...item,
      username: getUsernameById(item.user_id),
      action: item.access_type,
      createdAt: formatDateTime(item.created_at)
    }))

    if (historyDateRange.value && historyDateRange.value.length === 2) {
      const [startDate, endDate] = historyDateRange.value
      const rangeEnd = new Date(endDate)
      rangeEnd.setHours(23, 59, 59, 999)
      normalized = normalized.filter(item => {
        const createdAt = new Date(item.created_at)
        return createdAt >= startDate && createdAt <= rangeEnd
      })
    }

    ipHistory.value = normalized
    historyTotal.value = items.length === historyPageSize.value
      ? (historyPage.value * historyPageSize.value + 1)
      : ((historyPage.value - 1) * historyPageSize.value + items.length)
  } catch (error) {
    console.error('Failed to fetch IP history:', error)
    ElMessage.error('获取 IP 历史失败')
  } finally {
    historyLoading.value = false
  }
}

const fetchUsers = async () => {
  try {
    const response = await api.get('/users')
    userOptions.value = Array.isArray(response)
      ? response
      : (response.users || response.data || response.list || [])
  } catch (error) {
    console.error('Failed to fetch users:', error)
  }
}

const getUsernameById = (userId) => {
  if (!userId) return '-'
  const user = userOptions.value.find(item => Number(item.id) === Number(userId))
  return user?.username || `#${userId}`
}

const getActionTagType = (action) => {
  const types = {
    'subscription': 'success',
    'proxy': 'primary',
    'api': 'info',
    'connect': 'success',
    'disconnect': 'info',
    'kick': 'warning',
    'block': 'danger',
    'suspicious': 'danger'
  }
  return types[action] || 'info'
}

const getActionLabel = (action) => {
  const labels = {
    'subscription': '订阅访问',
    'proxy': '代理访问',
    'api': '接口访问',
    'connect': '连接',
    'disconnect': '断开',
    'kick': '踢出',
    'block': '封禁',
    'suspicious': '可疑'
  }
  return labels[action] || action
}

// ==================== 生命周期 ====================
onMounted(async () => {
  await fetchUsers()
  refreshStats()
  fetchSettings()
  fetchWhitelist()
  fetchBlacklist()
  fetchOnlineIPs()
})
</script>

<style scoped>
.ip-restriction-container {
  padding: 20px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 20px;
}

.stat-card {
  text-align: center;
}

.stat-card .card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stat-value {
  font-size: 36px;
  font-weight: bold;
  color: var(--el-color-primary);
  margin: 10px 0;
}

.stat-label {
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.country-stats .card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.settings-form {
  max-width: 800px;
}

.form-tips {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.actions {
  margin-bottom: 20px;
  display: flex;
  align-items: center;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

.text-warning {
  color: var(--el-color-warning);
}
</style>
