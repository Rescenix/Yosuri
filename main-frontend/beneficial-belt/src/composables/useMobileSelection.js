// src/composables/useMobileSelection.js
import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useMobileSelection(flipContainerRef, annotation) {
  const { generateComment, showResultCard, closeCard } = annotation

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

  // 选区发生变化时立即响应
  function onSelectionChange() {
    const selection = window.getSelection()
    if (!selection || selection.rangeCount === 0 || selection.isCollapsed) {
      return
    }

    const container = flipContainerRef.value
    if (!container) return

    let node = selection.anchorNode
    let inContainer = false
    while (node) {
      if (node === container) {
        inContainer = true
        break
      }
      node = node.parentNode
    }
    if (!inContainer) return

    const text = selection.toString().trim()
    if (!text) return

    const range = selection.getRangeAt(0).cloneRange()
    mobileSelectedText.value = text
    mobileSelectedRange.value = range

    const rect = range.getBoundingClientRect()
    const menuWidth = 180  // 毛玻璃菜单估计宽度
    let left = rect.left + rect.width / 2 - menuWidth / 2
    left = Math.max(8, Math.min(left, window.innerWidth - menuWidth - 8))
    let top = rect.bottom + 8
    if (top + 60 > window.innerHeight - 8) {
      top = rect.top - 60
    }

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
    if (menuEl && !menuEl.contains(e.target)) {
      clearMobileSelection()
    }
  }

  async function mobileChooseComment() {
    const text = mobileSelectedText.value
    const range = mobileSelectedRange.value
    if (!text || !range) return

    const rect = range.getBoundingClientRect()
    clearMobileSelection()
    closeCard?.()
    const comment = await generateComment(text)
    showResultCard(text, rect, comment, true)
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
    mobileSelectedText,
    mobileSelectedRange,
  }
}