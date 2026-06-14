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

    <!-- 批注卡片（支持 HTML 渲染，用于旋转动画） -->
    <div v-if="showCommentCard" class="comment-card" :style="commentCardStyle" @click.stop>
      <div class="comment-text" v-html="displayedComment"></div>
      <span v-if="commentTyping" class="typing-cursor">|</span>
     
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
        <div class="menu-item" @click="isMobile ? mobileChooseComment() : desktopChooseComment()">
          <Icon icon="ph:pen" width="16" /><span>注释</span>
        </div>
        <div class="menu-item" @click="isMobile ? mobileChooseSearch() : desktopChooseSearch()">
          <Icon icon="ph:magnifying-glass" width="16" /><span>搜索</span>
        </div>
        <div class="menu-item" @click="isMobile ? mobileCopySelection() : copyDesktopSelection()">
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

const mobile = useMobileReader(
  flipContainerRef, props.reader, statusMsg, progressPercent,
  totalPages, currentPage, () => {}
)
const { htmlPages, mobilePageIndex, mobileFlipPrev, mobileFlipNext, initMobileView } = mobile

const textProvider = () => props.reader.fullText.value || ''
const realTotalPages = computed(() => {
  if (isMobile.value) return Math.max(0, htmlPages.value.length - 2)
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
  generateComment, showResultCard, highlightText,
} = annotation

// ========== 桌面端注释（菜单消失后再显示卡片） ==========
async function desktopChooseComment() {
  showActionMenu.value = false
  const selection = window.getSelection()
  const text = selection.toString().trim()
  if (!text) return
  const range = selection.getRangeAt(0).cloneRange()
  
  highlightText(text, range)
  const rect = range.getBoundingClientRect()
  // ★ 加上第五个参数 true
  showResultCard(text, rect, '<span class="loading-dots">杉汐正在思考<span class="dots">...</span></span>', false, true)
  const comment = await generateComment(text)
  showResultCard(text, rect, comment, true)
}

// ========== 移动端选区菜单 ==========
const {
  mobileShowActionMenu, mobileActionMenuStyle, clearMobileSelection,
  mobileChooseComment, mobileChooseSearch, mobileCopySelection,
} = useMobileSelection(flipContainerRef, {
  generateComment,
  showResultCard,
  closeCard,
  highlightText,
})
// 点击页面任意位置关闭卡片（但不影响菜单）
function onDocumentClick(e) {
  const card = document.querySelector('.comment-card')
  const menu = document.querySelector('.action-menu')
  if (!card) return
  if (!card.contains(e.target) && (!menu || !menu.contains(e.target))) {
    closeCard()
  }
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

function highlightOnPage(text, targetIndex) { /* 备用 */ }

function handleSidebarFlip(pageIndex, quote) {
  if (isMobile.value) {
    if (pageIndex >= 0 && pageIndex < htmlPages.value.length) {
      mobilePageIndex.value = pageIndex
      currentPage.value = pageIndex
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
  } else {
    try {
      const flip = await desktopInitFlip()
      if (flip) {
        bindFlipEvent(flip)
        statusMsg.value = ''
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
  // 恢复进度已经在 initMobileView 内部完成，无需再手动处理
} else {
    try {
      const flip = await desktopInitFlip()
      if (flip) {
        bindFlipEvent(flip)
        statusMsg.value = ''
      } else statusMsg.value = '暂无内容'
    } catch (e) {
      statusMsg.value = '加载失败，请重试'
    }
    flipContainerRef.value?.addEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('keydown', onKeyDown)

  document.addEventListener('click', onDocumentClick)
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  document.removeEventListener('click', onDocumentClick)
  document.removeEventListener('contextmenu', blockCtx)
  document.removeEventListener('keydown', onKeyDown)
  if (!isMobile.value) {
    flipContainerRef.value?.removeEventListener('mouseup', onMouseUp)
  }
  destroyFlip()
  destroyStats()
  clearMobileSelection()
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