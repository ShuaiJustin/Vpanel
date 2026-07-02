<template>
  <div class="auth-settings-page">
    <div class="page-header">
      <div class="page-heading-block">
        <el-button
          class="back-button"
          text
          :icon="ArrowLeft"
          @click="router.push({ name: 'AdminSettings' })"
        >
          配置管理
        </el-button>
        <div class="page-heading">
          <p class="page-kicker">系统设置 / 身份验证</p>
          <h1>身份验证</h1>
          <p class="page-subtitle">基本身份验证、OAuth 集成和第三方登录凭据</p>
        </div>
      </div>
      <el-button
        type="primary"
        :icon="Key"
        :loading="saving"
        class="save-button"
        @click="saveSettings"
      >
        保存设置
      </el-button>
    </div>

    <el-tabs v-model="activeTab" class="auth-tabs">
      <el-tab-pane label="基本身份验证" name="basic-auth">
        <section class="auth-section basic-section">
          <div class="section-head">
            <div>
              <h2>基本身份验证</h2>
              <p>浏览器访问面板前的额外 HTTP Basic Auth 门禁</p>
            </div>
            <div class="section-switch">
              <span>{{ basicForm.enabled ? '已启用' : '未启用' }}</span>
              <el-switch v-model="basicForm.enabled" />
            </div>
          </div>

          <el-form
            :model="basicForm"
            :label-position="labelPosition"
            :label-width="labelWidth"
            class="auth-form basic-form"
          >
            <div class="form-grid">
              <el-form-item label="Realm">
                <el-input v-model="basicForm.realm" placeholder="V Panel" />
              </el-form-item>
              <el-form-item label="用户名">
                <el-input v-model="basicForm.username" placeholder="admin" />
              </el-form-item>
              <el-form-item label="密码">
                <el-input
                  v-model="basicForm.password"
                  type="password"
                  show-password
                  :placeholder="basicForm.passwordConfigured ? '已配置，留空保持不变' : '输入 Basic Auth 密码'"
                />
                <div v-if="basicForm.passwordConfigured" class="field-tip">
                  当前已配置密码
                </div>
              </el-form-item>
              <el-form-item v-if="basicForm.passwordConfigured" label="清除密码">
                <el-checkbox v-model="basicForm.clearPassword">
                  保存时清除当前密码
                </el-checkbox>
              </el-form-item>
            </div>
          </el-form>
        </section>
      </el-tab-pane>

      <el-tab-pane label="OAuth 集成" name="oauth">
        <section class="auth-section oauth-section">
          <div class="section-head">
            <div>
              <h2>OAuth 集成</h2>
              <p>GitHub、Discord、OIDC、Telegram、LinuxDO、微信和企业微信</p>
            </div>
            <div class="section-switch">
              <span>{{ oauthForm.enabled ? '已启用' : '未启用' }}</span>
              <el-switch v-model="oauthForm.enabled" />
            </div>
          </div>

          <el-form
            :model="oauthForm"
            :label-position="labelPosition"
            :label-width="labelWidth"
            class="auth-form oauth-policy-form"
          >
            <div class="policy-grid">
              <el-form-item label="新用户注册">
                <el-switch v-model="oauthForm.allowRegistration" />
              </el-form-item>
              <el-form-item label="账号绑定">
                <el-switch v-model="oauthForm.allowAccountLinking" />
              </el-form-item>
              <el-form-item label="验证邮箱">
                <el-switch v-model="oauthForm.requireVerifiedEmail" />
              </el-form-item>
              <el-form-item label="默认角色">
                <el-select v-model="oauthForm.defaultRole">
                  <el-option label="普通用户" value="user" />
                  <el-option label="管理员" value="admin" />
                </el-select>
              </el-form-item>
            </div>
          </el-form>

          <div class="provider-layout">
            <nav class="provider-nav" aria-label="OAuth providers">
              <button
                v-for="provider in providers"
                :key="provider.key"
                type="button"
                class="provider-nav-item"
                :class="{ active: activeProvider === provider.key }"
                :aria-pressed="activeProvider === provider.key"
                @click="selectProvider(provider.key)"
              >
                <span class="provider-status" :class="{ enabled: providerForms[provider.key].enabled }" />
                <span>{{ provider.label }}</span>
              </button>
            </nav>

            <div class="provider-panel">
              <div class="provider-head">
                <div>
                  <h3>{{ activeProviderConfig.label }}</h3>
                  <p>{{ activeProviderConfig.description }}</p>
                </div>
                <div class="section-switch">
                  <span>{{ providerForms[activeProvider].enabled ? '已启用' : '未启用' }}</span>
                  <el-switch v-model="providerForms[activeProvider].enabled" />
                </div>
              </div>

              <el-form
                :model="providerForms[activeProvider]"
                :label-position="labelPosition"
                :label-width="labelWidth"
                class="auth-form"
              >
                <div class="provider-grid">
                  <el-form-item label="Client ID / App ID">
                    <el-input v-model="providerForms[activeProvider].clientId" />
                  </el-form-item>
                  <el-form-item label="Client Secret">
                    <el-input
                      v-model="providerForms[activeProvider].clientSecret"
                      type="password"
                      show-password
                      :placeholder="providerForms[activeProvider].clientSecretConfigured ? '已配置，留空保持不变' : '输入 Secret'"
                    />
                    <div v-if="providerForms[activeProvider].clientSecretConfigured" class="field-tip">
                      当前已配置 Client Secret
                    </div>
                  </el-form-item>
                  <el-form-item v-if="providerForms[activeProvider].clientSecretConfigured" label="清除 Secret">
                    <el-checkbox v-model="providerForms[activeProvider].clearClientSecret">
                      保存时清除当前 Secret
                    </el-checkbox>
                  </el-form-item>

                  <el-form-item v-if="activeProvider === 'telegram'" label="Bot Token">
                    <el-input
                      v-model="providerForms.telegram.botToken"
                      type="password"
                      show-password
                      :placeholder="providerForms.telegram.botTokenConfigured ? '已配置，留空保持不变' : '输入 Bot Token'"
                    />
                  </el-form-item>
                  <el-form-item v-if="activeProvider === 'telegram' && providerForms.telegram.botTokenConfigured" label="清除 Bot Token">
                    <el-checkbox v-model="providerForms.telegram.clearBotToken">
                      保存时清除当前 Bot Token
                    </el-checkbox>
                  </el-form-item>

                  <el-form-item v-if="activeProvider === 'wecom'" label="Corp ID">
                    <el-input v-model="providerForms.wecom.corpId" />
                  </el-form-item>
                  <el-form-item v-if="activeProvider === 'wecom'" label="Agent ID">
                    <el-input v-model="providerForms.wecom.agentId" />
                  </el-form-item>

                  <el-form-item label="回调地址">
                    <el-input
                      v-model="providerForms[activeProvider].redirectUri"
                      :placeholder="defaultRedirectUri(activeProvider)"
                    />
                  </el-form-item>
                  <el-form-item label="Scopes">
                    <el-input
                      v-model="providerForms[activeProvider].scopes"
                      placeholder="openid profile email"
                    />
                  </el-form-item>

                  <el-form-item label="授权地址">
                    <el-input v-model="providerForms[activeProvider].authorizeUrl" />
                  </el-form-item>
                  <el-form-item label="Token 地址">
                    <el-input v-model="providerForms[activeProvider].tokenUrl" />
                  </el-form-item>
                  <el-form-item label="用户信息地址">
                    <el-input v-model="providerForms[activeProvider].userInfoUrl" />
                  </el-form-item>
                  <el-form-item v-if="activeProvider === 'oidc' || activeProvider === 'custom'" label="Issuer">
                    <el-input v-model="providerForms[activeProvider].issuerUrl" />
                  </el-form-item>
                </div>
              </el-form>
            </div>
          </div>
        </section>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Key } from '@element-plus/icons-vue'
import { settingsApi } from '@/api'
import { useViewport } from '@/composables/useViewport'
import { extractErrorMessage } from '@/utils/entitlement'

const route = useRoute()
const router = useRouter()
const { isMobile } = useViewport()

const labelPosition = computed(() => (isMobile.value ? 'top' : 'right'))
const labelWidth = computed(() => (isMobile.value ? 'auto' : '156px'))
const saving = ref(false)
const activeTab = ref(route.params.section === 'oauth' ? 'oauth' : 'basic-auth')
const activeProvider = ref('github')

const providers = [
  { key: 'github', label: 'GitHub', description: 'GitHub OAuth App 登录' },
  { key: 'discord', label: 'Discord', description: 'Discord OAuth2 登录' },
  { key: 'oidc', label: 'OIDC', description: '标准 OpenID Connect Provider' },
  { key: 'telegram', label: 'Telegram', description: 'Telegram Login Widget' },
  { key: 'linuxdo', label: 'LinuxDO', description: 'LinuxDO Connect 登录' },
  { key: 'wechat', label: '微信', description: '微信开放平台扫码登录' },
  { key: 'wecom', label: '企业微信', description: '企业微信扫码登录' },
  { key: 'custom', label: '自定义 OAuth', description: '自定义 OAuth2/OIDC 兼容服务' }
]

const basicForm = reactive({
  enabled: false,
  realm: 'V Panel',
  username: '',
  password: '',
  passwordConfigured: false,
  clearPassword: false
})

const oauthForm = reactive({
  enabled: false,
  allowRegistration: false,
  allowAccountLinking: true,
  requireVerifiedEmail: true,
  defaultRole: 'user'
})

const providerForms = reactive({})
const activeProviderConfig = computed(() => providers.find((provider) => provider.key === activeProvider.value) || providers[0])

const selectProvider = (providerKey) => {
  activeProvider.value = providerKey
}

const emptyProviderForm = () => ({
  enabled: false,
  clientId: '',
  clientSecret: '',
  clientSecretConfigured: false,
  clearClientSecret: false,
  authorizeUrl: '',
  tokenUrl: '',
  userInfoUrl: '',
  issuerUrl: '',
  redirectUri: '',
  scopes: '',
  botToken: '',
  botTokenConfigured: false,
  clearBotToken: false,
  corpId: '',
  agentId: ''
})

providers.forEach((provider) => {
  providerForms[provider.key] = emptyProviderForm()
})

const normalizeSettingsResponse = (response) => response?.data || response || {}
const splitScopes = (value) => String(value || '').split(/[\s,]+/).map((item) => item.trim()).filter(Boolean)
const joinScopes = (value) => Array.isArray(value) ? value.join(' ') : ''

const defaultRedirectUri = (providerKey) => {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}/api/portal/auth/oauth/${providerKey}/callback`
}

const applyAuthSettings = (settings) => {
  const auth = settings?.auth || {}
  const basic = auth.basic_auth || {}
  Object.assign(basicForm, {
    enabled: basic.enabled ?? false,
    realm: basic.realm || 'V Panel',
    username: basic.username || '',
    password: '',
    passwordConfigured: basic.password_configured ?? false,
    clearPassword: false
  })

  const oauth = auth.oauth || {}
  Object.assign(oauthForm, {
    enabled: oauth.enabled ?? false,
    allowRegistration: oauth.allow_registration ?? false,
    allowAccountLinking: oauth.allow_account_linking ?? true,
    requireVerifiedEmail: oauth.require_verified_email ?? true,
    defaultRole: oauth.default_role || 'user'
  })

  const storedProviders = oauth.providers || {}
  providers.forEach((provider) => {
    const current = storedProviders[provider.key] || {}
    Object.assign(providerForms[provider.key], {
      enabled: current.enabled ?? false,
      clientId: current.client_id || '',
      clientSecret: '',
      clientSecretConfigured: current.client_secret_configured ?? false,
      clearClientSecret: false,
      authorizeUrl: current.authorize_url || '',
      tokenUrl: current.token_url || '',
      userInfoUrl: current.userinfo_url || '',
      issuerUrl: current.issuer_url || '',
      redirectUri: current.redirect_uri || '',
      scopes: joinScopes(current.scopes),
      botToken: '',
      botTokenConfigured: current.bot_token_configured ?? false,
      clearBotToken: false,
      corpId: current.corp_id || '',
      agentId: current.agent_id || ''
    })
  })
}

const loadSettings = async () => {
  try {
    const response = await settingsApi.getAll()
    applyAuthSettings(normalizeSettingsResponse(response))
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载身份验证设置失败')
  }
}

const buildProviderPayload = (providerKey) => {
  const form = providerForms[providerKey]
  const payload = {
    enabled: form.enabled,
    client_id: form.clientId.trim(),
    authorize_url: form.authorizeUrl.trim(),
    token_url: form.tokenUrl.trim(),
    userinfo_url: form.userInfoUrl.trim(),
    issuer_url: form.issuerUrl.trim(),
    redirect_uri: form.redirectUri.trim(),
    scopes: splitScopes(form.scopes),
    corp_id: form.corpId.trim(),
    agent_id: form.agentId.trim()
  }
  if (form.clientSecret) payload.client_secret = form.clientSecret
  if (form.clearClientSecret) payload.clear_client_secret = true
  if (form.botToken) payload.bot_token = form.botToken
  if (form.clearBotToken) payload.clear_bot_token = true
  return payload
}

const saveSettings = async () => {
  saving.value = true
  try {
    const providerPayload = {}
    providers.forEach((provider) => {
      providerPayload[provider.key] = buildProviderPayload(provider.key)
    })

    const basicPayload = {
      enabled: basicForm.enabled,
      realm: basicForm.realm.trim() || 'V Panel',
      username: basicForm.username.trim()
    }
    if (basicForm.password) basicPayload.password = basicForm.password
    if (basicForm.clearPassword) basicPayload.clear_password = true

    const response = await settingsApi.update({
      auth: {
        basic_auth: basicPayload,
        oauth: {
          enabled: oauthForm.enabled,
          allow_registration: oauthForm.allowRegistration,
          allow_account_linking: oauthForm.allowAccountLinking,
          require_verified_email: oauthForm.requireVerifiedEmail,
          default_role: oauthForm.defaultRole,
          provider_order: providers.map((provider) => provider.key),
          providers: providerPayload
        }
      }
    })
    applyAuthSettings(normalizeSettingsResponse(response))
    ElMessage.success('身份验证设置已保存')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '保存身份验证设置失败')
  } finally {
    saving.value = false
  }
}

watch(activeTab, (tab) => {
  const basePath = route.path.startsWith('/admin/') ? '/admin/system-settings/auth' : '/system-settings/auth'
  const nextPath = `${basePath}/${tab}`
  if (route.path !== nextPath) {
    router.replace(nextPath)
  }
})

onMounted(loadSettings)
</script>

<style scoped>
.auth-settings-page {
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 18px;
}

.page-heading-block {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
}

.back-button {
  flex: 0 0 auto;
  margin-top: 2px;
  padding-inline: 0;
}

.page-heading {
  min-width: 0;
}

.page-kicker {
  margin: 0 0 4px;
  color: var(--el-color-primary);
  font-size: 13px;
  font-weight: 700;
}

.page-heading h1 {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
}

.page-subtitle {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
}

.save-button {
  flex: 0 0 auto;
  min-width: 128px;
  min-height: 40px;
}

.auth-tabs {
  --el-tabs-header-height: 46px;
}

.auth-tabs :deep(.el-tabs__header) {
  margin-bottom: 18px;
}

.auth-tabs :deep(.el-tabs__item) {
  padding-inline: 22px;
  font-weight: 700;
}

.auth-tabs :deep(.el-tabs__content) {
  overflow: visible;
}

.auth-section {
  max-width: 1180px;
  padding: 20px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  background: var(--el-bg-color);
}

.section-head,
.provider-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 18px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.section-head h2,
.provider-head h3 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 20px;
  font-weight: 700;
}

.section-head p,
.provider-head p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

.section-switch {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 10px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  white-space: nowrap;
}

.auth-form {
  width: 100%;
}

.auth-form :deep(.el-form-item__content) {
  min-width: 0;
}

.auth-form :deep(.el-input),
.auth-form :deep(.el-select) {
  width: 100%;
}

.form-grid,
.policy-grid,
.provider-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(260px, 1fr));
  column-gap: 22px;
  row-gap: 2px;
}

.policy-grid {
  grid-template-columns: repeat(4, minmax(160px, 1fr));
  padding-bottom: 8px;
  margin-bottom: 18px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.field-tip {
  margin-top: 6px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.provider-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}

.provider-nav {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-fill-color-light);
}

.provider-nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 38px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--el-text-color-regular);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.provider-nav-item:hover,
.provider-nav-item.active {
  border-color: var(--el-color-primary-light-7);
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.provider-status {
  width: 8px;
  height: 8px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--el-border-color);
}

.provider-status.enabled {
  background: var(--el-color-success);
}

.provider-panel {
  min-width: 0;
}

@media (max-width: 768px) {
  .auth-settings-page {
    padding: 14px;
  }

  .page-header,
  .page-heading-block,
  .section-head,
  .provider-head {
    flex-direction: column;
  }

  .save-button {
    width: 100%;
  }

  .auth-section {
    padding: 14px;
  }

  .form-grid,
  .policy-grid,
  .provider-grid {
    grid-template-columns: 1fr;
  }

  .provider-layout {
    grid-template-columns: 1fr;
  }

  .provider-nav {
    flex-direction: row;
    overflow-x: auto;
    scrollbar-width: thin;
  }

  .provider-nav-item {
    width: auto;
    min-width: max-content;
  }
}
</style>
