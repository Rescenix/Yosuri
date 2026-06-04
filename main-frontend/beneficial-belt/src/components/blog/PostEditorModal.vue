<template>
  <Teleport to="body">
    <div v-if="visible" class="modal-overlay" @click.self="close">
      <div class="modal-container">
        <div class="modal-header">
          <h2>写新文章</h2>
          <button class="close-btn" @click="close">✕</button>
        </div>

        <div class="modal-body">
          <!-- 标题 -->
          <input
            v-model="title"
            type="text"
            placeholder="标题"
            class="title-input"
          />

          <!-- 编辑器 -->
          <div class="editor-wrapper">
            <EditorContent :editor="editor" class="editor-content" />
          </div>

          <!-- 标签输入（放在编辑器下方） -->
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
      // 重新获取所有文章
      const allPostsRes = await fetch('/api/posts')
      if (allPostsRes.ok) {
        const allPosts = await allPostsRes.json()
        // 找到刚发布的文章（根据标题和内容匹配）
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
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(12px);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-container {
  width: 90%;
  max-width: 900px;
  max-height: 85vh;
  background: #ffffff;
  border-radius: 32px;
  box-shadow: 0 20px 35px -12px rgba(0, 0, 0, 0.15);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: fadeInUp 0.2s ease;
}

@keyframes fadeInUp {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid #eef2f6;
}
.modal-header h2 {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 500;
  color: #0f172a;
}
.close-btn {
  background: none;
  border: none;
  font-size: 1.4rem;
  cursor: pointer;
  color: #94a3b8;
  transition: color 0.2s;
}
.close-btn:hover { color: #475569; }

.modal-body {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
}

.title-input {
  width: 100%;
  font-size: 1.8rem;
  font-weight: 600;
  border: none;
  border-bottom: 2px solid #e2e8f0;
  padding: 0.5rem 0;
  outline: none;
  transition: border-color 0.2s;
  color: #0f172a;
}
.title-input:focus { border-bottom-color: #2563eb; }
.title-input::placeholder { color: #cbd5e1; font-weight: 400; }

.editor-wrapper {
  border: 1px solid #e2e8f0;
  border-radius: 20px;
  overflow: hidden;
  background: #ffffff;
}
.editor-content :deep(.ProseMirror) {
  min-height: 320px;
  padding: 1rem;
  outline: none;
  color: #1e293b;
  font-size: 0.95rem;
  line-height: 1.6;
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
  border-radius: 12px;
  margin: 0.5rem 0;
}

/* 标签区域（放在编辑器下方） */
.tags-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.tags-input-wrapper {
  width: 100%;
}
.tag-input-field {
  width: 100%;
  padding: 10px 0;
  border: none;
  border-bottom: 1px solid #e2e8f0;
  font-size: 0.95rem;
  outline: none;
  transition: border-color 0.2s;
  color: #1e293b;
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
  padding: 4px 10px;
  border-radius: 30px;
  font-size: 0.8rem;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.remove-tag {
  background: none;
  border: none;
  font-size: 1rem;
  cursor: pointer;
  color: #64748b;
  padding: 0 4px;
}
.remove-tag:hover { color: #ef4444; }

.modal-footer {
  padding: 1rem 1.5rem;
  border-top: 1px solid #eef2f6;
  display: flex;
  justify-content: flex-start;
  gap: 1rem;
}
.publish-btn, .cancel-btn {
  padding: 0.5rem 1.2rem;
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
.publish-btn:hover { background: #1d4ed8; }
.publish-btn:disabled {
  background: #94a3b8;
  cursor: not-allowed;
}
.cancel-btn {
  background: #f1f5f9;
  border: none;
  color: #475569;
}
.cancel-btn:hover { background: #e2e8f0; }

@media (max-width: 640px) {
  .modal-container { width: 95%; max-height: 90vh; }
  .modal-body { padding: 1rem; }
  .title-input { font-size: 1.4rem; }
  .editor-content :deep(.ProseMirror) { min-height: 240px; }
}
</style>