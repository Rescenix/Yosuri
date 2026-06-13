// src/components/reading/useMobileReader.js
import { ref, nextTick } from 'vue'
import { getCachedPages, setCachedPages } from './cachePagination.js'

export function useMobileReader(flipContainerRef, reader, statusMsg, progressPercent, totalPages, currentPage) {
  const htmlPages = ref([])
  const mobilePageIndex = ref(0)

  // ─── HTML 辅助函数 ──────────
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

  // ─── 翻页（禁止翻到封面） ──
  function mobileFlipPrev() {
    if (mobilePageIndex.value > 1) {
      mobilePageIndex.value--
      currentPage.value = mobilePageIndex.value - 1
      savePosition()
      nextTick(() => applyAnnotationsToPage())
    }
  }

  function mobileFlipNext() {
    if (mobilePageIndex.value < htmlPages.value.length - 1) {
      mobilePageIndex.value++
      currentPage.value = mobilePageIndex.value - 1
      savePosition()
      nextTick(() => applyAnnotationsToPage())
    }
  }

  // 保存/恢复阅读位置
  function savePosition() {
    try {
      localStorage.setItem(`mobile_pos_${reader.title.value}`, mobilePageIndex.value)
    } catch (e) {}
  }

  function loadPosition() {
    try {
      const val = localStorage.getItem(`mobile_pos_${reader.title.value}`)
      return val !== null ? parseInt(val, 10) : null
    } catch (e) {
      return null
    }
  }

  // ─── 批注高亮恢复（翻页后自动应用） ──
  function applyAnnotationsToPage() {
    const page = document.querySelector('.mobile-page-view')
    if (!page) return

    const stored = localStorage.getItem('shanxi_annotations')
    if (!stored) return
    const annotations = JSON.parse(stored)

    const walker = document.createTreeWalker(page, NodeFilter.SHOW_TEXT)
    const tasks = []
    while (walker.nextNode()) {
      const node = walker.currentNode
      // 跳过已位于 shanxi-highlight 内的文本节点
      if (node.parentElement && node.parentElement.classList.contains('shanxi-highlight')) continue

      for (const anno of annotations) {
        const idx = node.textContent.indexOf(anno.text)
        if (idx !== -1) {
          tasks.push({ node, text: anno.text, offset: idx })
        }
      }
    }

    tasks.forEach(({ node, text, offset }) => {
      const before = document.createTextNode(node.textContent.substring(0, offset))
      const after = document.createTextNode(node.textContent.substring(offset + text.length))
      const span = document.createElement('span')
      span.className = 'shanxi-highlight'
      span.textContent = text
      // 与 useAnnotation 完全相同的样式，确保不影响布局
      span.style.cssText = `
        background-color: rgba(180, 80, 50, 0.25) !important;
        display: inline !important;
        line-height: inherit !important;
        font-size: inherit !important;
        font-family: inherit !important;
        font-weight: inherit !important;
        font-style: inherit !important;
        vertical-align: baseline !important;
        white-space: inherit !important;
        word-spacing: inherit !important;
        letter-spacing: inherit !important;
        text-indent: inherit !important;
        text-transform: inherit !important;
        padding: 0 !important;
        margin: 0 !important;
        border: none !important;
        outline: none !important;
        box-shadow: none !important;
        border-radius: 0 !important;
        width: auto !important;
        height: auto !important;
        float: none !important;
        clear: none !important;
        position: static !important;
        top: auto !important;
        left: auto !important;
      `
      node.parentNode.insertBefore(before, node)
      node.parentNode.insertBefore(span, node)
      node.parentNode.insertBefore(after, node)
      node.parentNode.removeChild(node)
    })
  }

  // ─── 流式排版与缓存 ──────────
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

    statusMsg.value = ''
    progressPercent.value = 0

    // 1. 尝试从 IndexedDB 加载缓存（无宽高）
    const cached = await getCachedPages(bookId, fontSize)
    if (cached && cached.length > 0) {
      htmlPages.value = cached
      totalPages.value = cached.length

      // 恢复上次阅读位置
      const saved = loadPosition()
      if (saved !== null && saved >= 1 && saved < cached.length) {
        mobilePageIndex.value = saved
        currentPage.value = Math.max(0, saved - 1)
      } else {
        mobilePageIndex.value = 1
        currentPage.value = 0
      }
      statusMsg.value = ''
      progressPercent.value = 100
      nextTick(() => applyAnnotationsToPage())
      return
    }

    // 2. 无缓存，流式排版
    const coverHTML = createCoverHTML(reader.title.value)
    htmlPages.value = [coverHTML]
    totalPages.value = 1
    mobilePageIndex.value = 0
    currentPage.value = 0

    const paragraphs = text.split('\n')
    const total = paragraphs.length
    const CHUNK_SIZE = 200
    let bodyPages = []
    let finished = false

    function processNextChunk(index) {
      if (finished) return
      if (index >= total) {
        finished = true
        const backHTML = createBackHTML()
        const fullPages = [coverHTML, ...bodyPages, backHTML]
        htmlPages.value = fullPages
        totalPages.value = fullPages.length

        // 恢复位置
        const saved = loadPosition()
        if (saved !== null && saved >= 1 && saved < fullPages.length) {
          mobilePageIndex.value = saved
          currentPage.value = Math.max(0, saved - 1)
        } else if (bodyPages.length > 0) {
          mobilePageIndex.value = 1
          currentPage.value = 0
        }

        // 写入缓存
        setCachedPages(bookId, fontSize, fullPages)
        statusMsg.value = ''
        progressPercent.value = 100
        nextTick(() => applyAnnotationsToPage())
        return
      }

      const end = Math.min(index + CHUNK_SIZE, total)
      const chunkText = paragraphs.slice(index, end).join('\n')

      setTimeout(async () => {
        if (finished) return
        try {
          const { exactPaginate } = await import('./ExactPaginator.js')
          const chunkPages = await exactPaginate(chunkText, fontSize, w, h, () => {})
          bodyPages = bodyPages.concat(chunkPages)

          // 动态更新页面（暂不加封底）
          htmlPages.value = [coverHTML, ...bodyPages]
          totalPages.value = htmlPages.value.length

          // 如果当前还在封面且正文已出现，自动翻到第一页
          if (mobilePageIndex.value === 0 && bodyPages.length > 0) {
            mobilePageIndex.value = 1
            currentPage.value = 0
          }

          processNextChunk(end)
        } catch (e) {
          console.error('流式排版失败', e)
        }
      }, 0)
    }

    processNextChunk(0)
  }

  function updateMobileSelection() {}
  function onMobileTouchEnd(event) {}
  async function handleMobileComment() {}
  async function handleMobileSearch() {}

  return {
    htmlPages,
    mobilePageIndex,
    mobileFlipPrev,
    mobileFlipNext,
    onMobileTouchEnd,
    updateMobileSelection,
    initMobileView,
    handleMobileComment,
    handleMobileSearch,
  }
}