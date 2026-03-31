<template>
  <div class="admin-payment-settings-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          支付/充值配置
        </h1>
        <p class="page-subtitle">
          在商业化管理中统一维护订单支付与余额充值所需的支付宝和微信支付商户参数
        </p>
      </div>
      <div class="page-actions">
        <el-button
          :loading="loading"
          @click="loadSettings"
        >
          刷新
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="saveSettings"
        >
          保存配置
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">前台在线方式</span>
        <strong class="overview-value">{{ onlineMethodCountLabel }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">已配置网关</span>
        <strong class="overview-value is-primary">{{ configuredGatewayCount }} / 2</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">沙箱模式</span>
        <strong class="overview-value is-warning">{{ sandboxGatewayCount }} 个</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">前台可见方式</span>
        <strong class="overview-value is-success">{{ availableMethodHeadline }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <span class="toolbar-summary">用户前台当前展示：{{ availableMethodsLabel }}</span>
      </div>
      <div class="toolbar-actions">
        <span class="toolbar-summary">只有在线支付方式已启用且参数完整时，用户前台才会出现对应入口；余额支付不依赖外部商户参数，但余额充值依赖在线支付网关。</span>
      </div>
    </div>

    <el-alert
      title="保存后会立即刷新运行中的支付/充值网关"
      description="建议先补齐参数，再打开支付方式开关。通知地址和返回地址留空时，系统会按面板公网地址自动拼接默认回调。"
      type="info"
      :closable="false"
      show-icon
      class="page-alert"
    />

    <el-form
      v-loading="loading"
      :model="form"
      label-position="top"
      class="payment-settings-form"
    >
      <div class="detail-grid payment-config-grid">
        <el-card
          shadow="never"
          class="gateway-card"
        >
          <template #header>
            <div class="card-header gateway-card__header">
              <div class="gateway-card__heading">
                <span class="gateway-card__title">支付宝</span>
                <span class="gateway-card__subtitle">网页支付 / 回调通知 / 沙箱调试</span>
              </div>
              <span :class="['metric-pill', alipayStatusClass]">{{ alipayStatusLabel }}</span>
            </div>
          </template>

          <div class="gateway-summary">
            <div class="stack-item stack-item--inline">
              <span class="stack-label">启用状态</span>
              <el-switch v-model="form.alipayEnabled" />
            </div>
            <div class="stack-item">
              <span class="stack-label">参数情况</span>
              <span class="stack-value is-strong">{{ alipaySummary }}</span>
            </div>
            <div class="entity-cell__hint">
              {{ alipayExposureHint }}
            </div>
          </div>

          <div class="gateway-form-grid">
            <el-form-item label="App ID">
              <el-input
                v-model="form.alipayAppId"
                placeholder="请输入支付宝 App ID"
              />
            </el-form-item>
            <el-form-item label="沙箱模式">
              <el-switch v-model="form.alipaySandbox" />
            </el-form-item>
          </div>

          <el-form-item label="商户私钥">
            <el-input
              v-model="form.alipayPrivateKey"
              type="textarea"
              :rows="6"
              :placeholder="form.alipayPrivateKeyConfigured ? '已配置，留空则保持不变' : '请输入支付宝 RSA 私钥 PEM 内容'"
            />
          </el-form-item>

          <el-form-item label="支付宝公钥">
            <el-input
              v-model="form.alipayPublicKey"
              type="textarea"
              :rows="6"
              placeholder="请输入支付宝公钥 PEM 内容"
            />
          </el-form-item>

          <div class="gateway-form-grid">
            <el-form-item label="通知地址">
              <el-input
                v-model="form.alipayNotifyUrl"
                placeholder="留空则自动使用 /api/payments/callback/alipay"
              />
            </el-form-item>
            <el-form-item label="返回地址">
              <el-input
                v-model="form.alipayReturnUrl"
                placeholder="留空则自动使用 /user/orders"
              />
            </el-form-item>
          </div>

          <div class="surface-inline">
            <div class="stack-item">
              <span class="stack-label">通知回调预期</span>
              <span class="stack-value">{{ form.alipayNotifyUrl || '自动拼接 /api/payments/callback/alipay' }}</span>
            </div>
            <div class="stack-item">
              <span class="stack-label">成功返回预期</span>
              <span class="stack-value">{{ form.alipayReturnUrl || '自动拼接 /user/orders' }}</span>
            </div>
          </div>
        </el-card>

        <el-card
          shadow="never"
          class="gateway-card"
        >
          <template #header>
            <div class="card-header gateway-card__header">
              <div class="gateway-card__heading">
                <span class="gateway-card__title">微信支付</span>
                <span class="gateway-card__subtitle">商户号 / API Key / 异步通知</span>
              </div>
              <span :class="['metric-pill', wechatStatusClass]">{{ wechatStatusLabel }}</span>
            </div>
          </template>

          <div class="gateway-summary">
            <div class="stack-item stack-item--inline">
              <span class="stack-label">启用状态</span>
              <el-switch v-model="form.wechatEnabled" />
            </div>
            <div class="stack-item">
              <span class="stack-label">参数情况</span>
              <span class="stack-value is-strong">{{ wechatSummary }}</span>
            </div>
            <div class="entity-cell__hint">
              {{ wechatExposureHint }}
            </div>
          </div>

          <div class="gateway-form-grid">
            <el-form-item label="App ID">
              <el-input
                v-model="form.wechatAppId"
                placeholder="请输入微信支付 App ID"
              />
            </el-form-item>
            <el-form-item label="商户号">
              <el-input
                v-model="form.wechatMchId"
                placeholder="请输入微信支付商户号"
              />
            </el-form-item>
          </div>

          <div class="gateway-form-grid">
            <el-form-item label="API Key">
              <el-input
                v-model="form.wechatApiKey"
                type="password"
                show-password
                :placeholder="form.wechatApiKeyConfigured ? '已配置，留空则保持不变' : '请输入微信支付 API Key'"
              />
            </el-form-item>
            <el-form-item label="沙箱模式">
              <el-switch v-model="form.wechatSandbox" />
            </el-form-item>
          </div>

          <el-form-item label="通知地址">
            <el-input
              v-model="form.wechatNotifyUrl"
              placeholder="留空则自动使用 /api/payments/callback/wechat"
            />
          </el-form-item>

          <div class="surface-inline">
            <div class="stack-item">
              <span class="stack-label">通知回调预期</span>
              <span class="stack-value">{{ form.wechatNotifyUrl || '自动拼接 /api/payments/callback/wechat' }}</span>
            </div>
            <div class="stack-item">
              <span class="stack-label">前台展示条件</span>
              <span class="stack-value">启用开关 + 商户号 + API Key</span>
            </div>
          </div>
        </el-card>
      </div>

      <el-card
        shadow="never"
        class="box-card payment-guide-card"
      >
        <template #header>
          <div class="card-header">
            <span>生效说明</span>
            <span class="toolbar-summary">保存后立即刷新支付网关</span>
          </div>
        </template>
        <div class="guide-grid">
          <div class="entity-cell">
            <div class="entity-cell__title">
              网关展示逻辑
            </div>
            <div class="entity-cell__hint">
              前台可见方式：{{ availableMethodsLabel }}
            </div>
          </div>
          <div class="entity-cell">
            <div class="entity-cell__title">
              推荐顺序
            </div>
            <div class="entity-cell__hint">
              先补齐 App ID / 商户号 / 密钥，再打开启用开关，最后用沙箱或小额实付验证回调链路。
            </div>
          </div>
          <div class="entity-cell">
            <div class="entity-cell__title">
              默认回调
            </div>
            <div class="entity-cell__hint">
              通知地址留空时系统会按面板公网地址自动拼接默认路径，适合单站点部署；反向代理场景建议显式填写。
            </div>
          </div>
        </div>
      </el-card>
    </el-form>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { paymentsApi, settingsApi } from '@/api'
import { extractErrorMessage } from '@/utils/entitlement'

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

const isAlipayReady = computed(() => Boolean(form.alipayPrivateKeyConfigured && form.alipayPublicKey && form.alipayAppId))
const isWechatReady = computed(() => Boolean(form.wechatApiKeyConfigured && form.wechatMchId && form.wechatAppId))
const configuredGatewayCount = computed(() => Number(isAlipayReady.value) + Number(isWechatReady.value))
const sandboxGatewayCount = computed(() => Number(form.alipaySandbox) + Number(form.wechatSandbox))

const onlineMethodCountLabel = computed(() => {
  const count = availableMethods.value.filter((method) => method !== 'balance').length
  return count > 0 ? `${count} 个已启用` : '未启用'
})

const availableMethodsLabel = computed(() => {
  if (!availableMethods.value.length) {
    return '当前前台仅可见余额支付'
  }

  return availableMethods.value
    .map((method) => methodLabels[method] || method)
    .join(' / ')
})

const availableMethodHeadline = computed(() => {
  const onlineMethods = availableMethods.value.filter((method) => method !== 'balance')
  if (!onlineMethods.length) {
    return '仅余额'
  }

  return onlineMethods
    .map((method) => methodLabels[method] || method)
    .join(' + ')
})

const alipayStatusLabel = computed(() => {
  if (form.alipayEnabled && isAlipayReady.value) return '已启用'
  if (isAlipayReady.value) return '已配置'
  if (form.alipayEnabled) return '待补全'
  return '未配置'
})

const alipayStatusClass = computed(() => {
  if (form.alipayEnabled && isAlipayReady.value) return 'is-success'
  if (form.alipayEnabled || isAlipayReady.value) return 'is-warning'
  return 'is-muted'
})

const wechatStatusLabel = computed(() => {
  if (form.wechatEnabled && isWechatReady.value) return '已启用'
  if (isWechatReady.value) return '已配置'
  if (form.wechatEnabled) return '待补全'
  return '未配置'
})

const wechatStatusClass = computed(() => {
  if (form.wechatEnabled && isWechatReady.value) return 'is-success'
  if (form.wechatEnabled || isWechatReady.value) return 'is-warning'
  return 'is-muted'
})

const alipaySummary = computed(() => {
  if (!form.alipayAppId && !form.alipayPrivateKeyConfigured && !form.alipayPublicKey) {
    return '尚未填写商户参数'
  }

  return `${form.alipayAppId ? 'App ID 已填' : '缺少 App ID'} / ${form.alipayPrivateKeyConfigured ? '私钥已存储' : '缺少私钥'} / ${form.alipayPublicKey ? '公钥已填' : '缺少公钥'}`
})

const wechatSummary = computed(() => {
  if (!form.wechatAppId && !form.wechatMchId && !form.wechatApiKeyConfigured) {
    return '尚未填写商户参数'
  }

  return `${form.wechatAppId ? 'App ID 已填' : '缺少 App ID'} / ${form.wechatMchId ? '商户号已填' : '缺少商户号'} / ${form.wechatApiKeyConfigured ? 'API Key 已存储' : '缺少 API Key'}`
})

const alipayExposureHint = computed(() => {
  if (form.alipayEnabled && isAlipayReady.value) {
    return '支付宝当前可直接展示给用户前台。'
  }

  if (form.alipayEnabled && !isAlipayReady.value) {
    return '已打开启用开关，但参数还不完整，前台不会展示。'
  }

  if (!form.alipayEnabled && isAlipayReady.value) {
    return '参数已就绪，但当前未对用户开放。'
  }

  return '当前未启用，且参数尚未补齐。'
})

const wechatExposureHint = computed(() => {
  if (form.wechatEnabled && isWechatReady.value) {
    return '微信支付当前可直接展示给用户前台。'
  }

  if (form.wechatEnabled && !isWechatReady.value) {
    return '已打开启用开关，但参数还不完整，前台不会展示。'
  }

  if (!form.wechatEnabled && isWechatReady.value) {
    return '参数已就绪，但当前未对用户开放。'
  }

  return '当前未启用，且参数尚未补齐。'
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
    ElMessage.error(extractErrorMessage(error) || '加载支付/充值配置失败')
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
    ElMessage.success('支付/充值配置已保存')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '保存支付/充值配置失败')
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

.payment-settings-form {
  display: grid;
  gap: 20px;
}

.payment-config-grid {
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
}

.gateway-card__header {
  align-items: center;
}

.gateway-card__heading {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.gateway-card__title {
  font-size: 18px;
  font-weight: 700;
  color: var(--admin-title);
}

.gateway-card__subtitle {
  font-size: 12px;
  color: var(--admin-text-muted);
}

.gateway-summary {
  display: grid;
  gap: 12px;
  margin-bottom: 18px;
}

.gateway-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0 16px;
}

.guide-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

@media (max-width: 768px) {
  .admin-payment-settings-page {
    padding: 12px;
  }

  .payment-config-grid {
    grid-template-columns: 1fr;
  }
}
</style>
