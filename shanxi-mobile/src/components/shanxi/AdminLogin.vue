<template>
  <div class="login-box">
    <!-- 永远显示“已解锁”，无需输入密码 -->
    <span class="login-status">🔓 开发者模式已解锁</span>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'

// 硬编码的万能 token（后端 DEV_MODE=true 时不需要验证，仅用于本地存储）
const hardcodedToken = 'dev-permanent-token'

onMounted(() => {
  // 强制写入 token 并通知全局状态
  localStorage.setItem('token', hardcodedToken)
  window.dispatchEvent(new Event('auth-change'))
  window.dispatchEvent(new Event('login-state-changed'))
})
</script>

<style scoped>
.login-box {
  display: flex;
  align-items: center;
  gap: 8px;
}
.login-status {
  color: #000000;
  font-size: 13px;
}
</style>