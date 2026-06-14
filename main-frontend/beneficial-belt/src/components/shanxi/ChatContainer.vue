<template>
  <div class="chat-layout" :class="{ 'standalone': isStandalone }">
    <SessionList 
      v-if="!isStandalone"
      :currentSessionId="currentSessionId"
      @select="switchSession"
    />
    <ChatWidget 
      :sessionId="currentSessionId"
      :autoOpen="isStandalone"
    />
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

// 用 ref 替代 computed，确保可以强制修改
const isStandalone = ref(false)

onMounted(() => {
  // 强制根据 URL 设置独立模式
  if (typeof window !== 'undefined' && window.location.pathname.startsWith('/chat')) {
    isStandalone.value = true
  }
  console.log('[ChatContainer] isStandalone:', isStandalone.value)
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

.standalone {
  flex-direction: column;
}
</style>