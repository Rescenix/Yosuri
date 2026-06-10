<template>
  <div class="reader-root">
    <div v-if="reader.loading.value" class="status-msg">加载中...</div>
    <div v-else-if="reader.error.value" class="status-msg error">{{ reader.error.value }}</div>

    <template v-else>
      <div class="toolbar">
        <button class="tb-btn" @click="back">← 书架</button>
        <div class="tb-actions">
          <button v-if="isMobile" class="tb-btn" @click="openMobilePanel('notes')">
            <Icon icon="ph:notebook" width="18" />
          </button>
          <button v-if="isMobile" class="tb-btn" @click="openMobilePanel('progress')">
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
        <div class="left-spacer"></div>
        <div class="reader-card">
          <div class="three-reader-wrapper">
            <ThreeReader ref="threeReaderRef" :reader="reader" />
          </div>
        </div>
        <div class="right-panels">
          <NotesPanel />
          <div class="side-panel">
            <SidePanel :threeReaderRef="threeReaderRef" />
          </div>
        </div>
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
                  @click="isMobile ? null : handleBookmarkClick(bm, idx)"
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
      <div v-if="isMobile && activeMobilePanel" class="mobile-panel-overlay" @click.self="activeMobilePanel = null">
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
import { ref, onMounted, nextTick, computed, onBeforeUnmount } from 'vue'
import { Icon } from '@iconify/vue'
import { useReader } from './useReader.js'
import ThreeReader from './ThreeReader.vue'
import SidePanel from './SidePanel.vue'
import NotesPanel from './NotesPanel.vue'
import MarkdownNotes from './MarkdownNotes.vue'
import ReadingProgress from './ReadingProgress.vue'

const activeMobilePanel = ref(null)
const reader = useReader()
const threeReaderRef = ref(null)
const showOutline = ref(false)
const outline = ref([])
const outlineTab = ref('outline')
const swipedBookmarkIndex = ref(-1)
const panelTitle = computed(() => {
  if (activeMobilePanel.value === 'notes') return '读书笔记'
  if (activeMobilePanel.value === 'progress') return '阅读进度'
  return ''
})
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

const isMobile = ref(window.innerWidth <= 768)
let mediaQuery = null

onMounted(() => {
  mediaQuery = window.matchMedia('(max-width: 768px)')
  isMobile.value = mediaQuery.matches
  const handler = (e) => { isMobile.value = e.matches }
  mediaQuery.addEventListener('change', handler)
  mediaQuery._handler = handler
})

onBeforeUnmount(() => {
  if (mediaQuery && mediaQuery._handler) {
    mediaQuery.removeEventListener('change', mediaQuery._handler)
  }
})

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

const toggleBookmarkAtCurrentPage = () => {
  const page = currentPage.value
  const text = threeReaderRef.value?.getCurrentPageText?.() || `第${page + 1}页`
  reader.toggleBookmark(page, text)
}

function handleBookmarkClick(bm, idx) {
  if (swipedBookmarkIndex.value === idx) {
    jumpToPage(bm.page)
    swipedBookmarkIndex.value = -1
  } else {
    swipedBookmarkIndex.value = idx
  }
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

// ★ 离线缓存与文本加载
onMounted(async () => {
  // 立即缓存当前阅读页 URL
  if ('caches' in window) {
    caches.open('shanxi-reader-v5').then(cache => {
      cache.add(window.location.href).catch(() => {})
    })
  }

  try {
    const params = new URLSearchParams(window.location.search)
    const file = params.get('book')
    if (!file) throw new Error('未指定书籍')
    reader.title.value = file.replace(/\.txt$/i, '')

    let text = ''
    try {
      const res = await fetch(`/api/book/content?bookId=${encodeURIComponent(file)}`)
      if (res.ok) text = await res.text()
    } catch (e) {
      console.warn('书籍文本获取失败，将使用本地缓存')
    }

    reader.fullText.value = text
    await nextTick()
    outline.value = parseOutline(text || '')
    reader.restoreProgress()
  } catch (e) {
    reader.error.value = e.message
  } finally {
    reader.loading.value = false
  }
})

const back = async () => {
  if (threeReaderRef.value?.flipToCoverAnimated) {
    await threeReaderRef.value.flipToCoverAnimated()
  }
  window.location.href = '/reading-hut'
}
</script>


<style scoped>
/* ========== 根容器 ========== */
.reader-root {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.status-msg { text-align: center; padding: 40px; color: var(--text-secondary); }
.error { color: red; }

/* 工具栏 */
.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 0; border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.tb-title { font-weight: 600; }
.tb-actions { display: flex; gap: 8px; }
.tb-btn {
  background: var(--bg-card); border: 1px solid var(--border);
  padding: 4px 10px; border-radius: 6px; cursor: pointer;
}

/* ========== 主体区域：三列 ========== */
.reader-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  padding: 0;
  gap: 16px;
}

/* 左侧空白占位 */
.left-spacer {
  width: 460px;
  flex-shrink: 0;
}

/* 书页卡片 */
.reader-card {
  flex: 0 1 auto;
  max-width: 550px;
  min-width: 0;
  margin: 8px 0;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  position: relative;
  z-index: 1;
}

/* 书页固定尺寸 */
.three-reader-wrapper {
  width: 550px;
  height: 700px;
  flex-shrink: 0;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 8px 30px rgba(0,0,0,0.08);
}

/* 右侧面板容器 */
.right-panels {
  width: 460px;
  flex-shrink: 0;
  margin: 8px 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  position: relative;
  z-index: 2;
}

/* 笔记与侧边栏 */
.notes-panel,
.side-panel {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
  box-sizing: border-box;
  width: 100%;
  max-width: 100%;
}

/* 内部组件 */
.reader-scroller {
  height: 100%;
  overflow-y: auto;
  padding: 24px 32px;
  box-sizing: border-box;
}

/* 目录浮层 */
.outline-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.3); z-index: 20; display: flex; justify-content: center; align-items: center; }
.outline-panel { background: #fff; width: 90%; max-width: 400px; max-height: 70vh; border-radius: 12px; overflow: hidden; display: flex; flex-direction: column; }
.outline-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.outline-header h3 { margin: 0; font-size: 1.1rem; }
.outline-list { overflow-y: auto; padding: 8px; }
.outline-item { padding: 10px 12px; border-radius: 8px; cursor: pointer; font-size: 0.9rem; color: var(--text-primary); transition: background 0.15s; }
.outline-item:hover { background: var(--bg-card); }
.outline-empty { padding: 20px; text-align: center; color: var(--text-secondary); }
.outline-fade-enter-active, .outline-fade-leave-active { transition: opacity 0.2s ease; }
.outline-fade-enter-from, .outline-fade-leave-to { opacity: 0; }
.outline-tabs { display: flex; gap: 8px; }
.tab-btn {
  background: transparent; border: none; color: var(--text-secondary);
  padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 0.9rem;
}
.tab-btn.active { background: var(--bg-card); color: var(--text-primary); font-weight: 600; }
.bm-text { flex: 1; }
.bm-page { color: var(--text-secondary); font-size: 0.8rem; }

/* ========== 移动端适配 ========== */
/* ========== 移动端适配 ========== */
@media (max-width: 768px) {
  /* 根容器全屏 */
  .reader-root {
    height: 100vh;
  }

  /* 隐藏左右面板 */
  .left-spacer,
  .right-panels,
  .notes-panel,
  .side-panel {
    display: none !important;
  }

  /* 主体区域填满 */
  .reader-body {
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden;
    display: flex;
    justify-content: center;
    align-items: center;
    padding: 0 !important;
    margin: 0 !important;
    gap: 0 !important;
  }

  /* 书页外层卡片：去掉所有多余样式，完全填充 */
  .reader-card {
    width: 100% !important;
    max-width: 100% !important;
    height: 100% !important;
    margin: 0 !important;
    padding: 0 !important;
    border: none !important;
    border-radius: 0 !important;
    box-shadow: none !important;
    background: transparent !important;
    flex: 1 1 auto !important;
    display: flex;
    justify-content: center;
    align-items: center;
  }

  /* 3D 容器填满 */
  .three-reader-wrapper {
    width: 100% !important;
    height: 100% !important;
    margin: 0 !important;
    padding: 0 !important;
    border: none !important;
    border-radius: 0 !important;
    box-shadow: none !important;
    background: transparent !important;
  }

  /* 工具栏紧凑 */
  .toolbar {
    padding: 8px 12px;
    flex-shrink: 0;
  }
  .tb-title {
    font-size: 0.9rem;
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tb-btn {
    padding: 4px 8px;
    font-size: 0.8rem;
  }
  .tb-actions {
    gap: 4px;
  }

  /* 移动端面板浮层 */
  .mobile-panel-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0,0,0,0.2);
    z-index: 90;
    display: flex;
    justify-content: center;
  }
  .mobile-panel {
    background: #fff;
    width: 100%;
    height: calc(100vh - 60px);
    margin-top: 60px;
    border-radius: 0 0 16px 16px;
    overflow-y: auto;
    box-shadow: 0 8px 20px rgba(0,0,0,0.1);
  }
  .mobile-panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px;
    border-bottom: 1px solid #e2e8f0;
    font-weight: 600;
  }
  .mobile-panel-content {
    padding: 8px;
  }

  .slide-down-enter-active,
  .slide-down-leave-active {
    transition: transform 0.3s ease;
  }
  .slide-down-enter-from,
  .slide-down-leave-to {
    transform: translateY(-100%);
  }
}
@media (max-width: 768px) {
  .toolbar.mobile-hidden {
    transform: translateY(-100%);
    transition: transform 0.3s ease;
  }
}
/* 书签列表滑动删除样式 */
.bookmark-wrapper {
  position: relative;
  margin-bottom: 6px;
  border-radius: 8px;
}

.bookmark-item {
  display: flex;
  align-items: center;
  transition: transform 0.25s ease;
  transform: translateX(0);
  padding: 10px 12px;
  background: #fff;
  cursor: pointer;
  position: relative;
  z-index: 2;
  border-radius: 8px;
  overflow: hidden;
  touch-action: pan-y;
}

.bookmark-item.swiped {
  transform: translateX(-36px);
}

.bm-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding-right: 8px;
}

.bm-page {
  font-size: 0.75rem;
  color: #94a3b8;
  line-height: 1.4;
}

.bm-text {
  font-size: 0.85rem;
  color: #334155;
  line-height: 1.5;
  word-break: break-word;
}

.delete-btn {
  position: absolute;
  right: -36px;
  top: 0;
  bottom: 0;
  width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fee2e2;
  color: #ef4444;
  cursor: pointer;
  border-radius: 0;
  transition: right 0.25s ease;
  z-index: 1;
  border: none;
  outline: none;
}

.bookmark-item.swiped .delete-btn {
  right: 0;
}

</style>