<template>
  <div class="chat-layout" :class="{ 'standalone': isStandalone }">
    <!-- 仅在非独立模式下显示会话列表 -->
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
import { ref, computed } from 'vue'
import SessionList from './SessionList.vue'
import ChatWidget from './ChatWidget.vue'

const currentSessionId = ref(localStorage.getItem('sessionId') || Date.now().toString(36))

function switchSession(id) {
  currentSessionId.value = id
  localStorage.setItem('sessionId', id)
}

// 防 tree shaking：直接在模板根元素上使用这个值
const isStandalone = computed(() => {
  if (typeof window !== 'undefined') {
    return window.location.pathname === '/chat'
  }
  return false
})

// 额外保险：在 onMounted 中打印，强制副作用保留
import { onMounted } from 'vue'
onMounted(() => {
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