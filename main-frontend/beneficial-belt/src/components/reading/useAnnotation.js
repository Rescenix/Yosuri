import { ref } from 'vue'

const STORAGE_KEY = 'shanxi_annotations'

export function useAnnotation(flipContainerRef, currentPage) {
  const showCommentCard = ref(false)
  const commentCardStyle = ref({})
  const displayedComment = ref('')
  const commentTyping = ref(false)
  let typingTimer = null

  const showActionMenu = ref(false)
  const actionMenuStyle = ref({})
  const selectedText = ref('')
  const selectedRange = ref(null)

  const annotations = ref(JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]'))

  let outsideClickListener = null

  function saveAnnotations() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(annotations.value))
    window.dispatchEvent(new CustomEvent('annotations-updated'))
  }

  function highlightText(text, range) {
    const span = document.createElement('span')
    span.className = 'shanxi-highlight selected'
    span.textContent = text
    Object.assign(span.style, {
      outline: '2px solid rgba(180, 80, 50, 0.6)',
      outlineOffset: '1px',
      borderRadius: '6px',
      boxShadow: '0 0 0 3px rgba(180, 80, 50, 0.2)',
      display: 'inline',
      lineHeight: 'inherit',
      padding: '0',
      margin: '0'
    })
    range.deleteContents()
    range.insertNode(span)
  }

  function typewrite(text) {
    clearInterval(typingTimer)
    displayedComment.value = ''
    commentTyping.value = true
    let i = 0
    typingTimer = setInterval(() => {
      if (i < text.length) {
        displayedComment.value += text[i]
        i++
      } else {
        clearInterval(typingTimer)
        commentTyping.value = false
      }
    }, 40)
  }

  async function generateComment(text) {
    try {
      const response = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          message: `请用一句话批注以下文字：“${text}”，语气像脂砚斋，直接给出批语，不要多余解释。`
        })
      })
      const data = await response.json()
      return data.reply || data.message || data.content || '此处妙极，值得细品。'
    } catch (e) {
      return '杉汐暂时无法批注，但此句确有深意。'
    }
  }

function showResultCard(text, rangeOrRect, resultText, save = true) {
  let rect
  if (rangeOrRect && typeof rangeOrRect.left === 'number') {
    rect = rangeOrRect
  } else if (rangeOrRect) {
    rect = rangeOrRect.getBoundingClientRect()
  } else {
    rect = {
      left: window.innerWidth / 2,
      top: window.innerHeight / 2,
      width: 0,
      bottom: window.innerHeight / 2
    }
  }

  const cardWidth = Math.min(280, window.innerWidth - 32)
  let left = rect.left + rect.width / 2 - cardWidth / 2
  left = Math.max(8, Math.min(left, window.innerWidth - cardWidth - 8))
  let top = rect.bottom + 8

  // 先设置初始样式（隐藏，避免闪烁）
  commentCardStyle.value = {
    position: 'fixed',
    left: `${left}px`,
    top: `${top}px`,
    maxWidth: `${cardWidth}px`,
    zIndex: 99999,
    wordBreak: 'break-word',
    visibility: 'hidden'
  }

  showCommentCard.value = true
  displayedComment.value = ''
  commentTyping.value = true
  typewrite(resultText)

  if (save) {
    annotations.value.push({
      text: selectedText.value || text,
      comment: resultText,
      page: currentPage.value,
      time: Date.now()
    })
    saveAnnotations()
  }

  // 渲染后根据实际高度调整垂直位置
  requestAnimationFrame(() => {
    const card = document.querySelector('.comment-card')
    if (card) {
      const cardRect = card.getBoundingClientRect()
      // 底部溢出 → 移到选区上方
      if (cardRect.bottom > window.innerHeight - 8) {
        top = rect.top - cardRect.height - 8
        if (top < 8) top = 8
      }
      // 顶部溢出保护
      if (top < 8) top = 8
      // 水平再次确认
      if (cardRect.right > window.innerWidth - 8) {
        left = window.innerWidth - cardRect.width - 8
      }
      if (left < 8) left = 8

      commentCardStyle.value = {
        ...commentCardStyle.value,
        left: `${left}px`,
        top: `${top}px`,
        visibility: 'visible'
      }
    } else {
      commentCardStyle.value.visibility = 'visible'
    }
  })
}
  function closeCard() {
    showCommentCard.value = false
    clearInterval(typingTimer)
    commentTyping.value = false
  }

  function closeActionMenu() {
    showActionMenu.value = false
    if (outsideClickListener) {
      document.removeEventListener('mousedown', outsideClickListener)
      outsideClickListener = null
    }
    // 不清除选区，保留高亮
  }

  async function chooseComment() {
    const text = selectedText.value
    const range = selectedRange.value
    if (!text || !range) return
    closeActionMenu()
    highlightText(text, range)
    const comment = await generateComment(text)
    showResultCard(text, range, comment, true)
  }

  function chooseSearch() {
    const text = selectedText.value
    if (!text) return
    closeActionMenu()
    window.dispatchEvent(new CustomEvent('search-text', { detail: { text } }))
  }

  function onMouseUp(event) {
    // 桌面端专用，移动端不要调用此函数
    const selection = window.getSelection()
    const text = selection.toString().trim()
    if (text.length === 0) return

    event.stopPropagation()
    const range = selection.getRangeAt(0).cloneRange()
    selectedText.value = text
    selectedRange.value = range

    const rect = range.getBoundingClientRect()
    const menuWidth = 140
    let left = rect.left + rect.width / 2 - menuWidth / 2
    left = Math.max(8, Math.min(left, window.innerWidth - menuWidth - 8))
    let top = rect.bottom + 8
    if (top + 60 > window.innerHeight) {
      top = rect.top - 60
    }

    actionMenuStyle.value = {
      position: 'fixed',
      left: `${left}px`,
      top: `${top}px`,
      width: `${menuWidth}px`
    }
    showActionMenu.value = true

    // 不清除选区，保留蓝色高亮
    if (outsideClickListener) document.removeEventListener('mousedown', outsideClickListener)
    outsideClickListener = (e) => {
      if (!e.target.closest('.action-menu')) {
        closeActionMenu()
      }
    }
    setTimeout(() => document.addEventListener('mousedown', outsideClickListener), 100)
  }

  return {
    showCommentCard,
    commentCardStyle,
    displayedComment,
    commentTyping,
    annotations,
    showActionMenu,
    actionMenuStyle,
    onMouseUp,
    closeCard,
    chooseComment,
    chooseSearch,
    // 移动端需要的函数
    highlightText,
    generateComment,
    showResultCard,
    closeCard
  }
}