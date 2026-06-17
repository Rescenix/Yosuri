import { ref, computed, watch, nextTick, onMounted } from 'vue'

import { useMemory } from '../composables/useMemory.js'
import { useWelcome } from '../composables/useWelcome.js'
import { useChatLogic } from '../composables/useChatLogic.js'
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

  const isLoggedIn = ref(false)
  const debugTemp = ref(localStorage.getItem('debugTemp') ? parseFloat(localStorage.getItem('debugTemp')) : 0.7)
  const debugTopP = ref(localStorage.getItem('debugTopP') ? parseFloat(localStorage.getItem('debugTopP')) : 0.9)
  const debugReasoning = ref(localStorage.getItem('debugReasoning') || '')
  const lastTokenUsage = ref('')
  const lastLatency = ref('')
  const debugMaxTokens = ref(localStorage.getItem('debugMaxTokens') ? parseInt(localStorage.getItem('debugMaxTokens')) : 2000)
  const balance = ref('')

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

  async function fetchBalance() {
    try {
      const res = await fetch(`${apiBase}/api/balance`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      })
      if (res.ok) {
        const data = await res.json()
        if (data.is_available && data.balance_infos.length > 0) {
          const info = data.balance_infos[0]
          balance.value = `${info.total_balance} ${info.currency}`
        }
      }
    } catch (e) {
      console.warn('余额查询失败', e)
    }
  }

  function adjustInputHeight() {
    if (!chatInputRef.value) return
    chatInputRef.value.style.height = 'auto'
    chatInputRef.value.style.height = Math.min(chatInputRef.value.scrollHeight, 200) + 'px'
  }

  const { sendMessage } = useChatLogic({
    messages, userInput, sessionId,
    saveMemory, lastTokenUsage, lastLatency,
    onNewMessage: () => {
      forceScrollToBottom()
      nextTick(() => {
        if (chatInputRef.value) chatInputRef.value.style.height = 'auto'
      })
    },
    onStreamUpdate: smartScrollAndRefresh
  })

  // ==================== 图片上传逻辑 ====================
  const imageInput = ref(null)
  let msgId = 0

  async function handleImageUpload(e) {
    const file = e.target.files[0]
    if (!file) return

    // 1. 显示用户图片
    messages.value.push({
      id: msgId++,
      type: 'image',
      image: URL.createObjectURL(file),
      sender: 'user',
      timestamp: new Date()
    })

    // 2. 创建杉汐占位消息
    const botMsg = {
      id: msgId++,
      content: '',
      reasoning: '',
      recalling: true,
      toolCallName: null,
      toolCallDetail: '',
      sender: 'bot',
      isStreaming: true,
      timestamp: new Date()
    }
    messages.value.push(botMsg)
    forceScrollToBottom()

    // 3. 图片转 Base64
    const reader = new FileReader()
    reader.onload = async (loadEvent) => {
      const base64 = loadEvent.target.result.split(',')[1]

      const requestBody = {
        message: '帮我看看这张图片',
        sessionId: sessionId.value,
        image: base64,
        temperature: parseFloat(localStorage.getItem('debugTemp') || 0.7),
        top_p: parseFloat(localStorage.getItem('debugTopP') || 0.9),
        max_tokens: parseInt(localStorage.getItem('debugMaxTokens') || 2000),
        reasoning_effort: localStorage.getItem('debugReasoning') || undefined
      }

      const token = localStorage.getItem('token')
      const headers = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      try {
        const response = await fetch('/api/chat/stream', {
          method: 'POST',
          headers,
          body: JSON.stringify(requestBody)
        })

        if (!response.ok) throw new Error('网络错误')

        const streamReader = response.body.getReader()
        const decoder = new TextDecoder()
        let partialLine = ''

        while (true) {
          const { done, value } = await streamReader.read()
          if (done) break

          const chunk = decoder.decode(value, { stream: true })
          const lines = (partialLine + chunk).split('\n')
          partialLine = lines.pop() || ''

          for (const line of lines) {
            if (!line.startsWith('data: ')) continue
            const dataStr = line.slice(6)
            if (!dataStr) continue

            try {
              const payload = JSON.parse(dataStr)
              switch (payload.type) {
                case 'reasoning':
                  botMsg.recalling = false
                  botMsg.reasoning += payload.content || ''
                  break
                case 'content':
                  botMsg.recalling = false
                  botMsg.content += payload.content || ''
                  break
                case 'tool_call_start':
                  botMsg.toolCallName = payload.name || ''
                  botMsg.toolCallDetail = payload.args || ''
                  break
                case 'tool_call_result':
                case 'tool_call_error':
                  botMsg.toolCallName = null
                  botMsg.toolCallDetail = ''
                  if (payload.type === 'tool_call_error') {
                    botMsg.content += `\n[工具调用失败: ${payload.error}]\n`
                  }
                  break
                case 'done':
                  botMsg.content = payload.content || botMsg.content
                  botMsg.reasoning = payload.reasoning || botMsg.reasoning
                  botMsg.isStreaming = false
                  botMsg.recalling = false
                  botMsg.toolCallName = null
                  botMsg.toolCallDetail = ''
                  if (payload.token_usage) lastTokenUsage.value = payload.token_usage
                  if (payload.latency) lastLatency.value = payload.latency
                  if (saveMemory) {
                    saveMemory('leader', requestBody.message)
                    saveMemory('shanshi', botMsg.content)
                  }
                  break
                case 'error':
                  botMsg.content = `杉汐：抱歉，图片分析失败：${payload.message || '未知错误'}`
                  botMsg.isStreaming = false
                  botMsg.recalling = false
                  break
              }
              smartScrollAndRefresh()
            } catch (e) {}
          }
        }
        botMsg.isStreaming = false
        botMsg.recalling = false
        smartScrollAndRefresh()
      } catch (err) {
        botMsg.content = '杉汐：抱歉，我的灵魂好像被风吹散了…稍等片刻可好？'
        botMsg.isStreaming = false
        botMsg.recalling = false
        smartScrollAndRefresh()
      }
    }
    reader.readAsDataURL(file)
  }
  // ==================== 图片上传逻辑结束 ====================

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
      if (!isMobile.value) {
        isExpanded.value = true
      }
      nextTick(() => forceScrollToBottom())
      setTimeout(() => forceScrollToBottom(), 200)
    }
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

  const apiBase = import.meta.env.VITE_API_BASE || ''

  async function loadAllHistory() {
    try {
      const res = await fetch(`${apiBase}/api/all-messages`)
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

    localStorage.setItem('token', 'dev-permanent-token')
    isLoggedIn.value = true
    await loadAllHistory()
    fetchBalance()
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
    toggleExpand, toggleChat, updateParams, fetchBalance,
    groupedMessages, formatChatTime
  }
}