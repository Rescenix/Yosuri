// src/components/reading/useMobileReader.js
import { ref, nextTick } from 'vue'
import { getCachedPages, setCachedPages } from './cachePagination.js'

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

  // ─── Worker 分页（带超时回退到主线程）─────────
  async function paginateWithWorker(text, fontSize, pageWidth, pageHeight, onProgress) {
    return new Promise((resolve, reject) => {
      let worker = null
      try {
        worker = new Worker(
          new URL('./pagination.worker.js', import.meta.url),
          { type: 'module' }
        )
      } catch (e) {
        console.warn('Worker 创建失败，回退到主线程分页')
        resolve(null) // 返回 null 表示需要回退
        return
      }

      const timeout = setTimeout(() => {
        console.warn('Worker 超时，回退到主线程分页')
        worker.terminate()
        resolve(null)
      }, 30000)

      worker.onmessage = (e) => {
        clearTimeout(timeout)
        const { type, percent, pages } = e.data
        if (type === 'progress') {
          onProgress(percent)
        } else if (type === 'result') {
          resolve(pages)
          worker.terminate()
        } else if (type === 'error') {
          reject(new Error(e.data.message))
          worker.terminate()
        }
      }

      worker.onerror = (err) => {
        clearTimeout(timeout)
        console.error('Worker 出错:', err)
        worker.terminate()
        resolve(null) // 回退
      }

      worker.postMessage({ text, fontSize, pageWidth, pageHeight })
    })
  }

  // ─── 主线程分片分页（回退方案）─────────────
  async function paginateInMainThread(text, fontSize, pageWidth, pageHeight, onProgress) {
    const paragraphs = text.split('\n')
    const total = paragraphs.length
    let bodyPages = []

    const CHUNK_SIZE = 300
    let chunkIndex = 0
    while (chunkIndex * CHUNK_SIZE < total) {
      const start = chunkIndex * CHUNK_SIZE
      const end = Math.min(start + CHUNK_SIZE, total)
      const chunkText = paragraphs.slice(start, end).join('\n')

      const { exactPaginate } = await import('./ExactPaginator.js')
      const chunkPages = await exactPaginate(chunkText, fontSize, pageWidth, pageHeight, () => {})

      bodyPages = bodyPages.concat(chunkPages)

      const progress = Math.floor((end / total) * 90)
      onProgress(progress)

      await new Promise(resolve => setTimeout(resolve, 0))
      chunkIndex++
    }

    return bodyPages
  }

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

    // 1. 尝试从 IndexedDB 加载全本缓存
    const cached = await getCachedPages(bookId, fontSize, w, h)
    if (cached && cached.length > 0) {
      htmlPages.value = cached
      totalPages.value = cached.length
      mobilePageIndex.value = 1
      currentPage.value = 1
      statusMsg.value = ''
      progressPercent.value = 100
      // 后台预取附近字号
      prefetchNearbySizes(text, bookId, fontSize, w, h)
      return
    }

    // 2. 执行排版（优先 Worker，回退主线程）
    statusMsg.value = '正在排版... 0%'
    progressPercent.value = 0

    let bodyPages = await paginateWithWorker(text, fontSize, w, h, (pct) => {
      statusMsg.value = `正在排版... ${pct}%`
      progressPercent.value = pct
    })

    // Worker 失败或不可用，回退到主线程分片排版
    if (!bodyPages) {
      bodyPages = await paginateInMainThread(text, fontSize, w, h, (pct) => {
        statusMsg.value = `正在排版... ${pct}%`
        progressPercent.value = pct
      })
    }

    const coverHTML = createCoverHTML(reader.title.value)
    const backHTML = createBackHTML()
    const fullPages = [coverHTML, ...bodyPages, backHTML]

    // 3. 写入 IndexedDB 缓存
    await setCachedPages(bookId, fontSize, w, h, fullPages)

    htmlPages.value = fullPages
    totalPages.value = fullPages.length
    mobilePageIndex.value = 1
    currentPage.value = 1
    statusMsg.value = ''
    progressPercent.value = 100

    // 4. 后台预取附近字号
    prefetchNearbySizes(text, bookId, fontSize, w, h)
  }

  // ─── 预取附近字号 ───────────────────────
  async function prefetchNearbySizes(text, bookId, currentSize, pageWidth, pageHeight) {
    const sizes = [currentSize - 1, currentSize + 1, currentSize - 2, currentSize + 2]
      .filter(size => size >= 12 && size <= 24)

    for (const size of sizes) {
      const cached = await getCachedPages(bookId, size, pageWidth, pageHeight)
      if (cached && cached.length > 0) continue

      // 后台排版，不阻塞
      setTimeout(async () => {
        try {
          let pages = await paginateWithWorker(text, size, pageWidth, pageHeight, () => {})
          if (!pages) {
            pages = await paginateInMainThread(text, size, pageWidth, pageHeight, () => {})
          }
          const cover = createCoverHTML(reader.title.value)
          const back = createBackHTML()
          const full = [cover, ...pages, back]
          await setCachedPages(bookId, size, pageWidth, pageHeight, full)
        } catch (e) {
          // 预排版失败不影响主流程
        }
      }, 200)
    }
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
    handleMobileSearch,
    prefetchNearbySizes,
  }
}