import { ref, reactive, computed, watch, nextTick, onMounted } from 'vue'

import { useWelcome } from '../composables/useWelcome.js'
import { useAgentWorkflow } from '../composables/useAgentWorkflow.js'
import { useVoicePlay } from '../composables/useVoicePlay.js'
import { useStatusPolling } from '../composables/useStatusPolling.js'
import { sessionTokenStats, loadSessionTokenStats } from '../composables/sessionTokenStats.js'
import { loadContextBreakdown } from '../composables/contextBreakdown.js'
import { useAuth } from '../../../composables/useAuth.js'

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

  // 登录态统一由 useAuth 管理（含验真 + 拉用户名/头像），不再自行伪造占位 token
  const isLoggedIn = useAuth().isLoggedIn
  const debugTemp = ref(localStorage.getItem('debugTemp') ? parseFloat(localStorage.getItem('debugTemp')) : 0.7)
  const debugTopP = ref(localStorage.getItem('debugTopP') ? parseFloat(localStorage.getItem('debugTopP')) : 0.9)
  const debugReasoning = ref(localStorage.getItem('debugReasoning') || '')
  const debugMaxTokens = ref(localStorage.getItem('debugMaxTokens') ? parseInt(localStorage.getItem('debugMaxTokens')) : 2000)
  const balance = ref('')

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

  // 四态机 Code 工作流（GET /api/code/workflow，EventSource）
  // 流式期间用 forceScrollToBottom：长工作流流式中持续跟底，无视用户是否上滑，
  // 避免 smartScrollToBottom 因 userScrolledUp 被置 true 后永远不滚（原本的卡死缺陷）
  const {
    flowState, approvalState, respondApproval, startCodeWorkflow: startFlow, stopCodeWorkflow,
    resumeState, refreshResumable, resumeCodeWorkflow, dismissResumable, todoState, sendSteerMessage
  } = useAgentWorkflow({
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

  // 工具参数在后端一路都是原始 JSON 串（模型吐什么存什么），坏串也不该让整段历史崩掉
  function parseArgs(raw) {
    if (!raw) return {}
    if (typeof raw === 'object') return raw
    try { return JSON.parse(raw) } catch { return {} }
  }

  function cleanContent(content) {
    return content ? content.replace(/\[(action|emotion):[^\]]*\]/g, '') : ''
  }

  function extOf(name) {
    const m = /\.([a-zA-Z0-9]+)$/.exec(name || '')
    return m ? m[1].toUpperCase() : 'FILE'
  }

  // 发送时 buildOutgoingMessage() 把附件拍平进正文（"[文件: x.py]"/"[文件夹: x，共 N 个文件]\n<清单>"），
  // 只在实时会话里才有单独的 attachments 数组渲染成 chip；历史一刷新回来就只剩这段拍平文本，
  // 于是旧消息显示成一行方括号裸文本，跟当前会话里的附件 chip 长得不一样。
  // 这里把 buildOutgoingMessage 的拼接逆过来，从正文头部识别出这些标记块，
  // 还原成 attachments 数组交给 AttachmentChipRow，跟实时发送时的气泡外观对齐。
  // 只处理 文件/文件夹（单行 或 行数由 fileCount 精确推出，边界无歧义）；
  // 图片块的分析文本长度不固定，无法安全地和后面用户自己打的字分开，不处理，保留原样。
  function extractAttachmentsFromContent(content) {
    const lines = (content ?? '').split('\n')
    const attachments = []
    let i = 0
    let seq = 0
    while (i < lines.length) {
      const fileMatch = /^\[文件: (.+)\]$/.exec(lines[i])
      if (fileMatch) {
        attachments.push({ id: `hist_${seq++}`, kind: 'file', name: fileMatch[1], ext: extOf(fileMatch[1]), status: 'ready' })
        i++
        continue
      }
      const folderMatch = /^\[文件夹: (.+)，共 (\d+) 个文件\]$/.exec(lines[i])
      if (folderMatch) {
        const fileCount = parseInt(folderMatch[2], 10)
        attachments.push({ id: `hist_${seq++}`, kind: 'folder', name: folderMatch[1], fileCount, status: 'ready' })
        i++
        // 清单正文行数与 onAttachFolderSelected 的截断规则（最多 200 行 + 超限提示行）一致，
        // 由 fileCount 精确算出要跳过几行，不用猜清单在哪结束
        i += Math.min(fileCount, 200) + (fileCount > 200 ? 1 : 0)
        continue
      }
      break
    }
    if (attachments.length === 0) return null
    return { attachments, text: lines.slice(i).join('\n').trim() }
  }

  const apiBase = import.meta.env.VITE_API_BASE || ''

 async function loadAllHistory() {
   const id = sessionId.value
   try {
     const res = await fetch(`${apiBase}/api/sessions/${id}`)
     // 竞态守卫：请求在途时用户又切了会话，这份结果已过期，直接丢弃，
     // 否则后返回的旧会话历史会覆盖新会话的内容
     if (sessionId.value !== id) return
     if (res.ok) {
       const history = await res.json()
       if (sessionId.value !== id) return
       // 后端对不存在/空的会话返回 null 或空 body，这里兜底成数组，避免 null.map 崩溃
       const list = Array.isArray(history) ? history : []
       messages.value = list.map((item, idx) => {
         // 四态机工作流留下的轨迹（后端 FlowBlock）：还原成一条 agentflow 消息，
         // AgentWorkflowPanel 照常渲染，工具行和展开的 Diff/输出跟刚跑完时一样。
         if (item?.blocks?.length) {
           return {
             id: idx,
             kind: 'agentflow',
             sender: 'bot',
             status: 'completed',
             blocks: item.blocks.map(b => ({
               ...b,
               // 落盘的是原始 JSON 参数串（跟 SSE action 事件同口径），面板要对象
               args: parseArgs(b.args),
               expanded: false
             })),
             subagents: [],
             timestamp: item?.timestamp || new Date()
           }
         }
         const role = item?.role === 'assistant' ? 'bot' : (item?.role ?? 'user')
         const extracted = role === 'user' ? extractAttachmentsFromContent(item?.content) : null
         return {
           id: idx,
           content: cleanContent(extracted ? extracted.text : (item?.content ?? '')),
           attachments: extracted ? extracted.attachments : [],
           sender: role,
           timestamp: item?.timestamp || new Date(),
           isStreaming: false,
           reasoning: ''
         }
       })
       await nextTick()
       forceScrollToBottom()
     }
   } catch (e) {
     console.error('加载历史失败', e)
     // 加载失败时别把上一个会话的内容留在屏幕上冒充新会话
     if (sessionId.value === id) messages.value = []
   }
 }

// 真正切换到另一个后端会话（不只是改左侧列表的高亮）。
// 不预先清空 messages：清空会让 messages.length===0 的首页视图闪一下再跳到
// 新会话（"闪烁 bug"）。改为等新历史拿到后一次性替换——期间短暂显示旧会话
// 内容，比闪首页顺眼；竞态由 loadAllHistory 里的 id 守卫兜住。
async function switchSession(id) {
  if (!id || id === sessionId.value) return
  sessionId.value = id
  localStorage.setItem('prism_session_id', id)
  // 切会话时同步恢复该会话持久化的真实 token（横条绑定会话，刷新/切换都不丢）
  sessionTokenStats.value = loadSessionTokenStats(id)
  // 上下文分类明细同样要跟着会话走。之前只在 ChatWidget setup 时 load 过一次，
  // 切会话不重载 —— 结果面板一直显示上一个会话的分类，只有刷新页面才纠正。
  loadContextBreakdown(id)
  // 新会话里可能躺着上次没跑完的工作流（后端重启/断线留下的检查点）
  refreshResumable()
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

    // 登录态交由 useAuth 统一管理（启动时会用 localStorage 里的真 token 验真）；
    // 不再写入占位 token 覆盖 GitHub 真登录换来的 JWT。
    await loadAllHistory()
    fetchBalance()
    // 初始化时恢复当前会话持久化的真实 token（横条绑定会话，刷新不丢）
    sessionTokenStats.value = loadSessionTokenStats(sessionId.value)
    // 关掉页面/后端崩了之后重新打开：把没跑完的工作流捞出来问要不要续跑
    refreshResumable()
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
    isLoggedIn, debugTemp, debugTopP, debugReasoning, debugMaxTokens, balance,
    welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
    messagesContainer, chatInputRef, userScrolledUp,
    forceScrollToBottom, smartScrollToBottom, smartScrollAndRefresh, adjustInputHeight, switchSession,
    backgroundTaskList, playVoice,
    flowState, startCodeWorkflow, stopCodeWorkflow, approvalState, respondApproval,
    resumeState, resumeCodeWorkflow, dismissResumable, todoState, sendSteerMessage,
    toggleExpand, toggleChat, updateParams,
    groupedMessages, formatChatTime
  }
}