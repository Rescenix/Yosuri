<template>
  <DynamicWallpaper />
  <router-view />
</template>

<script setup>
import { onMounted } from 'vue'
import { useAuth } from './composables/useAuth.js'
import DynamicWallpaper from './components/shanxi/chat/DynamicWallpaper.vue'

const auth = useAuth()

// GitHub OAuth 回调回收：GitHubCallback 把 JWT 通过 ?token= 带回前端首页。
// 先存好 token 再派发 auth-change，由 useAuth 统一去 /api/auth/me 验真并拉用户名/头像；
// 伪造/过期的 token 会被 useAuth 清掉，不会误判登录成功。URL 里的 token 立即清掉防泄露。
onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (token) {
    const url = new URL(window.location.href)
    url.searchParams.delete('token')
    window.history.replaceState({}, document.title, url.pathname + url.search)

    localStorage.setItem('token', token)
    window.dispatchEvent(new Event('auth-change'))
  }
})
</script>
