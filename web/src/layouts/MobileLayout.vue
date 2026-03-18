<template>
  <div class="mobile-layout">
    <!-- 顶部导航栏 -->
    <header class="mobile-header">
      <div class="header-left">
        <el-button 
          v-if="showBackButton" 
          link 
          class="back-btn"
          @click="goBack"
        >
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <span class="header-title">{{ pageTitle }}</span>
      </div>
      <div v-if="showPortalActions" class="header-right">
        <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99">
          <el-button link class="header-btn" @click="goToAnnouncements">
            <el-icon><Bell /></el-icon>
          </el-button>
        </el-badge>
        <el-dropdown trigger="click" placement="bottom-end" @command="handleAction">
          <el-button link class="header-btn" aria-label="更多操作">
            <el-icon><MoreFilled /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="settings">
                <el-icon><Setting /></el-icon>
                个人设置
              </el-dropdown-item>
              <el-dropdown-item command="help">
                <el-icon><QuestionFilled /></el-icon>
                帮助中心
              </el-dropdown-item>
              <el-dropdown-item divided command="logout">
                <el-icon><SwitchButton /></el-icon>
                退出登录
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <!-- 主内容区 -->
    <main class="mobile-main" :class="{ 'with-tabbar': showTabbar }">
      <router-view v-slot="{ Component }">
        <transition name="slide" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>

    <!-- 底部导航栏 -->
    <nav v-if="showTabbar" class="mobile-tabbar">
      <div 
        v-for="item in tabItems" 
        :key="item.path"
        class="tab-item"
        :class="{ active: isActive(item.path) }"
        @click="navigateTo(item.path)"
      >
        <el-icon class="tab-icon">
          <component :is="item.icon" />
        </el-icon>
        <span class="tab-label">{{ item.label }}</span>
      </div>
    </nav>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  ArrowLeft, Bell, MoreFilled, Setting, QuestionFilled,
  SwitchButton, HomeFilled, Connection, Link, Download,
  ChatDotRound, Monitor
} from '@element-plus/icons-vue'
import { usePortalAnnouncementsStore } from '@/stores/portalAnnouncements'
import { useUserPortalStore } from '@/stores/userPortal'

const router = useRouter()
const route = useRoute()
const announcementsStore = usePortalAnnouncementsStore()
const userStore = useUserPortalStore()

// 底部导航项
const tabItems = [
  { path: '/user/dashboard', label: '首页', icon: HomeFilled },
  { path: '/user/nodes', label: '节点', icon: Connection },
  { path: '/user/subscription', label: '订阅', icon: Link },
  { path: '/user/devices', label: '设备', icon: Monitor },
  { path: '/user/download', label: '下载', icon: Download },
  { path: '/user/tickets', label: '工单', icon: ChatDotRound }
]

// 计算属性
const pageTitle = computed(() => {
  return route.meta?.title || 'V Panel'
})

const hasPortalSession = computed(() => {
  route.fullPath
  return Boolean(sessionStorage.getItem('userToken') || localStorage.getItem('userToken'))
})

const showPortalActions = computed(() => hasPortalSession.value)
const showTabbar = computed(() => hasPortalSession.value)

const showBackButton = computed(() => {
  const detailPaths = ['/user/announcements/', '/user/tickets/', '/user/help/']
  const standalonePages = ['/user/terms', '/user/privacy']
  return (
    standalonePages.includes(route.path) ||
    detailPaths.some(p => route.path.startsWith(p) && route.path !== p.slice(0, -1))
  )
})

const unreadCount = computed(() => {
  return announcementsStore.unreadCount
})

// 方法
function isActive(path) {
  return route.path === path || route.path.startsWith(path + '/')
}

function navigateTo(path) {
  router.push(path)
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }

  if (route.path.startsWith('/user/help')) {
    router.push('/user/help')
    return
  }

  router.push(hasPortalSession.value ? '/user/dashboard' : '/user/login')
}

function goToAnnouncements() {
  router.push('/user/announcements')
}

function goToSettings() {
  router.push('/user/settings')
}

function goToHelp() {
  router.push('/user/help')
}

function handleAction(command) {
  switch (command) {
    case 'settings':
      goToSettings()
      break
    case 'help':
      goToHelp()
      break
    case 'logout':
      handleLogout()
      break
  }
}

function handleLogout() {
  ElMessageBox.confirm('确定要退出登录吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    await userStore.logout()
    ElMessage.success('已退出登录')
    router.push('/user/login')
  }).catch(() => {})
}
</script>

<style scoped>
.mobile-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  background: #f5f7fa;
}

/* 顶部导航栏 */
.mobile-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 50px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 12px;
  background: #fff;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.back-btn {
  padding: 8px;
  font-size: 18px;
  color: #303133;
}

.header-title {
  font-size: 17px;
  font-weight: 600;
  color: #303133;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

.header-btn {
  padding: 8px;
  font-size: 20px;
  color: #606266;
}

/* 主内容区 */
.mobile-main {
  flex: 1;
  padding: 50px 0 16px;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.mobile-main.with-tabbar {
  padding-bottom: 60px;
}

/* 底部导航栏 */
.mobile-tabbar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 60px;
  display: flex;
  background: #fff;
  box-shadow: 0 -1px 4px rgba(0, 0, 0, 0.08);
  z-index: 100;
  padding-bottom: env(safe-area-inset-bottom);
}

.tab-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: #909399;
  transition: color 0.3s;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}

.tab-item.active {
  color: #409eff;
}

.tab-icon {
  font-size: 22px;
}

.tab-label {
  font-size: 11px;
}

/* 页面切换动画 */
.slide-enter-active,
.slide-leave-active {
  transition: all 0.2s ease;
}

.slide-enter-from {
  opacity: 0;
  transform: translateX(20px);
}

.slide-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

/* 安全区域适配 */
@supports (padding-bottom: env(safe-area-inset-bottom)) {
  .mobile-tabbar {
    height: calc(60px + env(safe-area-inset-bottom));
  }
  
  .mobile-main {
    padding-bottom: calc(60px + env(safe-area-inset-bottom));
  }
}
</style>
