<template>
  <div class="download-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        客户端下载
      </h1>
      <p class="page-subtitle">
        选择适合您设备的客户端，安装后即可导入订阅开始使用
      </p>
    </div>

    <el-alert
      v-if="showSubscriptionUnavailableAlert"
      type="warning"
      show-icon
      :closable="false"
      class="subscription-alert"
      :title="subscriptionUnavailableMessage"
    >
      <template #default>
        <div class="subscription-alert__actions">
          <el-button type="primary" @click="goToPlans">购买/续费套餐</el-button>
          <el-button @click="goToSubscription">查看订阅管理</el-button>
        </div>
      </template>
    </el-alert>

    <el-card
      class="quick-start-card"
      shadow="never"
    >
      <div class="quick-start-content">
        <div class="quick-start-main">
          <span class="platform-hint">
            <el-icon><component :is="currentPlatform?.icon || Platform" /></el-icon>
            当前平台：{{ currentPlatform?.label || '当前设备' }}
          </span>
          <h2 class="quick-start-title">
            {{ recommendedClient ? `${recommendedClient.name} 更适合当前设备` : '先选择合适的客户端' }}
          </h2>
          <p class="quick-start-subtitle">
            {{
              recommendedClient
                ? `建议先安装 ${recommendedClient.name}，再复制订阅链接导入。`
                : '请选择系统平台后，下载客户端并导入订阅。'
            }}
          </p>
          <div
            v-if="subscriptionLink"
            class="quick-start-link"
          >
            <span class="quick-start-link__label">订阅链接</span>
            <span class="quick-start-link__value">{{ subscriptionLinkPreview }}</span>
          </div>
        </div>

        <div class="quick-start-actions">
          <el-button
            type="primary"
            :disabled="!recommendedClient"
            @click="downloadClient(recommendedClient)"
          >
            <el-icon><Download /></el-icon>
            下载推荐客户端
          </el-button>
          <el-button
            :disabled="!recommendedClient"
            @click="showTutorial(recommendedClient)"
          >
            <el-icon><Document /></el-icon>
            查看教程
          </el-button>
          <el-button @click="copySubscriptionLink">
            <el-icon><Link /></el-icon>
            复制订阅链接
          </el-button>
          <el-button @click="goToSubscription">
            前往订阅管理
          </el-button>
        </div>
      </div>
    </el-card>

    <!-- 平台选择 -->
    <div class="platform-tabs">
      <el-radio-group
        v-model="selectedPlatform"
        size="large"
      >
        <el-radio-button
          v-for="platform in platforms"
          :key="platform.value"
          :value="platform.value"
        >
          <el-icon><component :is="platform.icon" /></el-icon>
          {{ platform.label }}
        </el-radio-button>
      </el-radio-group>
    </div>

    <!-- 镜像加速 -->
    <div class="mirror-toggle">
      <el-checkbox
        v-model="useChinaMirror"
        @change="onMirrorToggle"
      >
        通过国内镜像加速下载（适合大陆用户访问 GitHub 慢的场景）
      </el-checkbox>
      <el-select
        v-if="useChinaMirror"
        v-model="selectedMirror"
        size="small"
        class="mirror-select"
        @change="onMirrorChange"
      >
        <el-option
          v-for="m in availableMirrors"
          :key="m.value"
          :label="m.label"
          :value="m.value"
        />
      </el-select>
    </div>

    <!-- 客户端列表 -->
    <div class="clients-section">
      <div
        v-if="filteredClients.length === 0"
        class="clients-empty"
      >
        <el-empty
          :description="`暂无适合 ${currentPlatform?.label || '当前'} 平台的推荐客户端`"
          :image-size="96"
        />
      </div>
      <div
        v-else
        class="clients-grid"
      >
        <div
          v-for="client in filteredClients"
          :key="client.name"
          class="client-card"
        >
          <!-- 客户端信息 -->
          <div class="client-header">
            <div class="client-logo">
              <img
                v-if="client.logo"
                :src="client.logo"
                :alt="client.name"
              >
              <el-icon v-else>
                <Box />
              </el-icon>
            </div>
            <div class="client-info">
              <div class="client-title-row">
                <h3 class="client-name">
                  {{ client.name }}
                </h3>
                <span
                  v-if="client.recommended"
                  class="client-recommend-chip"
                >
                  <el-icon><Star /></el-icon>
                  首选
                </span>
              </div>
              <div class="client-meta">
                <span class="client-version">{{ getClientReleaseLabel(client) }}</span>
                <el-tag
                  v-for="badge in getClientBadges(client)"
                  :key="`${client.platform}-${client.name}-${badge.label}`"
                  :type="badge.type"
                  effect="plain"
                  size="small"
                >
                  {{ badge.label }}
                </el-tag>
              </div>
            </div>
          </div>

          <p class="client-description">
            {{ client.description }}
          </p>

          <!-- 特性标签 -->
          <div class="client-features">
            <el-tag 
              v-for="feature in client.features" 
              :key="feature"
              size="small"
              type="info"
            >
              {{ feature }}
            </el-tag>
          </div>

          <!-- 操作按钮 -->
          <div class="client-actions">
            <el-button 
              type="primary" 
              :disabled="!client.downloadUrl"
              @click="downloadClient(client)"
            >
              <el-icon><Download /></el-icon>
              下载
            </el-button>
            <el-button 
              @click="showTutorial(client)"
            >
              <el-icon><Document /></el-icon>
              教程
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 使用说明 -->
    <el-card
      class="tips-card"
      shadow="never"
    >
      <template #header>
        <span>
          <el-icon><InfoFilled /></el-icon>
          使用说明
        </span>
      </template>

      <el-collapse v-model="activeTip">
        <el-collapse-item
          title="如何选择客户端？"
          name="1"
        >
          <p>根据您的设备系统选择对应的客户端。推荐使用带有"推荐"标签的客户端，它们通常具有更好的兼容性和用户体验。</p>
        </el-collapse-item>
        <el-collapse-item
          title="如何导入订阅？"
          name="2"
        >
          <p>1. 下载并安装客户端</p>
          <p>2. 打开客户端，找到"订阅"或"配置"选项</p>
          <p>3. 添加订阅链接（可在"订阅管理"页面获取）</p>
          <p>4. 更新订阅，选择节点连接</p>
        </el-collapse-item>
        <el-collapse-item
          title="遇到问题怎么办？"
          name="3"
        >
          <p>如果在使用过程中遇到问题，您可以：</p>
          <p>1. 查看帮助中心的常见问题</p>
          <p>2. 提交工单获取技术支持</p>
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <!-- 教程对话框 -->
    <el-dialog
      v-model="tutorialVisible"
      :title="`${currentClient?.name} 使用教程`"
      :width="isMobile ? '100%' : '800px'"
      :fullscreen="isMobile"
      class="tutorial-dialog"
    >
      <div
        v-if="currentClient"
        class="tutorial-content"
      >
        <!-- 教程步骤 -->
        <el-steps
          :active="tutorialStep"
          finish-status="success"
          align-center
        >
          <el-step title="下载安装" />
          <el-step title="导入订阅" />
          <el-step title="连接使用" />
        </el-steps>

        <!-- 步骤内容 -->
        <div class="tutorial-step-content">
          <div 
            v-for="(step, index) in tutorialSteps" 
            v-show="tutorialStep === index"
            :key="index" 
            class="step-panel"
          >
            <h3>{{ step.title }}</h3>
            <div class="step-content">
              <!-- eslint-disable-next-line vue/no-v-html -->
              <div v-html="step.content" />
              
              <!-- 步骤 1 的下载按钮 -->
              <el-button 
                v-if="index === 0"
                type="primary" 
                style="margin-top: 16px"
                @click="downloadClient(currentClient)"
              >
                <el-icon><Download /></el-icon>
                立即下载
              </el-button>

              <!-- 步骤 2 的订阅链接提示 -->
              <el-alert 
                v-if="index === 1"
                type="info" 
                :closable="false"
                style="margin-top: 16px"
              >
                <template #title>
                  <div class="subscription-inline">
                    <div class="subscription-inline__header">
                      <span>您的订阅链接</span>
                      <div class="subscription-inline__actions">
                        <el-button
                          size="small"
                          link
                          @click="copySubscriptionLink"
                        >
                          复制链接
                        </el-button>
                        <el-button
                          size="small"
                          link
                          @click="goToSubscription"
                        >
                          订阅管理
                        </el-button>
                      </div>
                    </div>
                    <div class="subscription-inline__value">
                      {{ subscriptionLink || '订阅链接暂未加载，请前往订阅管理获取' }}
                    </div>
                  </div>
                </template>
              </el-alert>
            </div>
          </div>
        </div>

        <!-- 导航按钮 -->
        <div class="tutorial-actions">
          <el-button 
            v-if="tutorialStep > 0"
            @click="tutorialStep--"
          >
            上一步
          </el-button>
          <el-button 
            v-if="tutorialStep < TOTAL_TUTORIAL_STEPS - 1"
            type="primary"
            @click="tutorialStep++"
          >
            下一步
          </el-button>
          <el-button 
            v-if="tutorialStep === TOTAL_TUTORIAL_STEPS - 1"
            type="success"
            @click="tutorialVisible = false"
          >
            完成
          </el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useViewport } from '@/composables/useViewport'
import { useSubscriptionStore } from '@/stores/subscription'
import { copyText } from '@/utils/clipboard'
import { sanitizeHtml } from '@/utils/htmlSanitizer'
import { extractErrorMessage, getNoEntitlementMessage, isNoEntitlementError } from '@/utils/entitlement'
import { 
  Monitor, Iphone, Apple, Platform,
  Download, Document, Star, Box, InfoFilled, Link
} from '@element-plus/icons-vue'

const router = useRouter()
const subscriptionStore = useSubscriptionStore()
const { isMobile } = useViewport()

// 常量
const TOTAL_TUTORIAL_STEPS = 3
const paidClientNames = new Set(['Shadowrocket', 'Quantumult X', 'Surge', 'Loon'])
const archivedClientNames = new Set(['Clash for Windows', 'Qv2ray', 'SagerNet'])
const legacyClientNames = new Set(['ClashX Pro', 'V2RayXS'])
const cliClientNames = new Set(['Mihomo CLI', 'sing-box CLI', 'Xray-core'])
const vlessUnsupportedFormats = new Set(['clash', 'surge', 'quantumultx'])
const clientFormatLabels = {
  clash: 'Clash Legacy',
  clashmeta: 'Clash Meta/Mihomo',
  singbox: 'Sing-box',
  v2rayn: 'V2Ray',
  shadowrocket: 'Shadowrocket',
  surge: 'Surge',
  quantumultx: 'Quantumult X'
}
const tutorialAliases = {
  'Clash Verge Rev': 'Clash Verge'
}

// 状态
const selectedPlatform = ref('windows')
const activeTip = ref(['1'])
const tutorialVisible = ref(false)
const tutorialStep = ref(0)
const currentClient = ref(null)
const subscriptionUnavailableMessage = ref('')

// GitHub mirror accelerator. Default ON so 大陆 users get a fast download
// out of the box; toggle off if you have direct access. Persisted in
// localStorage so the choice survives across visits.
const VPANEL_MIRROR_KEY = 'vpanel_download_mirror'
const availableMirrors = [
  // Prefix form: replace `https://github.com` with `{prefix}/https://github.com`.
  // These services act as github → CN CDN proxies. Order = preference.
  { label: 'gh-proxy.com', value: 'https://gh-proxy.com/' },
  { label: 'ghproxy.net', value: 'https://ghproxy.net/' },
  { label: 'mirror.ghproxy.com', value: 'https://mirror.ghproxy.com/' },
]
const useChinaMirror = ref(false)
const selectedMirror = ref(availableMirrors[0].value)
try {
  const saved = JSON.parse(localStorage.getItem(VPANEL_MIRROR_KEY) || 'null')
  if (saved && typeof saved.enabled === 'boolean') {
    useChinaMirror.value = saved.enabled
    if (saved.url && availableMirrors.some(m => m.value === saved.url)) {
      selectedMirror.value = saved.url
    }
  }
} catch {
  /* ignore corrupt localStorage */
}
function persistMirrorChoice() {
  try {
    localStorage.setItem(VPANEL_MIRROR_KEY, JSON.stringify({
      enabled: useChinaMirror.value,
      url: selectedMirror.value,
    }))
  } catch { /* quota / private mode */ }
}
function onMirrorToggle() { persistMirrorChoice() }
function onMirrorChange() { persistMirrorChoice() }

// rewriteIfGithub prepends the chosen mirror to github.com URLs when the
// toggle is on. Non-github URLs (Apple Store, official sites, local) pass
// through untouched.
function rewriteIfGithub(url) {
  if (!useChinaMirror.value || !url || !url.includes('github.com')) {
    return url
  }
  const prefix = selectedMirror.value.replace(/\/+$/, '')
  return `${prefix}/${url}`
}

// 平台列表
const platforms = [
  { value: 'windows', label: 'Windows', icon: Monitor },
  { value: 'macos', label: 'macOS', icon: Apple },
  { value: 'linux', label: 'Linux', icon: Monitor },
  { value: 'ios', label: 'iOS', icon: Iphone },
  { value: 'android', label: 'Android', icon: Iphone }
]

// 客户端列表
// 注：version 字段历史上用于显示版本号，当前 UI 已改为显示下载渠道
// (getClientReleaseLabel)，即"GitHub Releases / App Store / 官网"，避免硬编码
// 版本号随上游迭代过期。保留 name/platform/description/features/downloadUrl 即可。
const clients = [
  // Windows
  {
    name: 'Clash Verge Rev',
    platform: 'windows',
    description: '现代化桌面客户端，内置 Mihomo 核心，适合大多数 Windows 用户。',
    features: ['规则分流', 'TUN 模式', '系统代理'],
    format: 'clashmeta',
    recommended: true,
    downloadUrl: 'https://github.com/clash-verge-rev/clash-verge-rev/releases',
    tutorialUrl: '#'
  },
  {
    name: 'v2rayN',
    platform: 'windows',
    description: '老牌 Windows 客户端，适合直接导入 V2Ray/Xray 通用订阅。',
    features: ['Xray 核心', 'sing-box 核心', '订阅管理'],
    format: 'v2rayn',
    recommended: false,
    downloadUrl: 'https://github.com/2dust/v2rayN/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Hiddify',
    platform: 'windows',
    description: '跨平台通用代理客户端，界面简单，适合新手和 sing-box/Xray 节点。',
    features: ['跨平台', '自动选择', 'TUN 模式'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://github.com/hiddify/hiddify-app/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Clash for Windows',
    platform: 'windows',
    description: '经典 Clash 客户端，已停止维护，不支持 VLESS/Reality 节点。',
    features: ['旧版客户端', '规则分流', '不支持 VLESS'],
    format: 'clash',
    recommended: false,
    downloadUrl: 'https://archive.org/download/clash_for_windows_pkg/',
    tutorialUrl: '#'
  },
  // macOS
  {
    name: 'Clash Verge Rev',
    platform: 'macos',
    description: '现代化 macOS 桌面客户端，支持 Apple Silicon 与 Intel。',
    features: ['规则分流', 'TUN 模式', '跨平台'],
    format: 'clashmeta',
    recommended: true,
    downloadUrl: 'https://github.com/clash-verge-rev/clash-verge-rev/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Hiddify',
    platform: 'macos',
    description: '跨平台通用代理客户端，适合想要简单导入和自动选择节点的用户。',
    features: ['跨平台', '自动选择', 'TUN 模式'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://github.com/hiddify/hiddify-app/releases',
    tutorialUrl: '#'
  },
  {
    name: 'sing-box VT',
    platform: 'macos',
    description: 'sing-box 官方 Apple 平台客户端，适合使用 sing-box 配置的用户。',
    features: ['sing-box', 'TUN 模式', 'Apple 平台'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://sing-box.sagernet.org/clients/apple/',
    tutorialUrl: '#'
  },
  {
    name: 'Surge',
    platform: 'macos',
    description: '专业级网络调试与代理工具，适合高级用户（付费软件）。',
    features: ['专业级', '网络调试', '规则分流'],
    format: 'surge',
    recommended: false,
    downloadUrl: 'https://nssurge.com/',
    tutorialUrl: '#'
  },
  {
    name: 'ClashX Pro',
    platform: 'macos',
    description: '老牌 macOS Clash 客户端，适合旧配置，不支持 VLESS/Reality 节点。',
    features: ['旧版客户端', '菜单栏', '不支持 VLESS'],
    format: 'clash',
    recommended: false,
    downloadUrl: 'https://install.appcenter.ms/users/clashx/apps/clashx-pro/distribution_groups/public',
    tutorialUrl: '#'
  },
  // Linux
  {
    name: 'Clash Verge Rev',
    platform: 'linux',
    description: 'Linux 桌面首选客户端，提供 deb/rpm/AppImage 安装包。',
    features: ['图形界面', 'TUN 模式', 'deb/rpm'],
    format: 'clashmeta',
    recommended: true,
    downloadUrl: 'https://github.com/clash-verge-rev/clash-verge-rev/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Mihomo CLI',
    platform: 'linux',
    description: 'Clash Meta 的命令行核心，适合 Linux 服务器、Shell 环境和规则分流。',
    features: ['命令行', '规则分流', '服务器'],
    format: 'clashmeta',
    recommended: false,
    downloadUrl: '/downloads/mihomo-manager.sh',
    downloadFileName: 'mihomo-manager.sh',
    tutorialUrl: '#'
  },
  {
    name: 'sing-box CLI',
    platform: 'linux',
    description: '现代通用代理核心，适合使用 sing-box 配置和新协议的命令行用户。',
    features: ['命令行', '现代配置', '新协议'],
    format: 'singbox',
    recommended: false,
    downloadUrl: '/downloads/sing-box-manager.sh',
    downloadFileName: 'sing-box-manager.sh',
    tutorialUrl: '#'
  },
  {
    name: 'Xray-core',
    platform: 'linux',
    description: 'Xray 官方核心，适合只需要 VLESS/VMess/Trojan/SS 的用户。',
    features: ['命令行', 'Xray 核心', '轻量级'],
    format: 'v2rayn',
    recommended: false,
    downloadUrl: '/downloads/xray-manager.sh',
    downloadFileName: 'xray-manager.sh',
    tutorialUrl: '#'
  },
  {
    name: 'Hiddify',
    platform: 'linux',
    description: 'Linux 图形客户端，适合不想手写命令行配置的新手用户。',
    features: ['图形界面', '自动选择', 'AppImage'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://github.com/hiddify/hiddify-app/releases',
    tutorialUrl: '#'
  },
  {
    name: 'v2rayN',
    platform: 'linux',
    description: '支持 Linux 的图形客户端，适合习惯 V2Ray/Xray 订阅的用户。',
    features: ['图形界面', 'Xray 核心', 'sing-box 核心'],
    format: 'v2rayn',
    recommended: false,
    downloadUrl: 'https://github.com/2dust/v2rayN/releases',
    tutorialUrl: '#'
  },
  // iOS
  {
    name: 'Shadowrocket',
    platform: 'ios',
    description: 'iOS 常用代理客户端，兼容性好，适合多数用户（付费软件）。',
    features: ['多协议', '规则分流', '按需连接'],
    format: 'shadowrocket',
    recommended: true,
    downloadUrl: 'https://apps.apple.com/app/shadowrocket/id932747118',
    tutorialUrl: '#'
  },
  {
    name: 'Hiddify',
    platform: 'ios',
    description: '免费跨平台客户端，适合希望简单导入订阅的新手用户。',
    features: ['免费', '跨平台', '自动选择'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://apps.apple.com/us/app/hiddify-proxy-vpn/id6596777532',
    tutorialUrl: '#'
  },
  {
    name: 'sing-box VT',
    platform: 'ios',
    description: 'sing-box 官方 Apple 平台客户端，适合使用 sing-box 配置。',
    features: ['sing-box', 'TUN 模式', '免费'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://apps.apple.com/us/app/sing-box-vt/id6673731168',
    tutorialUrl: '#'
  },
  {
    name: 'Quantumult X',
    platform: 'ios',
    description: '功能强大的网络工具，支持复杂规则。',
    features: ['脚本支持', '规则分流', 'MitM'],
    format: 'quantumultx',
    recommended: false,
    downloadUrl: 'https://apps.apple.com/app/quantumult-x/id1443988620',
    tutorialUrl: '#'
  },
  {
    name: 'Surge',
    platform: 'ios',
    description: '专业级网络调试工具（付费软件）。',
    features: ['专业级', '网络调试', 'MitM'],
    format: 'surge',
    recommended: false,
    downloadUrl: 'https://apps.apple.com/app/surge-5/id1442620678',
    tutorialUrl: '#'
  },
  {
    name: 'Loon',
    platform: 'ios',
    description: '功能强大的代理工具，支持脚本。',
    features: ['脚本支持', '规则分流', '插件系统'],
    format: 'shadowrocket',
    recommended: false,
    downloadUrl: 'https://apps.apple.com/app/loon/id1373567447',
    tutorialUrl: '#'
  },
  // Android
  {
    name: 'Clash Meta for Android',
    platform: 'android',
    description: 'Android 上的 Clash Meta/Mihomo 客户端，适合规则分流用户。',
    features: ['Clash Meta', '规则分流', '自动更新'],
    format: 'clashmeta',
    recommended: true,
    downloadUrl: 'https://github.com/MetaCubeX/ClashMetaForAndroid/releases',
    tutorialUrl: '#'
  },
  {
    name: 'v2rayNG',
    platform: 'android',
    description: 'Android 上的 V2Ray/Xray 客户端，轻量稳定。',
    features: ['多协议', '轻量级', '订阅管理'],
    format: 'v2rayn',
    recommended: false,
    downloadUrl: 'https://github.com/2dust/v2rayNG/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Hiddify',
    platform: 'android',
    description: '免费跨平台客户端，适合新手导入订阅后直接使用。',
    features: ['免费', '跨平台', '自动选择'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://github.com/hiddify/hiddify-app/releases',
    tutorialUrl: '#'
  },
  {
    name: 'NekoBox for Android',
    platform: 'android',
    description: '基于 sing-box 的 Android 通用代理客户端，适合进阶用户。',
    features: ['sing-box', '多协议', '订阅管理'],
    format: 'singbox',
    recommended: false,
    downloadUrl: 'https://github.com/MatsuriDayo/NekoBoxForAndroid/releases',
    tutorialUrl: '#'
  },
  {
    name: 'FlClash',
    platform: 'android',
    description: '基于 Clash Meta 的多平台客户端，Android 上界面简洁。',
    features: ['Clash Meta', '规则分流', '跨平台'],
    format: 'clashmeta',
    recommended: false,
    downloadUrl: 'https://github.com/chen08209/FlClash/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Surfboard',
    platform: 'android',
    description: '支持 Surge 配置的 Android 客户端。',
    features: ['Surge 配置', '规则分流', '美观界面'],
    format: 'surge',
    recommended: false,
    downloadUrl: 'https://github.com/getsurfboard/surfboard/releases',
    tutorialUrl: '#'
  }
]

// 教程内容
const tutorials = {
  'Clash Verge': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>根据您的系统选择对应的安装包：
          <ul>
            <li>Windows: <code>Clash.Verge_xxx_x64-setup.exe</code></li>
            <li>macOS: <code>Clash.Verge_xxx_x64.dmg</code></li>
            <li>Linux: <code>clash-verge_xxx_amd64.deb</code> 或 <code>.AppImage</code></li>
          </ul>
        </li>
        <li>下载完成后，双击安装包进行安装</li>
        <li>首次运行可能需要安装 Service Mode（服务模式），按提示操作即可</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 Clash Verge 客户端</li>
        <li>点击左侧菜单的 <strong>"订阅"</strong> 选项</li>
        <li>点击右上角的 <strong>"新建"</strong> 按钮</li>
        <li>在弹出的对话框中：
          <ul>
            <li>类型选择：<strong>URL</strong></li>
            <li>名称：随意填写（如：我的订阅）</li>
            <li>订阅链接：粘贴您的订阅链接</li>
          </ul>
        </li>
        <li>点击 <strong>"保存"</strong>，等待订阅更新完成</li>
        <li>更新成功后，您将看到所有可用节点</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在 <strong>"代理"</strong> 页面，选择一个节点</li>
        <li>点击主界面的 <strong>"系统代理"</strong> 开关，启用代理</li>
        <li>（可选）启用 <strong>"TUN 模式"</strong> 以实现全局代理</li>
        <li>打开浏览器，访问 <a href="https://www.google.com" target="_blank">google.com</a> 测试连接</li>
        <li>如需切换节点，返回代理页面选择其他节点即可</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>建议启用 <strong>"自动更新订阅"</strong>，保持节点信息最新</li>
          <li>可以在设置中配置开机自启动</li>
          <li>TUN 模式需要管理员权限，但可以代理所有应用</li>
        </ul>
      </div>
    `
  },
  'v2rayN': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>根据系统下载对应版本：
          <ul>
            <li>Windows：<code>v2rayN-With-Core.zip</code></li>
            <li>Linux：<code>v2rayN-linux-64.deb</code> 或 Linux 压缩包</li>
            <li>macOS：选择 macOS 对应压缩包或安装包</li>
          </ul>
        </li>
        <li>Windows 解压后运行 <code>v2rayN.exe</code>；Linux/macOS 按 Release 页面说明安装后启动</li>
        <li>首次运行可能会在托盘或菜单栏显示图标</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 v2rayN 主界面或右键托盘图标</li>
        <li>进入 <strong>"订阅分组" → "订阅分组设置"</strong></li>
        <li>点击 <strong>"添加"</strong> 按钮</li>
        <li>填写信息：
          <ul>
            <li>别名：随意填写</li>
            <li>可选地址（url）：粘贴您的订阅链接</li>
          </ul>
        </li>
        <li>点击 <strong>"确定"</strong> 保存</li>
        <li>返回主界面，选择 <strong>"订阅分组" → "更新全部订阅"</strong></li>
      </ol>
    `,
    step3: `
      <ol>
        <li>右键托盘图标，在服务器列表中选择一个节点</li>
        <li>选择 <strong>"系统代理" → "自动配置系统代理"</strong></li>
        <li>确认托盘图标变为彩色（表示已连接）</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>可以使用 <strong>"服务器" → "测试服务器真连接延迟"</strong> 测速</li>
          <li>支持路由规则设置，实现智能分流</li>
          <li>建议定期更新订阅以获取最新节点</li>
        </ul>
      </div>
    `
  },
  'Mihomo CLI': {
    step1: `
      <ol>
        <li>点击下载按钮，下载 <code>mihomo-manager.sh</code> 管理脚本</li>
        <li>上传到 Linux 服务器后执行：
          <pre><code>chmod +x mihomo-manager.sh
sudo ./mihomo-manager.sh</code></pre>
        </li>
        <li>进入菜单后选择 <strong>1) 安装 Mihomo</strong>；也可以直接执行 <code>sudo ./mihomo-manager.sh install</code></li>
        <li>脚本会自动识别系统架构、下载 Mihomo、创建 systemd 服务和示例配置</li>
        <li>国内访问 GitHub 慢时，脚本默认优先使用内置加速地址，低速会自动切换下一个地址</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>在订阅管理页选择 <strong>Clash Meta/Mihomo</strong> 格式并复制订阅链接</li>
        <li>使用脚本更新订阅：
          <pre><code>sudo ./mihomo-manager.sh update "您的订阅链接"</code></pre>
        </li>
        <li>脚本会保存订阅链接、校验配置，并自动备份旧配置</li>
        <li>以后更新可直接执行：<code>sudo ./mihomo-manager.sh update</code></li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>订阅链接需要用引号包裹，避免特殊字符导致命令错误。
      </div>
    `,
    step3: `
      <ol>
        <li>启动服务：<code>sudo ./mihomo-manager.sh start</code></li>
        <li>启用开机自启：<code>sudo ./mihomo-manager.sh enable</code></li>
        <li>一键验证是否可用：<code>sudo ./mihomo-manager.sh verify</code></li>
        <li>默认监听端口：
          <ul>
            <li>Mixed/HTTP 代理：7890</li>
            <li>HTTP 代理：7891</li>
            <li>SOCKS5 代理：7892</li>
            <li>控制面板：9090</li>
          </ul>
        </li>
        <li>配置系统代理（可选）：
          <pre><code>export http_proxy=http://127.0.0.1:7890
export https_proxy=http://127.0.0.1:7890</code></pre>
        </li>
        <li>查看状态：<code>sudo ./mihomo-manager.sh status</code></li>
        <li>查看日志：<code>sudo ./mihomo-manager.sh logs</code></li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>可以访问 <code>http://127.0.0.1:9090/ui</code> 使用 Web 控制面板</li>
          <li>默认不会写入全局系统代理，避免服务停止时影响系统下载</li>
          <li>需要写入环境代理时执行：<code>sudo ./mihomo-manager.sh proxy on</code></li>
        </ul>
      </div>
    `
  },
  'sing-box CLI': {
    step1: `
      <ol>
        <li>点击下载按钮，下载 <code>sing-box-manager.sh</code> 管理脚本</li>
        <li>上传到 Linux 服务器后执行：
          <pre><code>chmod +x sing-box-manager.sh
sudo ./sing-box-manager.sh</code></pre>
        </li>
        <li>进入菜单后选择 <strong>1) 安装 sing-box</strong>；也可以直接执行 <code>sudo ./sing-box-manager.sh install</code></li>
        <li>脚本会自动识别架构、优先使用内置加速地址下载 sing-box，并创建 systemd 服务</li>
        <li>默认本机 mixed HTTP/SOCKS 入口为 <code>127.0.0.1:2080</code></li>
      </ol>
    `,
    step2: `
      <ol>
        <li>在订阅管理页选择 <strong>Sing-box</strong> 格式并复制链接</li>
        <li>使用脚本更新订阅：
          <pre><code>sudo ./sing-box-manager.sh update "您的 Sing-box 订阅链接"</code></pre>
        </li>
        <li>脚本会保存订阅链接、校验配置、自动补齐本机 mixed 入站，并备份旧配置</li>
        <li>以后更新可直接执行：<code>sudo ./sing-box-manager.sh update</code></li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>订阅链接需要用引号包裹，避免特殊字符导致命令错误。
      </div>
    `,
    step3: `
      <ol>
        <li>启动服务：<code>sudo ./sing-box-manager.sh start</code></li>
        <li>启用开机自启：<code>sudo ./sing-box-manager.sh enable</code></li>
        <li>一键验证是否可用：<code>sudo ./sing-box-manager.sh verify</code></li>
        <li>查看状态：<code>sudo ./sing-box-manager.sh status</code></li>
        <li>查看日志：<code>sudo ./sing-box-manager.sh logs</code></li>
        <li>配置系统代理（可选）：
          <pre><code>export http_proxy=http://127.0.0.1:2080
export https_proxy=http://127.0.0.1:2080</code></pre>
        </li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>默认不会写入全局系统代理，避免服务停止时影响系统下载；需要写入环境代理时执行 <code>sudo ./sing-box-manager.sh proxy on</code>。
      </div>
    `
  },
  'Xray-core': {
    step1: `
      <ol>
        <li>点击下载按钮，下载 <code>xray-manager.sh</code> 管理脚本</li>
        <li>上传到 Linux 服务器后执行：
          <pre><code>chmod +x xray-manager.sh
sudo ./xray-manager.sh</code></pre>
        </li>
        <li>进入菜单后选择 <strong>1) 安装 Xray-core</strong>；也可以直接执行 <code>sudo ./xray-manager.sh install</code></li>
        <li>脚本会自动识别架构，优先使用内置加速地址下载 Xray-core，并创建 systemd 服务</li>
        <li>检查状态：<code>sudo ./xray-manager.sh status</code></li>
      </ol>
    `,
    step2: `
      <ol>
        <li>配置文件位置：<code>/usr/local/etc/xray/config.json</code></li>
        <li>编辑配置：<code>sudo ./xray-manager.sh edit</code></li>
        <li>Xray-core 通常使用单节点 JSON 配置；<code>format=clashmeta</code> 链接请使用 Mihomo CLI，V2Ray/v2rayN 分享链接订阅也不能被 Xray-core 直接加载</li>
        <li>如果已有完整 Xray JSON 配置链接，也可以执行：
          <pre><code>sudo ./xray-manager.sh update "您的 Xray JSON 配置链接"</code></pre>
        </li>
        <li>验证配置：<code>sudo ./xray-manager.sh test</code></li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>Xray-core 更适合熟悉 JSON 配置的用户；需要直接使用平台订阅或规则分流时优先用 Mihomo 或 sing-box。
      </div>
    `,
    step3: `
      <ol>
        <li>启动服务：<code>sudo ./xray-manager.sh start</code></li>
        <li>启用开机自启：<code>sudo ./xray-manager.sh enable</code></li>
        <li>一键验证是否可用：<code>sudo ./xray-manager.sh verify</code></li>
        <li>查看日志：<code>sudo ./xray-manager.sh logs</code></li>
        <li>默认示例入口端口：HTTP <code>10809</code>，SOCKS <code>10808</code></li>
        <li>根据配置中的 inbound 端口设置系统代理或应用代理</li>
        <li>测试连接：<code>curl -I https://www.google.com</code></li>
      </ol>
    `
  },
  'Hiddify': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 Hiddify 官方下载页或应用商店</li>
        <li>按当前系统下载安装包：
          <ul>
            <li>Windows：便携版或安装版</li>
            <li>macOS：<code>.pkg</code> 安装包</li>
            <li>Linux：<code>.AppImage</code> 或对应发行版包</li>
            <li>iOS / Android：从 App Store、Google Play 或 GitHub Releases 安装</li>
          </ul>
        </li>
        <li>安装完成后打开 Hiddify</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>复制您的订阅链接</li>
        <li>在 Hiddify 中点击添加配置或导入配置</li>
        <li>粘贴订阅链接并保存</li>
        <li>等待客户端拉取节点列表</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>选择自动节点或手动选择一个节点</li>
        <li>点击连接开关</li>
        <li>桌面端可按需开启系统代理或 TUN 模式</li>
        <li>打开浏览器测试连接</li>
      </ol>
    `
  },
  'sing-box VT': {
    step1: `
      <ol>
        <li>点击下载按钮，打开 sing-box Apple 平台客户端页面</li>
        <li>根据页面提示从 App Store / TestFlight 安装</li>
        <li>安装完成后打开 sing-box VT</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>在订阅管理页选择 <strong>Sing-box</strong> 格式并复制链接</li>
        <li>在 sing-box VT 中新建远程配置</li>
        <li>粘贴订阅链接并更新配置</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>选中刚导入的配置</li>
        <li>启动连接，首次使用按系统提示允许 VPN/TUN 权限</li>
        <li>打开浏览器测试连接</li>
      </ol>
    `
  },
  'NekoBox for Android': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载与手机 ABI 对应的 apk，通常选择 <code>arm64-v8a</code> 或 universal</li>
        <li>允许安装未知来源应用并完成安装</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 NekoBox for Android</li>
        <li>添加订阅或从剪贴板导入订阅链接</li>
        <li>建议使用 <strong>Sing-box</strong> 格式订阅</li>
        <li>保存后更新订阅</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>选择一个节点</li>
        <li>点击连接按钮，首次使用允许创建 VPN 连接</li>
        <li>打开浏览器测试连接</li>
      </ol>
    `
  },
  'Qv2ray': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载对应系统的安装包：
          <ul>
            <li>Linux: <code>Qv2ray.xxx.AppImage</code> 或 <code>.deb</code></li>
            <li>Windows: <code>Qv2ray.xxx.exe</code></li>
            <li>macOS: <code>Qv2ray.xxx.dmg</code></li>
          </ul>
        </li>
        <li>Linux AppImage 需要添加执行权限：<code>chmod +x Qv2ray.xxx.AppImage</code></li>
        <li>双击运行或安装</li>
        <li>首次运行需要配置 V2Ray 核心路径</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 Qv2ray 应用</li>
        <li>点击 <strong>"分组"</strong> → <strong>"订阅设置"</strong></li>
        <li>点击 <strong>"添加订阅"</strong></li>
        <li>填写信息：
          <ul>
            <li>订阅名称：随意填写</li>
            <li>订阅地址：粘贴您的订阅链接</li>
            <li>更新间隔：建议设置为自动更新</li>
          </ul>
        </li>
        <li>点击 <strong>"确定"</strong></li>
        <li>右键订阅组，选择 <strong>"更新订阅"</strong></li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在节点列表中，双击选择一个节点</li>
        <li>点击主界面的 <strong>"连接"</strong> 按钮</li>
        <li>或者右键托盘图标，选择 <strong>"连接"</strong></li>
        <li>连接成功后，托盘图标会变色</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>支持插件系统，可以扩展更多功能</li>
          <li>可以在设置中配置系统代理和路由规则</li>
          <li>支持延迟测试和自动选择最快节点</li>
        </ul>
      </div>
    `
  },
  'Surfboard': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载 <code>Surfboard-xxx.apk</code></li>
        <li>在手机上打开下载的 APK 文件</li>
        <li>允许安装未知来源应用（如有提示）</li>
        <li>安装完成后打开应用</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 Surfboard 应用</li>
        <li>点击右上角的 <strong>"+"</strong> 按钮</li>
        <li>选择 <strong>"从 URL 下载配置"</strong></li>
        <li>填写信息：
          <ul>
            <li>配置名称：随意填写</li>
            <li>配置 URL：粘贴您的订阅链接</li>
          </ul>
        </li>
        <li>点击 <strong>"下载"</strong></li>
        <li>等待配置下载完成</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在配置列表中，点击刚下载的配置</li>
        <li>点击底部的 <strong>"启动"</strong> 按钮</li>
        <li>首次使用需要允许创建 VPN 连接</li>
        <li>连接成功后，状态栏会显示钥匙图标</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>支持 Surge 配置格式</li>
          <li>可以在策略组中切换不同节点</li>
          <li>支持规则分流和自定义规则</li>
        </ul>
      </div>
    `
  },
  'SagerNet': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载对应架构的 APK 文件：
          <ul>
            <li>arm64-v8a（推荐）</li>
            <li>armeabi-v7a</li>
            <li>universal（通用版本）</li>
          </ul>
        </li>
        <li>在手机上打开下载的 APK 文件</li>
        <li>允许安装未知来源应用（如有提示）</li>
        <li>安装完成后打开应用</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 SagerNet 应用</li>
        <li>点击右上角的 <strong>"+"</strong> 按钮</li>
        <li>选择 <strong>"从订阅导入"</strong></li>
        <li>填写信息：
          <ul>
            <li>名称：随意填写</li>
            <li>URL：粘贴您的订阅链接</li>
            <li>自动更新：建议开启</li>
          </ul>
        </li>
        <li>点击 <strong>"确定"</strong></li>
        <li>等待订阅更新完成</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在配置列表中，点击选择一个节点</li>
        <li>点击底部的 <strong>"连接"</strong> 按钮</li>
        <li>首次使用需要允许创建 VPN 连接</li>
        <li>连接成功后，状态栏会显示钥匙图标</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>基于 sing-box 核心，性能优秀</li>
          <li>支持插件系统，可扩展功能</li>
          <li>支持多种协议和路由规则</li>
        </ul>
      </div>
    `
  },
  'V2RayXS': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载 <code>V2RayXS.dmg</code></li>
        <li>打开 dmg 文件，将 V2RayXS 拖到 Applications 文件夹</li>
        <li>首次打开时，可能需要在 <strong>"系统偏好设置" → "安全性与隐私"</strong> 中允许运行</li>
        <li>运行后会在菜单栏显示图标</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>点击菜单栏的 V2RayXS 图标</li>
        <li>选择 <strong>"订阅设置"</strong></li>
        <li>点击 <strong>"+"</strong> 添加订阅</li>
        <li>填写信息：
          <ul>
            <li>备注：随意填写</li>
            <li>URL：粘贴您的订阅链接</li>
          </ul>
        </li>
        <li>点击 <strong>"确定"</strong></li>
        <li>点击 <strong>"更新订阅"</strong></li>
      </ol>
    `,
    step3: `
      <ol>
        <li>点击菜单栏图标，在服务器列表中选择一个节点</li>
        <li>选择 <strong>"打开 V2Ray"</strong></li>
        <li>启用 <strong>"系统代理"</strong></li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>轻量级客户端，占用资源少</li>
          <li>界面简洁，操作简单</li>
          <li>适合日常使用</li>
        </ul>
      </div>
    `
  },
  'Loon': {
    step1: `
      <ol>
        <li>在 App Store 搜索 <strong>"Loon"</strong></li>
        <li>购买并下载（需要非中国区 Apple ID，价格约 $5.99）</li>
        <li>安装完成后打开应用</li>
      </ol>
      <div class="tip-box">
        <strong>注意：</strong>中国区 App Store 已下架此应用，需要使用美区或其他地区账号购买。
      </div>
    `,
    step2: `
      <ol>
        <li>打开 Loon 应用</li>
        <li>点击 <strong>"配置"</strong> 标签</li>
        <li>点击 <strong>"订阅"</strong></li>
        <li>点击右上角的 <strong>"+"</strong> 按钮</li>
        <li>填写信息：
          <ul>
            <li>别名：随意填写</li>
            <li>URL：粘贴您的订阅链接</li>
          </ul>
        </li>
        <li>点击 <strong>"保存"</strong></li>
        <li>点击订阅右侧的刷新按钮更新</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>返回 <strong>"仪表"</strong> 标签</li>
        <li>在节点列表中选择一个节点</li>
        <li>点击顶部的连接开关</li>
        <li>首次使用需要允许添加 VPN 配置</li>
        <li>连接成功后，状态栏会显示 VPN 图标</li>
        <li>打开 Safari 浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>支持 JavaScript 脚本，功能强大</li>
          <li>支持插件系统和规则分流</li>
          <li>可以自定义 MitM 和重写规则</li>
        </ul>
      </div>
    `
  },
  'Quantumult X': {
    step1: `
      <ol>
        <li>在 App Store 搜索 <strong>"Quantumult X"</strong></li>
        <li>购买并下载（需要非中国区 Apple ID，价格约 $7.99）</li>
        <li>安装完成后打开应用</li>
      </ol>
      <div class="tip-box">
        <strong>注意：</strong>中国区 App Store 已下架此应用，需要使用美区或其他地区账号购买。
      </div>
    `,
    step2: `
      <ol>
        <li>打开 Quantumult X 应用</li>
        <li>点击右下角的 <strong>"风车"</strong> 图标</li>
        <li>选择 <strong>"节点"</strong> → <strong>"引用（订阅）"</strong></li>
        <li>点击右上角的 <strong>"+"</strong> 按钮</li>
        <li>填写信息：
          <ul>
            <li>标签：随意填写</li>
            <li>资源路径：粘贴您的订阅链接</li>
          </ul>
        </li>
        <li>点击右上角 <strong>"保存"</strong></li>
        <li>长按订阅，选择 <strong>"更新"</strong></li>
      </ol>
    `,
    step3: `
      <ol>
        <li>返回首页</li>
        <li>点击右下角的 <strong>"节点"</strong> 图标</li>
        <li>选择一个节点</li>
        <li>点击右上角的连接开关</li>
        <li>首次使用需要允许添加 VPN 配置</li>
        <li>连接成功后，状态栏会显示 VPN 图标</li>
        <li>打开 Safari 浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>支持 JavaScript 脚本和重写规则</li>
          <li>功能强大，可高度自定义</li>
          <li>支持 MitM 和网络调试</li>
        </ul>
      </div>
    `
  },
  'Surge': {
    step1: `
      <ol>
        <li>访问官网或 App Store 下载 Surge</li>
        <li>Surge 是付费软件：
          <ul>
            <li>macOS 版本：需要购买许可证</li>
            <li>iOS 版本：App Store 内购（约 $49.99）</li>
          </ul>
        </li>
        <li>安装完成后打开应用</li>
      </ol>
      <div class="tip-box">
        <strong>注意：</strong>Surge 是专业级工具，价格较高，适合高级用户。
      </div>
    `,
    step2: `
      <ol>
        <li>打开 Surge 应用</li>
        <li>点击 <strong>"配置"</strong> 或 <strong>"Profiles"</strong></li>
        <li>选择 <strong>"从 URL 下载配置"</strong></li>
        <li>输入您的订阅链接</li>
        <li>点击 <strong>"下载"</strong></li>
        <li>等待配置下载完成</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>选择刚下载的配置文件</li>
        <li>点击 <strong>"启动"</strong> 或打开开关</li>
        <li>首次使用需要允许添加 VPN 配置（iOS）或安装证书（macOS）</li>
        <li>在策略组中选择节点</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>专业级网络调试工具</li>
          <li>支持 MitM、脚本、模块等高级功能</li>
          <li>性能优秀，稳定性好</li>
        </ul>
      </div>
    `
  },
  'Clash for Windows': {
    step1: `
      <ol>
        <li>点击下载按钮（注意：此软件已停止更新）</li>
        <li>下载 <code>Clash.for.Windows.Setup.xxx.exe</code></li>
        <li>双击安装包进行安装</li>
        <li>安装完成后打开应用</li>
      </ol>
      <div class="tip-box">
        <strong>注意：</strong>Clash for Windows 已停止维护，且不支持 VLESS/Reality 节点；这类节点请使用 Clash Verge Rev。
      </div>
    `,
    step2: `
      <ol>
        <li>打开 Clash for Windows</li>
        <li>点击左侧的 <strong>"Profiles"</strong></li>
        <li>在顶部输入框粘贴您的订阅链接</li>
        <li>点击 <strong>"Download"</strong></li>
        <li>等待配置下载完成；如果提示不支持当前协议，请改用 Clash Verge Rev</li>
        <li>点击配置文件使其生效（会显示绿色）</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>点击左侧的 <strong>"Proxies"</strong></li>
        <li>选择一个节点</li>
        <li>点击左侧的 <strong>"General"</strong></li>
        <li>打开 <strong>"System Proxy"</strong> 开关</li>
        <li>（可选）打开 <strong>"TUN Mode"</strong> 实现全局代理</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>TUN 模式需要安装虚拟网卡驱动</li>
          <li>可以在设置中配置开机自启动</li>
          <li>VLESS/Reality 节点请使用 Clash Verge Rev 或 Mihomo 客户端</li>
        </ul>
      </div>
    `
  },
  'ClashX Pro': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 App Center 页面</li>
        <li>点击页面上的 <strong>"Download"</strong> 下载 dmg 安装包</li>
        <li>双击 dmg，将 ClashX Pro 拖入 <code>Applications</code> 文件夹</li>
        <li>首次启动如被 Gatekeeper 拦截，前往 <strong>"系统设置 → 隐私与安全性"</strong> 点击 <strong>"仍要打开"</strong></li>
      </ol>
    `,
    step2: `
      <ol>
        <li>点击菜单栏 <strong>ClashX Pro 图标</strong></li>
        <li>选择 <strong>"配置" → "远程配置" → "管理"</strong></li>
        <li>点击 <strong>"添加"</strong>，填入订阅链接并保存</li>
        <li>回到菜单选择该配置，点击 <strong>"更新配置"</strong> 或开启 <strong>"自动更新"</strong></li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在菜单中选择 <strong>"出站模式 → 规则"</strong>（或 <strong>"全局"</strong>）</li>
        <li>勾选 <strong>"设置为系统代理"</strong></li>
        <li>需要全局路由时，启用 <strong>"增强模式"</strong>（需授权一次性安装 helper）</li>
        <li>浏览器测试连接；可在 <strong>"代理节点"</strong> 中切换</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>增强模式可代理无法感知系统代理的应用</li>
          <li>VLESS/Reality 节点请使用 Clash Verge Rev 或 Mihomo 客户端</li>
        </ul>
      </div>
    `
  },
  'Clash Meta for Android': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载带 <code>-premium-</code> 或 <code>-meta-</code> 后缀的最新 apk（根据手机 ABI 选择 arm64-v8a / armeabi-v7a）</li>
        <li>手机上打开下载的 apk，按系统提示允许"从此来源安装"</li>
        <li>完成安装后启动 Clash Meta for Android</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 Clash Meta for Android，进入 <strong>"配置"</strong> 页</li>
        <li>点击右下角 <strong>"+"</strong>，选择 <strong>"URL"</strong></li>
        <li>名称自填，链接粘贴您的订阅链接</li>
        <li>保存后点击配置右侧的 <strong>下载按钮</strong> 拉取节点</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>回到主页选中刚才导入的配置</li>
        <li>点击主界面的 <strong>启动开关</strong>，首次启动允许建立 VPN 连接</li>
        <li>进入 <strong>"代理"</strong> 选择节点 / 策略组</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>可在"设置 → 网络 → 访问控制"指定哪些应用走代理。
      </div>
    `
  },
  'Shadowrocket': {
    step1: `
      <ol>
        <li>在 <strong>海外 Apple ID</strong> 的 App Store 搜索 Shadowrocket 下载（¥18-30）</li>
        <li>首次启动时，根据提示允许通知及 VPN 配置</li>
      </ol>
      <div class="tip-box">
        <strong>注意：</strong>国区 App Store 已下架 Shadowrocket，需使用其他区域的 Apple ID 购买。
      </div>
    `,
    step2: `
      <ol>
        <li>打开 Shadowrocket，点击右上角 <strong>"+"</strong></li>
        <li>将订阅链接复制到剪贴板，Shadowrocket 会自动识别；或选择 <strong>"Subscribe"</strong> 手动粘贴</li>
        <li>保存后返回首页，下拉刷新订阅</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在节点列表中选择要使用的节点</li>
        <li>打开主页顶部的连接开关，系统弹出"添加 VPN 配置"请允许</li>
        <li>浏览器测试连接；切换节点只需在节点列表点击其他节点</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>"设置 → 配置"可切换全局 / 规则 / 直连模式</li>
          <li>建议打开"自动连接"，让 Shadowrocket 开机自动启用</li>
        </ul>
      </div>
    `
  },
  'v2rayNG': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载与手机 ABI 对应的 apk（大多为 <code>v2rayNG_x.x.x_arm64-v8a.apk</code>）</li>
        <li>在手机上安装该 apk（允许未知来源安装）</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 v2rayNG，点击右上角 <strong>"≡"</strong></li>
        <li>选择 <strong>"订阅设置" → "+"</strong></li>
        <li>备注随意，URL 填入订阅链接，其他保持默认，保存</li>
        <li>回到主页下拉刷新订阅，等待节点列表出现</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>点击节点行测速，选择延迟较低的节点</li>
        <li>按右下角 <strong>V 型圆形按钮</strong> 连接，允许添加 VPN 配置</li>
        <li>打开浏览器测试；长按节点可设为默认</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>在"设置 → 内核类型"可切换 Xray / v2ray-core；遇到连不上可尝试改用 Xray。
      </div>
    `
  }
}

// 默认教程（用于没有特定教程的客户端）
const defaultTutorial = {
  step1: `
    <ol>
      <li>点击下载按钮，前往官方下载页面</li>
      <li>根据您的系统选择对应的安装包</li>
      <li>下载完成后，按照常规方式安装</li>
      <li>安装完成后打开应用</li>
    </ol>
  `,
  step2: `
    <ol>
      <li>打开客户端应用</li>
      <li>找到 <strong>"订阅"</strong> 或 <strong>"配置"</strong> 相关选项</li>
      <li>添加新的订阅配置</li>
      <li>粘贴您的订阅链接</li>
      <li>保存并更新订阅</li>
    </ol>
    <div class="tip-box">
      不同客户端的界面可能略有差异，但基本流程相似。
    </div>
  `,
  step3: `
    <ol>
      <li>在节点列表中选择一个节点</li>
      <li>启用代理连接</li>
      <li>打开浏览器测试连接是否正常</li>
    </ol>
    <div class="tip-box">
      <strong>常见问题：</strong>
      <ul>
        <li>如果无法连接，尝试切换其他节点</li>
        <li>确保订阅链接正确且未过期</li>
        <li>检查系统防火墙设置</li>
      </ul>
    </div>
  `
}

// 计算属性
const filteredClients = computed(() => {
  return clients.filter(c => c.platform === selectedPlatform.value)
})

const currentPlatform = computed(() => {
  return platforms.find(platform => platform.value === selectedPlatform.value) || null
})

const recommendedClient = computed(() => {
  return filteredClients.value.find(client => client.recommended) || filteredClients.value[0] || null
})

const currentTutorial = computed(() => {
  if (!currentClient.value) return defaultTutorial
  const tutorialName = tutorialAliases[currentClient.value.name] || currentClient.value.name
  return tutorials[tutorialName] || defaultTutorial
})

const subscriptionLink = computed(() =>
  buildFormattedSubscriptionLink(subscriptionStore.link || '', recommendedClient.value?.format)
)

const subscriptionLinkPreview = computed(() => {
  if (!subscriptionLink.value) {
    return ''
  }

  if (subscriptionLink.value.length <= 96) {
    return subscriptionLink.value
  }

  return `${subscriptionLink.value.slice(0, 64)}...${subscriptionLink.value.slice(-24)}`
})

const showSubscriptionUnavailableAlert = computed(() => {
  return !subscriptionLink.value && Boolean(subscriptionUnavailableMessage.value)
})

const tutorialSteps = computed(() => {
  const tutorial = currentTutorial.value
  return [
    { title: '第一步：下载并安装客户端', content: sanitizeHtml(tutorial.step1) },
    { title: '第二步：导入订阅链接', content: sanitizeHtml(tutorial.step2) },
    { title: '第三步：连接并开始使用', content: sanitizeHtml(tutorial.step3) }
  ]
})

// 方法
function detectPlatform() {
  if (typeof navigator === 'undefined') {
    return 'windows'
  }

  const userAgent = (navigator.userAgent || '').toLowerCase()
  const platform = (navigator.userAgentData?.platform || navigator.platform || '').toLowerCase()
  const maxTouchPoints = navigator.maxTouchPoints || 0

  if (userAgent.includes('android')) {
    return 'android'
  }

  if (/iphone|ipad|ipod/.test(userAgent)) {
    return 'ios'
  }

  if (platform.includes('mac') && maxTouchPoints > 1) {
    return 'ios'
  }

  if (platform.includes('win')) {
    return 'windows'
  }

  if (platform.includes('mac')) {
    return 'macos'
  }

  if (platform.includes('linux') || userAgent.includes('linux') || userAgent.includes('x11')) {
    return 'linux'
  }

  return 'windows'
}

function getClientReleaseLabel(client) {
  const url = client?.downloadUrl || ''

  if (url.startsWith('/')) {
    return '本地下载'
  }
  if (url.includes('apps.apple.com')) {
    return 'App Store'
  }
  if (url.includes('github.com')) {
    return 'GitHub Releases'
  }
  if (url.includes('appcenter.ms')) {
    return 'App Center'
  }
  if (url.includes('archive.org')) {
    return '归档镜像'
  }
  return '官网'
}

function getClientBadges(client) {
  const badges = []

  if (client.format && clientFormatLabels[client.format]) {
    badges.push({ label: `订阅：${clientFormatLabels[client.format]}`, type: 'success' })
  }

  if (vlessUnsupportedFormats.has(client.format)) {
    badges.push({ label: '不支持 VLESS', type: 'warning' })
  }

  if (cliClientNames.has(client.name)) {
    badges.push({ label: '命令行', type: 'info' })
  }

  if (archivedClientNames.has(client.name)) {
    badges.push({ label: '已停更', type: 'warning' })
  }

  if (legacyClientNames.has(client.name)) {
    badges.push({ label: '旧版', type: 'warning' })
  }

  if (paidClientNames.has(client.name)) {
    badges.push({ label: '付费', type: 'danger' })
  }

  if (
    client.platform === 'ios' &&
    ['Shadowrocket', 'Quantumult X', 'Loon'].includes(client.name)
  ) {
    badges.push({ label: '需海外 Apple ID', type: 'info' })
  }

  return badges
}

function updateSubscriptionUnavailableMessage(error = null) {
  if (!error) {
    subscriptionUnavailableMessage.value = ''
    return
  }

  if (isNoEntitlementError(error)) {
    subscriptionUnavailableMessage.value = getNoEntitlementMessage('download')
    return
  }

  subscriptionUnavailableMessage.value = '订阅链接暂时无法加载，您可以先安装客户端，稍后再来复制订阅链接。'
}

function openExternal(url, downloadFileName = '') {
  if (typeof window === 'undefined' || !url) {
    return
  }

  // Unified <a>-click approach. The previous code tried window.open() first
  // and used <a> as a "popup blocked" fallback, but window.open() with the
  // 'noopener' window-feature **always** returns null per spec, so the
  // fallback fired on every click — opening two tabs. One link, one click,
  // one new tab. Local download paths use the same element with `download`
  // attribute to trigger save-as instead of navigation.
  const link = document.createElement('a')
  link.href = url
  if (url.startsWith('/') && downloadFileName) {
    link.download = downloadFileName
  } else {
    link.target = '_blank'
    link.rel = 'noopener noreferrer'
  }
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

async function ensureSubscriptionLink() {
  if (subscriptionStore.link) {
    updateSubscriptionUnavailableMessage()
    return buildFormattedSubscriptionLink(subscriptionStore.link, recommendedClient.value?.format)
  }

  try {
    const result = await subscriptionStore.fetchLink()
    updateSubscriptionUnavailableMessage()
    return buildFormattedSubscriptionLink(result?.link || '', recommendedClient.value?.format)
  } catch (error) {
    updateSubscriptionUnavailableMessage(error)
    throw error
  }
}

function buildFormattedSubscriptionLink(link, format) {
  if (!link || !format || format === 'auto') {
    return link || ''
  }

  try {
    const base = typeof window !== 'undefined' ? window.location.origin : undefined
    const url = base ? new URL(link, base) : new URL(link)
    url.searchParams.set('format', format)
    return url.toString()
  } catch {
    const separator = link.includes('?') ? '&' : '?'
    return `${link}${separator}format=${encodeURIComponent(format)}`
  }
}

function downloadClient(client) {
  if (!client) {
    ElMessage.warning('当前没有可用的客户端')
    return
  }

  if (!client.downloadUrl || client.downloadUrl === '#') {
    ElMessage.info('下载链接暂不可用')
    return
  }

  const targetUrl = rewriteIfGithub(client.downloadUrl)
  openExternal(targetUrl, client.downloadFileName)
  if (client.downloadUrl.startsWith('/')) {
    ElMessage.success(`已开始下载 ${client.name}`)
  } else if (targetUrl !== client.downloadUrl) {
    ElMessage.success(`已通过国内镜像打开 ${client.name} 下载页`)
  } else {
    ElMessage.success(`已为您打开 ${client.name} 下载页`)
  }
}

function showTutorial(client) {
  if (!client) {
    ElMessage.warning('当前没有可用的客户端')
    return
  }
  currentClient.value = client
  tutorialStep.value = 0
  tutorialVisible.value = true
}

async function copySubscriptionLink() {
  try {
    const link = await ensureSubscriptionLink()
    if (!link) {
      ElMessage.warning('订阅链接暂未生成，请前往订阅管理查看')
      return
    }

    await copyText(link)
    ElMessage.success('订阅链接已复制')
  } catch (error) {
    updateSubscriptionUnavailableMessage(error)
    if (isNoEntitlementError(error)) {
      ElMessage.warning(getNoEntitlementMessage('download'))
      return
    }

    console.error('复制订阅链接失败:', error)
    ElMessage.error(extractErrorMessage(error) || '复制失败，请前往订阅管理手动复制')
  }
}

function goToSubscription() {
  tutorialVisible.value = false
  router.push('/user/subscription').catch(err => {
    console.error('路由跳转失败:', err)
    ElMessage.error('跳转失败，请稍后重试')
  })
}

function goToPlans() {
  tutorialVisible.value = false
  router.push('/user/plans').catch(err => {
    console.error('跳转到套餐页面失败:', err)
    ElMessage.error('跳转失败，请稍后重试')
  })
}

onMounted(async () => {
  selectedPlatform.value = detectPlatform()

  try {
    await ensureSubscriptionLink()
  } catch (error) {
    updateSubscriptionUnavailableMessage(error)
    if (!isNoEntitlementError(error)) {
      console.warn('下载页预加载订阅链接失败:', error)
    }
  }
})
</script>

<style scoped>
.download-page {
  padding: 20px;
  max-width: 1280px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin: 0;
}

.subscription-alert {
  margin-bottom: 16px;
}

.subscription-alert__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 12px;
}

.quick-start-card {
  margin-bottom: 24px;
  border-radius: 16px;
  border: 1px solid var(--color-border);
  background: linear-gradient(135deg, var(--color-bg-card) 0%, var(--color-border-light) 100%);
}

.quick-start-content {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.quick-start-main {
  flex: 1;
  min-width: 0;
}

.platform-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  background: rgba(64, 158, 255, 0.12);
  color: #409eff;
  font-size: 13px;
  font-weight: 500;
}

.quick-start-title {
  margin: 14px 0 8px;
  font-size: 22px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.quick-start-subtitle {
  margin: 0;
  font-size: 14px;
  line-height: 1.7;
  color: var(--color-text-regular);
}

.quick-start-link {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.quick-start-link__label {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.quick-start-link__value {
  display: block;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-primary);
  word-break: break-all;
}

.quick-start-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
  min-width: 320px;
}

/* 平台选择 */
.platform-tabs {
  margin-bottom: 24px;
  overflow-x: auto;
}

.platform-tabs :deep(.el-radio-group) {
  display: inline-flex;
  min-width: max-content;
}

.platform-tabs :deep(.el-radio-button__inner) {
  display: flex;
  align-items: center;
  gap: 6px;
}

.mirror-toggle {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
  padding: 10px 14px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--color-primary) 6%, var(--color-bg-card));
  border: 1px solid color-mix(in srgb, var(--color-primary) 18%, var(--color-border));
}

.mirror-select {
  width: 220px;
}

/* 客户端网格 */
.clients-section {
  margin-bottom: 24px;
}

.clients-empty {
  display: flex;
  justify-content: center;
  padding: 32px 0;
  background: var(--color-bg-card);
  border: 1px dashed var(--color-border);
  border-radius: 12px;
}

.clients-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
  align-items: stretch;
}

.client-card {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 100%;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 24px;
  box-shadow: var(--shadow-sm);
  transition: all 0.3s;
}

.client-card:hover {
  border-color: var(--color-primary-light);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

/* 客户端头部 */
.client-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.client-logo {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-border-light);
  border-radius: 12px;
  font-size: 24px;
  color: var(--color-text-secondary);
}

.client-logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 12px;
}

.client-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.client-name {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 4px 0;
}

.client-recommend-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(64, 158, 255, 0.12);
  color: #3d7fe3;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
}

.client-version {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.client-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.client-description {
  font-size: 14px;
  color: var(--color-text-regular);
  line-height: 1.6;
  margin: 0 0 16px 0;
}

/* 特性标签 */
.client-features {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 20px;
}

/* 操作按钮 */
.client-actions {
  display: flex;
  margin-top: auto;
  gap: 12px;
}

.client-actions .el-button {
  flex: 1;
}

/* 提示卡片 */
.tips-card {
  border-radius: 16px;
}

.tips-card :deep(.el-card__header) {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tips-card p {
  margin: 8px 0;
  font-size: 14px;
  color: var(--color-text-regular);
  line-height: 1.6;
}

/* 教程对话框 */
.tutorial-dialog :deep(.el-dialog__body) {
  padding: 24px;
}

.tutorial-content {
  min-height: 400px;
}

.tutorial-step-content {
  margin: 32px 0;
  min-height: 300px;
}

.step-panel h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 20px 0;
}

.step-content {
  font-size: 14px;
  color: var(--color-text-regular);
  line-height: 1.8;
}

.step-content :deep(ol) {
  padding-left: 20px;
  margin: 12px 0;
}

.step-content :deep(ol li) {
  margin: 8px 0;
}

.step-content :deep(ul) {
  padding-left: 20px;
  margin: 8px 0;
}

.step-content :deep(ul li) {
  margin: 4px 0;
}

.step-content :deep(code) {
  background: var(--color-border-light);
  padding: 2px 8px;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #e83e8c;
}

.step-content :deep(.tip-box) {
  background: rgba(64, 158, 255, 0.1);
  border-left: 4px solid #409eff;
  padding: 12px 16px;
  margin: 16px 0;
  border-radius: 4px;
}

.step-content :deep(.tip-box strong) {
  color: #409eff;
  display: block;
  margin-bottom: 8px;
}

.step-content :deep(.tip-box ul) {
  margin: 8px 0 0 0;
}

.step-content :deep(.tip-box li) {
  margin: 4px 0;
}

.subscription-inline {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.subscription-inline__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.subscription-inline__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.subscription-inline__value {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-regular);
  word-break: break-all;
}

.tutorial-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--color-border);
}

/* 响应式 */
@media (max-width: 768px) {
  .download-page {
    padding: 16px 16px 96px;
  }

  .quick-start-content {
    flex-direction: column;
  }

  .quick-start-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    min-width: 0;
    width: 100%;
    gap: 10px;
    justify-content: stretch;
  }

  .quick-start-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .clients-grid {
    grid-template-columns: 1fr;
  }

  .client-card {
    padding: 20px;
  }

  .client-actions {
    flex-direction: column;
  }

  .subscription-inline__header {
    flex-direction: column;
    align-items: flex-start;
  }

  .subscription-inline__actions {
    width: 100%;
    justify-content: flex-start;
  }

  .tutorial-step-content {
    min-height: 250px;
  }

  .tutorial-actions {
    flex-direction: column;
  }
}
</style>
