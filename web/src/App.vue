<template>
  <router-view />
</template>

<script setup>
import { useTheme } from '@/composables/useTheme'

// 初始化主题
useTheme()
</script>

<style>
html, body {
  margin: 0;
  padding: 0;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}

#app {
  height: 100%;
}

/* 滚动条样式 */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

::-webkit-scrollbar-track {
  background: var(--color-border-light, #f1f1f1);
  border-radius: 3px;
}

::-webkit-scrollbar-thumb {
  background: var(--color-border-dark, #c1c1c1);
  border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
  background: var(--color-border, #a8a8a8);
}
</style> 