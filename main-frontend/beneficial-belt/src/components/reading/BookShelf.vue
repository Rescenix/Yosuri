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
        <button class="edit-btn" @click.stop="openEditor(book)">
          <Icon icon="ph:gear" width="16" />
        </button>
        <div class="book-cover-placeholder" @click="openBook(book)">
          <img v-if="book.cover" :src="book.cover" class="cover-image" />
          <span v-else class="book-icon">📖</span>
        </div>
        <h3 class="book-title" @click="openBook(book)">{{ book.title }}</h3>
      </div>
    </div>

    <UploadModal
      :visible="uploadModalVisible"
      @close="uploadModalVisible = false"
      @uploaded="onBooksUploaded"
    />

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
import { cacheBookList, getBookListFromCache, cacheBookText } from '../composables/bookCache.js'

const books = ref([])
const uploadModalVisible = ref(false)
const editModalVisible = ref(false)
const editingBook = ref(null)

// 保存书架到 localStorage（辅助函数）
function saveBooksToLocal(list) {
  try {
    localStorage.setItem('shanxi_book_list', JSON.stringify(list))
  } catch(e) {}
}

async function loadBooks() {
  // 1. 先加载 IndexedDB 缓存
  const cached = await getBookListFromCache()
  if (cached && cached.length) {
    books.value = cached
  } else {
    // 无缓存时尝试 localStorage 旧缓存
    const local = localStorage.getItem('shanxi_book_list')
    if (local) {
      try {
        books.value = JSON.parse(local)
      } catch(e) {}
    }
  }

  // 2. 联网更新书架并缓存书籍文本
  try {
    const res = await fetch('/api/book/list')
    if (res.ok) {
      const data = await res.json()
      const list = data.books || []
      books.value = list
      await cacheBookList(list)
      saveBooksToLocal(list)

      // 后台缓存每本书的文本（仅当在线）
      for (const book of list) {
        try {
          const textRes = await fetch(`/api/book/content?bookId=${encodeURIComponent(book.id)}`)
          if (textRes.ok) {
            const text = await textRes.text()
            await cacheBookText(book.id, text)
          }
        } catch(e) {}
      }
    }
  } catch (e) {
    console.warn('联网更新失败，使用缓存数据')
  }
}

onMounted(() => {
  loadBooks()
})

function openBook(book) {
  // 保存当前书籍ID，用于离线恢复
  localStorage.setItem('shanxi_last_book', book.id)
  window.location.href = `/read?book=${encodeURIComponent(book.id)}`
}

function onBooksUploaded(newBooks) {
  books.value = newBooks || []
  saveBooksToLocal(books.value)
  loadBooks() // 重新加载以缓存新书文本
}

function openEditor(book) {
  editingBook.value = book
  editModalVisible.value = true
}

async function handleSaveBook({ title, cover }) {
  await loadBooks()
  editModalVisible.value = false
}

async function handleDeleteBook() {
  const bookId = editingBook.value.id
  try {
    const res = await fetch(`/api/book/delete?bookId=${encodeURIComponent(bookId)}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('删除失败')
    editModalVisible.value = false
    await loadBooks()
  } catch (e) {
    alert('删除失败: ' + e.message)
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

.book-card {
  position: relative;
  cursor: pointer;
  transition: transform 0.2s;
  text-align: center;
}
.book-card:hover { transform: translateY(-4px); }

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

.book-title {
  font-size: 0.85rem;
  margin: 8px 0 0;
  color: var(--text-primary, #0f172a);
  cursor: pointer;
}
</style>