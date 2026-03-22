<template>
  <component
    :is="layoutComponent"
    :key="layoutKey"
  />
</template>

<script setup>
import { computed, markRaw } from 'vue'
import { VIEWPORT_BREAKPOINTS, useViewport } from '@/composables/useViewport'
import MobileLayout from './MobileLayout.vue'
import UserLayout from './UserLayout.vue'

const { viewportWidth } = useViewport()
const isMobile = computed(() => viewportWidth.value <= VIEWPORT_BREAKPOINTS.portalLayout)

const desktopLayout = markRaw(UserLayout)
const mobileLayout = markRaw(MobileLayout)

const layoutComponent = computed(() => (isMobile.value ? mobileLayout : desktopLayout))
const layoutKey = computed(() => (isMobile.value ? 'mobile' : 'desktop'))
</script>
