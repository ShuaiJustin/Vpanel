import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const viewportWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1440)

let listenersBound = false
let detachListeners = null

function bindListeners() {
  if (listenersBound || typeof window === 'undefined') {
    return
  }

  const update = () => {
    viewportWidth.value = window.innerWidth
  }

  window.addEventListener('resize', update, { passive: true })
  window.addEventListener('orientationchange', update, { passive: true })
  update()

  detachListeners = () => {
    window.removeEventListener('resize', update)
    window.removeEventListener('orientationchange', update)
    listenersBound = false
    detachListeners = null
  }

  listenersBound = true
}

export function useViewport(options = {}) {
  const mobileBreakpoint = options.mobileBreakpoint ?? 768
  const tabletBreakpoint = options.tabletBreakpoint ?? 1024

  onMounted(() => {
    bindListeners()
  })

  onBeforeUnmount(() => {
    // Keep the shared listener for the lifetime of the app.
  })

  const isMobile = computed(() => viewportWidth.value <= mobileBreakpoint)
  const isTablet = computed(
    () => viewportWidth.value > mobileBreakpoint && viewportWidth.value <= tabletBreakpoint
  )
  const isDesktop = computed(() => viewportWidth.value > tabletBreakpoint)

  return {
    viewportWidth,
    isMobile,
    isTablet,
    isDesktop
  }
}
