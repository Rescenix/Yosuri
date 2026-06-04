import { ref, onMounted } from 'vue'

export function useReader() {
  const loading = ref(true)
  const error = ref('')
  const title = ref('')
  const fullText = ref('')
  const fontSize = ref(20)
  const bookmarks = ref([]) // 格式：[{ page: number, text: string, time: number }]

  const currentProgress = ref(0) // 0-100 (百分比)
  
  const changeFont = () => {
    const sizes = [16, 20, 24]
    const idx = sizes.indexOf(fontSize.value)
    fontSize.value = sizes[(idx + 1) % 3]
    saveProgress()
  }

  // 添加或删除书签（根据页码）
  function toggleBookmark(page, text) {
    const index = bookmarks.value.findIndex(b => b.page === page)
    if (index !== -1) {
      bookmarks.value.splice(index, 1)
    } else {
      bookmarks.value.push({ page, text, time: Date.now() })
    }
    saveProgress()
  }

  // 判断某页是否为书签
  function isBookmarked(page) {
    return bookmarks.value.some(b => b.page === page)
  }

  // 删除指定书签（可选）
  function removeBookmark(page) {
    bookmarks.value = bookmarks.value.filter(b => b.page !== page)
    saveProgress()
  }

  const STORAGE_KEY = 'reading-progress'
  const saveProgress = () => {
    const key = `${STORAGE_KEY}-${title.value}`
    localStorage.setItem(key, JSON.stringify({
      progress: currentProgress.value,
      fontSize: fontSize.value,
      bookmarks: bookmarks.value
    }))
  }

  const readingMode = ref('traditional')
  const toggleReadingMode = () => {
    readingMode.value = readingMode.value === 'traditional' ? 'three-d' : 'traditional'
    localStorage.setItem('reading-mode', readingMode.value)
  }

  const restoreProgress = () => {
    const key = `${STORAGE_KEY}-${title.value}`
    const raw = localStorage.getItem(key)
    if (raw) {
      try {
        const data = JSON.parse(raw)
        if (data.progress !== undefined) currentProgress.value = data.progress
        if (data.fontSize) fontSize.value = data.fontSize
        if (data.bookmarks) bookmarks.value = data.bookmarks
      } catch (e) { /* ignore */ }
    }
  }

  onMounted(() => {
    const savedMode = localStorage.getItem('reading-mode')
    if (savedMode) readingMode.value = savedMode
  })

  return {
    loading, error, title, fullText, fontSize, bookmarks,
    currentProgress, isBookmarked, readingMode, toggleReadingMode,
    changeFont, toggleBookmark, removeBookmark,
    saveProgress, restoreProgress
  }
}