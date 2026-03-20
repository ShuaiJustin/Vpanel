<template>
  <div class="app-layout">
    <!-- 侧边栏 -->
    <div class="sidebar">
      <div class="logo">
        <h1>Xray管理面板</h1>
      </div>
      <Sidebar />
    </div>
    
    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 顶部导航 -->
      <header class="header">
        <div class="header-left">
          <el-button 
            text 
            :icon="expand ? Fold : Expand" 
            @click="toggleSidebar"
          />
        </div>
        <div class="header-right">
          <el-dropdown trigger="click" @command="handleCommand">
            <span class="user-dropdown">
              {{ username }}
              <el-icon class="el-icon--right"><arrow-down /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">个人资料</el-dropdown-item>
                <el-dropdown-item command="password">修改密码</el-dropdown-item>
                <el-dropdown-item divided command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
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
import { useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { ArrowDown, Expand, Fold } from '@element-plus/icons-vue'
import Sidebar from '@/components/Sidebar.vue'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()
const expand = ref(false)

const username = computed(() => {
  return userStore.user?.username || '管理员'
})

// 切换侧边栏
const toggleSidebar = () => {
  expand.value = !expand.value
}

// 处理下拉菜单命令
const handleCommand = (command) => {
  switch (command) {
    case 'profile':
      // 跳转到个人资料页
      // router.push('/profile')
      break
    case 'password':
      // 跳转到修改密码页
      // router.push('/change-password')
      break
    case 'logout':
      ElMessageBox.confirm(
        '确定要退出登录吗?',
        '提示',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning',
        }
      ).then(async () => {
        await userStore.logout()
        router.replace('/user/login')
      }).catch(() => {})
      break
  }
}
</script>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  background-color: #f5f7fa;
}

.sidebar {
  width: 220px;
  height: 100%;
  background-color: #304156;
  color: #fff;
  overflow-y: auto;
  transition: all 0.3s;
}

.sidebar.collapsed {
  width: 64px;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.logo h1 {
  font-size: 18px;
  font-weight: 600;
  color: #fff;
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.header {
  height: 60px;
  background-color: var(--el-bg-color, #fff);
  border-bottom: 1px solid var(--el-border-color, #e6e6e6);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
}

.header-left, .header-right {
  display: flex;
  align-items: center;
}

.user-dropdown {
  display: flex;
  align-items: center;
  cursor: pointer;
  font-size: 14px;
  color: var(--el-text-color-primary, #333);
}

.content {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
}
</style>
