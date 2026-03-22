import { computed, onMounted, ref } from 'vue'

export const VIEWPORT_BREAKPOINTS = Object.freeze({
  mobile: 768,
  tablet: 1280,
  adminNavigation: 1024,
  portalLayout: 1080,
  designWidth: 1440,
  minScale: 0.9,
  maxScale: 1.08
})

export function calculateViewportScale(width) {
  const safeWidth = Number.isFinite(width) && width > 0 ? width : VIEWPORT_BREAKPOINTS.designWidth
  const rawScale = safeWidth / VIEWPORT_BREAKPOINTS.designWidth
  const scale = Math.min(Math.max(rawScale, VIEWPORT_BREAKPOINTS.minScale), VIEWPORT_BREAKPOINTS.maxScale)
  return Number(scale.toFixed(3))
}

const viewportWidth = ref(
  typeof window !== 'undefined' ? window.innerWidth : VIEWPORT_BREAKPOINTS.designWidth
)
const viewportScale = ref(calculateViewportScale(viewportWidth.value))

let listenersBound = false

function applyViewportVariables(width, scale) {
  if (typeof document === 'undefined') {
    return
  }

  const root = document.documentElement
  root.style.setProperty('--vp-scale', `${scale}`)
  root.style.setProperty('--vp-viewport-width', `${Math.round(width)}px`)
}

function updateViewport() {
  if (typeof window === 'undefined') {
    return
  }

  viewportWidth.value = window.innerWidth
  viewportScale.value = calculateViewportScale(viewportWidth.value)
  applyViewportVariables(viewportWidth.value, viewportScale.value)
}

function bindListeners() {
  if (listenersBound || typeof window === 'undefined') {
    return
  }

  window.addEventListener('resize', updateViewport, { passive: true })
  window.addEventListener('orientationchange', updateViewport, { passive: true })
  updateViewport()

  listenersBound = true
}

export function initializeViewportScaling() {
  bindListeners()
}

export function useViewport(options = {}) {
  const mobileBreakpoint = options.mobileBreakpoint ?? VIEWPORT_BREAKPOINTS.mobile
  const tabletBreakpoint = options.tabletBreakpoint ?? VIEWPORT_BREAKPOINTS.tablet

  onMounted(() => {
    bindListeners()
  })

  const isMobile = computed(() => viewportWidth.value <= mobileBreakpoint)
  const isTablet = computed(
    () => viewportWidth.value > mobileBreakpoint && viewportWidth.value <= tabletBreakpoint
  )
  const isDesktop = computed(() => viewportWidth.value > tabletBreakpoint)

  return {
    viewportWidth,
    viewportScale,
    isMobile,
    isTablet,
    isDesktop
  }
}
