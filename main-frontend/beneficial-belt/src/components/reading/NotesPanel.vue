<template>
  <div class="notes-panel" :class="{ collapsed: !expanded }">
    <div class="notes-header" @click="toggleExpand">
      <h4>读书笔记</h4>
      <Icon :icon="expanded ? 'ph:caret-left' : 'ph:caret-right'" width="16" />
    </div>
    <div v-show="expanded" class="notes-body">
      <textarea
        v-model="newNote"
        placeholder="写点笔记..."
        class="note-input"
      ></textarea>
      <button class="save-btn" @click="saveNote">
        <Icon icon="ph:plus-circle" width="16" />
        保存
      </button>
      <div class="notes-list" v-if="notes.length > 0">
        <div v-for="(note, idx) in notes" :key="idx" class="note-wrapper">
          <div
            class="note-item"
            :class="{ swiped: swipedIndex === idx }"
            @click="handleNoteClick(idx)"
          >
            <div class="note-main">
              <div class="note-time">{{ formatTime(note.time) }}</div>
              <div class="note-content">{{ note.text }}</div>
            </div>
          </div>
          <div class="delete-btn" @click.stop="deleteNote(idx)">
            <Icon icon="ph:trash" width="18" />
          </div>
        </div>
      </div>
      <div class="ai-section">
        <button class="ai-btn" @click="generateSummary">
          <Icon icon="ph:sparkle" width="16" />
          杉汐说说
        </button>
        <div v-if="aiResult" class="ai-result">{{ aiResult }}</div>
        <!-- 图片展示区域 -->
        <div v-if="aiImage" class="ai-image-container">
          <img :src="aiImage" alt="杉汐生成的插图" class="ai-image" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const STORAGE_KEY = 'reading_notes'
const notes = ref([])
const newNote = ref('')
const expanded = ref(true)
const aiResult = ref('')
const aiImage = ref('')
const swipedIndex = ref(-1)
const isGenerating = ref(false)

onMounted(() => {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved) notes.value = JSON.parse(saved)
})

function saveNotes() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(notes.value))
}

function saveNote() {
  const text = newNote.value.trim()
  if (!text) return
  notes.value.push({ text, time: Date.now() })
  newNote.value = ''
  saveNotes()
}

function formatTime(ts) {
  const d = new Date(ts)
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

function toggleExpand() {
  expanded.value = !expanded.value
}

function handleNoteClick(idx) {
  swipedIndex.value = swipedIndex.value === idx ? -1 : idx
}

function deleteNote(idx) {
  notes.value.splice(idx, 1)
  saveNotes()
  swipedIndex.value = -1
}

async function generateSummary() {
  if (isGenerating.value) return
  isGenerating.value = true
  aiResult.value = '杉汐正在思考...'
  aiImage.value = ''

  try {
    // 1. 生成总结
    const allNotes = notes.value.map(n => n.text).join('\n')
    const prompt = allNotes
      ? `根据以下读书笔记，写一段简短的阅读总结（不超过100字）。笔记内容：\n${allNotes}`
      : '请为读者写一段简短的阅读总结（不超过100字）。'

    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: prompt })
    })
    if (!res.ok) throw new Error(`总结请求失败 (${res.status})`)
    const data = await res.json()
    const summary = data.reply || data.message || '暂时无法生成总结。'
    aiResult.value = summary

    // 2. 生成插图
    aiResult.value += '\n\n杉汐正在画图...'
    const imgRes = await fetch('/api/image/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ prompt: summary })
    })

    if (imgRes.ok) {
  const imgData = await imgRes.json()
  let rawUrl = imgData.url || imgData.imageUrl || ''
  if (rawUrl) {
    // 自动替换 localhost 为当前域名
    if (rawUrl.includes('localhost')) {
      const url = new URL(rawUrl)
      rawUrl = rawUrl.replace(url.origin, window.location.origin)
    }
    if (!rawUrl.startsWith('http')) {
      rawUrl = window.location.origin + (rawUrl.startsWith('/') ? '' : '/') + rawUrl
    }
    aiImage.value = rawUrl
  } else {
    // ★ 如果后端返回空 URL，则隐藏图片区域，只显示总结
    aiImage.value = ''
    console.warn('后端返回图片URL为空')
  }

    } else if (imgRes.status === 429) {
      aiResult.value = summary + '\n\n（绘图接口繁忙，请稍后再试）'
      console.warn('绘图接口 429 限流')
    } else {
      console.warn('绘图接口返回错误', imgRes.status)
    }

    aiResult.value = summary
  } catch (e) {
    aiResult.value = '生成失败，请重试。'
    console.error(e)
  } finally {
    setTimeout(() => { isGenerating.value = false }, 5000)
  }
}
</script>

<style scoped>
.notes-panel {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  box-sizing: border-box;
  width: 100%;
  max-width: 100%;
  transition: width 0.2s ease;
  font-family: 'Inter', system-ui, sans-serif;
}
.notes-panel.collapsed {
  width: 40px;
  flex: none;
}
.notes-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  cursor: pointer;
  border-bottom: 1px solid #e2e8f0;
  user-select: none;
}
.notes-header h4 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
  color: #1e293b;
  white-space: nowrap;
}
.notes-body {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: calc(100vh - 200px);
  overflow-y: auto;
}
.note-input {
  width: 100%;
  height: 80px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 10px 12px;
  font-size: 0.9rem;
  color: #334155;
  background: #f8fafc;
  resize: vertical;
  box-sizing: border-box;
  outline: none;
  transition: border-color 0.2s;
  font-family: inherit;
}
.note-input:focus {
  border-color: #60a5fa;
  background: #ffffff;
}
.note-input::placeholder {
  color: #94a3b8;
}
.save-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  padding: 8px;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  color: #2563eb;
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
  font-family: inherit;
}
.save-btn:hover {
  background: #dbeafe;
}
.notes-list {
  max-height: 200px;
  overflow-y: auto;
}
.note-wrapper {
  position: relative;
  overflow: hidden;
  margin-bottom: 6px;
  border-radius: 8px;
}
.note-item {
  display: flex;
  align-items: center;
  transition: transform 0.25s ease;
  transform: translateX(0);
  padding: 10px 0;
  border-bottom: 1px solid #f1f5f9;
  background: #fff;
  cursor: pointer;
  position: relative;
  z-index: 2;
}
.note-item.swiped {
  transform: translateX(-36px);
}
.note-main {
  flex: 1;
  min-width: 0;
  padding-right: 8px;
}
.note-time {
  font-size: 0.75rem;
  color: #94a3b8;
  margin-bottom: 4px;
}
.note-content {
  font-size: 0.85rem;
  color: #334155;
  line-height: 1.6;
  word-break: break-word;
}
.delete-btn {
  position: absolute;
  right: -36px;
  top: 0;
  bottom: 0;
  width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fee2e2;
  color: #ef4444;
  cursor: pointer;
  border-radius: 0 8px 8px 0;
  transition: right 0.25s ease;
  z-index: 1;
}
.note-item.swiped ~ .delete-btn {
  right: 0;
}
.ai-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid #f1f5f9;
}
.ai-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  padding: 10px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  color: #475569;
  font-size: 0.85rem;
  cursor: pointer;
  transition: all 0.2s;
  font-family: inherit;
}
.ai-btn:hover {
  background: #f1f5f9;
  border-color: #cbd5e1;
}
.ai-result {
  font-size: 0.85rem;
  color: #334155;
  background: #f8fafc;
  padding: 10px;
  border-radius: 8px;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.6;
  border: 1px solid #e2e8f0;
}
.ai-image-container {
  margin-top: 8px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
}
.ai-image {
  width: 100%;
  display: block;
}
</style>