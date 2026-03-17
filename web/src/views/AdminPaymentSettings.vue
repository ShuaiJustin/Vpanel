<template>
  <div class="admin-payment-settings-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">支付配置</h1>
        <p class="page-subtitle">在商业化管理中直接维护支付宝和微信支付商户参数</p>
      </div>
      <div class="page-actions">
        <el-button :loading="loading" @click="loadSettings">刷新</el-button>
        <el-button type="primary" :loading="saving" @click="saveSettings">保存配置</el-button>
      </div>
    </div>

    <el-row :gutter="16" class="status-grid">
      <el-col :xs="24" :md="8">
        <el-card shadow="never" class="status-card">
          <div class="status-card__label">前台在线支付</div>
          <div class="status-card__value">{{ onlineMethodCountLabel }}</div>
          <div class="status-card__meta">{{ availableMethodsLabel }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="8">
        <el-card shadow="never" class="status-card">
          <div class="status-card__label">支付宝商户参数</div>
          <div class="status-card__value">
            <el-tag :type="form.alipayPrivateKeyConfigured && form.alipayPublicKey ? 'success' : 'info'">
              {{ form.alipayPrivateKeyConfigured && form.alipayPublicKey ? '已配置' : '未完整配置' }}
            </el-tag>
          </div>
          <div class="status-card__meta">
            {{ form.alipayEnabled ? '当前已启用' : '当前未启用' }}
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="8">
        <el-card shadow="never" class="status-card">
          <div class="status-card__label">微信支付商户参数</div>
          <div class="status-card__value">
            <el-tag :type="form.wechatApiKeyConfigured && form.wechatMchId ? 'success' : 'info'">
              {{ form.wechatApiKeyConfigured && form.wechatMchId ? '已配置' : '未完整配置' }}
            </el-tag>
          </div>
          <div class="status-card__meta">
            {{ form.wechatEnabled ? '当前已启用' : '当前未启用' }}
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-alert
      title="保存后会立即刷新运行中的支付网关"
      description="只有当支付方式启用且参数完整时，用户前台才会出现对应入口。余额支付不依赖外部商户参数。"
      type="info"
      :closable="false"
      show-icon
      class="page-alert"
    />

    <el-card shadow="never" v-loading="loading">
      <el-form :model="form" label-width="150px" class="payment-form">
        <el-divider content-position="left">支付宝</el-divider>
        <el-form-item label="启用支付宝">
          <el-switch v-model="form.alipayEnabled" />
        </el-form-item>
        <el-form-item label="App ID">
          <el-input v-model="form.alipayAppId" placeholder="请输入支付宝 App ID" />
        </el-form-item>
        <el-form-item label="商户私钥">
          <el-input
            v-model="form.alipayPrivateKey"
            type="textarea"
            :rows="5"
            :placeholder="form.alipayPrivateKeyConfigured ? '已配置，留空则保持不变' : '请输入支付宝 RSA 私钥 PEM 内容'"
          />
        </el-form-item>
        <el-form-item label="支付宝公钥">
          <el-input
            v-model="form.alipayPublicKey"
            type="textarea"
            :rows="5"
            placeholder="请输入支付宝公钥 PEM 内容"
          />
        </el-form-item>
        <el-form-item label="通知地址">
          <el-input
            v-model="form.alipayNotifyUrl"
            placeholder="留空则自动使用面板公网地址 + /api/payments/callback/alipay"
          />
        </el-form-item>
        <el-form-item label="返回地址">
          <el-input
            v-model="form.alipayReturnUrl"
            placeholder="留空则自动使用面板公网地址 + /user/orders"
          />
        </el-form-item>
        <el-form-item label="沙箱模式">
          <el-switch v-model="form.alipaySandbox" />
        </el-form-item>

        <el-divider content-position="left">微信支付</el-divider>
        <el-form-item label="启用微信支付">
          <el-switch v-model="form.wechatEnabled" />
        </el-form-item>
        <el-form-item label="App ID">
          <el-input v-model="form.wechatAppId" placeholder="请输入微信支付 App ID" />
        </el-form-item>
        <el-form-item label="商户号">
          <el-input v-model="form.wechatMchId" placeholder="请输入微信支付商户号" />
        </el-form-item>
        <el-form-item label="API Key">
          <el-input
            v-model="form.wechatApiKey"
            type="password"
            show-password
            :placeholder="form.wechatApiKeyConfigured ? '已配置，留空则保持不变' : '请输入微信支付 API Key'"
          />
        </el-form-item>
        <el-form-item label="通知地址">
          <el-input
            v-model="form.wechatNotifyUrl"
            placeholder="留空则自动使用面板公网地址 + /api/payments/callback/wechat"
          />
        </el-form-item>
        <el-form-item label="沙箱模式">
          <el-switch v-model="form.wechatSandbox" />
        </el-form-item>

        <el-divider />
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveSettings">保存配置</el-button>
          <el-button :loading="loading" @click="loadSettings">重新加载</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { paymentsApi, settingsApi } from '@/api'

const loading = ref(false)
const saving = ref(false)
const availableMethods = ref([])

const form = reactive({
  alipayEnabled: false,
  alipayAppId: '',
  alipayPrivateKey: '',
  alipayPrivateKeyConfigured: false,
  alipayPublicKey: '',
  alipayNotifyUrl: '',
  alipayReturnUrl: '',
  alipaySandbox: false,
  wechatEnabled: false,
  wechatAppId: '',
  wechatMchId: '',
  wechatApiKey: '',
  wechatApiKeyConfigured: false,
  wechatNotifyUrl: '',
  wechatSandbox: false
})

const methodLabels = {
  balance: '余额支付',
  alipay: '支付宝',
  wechat: '微信支付'
}

const onlineMethodCountLabel = computed(() => {
  const count = availableMethods.value.filter(method => method !== 'balance').length
  return count > 0 ? `${count} 个已启用` : '未启用'
})

const availableMethodsLabel = computed(() => {
  if (!availableMethods.value.length) {
    return '当前前台仅可见余额支付'
  }

  return availableMethods.value
    .map(method => methodLabels[method] || method)
    .join(' / ')
})

const applySettings = (settings) => {
  form.alipayEnabled = settings?.payment_alipay_enabled ?? false
  form.alipayAppId = settings?.payment_alipay_app_id || ''
  form.alipayPrivateKey = ''
  form.alipayPrivateKeyConfigured = settings?.payment_alipay_private_key_configured ?? false
  form.alipayPublicKey = settings?.payment_alipay_public_key || ''
  form.alipayNotifyUrl = settings?.payment_alipay_notify_url || ''
  form.alipayReturnUrl = settings?.payment_alipay_return_url || ''
  form.alipaySandbox = settings?.payment_alipay_sandbox ?? false
  form.wechatEnabled = settings?.payment_wechat_enabled ?? false
  form.wechatAppId = settings?.payment_wechat_app_id || ''
  form.wechatMchId = settings?.payment_wechat_mch_id || ''
  form.wechatApiKey = ''
  form.wechatApiKeyConfigured = settings?.payment_wechat_api_key_configured ?? false
  form.wechatNotifyUrl = settings?.payment_wechat_notify_url || ''
  form.wechatSandbox = settings?.payment_wechat_sandbox ?? false
}

const loadAvailableMethods = async () => {
  const response = await paymentsApi.getMethods()
  availableMethods.value = response?.methods || []
}

const loadSettings = async () => {
  loading.value = true
  try {
    const response = await settingsApi.getAll()
    applySettings(response?.data || {})
    await loadAvailableMethods()
  } catch (error) {
    ElMessage.error(error.message || '加载支付配置失败')
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  saving.value = true
  try {
    const payload = {
      payment_alipay_enabled: form.alipayEnabled,
      payment_alipay_app_id: form.alipayAppId.trim(),
      payment_alipay_public_key: form.alipayPublicKey.trim(),
      payment_alipay_notify_url: form.alipayNotifyUrl.trim(),
      payment_alipay_return_url: form.alipayReturnUrl.trim(),
      payment_alipay_sandbox: form.alipaySandbox,
      payment_wechat_enabled: form.wechatEnabled,
      payment_wechat_app_id: form.wechatAppId.trim(),
      payment_wechat_mch_id: form.wechatMchId.trim(),
      payment_wechat_notify_url: form.wechatNotifyUrl.trim(),
      payment_wechat_sandbox: form.wechatSandbox
    }

    if (form.alipayPrivateKey.trim()) {
      payload.payment_alipay_private_key = form.alipayPrivateKey.trim()
    }
    if (form.wechatApiKey.trim()) {
      payload.payment_wechat_api_key = form.wechatApiKey.trim()
    }

    const response = await settingsApi.update(payload)
    applySettings(response?.data || {})
    await loadAvailableMethods()
    ElMessage.success('支付配置已保存')
  } catch (error) {
    ElMessage.error(error.message || '保存支付配置失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.admin-payment-settings-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
}

.page-subtitle {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
}

.page-actions {
  display: flex;
  gap: 12px;
}

.status-grid {
  margin-bottom: 20px;
}

.status-card {
  min-height: 136px;
}

.status-card__label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  margin-bottom: 12px;
}

.status-card__value {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 10px;
}

.status-card__meta {
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

.page-alert {
  margin-bottom: 20px;
}

.payment-form {
  max-width: 920px;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .page-actions {
    width: 100%;
  }

  .page-actions :deep(.el-button) {
    flex: 1;
  }
}
</style>
