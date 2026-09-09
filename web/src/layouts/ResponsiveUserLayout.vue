<template>
  <component
    :is="layoutComponent"
  />
</template>

<script setup>
import { markRaw } from 'vue'
import { VIEWPORT_BREAKPOINTS, useViewport } from '@/composables/useViewport'
import MobileLayout from './MobileLayout.vue'
import UserLayout from './UserLayout.vue'

const { viewportWidth } = useViewport()
const desktopLayout = markRaw(UserLayout)
const mobileLayout = markRaw(MobileLayout)

// Pick the navigation shell once for the lifetime of this routed layout.
// Both shells render their own router-view; swapping them on every resize used
// to destroy the active page and silently clear searches and form drafts.
// Page content remains responsive through CSS, while a reload selects the best
// shell for a genuinely changed device/orientation.
const layoutComponent = viewportWidth.value <= VIEWPORT_BREAKPOINTS.portalLayout
  ? mobileLayout
  : desktopLayout
</script>
