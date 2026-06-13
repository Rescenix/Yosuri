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

const books = ref([])
const uploadModalVisible = ref(false)
const editModalVisible = ref(false)
const editingBook = ref(null)

// 从 localStorage 读取书架缓存
function loadBooksFromCache() {
  try {
    const cached = localStorage.getItem('shanxi_book_list')
    if (cached) {
      const list = JSON.parse(cached)
      if (Array.isArray(list)) {
        books.value = list
        return true
      }
    }
  } catch (e) {}
  return false
}

// 保存书架数据到 localStorage
function saveBooksToCache(list) {
  try {
    localStorage.setItem('shanxi_book_list', JSON.stringify(list))
  } catch (e) {}
}

async function loadBooks() {
  // 1. 优先显示缓存数据，提升离线体验
  if (loadBooksFromCache()) {
    // 后台静默更新（仅在线时成功）
    fetch('/api/book/list')
      .then(res => res.json())
      .then(data => {
        const list = data.books || []
        books.value = list
        saveBooksToCache(list)
      })
      .catch(() => {})  // 网络失败不影响已显示的数据
    return
  }

  // 2. 无缓存，必须网络请求
  try {
    const res = await fetch('/api/book/list')
    if (res.ok) {
      const list = (await res.json()).books || []
      books.value = list
      saveBooksToCache(list)
    }
  } catch (e) {
    console.error('书架加载失败', e)
  }
}

onMounted(async () => {
  await loadBooks()
})

function openBook(book) {
  const baseUrl = `/read?book=${encodeURIComponent(book.id)}`;
  if (book.id.startsWith('local_')) {
    window.location.href = baseUrl + '&local=true';
  } else {
    window.location.href = baseUrl;
  }
}

function onBooksUploaded(newBooks) {
  books.value = newBooks || []
  saveBooksToCache(books.value)
}

function openEditor(book) {
  editingBook.value = book
  editModalVisible.value = true
}

async function handleSaveBook({ title, cover }) {
  await loadBooks()
  editModalVisible.value = false
  saveBooksToCache(books.value)
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

    // 清除 IndexedDB 中该书的所有缓存
    try {
      const db = await openDB()
      const tx = db.transaction(STORE_NAME, 'readwrite')
      const store = tx.objectStore(STORE_NAME)
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
    saveBooksToCache(books.value)
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