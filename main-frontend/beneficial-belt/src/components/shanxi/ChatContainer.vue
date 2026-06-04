<template>
  <div class="chat-layout">
    <SessionList 
      :currentSessionId="currentSessionId"
      @select="switchSession"
    />
    <ChatWidget 
      :sessionId="currentSessionId"
    />
  </div>
</template>

<script setup>
import { ref } from 'vue'
import SessionList from './SessionList.vue'
import ChatWidget from './ChatWidget.vue'

const currentSessionId = ref(localStorage.getItem('sessionId') || Date.now().toString(36))

function switchSession(id) {
    currentSessionId.value = id
    localStorage.setItem('sessionId', id)
}
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
</style>