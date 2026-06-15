import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useEmotion } from '../composables/useEmotion.js'
import { useMemory } from '../composables/useMemory.js'
import { useWelcome } from '../composables/useWelcome.js'
import { useChatLogic } from '../composables/useChatLogic.js'
import { useImageUpload } from '../composables/useImageUpload.js'
import { useVoicePlay } from '../composables/useVoicePlay.js'
import { useStatusPolling } from '../composables/useStatusPolling.js'

export function useChatWidget(props) {
  const isOpen = ref(false)
  const isExpanded = ref(false)
  const userInput = ref('')
  const messages = ref([])
 const sessionId = ref(props.sessionId || 'sess_' + Date.now().toString(36))

const isMobile = computed(() => {
  return typeof window !== 'undefined' && window.innerWidth <= 768
})

  watch(() => props.sessionId, (newVal) => {
    if (newVal) sessionId.value = newVal
  })

  const isLoggedIn = ref(!!localStorage.getItem('token'))
  const debugTemp = ref(localStorage.getItem('debugTemp') ? parseFloat(localStorage.getItem('debugTemp')) : 0.7)
  const debugTopP = ref(localStorage.getItem('debugTopP') ? parseFloat(localStorage.getItem('debugTopP')) : 0.9)
  const debugReasoning = ref(localStorage.getItem('debugReasoning') || '')
  const lastTokenUsage = ref('')
  const lastLatency = ref('')
  const debugMaxTokens = ref(localStorage.getItem('debugMaxTokens') ? parseInt(localStorage.getItem('debugMaxTokens')) : 2000)
  const balance = ref('')

  const { updateEmotion } = useEmotion()
  const { saveMemory } = useMemory()
  const { welcomeMessage, welcomeLoading } = useWelcome()
  const { currentStatus } = useStatusPolling()

  const messagesContainer = ref(null)
  const chatInputRef = ref(null)
  const userScrolledUp = ref(false)

  function forceScrollToBottom() {
    if (!messagesContainer.value) return
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    userScrolledUp.value = false
  }

  function smartScrollToBottom() {
    if (!messagesContainer.value || userScrolledUp.value) return
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }

  function smartScrollAndRefresh() {
    smartScrollToBottom()
    messages.value = [...messages.value]
  
  }

  function adjustInputHeight() {
    if (!chatInputRef.value) return
    chatInputRef.value.style.height = 'auto'
    chatInputRef.value.style.height = Math.min(chatInputRef.value.scrollHeight, 200) + 'px'
  }

  const { sendMessage } = useChatLogic({
    messages, userInput, sessionId,
    updateEmotion, saveMemory, lastTokenUsage, lastLatency,
    onNewMessage: () => {
      forceScrollToBottom()
      nextTick(() => {
        if (chatInputRef.value) chatInputRef.value.style.height = 'auto'
      })
    },
    onStreamUpdate: smartScrollAndRefresh
  })

  const { imageInput, handleImageUpload } = useImageUpload({ messages, sessionId, saveMemory })
  const { playVoice } = useVoicePlay()

  function toggleExpand() {
    isExpanded.value = !isExpanded.value
  }

function toggleChat() {
    if (props.autoOpen || (typeof window !== 'undefined' && window.location.pathname.startsWith('/chat'))) {
      window.location.href = '/'
      return
    }
    isOpen.value = !isOpen.value
    if (isOpen.value) {
      // 桌面端打开时默认放大
      if (!isMobile.value) {
        isExpanded.value = true
      }
      nextTick(() => forceScrollToBottom())
      setTimeout(() => forceScrollToBottom(), 200)
    }
  }

  async function fetchBalance() {
    try {
      const res = await fetch('/api/balance')
      if (res.ok) {
        const data = await res.json()
        if (data.is_available && data.balance_infos?.length > 0) {
          balance.value = `${data.balance_infos[0].total_balance} ${data.balance_infos[0].currency}`
        } else {
          balance.value = '不可用'
        }
      }
    } catch {}
  }

  function updateParams() {
    localStorage.setItem('debugTemp', debugTemp.value)
    localStorage.setItem('debugTopP', debugTopP.value)
    localStorage.setItem('debugMaxTokens', debugMaxTokens.value)
    localStorage.setItem('debugReasoning', debugReasoning.value)
  }

  const statusDotColor = computed(() => {
    const status = currentStatus.value
    if (!status) return '#98a2b3'
    if (status.includes('活跃') || status.includes('在线') || status.includes('帮忙') || status.includes('聊聊天')) return '#12b76a'
    if (status.includes('发呆') || status.includes('思绪') || status.includes('休眠')) return '#f59e0b'
    if (status.includes('忙碌') || status.includes('整理') || status.includes('写文章')) return '#ef4444'
    return '#98a2b3'
  })

  function cleanContent(content) {
    return content ? content.replace(/\[(action|emotion):[^\]]*\]/g, '') : ''
  }

  let lastScrollTop = 0
  onMounted(async () => {
    if (window.location.pathname.startsWith('/chat')) {
      isOpen.value = true
      if (!isMobile.value) isExpanded.value = true
    }
    if (props.autoOpen) {
      isOpen.value = true
      if (!isMobile.value) isExpanded.value = true
    }

    fetchBalance()
    await loadAllHistory()

    if (messagesContainer.value) {
      messagesContainer.value.addEventListener('scroll', () => {
        const el = messagesContainer.value
        const currentScrollTop = el.scrollTop
        const maxScroll = el.scrollHeight - el.clientHeight
        const isAtBottom = Math.abs(currentScrollTop - maxScroll) < 10
        if (isAtBottom) {
          userScrolledUp.value = false
        } else if (currentScrollTop < lastScrollTop) {
          userScrolledUp.value = true
        }
        lastScrollTop = currentScrollTop
      })
    }
  })

  async function loadAllHistory() {
    try {
      const res = await fetch('/api/all-messages')
      if (res.ok) {
        const history = await res.json()
        messages.value = history.map((item, idx) => ({
          id: idx,
          content: cleanContent(item.content),
          sender: item.role === 'assistant' ? 'bot' : item.role,
          timestamp: item.timestamp,
          isStreaming: false,
          reasoning: ''
        }))
        await nextTick()
        forceScrollToBottom()
      }
    } catch (e) {
      console.error('加载历史失败', e)
    }
  }

  function shouldShowTime(prevMsg, currentMsg) {
    if (!prevMsg) return true
    const prevTime = new Date(prevMsg.timestamp)
    const currTime = new Date(currentMsg.timestamp)
    if (prevTime.toDateString() !== currTime.toDateString()) return true
    const diffMinutes = (currTime - prevTime) / (1000 * 60)
    return diffMinutes > 5
  }

  const groupedMessages = computed(() => {
    const result = []
    for (let i = 0; i < messages.value.length; i++) {
      const msg = messages.value[i]
      const prevMsg = i > 0 ? messages.value[i-1] : null
      if (shouldShowTime(prevMsg, msg)) {
        result.push({ type: 'time', timestamp: msg.timestamp, id: `time-${i}` })
      }
      result.push({ type: 'message', ...msg })
    }
    return result
  })

  function formatChatTime(timestamp) {
    if (!timestamp) return ''
    const date = new Date(timestamp)
    const now = new Date()
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
    const yesterday = new Date(today.getTime() - 86400000)
    const msgDate = new Date(date.getFullYear(), date.getMonth(), date.getDate())
    if (msgDate.getTime() === today.getTime()) {
      return `${date.getHours().toString().padStart(2,'0')}:${date.getMinutes().toString().padStart(2,'0')}`
    } else if (msgDate.getTime() === yesterday.getTime()) {
      return `昨天 ${date.getHours().toString().padStart(2,'0')}:${date.getMinutes().toString().padStart(2,'0')}`
    } else {
      return `${date.getMonth()+1}/${date.getDate()} ${date.getHours().toString().padStart(2,'0')}:${date.getMinutes().toString().padStart(2,'0')}`
    }
  }

  return {
    isOpen, isExpanded, isMobile, userInput, messages, sessionId,
    isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
    welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
    messagesContainer, chatInputRef, userScrolledUp,
    forceScrollToBottom, smartScrollToBottom, smartScrollAndRefresh, adjustInputHeight,
    sendMessage, handleImageUpload, playVoice,
    toggleExpand, toggleChat, fetchBalance, updateParams,
    groupedMessages, formatChatTime
  }
}