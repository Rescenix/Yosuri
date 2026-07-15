<template>
  <div class="app-shell">
    <main class="chat-page">
      <ChatWidget sessionId="" :autoOpen="true" />
    </main>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'
import ChatWidget from '../components/shanxi/chat/ChatWidget.vue'

let prevOverflow = ''

onMounted(() => {
  document.title = 'Aurora'
  prevOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
})

onUnmounted(() => {
  document.body.style.overflow = prevOverflow
})
</script>

<style scoped>
.app-shell {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}

.chat-page {
  flex: 1;
  min-height: 0;
  padding: 0;
  background: #ffffff;
  position: relative;
}

.chat-page :deep(.chat-window) {
  position: absolute;
  inset: 0;
  width: auto;
  height: auto;
  max-height: none;
  border-radius: 24px;
}

.chat-page :deep(.chat-window.expanded) {
  inset: 0;
  border-radius: 0;
}

.chat-page :deep(.chat-toggle-button) {
  display: none;
}

.chat-page :deep(.chat-window .message.bot) {
  max-width: 100%;
}

.chat-page :deep(.chat-window .message.user) {
  max-width: 85%;
}

.chat-page :deep(.chat-window .chat-messages) {
  padding-bottom: 18px;
}
</style>
