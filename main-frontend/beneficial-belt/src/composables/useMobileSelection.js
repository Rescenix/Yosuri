// src/composables/useMobileSelection.js
import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useMobileSelection(flipContainerRef, annotation) {
  const { highlightText, generateComment, showResultCard, closeCard } = annotation

  const mobileShowActionMenu = ref(false)
  const mobileActionMenuStyle = ref({})
  const mobileSelectedText = ref('')
  const mobileSelectedRange = ref(null)
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
    mobileSelectedText.value = ''
    mobileSelectedRange.value = null
  }

  // 手指离开屏幕时检查是否有选区，有则显示自定义菜单
  function onContainerTouchEnd(e) {
    // 如果触摸点在翻页区或菜单上，不处理
    if (e.target.closest('.flip-tap-area') || e.target.closest('.action-menu')) return

    const selection = window.getSelection()
    if (!selection || selection.rangeCount === 0 || selection.isCollapsed) {
      // 没有有效选区，可能之前有菜单，此时不清除，用户可点其他地方关闭
      return
    }

    const text = selection.toString().trim()
    if (!text) return

    // 保存选区和位置
    const range = selection.getRangeAt(0).cloneRange()
    mobileSelectedText.value = text
    mobileSelectedRange.value = range

    // 创建模拟高亮并立即关闭系统选区（系统菜单会瞬间消失）
    try {
      const span = document.createElement('span')
      span.className = 'shanxi-highlight selected'
      range.surroundContents(span)
      mobileHighlightEl.value = span
    } catch (e) {
      // 跨节点选择，暂时忽略
    }
    window.getSelection()?.removeAllRanges()

    // 显示自定义菜单
    const rect = range.getBoundingClientRect()
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

  // 点击菜单外部关闭
  function onDocumentTouchStart(e) {
    if (!mobileShowActionMenu.value) return
    const menuEl = document.querySelector('.action-menu')
    if (menuEl && !menuEl.contains(e.target)) {
      clearMobileSelection()
    }
  }

  // 菜单项：批注
async function mobileChooseComment() {
  const text = mobileSelectedText.value
  const range = mobileSelectedRange.value
  if (!text || !range) return

  const rect = range.getBoundingClientRect()
  clearMobileSelection()
  closeCard?.()
  highlightText(text, range)
  const comment = await generateComment(text)
  showResultCard(text, rect, comment, true, true)   // 第五个参数 true = 固定定位
}

// 菜单项：搜索
async function mobileChooseSearch() {
  console.log('[mobileChooseSearch] 开始')
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
      showResultCard(text, rect, '搜索服务暂不可用，请稍后重试。', false, true)
      return
    }
    const data = await res.json()
    const reply = data.reply || data.message || data.content || '暂无搜索结果。'
    showResultCard(text, rect, reply, false, true)
  } catch (e) {
    showResultCard(text, rect, '搜索失败，请重试。', false, true)
  }
}
  onMounted(() => {
    const container = flipContainerRef.value
    if (container) {
      container.addEventListener('touchend', onContainerTouchEnd, { passive: false })
    }
    document.addEventListener('touchstart', onDocumentTouchStart, true)
    document.addEventListener('contextmenu', (e) => e.preventDefault())
  })

  onBeforeUnmount(() => {
    const container = flipContainerRef.value
    if (container) {
      container.removeEventListener('touchend', onContainerTouchEnd)
    }
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