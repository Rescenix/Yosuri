<template>
  <div class="login-box">
    <input
      v-if="!auth.isLoggedIn.value"
      v-model="password"
      type="password"
      placeholder="Admin密码"
      @keypress.enter="login"
    />
    <button v-if="!auth.isLoggedIn.value" @click="login">登录</button>
    <a
      v-if="!auth.isLoggedIn.value"
      class="gh-btn"
      href="/api/auth/github"
      title="使用 GitHub 账号登录"
    >GitHub 登录</a>

    <!-- 已登录：展示头像 + 用户名 -->
    <div v-else class="user-chip">
      <img v-if="auth.displayAvatar.value" :src="auth.displayAvatar.value" class="user-avatar" alt="avatar" />
      <Icon v-else icon="mdi:account-circle" width="20" color="#e2e8f0" />
      <span class="user-name">{{ auth.displayName.value }}</span>
      <button class="logout-btn" @click="auth.logout()">退出</button>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { Icon } from '@iconify/vue'
import { useAuth } from '../../composables/useAuth.js'

const auth = useAuth()
const password = ref('')

const login = async () => {
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password: password.value })
  })
  if (res.ok) {
    const data = await res.json()
    localStorage.setItem('token', data.token)
    password.value = ''
    // 通知登录态变化；useAuth 监听 auth-change 会自动刷新用户名/头像
    window.dispatchEvent(new Event('login-state-changed'))
    window.dispatchEvent(new Event('auth-change'))
  } else {
    alert('密码错误')
  }
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
.user-chip {
  display: flex;
  align-items: center;
  gap: 8px;
}
.user-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  object-fit: cover;
  border: 1px solid rgba(255, 140, 180, 0.4);
}
.user-name {
  color: #e2e8f0;
  font-size: 13px;
}
.logout-btn {
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid rgba(255, 140, 180, 0.3);
  background: rgba(15, 10, 20, 0.6);
  color: #e2e8f0;
  cursor: pointer;
  font-size: 13px;
}
.gh-btn {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: 4px;
  border: 1px solid rgba(255, 140, 180, 0.3);
  background: rgba(15, 10, 20, 0.6);
  color: #e2e8f0;
  font-size: 13px;
  text-decoration: none;
  cursor: pointer;
}
.gh-btn:hover {
  border-color: rgba(255, 140, 180, 0.6);
}
</style>
