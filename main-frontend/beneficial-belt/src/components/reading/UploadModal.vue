<template>
  <div v-if="visible" class="modal-overlay" @click.self="close">
    <div class="modal-card">
      <div class="card-header">
        <h3>添加书籍</h3>
        <button class="close-btn" @click="close">
          <Icon icon="ph:x" width="18" />
        </button>
      </div>

      <div class="card-body">
        <!-- 文件选择区域 -->
        <div class="file-area" :class="{ 'has-file': file }" @click="triggerFileInput">
          <input
            ref="fileInput"
            type="file"
            accept=".txt"
            hidden
            @change="onFileChange"
          />
          <Icon v-if="!file" icon="ph:upload-simple" width="24" />
          <div v-else class="file-info">
            <Icon icon="ph:file-text" width="20" />
            <span class="file-name">{{ fileName }}</span>
          </div>
        </div>

        <!-- 书名输入 -->
        <div class="form-group">
          <label>书名</label>
          <input
            v-model="title"
            class="input"
            placeholder="输入书名（不填则使用文件名）"
          />
        </div>

        <!-- 可选封面 -->
        <div class="form-group">
          <label>封面图片（可选）</label>
          <div class="cover-area" @click="triggerCoverInput">
            <input
              ref="coverInput"
              type="file"
              accept="image/*"
              hidden
              @change="onCoverChange"
            />
            <img v-if="coverPreview" :src="coverPreview" class="cover-preview" />
            <div v-else class="cover-placeholder">
              <Icon icon="ph:image" width="24" />
              <span>点击选择封面</span>
            </div>
          </div>
        </div>
      </div>

      <div class="card-footer">
        <button class="btn-cancel" @click="close">取消</button>
        <button class="btn-upload" @click="upload" :disabled="!file">
          <Icon icon="ph:plus" width="18" />
          <span>上传</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({ visible: Boolean })
const emit = defineEmits(['close', 'uploaded'])

const fileInput = ref(null)
const coverInput = ref(null)

const file = ref(null)
const fileName = ref('')
const title = ref('')
const coverFile = ref(null)
const coverPreview = ref('')

function triggerFileInput() {
  fileInput.value?.click()
}

function triggerCoverInput() {
  coverInput.value?.click()
}

function onFileChange(e) {
  const f = e.target.files[0]
  if (f) {
    file.value = f
    fileName.value = f.name
    if (!title.value) {
      title.value = f.name.replace(/\.txt$/i, '')
    }
  }
}

function onCoverChange(e) {
  const f = e.target.files[0]
  if (f) {
    coverFile.value = f
    const reader = new FileReader()
    reader.onload = (ev) => { coverPreview.value = ev.target.result }
    reader.readAsDataURL(f)
  }
}

async function upload() {
  if (!file.value) return
  const formData = new FormData()
  formData.append('file', file.value)
  formData.append('title', title.value || file.value.name.replace(/\.txt$/i, ''))
  if (coverFile.value) formData.append('cover', coverFile.value)

  try {
    const res = await fetch('/api/book/upload', { method: 'POST', body: formData })
    const text = await res.text() // 先读为文本
    let data
    try {
      data = JSON.parse(text) // 尝试解析 JSON
    } catch (e) {
      throw new Error('服务器返回异常: ' + text.slice(0, 200))
    }
    if (!res.ok) throw new Error(data.error || '上传失败')
    emit('uploaded', data.books)
    close()
  } catch (e) {
    alert('上传失败: ' + e.message)
    console.error(e)
  }
}

function close() {
  emit('close')
  // 重置表单
  file.value = null
  fileName.value = ''
  title.value = ''
  coverFile.value = null
  coverPreview.value = ''
}
</script>

<style scoped>
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.4);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal-card {
  background: #fff; border-radius: 16px; width: 400px;
  box-shadow: 0 8px 30px rgba(0,0,0,0.12);
  overflow: hidden;
}
.card-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 16px 20px; border-bottom: 1px solid #f1f5f9;
}
.card-header h3 { margin: 0; font-size: 1.1rem; color: #1e293b; }
.close-btn { background: transparent; border: none; cursor: pointer; color: #94a3b8; }
.card-body { padding: 20px; }
.file-area {
  border: 2px dashed #e2e8f0; border-radius: 12px; padding: 24px;
  text-align: center; cursor: pointer; transition: all 0.2s;
  margin-bottom: 20px;
}
.file-area:hover { border-color: #3b82f6; background: #f8fafc; }
.file-area.has-file { border-color: #3b82f6; background: #eff6ff; }
.file-info {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  color: #1e293b; font-size: 0.9rem;
}
.file-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 200px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 6px; font-size: 0.9rem; color: #475569; }
.input {
  width: 100%; padding: 10px 12px; border: 1px solid #e2e8f0; border-radius: 8px;
  font-size: 0.9rem; color: #334155; outline: none; transition: border-color 0.2s;
  box-sizing: border-box;
}
.input:focus { border-color: #60a5fa; }
.cover-area {
  border: 2px dashed #e2e8f0; border-radius: 8px; padding: 16px;
  text-align: center; cursor: pointer; transition: all 0.2s;
  min-height: 80px; display: flex; align-items: center; justify-content: center;
}
.cover-area:hover { border-color: #3b82f6; background: #f8fafc; }
.cover-placeholder {
  display: flex; align-items: center; gap: 8px; color: #94a3b8; font-size: 0.9rem;
}
.cover-preview { max-width: 100%; max-height: 120px; object-fit: contain; border-radius: 6px; }
.card-footer {
  display: flex; justify-content: flex-end; gap: 12px; padding: 16px 20px;
  background: #f8fafc; border-top: 1px solid #f1f5f9;
}
.btn-cancel {
  background: #fff; border: 1px solid #e2e8f0; padding: 8px 20px;
  border-radius: 8px; font-size: 0.9rem; cursor: pointer; color: #475569; transition: background 0.2s;
}
.btn-cancel:hover { background: #f1f5f9; }
.btn-upload {
  display: flex; align-items: center; gap: 6px;
  background: #3b82f6; color: #fff; border: none; padding: 8px 20px;
  border-radius: 8px; font-size: 0.9rem; cursor: pointer; transition: background 0.2s;
}
.btn-upload:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-upload:hover:not(:disabled) { background: #2563eb; }
</style>