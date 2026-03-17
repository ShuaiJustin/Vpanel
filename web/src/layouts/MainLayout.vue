<template>
  <div class="app-container" :class="{ 'dark-mode': isDark }">
    <!-- 侧边栏 -->
    <div class="sidebar">
      <div class="logo">
        <h1>V 管理面板</h1>
      </div>
      <el-menu
        class="sidebar-menu"
        background-color="#17212b"
        text-color="#d6deea"
        active-text-color="#ffffff"
        :default-active="activeMenu"
        :collapse="isCollapse"
        router
      >
        <el-menu-item index="/admin/dashboard">
          <el-icon><Monitor /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        
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
          <el-menu-item index="/admin/users">用户列表</el-menu-item>
          <el-menu-item index="/admin/roles">角色管理</el-menu-item>
        </el-sub-menu>
        
        <el-sub-menu index="monitor">
          <template #title>
            <el-icon><DataAnalysis /></el-icon>
            <span>监控与统计</span>
          </template>
          <el-menu-item index="/admin/traffic-monitor">流量监控</el-menu-item>
          <el-menu-item index="/admin/stats">统计数据</el-menu-item>
        </el-sub-menu>
        
        <el-menu-item index="/admin/certificates">
          <el-icon><Tools /></el-icon>
          <span>证书管理</span>
        </el-menu-item>

        <el-sub-menu index="commercial" v-if="isAdmin">
          <template #title>
            <el-icon><ShoppingCart /></el-icon>
            <span>商业化管理</span>
          </template>
          <el-menu-item index="/admin/plans">套餐管理</el-menu-item>
          <el-menu-item index="/admin/orders">订单管理</el-menu-item>
          <el-menu-item index="/admin/coupons">优惠券管理</el-menu-item>
          <el-menu-item index="/admin/gift-cards">礼品卡管理</el-menu-item>
          <el-menu-item index="/admin/trials">试用管理</el-menu-item>
          <el-menu-item index="/admin/payment-settings">支付配置</el-menu-item>
          <el-menu-item index="/admin/reports">财务报表</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="nodes" v-if="isAdmin">
          <template #title>
            <el-icon><Connection /></el-icon>
            <span>节点管理</span>
          </template>
          <el-menu-item index="/admin/node-dashboard">集群概览</el-menu-item>
          <el-menu-item index="/admin/nodes">节点列表</el-menu-item>
          <el-menu-item index="/admin/node-groups">节点分组</el-menu-item>
          <el-menu-item index="/admin/node-map">地理分布</el-menu-item>
          <el-menu-item index="/admin/node-comparison">性能对比</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="settings">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </template>
          <el-menu-item index="/admin/settings">配置管理</el-menu-item>
          <el-menu-item index="/admin/logs">日志管理</el-menu-item>
          <el-menu-item index="/admin/ip-restriction">IP 限制</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </div>
    
    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 顶部栏 -->
      <header class="header">
        <div class="header-left">
          <el-icon
            class="collapse-btn"
            @click="toggleSidebar"
          >
            <Fold v-if="!isCollapse" />
            <Expand v-else />
          </el-icon>
        </div>
        <div class="header-right">
          <el-button circle @click="toggleTheme" class="theme-toggle-btn" title="切换主题">
            <el-icon><Sunny v-if="isDark" /><Moon v-else /></el-icon>
          </el-button>
          <div class="user-info">
            <el-avatar size="small" class="user-avatar">{{ username.charAt(0).toUpperCase() }}</el-avatar>
            <span class="username">{{ username }}</span>
            <el-button link size="small" @click="goToProfile" class="user-action-btn">个人资料</el-button>
            <el-button link size="small" @click="goToChangePassword" class="user-action-btn">修改密码</el-button>
            <el-button link size="small" @click="confirmLogout" class="user-action-btn logout-btn">退出登录</el-button>
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
import { ref, computed } from 'vue'
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
  ShoppingCart,
  Sunny,
  Moon
} from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { useTheme } from '@/composables/useTheme'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const { isDark, toggleDarkMode } = useTheme()

const isCollapse = ref(false)
const username = computed(() => userStore.user?.username || '管理员')
const activeMenu = computed(() => route.path)

// Check if user is admin
const isAdmin = computed(() => {
  const userRole = localStorage.getItem('userRole')
  return userRole === 'admin'
})

// 切换侧边栏
const toggleSidebar = () => {
  isCollapse.value = !isCollapse.value
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

// 确认退出登录
const confirmLogout = () => {
  ElMessageBox.confirm('确定要退出登录吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    userStore.logout()
    router.push('/user/login')
  }).catch(() => {})
}
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
  height: 100vh;
}

.sidebar {
  width: 200px;
  height: 100%;
  background: linear-gradient(180deg, var(--sidebar-surface) 0%, var(--sidebar-bg) 100%);
  color: var(--sidebar-text);
  transition: all 0.3s;
  overflow-y: auto;
  flex-shrink: 0;
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.18);
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

.sidebar-menu {
  border-right: none;
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
  transition: background-color 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

:deep(.el-sub-menu__title) {
  height: 50px;
  line-height: 50px;
  font-size: 14px;
  color: var(--sidebar-text) !important;
  border-radius: 12px;
  margin: 6px 10px;
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
  min-width: 200px;
  padding-left: 40px !important;
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
  padding: 0 20px;
}

.header-left, .header-right {
  display: flex;
  align-items: center;
}

.collapse-btn {
  font-size: 20px;
  cursor: pointer;
  transition: all 0.3s;
}

.collapse-btn:hover {
  color: #409EFF;
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
  padding: 20px;
  overflow-y: auto;
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
  height: 40px;
  padding: 0 5px;
}

.theme-toggle-btn {
  margin-right: 15px;
  border: none;
  background-color: transparent;
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
