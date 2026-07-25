<template>
  <router-view />
</template>

<script setup>
import { onMounted } from 'vue'

// GitHub OAuth 回调回收：GitHubCallback 会把 JWT 通过 ?token= 带回前端首页，
// 这里解析并存入 localStorage（与 AdminLogin 的 token 键一致），随后清掉 URL 里的 token。
onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (token) {
    localStorage.setItem('token', token)
    // 清掉 URL 中的 token，避免刷新重复写入 / 泄露
    const url = new URL(window.location.href)
    url.searchParams.delete('token')
    window.history.replaceState({}, document.title, url.pathname + url.search)
    // 通知登录态变化（AdminLogin 等组件监听此事件）
    window.dispatchEvent(new Event('auth-change'))
  }
})
</script>
