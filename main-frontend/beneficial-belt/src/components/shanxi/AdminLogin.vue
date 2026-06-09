<template>
  <div class="login-box">
    <input
      v-if="!isLoggedIn"
      v-model="password"
      type="password"
      placeholder="Admin密码"
      @keypress.enter="login"
    />
    <button v-if="!isLoggedIn" @click="login">登录</button>
    <span v-else class="login-status">开发者模式</span>
    <button v-if="isLoggedIn" @click="logout">退出</button>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const password = ref('')
const isLoggedIn = ref(!!localStorage.getItem('token'))

const login = () => {
  // ★ 直接生成模拟 token，不再请求后端
  const fakeToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYWRtaW4iLCJleHAiOjk5OTk5OTk5OTl9.fake-signature'
  localStorage.setItem('token', fakeToken)
  isLoggedIn.value = true
  password.value = ''
  window.dispatchEvent(new Event('login-state-changed'))
  window.dispatchEvent(new Event('auth-change'))
}

const logout = () => {
  localStorage.removeItem('token')
  isLoggedIn.value = false
  window.dispatchEvent(new Event('auth-change'))
}
</script>

<style scoped>
.login-box {
  display: flex;
  align-items: center;
  gap: 8px;
}
.login-box input {
  width: 120px;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid rgba(255, 140, 180, 0.3);
  background: rgba(15, 10, 20, 0.6);
  color: #e2e8f0;
  font-size: 13px;
}
.login-box button {
  padding: 4px 12px;
  border-radius: 4px;
  border: none;
  color: #1a1a2e;
  cursor: pointer;
  font-size: 13px;
}
.login-status {
  color: #000000;
  font-size: 13px;
}
</style>