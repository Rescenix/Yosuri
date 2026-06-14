<template>
  <div class="three-reader" ref="flipContainerRef">
    <div v-if="isCurrentPageBookmarked" class="paper-bookmark" @click="removeCurrentBookmark">
      <div class="bookmark-body"><span class="bookmark-char">签</span></div>
      <div class="bookmark-tassel"></div>
    </div>
    <div class="footer-info" v-if="totalPages > 0">
      <span class="time">{{ currentTime }}</span>
      <span class="page-num">{{ currentPage + 1 }} / {{ totalPages }}</span>
      <span class="remain">剩余约 {{ remainingTime }} 分钟</span>
    </div>

    <!-- 移动端页面 -->
    <template v-if="htmlPages.length > 0 && mobilePageIndex > 0">
      <Transition name="fade">
        <div
          :key="mobilePageIndex"
          class="mobile-page-view"
          v-html="htmlPages[mobilePageIndex]"
          ref="mobilePageViewRef"
        ></div>
      </Transition>
      <div class="flip-tap-area left" @touchstart.prevent="mobileFlipPrev"></div>
      <div class="flip-tap-area right" @touchstart.prevent="mobileFlipNext"></div>
    </template>

    <!-- 批注卡片 -->
    <div v-if="showCommentCard" class="comment-card" :style="commentCardStyle" @click.stop>
      <div class="comment-text" v-html="displayedComment"></div>
      <span v-if="commentTyping" class="typing-cursor">|</span>
    </div>

    <div v-if="statusMsg" class="status-overlay">
      <div class="status-box">
        <span class="status-text">{{ statusMsg }}</span>
        <div v-if="statusMsg.includes('排版')" class="progress-bar">
          <div class="progress-fill" :style="{ width: progressPercent + '%' }"></div>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="mobileShowActionMenu"
        class="action-menu mobile-action-menu"
        :style="mobileActionMenuStyle"
        @click.stop
        @touchstart.stop
      >
        <div class="menu-item" @click="mobileChooseComment()"><Icon icon="ph:pen" width="16" /><span>注释</span></div>
        <div class="menu-item" @click="mobileChooseSearch()"><Icon icon="ph:magnifying-glass" width="16" /><span>搜索</span></div>
        <div class="menu-item" @click="mobileCopySelection()"><Icon icon="ph:copy" width="16" /><span>复制</span></div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useReader } from './useReader.js'
import { useMobileReader } from './useMobileReader.js'
import { useReadingStats } from './useReadingStats.js'
import { useBookmarks } from './useBookmarks.js'
import { useAnnotation } from './useAnnotation.js'
import { useMobileSelection } from './useMobileSelection.js'

const props = defineProps({ reader: Object })

const flipContainerRef = ref(null)
const mobilePageViewRef = ref(null)
const statusMsg = ref('正在准备...')
const progressPercent = ref(0)

const width = ref(Math.floor(window.innerWidth * 0.92))
const height = ref(Math.floor(window.innerHeight * 0.85))

function updatePageSize() {
  width.value = Math.floor(window.innerWidth * 0.92)
  height.value = Math.floor(window.innerHeight * 0.85)
}

const totalPagesRef = ref(0)
const currentPageRef = ref(0)

const mobile = useMobileReader(
  flipContainerRef,
  props.reader,
  statusMsg,
  progressPercent,
  totalPagesRef,
  currentPageRef,
  () => {}
)

const { htmlPages, mobilePageIndex, mobileFlipPrev, mobileFlipNext, initMobileView } = mobile

const totalPages = computed(() => Math.max(0, htmlPages.value.length - 2))
const currentPage = computed(() => Math.max(0, mobilePageIndex.value - 1))

// 每日阅读记录
const saveDailyProgress = () => {
  const today = new Date().toLocaleDateString()
  const stored = localStorage.getItem('shanxi_reading_progress')
  let records = stored ? JSON.parse(stored) : []
  const existing = records.find(r => r.date === today)
  if (existing) {
    existing.pages += 1
  } else {
    records.push({ date: today, pages: 1 })
  }
  localStorage.setItem('shanxi_reading_progress', JSON.stringify(records))
}

watch(currentPage, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal > oldVal) {
    saveDailyProgress()
  }
})

const textProvider = () => props.reader.fullText.value || ''
const stats = useReadingStats(textProvider, currentPage, totalPages, flipContainerRef)
const { currentTime, recordPageTurn, startClock, destroy: destroyStats } = stats

const remainingTime = computed(() => {
  const total = totalPages.value
  const cur = currentPage.value
  if (total === 0 || cur >= total) return 0
  return Math.ceil((total - cur) * 2)
})

const { isCurrentPageBookmarked, removeCurrentBookmark } = useBookmarks(
  props.reader,
  currentPage,
  flipContainerRef,
  null
)

// 批注（共享卡片、生成等）
const annotation = useAnnotation(flipContainerRef, currentPage)
const {
  showCommentCard,
  commentCardStyle,
  displayedComment,
  commentTyping,
  generateComment,
  showResultCard,
  highlightText,
  closeCard
} = annotation

// 移动端选区菜单（传入批注相关函数）
const {
  mobileShowActionMenu,
  mobileActionMenuStyle,
  mobileChooseComment,
  mobileChooseSearch,
  mobileCopySelection,
  clearMobileSelection
} = useMobileSelection(flipContainerRef, {
  generateComment,
  showResultCard,
  highlightText,
  closeCard
})

// 跳转章节
function mobileJumpToChapter(title) {
  const cleanTitle = title.replace(/\s+/g, '')
  for (let i = 1; i < htmlPages.value.length - 1; i++) {
    if (htmlPages.value[i].replace(/\s+/g, '').includes(cleanTitle)) {
      mobilePageIndex.value = i
      nextTick(() => {
        const view = mobilePageViewRef.value
        if (view) {
          const headings = view.querySelectorAll('h1, h2, h3, h4')
          for (const heading of headings) {
            if (heading.textContent.includes(title.trim())) {
              heading.scrollIntoView({ behavior: 'smooth', block: 'start' })
              return
            }
          }
        }
      })
      break
    }
  }
}

function getCurrentPageText() {
  const el = document.querySelector('.mobile-page-view')
  if (!el) return ''
  const heading = el.querySelector('h1, h2, h3, h4')
  if (heading) return heading.textContent.trim().slice(0, 80)
  return el.textContent.trim().slice(0, 50)
}

defineExpose({
  flipToPage: (page) => {
    if (page >= 0 && page < htmlPages.value.length) mobilePageIndex.value = page
  },
  jumpToChapter: mobileJumpToChapter,
  getCurrentPageText,
  currentPage,
  totalPages
})

let resizeTimer
const handleResize = () => {
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    updatePageSize()
    initMobileView()
  }, 300)
}

// 监听全文本变化，触发排版
watch(
  () => props.reader.fullText.value,
  (newVal) => {
    if (newVal) {
      initMobileView()
    }
  }
)

// 监听字体大小变化，触发重新排版
watch(
  () => props.reader.fontSize.value,
  () => {
    if (props.reader.fullText.value) {
      initMobileView()
    }
  }
)
function onDocumentClick(e) {
  if (showCommentCard.value) {
    const card = document.querySelector('.comment-card')
    const menu = document.querySelector('.action-menu')
    if (card && !card.contains(e.target) && (!menu || !menu.contains(e.target))) {
      closeCard()
    }
  }
}
onMounted(async () => {
   document.addEventListener('click', onDocumentClick)
  window.addEventListener('resize', handleResize)
  startClock()
  await nextTick()
  if (props.reader.fullText.value) {
    initMobileView()
  }
})

onBeforeUnmount(() => {
    document.removeEventListener('click', onDocumentClick)
  window.removeEventListener('resize', handleResize)
  destroyStats()
})
</script>

<style src="./ThreeReader.css"></style>