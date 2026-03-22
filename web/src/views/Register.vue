<template>
  <div class="register-container">
    <h2>注册账号</h2>
    <button @click="goToLogin">
      返回登录
    </button>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()
const goToLogin = () => {
  router.push('/user/login')
}
</script>

<style scoped>
.register-container {
  padding: 20px;
  text-align: center;
}
</style>
