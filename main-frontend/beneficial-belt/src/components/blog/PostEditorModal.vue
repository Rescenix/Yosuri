<template>
  <Teleport to="body">
    <div v-if="visible" class="modal-overlay" @click.self="close">
      <div class="modal-fullscreen">
        <div class="modal-header">
          <h2>写新文章</h2>
          <button class="close-btn" @click="close">✕</button>
        </div>

        <div class="modal-body">
          <input
            v-model="title"
            type="text"
            placeholder="标题"
            class="title-input"
          />

          <div class="editor-wrapper">
            <EditorContent :editor="editor" class="editor-content" />
          </div>

          <div class="tags-section">
            <div class="tags-input-wrapper">
              <input
                v-model="tagInput"
                @keyup.enter="addTag"
                placeholder="输入标签，回车添加"
                class="tag-input-field"
              />
            </div>
            <div class="tag-list">
              <span v-for="tag in tags" :key="tag" class="tag-badge">
                {{ tag }}
                <button type="button" class="remove-tag" @click="removeTag(tag)">×</button>
              </span>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="publish-btn" @click="publish" :disabled="!title || !editor?.getText().trim()">
            发布
          </button>
          <button class="cancel-btn" @click="close">取消</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
// 脚本保持不变，无需修改
import { ref, watch, onBeforeUnmount } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'

const props = defineProps({
  visible: { type: Boolean, default: false }
})
const emit = defineEmits(['close', 'published'])

const title = ref('')
const tags = ref([])
const tagInput = ref('')

function addTag() {
  const val = tagInput.value.trim()
  if (val && !tags.value.includes(val)) {
    tags.value.push(val)
  }
  tagInput.value = ''
}

function removeTag(tag) {
  tags.value = tags.value.filter(t => t !== tag)
}

const editor = useEditor({
  content: '',
  extensions: [
    StarterKit,
    Image.configure({ inline: true, allowBase64: false }),
    Placeholder.configure({ placeholder: '写下你的思考… (支持 Markdown 快捷键)' }),
  ],
  editorProps: {
    handleDrop: (view, event) => {
      const file = event.dataTransfer?.files[0]
      if (file && file.type.startsWith('image/')) {
        uploadEditorImage(file)
        return true
      }
      return false
    },
    handlePaste: (view, event) => {
      const file = event.clipboardData?.files[0]
      if (file && file.type.startsWith('image/')) {
        uploadEditorImage(file)
        return true
      }
      return false
    }
  }
})

async function uploadToOSS(file) {
  const formData = new FormData()
  formData.append('file', file)
  const token = localStorage.getItem('token')
  const res = await fetch('/api/upload', {
    method: 'POST',
    headers: token ? { 'Authorization': `Bearer ${token}` } : {},
    body: formData
  })
  if (!res.ok) throw new Error('上传失败')
  const data = await res.json()
  return data.url
}

async function uploadEditorImage(file) {
  try {
    const url = await uploadToOSS(file)
    editor.value?.chain().focus().setImage({ src: url }).run()
  } catch (err) {
    console.error('图片上传失败', err)
    alert('图片上传失败')
  }
}

async function publish() {
  const content = editor.value?.getHTML() || ''
  const payload = {
    title: title.value,
    content: content,
  }
  try {
    const token = localStorage.getItem('token')
    const res = await fetch('/api/posts', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {})
      },
      body: JSON.stringify(payload)
    })
    if (res.ok) {
      const allPostsRes = await fetch('/api/posts')
      if (allPostsRes.ok) {
        const allPosts = await allPostsRes.json()
        const newPost = allPosts.find(p => p.title === title.value && p.content === content)
        if (newPost && tags.value.length > 0) {
          await fetch(`/api/posts/${newPost.id}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tags: tags.value })
          })
        }
      }
      emit('published')
      close()
    } else {
      const err = await res.json()
      alert('发布失败：' + (err.error || '未知错误'))
    }
  } catch (err) {
    console.error(err)
    alert('网络错误')
  }
}

function close() {
  emit('close')
}

watch(() => props.visible, (val) => {
  if (val) {
    title.value = ''
    tags.value = []
    tagInput.value = ''
    editor.value?.commands.setContent('')
  }
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(12px);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-fullscreen {
  width: 100vw;
  height: 100vh;
  background: #ffffff;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 2rem;
  border-bottom: 1px solid #eef2f6;
  background: rgba(255, 255, 255, 0.96);
  backdrop-filter: blur(20px);
  flex-shrink: 0;
}
.modal-header h2 {
  margin: 0;
  font-size: 1.3rem;
  font-weight: 500;
  color: #0f172a;
  letter-spacing: -0.01em;
}
.close-btn {
  background: none;
  border: none;
  font-size: 1.8rem;
  line-height: 1;
  cursor: pointer;
  color: #94a3b8;
  transition: all 0.2s;
  width: 36px;
  height: 36px;
  border-radius: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.close-btn:hover {
  background: #f1f5f9;
  color: #475569;
}

.modal-body {
  flex: 1;
  overflow-y: auto;
  padding: 2rem 2rem 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  background: #ffffff;
}

.title-input {
  width: 100%;
  font-size: 2rem;
  font-weight: 600;
  border: none;
  border-bottom: 2px solid #e2e8f0;
  padding: 0.5rem 0 0.5rem 0;
  outline: none;
  transition: border-color 0.2s;
  color: #0f172a;
  background: transparent;
}
.title-input:focus {
  border-bottom-color: #2563eb;
}
.title-input::placeholder {
  color: #cbd5e1;
  font-weight: 400;
}

.editor-wrapper {
  border: 1px solid #e2e8f0;
  border-radius: 24px;
  overflow: hidden;
  background: #ffffff;
  transition: box-shadow 0.2s;
}
.editor-wrapper:focus-within {
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
  border-color: #2563eb;
}
.editor-content :deep(.ProseMirror) {
  min-height: 380px;
  padding: 1.2rem;
  outline: none;
  color: #1e293b;
  font-size: 1rem;
  line-height: 1.7;
}
.editor-content :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  color: #94a3b8;
  float: left;
  pointer-events: none;
  height: 0;
}
.editor-content :deep(.ProseMirror img) {
  max-width: 100%;
  border-radius: 16px;
  margin: 0.8rem 0;
  box-shadow: 0 4px 12px rgba(0,0,0,0.05);
}

.tags-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.tags-input-wrapper {
  width: 100%;
}
.tag-input-field {
  width: 100%;
  padding: 0.75rem 0;
  border: none;
  border-bottom: 1px solid #e2e8f0;
  font-size: 0.95rem;
  outline: none;
  transition: border-color 0.2s;
  color: #1e293b;
  background: transparent;
}
.tag-input-field:focus {
  border-bottom-color: #2563eb;
}
.tag-input-field::placeholder {
  color: #94a3b8;
}
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 4px;
}
.tag-badge {
  background: #eef2ff;
  color: #2563eb;
  padding: 4px 12px;
  border-radius: 40px;
  font-size: 0.85rem;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  transition: background 0.2s;
}
.tag-badge:hover {
  background: #e0e7ff;
}
.remove-tag {
  background: none;
  border: none;
  font-size: 1.1rem;
  cursor: pointer;
  color: #64748b;
  padding: 0;
  width: 20px;
  height: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 20px;
}
.remove-tag:hover {
  background: rgba(0,0,0,0.05);
  color: #ef4444;
}

.modal-footer {
  padding: 1rem 2rem;
  border-top: 1px solid #eef2f6;
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  background: rgba(255, 255, 255, 0.96);
  backdrop-filter: blur(20px);
  flex-shrink: 0;
}
.publish-btn, .cancel-btn {
  padding: 0.6rem 1.6rem;
  border-radius: 40px;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}
.publish-btn {
  background: #2563eb;
  border: none;
  color: white;
}
.publish-btn:hover {
  background: #1d4ed8;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(37,99,235,0.3);
}
.publish-btn:disabled {
  background: #94a3b8;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}
.cancel-btn {
  background: #f1f5f9;
  border: none;
  color: #475569;
}
.cancel-btn:hover {
  background: #e2e8f0;
  transform: translateY(-1px);
}

@media (max-width: 640px) {
  .modal-header { padding: 0.75rem 1rem; }
  .modal-body { padding: 1rem; }
  .title-input { font-size: 1.5rem; }
  .editor-content :deep(.ProseMirror) { min-height: 260px; }
  .modal-footer { padding: 0.75rem 1rem; }
  .publish-btn, .cancel-btn { padding: 0.5rem 1.2rem; }
}
</style>