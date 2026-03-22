<template>
  <el-menu
    router
    :default-active="activeMenu"
    class="sidebar-menu"
    :collapse="isCollapse"
  >
    <el-menu-item index="/">
      <el-icon><Odometer /></el-icon>
      <span>仪表盘</span>
    </el-menu-item>
    
    <el-menu-item index="/inbounds">
      <el-icon><Connection /></el-icon>
      <span>入站管理</span>
    </el-menu-item>
    
    <el-menu-item index="/subscription">
      <el-icon><Link /></el-icon>
      <span>我的订阅</span>
    </el-menu-item>
    
    <el-sub-menu
      v-if="isAdmin"
      index="admin"
    >
      <template #title>
        <el-icon><Management /></el-icon>
        <span>管理</span>
      </template>
      <el-menu-item index="/users">
        <el-icon><User /></el-icon>
        <span>用户管理</span>
      </el-menu-item>
      <el-menu-item index="/roles">
        <el-icon><UserFilled /></el-icon>
        <span>角色管理</span>
      </el-menu-item>
      <el-menu-item index="/admin/subscriptions">
        <el-icon><Tickets /></el-icon>
        <span>订阅管理</span>
      </el-menu-item>
    </el-sub-menu>
    
    <el-menu-item index="/settings">
      <el-icon><Setting /></el-icon>
      <span>系统设置</span>
    </el-menu-item>
    
    <el-menu-item index="/logs">
      <el-icon><DocumentCopy /></el-icon>
      <span>系统日志</span>
    </el-menu-item>
  </el-menu>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { 
  Odometer, 
  Setting, 
  DocumentCopy, 
  Connection, 
  Link, 
  Management, 
  User, 
  UserFilled,
  Tickets
} from '@element-plus/icons-vue'

const route = useRoute()
const isCollapse = ref(false)

const activeMenu = computed(() => {
  return route.path
})

const isAdmin = computed(() => {
  const userRole = localStorage.getItem('userRole')
  return userRole === 'admin'
})
</script>

<style scoped>
.sidebar-menu {
  height: 100%;
  border-right: none;
}

.sidebar-menu:not(.el-menu--collapse) {
  width: 200px;
}
</style> 