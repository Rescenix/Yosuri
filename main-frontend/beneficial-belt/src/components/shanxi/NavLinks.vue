<template>
  <ul class="nav-links">
    <li><a href="/blog">博客</a></li>
  </ul>
</template>

<script setup>
import { useAuth } from '../composables/useAuth.js'

// 登录态统一由 useAuth 管理（含验真 + 监听 auth-change 自动刷新）
const isLoggedIn = useAuth().isLoggedIn
</script>

<style scoped>
.nav-links {
  list-style: none;
  display: flex;
  gap: 32px;
  margin: 0;
  padding: 0;
}

.nav-links a,
.nav-links a:visited,
.nav-links a:focus,
.nav-links a:active {
  color: #334155 !important;
  text-decoration: none !important;
  font-size: 0.9rem;
  font-weight: 450;
  border-bottom: 2px solid transparent;
  transition: color 0.2s, border-color 0.2s;
  padding: 6px 0;
  outline: none;
}

.nav-links a:hover {
  color: #0066cc;
  border-bottom-color: #0066cc;
}
</style>
