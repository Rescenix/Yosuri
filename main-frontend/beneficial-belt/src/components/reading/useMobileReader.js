// src/components/reading/useMobileReader.js
import { ref, nextTick } from 'vue'
import { getCachedPages, setCachedPages } from './cachePagination.js'
import { savePage, getPage } from '../../composables/usePageCache.js'

export function useMobileReader(flipContainerRef, reader, statusMsg, progressPercent, totalPages, currentPage) {
  const htmlPages = ref([])
  const mobilePageIndex = ref(0)
  const mobileSelectedText = ref('')
  const mobileSelectedRange = ref(null)
  const mobileSelectionStyle = ref({})

  // ─── HTML 辅助函数 ──────────────────────────
  function escapeHtml(str) {
    const div = document.createElement('div')
    div.textContent = str
    return div.innerHTML
  }

  function createCoverHTML(title) {
    const safe = escapeHtml(title)
    return `<div style="width:100%;height:100%;background:linear-gradient(135deg,#1e2a3a,#2c3e50);display:flex;flex-direction:column;justify-content:center;align-items:center;color:#e8d5b7;font-family:Georgia,serif;"><h1>${safe}</h1><p>杉汐注</p></div>`
  }

  function createBackHTML() {
    return `<div style="width:100%;height:100%;background:linear-gradient(135deg,#1e2a3a,#2c3e50);display:flex;justify-content:center;align-items:center;color:#e8d5b7;">封底</div>`
  }

  // ─── 翻页 ────────────────────────────────
  function mobileFlipPrev() {
    if (mobilePageIndex.value > 0) {
      mobilePageIndex.value--
      currentPage.value = mobilePageIndex.value
    }
  }

  function mobileFlipNext() {
    if (mobilePageIndex.value < htmlPages.value.length - 1) {
      mobilePageIndex.value++
      currentPage.value = mobilePageIndex.value
    }
  }

  // ─── 选区处理（移动端）—— 已由 useMobileSelection 接管，保留空函数以防引用 ──
  function updateMobileSelection() {}
  function onMobileTouchEnd(event) {}

  // ─── 初始化移动端视图（纯客户端排版 + 本地缓存）───────────
  async function initMobileView() {
    const text = reader.fullText.value || ''
    const fontSize = reader.fontSize.value
    const bookId = reader.title.value || 'unknown'

    await nextTick()
    const w = flipContainerRef.value?.clientWidth
    const h = flipContainerRef.value?.clientHeight
    if (!w || !h) {
      setTimeout(initMobileView, 100)
      return
    }

    // 1. 尝试从 IndexedDB 加载全本缓存（如果之前已排版并缓存过）
    const cached = await getCachedPages(bookId, fontSize, w, h)
    if (cached && cached.length > 0) {
      htmlPages.value = cached
      totalPages.value = cached.length
      mobilePageIndex.value = 1
      currentPage.value = 1
      statusMsg.value = ''
      progressPercent.value = 100
      return
    }

    // 2. 无缓存，执行客户端精确排版（exactPaginate）
    statusMsg.value = '正在排版... 0%'
    progressPercent.value = 0

    const { exactPaginate } = await import('./ExactPaginator.js')
    const bodyPages = await exactPaginate(text, fontSize, w, h, (pct) => {
      progressPercent.value = pct
    })

    const coverHTML = createCoverHTML(reader.title.value)
    const backHTML = createBackHTML()
    const fullPages = [coverHTML, ...bodyPages, backHTML]

    // 3. 写入 IndexedDB 缓存（下次秒开）
    await setCachedPages(bookId, fontSize, w, h, fullPages)

    htmlPages.value = fullPages
    totalPages.value = fullPages.length
    mobilePageIndex.value = 1
    currentPage.value = 1
    statusMsg.value = ''
    progressPercent.value = 100
  }

  // ─── 占位函数（由 ThreeReader 覆盖实际逻辑）───
  async function handleMobileComment() {}
  async function handleMobileSearch() {}

  return {
    htmlPages,
    mobilePageIndex,
    mobileSelectedText,
    mobileSelectedRange,
    mobileSelectionStyle,
    mobileFlipPrev,
    mobileFlipNext,
    onMobileTouchEnd,
    updateMobileSelection,
    initMobileView,
    handleMobileComment,
    handleMobileSearch
  }
}