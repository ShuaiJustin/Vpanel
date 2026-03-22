<template>
  <el-container class="layout-container">
    <el-aside
      :width="isCollapse ? '64px' : '200px'"
      class="aside"
    >
      <div class="logo">
        <img
          src="@/assets/logo.png"
          alt="Logo"
        >
        <span v-show="!isCollapse">V Panel</span>
      </div>
      <el-menu
        :default-active="activeMenu"
        class="menu"
        :collapse="isCollapse"
        :default-openeds="openedMenus"
        router
      >
        <el-menu-item index="/">
          <el-icon><Monitor /></el-icon>
          <template #title>
            仪表盘
          </template>
        </el-menu-item>
        <el-menu-item index="/users">
          <el-icon><User /></el-icon>
          <template #title>
            用户管理
          </template>
        </el-menu-item>
        <el-menu-item index="/proxies">
          <el-icon><Connection /></el-icon>
          <template #title>
            代理管理
          </template>
        </el-menu-item>
        <el-menu-item index="/clients">
          <el-icon><Avatar /></el-icon>
          <template #title>
            客户端管理
          </template>
        </el-menu-item>
        <el-menu-item index="/certificates">
          <el-icon><Lock /></el-icon>
          <template #title>
            证书管理
          </template>
        </el-menu-item>
        <el-sub-menu index="monitor-submenu">
          <template #title>
            <el-icon><DataAnalysis /></el-icon>
            <span>系统监控</span>
          </template>
          <el-menu-item index="/traffic">
            <el-icon><Histogram /></el-icon>
            <template #title>
              流量监控
            </template>
          </el-menu-item>
          <el-menu-item index="/monitor">
            <el-icon><Monitor /></el-icon>
            <template #title>
              资源监控
            </template>
          </el-menu-item>
          <el-menu-item index="/logs">
            <el-icon><Document /></el-icon>
            <template #title>
              系统日志
            </template>
          </el-menu-item>
          <el-menu-item index="/alerts">
            <el-icon><Bell /></el-icon>
            <template #title>
              告警设置
            </template>
          </el-menu-item>
        </el-sub-menu>
        <el-menu-item index="/backups">
          <el-icon><Files /></el-icon>
          <template #title>
            备份恢复
          </template>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <template #title>
            系统设置
          </template>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-button
            type="text"
            class="collapse-btn"
            @click="toggleCollapse"
          >
            <el-icon>
              <component :is="isCollapse ? Expand : Fold" />
            </el-icon>
          </el-button>
        </div>
        <div class="header-right">
          <el-dropdown @command="handleCommand">
            <span class="user-dropdown">
              {{ userStore.username }}
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">
                  个人信息
                </el-dropdown-item>
                <el-dropdown-item command="logout">
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useUserStore } from '@/stores/user'
import {
  Monitor,
  User,
  Connection,
  Lock,
  Setting,
  Expand,
  Fold,
  ArrowDown,
  Avatar,
  DataAnalysis,
  Histogram,
  Document,
  Bell,
  Files
} from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const isCollapse = ref(false)
const activeMenu = computed(() => {
  // 获取当前路径
  const path = route.path
  
  // 根据当前路径判断是否为系统监控子菜单项
  if (['/traffic', '/monitor', '/logs', '/alerts'].includes(path)) {
    // 当访问系统监控下的子页面时，高亮显示对应菜单项，同时保持子菜单展开
    return path
  }
  
  return path
})

// 默认展开的子菜单，确保即使页面刷新，当前选中的子菜单也能保持展开状态
const openedMenus = ref(['monitor-submenu'])

const toggleCollapse = () => {
  isCollapse.value = !isCollapse.value
}

const handleCommand = async (command) => {
  if (command === 'profile') {
    router.push('/profile')
  } else if (command === 'logout') {
    await userStore.logout()
    router.push('/user/login')
  }
}
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.aside {
  background-color: #304156;
  transition: width 0.3s;
  overflow: hidden;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  color: #fff;
  font-size: 18px;
  font-weight: bold;
}

.logo img {
  width: 32px;
  height: 32px;
  margin-right: 12px;
}

.menu {
  border-right: none;
}

.header {
  background-color: #fff;
  border-bottom: 1px solid #dcdfe6;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}

.collapse-btn {
  font-size: 20px;
  cursor: pointer;
}

.user-dropdown {
  display: flex;
  align-items: center;
  cursor: pointer;
  color: #606266;
}

.user-dropdown .el-icon {
  margin-left: 4px;
}

.el-main {
  background-color: #f0f2f5;
  padding: 20px;
}
</style> 
