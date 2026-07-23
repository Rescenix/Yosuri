<template>
  <nav class="navbar">
    <div class="nav-content">
      <div class="nav-brand">
        <router-link to="/">星尘核心</router-link>
      </div>

      <!-- 桌面端登录 -->
      <div class="nav-actions desktop-only">
        <AdminLogin />
      </div>
    </div>
  </nav>
</template>

<script setup>
import AdminLogin from './shanxi/AdminLogin.vue'
</script>

<style scoped>
.navbar {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  z-index: 100;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid color-mix(in srgb, var(--app-accent) 12%, transparent);
  box-sizing: border-box;
}

.nav-content {
  max-width: 1400px;
  margin: 0 auto;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 24px;
  height: 60px;
}

.nav-brand a {
  font-size: 1.3rem;
  font-weight: 600;
  color: var(--app-accent);
  text-decoration: none;
  letter-spacing: -0.3px;
}

.nav-actions {
  display: flex;
  align-items: center;
}
</style>
