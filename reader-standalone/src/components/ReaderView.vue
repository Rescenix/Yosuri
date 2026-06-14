<template>
  <div class="reader-root">
    <!-- 加载动画 -->
    <div v-if="reader.loading.value" class="loading-overlay">
      <div class="ink-loader">
        <div class="ink-drop"></div>
        <div class="ink-ripple ripple-1"></div>
        <div class="ink-ripple ripple-2"></div>
        <div class="ink-ripple ripple-3"></div>
      </div>
      <p class="loading-text">墨韵渐染，书页将开</p>
    </div>

    <!-- 错误提示 -->
    <div v-else-if="reader.error.value" class="status-msg error">{{ reader.error.value }}</div>

    <template v-else>
      <div class="toolbar">
        <button class="tb-btn" @click="back">← 书架</button>
        <div class="tb-actions">
          <button class="tb-btn" @click="openMobilePanel('notes')">
            <Icon icon="ph:notebook" width="18" />
          </button>
          <button class="tb-btn" @click="openMobilePanel('progress')">
            <Icon icon="ph:chart-bar" width="18" />
          </button>
          <button class="tb-btn" @click="toggleBookmarkAtCurrentPage">
            <Icon :icon="isCurrentPageBookmarked ? 'ph:bookmark-simple-fill' : 'ph:bookmark-simple'" width="18" />
          </button>
          <button class="tb-btn" @click="reader.changeFont()">{{ reader.fontSize.value }}px</button>
          <button class="tb-btn" @click="showOutline = !showOutline">
            <Icon icon="ph:list-bullets" width="18" />
          </button>
        </div>
      </div>

      <div class="reader-body">
        <ThreeReader ref="threeReaderRef" :reader="reader" />
      </div>

      <!-- 目录/书签浮层 -->
      <transition name="outline-fade">
        <div v-if="showOutline" class="outline-overlay" @click.self="showOutline = false">
          <div class="outline-panel">
            <div class="outline-header">
              <div class="outline-tabs">
                <button :class="['tab-btn', { active: outlineTab === 'outline' }]" @click="outlineTab = 'outline'">目录</button>
                <button :class="['tab-btn', { active: outlineTab === 'bookmarks' }]" @click="outlineTab = 'bookmarks'">书签</button>
              </div>
              <button class="tb-btn" @click="showOutline = false">
                <Icon icon="ph:x" width="18" />
              </button>
            </div>

            <div v-show="outlineTab === 'outline'" class="outline-list">
              <div v-for="(item, idx) in outline" :key="idx" class="outline-item" @click="jumpToChapter(item)">
                {{ item.title }}
              </div>
              <div v-if="outline.length === 0" class="outline-empty">未识别到章节标题</div>
            </div>

            <div v-show="outlineTab === 'bookmarks'" class="outline-list">
              <div v-for="(bm, idx) in reader.bookmarks.value" :key="idx" class="bookmark-wrapper">
                <div
                  class="bookmark-item"
                  :class="{ swiped: swipedBookmarkIndex === idx }"
                  @touchstart.prevent="onBookmarkTouchStart($event, idx)"
                  @touchmove="onBookmarkTouchMove"
                  @touchend="onBookmarkTouchEnd($event, bm, idx)"
                >
                  <div class="bm-info">
                    <span class="bm-page">第{{ bm.page + 1 }}页</span>
                    <span class="bm-text">{{ bm.text }}</span>
                  </div>
                  <div class="delete-btn" @touchstart.prevent.stop="deleteBookmark(idx)">
                    <Icon icon="ph:trash" width="18" />
                  </div>
                </div>
              </div>
              <div v-if="reader.bookmarks.value.length === 0" class="outline-empty">暂无书签</div>
            </div>
          </div>
        </div>
      </transition>
    </template>

    <!-- 移动端面板浮层 -->
    <transition name="slide-down">
      <div v-if="activeMobilePanel" class="mobile-panel-overlay" @click.self="activeMobilePanel = null">
        <div class="mobile-panel">
          <div class="mobile-panel-header">
            <span>{{ activeMobilePanel === 'notes' ? '读书笔记' : '杉汐的痕迹' }}</span>
            <button class="tb-btn" @click="activeMobilePanel = null">
              <Icon icon="ph:x" width="18" />
            </button>
          </div>
          <div class="mobile-panel-content">
            <MarkdownNotes v-if="activeMobilePanel === 'notes'" />
            <ReadingProgress
              v-else-if="activeMobilePanel === 'progress'"
              :book-title="reader.title.value"
              :total-pages="totalPages"
              :current-page="currentPage"
            />
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick, computed } from 'vue'
import { Icon } from '@iconify/vue'
import { useReader } from './useReader.js'
import ThreeReader from './ThreeReader.vue'
import MarkdownNotes from './MarkdownNotes.vue'
import ReadingProgress from './ReadingProgress.vue'

const activeMobilePanel = ref(null)
const reader = useReader()
window.__reader__ = reader

const threeReaderRef = ref(null)
const showOutline = ref(false)
const outline = ref([])
const outlineTab = ref('outline')
const swipedBookmarkIndex = ref(-1)
const touchStartX = {}
const SWIPE_THRESHOLD = 40

let panelTimer = null
function openMobilePanel(type) {
  if (panelTimer) return
  panelTimer = setTimeout(() => {
    activeMobilePanel.value = activeMobilePanel.value === type ? null : type
    panelTimer = null
  }, 100)
}

function parseOutline(fullText) {
  const lines = fullText.split('\n')
  const chapterPattern = /^(第[一二三四五六七八九十百千\d]+[卷章节回部篇]|序章|尾声|楔子|前言|后记|[Pp]art\s+\d+|Chapter\s+\d+|第[0-9]+[章節回]|[零壹贰叁肆伍陆柒捌玖拾佰仟\d]+[、．.]\s*)([ 　\t].*)?$/u
  const result = []
  lines.forEach((line, idx) => {
    const trimmed = line.trim()
    if (chapterPattern.test(trimmed)) {
      result.push({ title: trimmed, lineIndex: idx })
    }
  })
  return result
}

const currentPage = computed(() => threeReaderRef.value?.currentPage ?? 0)
const isCurrentPageBookmarked = computed(() => reader.isBookmarked(currentPage.value))
const totalPages = computed(() => threeReaderRef.value?.totalPages ?? 0)

const toggleBookmarkAtCurrentPage = () => {
  const page = currentPage.value
  const text = threeReaderRef.value?.getCurrentPageText?.() || `第${page + 1}页`
  reader.toggleBookmark(page, text)
}

function onBookmarkTouchStart(e, idx) { touchStartX[idx] = e.touches[0].clientX }
function onBookmarkTouchMove() {}
function onBookmarkTouchEnd(e, bm, idx) {
  if (e.target.closest('.delete-btn')) return
  const startX = touchStartX[idx]
  delete touchStartX[idx]
  if (startX === undefined) return
  const endX = e.changedTouches[0].clientX
  const dx = endX - startX
  if (dx < -SWIPE_THRESHOLD) swipedBookmarkIndex.value = idx
  else if (dx > SWIPE_THRESHOLD) swipedBookmarkIndex.value = -1
  else {
    if (swipedBookmarkIndex.value === idx) swipedBookmarkIndex.value = -1
    else { jumpToPage(bm.page); swipedBookmarkIndex.value = -1 }
  }
}
function deleteBookmark(idx) {
  reader.bookmarks.value = reader.bookmarks.value.filter((_, i) => i !== idx)
  localStorage.setItem('shanxi_bookmarks', JSON.stringify(reader.bookmarks.value))
  swipedBookmarkIndex.value = -1
}
function jumpToPage(page) {
  if (threeReaderRef.value?.flipToPage) threeReaderRef.value.flipToPage(page)
  showOutline.value = false
}
function jumpToChapter(item) {
  if (threeReaderRef.value?.jumpToChapter) threeReaderRef.value.jumpToChapter(item.title)
  showOutline.value = false
}

// ========== 统一书籍加载（修复时序） ==========
async function loadBookContent(bookId) {
  reader.loading.value = true
  reader.error.value = ''
  reader.title.value = bookId.replace(/\.txt$/i, '')
  localStorage.setItem('shanxi_last_book', bookId)

  let text = ''
  if (typeof window.getBookText === 'function') {
    text = window.getBookText(bookId)
  } else if (import.meta.env.DEV) {
    try {
      const res = await fetch(`/api/book/content?bookId=${encodeURIComponent(bookId)}`)
      if (res.ok) text = await res.text()
    } catch (e) {
      console.warn('在线获取失败')
    }
  }

  if (!text) {
    reader.error.value = '无法获取书籍内容'
    reader.loading.value = false
    return
  }

  // 关键：动画持续 200ms，期间不注入文本（无排版、无进度恢复）
  setTimeout(() => {
    reader.fullText.value = text
    nextTick().then(() => {
      outline.value = parseOutline(text)
      reader.restoreProgress?.()
      reader.loading.value = false
    })
  }, 200)
}

// ========== 初始化 ==========
onMounted(() => {
  reader.loading.value = true
  reader.error.value = ''

  if (import.meta.env.DEV) {
    const params = new URLSearchParams(window.location.search)
    const file = params.get('book')
    if (file) {
      loadBookContent(file)
      return
    }
  }
  // Android 环境：保持 loading，等待原生 setBook
})

const back = () => {
  if (window.androidJsBridge?.backToShelf) {
    window.androidJsBridge.backToShelf()
  } else if (import.meta.env.DEV) {
    window.location.href = '/reading-hut'
  }
}

window.setBook = (bookId, isLocal) => {
  loadBookContent(bookId)
}
</script>

<style src="./ReaderView.css" scoped></style>