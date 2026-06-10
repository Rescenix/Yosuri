import { ref } from 'vue'
import { PageFlip } from 'page-flip'
import { exactPaginate } from './ExactPaginator.js'
import { getCachedPages, setCachedPages } from './cachePagination.js'

export function usePageFlip(flipContainerRef, reader, width, height, statusMsg, progressPercent) {
  const currentPage = ref(0)
  const totalPages = ref(0)
  let pageFlip = null
  let taskId = 0
  let worker = null

  function escapeHtml(str) {
    const div = document.createElement('div')
    div.textContent = str
    return div.innerHTML
  }

  function createCoverHTML(rawTitle) {
    const safeTitle = escapeHtml(rawTitle)
    return `<div style="width:100%;height:100%;background:linear-gradient(135deg,#1e2a3a 0%,#2c3e50 100%);display:flex;flex-direction:column;justify-content:center;align-items:center;font-family:'Georgia','Noto Serif SC',serif;box-shadow:inset 0 0 60px rgba(0,0,0,0.4);border-radius:4px;"><div style="width:80px;height:2px;background:rgba(200,160,80,0.6);margin-bottom:2rem;"></div><h1 style="font-size:2.2rem;margin-bottom:0.5rem;color:#e8d5b7;text-shadow:0 2px 6px rgba(0,0,0,0.5);letter-spacing:4px;">${safeTitle}</h1><p style="font-size:1rem;color:rgba(200,160,80,0.8);letter-spacing:2px;">杉汐注</p><div style="margin-top:3rem;font-size:0.8rem;color:rgba(255,255,255,0.4);">—— 脂砚斋风 · 活态传承 ——</div></div>`
  }

  function createBackHTML() {
    return `<div style="width:100%;height:100%;background:linear-gradient(135deg,#1e2a3a 0%,#2c3e50 100%);display:flex;flex-direction:column;justify-content:center;align-items:center;box-shadow:inset 0 0 40px rgba(0,0,0,0.3);border-radius:4px;"><div style="width:60px;height:60px;border:1px solid rgba(200,160,80,0.4);border-radius:50%;display:flex;align-items:center;justify-content:center;margin-bottom:1.5rem;"><span style="color:rgba(200,160,80,0.6);font-size:0.8rem;font-family:'Georgia',serif;">S</span></div><p style="color:rgba(200,160,80,0.6);font-size:0.85rem;letter-spacing:2px;">脂砚斋风 · 活态传承</p><p style="color:rgba(255,255,255,0.3);font-size:0.7rem;margin-top:2rem;">阅读小屋 · 杉汐</p></div>`
  }

  function destroyFlip() {
    if (worker) {
      worker.terminate()
      worker = null
    }
    if (pageFlip) {
      try { pageFlip.off('flip'); pageFlip.destroy() } catch (e) {}
      pageFlip = null
    }
    if (flipContainerRef.value) {
      const pages = flipContainerRef.value.querySelectorAll('.flip-page')
      pages.forEach(p => p.remove())
    }
  }

  // 带超时的 Worker 分页，失败则回退到主线程分片
  async function paginateInWorker(text, fontSize, w, h, bookId, onProgress) {
    return new Promise((resolve, reject) => {
      if (worker) {
        worker.terminate()
        worker = null
      }

      try {
        worker = new Worker(
          new URL('./pagination.worker.js', import.meta.url),
          { type: 'module' }
        )
      } catch (e) {
        console.warn('Worker 创建失败，回退到主线程分页')
        resolve(paginateInChunks(text, fontSize, w, h, onProgress))
        return
      }

      let timeoutId = setTimeout(() => {
        console.warn('Worker 超时，回退到主线程分页')
        worker.terminate()
        worker = null
        resolve(paginateInChunks(text, fontSize, w, h, onProgress))
      }, 30000) // 30秒超时

      worker.onmessage = (e) => {
        clearTimeout(timeoutId)
        const { type, percent, pages, message } = e.data
        if (type === 'progress') {
          onProgress(percent, `正在精确排版... ${percent}%`)
        } else if (type === 'result') {
          resolve(pages)
          worker.terminate()
          worker = null
        } else if (type === 'error') {
          reject(new Error(message))
          worker.terminate()
          worker = null
        }
      }

      worker.onerror = (err) => {
        clearTimeout(timeoutId)
        console.error('Worker 出错:', err)
        worker.terminate()
        worker = null
        // 回退到主线程
        resolve(paginateInChunks(text, fontSize, w, h, onProgress))
      }

      worker.postMessage({ text, fontSize, pageWidth: w, pageHeight: h })
    })
  }

  // 主线程分片分页（回退方案）
  async function paginateInChunks(text, fontSize, pageWidth, pageHeight, onProgress) {
    const paragraphs = text.split('\n')
    const total = paragraphs.length
    let bodyPages = []

    const CHUNK_SIZE = 300
    let chunkIndex = 0
    while (chunkIndex * CHUNK_SIZE < total) {
      const start = chunkIndex * CHUNK_SIZE
      const end = Math.min(start + CHUNK_SIZE, total)
      const chunkText = paragraphs.slice(start, end).join('\n')

      const chunkPages = await exactPaginate(chunkText, fontSize, pageWidth, pageHeight, () => {})

      bodyPages = bodyPages.concat(chunkPages)

      const progress = Math.floor((end / total) * 90)
      onProgress(progress, `正在精确排版... ${progress}%`)

      await new Promise(resolve => setTimeout(resolve, 0))
      chunkIndex++
    }

    onProgress(100, '排版完成')
    return bodyPages
  }


  

  async function initFlip() {
    if (!flipContainerRef.value) return

    const id = ++taskId
    destroyFlip()
    const w = width.value
    const h = height.value
    flipContainerRef.value.style.width = w + 'px'
    flipContainerRef.value.style.height = h + 'px'

    const fontSize = reader.fontSize.value
    const text = reader.fullText.value || ''
    const bookId = reader.title.value || 'unknown'

    try {
     let htmlPages = await getCachedPages(bookId, fontSize)

      if (!htmlPages) {
        statusMsg.value = '正在精确排版... 0%'
        progressPercent.value = 0

        const bodyPages = await paginateInWorker(text, fontSize, w, h, bookId, (pct, msg) => {
          statusMsg.value = msg
          progressPercent.value = pct
        })

        if (id !== taskId) return
        if (!bodyPages || bodyPages.length === 0) {
          console.warn('分页结果为空')
          htmlPages = [createCoverHTML(reader.title.value), `<div style="padding:24px;white-space:pre-wrap;">${escapeHtml(text)}</div>`, createBackHTML()]
        } else {
          const cover = createCoverHTML(reader.title.value)
          const back = createBackHTML()
          htmlPages = [cover, ...bodyPages, back]
        }
await setCachedPages(bookId, fontSize, htmlPages)
      }

      if (id !== taskId) return

      const pageElements = htmlPages.map(html => {
        const div = document.createElement('div')
        div.className = 'flip-page'
        div.style.width = w + 'px'
        div.style.height = h + 'px'
        div.innerHTML = html
        return div
      })

      if (id !== taskId || !flipContainerRef.value) return

      pageFlip = new PageFlip(flipContainerRef.value, {
        useMouseEvents: true,  
        width: w, height: h,
        size: 'fixed', autoSize: false,
        usePortrait: true, showCover: true,
        maxShadowOpacity: 0.1, flippingTime: 400,
        swipeDistance: 30, useMouseEvents: false,
        mobileScrollSupport: false, renderWhileFlipping: false,
      })
     pageFlip.loadFromHTML(pageElements)
      totalPages.value = htmlPages.length          // 使用总页数（不减2），PageFlip 会自行管理封面封底
      statusMsg.value = ''                         // ★ 清除排版提示
      progressPercent.value = 100                  // ★ 进度条满格
      return pageFlip
    } catch (err) {
      console.error('分页失败:', err)
      throw err
    }
  }

  function flipToPage(pageIndex, highlightText = null, highlightFn = null) {
    if (!pageFlip) return
    const target = pageIndex + 1
    if (typeof pageFlip.turnToPage === 'function') {
      pageFlip.turnToPage(target)
    } else if (typeof pageFlip.flip === 'function') {
      pageFlip.flip(target)
    } else {
      const current = pageFlip.getCurrentPageIndex()
      const diff = target - current
      const fn = diff > 0 ? () => pageFlip.flipNext() : () => pageFlip.flipPrev()
      for (let i = 0; i < Math.abs(diff); i++) setTimeout(fn, i * 100)
    }
    if (highlightText && highlightFn) {
      setTimeout(() => requestAnimationFrame(() => highlightFn(highlightText, target)), 450)
    }
  }

  function flipToPhysicalPage(pageIndex) {
    if (!pageFlip) return
    if (typeof pageFlip.turnToPage === 'function') {
      pageFlip.turnToPage(pageIndex)
    } else if (typeof pageFlip.flip === 'function') {
      pageFlip.flip(pageIndex)
    } else {
      const current = pageFlip.getCurrentPageIndex()
      const diff = pageIndex - current
      const fn = diff > 0 ? () => pageFlip.flipNext() : () => pageFlip.flipPrev()
      for (let i = 0; i < Math.abs(diff); i++) setTimeout(fn, i * 100)
    }
  }

  function jumpToChapter(title) {
    if (!flipContainerRef.value || !pageFlip) return
    const pages = flipContainerRef.value.querySelectorAll('.flip-page')
    for (let i = 1; i < pages.length - 1; i++) {
      if (pages[i].textContent.includes(title)) {
        pageFlip.turnToPage?.(i) ?? pageFlip.flip?.(i)
        return
      }
    }
  }

  async function flipToCoverAnimated() {
    if (!pageFlip) return
    const STEP_DELAY = 120
    const COVER_PAUSE = 800   // ★ 封面停留 1.5 秒
    const current = pageFlip.getCurrentPageIndex()

    if (current === 0) {
      await new Promise(r => setTimeout(r, COVER_PAUSE))
      return
    }

    if (current > 3) {
      pageFlip.turnToPage?.(5) ?? pageFlip.flip?.(5)
      await new Promise(r => requestAnimationFrame(r))
    }

    for (let i = 0; i < 3; i++) {
      if (pageFlip.getCurrentPageIndex() <= 0) break
      pageFlip.flipPrev()
      await new Promise(resolve => {
        const onFlip = () => { pageFlip.off('flip', onFlip); resolve() }
        pageFlip.on('flip', onFlip)
        setTimeout(() => { pageFlip.off('flip', onFlip); resolve() }, 1000)
      })
      await new Promise(r => setTimeout(r, STEP_DELAY))
    }
    await new Promise(r => setTimeout(r, COVER_PAUSE))
  }

  function flipPrev() { if (pageFlip) pageFlip.flipPrev() }
  function flipNext() { if (pageFlip) pageFlip.flipNext() }

  return {
    currentPage, totalPages, pageFlip, initFlip, destroyFlip,
    flipToPage, flipToPhysicalPage, jumpToChapter,
    flipToCoverAnimated, flipPrev, flipNext,
  }
}