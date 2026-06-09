<template>
  <div class="settings-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        个人设置
      </h1>
      <p class="page-subtitle">
        管理您的账户信息和安全设置
      </p>
    </div>

    <el-alert
      v-if="showForcedPasswordAlert"
      title="当前账户需要先完成密码修改，修改完成后才能继续正常使用其他页面。"
      type="warning"
      :closable="false"
      class="forced-password-alert"
      show-icon
    />

    <el-tabs
      v-model="activeTab"
      class="settings-tabs"
    >
      <!-- 个人资料 -->
      <el-tab-pane
        label="个人资料"
        name="profile"
      >
        <el-card
          shadow="never"
          class="settings-card"
        >
          <div
            v-if="userStore.isAdmin"
            class="admin-panel-entry"
          >
            <div class="admin-panel-copy">
              <h3 class="section-title">
                管理权限
              </h3>
              <p class="section-desc">
                当前账号具备管理员权限，可进入管理后台维护系统配置和用户数据。
              </p>
            </div>
            <el-button
              type="primary"
              @click="goToAdminPanel"
            >
              <el-icon><Monitor /></el-icon>
              管理后台
            </el-button>
          </div>

          <el-form
            ref="profileFormRef"
            :model="profileForm"
            :rules="profileRules"
            :label-width="formLabelWidth"
            :label-position="formLabelPosition"
          >
            <el-form-item label="用户名">
              <el-input
                v-model="userStore.username"
                disabled
              />
              <div class="form-tip">
                用户名不可修改
              </div>
            </el-form-item>

            <el-form-item label="邮箱">
              <el-input
                v-model="userStore.email"
                disabled
              >
                <template #append>
                  <el-tag
                    v-if="emailVerified"
                    type="success"
                    size="small"
                  >
                    已验证
                  </el-tag>
                  <el-button
                    v-else
                    link
                    type="primary"
                    @click="resendVerification"
                  >
                    发送验证
                  </el-button>
                </template>
              </el-input>
            </el-form-item>

            <el-form-item
              label="显示名称"
              prop="displayName"
            >
              <el-input
                v-model="profileForm.displayName"
                placeholder="设置显示名称"
              />
            </el-form-item>

            <el-form-item label="头像">
              <div class="avatar-upload">
                <el-avatar
                  :size="80"
                  :src="profileForm.avatarUrl"
                >
                  <el-icon><User /></el-icon>
                </el-avatar>
                <input
                  ref="avatarInputRef"
                  type="file"
                  accept="image/png,image/jpeg,image/gif,image/webp"
                  class="avatar-input-hidden"
                  @change="handleAvatarSelected"
                >
                <el-button
                  size="small"
                  :loading="uploadingAvatar"
                  @click="avatarInputRef?.click()"
                >
                  上传头像
                </el-button>
                <el-button
                  v-if="profileForm.avatarUrl"
                  size="small"
                  text
                  :disabled="uploadingAvatar"
                  @click="clearAvatar"
                >
                  清除头像
                </el-button>
                <div class="form-tip">
                  支持 JPG、PNG、GIF、WEBP，大小不超过 2MB
                </div>
              </div>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="saving"
                @click="saveProfile"
              >
                保存修改
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- 安全设置 -->
      <el-tab-pane
        label="安全设置"
        name="security"
      >
        <el-card
          shadow="never"
          class="settings-card"
        >
          <h3 class="section-title">
            修改密码
          </h3>
          <el-form
            ref="passwordFormRef"
            :model="passwordForm"
            :rules="passwordRules"
            :label-width="formLabelWidth"
            :label-position="formLabelPosition"
          >
            <el-form-item
              label="当前密码"
              prop="currentPassword"
            >
              <el-input
                v-model="passwordForm.currentPassword"
                type="password"
                show-password
                placeholder="输入当前密码"
              />
            </el-form-item>

            <el-form-item
              label="新密码"
              prop="newPassword"
            >
              <el-input
                v-model="passwordForm.newPassword"
                type="password"
                show-password
                placeholder="输入新密码"
              />
            </el-form-item>

            <el-form-item
              label="确认密码"
              prop="confirmPassword"
            >
              <el-input
                v-model="passwordForm.confirmPassword"
                type="password"
                show-password
                placeholder="再次输入新密码"
              />
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="changingPassword"
                @click="changePassword"
              >
                修改密码
              </el-button>
            </el-form-item>
          </el-form>

          <el-divider />

          <h3 class="section-title">
            两步验证
          </h3>
          <div class="two-factor-section">
            <div class="two-factor-info">
              <p>两步验证可以为您的账户提供额外的安全保护。启用后，登录时需要输入验证器应用生成的验证码。</p>
              <el-tag
                :type="twoFactorEnabled ? 'success' : 'info'"
                size="small"
              >
                {{ twoFactorEnabled ? '已启用' : '未启用' }}
              </el-tag>
            </div>
            <el-button 
              :type="twoFactorEnabled ? 'danger' : 'primary'"
              @click="toggleTwoFactor"
            >
              {{ twoFactorEnabled ? '禁用两步验证' : '启用两步验证' }}
            </el-button>
          </div>

          <el-divider />

          <h3 class="section-title">
            登录会话
          </h3>
          <div class="sessions-section">
            <div class="session-summary">
              <span>当前在线设备 {{ sessionDevices.length }} 台</span>
              <span v-if="sessionMaxDevices > 0">上限 {{ sessionMaxDevices }} 台</span>
            </div>
            <div
              v-loading="sessionsLoading"
              class="session-list"
            >
              <div
                v-for="device in sessionDevices"
                :key="device.ip"
                class="session-item"
              >
                <div class="session-main">
                  <div class="session-ip">
                    {{ device.ip }}
                    <el-tag
                      v-if="device.isCurrent"
                      size="small"
                      type="success"
                    >
                      当前设备
                    </el-tag>
                  </div>
                  <div class="session-meta">
                    {{ device.locationText }} · {{ device.lastActivity }}
                  </div>
                  <div
                    v-if="device.userAgent"
                    class="session-agent"
                  >
                    {{ device.userAgent }}
                  </div>
                </div>
                <el-button
                  size="small"
                  :disabled="device.isCurrent || device.kicking"
                  :loading="device.kicking"
                  @click="kickSession(device)"
                >
                  踢出
                </el-button>
              </div>
              <div
                v-if="!sessionsLoading && sessionDevices.length === 0"
                class="session-empty"
              >
                当前没有活跃会话
              </div>
            </div>
            <el-button @click="router.push('/user/devices')">
              查看完整设备列表
            </el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- 通知设置 -->
      <el-tab-pane
        label="通知设置"
        name="notifications"
      >
        <el-card
          shadow="never"
          class="settings-card"
        >
          <el-form
            :label-width="notificationLabelWidth"
            :label-position="formLabelPosition"
          >
            <el-form-item label="邮件通知">
              <el-switch v-model="notificationSettings.email" />
              <div class="form-tip">
                接收账户相关的邮件通知
              </div>
            </el-form-item>

            <el-form-item label="Telegram 通知">
              <el-switch
                v-model="notificationSettings.telegram"
                :disabled="!telegramBound"
              />
              <div class="form-tip">
                <template v-if="telegramBound">
                  已绑定 Chat ID：{{ telegramChatId }}
                </template>
                <template v-else>
                  绑定后可接收 Telegram 通知
                </template>
              </div>
            </el-form-item>

            <el-form-item label="Telegram 绑定">
              <div class="telegram-bind-row">
                <el-input
                  v-model="telegramForm.chatId"
                  :disabled="telegramBound || bindingTelegram"
                  placeholder="输入 Telegram Chat ID"
                />
                <el-button
                  type="primary"
                  :loading="bindingTelegram"
                  :disabled="telegramBound || !telegramForm.chatId.trim()"
                  @click="handleBindTelegram"
                >
                  绑定
                </el-button>
                <el-button
                  v-if="telegramBound"
                  :loading="unbindingTelegram"
                  @click="handleUnbindTelegram"
                >
                  解绑
                </el-button>
              </div>
              <div class="form-tip">
                先在 Telegram 中与机器人开始对话，再填写 Chat ID 完成绑定。
              </div>
            </el-form-item>

            <el-form-item label="流量预警">
              <el-switch v-model="notificationSettings.trafficWarning" />
              <div class="form-tip">
                当流量使用超过 80% 时发送提醒
              </div>
            </el-form-item>

            <el-form-item label="到期提醒">
              <el-switch v-model="notificationSettings.expiryReminder" />
              <div class="form-tip">
                在账户到期前 7 天发送提醒
              </div>
            </el-form-item>

            <el-form-item label="公告通知">
              <el-switch v-model="notificationSettings.announcements" />
              <div class="form-tip">
                接收系统公告和维护通知
              </div>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="savingNotifications"
                @click="saveNotifications"
              >
                保存设置
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- 偏好设置 -->
      <el-tab-pane
        label="偏好设置"
        name="preferences"
      >
        <el-card
          shadow="never"
          class="settings-card"
        >
          <el-form
            :label-width="formLabelWidth"
            :label-position="formLabelPosition"
          >
            <el-form-item label="界面主题">
              <el-radio-group v-model="preferences.theme">
                <el-radio value="auto">
                  跟随系统
                </el-radio>
                <el-radio value="light">
                  浅色
                </el-radio>
                <el-radio value="dark">
                  深色
                </el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="语言">
              <el-select
                v-model="preferences.language"
                class="language-select"
              >
                <el-option
                  label="简体中文"
                  value="zh-CN"
                />
                <el-option
                  label="English"
                  value="en-US"
                />
              </el-select>
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="savingPreferences"
                @click="savePreferences"
              >
                保存设置
              </el-button>
            </el-form-item>
          </el-form>

          <el-divider />

          <div class="account-actions">
            <h3 class="section-title">
              账户操作
            </h3>
            <p class="section-desc">
              在当前设备安全退出用户门户。
            </p>
            <el-button
              type="danger"
              plain
              :loading="loggingOut"
              @click="logout"
            >
              退出登录
            </el-button>
          </div>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 两步验证设置对话框 -->
    <el-dialog
      v-model="showTwoFactorDialog"
      :title="twoFactorEnabled ? '禁用两步验证' : '启用两步验证'"
      :width="dialogWidth"
      :close-on-click-modal="false"
    >
      <!-- 启用流程 -->
      <template v-if="!twoFactorEnabled">
        <div class="two-factor-setup">
          <div class="setup-step">
            <h4>1. 扫描二维码</h4>
            <p>使用验证器应用（如 Google Authenticator、Microsoft Authenticator）扫描下方二维码</p>
            <div class="qrcode-wrapper">
              <canvas ref="twoFactorQRCode" />
            </div>
            <p class="secret-key">
              或手动输入密钥：<code>{{ twoFactorSecret }}</code>
            </p>
          </div>

          <div class="setup-step">
            <h4>2. 输入验证码</h4>
            <p>输入验证器应用显示的 6 位验证码</p>
            <el-input
              v-model="twoFactorCode"
              placeholder="000000"
              maxlength="6"
              class="verify-input"
            />
          </div>

          <div class="setup-step">
            <h4>3. 保存备份码</h4>
            <p>请妥善保存以下备份码，当您无法使用验证器时可以使用备份码登录</p>
            <div class="backup-codes">
              <code
                v-for="code in backupCodes"
                :key="code"
              >{{ code }}</code>
            </div>
            <el-button
              size="small"
              @click="copyBackupCodes"
            >
              复制备份码
            </el-button>
          </div>
        </div>
      </template>

      <!-- 禁用流程 -->
      <template v-else>
        <el-alert
          type="warning"
          :closable="false"
          show-icon
        >
          禁用两步验证会降低账户安全性，请确认您要继续。
        </el-alert>
        <el-form style="margin-top: 20px">
          <el-form-item label="当前密码">
            <el-input
              v-model="disablePassword"
              type="password"
              show-password
              placeholder="输入当前密码确认"
            />
          </el-form-item>
        </el-form>
      </template>

      <template #footer>
        <el-button @click="showTwoFactorDialog = false">
          取消
        </el-button>
        <el-button 
          :type="twoFactorEnabled ? 'danger' : 'primary'" 
          :loading="processingTwoFactor"
          @click="confirmTwoFactor"
        >
          {{ twoFactorEnabled ? '确认禁用' : '确认启用' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api/base'
import { Monitor, User } from '@element-plus/icons-vue'
import QRCode from 'qrcode'
import { auth as portalAuthApi } from '@/api/modules/portal'
import { useUserPortalStore } from '@/stores/userPortal'
import { useTheme } from '@/composables/useTheme'
import { useViewport } from '@/composables/useViewport'
import { copyText } from '@/utils/clipboard'
import { extractErrorMessage } from '@/utils/entitlement'

const router = useRouter()
const route = useRoute()
const userStore = useUserPortalStore()
const { themeMode, setTheme } = useTheme()
const { isMobile } = useViewport()

const formLabelWidth = computed(() => isMobile.value ? undefined : '100px')
const notificationLabelWidth = computed(() => isMobile.value ? undefined : '140px')
const formLabelPosition = computed(() => isMobile.value ? 'top' : 'right')
const dialogWidth = computed(() => isMobile.value ? 'calc(100vw - 24px)' : '500px')

// 表单引用
const profileFormRef = ref(null)
const passwordFormRef = ref(null)
const twoFactorQRCode = ref(null)
const avatarInputRef = ref(null)

// 状态
const activeTab = ref('profile')
const saving = ref(false)
const changingPassword = ref(false)
const savingNotifications = ref(false)
const savingPreferences = ref(false)
const loggingOut = ref(false)
const showTwoFactorDialog = ref(false)
const processingTwoFactor = ref(false)
const uploadingAvatar = ref(false)
const sessionsLoading = ref(false)
const bindingTelegram = ref(false)
const unbindingTelegram = ref(false)

// 数据
const emailVerified = computed(() => Boolean(userStore.user?.email_verified ?? userStore.user?.emailVerified))
const twoFactorEnabled = computed(() => userStore.twoFactorEnabled)
const telegramBound = computed(() => Boolean(userStore.user?.telegram_bound ?? userStore.user?.telegram_id ?? userStore.user?.telegramId))
const telegramChatId = computed(() => userStore.user?.telegram_id || '')
const showForcedPasswordAlert = computed(() => {
  return route.query.forced === '1' || Boolean(userStore.user?.force_password_change ?? userStore.user?.forcePasswordChange)
})

// 表单数据
const profileForm = reactive({
  displayName: '',
  avatarUrl: ''
})

const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const notificationSettings = reactive({
  email: true,
  telegram: false,
  trafficWarning: true,
  expiryReminder: true,
  announcements: true
})

const preferences = reactive({
  theme: 'auto',
  language: 'zh-CN'
})

const telegramForm = reactive({
  chatId: ''
})

// 同步主题设置
watch(themeMode, (newMode) => {
  preferences.theme = newMode
}, { immediate: true })

// 监听偏好设置中的主题变化
watch(() => preferences.theme, (newTheme) => {
  if (newTheme !== themeMode.value) {
    setTheme(newTheme)
  }
})

watch(
  () => route.query.tab,
  (tab) => {
    activeTab.value = tab === 'security' ? 'security' : 'profile'
  },
  { immediate: true }
)

// 两步验证相关
const twoFactorSecret = ref('JBSWY3DPEHPK3PXP')
const twoFactorCode = ref('')
const backupCodes = ref(['12345678', '23456789', '34567890', '45678901', '56789012'])
const disablePassword = ref('')
const sessionDevices = ref([])
const sessionMaxDevices = ref(0)

// 验证规则
const profileRules = {
  displayName: [
    { max: 64, message: '显示名称不能超过 64 个字符', trigger: 'blur' }
  ]
}

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== passwordForm.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const passwordRules = {
  currentPassword: [
    { required: true, message: '请输入当前密码', trigger: 'blur' }
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 8, message: '密码长度不能少于 8 个字符', trigger: 'blur' },
    { pattern: /^(?=.*[A-Za-z])(?=.*\d)/, message: '密码必须包含字母和数字', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

// 方法
async function saveProfile() {
  if (!profileFormRef.value) return
  
  try {
    await profileFormRef.value.validate()
    saving.value = true
    
    const response = await userStore.updateProfile({
      display_name: profileForm.displayName,
      avatar_url: profileForm.avatarUrl
    })

    if (response?.need_email_verification) {
      ElMessage.success(response?.verification_email_sent ? '资料已保存，验证邮件已重新发送' : '资料已保存，请重新验证邮箱')
    } else {
      ElMessage.success('资料已保存')
    }
  } catch (error) {
    if (error !== false) {
      ElMessage.error(extractErrorMessage(error) || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

async function changePassword() {
  if (!passwordFormRef.value) return
  
  try {
    await passwordFormRef.value.validate()
    changingPassword.value = true
    
    await userStore.changePassword({
      current_password: passwordForm.currentPassword,
      new_password: passwordForm.newPassword
    })
    
    ElMessage.success('密码已修改')
    passwordFormRef.value.resetFields()
    if (route.query.forced === '1' || route.query.tab === 'security') {
      router.replace({ path: '/user/settings', query: { tab: 'security' } })
    }
  } catch (error) {
    if (error !== false) {
      ElMessage.error(extractErrorMessage(error) || '修改失败')
    }
  } finally {
    changingPassword.value = false
  }
}

async function resendVerification() {
  try {
    await userStore.resendVerificationEmail()
    ElMessage.success('验证邮件已发送，请前往邮箱查收')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '发送验证邮件失败')
  }
}

function goToAdminPanel() {
  if (!userStore.ensureAdminSession()) return
  router.push(userStore.adminEntryPath)
}

const formatSessionDateTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

const toCountryFlag = (countryCode) => {
  const code = String(countryCode || '').trim().toUpperCase()
  if (!/^[A-Z]{2}$/.test(code)) return ''
  return String.fromCodePoint(...[...code].map(char => char.charCodeAt(0) + 127397))
}

const buildSessionLocationText = (device) => {
  const country = device.country || '位置未知'
  const city = device.city ? ` - ${device.city}` : ''
  const flag = toCountryFlag(device.country_code || device.countryCode)
  return `${flag ? `${flag} ` : ''}${country}${city}`
}

const normalizeSessionDevice = (device, currentIP = '') => ({
  ...device,
  ip: device.ip || '-',
  isCurrent: Boolean(device.is_current ?? device.isCurrent ?? (currentIP && device.ip === currentIP)),
  userAgent: device.user_agent || device.userAgent || '',
  lastActivity: formatSessionDateTime(device.last_active || device.lastActivity),
  locationText: buildSessionLocationText(device),
  kicking: false
})

const loadSessions = async () => {
  if (!userStore.isAuthenticated) return

  sessionsLoading.value = true
  try {
    const response = await api.get('/user/devices')
    const payload = response?.data ?? response ?? {}
    const currentIP = payload.current_ip ?? payload.currentIp ?? ''
    sessionDevices.value = (payload.devices || []).map(device => normalizeSessionDevice(device, currentIP))
    sessionMaxDevices.value = Number(payload.max_devices ?? payload.maxDevices ?? 0)
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '获取登录会话失败')
  } finally {
    sessionsLoading.value = false
  }
}

const kickSession = async (device) => {
  try {
    await ElMessageBox.confirm(
      `确定要踢出设备 ${device.ip} 吗？`,
      '踢出设备',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    device.kicking = true
    await api.post(`/user/devices/${encodeURIComponent(device.ip)}/kick`)
    ElMessage.success('设备已踢出')
    await loadSessions()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(extractErrorMessage(error) || '踢出设备失败')
    }
  } finally {
    device.kicking = false
  }
}

const handleAvatarSelected = async (event) => {
  const [file] = event?.target?.files || []
  if (!file) return

  uploadingAvatar.value = true
  try {
    const response = await userStore.uploadAvatar(file)
    profileForm.avatarUrl = response?.user?.avatar_url || profileForm.avatarUrl
    ElMessage.success('头像上传成功')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '头像上传失败')
  } finally {
    uploadingAvatar.value = false
    if (avatarInputRef.value) {
      avatarInputRef.value.value = ''
    }
  }
}

const clearAvatar = async () => {
  try {
    await userStore.updateProfile({ avatar_url: '' })
    profileForm.avatarUrl = ''
    ElMessage.success('头像已清除')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '清除头像失败')
  }
}

const handleBindTelegram = async () => {
  bindingTelegram.value = true
  try {
    await userStore.bindTelegram(telegramForm.chatId.trim())
    ElMessage.success('Telegram 绑定成功，已发送验证消息')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '绑定 Telegram 失败')
  } finally {
    bindingTelegram.value = false
  }
}

const handleUnbindTelegram = async () => {
  unbindingTelegram.value = true
  try {
    await userStore.unbindTelegram()
    telegramForm.chatId = ''
    ElMessage.success('Telegram 已解绑')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '解绑 Telegram 失败')
  } finally {
    unbindingTelegram.value = false
  }
}

function toggleTwoFactor() {
  showTwoFactorDialog.value = true
  if (!twoFactorEnabled.value) {
    twoFactorCode.value = ''
    backupCodes.value = []
    generateTwoFactorQRCode()
  }
}

async function generateTwoFactorQRCode() {
  try {
    const response = await portalAuthApi.enable2FA()
    twoFactorSecret.value = response?.secret || ''
    backupCodes.value = Array.isArray(response?.backup_codes) ? response.backup_codes : []

    if (!twoFactorQRCode.value || !twoFactorSecret.value) {
      return
    }

    const issuer = encodeURIComponent('V Panel')
    const account = encodeURIComponent(userStore.username || 'user')
    const otpauthUrl = `otpauth://totp/${issuer}:${account}?secret=${twoFactorSecret.value}&issuer=${issuer}`

    await QRCode.toCanvas(twoFactorQRCode.value, otpauthUrl, {
      width: 180,
      margin: 1
    })
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '生成两步验证二维码失败')
  }
}

async function confirmTwoFactor() {
  processingTwoFactor.value = true
  try {
    if (twoFactorEnabled.value) {
      if (!disablePassword.value) {
        ElMessage.error('请输入密码')
        return
      }
      await portalAuthApi.disable2FA({ password: disablePassword.value })
      await userStore.fetchProfile({ silent: true })
      disablePassword.value = ''
      ElMessage.success('两步验证已禁用')
    } else {
      if (!twoFactorCode.value || twoFactorCode.value.length !== 6) {
        ElMessage.error('请输入 6 位验证码')
        return
      }
      await portalAuthApi.verify2FA({ code: twoFactorCode.value })
      await userStore.fetchProfile({ silent: true })
      twoFactorCode.value = ''
      ElMessage.success('两步验证已启用')
    }
    showTwoFactorDialog.value = false
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '操作失败')
  } finally {
    processingTwoFactor.value = false
  }
}

async function copyBackupCodes() {
  try {
    await copyText(backupCodes.value.join('\n'))
    ElMessage.success('备份码已复制')
  } catch (error) {
    console.error('复制备份码失败:', error)
    ElMessage.error('复制失败')
  }
}

async function saveNotifications() {
  savingNotifications.value = true
  try {
    await userStore.updateProfile({
      notify_email: notificationSettings.email,
      notify_telegram: notificationSettings.telegram,
      notify_traffic_warning: notificationSettings.trafficWarning,
      notify_expiry_reminder: notificationSettings.expiryReminder,
      notify_announcements: notificationSettings.announcements
    })
    ElMessage.success('通知设置已保存')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '保存失败')
  } finally {
    savingNotifications.value = false
  }
}

async function savePreferences() {
  savingPreferences.value = true
  try {
    await userStore.updateProfile({
      theme: preferences.theme,
      language: preferences.language
    })
    setTheme(preferences.theme)
    ElMessage.success('偏好设置已保存')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '保存失败')
  } finally {
    savingPreferences.value = false
  }
}

function logout() {
  ElMessageBox.confirm('确定要退出登录吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    loggingOut.value = true
    await userStore.logout()
    ElMessage.success('已退出登录')
    router.push('/user/login')
  }).catch(() => {}).finally(() => {
    loggingOut.value = false
  })
}

watch(
  () => userStore.user,
  (user) => {
    profileForm.displayName = user?.display_name || ''
    profileForm.avatarUrl = user?.avatar_url || ''
    notificationSettings.email = user?.notify_email ?? true
    notificationSettings.telegram = user?.notify_telegram ?? false
    notificationSettings.trafficWarning = user?.notify_traffic_warning ?? true
    notificationSettings.expiryReminder = user?.notify_expiry_reminder ?? true
    notificationSettings.announcements = user?.notify_announcements ?? true
    preferences.theme = user?.theme || preferences.theme
    preferences.language = user?.language || preferences.language
    telegramForm.chatId = user?.telegram_id || ''
  },
  { immediate: true }
)

onMounted(async () => {
  try {
    await userStore.fetchProfile({ silent: true })
    await loadSessions()
  } catch (error) {
    console.error('加载用户资料失败:', error)
  }

  profileForm.displayName = userStore.user?.display_name || ''
  profileForm.avatarUrl = userStore.user?.avatar_url || ''
})
</script>

<style scoped>
.settings-page {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin: 0;
}

.forced-password-alert {
  margin-bottom: 20px;
}

.settings-tabs {
  background: transparent;
}

.settings-card {
  border-radius: 12px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin: 0 0 16px 0;
}

.section-desc {
  font-size: 14px;
  color: var(--el-text-color-regular);
  margin: 0 0 12px 0;
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.admin-panel-entry {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  margin-bottom: 20px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--el-fill-color-lighter);
}

.admin-panel-copy {
  min-width: 0;
}

.admin-panel-entry .section-title {
  margin-bottom: 8px;
}

.admin-panel-entry .section-desc {
  margin-bottom: 0;
}

/* 头像上传 */
.avatar-upload {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}

.avatar-input-hidden {
  display: none;
}

/* 两步验证 */
.two-factor-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.two-factor-info {
  flex: 1;
}

.two-factor-info p {
  font-size: 14px;
  color: var(--el-text-color-regular);
  margin: 0 0 8px 0;
}

/* 两步验证设置 */
.two-factor-setup {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.setup-step h4 {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin: 0 0 8px 0;
}

.setup-step p {
  font-size: 14px;
  color: var(--el-text-color-regular);
  margin: 0 0 12px 0;
}

.qrcode-wrapper {
  display: flex;
  justify-content: center;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
}

.secret-key {
  text-align: center;
  font-size: 13px;
}

.secret-key code {
  background: var(--el-fill-color-light);
  padding: 4px 8px;
  border-radius: 4px;
  font-family: monospace;
}

.verify-input {
  max-width: 200px;
}

.verify-input :deep(.el-input__inner) {
  text-align: center;
  font-size: 20px;
  letter-spacing: 8px;
}

.backup-codes {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  margin-bottom: 12px;
}

.backup-codes code {
  display: block;
  padding: 8px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  text-align: center;
  font-family: monospace;
  font-size: 14px;
}

.language-select {
  width: min(200px, 100%);
}

.account-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}

.sessions-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.session-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.session-item {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
}

.session-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.session-ip {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.session-meta,
.session-agent,
.session-empty {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.session-agent {
  word-break: break-word;
}

/* 响应式 */
@media (max-width: 768px) {
  .settings-page {
    padding: 12px;
  }

  .page-title {
    font-size: 20px;
  }

  .two-factor-section {
    flex-direction: column;
    align-items: flex-start;
  }

  .admin-panel-entry {
    flex-direction: column;
    align-items: stretch;
  }

  .admin-panel-entry .el-button {
    width: 100%;
  }

  .backup-codes {
    grid-template-columns: 1fr;
  }

  .session-item {
    flex-direction: column;
  }

  .account-actions .el-button {
    width: 100%;
  }
}
</style>
