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
          <button v-if="isMobile" class="tb-btn" @click="openMobilePanel('sidebar')">
            <Icon icon="ph:list" width="18" />
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

            <!-- 目录列表 -->
            <div v-show="outlineTab === 'outline'" class="outline-list">
              <div v-for="(item, idx) in outline" :key="idx" class="outline-item" @click="jumpToChapter(item)">
                {{ item.title }}
              </div>
              <div v-if="outline.length === 0" class="outline-empty">未识别到章节标题</div>
            </div>

            <!-- 书签列表（滑动删除） -->
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
                  <!-- 修改为 @touchstart.prevent.stop 直接触发删除 -->
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
            <NotesPanel v-if="activeMobilePanel === 'notes'" />
            <SidePanel v-else :threeReaderRef="threeReaderRef" />
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

const activeMobilePanel = ref(null)
const reader = useReader()
const threeReaderRef = ref(null)
const showOutline = ref(false)
const outline = ref([])
const outlineTab = ref('outline')
const swipedBookmarkIndex = ref(-1)

// 移动端滑动相关
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

// 桌面端点击切换滑动状态
function handleBookmarkClick(bm, idx) {
  if (swipedBookmarkIndex.value === idx) {
    jumpToPage(bm.page)
    swipedBookmarkIndex.value = -1
  } else {
    swipedBookmarkIndex.value = idx
  }
}

// 移动端触摸事件
function onBookmarkTouchStart(e, idx) {
  touchStartX[idx] = e.touches[0].clientX
}

function onBookmarkTouchMove() {}

function onBookmarkTouchEnd(e, bm, idx) {
  // 如果触摸发生在删除按钮上，不做任何处理（避免关闭滑动或跳转）
  if (e.target.closest('.delete-btn')) return

  const startX = touchStartX[idx]
  delete touchStartX[idx]
  if (startX === undefined) return

  const endX = e.changedTouches[0].clientX
  const dx = endX - startX

  if (dx < -SWIPE_THRESHOLD) {
    // 左滑显示删除
    swipedBookmarkIndex.value = idx
  } else if (dx > SWIPE_THRESHOLD) {
    // 右滑关闭删除
    swipedBookmarkIndex.value = -1
  } else {
    // 点击行为
    if (swipedBookmarkIndex.value === idx) {
      // 已处于滑动状态，点击关闭删除（不跳转）
      swipedBookmarkIndex.value = -1
    } else {
      // 未滑动，直接跳转
      jumpToPage(bm.page)
      swipedBookmarkIndex.value = -1
    }
  }
}

function deleteBookmark(idx) {
  // 响应式删除
  reader.bookmarks.value = reader.bookmarks.value.filter((_, i) => i !== idx)
  localStorage.setItem('shanxi_bookmarks', JSON.stringify(reader.bookmarks.value))
  swipedBookmarkIndex.value = -1
}

function jumpToPage(page) {
  if (threeReaderRef.value?.flipToPage) {
    threeReaderRef.value.flipToPage(page)
  }
  showOutline.value = false
}

function jumpToChapter(item) {
  if (threeReaderRef.value?.jumpToChapter) {
    threeReaderRef.value.jumpToChapter(item.title)
  }
  showOutline.value = false
}

onMounted(async () => {
  try {
    const params = new URLSearchParams(window.location.search)
    const file = params.get('book')
    if (!file) throw new Error('未指定书籍')
    reader.title.value = file.replace(/\.txt$/i, '')

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

const back = async () => {
  if (threeReaderRef.value?.flipToCoverAnimated) {
    await threeReaderRef.value.flipToCoverAnimated()
  }
  window.location.href = '/reading-hut'
}
</script>

<style>
@import './ReaderView.css';
</style>

<style scoped>
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