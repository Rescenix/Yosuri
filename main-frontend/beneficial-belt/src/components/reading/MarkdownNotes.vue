<template>
  <div class="notes-container">
    <div class="notes-header">
     
      <button class="btn-new" @click="createNew">+ 新建</button>
    </div>

    <!-- 笔记列表（showEditor 为 false 时显示） -->
    <div class="notes-list" v-if="!showEditor && notes.length > 0">
      <div
        v-for="note in notes"
        :key="note.id"
        class="note-item"
        @click="editNote(note)"
      >
        <div class="note-meta">
          <span class="note-date">{{ formatTime(note.time) }}</span>
          <button class="btn-delete" @click.stop="deleteNote(note.id)">✕</button>
        </div>
        <div class="note-preview">{{ note.preview }}</div>
      </div>
    </div>

    <!-- 编辑器（showEditor 为 true 时显示） -->
    <div class="editor-area" v-if="showEditor">
      <div class="editor-toolbar">
        <button @click="showEditor = false">← 返回</button>
        <span class="editor-date">{{ editingNote ? formatTime(editingNote.time) : '新笔记' }}</span>
        <button @click="saveNote">保存</button>
      </div>
      <textarea
        v-model="content"
        placeholder="支持 Markdown 语法…"
        class="editor-textarea"
      ></textarea>
    </div>

    <div v-if="notes.length === 0 && !showEditor" class="empty">暂无笔记，点击上方按钮创建</div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const STORAGE_KEY = 'mobile_reading_notes'
const notes = ref([])
const editingNote = ref(null)
const content = ref('')
const showEditor = ref(false)

onMounted(() => {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved) notes.value = JSON.parse(saved)
})

function createNew() {
  editingNote.value = null
  content.value = ''
  showEditor.value = true
}

function editNote(note) {
  editingNote.value = note
  content.value = note.content
  showEditor.value = true
}

function saveNote() {
  const text = content.value.trim()
  if (!text) return
  if (editingNote.value) {
    const index = notes.value.findIndex(n => n.id === editingNote.value.id)
    if (index !== -1) {
      notes.value[index].content = text
      notes.value[index].time = Date.now()
      notes.value[index].preview = text.slice(0, 50) + (text.length > 50 ? '…' : '')
    }
  } else {
    notes.value.push({
      id: Date.now(),
      content: text,
      time: Date.now(),
      preview: text.slice(0, 50) + (text.length > 50 ? '…' : ''),
    })
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(notes.value))
  showEditor.value = false
}

function deleteNote(id) {
  notes.value = notes.value.filter(n => n.id !== id)
  localStorage.setItem(STORAGE_KEY, JSON.stringify(notes.value))
}

function formatTime(timestamp) {
  const d = new Date(timestamp)
  return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`
}
</script>

<style scoped>
.notes-container { padding: 12px; height: 100%; display: flex; flex-direction: column; }
.notes-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.notes-header h4 { margin: 0; }
.btn-new { background: #3b82f6; color: #fff; border: none; padding: 6px 12px; border-radius: 6px; cursor: pointer; }
.notes-list { flex: 1; overflow-y: auto; }
.note-item { background: #fff; border: 1px solid #e2e8f0; border-radius: 8px; padding: 10px; margin-bottom: 8px; cursor: pointer; }
.note-meta { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.note-date { font-size: 0.75rem; color: #94a3b8; }
.btn-delete { background: none; border: none; color: #ef4444; cursor: pointer; padding: 2px 4px; }
.note-preview { font-size: 0.85rem; color: #334155; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.editor-area { flex: 1; display: flex; flex-direction: column; }
.editor-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.editor-toolbar button { background: #f1f5f9; border: 1px solid #cbd5e1; padding: 4px 10px; border-radius: 6px; cursor: pointer; }
.editor-date { font-size: 0.8rem; color: #64748b; }
.editor-textarea { flex: 1; border: 1px solid #e2e8f0; border-radius: 8px; padding: 10px; font-size: 0.9rem; resize: none; outline: none; }
.empty { text-align: center; color: #94a3b8; margin-top: 30px; }
</style>