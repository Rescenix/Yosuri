<template>
  <ul class="nav-links">
    <li><a href="/shanxi-hut">项目库</a></li>
    <li><a href="/blog">研习录</a></li>
    <li><a href="/timeline">生命线</a></li>
    <li v-if="isLoggedIn"><a href="/image-bed">图片簿</a></li>
  </ul>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const isLoggedIn = ref(false)

function updateLoginState() {
  isLoggedIn.value = !!localStorage.getItem('token')
}

onMounted(() => {
  updateLoginState()
  window.addEventListener('auth-change', updateLoginState)
})

onUnmounted(() => {
  window.removeEventListener('auth-change', updateLoginState)
})
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