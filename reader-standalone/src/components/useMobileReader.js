// src/components/useMobileReader.js
import { ref, nextTick } from 'vue'
import { getCachedPages, setCachedPages } from './cachePagination.js'
import { exactPaginate } from './ExactPaginator.js'

export function useMobileReader(flipContainerRef, reader, statusMsg, progressPercent, totalPages, currentPage, onPageChanged) {
  const htmlPages = ref([])
  const mobilePageIndex = ref(0)

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

  function getStorageKey() {
    return `${reader.title.value}_pos`
  }

  function savePosition() {
    try {
      localStorage.setItem(getStorageKey(), mobilePageIndex.value)
    } catch (e) {}
  }

  function loadPosition() {
    try {
      const val = localStorage.getItem(getStorageKey())
      return val !== null ? parseInt(val, 10) : null
    } catch (e) {
      return null
    }
  }

  function mobileFlipPrev() {
    if (mobilePageIndex.value > 1) {
      mobilePageIndex.value--
      currentPage.value = mobilePageIndex.value - 1
      savePosition()
      nextTick(() => {
        if (onPageChanged) onPageChanged()
      })
    }
  }

  function mobileFlipNext() {
    if (mobilePageIndex.value < htmlPages.value.length - 2) {
      mobilePageIndex.value++
      currentPage.value = mobilePageIndex.value - 1
      savePosition()
      nextTick(() => {
        if (onPageChanged) onPageChanged()
      })
    }
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

    statusMsg.value = ''
    progressPercent.value = 0

    const cached = await getCachedPages(bookId, fontSize)
    if (cached && cached.length > 0) {
      htmlPages.value = cached
      totalPages.value = cached.length

      const saved = loadPosition()
      let target = 1
      if (saved !== null && saved >= 1 && saved < cached.length) {
        if (saved === cached.length - 1) {
          target = cached.length - 2
          if (target < 1) target = 1
        } else {
          target = saved
        }
      }
      mobilePageIndex.value = target
      currentPage.value = Math.max(0, target - 1)
      savePosition()

      statusMsg.value = ''
      progressPercent.value = 100
      nextTick(() => {
        if (onPageChanged) onPageChanged()
      })
      // 不再手动设置 reader.loading
      return
    }

    // 无缓存，开始流式排版
    const coverHTML = createCoverHTML(reader.title.value)
    htmlPages.value = [coverHTML]
    totalPages.value = 1
    mobilePageIndex.value = 0
    currentPage.value = 0

    const paragraphs = text.split('\n')
    const CHUNK_SIZE = 200
    let processed = 0

    function processNextChunk() {
      if (processed >= paragraphs.length) {
        const backHTML = createBackHTML()
        htmlPages.value.push(backHTML)
        totalPages.value = htmlPages.value.length

        const saved = loadPosition()
        let target = 1
        if (saved !== null && saved >= 1 && saved < htmlPages.value.length) {
          if (saved === htmlPages.value.length - 1) {
            target = htmlPages.value.length - 2
            if (target < 1) target = 1
          } else {
            target = saved
          }
        } else if (htmlPages.value.length > 1) {
          target = 1
        }
        mobilePageIndex.value = target
        currentPage.value = Math.max(0, target - 1)
        savePosition()

        setCachedPages(bookId, fontSize, htmlPages.value)
        statusMsg.value = ''
        progressPercent.value = 100
        nextTick(() => {
          if (onPageChanged) onPageChanged()
        })
        // 排版完成，同样不操作 loading
        return
      }

      const end = Math.min(processed + CHUNK_SIZE, paragraphs.length)
      const chunkText = paragraphs.slice(processed, end).join('\n')

      setTimeout(async () => {
        try {
          const chunkPages = await exactPaginate(chunkText, fontSize, w, h, () => {})
          htmlPages.value.push(...chunkPages)
          totalPages.value = htmlPages.value.length

          if (mobilePageIndex.value === 0 && htmlPages.value.length > 1) {
            mobilePageIndex.value = 1
            currentPage.value = 0
          }

          processed = end
          processNextChunk()
        } catch (e) {
          console.error('流式排版失败', e)
        }
      }, 0)
    }

    processNextChunk()
  }

  return {
    htmlPages,
    mobilePageIndex,
    mobileFlipPrev,
    mobileFlipNext,
    initMobileView,
  }
}