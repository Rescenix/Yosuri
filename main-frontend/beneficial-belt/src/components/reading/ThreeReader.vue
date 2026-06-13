<template>
  <div class="three-reader" ref="flipContainerRef">
    <EmotionGlow />
    <div v-if="isCurrentPageBookmarked" class="paper-bookmark" @click="removeCurrentBookmark">
      <div class="bookmark-body"><span class="bookmark-char">签</span></div>
      <div class="bookmark-tassel"></div>
    </div>
    <div class="footer-info" v-if="totalPages > 0">
      <span class="time">{{ currentTime }}</span>
      <span class="page-num">{{ currentPage + 1 }} / {{ totalPages }}</span>
      <span class="remain">剩余约 {{ remainingTime }} 分钟</span>
    </div>

    <div v-if="showCommentCard" class="comment-card" :style="commentCardStyle" @click.stop>
      <div class="comment-text">{{ displayedComment }}</div>
      <span v-if="commentTyping" class="typing-cursor">|</span>
      <button class="close-card" @click="closeCard">
        <Icon icon="ph:x" width="14" />
      </button>
    </div>

    <template v-if="isMobile && htmlPages.length > 0">
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

    <template v-if="!isMobile">
      <div class="flip-tap-area left" @click.stop="flipPrev"></div>
      <div class="flip-tap-area right" @click.stop="flipNext"></div>
    </template>

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
    v-if="isMobile ? mobileShowActionMenu : showActionMenu"
    class="action-menu"
    :class="{ 'mobile-action-menu': isMobile }"
    :style="isMobile ? mobileActionMenuStyle : actionMenuStyle"
    @click.stop
    @mousedown.prevent
    @touchstart.stop
  >
    <!-- 注释按钮 -->
    <div 
      class="menu-item" 
      :class="{ 'selected': activeMenuItem === 'comment' }"
      @click="isMobile ? mobileChooseComment() : desktopChooseComment()"
    >
      <Icon icon="ph:pen" width="16" /><span>注释</span>
    </div>
    <!-- 搜索按钮 -->
    <div 
      class="menu-item" 
      :class="{ 'selected': activeMenuItem === 'search' }"
      @click="isMobile ? mobileChooseSearch() : desktopChooseSearch()"
    >
      <Icon icon="ph:magnifying-glass" width="16" /><span>搜索</span>
    </div>
    <!-- 复制按钮 -->
    <div 
      class="menu-item" 
      :class="{ 'selected': activeMenuItem === 'copy' }"
      @click="isMobile ? mobileCopySelection() : copyDesktopSelection()"
    >
      <Icon icon="ph:copy" width="16" /><span>复制</span>
    </div>
  </div>
</Teleport>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount, nextTick, computed } from 'vue'
import { Icon } from '@iconify/vue'
import EmotionGlow from './EmotionGlow.vue'
import { usePageFlip } from './usePageFlip.js'
import { useReadingStats } from './useReadingStats.js'
import { useBookmarks } from './useBookmarks.js'
import { useAnnotation } from './useAnnotation.js'
import { useMobileReader } from './useMobileReader.js'
import { useMobileSelection } from '../../composables/useMobileSelection.js'
import { useWebHighlighter } from '../../composables/useWebHighlighter.js'

const props = defineProps({ reader: Object })
const flipContainerRef = ref(null)
const mobilePageViewRef = ref(null)
const statusMsg = ref('正在准备...')
const progressPercent = ref(0)
let currentFontSize = null
const isMobile = ref(window.innerWidth <= 768)
const width = ref(550)
const height = ref(700)

function updatePageSize() {
  isMobile.value = window.innerWidth <= 768
  if (isMobile.value) {
    width.value = Math.floor(window.innerWidth * 0.92)
    height.value = Math.floor(window.innerHeight * 0.85)
  } else {
    width.value = 550
    height.value = 700
  }
}

let resizeTimer
const handleResize = () => {
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    const oldW = width.value, oldH = height.value
    updatePageSize()
    if (width.value !== oldW || height.value !== oldH) reInit()
  }, 300)
}

const {
  currentPage, totalPages, pageFlip, initFlip: desktopInitFlip, destroyFlip,
  flipToPage: desktopFlipToPage, flipToPhysicalPage, jumpToChapter: desktopJumpToChapter,
  flipToCoverAnimated, flipPrev, flipNext,
} = usePageFlip(flipContainerRef, props.reader, width, height, statusMsg, progressPercent)

// 高亮器
const { 
  init: initHighlighter, 
  restoreHighlights, 
  createHighlightFromRange,
  addCommentToHighlight,
  setCurrentPageForLatestHighlight,
  destroy: destroyHighlighter 
} = useWebHighlighter()

// 移动端 reader
const mobile = useMobileReader(
  flipContainerRef, props.reader, statusMsg, progressPercent,
  totalPages, currentPage, restoreHighlights
)
const { htmlPages, mobilePageIndex, mobileFlipPrev, mobileFlipNext, initMobileView } = mobile

const textProvider = () => props.reader.fullText.value || ''

const realTotalPages = computed(() => {
  if (isMobile.value) {
    return Math.max(0, htmlPages.value.length - 2)
  }
  return totalPages.value
})

const stats = useReadingStats(textProvider, currentPage, realTotalPages, flipContainerRef)
const { currentTime, remainingTime, recordPageTurn, startClock, destroy: destroyStats } = stats

const {
  isCurrentPageBookmarked,
  getCurrentPageText: desktopGetCurrentPageText,
  removeCurrentBookmark
} = useBookmarks(props.reader, currentPage, flipContainerRef, pageFlip)

function mobileGetCurrentPageText() {
  const el = document.querySelector('.mobile-page-view')
  if (!el) return ''
  const heading = el.querySelector('h1, h2, h3, h4')
  if (heading) return heading.textContent.trim().slice(0, 80)
  return el.textContent.trim().slice(0, 50)
}

const annotation = useAnnotation(flipContainerRef, currentPage)
const {
  showCommentCard, commentCardStyle, displayedComment, commentTyping,
  showActionMenu, actionMenuStyle, onMouseUp, closeCard,
  chooseSearch: desktopChooseSearch,
  generateComment, showResultCard,
} = annotation

// ========== 桌面端注释（使用高亮器） ==========
async function desktopChooseComment() {
  const selection = window.getSelection()
  const text = selection.toString().trim()
  if (!text) return
  const range = selection.getRangeAt(0).cloneRange()
  
  const highlightId = await createHighlightFromRange(range)
  if (!highlightId) return
  
  const comment = await generateComment(text)
  addCommentToHighlight(highlightId, comment)
  setCurrentPageForLatestHighlight(currentPage.value)
  
  const rect = range.getBoundingClientRect()
  showResultCard(text, rect, comment, false)  // 不重复保存
  // 关闭菜单（简单处理）
  showActionMenu.value = false
}

// ========== 移动端选区菜单（重写） ==========
const {
  mobileShowActionMenu, mobileActionMenuStyle, clearMobileSelection,
  mobileSelectedText, mobileSelectedRange,activeMenuItem,   // 添加这一行
} = useMobileSelection(flipContainerRef, {})

// 移动端搜索（保持不变）
async function mobileChooseSearch() {
  const text = mobileSelectedText.value
  const range = mobileSelectedRange.value
  if (!text || !range) return
  const rect = range.getBoundingClientRect()
  clearMobileSelection()
  closeCard()
  try {
    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: `帮我搜索一下“${text}”` })
    })
    if (!res.ok) { showResultCard(text, rect, '搜索服务暂不可用，请稍后重试。', false); return }
    const data = await res.json()
    const reply = data.reply || data.message || data.content || '暂无搜索结果。'
    showResultCard(text, rect, reply, false)
  } catch (e) {
    showResultCard(text, rect, '搜索失败，请重试。', false)
  }
}

// 移动端复制
function mobileCopySelection() {
  const text = mobileSelectedText.value
  if (!text) return
  navigator.clipboard?.writeText(text)?.catch(() => {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.left = '-9999px'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  })
  clearMobileSelection()
}

// 移动端注释（使用高亮器）
async function mobileChooseComment() {
  const text = mobileSelectedText.value
  const range = mobileSelectedRange.value
  if (!text || !range) return
  
  const highlightId = await createHighlightFromRange(range)
  if (!highlightId) return
  
  const comment = await generateComment(text)
  addCommentToHighlight(highlightId, comment)
  setCurrentPageForLatestHighlight(currentPage.value)
  
  const rect = range.getBoundingClientRect()
  showResultCard(text, rect, comment, false)
  clearMobileSelection()
  closeCard()
}

// 桌面端复制
function copyDesktopSelection() {
  const text = window.getSelection()?.toString().trim()
  if (!text) return
  navigator.clipboard.writeText(text).catch(() => {
    const textarea = document.createElement('textarea')
    textarea.value = text
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  })
}

watch(isMobile, (val) => { if (!val) clearMobileSelection() })

function highlightOnPage(text, targetIndex) {
  // 保留空函数，避免报错
}

function handleSidebarFlip(pageIndex, quote) {
  if (isMobile.value) {
    if (pageIndex >= 0 && pageIndex < htmlPages.value.length) {
      mobilePageIndex.value = pageIndex
      currentPage.value = pageIndex
      nextTick(() => restoreHighlights())
    }
  } else {
    desktopFlipToPage(pageIndex, quote, highlightOnPage)
  }
}

function bindFlipEvent(flip) {
  if (!flip) return
  flip.on('flip', (e) => {
    currentPage.value = e.data ?? flip.getCurrentPageIndex()
    recordPageTurn()
    nextTick(() => restoreHighlights())
  })
}

function onKeyDown(e) {
  if (!pageFlip) return
  if (e.key === 'ArrowRight') pageFlip.flipNext()
  else if (e.key === 'ArrowLeft') pageFlip.flipPrev()
}

async function mobileJumpToChapter(title) {
  const cleanTitle = title.replace(/\s+/g, '')
  for (let i = 1; i < htmlPages.value.length - 1; i++) {
    const pageContent = htmlPages.value[i].replace(/\s+/g, '')
    if (pageContent.includes(cleanTitle)) {
      mobilePageIndex.value = i
      currentPage.value = i
      await nextTick()
      const view = mobilePageViewRef.value || flipContainerRef.value?.querySelector('.mobile-page-view')
      if (view) {
        const walker = document.createTreeWalker(view, NodeFilter.SHOW_TEXT)
        let node
        while ((node = walker.nextNode())) {
          if (node.textContent.includes(title.trim())) {
            const range = document.createRange()
            range.selectNodeContents(node)
            const rect = range.getBoundingClientRect()
            view.scrollTo({ top: rect.top + view.scrollTop - 80, behavior: 'smooth' })
            return
          }
        }
        const headings = view.querySelectorAll('h1, h2, h3, h4, h5, h6')
        for (const heading of headings) {
          if (heading.textContent.includes(title.trim())) {
            heading.scrollIntoView({ behavior: 'smooth', block: 'start' })
            return
          }
        }
      }
      break
    }
  }
}

function jumpToChapter(title) {
  if (isMobile.value) mobileJumpToChapter(title)
  else desktopJumpToChapter(title)
}

async function reInit() {
  destroyFlip()
  statusMsg.value = '正在准备...'
  if (isMobile.value) {
    await initMobileView()
    await nextTick()
    const container = document.querySelector('.mobile-page-view')
    if (container) {
      initHighlighter(container)
      restoreHighlights()
    }
  } else {
    try {
      const flip = await desktopInitFlip()
      if (flip) {
        bindFlipEvent(flip)
        statusMsg.value = ''
        await nextTick()
        const container = flipContainerRef.value?.querySelector('.flip-page')
        if (container) {
          initHighlighter(container)
          restoreHighlights()
        }
      } else statusMsg.value = '暂无内容'
    } catch (e) {
      statusMsg.value = '加载失败，请重试'
    }
  }
}

watch(() => props.reader.fontSize.value, (v) => {
  if (v !== currentFontSize) {
    currentFontSize = v
    reInit()
  }
})
watch(() => props.reader.fullText.value, () => reInit())

let blockCtx
onMounted(async () => {
  blockCtx = (e) => e.preventDefault()
  document.addEventListener('contextmenu', blockCtx)
  updatePageSize()
  startClock()
  await nextTick()

  if (isMobile.value) {
    await initMobileView()
    const saved = parseInt(localStorage.getItem(`${props.reader.title.value}_pos`) || '1')
    if (saved > 1 && saved < htmlPages.value.length) {
      mobilePageIndex.value = saved
      currentPage.value = Math.max(0, saved - 1)
    } else if (htmlPages.value.length > 1) {
      mobilePageIndex.value = 1
      currentPage.value = 0
    }
    await nextTick()
    const container = document.querySelector('.mobile-page-view')
    if (container) {
      initHighlighter(container)
      restoreHighlights()
    }
  } else {
    try {
      const flip = await desktopInitFlip()
      if (flip) {
        bindFlipEvent(flip)
        statusMsg.value = ''
        await nextTick()
        const container = flipContainerRef.value?.querySelector('.flip-page')
        if (container) {
          initHighlighter(container)
          restoreHighlights()
        }
      } else statusMsg.value = '暂无内容'
    } catch (e) {
      statusMsg.value = '加载失败，请重试'
    }
    flipContainerRef.value?.addEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('keydown', onKeyDown)
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  document.removeEventListener('contextmenu', blockCtx)
  document.removeEventListener('keydown', onKeyDown)
  if (!isMobile.value) {
    flipContainerRef.value?.removeEventListener('mouseup', onMouseUp)
  }
  destroyFlip()
  destroyStats()
  clearMobileSelection()
  destroyHighlighter()
})

function saveDailyProgress() {
  const today = new Date().toLocaleDateString()
  const stored = localStorage.getItem('shanxi_reading_progress')
  let records = stored ? JSON.parse(stored) : []
  const existing = records.find(r => r.date === today)
  if (existing) {
    existing.pages += 1
  } else {
    records.push({ date: today, pages: 1, minutes: 0 })
  }
  localStorage.setItem('shanxi_reading_progress', JSON.stringify(records))
}

watch(currentPage, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal > oldVal) {
    saveDailyProgress()
  }
})

defineExpose({
  flipToPage: handleSidebarFlip,
  flipToPhysicalPage,
  jumpToChapter,
  currentPage,
  getCurrentPageText: () => {
    return isMobile.value ? mobileGetCurrentPageText() : desktopGetCurrentPageText?.() || ''
  },
  flipToCoverAnimated,
})
</script>

<style src="./ThreeReader.css"></style>