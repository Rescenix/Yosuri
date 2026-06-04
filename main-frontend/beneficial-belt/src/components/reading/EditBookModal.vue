<template>
  <div v-if="visible" class="modal-overlay" @click.self="close">
    <div class="modal-card">
      <div class="card-header">
        <h3>编辑书籍</h3>
        <button class="close-btn" @click="close">
          <Icon icon="ph:x" width="18" />
        </button>
      </div>

      <div class="card-body">
        <!-- 封面预览与更换 -->
        <div class="cover-section">
          <div
            class="cover-preview"
            :style="{ backgroundImage: `url(${coverPreview})` }"
            @click="triggerCoverInput"
          >
            <div v-if="!coverPreview" class="cover-placeholder">
              <Icon icon="ph:book" width="32" />
            </div>
          </div>
          <label class="cover-upload-label">
            <Icon icon="ph:camera" width="16" />
            <span>更换封面</span>
            <input
              type="file"
              accept="image/*"
              hidden
              @change="onCoverChange"
            />
          </label>
        </div>

        <!-- 书名 -->
        <div class="form-group">
          <label>书名</label>
          <input
            v-model="editTitle"
            class="input"
            placeholder="输入书名"
          />
        </div>
      </div>

      <div class="card-footer">
        <button class="btn-save" @click="save">保存</button>
        <button class="btn-delete" @click="confirmDelete">删除书籍</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  visible: Boolean,
  book: Object
})
const emit = defineEmits(['close', 'save', 'delete'])

const editTitle = ref('')
const coverFile = ref(null)
const coverPreview = ref('')

watch(() => props.book, (book) => {
  if (book) {
    editTitle.value = book.title || book.id
    coverPreview.value = book.cover || ''
    // 重置文件选择状态（避免之前选择的文件残留）
    coverFile.value = null
  }
}, { immediate: true })

function triggerCoverInput() {
  // 点击预览区域也可触发文件选择
  const fileInput = document.querySelector('.cover-upload-label input[type="file"]')
  if (fileInput) fileInput.click()
}

function onCoverChange(e) {
  const file = e.target.files[0]
  if (file) {
    coverFile.value = file
    const reader = new FileReader()
    reader.onload = (ev) => { coverPreview.value = ev.target.result }
    reader.readAsDataURL(file)
  }
}

async function save() {
  // 封面上传（如果选择了新封面）
  if (coverFile.value) {
    try {
      const form = new FormData()
      form.append('cover', coverFile.value)
      form.append('bookId', props.book.id)
      const res = await fetch('/api/book/upload-cover', { method: 'POST', body: form })
      const text = await res.text()
      let data
      try { data = JSON.parse(text) } catch (e) {
        throw new Error('服务器返回异常: ' + text.slice(0, 200))
      }
      if (!res.ok) throw new Error(data.error || '封面上传失败')
      // 上传成功，将新封面 URL 传出去
      emit('save', { title: editTitle.value, cover: data.cover || coverPreview.value })
      return
    } catch (e) {
      alert('封面上传失败: ' + e.message)
      return
    }
  }

  // 未选择新封面，直接保存书名
  emit('save', { title: editTitle.value, cover: props.book?.cover || '' })
}

function confirmDelete() {
  if (confirm('确定删除这本书吗？所有相关缓存和数据将被清除。')) {
    emit('delete')
  }
}

function close() {
  emit('close')
}
</script>

<style scoped>
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.4);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal-card {
  background: #fff; border-radius: 16px; width: 380px;
  box-shadow: 0 8px 30px rgba(0,0,0,0.12); overflow: hidden;
}
.card-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 16px 20px; border-bottom: 1px solid #f1f5f9;
}
.card-header h3 { margin: 0; font-size: 1.1rem; color: #1e293b; }
.close-btn { background: transparent; border: none; cursor: pointer; color: #94a3b8; }
.card-body { padding: 20px; }
.cover-section {
  display: flex; flex-direction: column; align-items: center; margin-bottom: 20px;
}
.cover-preview {
  width: 120px; height: 160px; border-radius: 8px; background-size: cover;
  background-position: center; background-color: #f8fafc;
  border: 1px solid #e2e8f0; display: flex; align-items: center; justify-content: center;
  margin-bottom: 12px; cursor: pointer;
}
.cover-placeholder { color: #cbd5e1; }
.cover-upload-label {
  display: flex; align-items: center; gap: 6px; font-size: 0.85rem;
  color: #3b82f6; cursor: pointer; padding: 6px 12px; border-radius: 6px;
  transition: background 0.2s;
}
.cover-upload-label:hover { background: #eff6ff; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 6px; font-size: 0.9rem; color: #475569; }
.input {
  width: 100%; padding: 10px 12px; border: 1px solid #e2e8f0; border-radius: 8px;
  font-size: 0.9rem; color: #334155; outline: none; transition: border-color 0.2s;
  box-sizing: border-box;
}
.input:focus { border-color: #60a5fa; }
.card-footer {
  display: flex; justify-content: space-between; padding: 16px 20px;
  background: #f8fafc; border-top: 1px solid #f1f5f9;
}
.btn-save {
  background: #3b82f6; color: #fff; border: none; padding: 8px 20px;
  border-radius: 8px; font-size: 0.9rem; cursor: pointer; transition: background 0.2s;
}
.btn-save:hover { background: #2563eb; }
.btn-delete {
  background: #fff; color: #ef4444; border: 1px solid #fecaca; padding: 8px 20px;
  border-radius: 8px; font-size: 0.9rem; cursor: pointer; transition: all 0.2s;
}
.btn-delete:hover { background: #fef2f2; }
</style>