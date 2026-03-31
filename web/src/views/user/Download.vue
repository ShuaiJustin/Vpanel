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

    <!-- 客户端列表 -->
    <div class="clients-section">
      <div class="clients-grid">
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
const legacyClientNames = new Set(['Clash for Windows', 'Qv2ray', 'SagerNet'])

// 状态
const selectedPlatform = ref('windows')
const activeTip = ref(['1'])
const tutorialVisible = ref(false)
const tutorialStep = ref(0)
const currentClient = ref(null)
const subscriptionUnavailableMessage = ref('')

// 平台列表
const platforms = [
  { value: 'windows', label: 'Windows', icon: Monitor },
  { value: 'macos', label: 'macOS', icon: Apple },
  { value: 'linux', label: 'Linux', icon: Monitor },
  { value: 'ios', label: 'iOS', icon: Iphone },
  { value: 'android', label: 'Android', icon: Iphone }
]

// 客户端列表
const clients = [
  // Windows
  {
    name: 'Clash Verge',
    platform: 'windows',
    version: 'v1.4.0',
    description: '基于 Clash Meta 的现代化代理客户端，界面美观，功能强大。',
    features: ['Clash 规则', '自动更新', '系统代理'],
    recommended: true,
    downloadUrl: 'https://github.com/clash-verge-rev/clash-verge-rev/releases',
    tutorialUrl: '#'
  },
  {
    name: 'v2rayN',
    platform: 'windows',
    version: 'v6.0',
    description: '功能全面的 V2Ray 客户端，支持多种协议。',
    features: ['多协议', '路由规则', '订阅管理'],
    recommended: false,
    downloadUrl: 'https://github.com/2dust/v2rayN/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Clash for Windows',
    platform: 'windows',
    version: 'v0.20.39',
    description: '经典的 Clash 客户端，稳定可靠（已停止更新）。',
    features: ['Clash 规则', 'TUN 模式', '配置管理'],
    recommended: false,
    downloadUrl: 'https://archive.org/download/clash_for_windows_pkg',
    tutorialUrl: '#'
  },
  // macOS
  {
    name: 'ClashX Pro',
    platform: 'macos',
    version: 'v1.118.0',
    description: 'macOS 上最受欢迎的 Clash 客户端，支持增强模式。',
    features: ['增强模式', '菜单栏', '自动更新'],
    recommended: true,
    downloadUrl: 'https://install.appcenter.ms/users/clashx/apps/clashx-pro/distribution_groups/public',
    tutorialUrl: '#'
  },
  {
    name: 'Clash Verge',
    platform: 'macos',
    version: 'v1.4.0',
    description: '跨平台的现代化 Clash 客户端。',
    features: ['Clash Meta', '美观界面', '跨平台'],
    recommended: false,
    downloadUrl: 'https://github.com/clash-verge-rev/clash-verge-rev/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Surge',
    platform: 'macos',
    version: 'v5',
    description: '专业级网络调试工具，功能强大（付费软件）。',
    features: ['专业级', '网络调试', 'MitM'],
    recommended: false,
    downloadUrl: 'https://nssurge.com/',
    tutorialUrl: '#'
  },
  {
    name: 'V2RayXS',
    platform: 'macos',
    version: 'v1.0',
    description: '简洁的 V2Ray 客户端，轻量级。',
    features: ['轻量级', '简洁', '开源'],
    recommended: false,
    downloadUrl: 'https://github.com/tzmax/V2RayXS/releases',
    tutorialUrl: '#'
  },
  // Linux
  {
    name: 'Clash Verge',
    platform: 'linux',
    version: 'v1.4.0',
    description: '支持 Linux 的现代化 Clash 客户端。',
    features: ['Clash Meta', 'AppImage', 'deb/rpm'],
    recommended: true,
    downloadUrl: 'https://github.com/clash-verge-rev/clash-verge-rev/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Clash Meta',
    platform: 'linux',
    version: 'v1.16.0',
    description: '命令行版本的 Clash，适合服务器使用。',
    features: ['命令行', '轻量级', '服务器'],
    recommended: false,
    downloadUrl: 'https://github.com/MetaCubeX/mihomo/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Qv2ray',
    platform: 'linux',
    version: 'v2.7.0',
    description: '跨平台的 V2Ray 图形客户端。',
    features: ['图形界面', '插件系统', '跨平台'],
    recommended: false,
    downloadUrl: 'https://github.com/Qv2ray/Qv2ray/releases',
    tutorialUrl: '#'
  },
  // iOS
  {
    name: 'Shadowrocket',
    platform: 'ios',
    version: 'v2.2.40',
    description: 'iOS 上最流行的代理客户端，功能全面。',
    features: ['多协议', '规则分流', '按需连接'],
    recommended: true,
    downloadUrl: 'https://apps.apple.com/app/shadowrocket/id932747118',
    tutorialUrl: '#'
  },
  {
    name: 'Quantumult X',
    platform: 'ios',
    version: 'v1.4.0',
    description: '功能强大的网络工具，支持复杂规则。',
    features: ['脚本支持', '规则分流', 'MitM'],
    recommended: false,
    downloadUrl: 'https://apps.apple.com/app/quantumult-x/id1443988620',
    tutorialUrl: '#'
  },
  {
    name: 'Surge',
    platform: 'ios',
    version: 'v5',
    description: '专业级网络调试工具（付费软件）。',
    features: ['专业级', '网络调试', 'MitM'],
    recommended: false,
    downloadUrl: 'https://apps.apple.com/app/surge-5/id1442620678',
    tutorialUrl: '#'
  },
  {
    name: 'Loon',
    platform: 'ios',
    version: 'v3.0',
    description: '功能强大的代理工具，支持脚本。',
    features: ['脚本支持', '规则分流', '插件系统'],
    recommended: false,
    downloadUrl: 'https://apps.apple.com/app/loon/id1373567447',
    tutorialUrl: '#'
  },
  // Android
  {
    name: 'Clash Meta for Android',
    platform: 'android',
    version: 'v2.9.0',
    description: 'Android 上的 Clash Meta 客户端。',
    features: ['Clash Meta', '规则分流', '自动更新'],
    recommended: true,
    downloadUrl: 'https://github.com/MetaCubeX/ClashMetaForAndroid/releases',
    tutorialUrl: '#'
  },
  {
    name: 'v2rayNG',
    platform: 'android',
    version: 'v1.8.0',
    description: 'Android 上的 V2Ray 客户端。',
    features: ['多协议', '轻量级', '订阅管理'],
    recommended: false,
    downloadUrl: 'https://github.com/2dust/v2rayNG/releases',
    tutorialUrl: '#'
  },
  {
    name: 'Surfboard',
    platform: 'android',
    version: 'v2.22.0',
    description: '支持 Surge 配置的 Android 客户端。',
    features: ['Surge 配置', '规则分流', '美观界面'],
    recommended: false,
    downloadUrl: 'https://github.com/getsurfboard/surfboard/releases',
    tutorialUrl: '#'
  },
  {
    name: 'SagerNet',
    platform: 'android',
    version: 'v0.10.0',
    description: '基于 sing-box 的通用代理工具箱。',
    features: ['多协议', '插件系统', '开源'],
    recommended: false,
    downloadUrl: 'https://github.com/SagerNet/SagerNet/releases',
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
        <li>下载 <code>v2rayN-With-Core.zip</code>（包含核心文件）</li>
        <li>解压到任意目录（建议：<code>C:\\Program Files\\v2rayN</code>）</li>
        <li>运行 <code>v2rayN.exe</code></li>
        <li>首次运行会在系统托盘显示图标</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>右键点击系统托盘的 v2rayN 图标</li>
        <li>选择 <strong>"订阅分组" → "订阅分组设置"</strong></li>
        <li>点击 <strong>"添加"</strong> 按钮</li>
        <li>填写信息：
          <ul>
            <li>别名：随意填写</li>
            <li>可选地址（url）：粘贴您的订阅链接</li>
          </ul>
        </li>
        <li>点击 <strong>"确定"</strong> 保存</li>
        <li>右键托盘图标，选择 <strong>"订阅分组" → "更新全部订阅"</strong></li>
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
  'ClashX Pro': {
    step1: `
      <ol>
        <li>点击下载按钮，下载 <code>ClashX Pro.dmg</code></li>
        <li>打开 dmg 文件，将 ClashX Pro 拖到 Applications 文件夹</li>
        <li>首次打开时，可能需要在 <strong>"系统偏好设置" → "安全性与隐私"</strong> 中允许运行</li>
        <li>运行后会在菜单栏显示图标</li>
        <li>按提示安装 Helper（需要输入系统密码）</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>点击菜单栏的 ClashX Pro 图标</li>
        <li>选择 <strong>"配置" → "托管配置" → "管理"</strong></li>
        <li>点击 <strong>"添加"</strong> 按钮</li>
        <li>填写信息：
          <ul>
            <li>Url：粘贴您的订阅链接</li>
            <li>Config Name：随意填写</li>
          </ul>
        </li>
        <li>点击 <strong>"确定"</strong>，等待配置下载完成</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>点击菜单栏图标，选择 <strong>"设置为系统代理"</strong></li>
        <li>在 <strong>"Proxy"</strong> 菜单中选择一个节点</li>
        <li>（推荐）选择 <strong>"增强模式"</strong> 以实现更好的代理效果</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>增强模式需要安装 TUN 驱动，按提示操作即可</li>
          <li>可以设置开机自启动和自动更新订阅</li>
          <li>支持规则模式、全局模式和直连模式切换</li>
        </ul>
      </div>
    `
  },
  'Shadowrocket': {
    step1: `
      <ol>
        <li>在 App Store 搜索 <strong>"Shadowrocket"</strong></li>
        <li>购买并下载（需要非中国区 Apple ID，价格约 $2.99）</li>
        <li>安装完成后打开应用</li>
      </ol>
      <div class="tip-box">
        <strong>注意：</strong>中国区 App Store 已下架此应用，需要使用美区或其他地区账号购买。
      </div>
    `,
    step2: `
      <ol>
        <li>打开 Shadowrocket 应用</li>
        <li>点击右上角的 <strong>"+"</strong> 按钮</li>
        <li>选择 <strong>"类型" → "Subscribe"</strong></li>
        <li>在 <strong>"URL"</strong> 栏粘贴您的订阅链接</li>
        <li>点击右上角 <strong>"完成"</strong></li>
        <li>等待订阅更新完成，您将看到所有节点</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在节点列表中，点击选择一个节点（会显示黄点）</li>
        <li>点击顶部的连接开关</li>
        <li>首次使用需要允许添加 VPN 配置，点击 <strong>"Allow"</strong></li>
        <li>输入设备密码或使用 Face ID 确认</li>
        <li>连接成功后，状态栏会显示 VPN 图标</li>
        <li>打开 Safari 浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>可以使用 <strong>"连通性测试"</strong> 功能测试节点延迟</li>
          <li>支持按需连接和自动代理规则</li>
          <li>建议启用 <strong>"订阅" → "自动更新"</strong></li>
        </ul>
      </div>
    `
  },
  'Clash Meta for Android': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载 <code>cmfa-xxx-meta-universal-release.apk</code></li>
        <li>在手机上打开下载的 APK 文件</li>
        <li>允许安装未知来源应用（如有提示）</li>
        <li>安装完成后打开应用</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 Clash Meta 应用</li>
        <li>点击顶部的 <strong>"配置"</strong> 标签</li>
        <li>点击右上角的 <strong>"+"</strong> 按钮</li>
        <li>选择 <strong>"URL"</strong></li>
        <li>填写信息：
          <ul>
            <li>名称：随意填写</li>
            <li>URL：粘贴您的订阅链接</li>
            <li>自动更新：建议开启</li>
          </ul>
        </li>
        <li>点击 <strong>"保存"</strong>，等待配置下载完成</li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在配置列表中，点击刚添加的配置使其生效</li>
        <li>切换到 <strong>"代理"</strong> 标签</li>
        <li>选择一个节点</li>
        <li>返回 <strong>"主页"</strong>，点击中间的开关按钮</li>
        <li>首次使用需要允许创建 VPN 连接</li>
        <li>连接成功后，状态栏会显示钥匙图标</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>可以在设置中启用 <strong>"开机自启动"</strong></li>
          <li>支持规则分流和全局代理模式切换</li>
          <li>可以使用延迟测试功能选择最快节点</li>
        </ul>
      </div>
    `
  },
  'Clash Meta': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载对应系统的版本：
          <ul>
            <li>Linux: <code>mihomo-linux-amd64-xxx.gz</code></li>
            <li>其他系统请选择对应架构的文件</li>
          </ul>
        </li>
        <li>解压文件：<code>gunzip mihomo-linux-amd64-xxx.gz</code></li>
        <li>添加执行权限：<code>chmod +x mihomo-linux-amd64-xxx</code></li>
        <li>移动到系统路径：<code>sudo mv mihomo-linux-amd64-xxx /usr/local/bin/mihomo</code></li>
      </ol>
    `,
    step2: `
      <ol>
        <li>创建配置目录：<code>mkdir -p ~/.config/mihomo</code></li>
        <li>下载订阅配置到本地：
          <pre><code>wget -O ~/.config/mihomo/config.yaml "您的订阅链接"</code></pre>
        </li>
        <li>或者手动创建配置文件，将订阅内容保存到 <code>~/.config/mihomo/config.yaml</code></li>
        <li>验证配置文件格式正确</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>订阅链接需要用引号包裹，避免特殊字符导致命令错误。
      </div>
    `,
    step3: `
      <ol>
        <li>启动 Clash Meta：<code>mihomo -d ~/.config/mihomo</code></li>
        <li>默认监听端口：
          <ul>
            <li>HTTP 代理：7890</li>
            <li>SOCKS5 代理：7891</li>
            <li>控制面板：9090</li>
          </ul>
        </li>
        <li>配置系统代理（可选）：
          <pre><code>export http_proxy=http://127.0.0.1:7890
export https_proxy=http://127.0.0.1:7890</code></pre>
        </li>
        <li>测试连接：<code>curl -I https://www.google.com</code></li>
        <li>后台运行：<code>nohup mihomo -d ~/.config/mihomo &gt; /dev/null 2&gt;&1 &</code></li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>可以访问 <code>http://127.0.0.1:9090/ui</code> 使用 Web 控制面板</li>
          <li>建议使用 systemd 服务管理，实现开机自启</li>
          <li>定期更新订阅：重新下载配置文件并重启服务</li>
        </ul>
      </div>
    `
  },
  'v2rayNG': {
    step1: `
      <ol>
        <li>点击下载按钮，前往 GitHub Releases 页面</li>
        <li>下载对应架构的 APK 文件：
          <ul>
            <li>arm64-v8a（推荐，适用于大多数新手机）</li>
            <li>armeabi-v7a（适用于较老的手机）</li>
            <li>universal（通用版本，体积较大）</li>
          </ul>
        </li>
        <li>在手机上打开下载的 APK 文件</li>
        <li>允许安装未知来源应用（如有提示）</li>
        <li>安装完成后打开应用</li>
      </ol>
    `,
    step2: `
      <ol>
        <li>打开 v2rayNG 应用</li>
        <li>点击左上角的 <strong>"☰"</strong> 菜单图标</li>
        <li>选择 <strong>"订阅分组设置"</strong></li>
        <li>点击右上角的 <strong>"+"</strong> 按钮</li>
        <li>填写信息：
          <ul>
            <li>备注：随意填写</li>
            <li>URL：粘贴您的订阅链接</li>
            <li>自动更新：建议开启</li>
          </ul>
        </li>
        <li>点击 <strong>"确定"</strong> 保存</li>
        <li>返回主界面，点击右上角的 <strong>"⋮"</strong> 菜单</li>
        <li>选择 <strong>"更新订阅"</strong></li>
      </ol>
    `,
    step3: `
      <ol>
        <li>在服务器列表中，点击选择一个节点</li>
        <li>点击右下角的 <strong>"V"</strong> 图标连接</li>
        <li>首次使用需要允许创建 VPN 连接</li>
        <li>连接成功后，状态栏会显示钥匙图标</li>
        <li>打开浏览器测试连接</li>
      </ol>
      <div class="tip-box">
        <strong>提示：</strong>
        <ul>
          <li>可以长按节点进行延迟测试</li>
          <li>支持路由规则设置，实现智能分流</li>
          <li>建议在设置中启用 <strong>"开机自启动"</strong></li>
        </ul>
      </div>
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
        <strong>注意：</strong>Clash for Windows 已停止维护，建议使用 Clash Verge 替代。
      </div>
    `,
    step2: `
      <ol>
        <li>打开 Clash for Windows</li>
        <li>点击左侧的 <strong>"Profiles"</strong></li>
        <li>在顶部输入框粘贴您的订阅链接</li>
        <li>点击 <strong>"Download"</strong></li>
        <li>等待配置下载完成</li>
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
          <li>支持规则分流和自定义规则</li>
        </ul>
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
  return tutorials[currentClient.value.name] || defaultTutorial
})

const subscriptionLink = computed(() => subscriptionStore.link || '')

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

  if (legacyClientNames.has(client.name)) {
    badges.push({ label: '已停更', type: 'warning' })
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

function openExternal(url) {
  if (typeof window === 'undefined' || !url) {
    return
  }

  const openedWindow = window.open(url, '_blank', 'noopener,noreferrer')
  if (openedWindow) {
    openedWindow.opener = null
    return
  }

  const link = document.createElement('a')
  link.href = url
  link.target = '_blank'
  link.rel = 'noopener noreferrer'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

async function ensureSubscriptionLink() {
  if (subscriptionStore.link) {
    updateSubscriptionUnavailableMessage()
    return subscriptionStore.link
  }

  try {
    const result = await subscriptionStore.fetchLink()
    updateSubscriptionUnavailableMessage()
    return result?.link || ''
  } catch (error) {
    updateSubscriptionUnavailableMessage(error)
    throw error
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

  openExternal(client.downloadUrl)
  ElMessage.success(`已为您打开 ${client.name} 下载页`)
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

/* 客户端网格 */
.clients-section {
  margin-bottom: 24px;
}

.clients-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.client-card {
  position: relative;
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
  gap: 12px;
}

.client-actions .el-button {
  flex: 1;
}

/* 提示卡片 */
.tips-card {
  border-radius: 8px;
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
    padding: 16px;
  }

  .quick-start-content {
    flex-direction: column;
  }

  .quick-start-actions {
    min-width: 0;
    width: 100%;
    justify-content: flex-start;
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
