// src/composables/useMobileSelection.js
import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useMobileSelection(flipContainerRef, options) {
  const { generateComment, showResultCard, closeCard, createHighlightFromRange, addCommentToHighlight, setCurrentPageForLatestHighlight, currentPage } = options

  const mobileShowActionMenu = ref(false)
  const mobileActionMenuStyle = ref({})
  const mobileSelectedText = ref('')
  const mobileSelectedRange = ref(null)
  
  // 记录当前高亮的菜单项 ( 'comment' | 'search' | 'copy' | null )
  const activeMenuItem = ref(null)
  
  // 双击检测相关变量
  let pendingType = null        // 等待第二次点击的菜单类型
  let tapTimer = null           // 定时器句柄

  function clearPending() {
    if (tapTimer) {
      clearTimeout(tapTimer)
      tapTimer = null
    }
    pendingType = null
    activeMenuItem.value = null   // 清除高亮
  }

  // 实际执行注释
  async function executeComment() {
    const text = mobileSelectedText.value
    const range = mobileSelectedRange.value
    if (!text || !range) return

    const highlightId = await createHighlightFromRange(range)
    if (highlightId) {
      const comment = await generateComment(text)
      addCommentToHighlight(highlightId, comment)
      if (currentPage && setCurrentPageForLatestHighlight) {
        setCurrentPageForLatestHighlight(currentPage.value)
      }
      const rect = range.getBoundingClientRect()
      showResultCard(text, rect, comment, false)
    }
    clearMobileSelection()
    closeCard?.()
  }

  // 实际执行搜索
  async function executeSearch() {
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

  // 实际执行复制
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

  // 通用单击/双击处理
  function handleMenuItemTap(type, executeFn) {
    // 如果已经高亮且类型相同，说明是双击 → 执行操作
    if (pendingType === type && tapTimer) {
      clearPending()
      executeFn()
    } 
    // 否则是首次单击（或切换到另一个菜单项）
    else {
      // 清除之前的 pending 和高亮
      clearPending()
      // 设置新的高亮
      pendingType = type
      activeMenuItem.value = type
      // 设置定时器，超时后清除高亮（避免一直高亮）
      tapTimer = setTimeout(() => {
        clearPending()
      }, 300)   // 300ms 内无第二次点击则取消高亮
    }
  }

  // 暴露给父组件的菜单点击处理方法（代替原来的 mobileChooseComment 等）
  function mobileChooseComment() {
    handleMenuItemTap('comment', executeComment)
  }
  function mobileChooseSearch() {
    handleMenuItemTap('search', executeSearch)
  }
  function mobileCopySelection() {
    handleMenuItemTap('copy', executeCopy)
  }

  // 清除选区（同时清除高亮等待）
  function clearMobileSelection() {
    mobileShowActionMenu.value = false
    mobileSelectedText.value = ''
    mobileSelectedRange.value = null
    window.getSelection()?.removeAllRanges()
    clearPending()
  }

  function onSelectionChange() {
    // ... 保持原有完全相同的逻辑 ...
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
    // 每次弹出新菜单时清除旧的 pending 和高亮
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
    activeMenuItem,   // 暴露高亮状态
  }
}