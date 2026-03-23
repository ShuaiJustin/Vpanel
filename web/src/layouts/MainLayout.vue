<template>
  <div
    class="app-container"
    :class="{ 'dark-mode': isDark, 'is-mobile': isMobile }"
  >
    <transition name="overlay-fade">
      <button
        v-if="isMobile && isMobileMenuOpen"
        class="sidebar-overlay"
        type="button"
        aria-label="关闭导航菜单"
        @click="closeMobileMenu"
      />
    </transition>
    <!-- 侧边栏 -->
    <div
      class="sidebar"
      :class="{
        collapsed: isCollapse && !isMobile,
        'is-mobile': isMobile,
        'mobile-open': isMobileMenuOpen
      }"
    >
      <div
        class="logo"
        :class="{ collapsed: isCollapse && !isMobile }"
      >
        <span
          v-if="isCollapse && !isMobile"
          class="logo-short"
        >V</span>
        <h1 v-else>
          V 管理面板
        </h1>
      </div>
      <el-menu
        class="sidebar-menu"
        background-color="#17212b"
        text-color="#d6deea"
        active-text-color="#ffffff"
        :default-active="activeMenu"
        :collapse="!isMobile && isCollapse"
        router
      >
        <el-menu-item index="/admin/dashboard">
          <el-icon><Monitor /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>

        <el-sub-menu
          v-if="isAdmin"
          index="nodes"
        >
          <template #title>
            <el-icon><Connection /></el-icon>
            <span>节点管理</span>
          </template>
          <el-menu-item index="/admin/node-dashboard">
            集群概览
          </el-menu-item>
          <el-menu-item index="/admin/nodes">
            节点列表
          </el-menu-item>
          <el-menu-item index="/admin/node-groups">
            节点分组
          </el-menu-item>
          <el-menu-item index="/admin/node-map">
            地理分布
          </el-menu-item>
          <el-menu-item index="/admin/node-comparison">
            性能对比
          </el-menu-item>
        </el-sub-menu>
        
        <el-menu-item index="/admin/inbounds">
          <el-icon><Connection /></el-icon>
          <span>代理服务</span>
        </el-menu-item>
        
        <el-menu-item index="/admin/subscriptions">
          <el-icon><Link /></el-icon>
          <span>订阅管理</span>
        </el-menu-item>
        
        <el-sub-menu index="user">
          <template #title>
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </template>
          <el-menu-item index="/admin/users">
            用户列表
          </el-menu-item>
          <el-menu-item index="/admin/roles">
            角色管理
          </el-menu-item>
        </el-sub-menu>
        
        <el-sub-menu index="monitor">
          <template #title>
            <el-icon><DataAnalysis /></el-icon>
            <span>监控与统计</span>
          </template>
          <el-menu-item index="/admin/system-monitor">
            系统监控
          </el-menu-item>
          <el-menu-item index="/admin/traffic-monitor">
            流量监控
          </el-menu-item>
          <el-menu-item index="/admin/stats">
            统计数据
          </el-menu-item>
        </el-sub-menu>
        
        <el-menu-item index="/admin/certificates">
          <el-icon><Tools /></el-icon>
          <span>证书管理</span>
        </el-menu-item>

        <el-sub-menu
          v-if="isAdmin"
          index="commercial"
        >
          <template #title>
            <el-icon><ShoppingCart /></el-icon>
            <span>商业化管理</span>
          </template>
          <el-menu-item index="/admin/plans">
            套餐管理
          </el-menu-item>
          <el-menu-item index="/admin/orders">
            订单管理
          </el-menu-item>
          <el-menu-item index="/admin/coupons">
            优惠券管理
          </el-menu-item>
          <el-menu-item index="/admin/gift-cards">
            礼品卡管理
          </el-menu-item>
          <el-menu-item index="/admin/trials">
            试用管理
          </el-menu-item>
          <el-menu-item index="/admin/payment-settings">
            支付配置
          </el-menu-item>
          <el-menu-item index="/admin/reports">
            财务报表
          </el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="settings">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </template>
          <el-menu-item index="/admin/settings">
            配置管理
          </el-menu-item>
          <el-menu-item index="/admin/logs">
            日志管理
          </el-menu-item>
          <el-menu-item index="/admin/ip-restriction">
            IP 限制
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </div>
    
    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 顶部栏 -->
      <header class="header">
        <div class="header-left">
          <el-button
            text
            class="collapse-btn"
            @click="toggleSidebar"
          >
            <el-icon>
              <Menu v-if="isMobile" />
              <Fold v-else-if="!isCollapse" />
              <Expand v-else />
            </el-icon>
          </el-button>
          <span
            v-if="isMobile"
            class="header-title"
          >{{ currentTitle }}</span>
        </div>
        <div class="header-right">
          <el-button
            circle
            class="theme-toggle-btn"
            title="切换主题"
            @click="toggleTheme"
          >
            <el-icon><Sunny v-if="isDark" /><Moon v-else /></el-icon>
          </el-button>
          <el-dropdown
            v-if="isMobile"
            trigger="click"
            @command="handleMobileUserCommand"
          >
            <el-button
              circle
              class="mobile-user-menu-btn"
            >
              <el-icon><User /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">
                  个人资料
                </el-dropdown-item>
                <el-dropdown-item command="password">
                  修改密码
                </el-dropdown-item>
                <el-dropdown-item
                  command="logout"
                  divided
                >
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <div
            v-else
            class="user-info"
          >
            <el-avatar
              size="small"
              class="user-avatar"
            >
              {{ username.charAt(0).toUpperCase() }}
            </el-avatar>
            <span class="username">{{ username }}</span>
            <el-button
              link
              size="small"
              class="user-action-btn"
              @click="goToProfile"
            >
              个人资料
            </el-button>
            <el-button
              link
              size="small"
              class="user-action-btn"
              @click="goToChangePassword"
            >
              修改密码
            </el-button>
            <el-button
              link
              size="small"
              class="user-action-btn logout-btn"
              @click="confirmLogout"
            >
              退出登录
            </el-button>
          </div>
        </div>
      </header>
      
      <!-- 内容区域 -->
      <main class="content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import {
  Monitor,
  Connection,
  User,
  DataAnalysis,
  Tools,
  Setting,
  Fold,
  Expand,
  Link,
  Menu,
  ShoppingCart,
  Sunny,
  Moon
} from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { useTheme } from '@/composables/useTheme'
import { VIEWPORT_BREAKPOINTS, useViewport } from '@/composables/useViewport'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const { isDark, toggleDarkMode } = useTheme()
const { viewportWidth } = useViewport()
const isMobile = computed(() => viewportWidth.value <= VIEWPORT_BREAKPOINTS.adminNavigation)

const isCollapse = ref(false)
const isMobileMenuOpen = ref(false)
const username = computed(() => userStore.user?.username || '管理员')
const activeMenu = computed(() => route.path)
const currentTitle = computed(() => route.meta?.title || 'V 管理面板')

function getStoredItem(key) {
  return sessionStorage.getItem(key) || localStorage.getItem(key)
}

// Check if user is admin
const isAdmin = computed(() => {
  const userRole = getStoredItem('userRole')
  return userRole === 'admin'
})

// 切换侧边栏
const toggleSidebar = () => {
  if (isMobile.value) {
    isMobileMenuOpen.value = !isMobileMenuOpen.value
    return
  }

  isCollapse.value = !isCollapse.value
}

const closeMobileMenu = () => {
  isMobileMenuOpen.value = false
}

// 切换主题
const toggleTheme = () => {
  toggleDarkMode()
}

// 导航到个人资料页面
const goToProfile = () => {
  router.push('/admin/profile')
}

// 导航到修改密码页面
const goToChangePassword = () => {
  router.push('/admin/change-password')
}

const handleMobileUserCommand = (command) => {
  if (command === 'profile') {
    goToProfile()
    return
  }

  if (command === 'password') {
    goToChangePassword()
    return
  }

  if (command === 'logout') {
    confirmLogout()
  }
}

// 确认退出登录
const confirmLogout = () => {
  ElMessageBox.confirm('确定要退出登录吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    await userStore.logout()
    closeMobileMenu()
    router.replace({ name: 'UserLogin' })
  }).catch(() => {})
}

watch(
  () => route.fullPath,
  () => {
    if (isMobile.value) {
      closeMobileMenu()
    }
  }
)

watch(isMobile, mobile => {
  if (!mobile) {
    closeMobileMenu()
  }
})
</script>

<style scoped>
.app-container {
  --sidebar-bg: #17212b;
  --sidebar-surface: #1e2a36;
  --sidebar-hover: #243244;
  --sidebar-nested: #141d27;
  --sidebar-text: #d6deea;
  --sidebar-muted: #9fb0c3;
  --sidebar-active: #3b82f6;
  --sidebar-active-shadow: rgba(59, 130, 246, 0.28);
  display: flex;
  width: 100%;
  height: 100vh;
  height: 100dvh;
  min-height: 0;
  overflow: hidden;
}

.sidebar-overlay {
  position: fixed;
  inset: 0;
  border: 0;
  background: rgba(15, 23, 42, 0.55);
  z-index: 1000;
}

.sidebar {
  width: 224px;
  height: 100vh;
  height: 100dvh;
  position: sticky;
  top: 0;
  background: linear-gradient(180deg, var(--sidebar-surface) 0%, var(--sidebar-bg) 100%);
  color: var(--sidebar-text);
  transition: all 0.3s;
  overflow-y: auto;
  overflow-x: hidden;
  flex-shrink: 0;
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.18);
  z-index: 1001;
}

.sidebar.collapsed {
  width: 64px;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: rgba(255, 255, 255, 0.04);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(10px);
}

.logo h1 {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: #f8fafc;
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
}

.logo-short {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--sidebar-active) 0%, #2563eb 100%);
  color: #ffffff;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.04em;
  box-shadow: 0 10px 24px var(--sidebar-active-shadow);
}

.logo.collapsed {
  justify-content: center;
}

.sidebar.collapsed :deep(.el-menu-item),
.sidebar.collapsed :deep(.el-sub-menu__title) {
  margin: 6px 8px;
}

.sidebar-menu {
  border-right: none;
  width: 100%;
  min-width: 0;
  overflow-x: hidden;
  background-color: transparent !important;
}

.sidebar-menu:not(.el-menu--collapse) {
  width: 100%;
}

:deep(.el-menu-item) {
  height: 50px;
  line-height: 50px;
  font-size: 14px;
  color: var(--sidebar-text) !important;
  border-radius: 12px;
  margin: 6px 10px;
  box-sizing: border-box;
  transition: background-color 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

:deep(.el-sub-menu__title) {
  height: 50px;
  line-height: 50px;
  font-size: 14px;
  color: var(--sidebar-text) !important;
  border-radius: 12px;
  margin: 6px 10px;
  box-sizing: border-box;
  transition: background-color 0.2s ease, color 0.2s ease;
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, var(--sidebar-active) 0%, #2563eb 100%) !important;
  color: #ffffff !important;
  box-shadow: 0 10px 24px var(--sidebar-active-shadow);
}

:deep(.el-menu-item:hover) {
  background-color: var(--sidebar-hover) !important;
  color: #ffffff !important;
}

:deep(.el-sub-menu__title:hover) {
  background-color: var(--sidebar-hover) !important;
  color: #ffffff !important;
}

:deep(.el-sub-menu .el-menu-item) {
  min-width: 0;
  width: calc(100% - 28px);
  margin: 4px 14px 4px 14px;
  padding-left: 52px !important;
  box-sizing: border-box;
  background-color: var(--sidebar-nested);
  color: var(--sidebar-muted) !important;
}

:deep(.el-sub-menu.is-opened > .el-sub-menu__title) {
  background-color: rgba(255, 255, 255, 0.04) !important;
  color: #ffffff !important;
}

:deep(.el-menu-item [class^='el-icon']),
:deep(.el-sub-menu__title [class^='el-icon']) {
  color: inherit;
}

.main-content {
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.header {
  height: 60px;
  background-color: #fff;
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--vp-inline-gap);
  padding: 0 var(--vp-page-padding);
  position: sticky;
  top: 0;
  z-index: 900;
}

.header-left, .header-right {
  display: flex;
  align-items: center;
  min-width: 0;
}

.collapse-btn {
  font-size: 20px;
  transition: all 0.3s;
  margin-left: -8px;
}

.collapse-btn:hover {
  color: #409EFF;
}

.header-title {
  font-size: 16px;
  font-weight: 600;
  color: #111827;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* User dropdown styles */
.user-dropdown {
  height: 40px;
}

.dropdown-link {
  display: flex;
  align-items: center;
  cursor: pointer;
  font-size: 14px;
  color: #333;
  height: 40px;
  padding: 0 10px;
  border-radius: 4px;
  transition: background-color 0.3s;
  border: 1px solid transparent;
}

.dropdown-link:hover {
  background-color: #f5f7fa;
  border-color: #e6e6e6;
}

.content {
  flex: 1;
  min-width: 0;
  min-height: 0;
  padding: var(--vp-page-padding);
  overflow: auto;
  background-color: #f0f2f5;
}

:deep(.el-dropdown-menu) {
  min-width: 130px;
  border-radius: 4px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

:deep(.el-dropdown-menu__item) {
  padding: 10px 16px;
  font-size: 14px;
  line-height: 20px;
  display: flex;
  align-items: center;
}

:deep(.el-dropdown-menu__item:not(.is-disabled):hover) {
  background-color: #f5f7fa;
  color: #409EFF;
}

:deep(.el-dropdown-menu__item--divided) {
  margin-top: 6px;
  border-top: 1px solid #ebeef5;
}

:deep(.el-dropdown-menu__item--divided:before) {
  height: 6px;
  margin-top: -6px;
}

:deep(.el-popper) {
  z-index: 9999 !important;
}

.user-info {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 4px;
  height: 40px;
  padding: 0 5px;
  min-width: 0;
}

.theme-toggle-btn {
  margin-right: 15px;
  border: none;
  background-color: transparent;
}

.mobile-user-menu-btn {
  border: none;
}

.theme-toggle-btn:hover {
  background-color: #f5f7fa;
}

.user-avatar {
  background-color: var(--sidebar-active) !important;
  color: white !important;
  font-weight: bold;
}

.username {
  margin: 0 8px;
  font-weight: 500;
  min-width: 0;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-action-btn {
  margin: 0 2px;
  font-size: 12px;
  color: #606266;
}

.user-action-btn:hover {
  color: #409EFF;
}

.logout-btn {
  color: #f56c6c;
}

.logout-btn:hover {
  color: #f56c6c;
  opacity: 0.8;
}

@media (max-width: 1024px) {
  .sidebar {
    position: fixed;
    top: 0;
    left: 0;
    width: min(82vw, 320px);
    height: 100vh;
    height: 100dvh;
    transform: translateX(-100%);
  }

  .sidebar.mobile-open {
    transform: translateX(0);
  }

  .main-content {
    width: 100%;
  }

  .header {
    padding: 0 12px;
  }

  .content {
    padding: 12px;
  }

  .theme-toggle-btn {
    margin-right: 8px;
  }
}

@media (max-width: 1280px) {
  .header {
    padding: 0 14px;
  }

  .username {
    max-width: 96px;
    margin: 0 4px;
  }

  .user-action-btn {
    margin: 0;
    padding: 0 4px;
  }
}

@media (max-width: 640px) {
  .header {
    min-height: 56px;
    height: auto;
    gap: 8px;
  }

  .content {
    padding: 10px;
  }
}

.overlay-fade-enter-active,
.overlay-fade-leave-active {
  transition: opacity 0.2s ease;
}

.overlay-fade-enter-from,
.overlay-fade-leave-to {
  opacity: 0;
}

/* 深色模式样式 */
.app-container.dark-mode {
  --sidebar-bg: #0f172a;
  --sidebar-surface: #162033;
  --sidebar-hover: #1d2a3d;
  --sidebar-nested: #101827;
  --sidebar-text: #dbe6f2;
  --sidebar-muted: #9aaec3;
  --sidebar-active: #60a5fa;
  --sidebar-active-shadow: rgba(96, 165, 250, 0.2);
  background-color: #1a1a1a;
}

.dark-mode .sidebar {
  box-shadow: 0 0 10px rgba(0, 0, 0, 0.5);
}

.dark-mode .logo {
  background-color: rgba(255, 255, 255, 0.03);
  border-bottom-color: rgba(255, 255, 255, 0.08);
}

.dark-mode .logo h1 {
  color: #e5eaf3;
}

.dark-mode .logo-short {
  background: linear-gradient(135deg, var(--sidebar-active) 0%, #3b82f6 100%);
}

.dark-mode .sidebar-menu {
  background-color: transparent;
}

.dark-mode :deep(.el-menu) {
  background-color: transparent;
}

.dark-mode :deep(.el-menu-item) {
  background-color: transparent;
  color: var(--sidebar-text) !important;
}

.dark-mode :deep(.el-menu-item:hover) {
  background-color: var(--sidebar-hover) !important;
  color: #ffffff !important;
}

.dark-mode :deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, var(--sidebar-active) 0%, #3b82f6 100%) !important;
  color: white !important;
}

.dark-mode :deep(.el-sub-menu__title) {
  background-color: transparent;
  color: var(--sidebar-text) !important;
}

.dark-mode :deep(.el-sub-menu__title:hover) {
  background-color: var(--sidebar-hover) !important;
  color: #ffffff !important;
}

.dark-mode :deep(.el-sub-menu .el-menu-item) {
  background-color: var(--sidebar-nested);
  color: var(--sidebar-muted) !important;
}

.dark-mode :deep(.el-sub-menu .el-menu-item:hover) {
  background-color: var(--sidebar-hover) !important;
  color: #ffffff !important;
}

.dark-mode .header {
  background-color: #242424;
  border-bottom-color: #303030;
}

.dark-mode .collapse-btn {
  color: #cfd3dc;
}

.dark-mode .collapse-btn:hover {
  color: #409eff;
}

.dark-mode .dropdown-link {
  color: #e5eaf3;
}

.dark-mode .dropdown-link:hover {
  background-color: #1f1f1f;
  border-color: #303030;
}

.dark-mode .username {
  color: #e5eaf3;
}

.dark-mode .user-action-btn {
  color: #a3a6ad;
}

.dark-mode .user-action-btn:hover {
  color: #409eff;
}

.dark-mode .logout-btn {
  color: #f56c6c;
}

.dark-mode .content {
  background-color: #1a1a1a;
}

.dark-mode .theme-toggle-btn {
  color: #cfd3dc;
}

.dark-mode .theme-toggle-btn:hover {
  background-color: #1f1f1f;
  color: #409eff;
}

.dark-mode :deep(.el-dropdown-menu) {
  background-color: #242424;
  border-color: #303030;
}

.dark-mode :deep(.el-dropdown-menu__item) {
  color: #cfd3dc;
}

.dark-mode :deep(.el-dropdown-menu__item:not(.is-disabled):hover) {
  background-color: #1f1f1f;
  color: #409eff;
}

.dark-mode :deep(.el-dropdown-menu__item--divided) {
  border-top-color: #303030;
}
</style> 
