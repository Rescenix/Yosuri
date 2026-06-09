import { ref, nextTick } from 'vue'
import { getCachedPages, setCachedPages } from './cachePagination.js'

export function useMobileReader(flipContainerRef, reader, statusMsg, progressPercent, totalPages, currentPage) {
  const htmlPages = ref([])
  const mobilePageIndex = ref(0)
  const mobileSelectedText = ref('')
  const mobileSelectedRange = ref(null)
  const mobileSelectionStyle = ref({})

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

 function updateMobileSelection() {
  const selection = window.getSelection()
  const text = selection.toString().trim()
  if (text.length === 0) {
    mobileSelectedText.value = ''
    mobileSelectedRange.value = null
    return
  }
  const range = selection.getRangeAt(0).cloneRange()
  mobileSelectedText.value = text
  mobileSelectedRange.value = range

  const rect = range.getBoundingClientRect()
  mobileSelectionStyle.value = {
    position: 'fixed',
    left: `${rect.left + rect.width / 2 - 80}px`,
    top: `${rect.bottom + 4}px`,
    display: 'flex',
    gap: '8px',
    zIndex: 100
  }

  // ★ 关键：立即清除系统选区，防止出现原生菜单
  window.getSelection()?.removeAllRanges()
}

  function onMobileTouchEnd(event) {
  event.preventDefault() // 阻止浏览器默认菜单
  event.stopPropagation()
  
  setTimeout(() => {
    updateMobileSelection()
    // 再次清除选区，确保原生菜单不会出现
    window.getSelection()?.removeAllRanges()
  }, 10)
}

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

    // 1. 尝试从缓存加载
    const cached = await getCachedPages(bookId, fontSize, w, h)
    if (cached) {
      htmlPages.value = cached
      totalPages.value = cached.length
      mobilePageIndex.value = 1
      currentPage.value = 1
      statusMsg.value = ''
      progressPercent.value = 100
      return
    }

    // 2. 无缓存，执行分页
    statusMsg.value = '正在排版... 0%'
    progressPercent.value = 0

    const { exactPaginate } = await import('./ExactPaginator.js')
    const bodyPages = await exactPaginate(text, fontSize, w, h, (pct) => {
      progressPercent.value = pct
    })

    const coverHTML = createCoverHTML(reader.title.value)
    const backHTML = createBackHTML()
    const fullPages = [coverHTML, ...bodyPages, backHTML]

    // 3. 写入缓存
    await setCachedPages(bookId, fontSize, w, h, fullPages)

    htmlPages.value = fullPages
    totalPages.value = fullPages.length
    mobilePageIndex.value = 1
    currentPage.value = 1
    statusMsg.value = ''
    progressPercent.value = 100
  }

  // 占位函数，具体逻辑由 ThreeReader 绑定
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