<template>
  <nav class="navbar">
    <div class="nav-content">
      <div class="nav-brand">
        <router-link to="/">星尘核心</router-link>
      </div>

      <!-- 桌面端正常链接 -->
      <NavLinks class="desktop-only" />

      <!-- 移动端侧边栏按钮 -->
      <button class="mobile-menu-btn" aria-label="菜单" @click="sidebarOpen = true">
        <span class="bar"></span>
        <span class="bar"></span>
        <span class="bar"></span>
      </button>

      <!-- 侧边栏覆盖层 -->
      <div class="mobile-sidebar" :class="{ open: sidebarOpen }" @click.self="sidebarOpen = false">
        <div class="sidebar-header">
          <span class="sidebar-title">导航</span>
          <button class="sidebar-close" @click="sidebarOpen = false">&times;</button>
        </div>
        <div class="sidebar-links">
          <NavLinks />
        </div>
        <div class="sidebar-actions">
          <AdminLogin />
        </div>
      </div>

      <!-- 桌面端登录 -->
      <div class="nav-actions desktop-only">
        <AdminLogin />
      </div>
    </div>
  </nav>
</template>

<script setup>
import { ref } from 'vue'
import NavLinks from './shanxi/NavLinks.vue'
import AdminLogin from './shanxi/AdminLogin.vue'

const sidebarOpen = ref(false)
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
  border-bottom: 1px solid rgba(0, 102, 204, 0.1);
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
  color: #0066cc;
  text-decoration: none;
  letter-spacing: -0.3px;
}

.desktop-only {
  display: block;
}

.mobile-menu-btn {
  display: none;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 4px;
  background: none;
  border: none;
  cursor: pointer;
  padding: 8px;
}

.bar {
  width: 22px;
  height: 2px;
  background: #334155;
  border-radius: 1px;
}

.mobile-sidebar {
  position: fixed;
  top: 0;
  right: -280px;
  width: 260px;
  height: 100vh;
  background: #ffffff;
  box-shadow: -2px 0 12px rgba(0,0,0,0.1);
  z-index: 200;
  transition: right 0.3s ease;
  display: flex;
  flex-direction: column;
  padding: 20px;
  box-sizing: border-box;
}

.mobile-sidebar.open {
  right: 0;
}

.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.sidebar-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: #0f172a;
}

.sidebar-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #64748b;
  padding: 4px;
}

.sidebar-links :deep(.nav-links) {
  flex-direction: column;
  gap: 16px;
}

.sidebar-links :deep(.nav-links a) {
  font-size: 1rem;
  padding: 8px 0;
}

.sidebar-actions {
  margin-top: 32px;
}

@media (max-width: 768px) {
  .desktop-only {
    display: none !important;
  }
  .mobile-menu-btn {
    display: flex;
  }
  .nav-content {
    padding: 0 16px;
  }
}
</style>
