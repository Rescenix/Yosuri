<template>
  <div class="mobile-menu-container">
    <!-- 汉堡按钮 -->
    <button id="mobile-menu-btn" class="mobile-menu-btn" aria-label="菜单" @click="openSidebar">
      <span class="bar"></span>
      <span class="bar"></span>
      <span class="bar"></span>
    </button>

    <!-- 侧边栏 -->
    <div :class="['mobile-sidebar', { open: isOpen }]" @click="closeOnOverlay($event)">
      <div class="sidebar-header">
        <span class="sidebar-title">导航</span>
        <button class="sidebar-close" @click="closeSidebar">&times;</button>
      </div>
      <div class="sidebar-links">
        <NavLinks />
      </div>
      <div class="sidebar-actions">
        <AdminLogin />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import NavLinks from './NavLinks.vue'
import AdminLogin from './AdminLogin.vue'

const isOpen = ref(false)

const openSidebar = () => { isOpen.value = true }
const closeSidebar = () => { isOpen.value = false }
const closeOnOverlay = (e) => {
  if (e.target === e.currentTarget) closeSidebar()
}
</script>

<style scoped>
/* 复用你之前定义的样式，并确保移动端显示 */
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
.sidebar-actions {
  margin-top: 32px;
}
@media (max-width: 768px) {
  .mobile-menu-btn {
    display: flex;
  }
}
</style>