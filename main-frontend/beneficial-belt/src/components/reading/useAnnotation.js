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
    const content = range.extractContents()
    const span = document.createElement('span')
    span.className = 'shanxi-highlight'
    span.appendChild(content)
    span.style.cssText = `
      background-color: rgba(180, 80, 50, 0.25) !important;
      display: inline !important;
      line-height: inherit !important;
      font-size: inherit !important;
      font-family: inherit !important;
      font-weight: inherit !important;
      font-style: inherit !important;
      vertical-align: baseline !important;
      white-space: inherit !important;
      word-spacing: inherit !important;
      letter-spacing: inherit !important;
      text-indent: inherit !important;
      text-transform: inherit !important;
      padding: 0 !important;
      margin: 0 !important;
      border: none !important;
      outline: none !important;
      box-shadow: none !important;
      border-radius: 0 !important;
      width: auto !important;
      height: auto !important;
      float: none !important;
      clear: none !important;
      position: static !important;
      top: auto !important;
      left: auto !important;
    `
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
          message: `请用一句话共读以下文字：“${text}”，语气像书友，结合理解和认识，200字内。`
        })
      })
      const data = await response.json()
      return data.reply || data.message || data.content || '此处妙极，值得细品。'
    } catch (e) {
      return '杉汐暂时无法共读，但此句确有深意。'
    }
  }

  function showResultCard(text, rangeOrRect, resultText, save = true) {
    const maxLen = 200
    const truncated = resultText.length > maxLen ? resultText.substring(0, maxLen) + '…' : resultText

    // 卡片固定底部，水平占满，圆角在上方
    commentCardStyle.value = {
      position: 'fixed',
      bottom: '0',
      left: '0',
      right: '0',
      maxWidth: '100%',
      maxHeight: '40vh',
      overflowY: 'auto',
      zIndex: 99999,
      background: '#f7e9d0',
      border: '1px solid #b8977a',
      borderRadius: '12px 12px 0 0',
      padding: '16px',
      boxShadow: '0 -4px 12px rgba(0,0,0,0.15)',
      wordBreak: 'break-word',
      display: 'block'
    }

    showCommentCard.value = true
    displayedComment.value = ''
    commentTyping.value = true
    typewrite(truncated)

    if (save) {
      annotations.value.push({
        text: selectedText.value || text,
        comment: resultText,
        page: currentPage.value,
        time: Date.now()
      })
      saveAnnotations()
    }
  }

  function closeCard() {
    showCommentCard.value = false
    clearInterval(typingTimer)
    commentTyping.value = false
    commentCardStyle.value = {}   // 重置样式，避免影响下次
  }

  function closeActionMenu() {
    showActionMenu.value = false
    if (outsideClickListener) {
      document.removeEventListener('mousedown', outsideClickListener)
      outsideClickListener = null
    }
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
    if (top + 60 > window.innerHeight) top = rect.top - 60

    actionMenuStyle.value = {
      position: 'fixed',
      left: `${left}px`,
      top: `${top}px`,
      width: `${menuWidth}px`
    }
    showActionMenu.value = true

    if (outsideClickListener) document.removeEventListener('mousedown', outsideClickListener)
    outsideClickListener = (e) => {
      if (!e.target.closest('.action-menu')) closeActionMenu()
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
    highlightText,
    generateComment,
    showResultCard,
    closeCard
  }
}