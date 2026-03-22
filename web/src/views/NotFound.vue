<template>
  <div class="not-found">
    <h1>404</h1>
    <p>页面未找到</p>
    <button @click="goHome">
      返回首页
    </button>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
const router = useRouter()
const goHome = () => { router.push('/') }
</script>
