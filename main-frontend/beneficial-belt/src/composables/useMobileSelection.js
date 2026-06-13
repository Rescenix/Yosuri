// src/composables/useMobileSelection.js
import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useMobileSelection(flipContainerRef, options) {
  const { 
    generateComment, showResultCard, closeCard, 
    createHighlightFromRange, addCommentToHighlight, 
    setCurrentPageForLatestHighlight, currentPage,
    highlightText   // 新增：回升高亮函数
  } = options

  const mobileShowActionMenu = ref(false)
  const mobileActionMenuStyle = ref({})
  const mobileSelectedText = ref('')
  const mobileSelectedRange = ref(null)
  
  // 记录当前高亮的菜单项
  const activeMenuItem = ref(null)
  
  // 双击检测
  let pendingType = null
  let tapTimer = null

  function clearPending() {
    if (tapTimer) {
      clearTimeout(tapTimer)
      tapTimer = null
    }
    pendingType = null
    activeMenuItem.value = null
  }

  // 实际执行注释
  async function executeComment() {
    const text = mobileSelectedText.value
    const range = mobileSelectedRange.value
    if (!text || !range) return

    // 立即显示“思考中”卡片
    const rect = range.getBoundingClientRect()
    showResultCard(text, rect, '🤔 杉汐正在思考中...', false)

    // 尝试 web-highlighter
    let highlightId = null
    try {
      highlightId = await createHighlightFromRange(range)
    } catch (err) {
      console.error('web-highlighter 失败', err)
    }

    let comment
    if (highlightId) {
      // 新方式
      comment = await generateComment(text)
      addCommentToHighlight(highlightId, comment)
      if (currentPage && setCurrentPageForLatestHighlight) {
        setCurrentPageForLatestHighlight(currentPage.value)
      }
    } else {
      // 回退到旧 highlightText（立即高亮）
      if (highlightText) {
        highlightText(text, range)
      }
      comment = await generateComment(text)
      // 旧方式也会保存到 annotations（save=true）
      showResultCard(text, rect, comment, true)
      clearMobileSelection()
      closeCard?.()
      return
    }

    // 最终显示结果卡片（新方式，save=false）
    showResultCard(text, rect, comment, false)
    clearMobileSelection()
    closeCard?.()
  }

  // 实际执行搜索（也加思考中反馈）
  async function executeSearch() {
    const text = mobileSelectedText.value
    const range = mobileSelectedRange.value
    if (!text || !range) return

    const rect = range.getBoundingClientRect()
    // 立即显示“搜索中”卡片
    showResultCard(text, rect, '🔍 正在搜索...', false)

    clearMobileSelection()
    closeCard?.()
    try {
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: `帮我搜索一下“${text}”` })
      })
      if (!res.ok) {
        showResultCard(text, rect, '搜索服务暂不可用，请稍后重试。', false)
        return
      }
      const data = await res.json()
      const reply = data.reply || data.message || data.content || '暂无搜索结果。'
      showResultCard(text, rect, reply, false)
    } catch (e) {
      showResultCard(text, rect, '搜索失败，请重试。', false)
    }
  }

  // 实际执行复制（无需反馈）
  function executeCopy() {
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

  // 单击/双击处理
  function handleMenuItemTap(type, executeFn) {
    if (pendingType === type && tapTimer) {
      clearPending()
      executeFn()
    } else {
      clearPending()
      pendingType = type
      activeMenuItem.value = type
      tapTimer = setTimeout(() => {
        clearPending()
      }, 300)
    }
  }

  function mobileChooseComment() {
    handleMenuItemTap('comment', executeComment)
  }
  function mobileChooseSearch() {
    handleMenuItemTap('search', executeSearch)
  }
  function mobileCopySelection() {
    handleMenuItemTap('copy', executeCopy)
  }

  function clearMobileSelection() {
    mobileShowActionMenu.value = false
    mobileSelectedText.value = ''
    mobileSelectedRange.value = null
    window.getSelection()?.removeAllRanges()
    clearPending()
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
    let top = rect.bottom + 24               // 下移 24px
    if (top + 70 > window.innerHeight - 16) top = rect.top - 70
    mobileActionMenuStyle.value = {
      position: 'fixed',
      left: `${left}px`,
      top: `${Math.max(8, top)}px`,
      zIndex: 99999
    }
    mobileShowActionMenu.value = true
    clearPending()
  }

  function onDocumentTouchStart(e) {
    if (!mobileShowActionMenu.value) return
    const menuEl = document.querySelector('.action-menu')
    if (menuEl && !menuEl.contains(e.target)) clearMobileSelection()
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
    activeMenuItem,
  }
}