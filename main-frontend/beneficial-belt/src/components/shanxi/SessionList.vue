<template>
  <div class="session-list">
    <button class="new-session-btn" @click="newSession">+ 新对话</button>
    <div 
      v-for="s in sessions" 
      :key="s.id"
      :class="['session-item', { active: s.id === currentSessionId }]"
      @click="$emit('select', s.id)"
    >
      <div class="session-title">{{ s.title }}</div>
      <div class="session-time">{{ formatTime(s.updated_at) }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

defineProps(['currentSessionId'])
const emit = defineEmits(['select'])

const sessions = ref([])

onMounted(async () => {
    const res = await fetch('/api/sessions')
    if (res.ok) sessions.value = await res.json()
})

async function newSession() {
    const res = await fetch('/api/sessions', { method: 'POST' })
    if (res.ok) {
        const data = await res.json()
        emit('select', data.session_id)
    }
}

function formatTime(t) {
    if (!t) return ''
    const d = new Date(t)
    return `${d.getMonth()+1}/${d.getDate()} ${d.getHours()}:${String(d.getMinutes()).padStart(2,'0')}`
}
</script>

<style scoped>
.session-list {
  width: 260px;
  background: #f8fafc;
  border-right: 1px solid #e5e5e5;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
}
.new-session-btn {
  margin-bottom: 8px;
  padding: 8px;
  border: 1px solid #d0d5dd;
  border-radius: 8px;
  background: #fff;
  cursor: pointer;
  font-size: 14px;
  color: #333;
}
.new-session-btn:hover {
  background: #f0f0f0;
}
.session-item {
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
}
.session-item:hover {
  background: #e8ecf1;
}
.session-item.active {
  background: #dbeafe;
}
.session-title {
  font-size: 14px;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.session-time {
  font-size: 11px;
  color: #999;
  margin-top: 2px;
}
</style>