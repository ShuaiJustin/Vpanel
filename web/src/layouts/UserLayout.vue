<template>
  <div class="user-portal">
    <!-- 顶部导航栏 -->
    <header class="user-header">
      <div class="header-container">
        <div class="header-left">
          <router-link
            to="/user/dashboard"
            class="logo"
          >
            <span class="logo-text">V Panel</span>
          </router-link>
          
          <!-- 桌面端导航 -->
          <nav class="desktop-nav">
            <router-link 
              v-for="item in navItems" 
              :key="item.path"
              :to="item.path"
              class="nav-item"
              :class="{ active: isActive(item.path) }"
            >
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
            </router-link>
          </nav>
        </div>
        
        <div class="header-right">
          <!-- 主题切换 -->
          <el-button
            circle
            @click="toggleTheme"
          >
            <el-icon><Sunny v-if="isDarkMode" /><Moon v-else /></el-icon>
          </el-button>

          <template v-if="isAuthenticated">
            <!-- 公告通知 -->
            <el-badge
              :value="unreadCount"
              :hidden="unreadCount === 0"
              class="notification-badge"
            >
              <el-button
                circle
                @click="goToAnnouncements"
              >
                <el-icon><Bell /></el-icon>
              </el-button>
            </el-badge>
            
            <!-- 用户菜单 -->
            <el-dropdown
              trigger="click"
              @command="handleCommand"
            >
              <div class="user-dropdown-trigger">
                <el-avatar
                  :size="32"
                  class="user-avatar"
                >
                  {{ userInitial }}
                </el-avatar>
                <span class="header-username">{{ username }}</span>
                <el-icon class="user-dropdown-arrow"><ArrowDown /></el-icon>
              </div>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="balance">
                    <el-icon><Coin /></el-icon>
                    我的余额
                  </el-dropdown-item>
                  <el-dropdown-item command="settings">
                    <el-icon><Setting /></el-icon>
                    个人设置
                  </el-dropdown-item>
                  <el-dropdown-item command="tickets">
                    <el-icon><ChatDotRound /></el-icon>
                    我的工单
                  </el-dropdown-item>
                  <el-dropdown-item command="devices">
                    <el-icon><Monitor /></el-icon>
                    在线设备
                  </el-dropdown-item>
                  <el-dropdown-item command="help">
                    <el-icon><QuestionFilled /></el-icon>
                    帮助中心
                  </el-dropdown-item>
                  <el-dropdown-item
                    divided
                    command="logout"
                  >
                    <el-icon><SwitchButton /></el-icon>
                    退出登录
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            
            <!-- 移动端菜单按钮 -->
            <el-button
              class="mobile-menu-btn"
              @click="showMobileMenu = true"
            >
              <el-icon><Menu /></el-icon>
            </el-button>
          </template>
          <template v-else>
            <el-button
              class="guest-action-btn"
              @click="router.push('/user/login')"
            >
              登录
            </el-button>
          </template>
        </div>
      </div>
    </header>
    
    <!-- 主内容区 -->
    <main class="user-main">
      <router-view v-slot="{ Component }">
        <transition
          name="fade"
          mode="out-in"
        >
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
    
    <!-- 页脚 -->
    <footer class="user-footer">
      <div class="footer-container">
        <div class="footer-links">
          <router-link to="/user/help">
            帮助中心
          </router-link>
          <router-link to="/user/terms">
            服务条款
          </router-link>
          <router-link to="/user/privacy">
            隐私政策
          </router-link>
          <button
            type="button"
            class="footer-link-button"
            @click="showContact"
          >
            联系我们
          </button>
        </div>
        <div class="footer-copyright">
          © {{ currentYear }} V Panel. All rights reserved.
        </div>
      </div>
    </footer>
    
    <!-- 移动端侧边菜单 -->
    <el-drawer
      v-if="isAuthenticated"
      v-model="showMobileMenu"
      direction="rtl"
      size="280px"
      :show-close="false"
    >
      <template #header>
        <div class="mobile-menu-header">
          <el-avatar
            :size="48"
            class="user-avatar"
          >
            {{ userInitial }}
          </el-avatar>
          <div class="user-info">
            <div class="drawer-username">
              {{ username }}
            </div>
            <div
              class="user-status"
              :class="accountStatus"
            >
              {{ statusText }}
            </div>
          </div>
        </div>
      </template>
      
      <div class="mobile-menu-content">
        <div class="mobile-nav">
          <router-link 
            v-for="item in navItems" 
            :key="item.path"
            :to="item.path"
            class="mobile-nav-item"
            @click="showMobileMenu = false"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </router-link>
        </div>
        
        <el-divider />
        
        <div class="mobile-nav">
          <router-link
            to="/user/balance"
            class="mobile-nav-item"
            @click="showMobileMenu = false"
          >
            <el-icon><Coin /></el-icon>
            <span>我的余额</span>
          </router-link>
          <router-link
            to="/user/settings"
            class="mobile-nav-item"
            @click="showMobileMenu = false"
          >
            <el-icon><Setting /></el-icon>
            <span>个人设置</span>
          </router-link>
          <router-link
            to="/user/tickets"
            class="mobile-nav-item"
            @click="showMobileMenu = false"
          >
            <el-icon><ChatDotRound /></el-icon>
            <span>我的工单</span>
          </router-link>
          <router-link
            to="/user/help"
            class="mobile-nav-item"
            @click="showMobileMenu = false"
          >
            <el-icon><QuestionFilled /></el-icon>
            <span>帮助中心</span>
          </router-link>
        </div>
        
        <el-divider />
        
        <el-button
          type="danger"
          plain
          class="logout-btn"
          @click="handleLogout"
        >
          <el-icon><SwitchButton /></el-icon>
          退出登录
        </el-button>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessageBox, ElMessage } from 'element-plus'
import {
  HomeFilled,
  Connection,
  Link,
  Monitor,
  Download,
  DataAnalysis,
  Bell,
  Coin,
  Setting,
  ArrowDown,
  Menu,
  Sunny,
  Moon,
  ChatDotRound,
  QuestionFilled,
  SwitchButton
} from '@element-plus/icons-vue'
import { useTheme } from '@/composables/useTheme'
import { usePortalAnnouncementsStore } from '@/stores/portalAnnouncements'
import { useUserPortalStore } from '@/stores/userPortal'

const router = useRouter()
const route = useRoute()
const { isDark, toggleDarkMode } = useTheme()
const announcementsStore = usePortalAnnouncementsStore()
const userStore = useUserPortalStore()

// 状态
const showMobileMenu = ref(false)

// 使用共享的主题状态
const isDarkMode = isDark

// 导航项
const portalNavItems = [
  { path: '/user/dashboard', label: '仪表板', icon: HomeFilled },
  { path: '/user/nodes', label: '节点列表', icon: Connection },
  { path: '/user/subscription', label: '订阅管理', icon: Link },
  { path: '/user/devices', label: '在线设备', icon: Monitor },
  { path: '/user/download', label: '客户端下载', icon: Download },
  { path: '/user/stats', label: '使用统计', icon: DataAnalysis }
]
const guestNavItems = [
  { path: '/user/help', label: '帮助中心', icon: QuestionFilled },
  { path: '/user/terms', label: '服务条款', icon: Link },
  { path: '/user/privacy', label: '隐私政策', icon: Setting }
]

// 计算属性
const isAuthenticated = computed(() => userStore.isAuthenticated)
const navItems = computed(() => (isAuthenticated.value ? portalNavItems : guestNavItems))
const username = computed(() => userStore.user?.display_name || userStore.user?.username || '用户')
const accountStatus = computed(() => userStore.user?.status || 'active')
const userInitial = computed(() => username.value.charAt(0).toUpperCase())
const currentYear = computed(() => new Date().getFullYear())
const unreadCount = computed(() => announcementsStore.unreadCount)
const statusText = computed(() => {
  const statusMap = {
    active: '正常',
    expired: '已过期',
    disabled: '已禁用'
  }
  return statusMap[accountStatus.value] || '未知'
})

// 方法
const isActive = (path) => {
  return route.path === path || route.path.startsWith(path + '/')
}

const toggleTheme = () => {
  toggleDarkMode()
}

const goToAnnouncements = () => {
  router.push('/user/announcements')
}

const handleCommand = (command) => {
  switch (command) {
    case 'balance':
      router.push('/user/balance')
      break
    case 'settings':
      router.push('/user/settings')
      break
    case 'devices':
      router.push('/user/devices')
      break
    case 'tickets':
      router.push('/user/tickets')
      break
    case 'help':
      router.push('/user/help')
      break
    case 'logout':
      handleLogout()
      break
  }
}

const handleLogout = () => {
  ElMessageBox.confirm('确定要退出登录吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    await userStore.logout()
    ElMessage.success('已退出登录')
    router.push('/user/login')
  }).catch(() => {})
  showMobileMenu.value = false
}

const showContact = () => {
  router.push(userStore.isAuthenticated ? '/user/tickets/create' : '/user/help')
}

// 初始化
onMounted(() => {
  if (userStore.isAuthenticated) {
    userStore.fetchProfile({ silent: true }).catch(() => {})
    announcementsStore.fetchUnreadCount().catch(() => {})
  }
})
</script>


<style scoped>
.user-portal {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background-color: var(--color-bg-page);
}

/* 顶部导航栏 */
.user-header {
  background-color: var(--color-bg-card);
  box-shadow: var(--shadow-sm);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 var(--vp-page-padding);
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--vp-inline-gap);
}

.header-left {
  display: flex;
  align-items: center;
  gap: clamp(16px, 3vw, 40px);
  min-width: 0;
}

.logo {
  text-decoration: none;
}

.logo-text {
  font-size: 20px;
  font-weight: 600;
  color: #409eff;
}

.desktop-nav {
  display: flex;
  gap: 8px;
  min-width: 0;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 6px;
  text-decoration: none;
  color: var(--color-text-regular);
  font-size: 14px;
  white-space: nowrap;
  transition: all 0.2s;
}

.nav-item:hover {
  background-color: var(--color-border-light);
  color: var(--color-primary);
}

.nav-item.active {
  background-color: rgba(64, 158, 255, 0.1);
  color: var(--color-primary);
}

.header-right {
  display: flex;
  align-items: center;
  gap: var(--vp-inline-gap);
  min-width: 0;
  flex-shrink: 0;
}

.notification-badge {
  margin-right: 4px;
  flex-shrink: 0;
}

.user-dropdown-trigger {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  padding: 4px 12px 4px 8px;
  border-radius: 6px;
  transition: background-color 0.2s;
  min-width: 0;
  max-width: min(220px, 22vw);
  flex-shrink: 0;
  white-space: nowrap;
}

.user-dropdown-trigger:hover {
  background-color: var(--color-border-light);
}

.user-avatar {
  background-color: var(--color-primary) !important;
  color: #fff !important;
  flex-shrink: 0;
}

.header-username {
  font-size: 14px;
  color: var(--color-text-primary);
  font-weight: 500;
  min-width: 0;
  max-width: min(140px, 14vw);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-dropdown-arrow {
  flex-shrink: 0;
  color: var(--color-text-secondary);
}

.mobile-menu-btn {
  display: none;
}

/* 主内容区 */
.user-main {
  flex: 1;
  max-width: 1400px;
  width: 100%;
  margin: 0 auto;
  padding: var(--vp-page-padding);
}

/* 页脚 */
.user-footer {
  background-color: var(--color-bg-card);
  border-top: 1px solid var(--color-border);
  padding: 20px 0;
}

.footer-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 var(--vp-page-padding);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.footer-links {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}

.footer-links a,
.footer-link-button {
  color: var(--color-text-secondary);
  text-decoration: none;
  font-size: 14px;
  transition: color 0.2s;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
  font: inherit;
}

.footer-links a:hover,
.footer-link-button:hover {
  color: var(--color-primary);
}

.footer-copyright {
  color: var(--color-text-secondary);
  font-size: 14px;
}

/* 移动端菜单 */
.mobile-menu-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  background-color: var(--color-border-light);
  margin: -20px -20px 0;
}

.drawer-username {
  font-size: 16px;
  font-weight: 500;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-info .user-status {
  font-size: 12px;
  margin-top: 4px;
}

.user-status.active {
  color: var(--color-success);
}

.user-status.expired {
  color: var(--color-danger);
}

.user-status.disabled {
  color: var(--color-info);
}

.mobile-menu-content {
  padding: 16px 0;
}

.mobile-nav {
  display: flex;
  flex-direction: column;
}

.mobile-nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  text-decoration: none;
  color: var(--color-text-primary);
  font-size: 15px;
  transition: background-color 0.2s;
}

.mobile-nav-item:hover {
  background-color: var(--color-border-light);
}

.logout-btn {
  width: 100%;
  margin-top: 16px;
}

/* 过渡动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 响应式 */
@media (max-width: 1280px) {
  .header-container {
    height: auto;
    min-height: 60px;
    padding-top: 10px;
    padding-bottom: 10px;
  }

  .header-left {
    gap: 16px;
    flex: 1;
  }

  .desktop-nav {
    flex: 1;
    overflow-x: auto;
    overflow-y: hidden;
    padding-bottom: 2px;
    scrollbar-width: thin;
  }

  .desktop-nav::-webkit-scrollbar {
    height: 3px;
  }

  .desktop-nav::-webkit-scrollbar-thumb {
    background: var(--el-border-color-lighter);
    border-radius: 3px;
  }
}

@media (max-width: 768px) {
  .header-container {
    height: 56px;
    padding: 0 12px;
    gap: 12px;
  }

  .header-left {
    gap: 0;
  }

  .desktop-nav {
    display: none;
  }
  
  .mobile-menu-btn {
    display: flex;
  }

  .user-dropdown-trigger {
    display: none;
  }

  .header-right {
    gap: 8px;
  }
  
  .footer-container {
    flex-direction: column;
    gap: 12px;
    text-align: center;
    padding: 0 12px;
  }

  .footer-links {
    justify-content: center;
    gap: 10px 18px;
  }
  
  .user-main {
    padding: 16px 12px;
  }
}

@media (max-width: 480px) {
  .header-container {
    padding: 8px 12px;
    height: auto;
    min-height: 56px;
  }

  .logo-text {
    font-size: 18px;
  }

  .footer-links {
    flex-direction: column;
    gap: 10px;
  }
}
</style>
