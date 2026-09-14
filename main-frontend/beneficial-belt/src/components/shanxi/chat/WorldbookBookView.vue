<template>
  <div class="wb-book-view">
    <!-- 空书：引导去写第一条设定 -->
    <div v-if="!fullText.trim()" class="wb-book-empty">
      <Icon icon="ph:book-open-text" width="42" color="#b0b0b0" />
      <p>{{ tr('世界书还是一本空书') }}</p>
      <p class="wb-book-empty-sub">{{ tr('切换到「条目」模式，写几条设定（含触发词），它就会出现在书页里') }}</p>
    </div>

    <!-- 书本模式 -->
    <template v-else>
      <div class="wb-book-toolbar">
        <div class="wb-book-toolbar-left">
          <button class="wb-book-tb-btn" type="button" @click="changeFontSize(-1)" title="缩小字体">A-</button>
          <span class="wb-book-font-label">{{ currentFontSize }}px</span>
          <button class="wb-book-tb-btn" type="button" @click="changeFontSize(1)" title="放大字体">A+</button>
        </div>
        <div class="wb-book-toolbar-right">
          <button class="wb-book-tb-btn" type="button" @click="toggleBookmark" :title="isCurrentBookmarked ? tr('取消书签') : tr('标记这页')">
            <Icon :icon="isCurrentBookmarked ? 'ph:bookmark-simple-fill' : 'ph:bookmark-simple'" width="16" />
          </button>
          <span class="wb-book-page-indicator">{{ currentPage + 1 }} / {{ totalPages }}</span>
        </div>
      </div>

      <!-- 书页主体：点击左右半区翻页（复用 reader-standalone 的交互） -->
      <div
        ref="pageAreaRef"
        class="wb-book-page-area"
        @click="handlePageClick"
        @touchstart="onTouchStart"
        @touchend="onTouchEnd"
      >
        <div class="wb-book-page" :style="{ fontSize: currentFontSize + 'px' }">
          <div v-html="currentPageHtml" class="wb-book-page-content"></div>
        </div>
        <div class="wb-book-nav-zone left" @click.stop="prevPage"></div>
        <div class="wb-book-nav-zone right" @click.stop="nextPage"></div>
      </div>
    </template>
  </div>
</template>

<script setup>
// WorldbookBookView.vue —— 世界书「书本视图」。
//
// 复用 re1/reader-standalone 的分页阅读心智（Reader.vue）：全文按估算字符数
// 分页、点击左右半区翻页、字体调节、书签。数据源换成世界书条目——
// 常驻条目在前，其余按 order 排序，每条正文带【标题】头。
//
// 世界书是离散设定条目，不是连续长文本，所以不搬 FlipReader 的 Three.js
// 3D 翻页 + Pretext 像素级分页（那套为整本小说设计，这里 80% 功能闲置）。
// 平面分页 + 条目标题分段，翻起来就是一本「设定集」。

import { ref, computed, nextTick } from 'vue'
import { tr } from '../../../composables/useI18n.js'

const props = defineProps({
  // 条目数组（WorldBook.Entries 的形状）
  entries: { type: Array, default: () => [] },
})

// ==================== 状态 ====================
const pageAreaRef = ref(null)
const currentPage = ref(0)
const fontSizeIndex = ref(1)
const fontSizes = [14, 17, 20, 23, 26]
const currentFontSize = computed(() => fontSizes[fontSizeIndex.value])

// 触摸滑动翻页
let touchStartX = 0
let touchStartY = 0

// ==================== 拼书 ====================
// 常驻条目在前（世界观底色先读），其余按 order 排。每条一行标题 + 正文，
// 空键条目标注「常驻」，有触发词的标注触发词（翻页时一眼看到谁在触发）。
const fullText = computed(() => {
  const es = [...props.entries].filter(e => e && (e.content || '').trim())
  es.sort((a, b) => {
    const ac = a.constant ? 0 : 1
    const bc = b.constant ? 0 : 1
    if (ac !== bc) return ac - bc
    return (a.order || 0) - (b.order || 0)
  })
  const parts = []
  for (const e of es) {
    let head = `【${e.name || '未命名条目'}】`
    if (e.constant) head += '（常驻）'
    else if (e.keys && e.keys.length) head += '（触发词：' + e.keys.join('、') + '）'
    parts.push(head + '\n' + (e.content || '').trim())
  }
  return parts.join('\n\n')
})

// ==================== 分页 ====================
const pages = ref([])
const totalPages = computed(() => pages.value.length)
const currentPageHtml = computed(() => pages.value[currentPage.value] || '')

function buildPages() {
  const text = fullText.value
  if (!text.trim()) { pages.value = ['']; return }
  const charPerPage = estimateCharsPerPage()
  const result = []
  let start = 0
  while (start < text.length) {
    let end = start + charPerPage
    if (end >= text.length) {
      result.push(text.slice(start))
      break
    }
    const segment = text.slice(start, end + 1)
    const lastNewline = segment.lastIndexOf('\n')
    if (lastNewline > 0 && lastNewline > charPerPage * 0.6) {
      end = start + lastNewline
    } else {
      const lastSpace = segment.lastIndexOf(' ')
      if (lastSpace > charPerPage * 0.7) end = start + lastSpace
    }
    result.push(text.slice(start, end).trim())
    start = end
  }
  pages.value = result
  if (currentPage.value >= pages.value.length) currentPage.value = Math.max(0, pages.value.length - 1)
}

function estimateCharsPerPage() {
  const area = pageAreaRef.value
  if (!area) return 600
  const width = area.clientWidth - 96
  const height = area.clientHeight - 90
  const charWidth = currentFontSize.value * 0.62
  const lineHeight = currentFontSize.value * 1.85
  const charsPerLine = Math.floor(width / charWidth)
  const linesPerPage = Math.floor(height / lineHeight)
  return Math.max(200, charsPerLine * linesPerPage - 8)
}

// ==================== 翻页 ====================
const nextPage = () => { if (currentPage.value < totalPages.value - 1) currentPage.value++ }
const prevPage = () => { if (currentPage.value > 0) currentPage.value-- }

function handlePageClick(e) {
  const area = pageAreaRef.value
  if (!area) return
  const rect = area.getBoundingClientRect()
  if ((e.clientX - rect.left) < rect.width / 2) prevPage()
  else nextPage()
}

function onTouchStart(e) {
  touchStartX = e.touches[0].clientX
  touchStartY = e.touches[0].clientY
}
function onTouchEnd(e) {
  const dx = e.changedTouches[0].clientX - touchStartX
  const dy = e.changedTouches[0].clientY - touchStartY
  if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy)) {
    if (dx < 0) nextPage()
    else prevPage()
  }
}

// ==================== 字体调节 ====================
function changeFontSize(delta) {
  const ni = fontSizeIndex.value + delta
  if (ni < 0 || ni >= fontSizes.length) return
  fontSizeIndex.value = ni
  nextTick(() => { buildPages() })
}

// ==================== 书签 ====================
const STORAGE_KEY = 'wb-bookmarks'
const bookmarks = ref([])
const isCurrentBookmarked = computed(() => bookmarks.value.includes(currentPage.value))

function loadBookmarks() {
  try { bookmarks.value = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]') } catch { bookmarks.value = [] }
}
function toggleBookmark() {
  const i = bookmarks.value.indexOf(currentPage.value)
  if (i >= 0) bookmarks.value.splice(i, 1)
  else bookmarks.value.push(currentPage.value)
  localStorage.setItem(STORAGE_KEY, JSON.stringify(bookmarks.value))
}

// ==================== 条目变化重分页 ====================
import { watch } from 'vue'
watch(fullText, () => { nextTick(() => { buildPages() }) })

loadBookmarks()
nextTick(() => { buildPages() })
</script>
