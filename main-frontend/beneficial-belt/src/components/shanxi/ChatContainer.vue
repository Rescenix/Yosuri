<template>
  <div class="chat-layout" :class="{ 'standalone': isStandalone }">
    <!-- 仅在非独立模式下显示会话列表 -->
    <!-- <SessionList 
      v-if="!isStandalone"
      :currentSessionId="currentSessionId"
      @select="switchSession"
    /> -->
    <ChatWidget :sessionId="currentSessionId" :autoOpen="isStandalone" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import SessionList from './SessionList.vue'
import ChatWidget from './ChatWidget.vue'

const currentSessionId = ref(localStorage.getItem('sessionId') || Date.now().toString(36))

function switchSession(id) {
  currentSessionId.value = id
  localStorage.setItem('sessionId', id)
}

// 判断是否为独立聊天页（例如路由为 /chat 或移动端）
const isStandalone = computed(() => {
  // 方式1：检查 URL 路径
  if (typeof window !== 'undefined') {
    return window.location.pathname === '/chat'
  }
  return false
  // 方式2：也可检测屏幕宽度，例如 window.innerWidth < 768
})
</script>

<style scoped>
.chat-layout {
  display: flex;
  height: 100vh;
  width: 100vw;
}
.chat-layout > :last-child {
  flex: 1;
}

/* 独立模式：去掉左侧边距 */
.standalone {
  flex-direction: column;
}
</style>