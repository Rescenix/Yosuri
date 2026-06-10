<template>
  <div class="shelf-root">
    <div class="shelf-toolbar">
      <button class="upload-btn" @click="uploadModalVisible = true">
        <Icon icon="ph:plus" width="18" />
        <span>添加书籍</span>
      </button>
    </div>

    <div v-if="books.length === 0" class="empty-shelf">
      <p>书架空空，点击上方按钮添加一本 TXT 书籍</p>
    </div>

    <div v-else class="book-grid">
      <div v-for="book in books" :key="book.id" class="book-card">
        <!-- 编辑按钮（右上角） -->
        <button class="edit-btn" @click.stop="openEditor(book)">
          <Icon icon="ph:gear" width="16" />
        </button>

        <!-- 封面区域（可点击打开书籍） -->
        <div class="book-cover-placeholder" @click="openBook(book)">
          <img v-if="book.cover" :src="book.cover" class="cover-image" />
          <span v-else class="book-icon">📖</span>
        </div>

        <!-- 书名（可点击打开书籍） -->
        <h3 class="book-title" @click="openBook(book)">{{ book.title }}</h3>
      </div>
    </div>

    <!-- 上传模态窗口 -->
    <UploadModal
      :visible="uploadModalVisible"
      @close="uploadModalVisible = false"
      @uploaded="onBooksUploaded"
    />

    <!-- 编辑弹窗 -->
    <EditBookModal
      :visible="editModalVisible"
      :book="editingBook"
      @close="editModalVisible = false"
      @save="handleSaveBook"
      @delete="handleDeleteBook"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import UploadModal from './UploadModal.vue'
import EditBookModal from './EditBookModal.vue'
import { openDB, STORE_NAME } from './cachePagination.js'

const books = ref([])
const uploadModalVisible = ref(false)
const editModalVisible = ref(false)
const editingBook = ref(null)

onMounted(async () => {
  try {
    const params = new URLSearchParams(window.location.search)
    const file = params.get('book')
    if (!file) throw new Error('未指定书籍')
    reader.title.value = file.replace(/\.txt$/i, '')

    // 直接从网络获取书籍内容，不使用任何缓存
    const res = await fetch(`/api/book/content?bookId=${encodeURIComponent(file)}`)
    if (!res.ok) throw new Error('书籍加载失败')
    const text = await res.text()

    reader.fullText.value = text
    await nextTick()
    outline.value = parseOutline(text)
    reader.restoreProgress()
  } catch (e) {
    reader.error.value = e.message
  } finally {
    reader.loading.value = false
  }
})

async function loadBooks() {
    const res = await fetch('/api/book/list')
    if (res.ok) {
        books.value = (await res.json()).books || []
    }
}


onMounted(async () => {
  await loadBooks()
})

function openBook(book) {
  window.location.href = `/read?book=${encodeURIComponent(book.id)}`
}

function onBooksUploaded(newBooks) {
  books.value = newBooks || []
}

function openEditor(book) {
  editingBook.value = book
  editModalVisible.value = true
}
async function handleSaveBook({ title, cover }) {
  // 如果 cover 是 URL，说明已经上传成功；否则可能是 base64 或留空
  // 这里无需再上传，直接刷新书架即可（后端已更新索引）
  await loadBooks()
  editModalVisible.value = false
}
async function handleDeleteBook() {
  const bookId = editingBook.value.id
  try {
    const res = await fetch(`/api/book/delete?bookId=${encodeURIComponent(bookId)}`, { method: 'DELETE' })
    const text = await res.text()
    let data
    try {
      data = JSON.parse(text)
    } catch (e) {
      throw new Error('服务器返回异常: ' + text.slice(0, 200))
    }
    if (!res.ok) throw new Error(data.error || '删除失败')

    // 清除 IndexedDB 中该书的所有缓存（兼容所有浏览器）
    try {
      const db = await openDB()
      const tx = db.transaction(STORE_NAME, 'readwrite')
      const store = tx.objectStore(STORE_NAME)
      
      // 使用游标遍历所有键，代替可能不支持的 getAllKeys()
      const keys = []
      await new Promise((resolve, reject) => {
        const cursorRequest = store.openCursor()
        cursorRequest.onsuccess = (event) => {
          const cursor = event.target.result
          if (cursor) {
            keys.push(cursor.key)
            cursor.continue()
          } else {
            resolve()
          }
        }
        cursorRequest.onerror = () => reject(cursorRequest.error)
      })

      // 删除匹配的键
      for (const key of keys) {
        if (key && key.toString().startsWith(`${bookId}_`)) {
          store.delete(key)
        }
      }

      await new Promise((resolve, reject) => {
        tx.oncomplete = resolve
        tx.onerror = reject
      })
    } catch (e) {
      console.warn('清除本地缓存失败:', e)
    }

    editModalVisible.value = false
    await loadBooks()
  } catch (e) {
    alert('删除失败: ' + e.message)
    console.error(e)
  }
}
</script>

<style scoped>
.shelf-root { padding: 20px 0; }
.shelf-toolbar { display: flex; justify-content: flex-end; gap: 8px; margin-bottom: 20px; }
.upload-btn {
  display: flex; align-items: center; gap: 6px;
  background: var(--bg-card, #ffffff); border: 1px solid var(--border, #e2e8f0);
  padding: 8px 16px; border-radius: 8px; color: var(--text-primary, #0f172a);
  font-size: 0.9rem; cursor: pointer; transition: background 0.2s, box-shadow 0.2s;
}
.upload-btn:hover { background: #f8fafc; box-shadow: 0 2px 8px rgba(0,0,0,0.04); }
.empty-shelf { text-align: center; color: var(--text-secondary, #64748b); padding: 40px; font-size: 0.95rem; }
.book-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 16px; }

/* 书籍卡片 */
.book-card {
  position: relative;
  cursor: pointer;
  transition: transform 0.2s;
  text-align: center;
}
.book-card:hover { transform: translateY(-4px); }

/* 编辑按钮（右上角） */
.edit-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  background: rgba(255,255,255,0.9);
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 4px 8px;
  cursor: pointer;
  color: #64748b;
  z-index: 2;
  opacity: 0;
  transition: opacity 0.2s;
}
.book-card:hover .edit-btn { opacity: 1; }
.edit-btn:hover { background: #f1f5f9; color: #3b82f6; }

/* 封面区域 */
.book-cover-placeholder {
  width: 100%; aspect-ratio: 2/3;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  font-size: 2rem;
  overflow: hidden;
}
.cover-image { width: 100%; height: 100%; object-fit: cover; border-radius: 8px; }

/* 书名 */
.book-title {
  font-size: 0.85rem;
  margin: 8px 0 0;
  color: var(--text-primary, #0f172a);
  cursor: pointer;
}
</style>