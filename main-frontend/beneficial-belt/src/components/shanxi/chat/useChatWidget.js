import { ref, computed, watch, nextTick, onMounted } from 'vue'

import { useMemory } from '../composables/useMemory.js'
import { useWelcome } from '../composables/useWelcome.js'
import { useChatLogic } from '../composables/useChatLogic.js'
import { useAgentWorkflow } from '../composables/useAgentWorkflow.js'
import { useVoicePlay } from '../composables/useVoicePlay.js'
import { useStatusPolling } from '../composables/useStatusPolling.js'
import { sessionTokenStats, loadSessionTokenStats } from '../composables/sessionTokenStats.js'

export function useChatWidget(props) {
  const isOpen = ref(false)
  const isExpanded = ref(false)
  const userInput = ref('')
  const messages = ref([])
  const sessionId = ref(
  localStorage.getItem('prism_session_id') || 
  'sess_' + Date.now().toString(36)
)
if (!localStorage.getItem('prism_session_id')) {
  localStorage.setItem('prism_session_id', sessionId.value)
}

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

  // 两个滚动函数都推到 nextTick 里执行——调用方基本都是紧跟在 messages.value.push(...)
  // 后面同步调用的，这时候 Vue 还没把新消息patch进 DOM，scrollHeight 量到的是旧高度，
  // 滚动会停在"上一条消息的底部"而不是真正的新底部，用户直观感觉就是"发消息不自动滚动"
  function forceScrollToBottom() {
    nextTick(() => {
      if (!messagesContainer.value) return
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
      userScrolledUp.value = false
    })
  }

  function smartScrollToBottom() {
    nextTick(() => {
      if (!messagesContainer.value || userScrolledUp.value) return
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    })
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

 // 自适应高度：内容多了就长高（到 max-height 封顶后内部滚动）。
 // 关键——绝对不能碰 scrollTop：之前每次输入都强制 scrollTop=0，本意是"复位"，
 // 实际是把光标所在行滚出可视区，正是"光标乱飘/看不见"的元凶。浏览器天然会让
 // 光标跟随可见，不去干预它就对了。
function adjustInputHeight() {
  if (!chatInputRef.value) return;
  const el = chatInputRef.value;
  // 先塌回 auto 量出真实内容高度，再赋值，保证删内容时也能回弹变矮
  el.style.height = 'auto';
  el.style.height = el.scrollHeight + 'px';
}

  const { sendMessage, sendWorkflow, stopWorkflow, workflowState, tokenStats, chatState } = useChatLogic({
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

  // 四态机 Code 工作流（GET /api/code/workflow，EventSource）
  // 流式期间用 forceScrollToBottom：长工作流流式中持续跟底，无视用户是否上滑，
  // 避免 smartScrollToBottom 因 userScrolledUp 被置 true 后永远不滚（原本的卡死缺陷）
  const { flowState, startCodeWorkflow: startFlow, stopCodeWorkflow } = useAgentWorkflow({
    messages,
    onNewMessage: forceScrollToBottom,
    // 流式增量用 smartScrollAndRefresh：尊重 userScrolledUp，用户上滑时不再被强制拉回底部
    onStreamUpdate: smartScrollAndRefresh
  })
  // display 透传给 startFlow——之前这里漏了第二个参数，附件 chip/纯文本气泡的展示信息
  // 全部在这层被吞掉，气泡又会退回显示拍平后的 task 全文
  function startCodeWorkflow(task, display) {
    startFlow(task, display)
    userInput.value = ''
  }

  // ==================== 图片上传逻辑（整合进 useChatWidget） ====================
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
                 console.log('[SSE event]', payload.type, payload); 
              switch (payload.type) {
               case 'reasoning':
  // 将推理内容追加到当前 bot 消息的 reasoning 字段上
  const botMsg = messages.value[messages.value.length - 1];
  if (botMsg && botMsg.sender === 'bot') {
    botMsg.reasoning = (botMsg.reasoning || '') + (payload.content || '');
    botMsg.recalling = false;
  }
  break;
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
                   if (payload.token_usage) lastTokenUsage.value = payload.token_usage
    if (payload.latency) lastLatency.value = payload.latency
                  botMsg.toolCallDetail = ''
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
      isExpanded.value = true
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
     const res = await fetch(`${apiBase}/api/sessions/${sessionId.value}`)
     if (res.ok) {
       const history = await res.json()
       // 后端对不存在/空的会话返回 null 或空 body，这里兜底成数组，避免 null.map 崩溃
       const list = Array.isArray(history) ? history : []
       messages.value = list.map((item, idx) => ({
         id: idx,
         content: cleanContent(item?.content ?? ''),
         sender: item?.role === 'assistant' ? 'bot' : (item?.role ?? 'user'),
         timestamp: item?.timestamp || new Date(),
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

// 真正切换到另一个后端会话（不只是改左侧列表的高亮）：立即清空当前消息，
// 避免切换瞬间残留上一个会话的内容，再按新 id 去加载历史——新会话/没有
// 历史记录的会话会得到空数组，messages.length===0 时首页视图自然显示
async function switchSession(id) {
  if (!id || id === sessionId.value) return
  sessionId.value = id
  localStorage.setItem('prism_session_id', id)
  // 切会话时同步恢复该会话持久化的真实 token（横条绑定会话，刷新/切换都不丢）
  sessionTokenStats.value = loadSessionTokenStats(id)
  messages.value = []
  await loadAllHistory()
}

  let lastScrollTop = 0
  onMounted(async () => {
    if (window.location.pathname.startsWith('/chat')) {
      isOpen.value = true
      isExpanded.value = true
    }
    if (props.autoOpen) {
      isOpen.value = true
      isExpanded.value = true
    }

    localStorage.setItem('token', 'dev-permanent-token')
    isLoggedIn.value = true
    await loadAllHistory()
    fetchBalance()
    // 初始化时恢复当前会话持久化的真实 token（横条绑定会话，刷新不丢）
    sessionTokenStats.value = loadSessionTokenStats(sessionId.value)
  })

  // 滚动监听挂在 messagesContainer ref 上（watch 而非 onMounted）：
  // 该容器是 v-else 条件渲染，仅 messages 非空时才创建 DOM。onMounted 时若首屏
  // 无消息，ref 为 null，监听会静默失败（按钮首屏不出现、上滑无法打断置底），
  // 刷新后因时机巧合才偶尔正常。watch ref 一旦绑定上 DOM 就挂，彻底规避时序问题。
  watch(messagesContainer, (el) => {
    if (!el) return
    lastScrollTop = el.scrollTop
    el.addEventListener('scroll', () => {
      const cur = el.scrollTop
      const maxScroll = el.scrollHeight - el.clientHeight
      const isAtBottom = Math.abs(cur - maxScroll) < 10
      if (isAtBottom) {
        userScrolledUp.value = false
      } else if (cur < lastScrollTop) {
        // 仅上滑（用户主动往上翻）时打断自动置底；流式下拉不算
        userScrolledUp.value = true
      }
      lastScrollTop = cur
    }, { passive: true })
  }, { immediate: true })

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

  // 后台任务清单（BackgroundTasksPanel 用）：
  // - 旧工作流的 kind:'group' 消息（形状本来就匹配面板）
  // - 四态机工作流派发的雨燕子代理（agentflow.subagents），点击跳转到所属流
  const backgroundTaskList = computed(() => {
    const out = []
    for (const m of messages.value) {
      if (m.kind === 'group') {
        out.push(m)
      } else if (m.kind === 'agentflow') {
        for (const sa of (m.subagents || [])) {
          out.push({
            id: m.id,               // 跳转目标 = 所属的 agentflow 消息
            key: `${m.id}_${sa.id}`, // 面板渲染 key（同一流可有多只雨燕）
            agentLabel: '雨燕',
            description: sa.task,
            status: sa.status,
            startTime: sa.startTime,
            endTime: sa.endTime,
            totalTokens: 0,
            toolUseCount: (sa.tools || []).length
          })
        }
      }
    }
    return out
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
    isOpen, isExpanded, userInput, messages, sessionId,
    isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
    welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
    messagesContainer, chatInputRef, userScrolledUp,
    forceScrollToBottom, smartScrollToBottom, smartScrollAndRefresh, adjustInputHeight, switchSession,
    sendMessage, sendWorkflow, stopWorkflow, workflowState, tokenStats, chatState, backgroundTaskList, handleImageUpload, playVoice,
    flowState, startCodeWorkflow, stopCodeWorkflow,
    toggleExpand, toggleChat, updateParams, fetchBalance,
    groupedMessages, formatChatTime
  }
}