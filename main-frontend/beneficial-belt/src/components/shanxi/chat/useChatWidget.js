import { ref, reactive, computed, watch, nextTick, onMounted } from 'vue'

import { useWelcome } from '../composables/useWelcome.js'
import { useAgentWorkflow } from '../composables/useAgentWorkflow.js'
import { useAgentsStore } from '../composables/useAgents.js'
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

// 动态测量输入区（含 todo/ask 条）真实高度 → 设 --input-clearance 变量，
// 让消息内容底部留出等量空白，消息能滚到输入区/todo 条底下并从透明背景后面透出来（Hermes 式 overlay）。
function updateInputClearance() {
  const area = document.querySelector('.chat-input-area')
  if (!area) return
  const h = area.offsetHeight
  document.documentElement.style.setProperty('--input-clearance', h + 'px')
}
// 监听输入区尺寸变化（todo/ask 条出现/消失、输入框增高）动态刷新 clearance
function watchInputClearance() {
  const area = document.querySelector('.chat-input-area')
  if (!area) return
  const ro = new ResizeObserver(() => updateInputClearance())
  ro.observe(area)
  updateInputClearance()
}

  // 四态机 Code 工作流（GET /api/code/workflow，EventSource）
  // 流式期间用 forceScrollToBottom：长工作流流式中持续跟底，无视用户是否上滑，
  // 避免 smartScrollToBottom 因 userScrolledUp 被置 true 后永远不滚（原本的卡死缺陷）
  const {
      flowState, approvalState, respondApproval, startCodeWorkflow: startFlow, stopCodeWorkflow,
      todoState, sendSteerMessage,
      questionState, answerQuestion
    } = useAgentWorkflow({
      messages,
      onNewMessage: forceScrollToBottom,
      // 流式增量用 smartScrollAndRefresh：尊重 userScrolledUp，用户上滑时不再被强制拉回底部
      onStreamUpdate: smartScrollAndRefresh,
      onTitleUpdate: (title) => {
        // 这里直接调用 ChatWidget 里的 updateSessionTitle（通过 props 或全局）
        // 由于 useChatWidget 不暴露 updateSessionTitle，改为发事件供 ChatWidget 监听
        window.dispatchEvent(new CustomEvent('session-title-update', { detail: title }))
      }
    })
  // display 透传给 startFlow——之前这里漏了第二个参数，附件 chip/纯文本气泡的展示信息
  // 全部在这层被吞掉，气泡又会退回显示拍平后的 task 全文
  //
  // ── 多 Agent 群聊编排 ──
  // 当前会话挂了 N 个 Agent 时，一条用户消息按成员顺序依次点名：
  // 第一位说完（工作流收尾）才轮到第二位，第二位能看到第一位的发言
  // （后端历史里带「【某某 说】」标记）。串行而不是并发：免费模型池并发
  // 会互相抢源拖慢，而且群聊里"接话"本来就该有先后。
  const agentStore = useAgentsStore()
  onMounted(() => { if (!agentStore.agentsLoaded.value) agentStore.loadAgents() })

  function startCodeWorkflow(task, display, opts) {
    userInput.value = ''
    const sid = localStorage.getItem('prism_session_id') || sessionId.value
    const members = (opts && opts.groupIds) || agentStore.groupOf(sid)
    // 没挂群聊成员：单 Agent 老链路，行为完全不变。
    if (!members || members.length <= 1) {
      startFlow(task, display, {
        ...opts,
        agentId: (members && members[0]) || agentStore.currentAgentId.value || '',
      })
      return
    }
    let idx = 0
    const next = () => {
      if (idx >= members.length) return
      const id = members[idx]
      idx += 1
      startFlow(task, display, {
        ...opts,
        agentId: id,
        // 第一位插用户气泡，后面的只出自己的回复
        skipUserBubble: idx > 1,
        onDone: next,
      })
    }
    next()
  }

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

  // 中断/失败工作流的「中断前已执行步骤摘要」文本 → tool 块列表。
  // 后端 workflowHistoryContent 把它拼成 "- 工具名({json}) => 结果" 每行一条，
  // 但结果常是多行 JSON（输出被截断跨行），按行解析会拆散。
  // 这里按「- 工具名(」开头识别条目起点，整段归并，再切出 args 与 output。
  function parseInterruptedSummary(content) {
    const text = String(content || '')
    const blocks = []
    const prose = []
    const lines = text.split('\n')
    let i = 0
    while (i < lines.length) {
      const line = lines[i]
      // 条目起点：- 工具名({...})
      const m = /^\s*-\s*([A-Za-z_][\w]*)\s*\(/.exec(line)
      if (m) {
        // 找到该条目结束（下一个 "- 工具名(" 或文本结尾）
        let j = i + 1
        while (j < lines.length && !/^\s*-\s*[A-Za-z_][\w]*\s*\(/.test(lines[j])) j++
        const entry = lines.slice(i, j).join('\n')
        i = j
        // 从条目里拆 args（第一个 '(' 到配对的 ')' 之后）+ 剩余为 output
        const openIdx = entry.indexOf('(')
        const closeIdx = entry.lastIndexOf(')')
        let name = m[1], argsRaw = '', outRaw = ''
        if (openIdx >= 0) {
          const after = entry.slice(openIdx + 1)
          // args 是 {...} JSON，输出可能在 ) 后面有 "=> 结果"
          const braceMatch = /^\{.*\}/s.exec(after)
          if (braceMatch) {
            argsRaw = braceMatch[0]
            const rest = after.slice(braceMatch[0].length)
            const arrow = rest.indexOf('=>')
            outRaw = (arrow >= 0 ? rest.slice(arrow + 2) : rest).trim()
          } else {
            argsRaw = closeIdx > openIdx ? entry.slice(openIdx + 1, closeIdx) : after
          }
        }
        blocks.push({
          type: 'tool',
          name,
          args: parseArgs(argsRaw),
          status: 'ok',
          text: '',
          output: outRaw || '',
          expanded: false
        })
      } else if (line.trim()) {
        prose.push(line.trim())
        i++
      } else {
        i++
      }
    }
    // 摘要外的说明文字（如"用户主动停止了工作流。"）作为 intent 展示，
    // 但只有真的有工具行时才生成 agentflow（避免纯文本消息被误判）
    if (blocks.length && prose.length) {
      blocks.unshift({ type: 'intent', text: prose.join('\n') })
    }
    return { blocks, prose }
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
           // 改动文件卡片持久化：历史里存了 type=changed-files 的特殊块，
           // 重放时从中恢复 flow.changedFiles（2026-08-28）
           let changedFiles = []
           const kept = []
           for (const b of item.blocks) {
             if (b.type === 'changed-files') {
               changedFiles = Array.isArray(b.changed_files) ? b.changed_files : []
             } else {
               kept.push(b)
             }
           }
           return {
             id: idx,
             kind: 'agentflow',
             sender: 'bot',
             // 群聊：还原说话人（后端 persistedMessage.agent）
             agentId: item.agent || '',
             status: 'completed',
             blocks: kept.map(b => ({
               ...b,
               // 落盘的是原始 JSON 参数串（跟 SSE action 事件同口径），面板要对象
               args: parseArgs(b.args),
               expanded: false
             })),
             subagents: [],
             changedFiles,
             timestamp: item?.timestamp || new Date()
           }
         }
         const role = item?.role === 'assistant' ? 'bot' : (item?.role ?? 'user')
                 // 中断/失败的工作流：content 里只有「中断前已执行步骤摘要」纯文本
                 // （- ask_user({...}) => 结果 这种格式），没有 blocks 也没有 tool_calls。
                 // 解析成 tool 块渲染，而不是把 raw JSON 当聊天文本显示（2026-08-29 修复）
                 if (role === 'bot' && item?.content && item.content.includes('中断前已执行步骤摘要') && !item?.blocks?.length) {
                   const parsed = parseInterruptedSummary(item.content)
                   if (parsed.blocks.length) {
                     return {
                       id: idx,
                       kind: 'agentflow',
                       sender: 'bot',
                       agentId: item.agent || '',
                       status: 'completed',
                       blocks: parsed.blocks,
                       subagents: [],
                       changedFiles: [],
                       timestamp: item?.timestamp || new Date()
                     }
                   }
                 }
                 // 助理消息落盘时 tool_calls 可能存了原始 JSON，但 blocks 为空（旧存档/其他agent写入）。
                 // 把它渲染成 tool 块，而不是把 raw JSON 当聊天文本显示（2026-08-29 修复）
                 if (role === 'bot' && item?.tool_calls?.length && !item?.blocks?.length) {
                                    return {
                                      id: idx,
                                      kind: 'agentflow',
                                      sender: 'bot',
                                      agentId: item.agent || '',
                                      status: 'completed',
                                      blocks: item.tool_calls.map((tc, ti) => ({
                                        type: 'tool',
                                        name: tc.function?.name || tc.name || 'tool',
                                        args: parseArgs(tc.function?.arguments || tc.arguments || '{}'),
                                        status: 'ok',
                                        text: '',
                                        expanded: false
                                      })),
                                      subagents: [],
                                      changedFiles: [],
                                      timestamp: item?.timestamp || new Date()
                                    }
                                  }
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
  // 切换会话前清空上一个会话残留的悬浮条（todo 清单 / 未决提问 / 审批条），
  // 否则旧会话的条会一直挂在输入框上方，换对话还卡着（2026-08-31 实测）。
  todoState.items = []
  questionState.pending = null
  approvalState.pending = []
  sessionId.value = id
  localStorage.setItem('prism_session_id', id)
  // 切会话时同步恢复该会话持久化的真实 token（横条绑定会话，刷新/切换都不丢）
  sessionTokenStats.value = loadSessionTokenStats(id)
  // 上下文分类明细同样要跟着会话走。之前只在 ChatWidget setup 时 load 过一次，
  // 切会话不重载 —— 结果面板一直显示上一个会话的分类，只有刷新页面才纠正。
  loadContextBreakdown(id)
  await loadAllHistory()
}

  let lastScrollTop = 0
  // 输入框上方悬浮条（todo / askuser）随上滑淡出：距底部越远越透明（仿 Hermes）。
  // scroll 事件里直接算，离底部 0→FADE_RANGE 线性降到保底值，滚回底部恢复 1。
  const inputBarFade = ref(1)
  const INPUT_BAR_FADE_RANGE = 350 // px：滑出这么远才淡到保底值（渐变拉平缓，稍微上滑只轻微变淡）
  const INPUT_BAR_FADE_MIN = 0.3 // 保底透明度：留 30% 存在感，不完全透明（用户要求不能全透明）
  function updateInputBarFade(el) {
    const maxScroll = el.scrollHeight - el.clientHeight
    const dist = Math.max(0, maxScroll - el.scrollTop)
    const t = Math.min(1, dist / INPUT_BAR_FADE_RANGE)
    inputBarFade.value = 1 - t * (1 - INPUT_BAR_FADE_MIN)
  }
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
    // 动态测量输入区高度 → 设 --input-clearance（Hermes 式 overlay：消息可滚到输入区/todo 条底下）
    // ❌ 疑似思考流式卡顿元凶，先注释隔离测试（2026-08-31）
    // watchInputClearance()
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
    updateInputBarFade(el)
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
      updateInputBarFade(el)
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
  // - 四态机工作流派发的雨燕子代理（agentflow.subagents）
  // - run_task 后台进程（agentflow.bgTasks，与子代理分开）
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
        for (const bt of (m.bgTasks || [])) {
          out.push({
            id: bt.id,              // task_id，点「查看日志」走 /api/bg-task/log
            key: `${m.id}_${bt.id}`,
            agentLabel: '后台进程',
            description: bt.task,
            status: bt.status,
            startTime: bt.startTime,
            endTime: bt.endTime,
            totalTokens: 0,
            toolUseCount: (bt.tools || []).length
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

  // 进行中的后台任务数（输入框上方悬浮任务条用）——含雨燕子代理 + 旧 group 工作流
  const runningTaskCount = computed(() =>
    backgroundTaskList.value.filter(t => t.status === 'running').length
  )
  // 进行中的子代理数（仅雨燕，不含后台进程）
  const runningSubagentCount = computed(() => {
    let n = 0
    for (const m of messages.value) {
      if (m.kind === 'agentflow') {
        for (const sa of (m.subagents || [])) {
          if (sa.status === 'running') n++
        }
      }
    }
    return n
  })
  // 进行中的后台进程数（仅 run_task，不含子代理）
  const runningBgTaskCount = computed(() => {
    let n = 0
    for (const m of messages.value) {
      if (m.kind === 'agentflow') {
        for (const bt of (m.bgTasks || [])) {
          if (bt.status === 'running') n++
        }
      }
    }
    return n
  })

  // ===== 知识库抽屉 =====
  const kbOpen = ref(localStorage.getItem('kb_open') === '1')
  const kbFiles = ref([])
  const kbLoading = ref(false)
  const kbDragOver = ref(false)
  const kbUploadInputRef = ref(null)

  function toggleKb() {
    kbOpen.value = !kbOpen.value
    localStorage.setItem('kb_open', kbOpen.value ? '1' : '0')
    if (kbOpen.value) loadKb()
  }

  // 从后端拉真实知识库清单
  async function loadKb() {
    kbLoading.value = true
    try {
      const res = await fetch('/api/knowledge/list')
      if (!res.ok) throw new Error('加载失败')
      const data = await res.json()
      kbFiles.value = (data.files || []).map(f => ({
        id: f.name,
        name: f.name,
        size: formatKbSize(f.size),
        chunks: f.chunks,
      }))
    } catch (e) {
      kbFiles.value = []
    } finally {
      kbLoading.value = false
    }
  }

  function triggerKbUpload() {
    kbUploadInputRef.value?.click()
  }

  async function onKbUploadSelected(e) {
    const files = Array.from(e.target.files || [])
    e.target.value = ''
    await uploadKbFiles(files)
  }

  async function onKbDrop(e) {
    kbDragOver.value = false
    const files = Array.from(e.dataTransfer?.files || [])
    await uploadKbFiles(files)
  }

  async function uploadKbFiles(files) {
    for (const f of files) {
      const fd = new FormData()
      fd.append('file', f)
      try {
        const res = await fetch('/api/knowledge/upload', { method: 'POST', body: fd })
        if (!res.ok) {
          const err = await res.json().catch(() => ({}))
          window.alert('上传失败：' + (err.error || f.name))
        }
      } catch (e) {
        window.alert('上传失败：' + f.name)
      }
    }
    await loadKb()
  }

  async function addKbFiles(files) {
    await uploadKbFiles(files)
  }

  async function removeKbFile(id) {
    try {
      await fetch('/api/knowledge/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: id }),
      })
      await loadKb()
    } catch (e) {
      window.alert('删除失败')
    }
  }

  function insertKbRef(file) {
    // 插入文件引用到输入框，格式如 [📎 文件名]
    const ref = ` [📎 ${file.name}] `
    userInput.value += ref
  }

  function fileIcon(name) {
    const ext = (name.split('.').pop() || '').toLowerCase()
    const map = {
      pdf: 'mdi:file-pdf-box',
      doc: 'mdi:file-word-box', docx: 'mdi:file-word-box',
      xls: 'mdi:file-excel-box', xlsx: 'mdi:file-excel-box',
      ppt: 'mdi:file-powerpoint-box', pptx: 'mdi:file-powerpoint-box',
      png: 'mdi:file-image-box', jpg: 'mdi:file-image-box', jpeg: 'mdi:file-image-box', gif: 'mdi:file-image-box', webp: 'mdi:file-image-box',
      mp4: 'mdi:file-video-box', mov: 'mdi:file-video-box',
      mp3: 'mdi:file-music-box', wav: 'mdi:file-music-box',
      zip: 'mdi:file-archive-box', rar: 'mdi:file-archive-box',
      txt: 'mdi:file-document-outline',
      md: 'mdi:language-markdown',
      json: 'mdi:code-json',
      py: 'mdi:language-python',
      js: 'mdi:language-javascript',
      go: 'mdi:language-go',
    }
    return map[ext] || 'mdi:file-outline'
  }

  function formatKbSize(bytes) {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  }

  return {
    isOpen, isExpanded, userInput, messages, sessionId,
    isLoggedIn, debugTemp, debugTopP, debugReasoning, debugMaxTokens, balance,
    welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
    messagesContainer, chatInputRef, userScrolledUp, inputBarFade,
    forceScrollToBottom, smartScrollToBottom, smartScrollAndRefresh, adjustInputHeight, switchSession,
    backgroundTaskList,
    runningTaskCount,
    runningSubagentCount,
    runningBgTaskCount,
    flowState, startCodeWorkflow, stopCodeWorkflow, approvalState, respondApproval,
    todoState, sendSteerMessage,
    questionState, answerQuestion,
    agentStore,
    toggleExpand, toggleChat, updateParams,
    groupedMessages, formatChatTime,
    kbOpen, kbFiles, kbDragOver, kbUploadInputRef, kbLoading,
    toggleKb, triggerKbUpload, onKbUploadSelected, onKbDrop, loadKb,
    addKbFiles, removeKbFile, insertKbRef, fileIcon, formatKbSize
  }
}
