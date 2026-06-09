// src/composables/useMobileSelection.js
import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useMobileSelection(flipContainerRef, annotation) {
  const { highlightText, generateComment, showResultCard, closeCard } = annotation

  const mobileShowActionMenu = ref(false)
  const mobileActionMenuStyle = ref({})
  const mobileHighlightEl = ref(null)

  function clearMobileSelection() {
    if (mobileHighlightEl.value) {
      const parent = mobileHighlightEl.value.parentNode
      if (parent) {
        parent.replaceChild(
          document.createTextNode(mobileHighlightEl.value.textContent),
          mobileHighlightEl.value
        )
      }
      mobileHighlightEl.value = null
    }
    mobileShowActionMenu.value = false
  }

  // 用 clone 的 range 创建高亮 span
  function applyHighlightFromRange(range) {
    clearMobileSelection()
    const rect = range.getBoundingClientRect()
    try {
      const span = document.createElement('span')
      span.className = 'shanxi-highlight selected'
      range.surroundContents(span)
      mobileHighlightEl.value = span
      return rect
    } catch (e) {
      // 跨节点降级：提取文本并包装
      const text = range.toString()
      if (!text) return null
      const span = document.createElement('span')
      span.className = 'shanxi-highlight selected'
      span.textContent = text
      range.deleteContents()
      range.insertNode(span)
      mobileHighlightEl.value = span
      return span.getBoundingClientRect()
    }
  }

  function showMenu(rect) {
    if (!rect) rect = { left: 100, top: 200, width: 100, bottom: 210 }
    mobileActionMenuStyle.value = {
      position: 'fixed',
      left: `${Math.min(rect.left + rect.width / 2 - 80, window.innerWidth - 170)}px`,
      top: `${rect.bottom + 6}px`,
      display: 'flex',
      gap: '8px',
      zIndex: 99999
    }
    mobileShowActionMenu.value = true
  }

  // 只在 touchend 时处理
  function onContainerTouchEnd(e) {
    // 翻页区、菜单区不管
    if (e.target.closest('.flip-tap-area') || e.target.closest('.action-menu')) return

    const selection = window.getSelection()
    if (!selection || selection.rangeCount === 0 || selection.isCollapsed) {
      clearMobileSelection()
      return
    }

    const container = flipContainerRef.value
    if (!container) return

    // 确认选区在阅读器内
    let node = selection.anchorNode
    let inContainer = false
    while (node) {
      if (node === container) { inContainer = true; break }
      node = node.parentNode
    }
    if (!inContainer) {
      clearMobileSelection()
      return
    }

    const text = selection.toString().trim()
    if (!text) {
      clearMobileSelection()
      return
    }

    // 克隆选区，然后立刻清除系统选区（关闭系统菜单）
    const range = selection.getRangeAt(0).cloneRange()
    selection.removeAllRanges()

    // 用克隆的 range 创建高亮
    const rect = applyHighlightFromRange(range)
    if (rect) showMenu(rect)
  }

  // 点击菜单外部关闭
  function onDocumentTouchStart(e) {
    if (!mobileShowActionMenu.value) return
    const menuEl = document.querySelector('.action-menu')
    if (menuEl && !menuEl.contains(e.target)) {
      clearMobileSelection()
    }
  }

  // 菜单项
  async function mobileChooseComment() {
    if (!mobileHighlightEl.value) return
    const text = mobileHighlightEl.value.textContent
    const rect = mobileHighlightEl.value.getBoundingClientRect()
    clearMobileSelection()
    closeCard?.()
    highlightText(text, document.createRange())
    const comment = await generateComment(text)
    showResultCard(text, rect, comment, true)
  }

  async function mobileChooseSearch() {
    if (!mobileHighlightEl.value) return
    const text = mobileHighlightEl.value.textContent
    const rect = mobileHighlightEl.value.getBoundingClientRect()
    clearMobileSelection()
    closeCard?.()
    try {
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: `帮我搜索一下“${text}”` })
      })
      const data = await res.json()
      const reply = data.reply || data.message || data.content || '暂无搜索结果。'
      showResultCard(text, rect, reply, false)
    } catch (e) {
      showResultCard(text, rect, '搜索失败，请重试。', false)
    }
  }

  onMounted(() => {
    const container = flipContainerRef.value
    if (container) {
      container.addEventListener('touchend', onContainerTouchEnd, { passive: false })
    }
    document.addEventListener('touchstart', onDocumentTouchStart, true)
    document.addEventListener('contextmenu', e => e.preventDefault())
  })

  onBeforeUnmount(() => {
    const container = flipContainerRef.value
    if (container) container.removeEventListener('touchend', onContainerTouchEnd)
    document.removeEventListener('touchstart', onDocumentTouchStart, true)
    document.removeEventListener('contextmenu', e => e.preventDefault())
    clearMobileSelection()
  })

  return {
    mobileShowActionMenu,
    mobileActionMenuStyle,
    clearMobileSelection,
    mobileChooseComment,
    mobileChooseSearch,
  }
}