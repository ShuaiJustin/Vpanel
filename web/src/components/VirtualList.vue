<template>
  <div 
    ref="containerRef"
    class="virtual-list-container"
    :style="containerStyle"
    @scroll="handleScroll"
  >
    <div 
      class="virtual-list-phantom"
      :style="{ height: totalHeight + 'px' }"
    />
    <div 
      class="virtual-list-content"
      :style="contentStyle"
    >
      <div
        v-for="item in visibleItems"
        :key="getItemKey(item)"
        class="virtual-list-item"
        :style="{ height: itemHeight + 'px' }"
      >
        <slot
          :item="item.data"
          :index="item.index"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
/**
 * 虚拟滚动列表组件
 * 用于高效渲染大量数据列表
 */
import { ref, computed, watch } from 'vue'

const props = defineProps({
  // 数据列表
  items: {
    type: Array,
    required: true
  },
  // 每项高度
  itemHeight: {
    type: Number,
    default: 50
  },
  // 容器高度
  height: {
    type: [Number, String],
    default: 400
  },
  // 缓冲区大小（额外渲染的项数）
  buffer: {
    type: Number,
    default: 5
  },
  // 获取项的唯一键
  itemKey: {
    type: [String, Function],
    default: 'id'
  }
})

const emit = defineEmits(['scroll', 'reach-bottom'])

// 容器引用
const containerRef = ref(null)

// 滚动位置
const scrollTop = ref(0)

// 容器样式
const containerStyle = computed(() => ({
  height: typeof props.height === 'number' ? `${props.height}px` : props.height,
  overflow: 'auto',
  position: 'relative'
}))

// 总高度
const totalHeight = computed(() => props.items.length * props.itemHeight)

// 可见区域高度
const viewportHeight = computed(() => {
  if (typeof props.height === 'number') {
    return props.height
  }
  return containerRef.value?.clientHeight || 400
})

// 可见项数量
const visibleCount = computed(() => 
  Math.ceil(viewportHeight.value / props.itemHeight) + props.buffer * 2
)

// 起始索引
const startIndex = computed(() => {
  const index = Math.floor(scrollTop.value / props.itemHeight) - props.buffer
  return Math.max(0, index)
})

// 结束索引
const endIndex = computed(() => {
  const index = startIndex.value + visibleCount.value
  return Math.min(props.items.length, index)
})

// 可见项
const visibleItems = computed(() => {
  return props.items.slice(startIndex.value, endIndex.value).map((data, i) => ({
    data,
    index: startIndex.value + i
  }))
})

// 内容偏移样式
const contentStyle = computed(() => ({
  transform: `translateY(${startIndex.value * props.itemHeight}px)`,
  position: 'absolute',
  left: 0,
  right: 0,
  top: 0
}))

// 获取项的唯一键
const getItemKey = (item) => {
  if (typeof props.itemKey === 'function') {
    return props.itemKey(item.data)
  }
  return item.data[props.itemKey] ?? item.index
}

// 处理滚动
const handleScroll = (event) => {
  scrollTop.value = event.target.scrollTop
  emit('scroll', { scrollTop: scrollTop.value, event })
  
  // 检测是否到达底部
  const { scrollHeight, clientHeight } = event.target
  if (scrollTop.value + clientHeight >= scrollHeight - props.itemHeight) {
    emit('reach-bottom')
  }
}

// 滚动到指定索引
const scrollToIndex = (index) => {
  if (containerRef.value) {
    containerRef.value.scrollTop = index * props.itemHeight
  }
}

// 滚动到顶部
const scrollToTop = () => {
  if (containerRef.value) {
    containerRef.value.scrollTop = 0
  }
}

// 滚动到底部
const scrollToBottom = () => {
  if (containerRef.value) {
    containerRef.value.scrollTop = totalHeight.value
  }
}

// 暴露方法
defineExpose({
  scrollToIndex,
  scrollToTop,
  scrollToBottom,
  getScrollTop: () => scrollTop.value
})

// 监听数据变化，重置滚动位置
watch(() => props.items.length, (newLength, oldLength) => {
  if (newLength < oldLength) {
    // 数据减少时，确保滚动位置有效
    const maxScrollTop = Math.max(0, newLength * props.itemHeight - viewportHeight.value)
    if (scrollTop.value > maxScrollTop) {
      scrollTop.value = maxScrollTop
      if (containerRef.value) {
        containerRef.value.scrollTop = maxScrollTop
      }
    }
  }
})
</script>

<style scoped>
.virtual-list-container {
  will-change: transform;
}

.virtual-list-phantom {
  position: absolute;
  left: 0;
  top: 0;
  right: 0;
  z-index: -1;
}

.virtual-list-content {
  will-change: transform;
}

.virtual-list-item {
  box-sizing: border-box;
}
</style>
