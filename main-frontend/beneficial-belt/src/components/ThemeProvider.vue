<!-- src/components/ThemeProvider.vue -->
<template>
  <div>
    <slot />
  </div>
</template>

<script setup>
import { ref, provide, onMounted, onBeforeUnmount } from 'vue'

const currentThemeColor = ref('#60a5fa')

// 监听全局主题切换事件
const handleThemeSwitch = (event) => {
  console.log('🔄 ThemeProvider 收到主题切换:', event.detail)
  currentThemeColor.value = event.detail
}

onMounted(() => {
  window.addEventListener('theme-switch', handleThemeSwitch)
})

onBeforeUnmount(() => {
  window.removeEventListener('theme-switch', handleThemeSwitch)
})

// 将当前主题色通过 provide 传递给子组件（如 ParticleBackground）
provide('currentThemeColor', currentThemeColor)
</script>