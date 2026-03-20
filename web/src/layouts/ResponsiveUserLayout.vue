<template>
  <component :is="layoutComponent" :key="layoutKey" />
</template>

<script setup>
import { computed, markRaw } from 'vue'
import { useViewport } from '@/composables/useViewport'
import MobileLayout from './MobileLayout.vue'
import UserLayout from './UserLayout.vue'

const { isMobile } = useViewport({ mobileBreakpoint: 1080, tabletBreakpoint: 1280 })

const desktopLayout = markRaw(UserLayout)
const mobileLayout = markRaw(MobileLayout)

const layoutComponent = computed(() => (isMobile.value ? mobileLayout : desktopLayout))
const layoutKey = computed(() => (isMobile.value ? 'mobile' : 'desktop'))
</script>
