<template>
  <div
    class="auth-layout"
    :class="{ 'dark-mode': isDarkMode }"
  >
    <!-- 背景装饰 -->
    <div class="auth-background">
      <div class="bg-shape bg-shape-1" />
      <div class="bg-shape bg-shape-2" />
      <div class="bg-shape bg-shape-3" />
    </div>
    
    <!-- 主内容 -->
    <div class="auth-container">
      <!-- Logo 和标题 -->
      <div class="auth-header">
        <router-link
          to="/user/login"
          class="logo"
        >
          <span class="logo-icon">V</span>
          <span class="logo-text">Panel</span>
        </router-link>
        <p class="auth-subtitle">
          安全、高效的代理服务管理平台
        </p>
      </div>
      
      <!-- 表单区域 -->
      <div class="auth-card">
        <router-view v-slot="{ Component }">
          <transition
            name="slide-fade"
            mode="out-in"
          >
            <component :is="Component" />
          </transition>
        </router-view>
      </div>
      
      <!-- 底部链接 -->
      <div class="auth-footer">
        <div class="footer-links">
          <router-link to="/user/help">
            帮助中心
          </router-link>
          <span class="divider">|</span>
          <router-link to="/user/terms">
            服务条款
          </router-link>
          <span class="divider">|</span>
          <router-link to="/user/privacy">
            隐私政策
          </router-link>
        </div>
        <div class="footer-copyright">
          © {{ currentYear }} V Panel. All rights reserved.
        </div>
      </div>
    </div>
    
    <!-- 主题切换按钮 -->
    <el-button 
      class="theme-toggle" 
      circle 
      @click="toggleTheme"
    >
      <el-icon><Sunny v-if="isDarkMode" /><Moon v-else /></el-icon>
    </el-button>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Sunny, Moon } from '@element-plus/icons-vue'
import { useTheme } from '@/composables/useTheme'

const { isDark, toggleDarkMode } = useTheme()
const isDarkMode = isDark
const currentYear = computed(() => new Date().getFullYear())

const toggleTheme = () => {
  toggleDarkMode()
}
</script>

<style scoped>
.auth-layout {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  overflow: hidden;
}

.auth-layout.dark-mode {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
}

/* 背景装饰 */
.auth-background {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}

.bg-shape {
  position: absolute;
  border-radius: 50%;
  opacity: 0.1;
}

.bg-shape-1 {
  width: 600px;
  height: 600px;
  background: #fff;
  top: -200px;
  right: -100px;
}

.bg-shape-2 {
  width: 400px;
  height: 400px;
  background: #fff;
  bottom: -100px;
  left: -100px;
}

.bg-shape-3 {
  width: 200px;
  height: 200px;
  background: #fff;
  top: 50%;
  left: 10%;
}

/* 主容器 */
.auth-container {
  width: 100%;
  max-width: 420px;
  padding: 20px;
  z-index: 1;
}

/* 头部 */
.auth-header {
  text-align: center;
  margin-bottom: 32px;
}

.logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  margin-bottom: 12px;
}

.logo-icon {
  width: 48px;
  height: 48px;
  background: #fff;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  font-weight: 700;
  color: #667eea;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.dark-mode .logo-icon {
  background: #409eff;
  color: #fff;
}

.logo-text {
  font-size: 28px;
  font-weight: 700;
  color: #fff;
}

.auth-subtitle {
  color: rgba(255, 255, 255, 0.8);
  font-size: 14px;
  margin: 0;
}

/* 卡片 */
.auth-card {
  background: #fff;
  border-radius: 16px;
  padding: 32px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
}

.dark-mode .auth-card {
  background: #1f2937;
}

/* 底部 */
.auth-footer {
  text-align: center;
  margin-top: 24px;
}

.footer-links {
  margin-bottom: 12px;
}

.footer-links a {
  color: rgba(255, 255, 255, 0.8);
  text-decoration: none;
  font-size: 13px;
  transition: color 0.2s;
}

.footer-links a:hover {
  color: #fff;
}

.footer-links .divider {
  color: rgba(255, 255, 255, 0.4);
  margin: 0 12px;
}

.footer-copyright {
  color: rgba(255, 255, 255, 0.6);
  font-size: 12px;
}

/* 主题切换按钮 */
.theme-toggle {
  position: fixed;
  top: 20px;
  right: 20px;
  background: rgba(255, 255, 255, 0.2) !important;
  border: none !important;
  color: #fff !important;
}

.theme-toggle:hover {
  background: rgba(255, 255, 255, 0.3) !important;
}

/* 过渡动画 */
.slide-fade-enter-active {
  transition: all 0.3s ease-out;
}

.slide-fade-leave-active {
  transition: all 0.2s ease-in;
}

.slide-fade-enter-from {
  transform: translateX(20px);
  opacity: 0;
}

.slide-fade-leave-to {
  transform: translateX(-20px);
  opacity: 0;
}

/* 响应式 */
@media (max-width: 480px) {
  .auth-container {
    padding: 16px;
  }
  
  .auth-card {
    padding: 24px;
    border-radius: 12px;
  }
  
  .logo-icon {
    width: 40px;
    height: 40px;
    font-size: 20px;
  }
  
  .logo-text {
    font-size: 24px;
  }
}
</style>
