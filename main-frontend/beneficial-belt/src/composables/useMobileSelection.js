import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useMobileSelection(flipContainerRef, options) {
  const { generateComment, showResultCard, closeCard, createHighlightFromRange, addCommentToHighlight, setCurrentPageForLatestHighlight, currentPage } = options

  const mobileShowActionMenu = ref(false)
  const mobileActionMenuStyle = ref({})
  const mobileSelectedText = ref('')
  const mobileSelectedRange = ref(null)

  function clearMobileSelection() {
    mobileShowActionMenu.value = false
    mobileSelectedText.value = ''
    mobileSelectedRange.value = null
    window.getSelection()?.removeAllRanges()
  }

  function onSelectionChange() {
    const selection = window.getSelection()
    if (!selection || selection.rangeCount === 0 || selection.isCollapsed) return

    const container = flipContainerRef.value
    if (!container) return

    let node = selection.anchorNode
    let inContainer = false
    while (node) {
      if (node === container) { inContainer = true; break }
      node = node.parentNode
    }
    if (!inContainer) return

    const text = selection.toString().trim()
    if (!text) return

    const range = selection.getRangeAt(0).cloneRange()
    mobileSelectedText.value = text
    mobileSelectedRange.value = range

    const rect = range.getBoundingClientRect()
    const menuWidth = 180
    let left = rect.left + rect.width / 2 - menuWidth / 2
    left = Math.max(8, Math.min(left, window.innerWidth - menuWidth - 8))
    let top = rect.bottom + 8
    if (top + 60 > window.innerHeight - 8) top = rect.top - 60

    mobileActionMenuStyle.value = {
      position: 'fixed',
      left: `${left}px`,
      top: `${Math.max(8, top)}px`,
      zIndex: 99999
    }
    mobileShowActionMenu.value = true
  }

  function onDocumentTouchStart(e) {
    if (!mobileShowActionMenu.value) return
    const menuEl = document.querySelector('.action-menu')
    if (menuEl && !menuEl.contains(e.target)) clearMobileSelection()
  }

  async function mobileChooseComment() {
    const text = mobileSelectedText.value
    const range = mobileSelectedRange.value
    if (!text || !range) return

    // 使用 web-highlighter 创建高亮
    const highlightId = await createHighlightFromRange(range)
    if (highlightId) {
      const comment = await generateComment(text)
      addCommentToHighlight(highlightId, comment)
      if (currentPage && setCurrentPageForLatestHighlight) {
        setCurrentPageForLatestHighlight(currentPage.value)
      }
      const rect = range.getBoundingClientRect()
      showResultCard(text, rect, comment, false)  // save=false 避免重复
    }
    clearMobileSelection()
    closeCard?.()
  }

  async function mobileChooseSearch() {
    const text = mobileSelectedText.value
    const range = mobileSelectedRange.value
    if (!text || !range) return

    const rect = range.getBoundingClientRect()
    clearMobileSelection()
    closeCard?.()
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

  function mobileCopySelection() {
    const text = mobileSelectedText.value
    if (!text) return
    navigator.clipboard?.writeText(text)?.then(() => {
      window.androidJsBridge?.toast?.('已复制')
    }).catch(() => {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.left = '-9999px'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      window.androidJsBridge?.toast?.('已复制')
    })
    clearMobileSelection()
  }

  onMounted(() => {
    document.addEventListener('selectionchange', onSelectionChange)
    document.addEventListener('touchstart', onDocumentTouchStart, true)
    document.addEventListener('contextmenu', (e) => e.preventDefault())
  })

  onBeforeUnmount(() => {
    document.removeEventListener('selectionchange', onSelectionChange)
    document.removeEventListener('touchstart', onDocumentTouchStart, true)
    clearMobileSelection()
  })

  return {
    mobileShowActionMenu,
    mobileActionMenuStyle,
    clearMobileSelection,
    mobileChooseComment,
    mobileChooseSearch,
    mobileCopySelection,
    mobileSelectedText,
    mobileSelectedRange,
  }
}