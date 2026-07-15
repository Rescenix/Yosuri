<template>
  <ChatWidget autoOpen sessionId="" />
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'
import ChatWidget from '../components/shanxi/chat/ChatWidget.vue'

let prevOverflow = ''

onMounted(() => {
  document.title = '杉汐 | 对话'
  prevOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
})

onUnmounted(() => {
  document.body.style.overflow = prevOverflow
})
</script>
