import { reactive } from 'vue'
import { requestPreview } from './previewBus.js'
import { generatePptxFile, generatePptxHtml } from '../../../utils/pptx.js'
import { contextBreakdown, setContextBreakdownFromBackend, setConversationTokens } from './contextBreakdown.js'
import { sessionTokenStats, loadSessionTokenStats, persistSessionTokens } from './sessionTokenStats.js'

// 四态机 Code 工作流的前端传输层。
// 直接用原生 EventSource 连 GET /api/code/workflow，事件契约见后端
// agent_workflow_handler.go 头注释。一次工作流 = messages 里一条
// kind:'agentflow' 的消息，blocks 数组按到达顺序平铺（thinking/intent/tool）。

let msgSeq = 0

export function useAgentWorkflow({ messages, onNewMessage, onStreamUpdate }) {
    const flowState = reactive({ active: false })
    let es = null
    let currentFlow = null

    // 审批状态：Ask 模式下后端推 approval_request 时压入；用户点允许/拒绝后该条消失。
    // 同一次工作流可能连续多个危险工具待批，所以用数组挂多个。
    // UI 是输入框上方的轻量条（不是打断式弹窗），每条带 60s 倒计时，到点自动同意。
    const approvalState = reactive({ pending: [] })
    const APPROVAL_TIMEOUT_SEC = 60
    const approvalTimers = new Map() // id -> intervalId

    // 当前任务 TODO：agent 调 update_todo 时后端推 todo 事件，输入框上方的 todo-bar 据此实时勾选。
    // 全局共享，不隶属某条消息，挂在 ChatWidget 输入框上方（仿 Hermes 勾选清单）。
    const todoState = reactive({ items: [] })

    // ask_user 提问：agent 调 ask_user 工具时后端推 question 事件，这里压入一个
    // 待回答项，ChatWidget 据此弹「提问弹窗」（复选/单选/自由输入 + 取消/确认）。
    // 同一工作流同一时刻只会有一个未决提问（后端循环阻塞着），数组是为了容错。
    const questionState = reactive({ pending: null })

    // 断点续跑：后端每轮把进行中的工作流落盘（workflow_checkpoint.go），
    // 后端重启/SSE 断线后这里查得到，输入框上方出一条「上次任务跑到第 N 轮」。
    // Yolo 全自动跑长任务时最要紧——否则一断就得从头重发，工具全再跑一遍。
    const resumeState = reactive({ pending: null })

    async function refreshResumable() {
        const sid = localStorage.getItem('prism_session_id') || ''
        if (!sid) { resumeState.pending = null; return }
        try {
            const res = await fetch('/api/code/workflow/checkpoints?session_id=' + encodeURIComponent(sid))
            const data = await res.json()
            // 只提示最近的那一个：同一会话堆着多个中断任务时，逐条问反而是噪音
            resumeState.pending = (data.checkpoints || [])[0] || null
        } catch {
            resumeState.pending = null // 查不到就当没有，不打扰用户
        }
        onStreamUpdate?.()
    }

    function dismissResumable() {
        const cp = resumeState.pending
        resumeState.pending = null
        onStreamUpdate?.()
        if (!cp) return
        fetch('/api/code/workflow/checkpoints/' + encodeURIComponent(cp.workflow_id), { method: 'DELETE' })
            .catch(err => console.error('删除检查点失败', err))
    }

    function resumeCodeWorkflow() {
        const cp = resumeState.pending
        if (!cp || flowState.active) return
        resumeState.pending = null
        // task 走检查点里的原文，model/mode/effort 后端也从检查点取，这里不用带
        startCodeWorkflow(cp.task, null, { resumeId: cp.workflow_id, resumedRound: cp.round })
    }

    function clearApprovalTimer(id) {
        const t = approvalTimers.get(id)
        if (t) { clearInterval(t); approvalTimers.delete(id) }
    }

    // auto=true 表示倒计时归零自动同意（不是用户点的），用于区分埋点/文案
    function respondApproval(item, allow, auto = false) {
        clearApprovalTimer(item.id)
        const idx = approvalState.pending.indexOf(item)
        if (idx >= 0) approvalState.pending.splice(idx, 1)
        const sid = localStorage.getItem('prism_session_id') || ''
        // remember: 仅允许时勾选「不再询问」才生效，把工具签名写进会话规则。
        // 自动同意不写 remember —— 用户没表态，不该给它留常设放行规则。
        const body = { id: item.id, allow, remember: allow && !auto && !!item.remember, tool: item.tool }
        fetch('/api/code/workflow/approve?session_id=' + encodeURIComponent(sid), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        }).catch(err => console.error('approve 请求失败', err))
        onStreamUpdate?.()
    }

    // 启动某条审批的 60s 倒计时；归零自动同意（后端也有 65s 兜底，防前端整个挂掉）
    function startApprovalCountdown(item) {
        clearApprovalTimer(item.id)
        const timer = setInterval(() => {
            item.remain -= 1
            if (item.remain <= 0) {
                respondApproval(item, true, true)
                return
            }
            onStreamUpdate?.()
        }, 1000)
        approvalTimers.set(item.id, timer)
    }

    function clearAllApprovals() {
        for (const t of approvalTimers.values()) clearInterval(t)
        approvalTimers.clear()
        approvalState.pending.length = 0
    }

    function closeStream() {
        if (es) { es.close(); es = null }
        flowState.active = false
        clearAllApprovals() // 流结束清掉残留审批条与倒计时
        onStreamUpdate?.()
    }

    // display 可选：{ text, attachments } —— 气泡展示用的"用户实际打的字 + 附件 chip"，
    // 跟真正发给模型的 task（附件内容已经拍平拼接）分开，不然气泡里会把图片解析原文/
    // 文件全文都摊开显示，等于把输入框背后的东西又倒回来给用户看一遍
    // opts.resumeId：从后端检查点续跑（见 resumeCodeWorkflow）。续跑时这条任务的
    // 用户消息上次已经上过屏、也已在后端历史里，不再重复插入用户气泡。
    function startCodeWorkflow(task, display, opts = {}) {
        task = (task || '').trim()
        if (!task || flowState.active) return
        flowState.active = true

        if (!opts.resumeId) {
            messages.value.push({
                id: `afu_${Date.now()}_${msgSeq++}`,
                sender: 'user',
                content: display?.text ?? task,
                attachments: display?.attachments ?? [],
                timestamp: new Date()
            })
        }

        const flow = reactive({
            id: `af_${Date.now()}_${msgSeq++}`,
            kind: 'agentflow',
            sender: 'bot',
            status: 'running', // running | completed | failed | stopped
            task,
            // 后端 workflow_start 事件回填的 workflow_id，中途插话（sendSteerMessage）
            // 靠它把消息投给正确的正在跑的工作流。
            workflowId: null,
            resumedFrom: opts.resumeId ? (opts.resumedRound || 0) : 0, // >0 时卡片头显示「从第 N 轮续跑」
            blocks: [],
            subagents: [], // 雨燕子代理生命周期（后台任务面板的数据源）
            startTime: Date.now(),
            endTime: null,
            inputTokens: 0,
            outputTokens: 0,
            // 实际承接这次请求的 backend 能力元数据（是否识图/上下文窗口/是否支持思考强度），
            // 由后端 model_info 事件回填，工作流开始时是 null
            modelInfo: null,
            timestamp: new Date()
        })
        currentFlow = flow
        messages.value.push(flow)
        onNewMessage?.()

        const sid = localStorage.getItem('prism_session_id') || ''
        // model 直接透传前端选好的模型 ID，命中 freeModelCatalog 就精确路由到那一个
        // （见 model_router.go resolveBackends），不再是"选了也白选"
        const model = localStorage.getItem('selectedModel') || ''
        // effort 只有当前 backend 真支持 reasoning 时后端才会真的采用（否则安静忽略），
        // 前端不需要自己先判断"这个模型支不支持"再决定发不发
        const effort = localStorage.getItem('debugReasoning') || ''
        // mode: yolo(全自动) / ask(危险工具每步问) / plan(执行前必问)，由底部工具条选出
        const mode = localStorage.getItem('agentMode') || 'yolo'
        // 生图提供商：设置面板选的，Go 侧拦截 image_generate 工具调用时自动注入，
        // 不走提示词——跟识图模型路由一个思路，模型不感知、不浪费 token
        const imageProvider = localStorage.getItem('imageProvider') || 'pollinations'
        // 联网搜索模型：「模型」tab 独立配置的搜索模型 ID（'' = 不启用）。
        // 后端收到后主链改用该 backend（Responses 协议 + 服务端 web_search）。
        const searchModel = localStorage.getItem('searchModel') || ''
        // 续跑：只带 resume=<workflow_id>，task/model/mode/effort 后端全从检查点取，
        // 免得前端此刻的模型选择跟中断前不一致（换模型会让已有 tool_calls 历史串味）
        const url = opts.resumeId
            ? `/api/code/workflow?resume=${encodeURIComponent(opts.resumeId)}`
            : `/api/code/workflow?task=${encodeURIComponent(task)}&session_id=${encodeURIComponent(sid)}&model=${encodeURIComponent(model)}&effort=${encodeURIComponent(effort)}&mode=${encodeURIComponent(mode)}&image_provider=${encodeURIComponent(imageProvider)}&search_model=${encodeURIComponent(searchModel)}`
        es = new EventSource(url)

        // thinking / intent 是文本增量：追加到同类型的最后一个块，类型切换时开新块
        const appendText = (type, text) => {
            if (!text) return
            const last = flow.blocks[flow.blocks.length - 1]
            if (last && last.type === type) last.text += text
            else flow.blocks.push({ type, text })
            onStreamUpdate?.()
        }

        // 大多数上游会在流式 tool call 一开始就给 id；少数兼容服务会在最终 action
        // 才补 id，导致 action_delta 创建的“生成预览”卡片无法按 id 找回，永久卡住。
        // id 精确匹配优先；只有找不到时才按同名、未完成的最近卡片兜底。
        const findPendingTool = (id, name) => {
            const blocks = [...flow.blocks].reverse()
            const exact = blocks.find(b => b.type === 'tool' && b.id === id)
            if (exact) return exact
            return blocks.find(b => b.type === 'tool' && b.name === name &&
                (b.status === 'generating' || b.status === 'running'))
        }

        const settlePendingTools = (status = 'error', output = '') => {
            for (const block of flow.blocks) {
                if (block.type !== 'tool' || (block.status !== 'generating' && block.status !== 'running')) continue
                block.status = status
                block.elapsedMs = block.startTime ? (Date.now() - block.startTime) : 0
                if (output && !block.output) block.output = output
            }
            onStreamUpdate?.()
        }

        // 之前这个事件完全没人听——workflow_id 从没进过前端状态，sendSteerMessage
        // 也就无从知道该往哪个工作流投消息。
        es.addEventListener('workflow_start', e => {
            flow.workflowId = JSON.parse(e.data).workflow_id
        })

        es.addEventListener('model_info', e => {
            flow.modelInfo = JSON.parse(e.data)
            // 后端回传的分类上下文占用（system/subagent/skill/memory/tools），落盘持久化
            setContextBreakdownFromBackend(flow.modelInfo.context_breakdown, flow.modelInfo.context_window)
        })
        es.addEventListener('thinking', e => appendText('thinking', JSON.parse(e.data).content))
        es.addEventListener('intent', e => appendText('intent', JSON.parse(e.data).content))

        // 工具参数同样是流式生成的，但绝不能把不断增长的完整 JSON 每个 token 都塞进
        // Vue 响应式状态：长 HTML 会导致整棵消息树反复渲染，DiffViewer 还会反复做
        // O(n) diff，累计成 O(n²)，最终把浏览器主线程拖死。
        //
        // 像主聊天正文一样做单向 SSE 瀑布：每个字符只解码一次，完成一行就推入
        // 环形窗口。绝不把累计全文重新塞回 Vue，也不重新 JSON.parse / diff / highlight。
        const LIVE_LINE_WINDOW = 36
        function feedLiveContent(progress, delta) {
            let source = delta
            if (!progress.inString) {
                progress.seek = (progress.seek + source).slice(-160)
                const match = /"(?:content|newText|new_string)"\s*:\s*"/.exec(progress.seek)
                if (!match) return
                const consumed = match.index + match[0].length
                source = progress.seek.slice(consumed)
                progress.seek = ''
                progress.inString = true
            }
            for (let i = 0; i < source.length && progress.inString; i++) {
                const ch = source[i]
                if (progress.unicodeLeft > 0) {
                    progress.unicode += ch
                    progress.unicodeLeft--
                    if (progress.unicodeLeft === 0) {
                        const code = Number.parseInt(progress.unicode, 16)
                        if (Number.isFinite(code)) progress.line += String.fromCharCode(code)
                        progress.unicode = ''
                    }
                    continue
                }
                if (progress.escaped) {
                    progress.escaped = false
                    if (ch === 'n') {
                        progress.lines.push({ no: ++progress.totalLines, text: progress.line })
                        progress.line = ''
                        if (progress.lines.length > LIVE_LINE_WINDOW) progress.lines.shift()
                    } else if (ch === 'r') {
                        // CRLF 的 CR 不单独生成一行。
                    } else if (ch === 't') progress.line += '\t'
                    else if (ch === 'u') {
                        progress.unicodeLeft = 4
                        progress.unicode = ''
                    } else {
                        progress.line += ({ b: '\b', f: '\f' })[ch] ?? ch
                    }
                    continue
                }
                if (ch === '\\') {
                    progress.escaped = true
                } else if (ch === '"') {
                    progress.inString = false
                } else {
                    progress.line += ch
                }
            }
        }
        const actionDeltaProgress = new Map()
        es.addEventListener('action_delta', e => {
            const d = JSON.parse(e.data)
            if (!d.id || !d.name) return
            let t = [...flow.blocks].reverse().find(b => b.type === 'tool' && b.id === d.id)
            if (!t) {
                // 生成参数时默认展开，让用户直接看到有界的红绿实时 Diff；
                // action 闭合、真正开始执行工具后会在下方自动收起。
                t = { type: 'tool', id: d.id, name: d.name, args: {}, _rawArgs: '', generatedChars: 0, status: 'generating', output: '', expanded: true, startTime: Date.now(), elapsedMs: 0 }
                flow.blocks.push(t)
            }
            t.name = d.name
            const now = Date.now()
            const delta = d.args_delta || ''
            const progress = actionDeltaProgress.get(d.id) || {
                chars: 0, lastUiAt: 0, seek: '', inString: false, escaped: false,
                unicodeLeft: 0, unicode: '', line: '', lines: [], totalLines: 0
            }
            progress.chars += delta.length
            feedLiveContent(progress, delta)
            if (now - progress.lastUiAt >= 120) {
                t.generatedChars = progress.chars
                const visible = progress.line
                    ? [...progress.lines, { no: progress.totalLines + 1, text: progress.line }]
                    : progress.lines
                t.liveLines = visible.slice(-LIVE_LINE_WINDOW)
                t.totalLiveLines = progress.totalLines + (progress.line ? 1 : 0)
                progress.lastUiAt = now
                onStreamUpdate?.()
            }
            actionDeltaProgress.set(d.id, progress)
        })

        // 上下文压缩：后端在上下文超窗口 80% 时把早期轮次折叠成摘要，插一个轻量块
        // 让用户知道"这里发生了压缩、省了多少"，而不是默默改写历史。
        es.addEventListener('context_compressed', e => {
            const d = JSON.parse(e.data)
            flow.blocks.push({
                type: 'compressed',
                foldedMessages: d.folded_messages || 0,
                beforeChars: d.before_chars || 0,
                afterChars: d.after_chars || 0
            })
            onStreamUpdate?.()
        })

        // 任务 TODO 更新:整份覆盖(后端全量下发)
        es.addEventListener('todo', e => {
            try { todoState.items = JSON.parse(e.data).items || [] } catch { /* 忽略坏包 */ }
            onStreamUpdate?.()
        })

        es.addEventListener('action', e => {
            const d = JSON.parse(e.data)
            actionDeltaProgress.delete(d.id)
            let args = {}
            try { args = JSON.parse(d.args || '{}') } catch { /* 参数留空对象，卡片仍可显示工具名 */ }
            // 【诊断】联网搜索：打印后端实际下发的 action args，确认 urls 有没有到
            if (d.name === 'web_search') {
                console.log('[refs] SSE action →', d.args, '| urls =', args.urls ? args.urls.length : 0)
                window.__refsDebug = window.__refsDebug || { log: [] }
                window.__refsDebug.log.push({ t: Date.now(), ev: 'action', name: d.name, args })
            }
            // startTime：记下发起时刻，result 到达时算耗时（图1 那种「完成 41ms」徽章）
            const t = findPendingTool(d.id, d.name)
            if (t) {
                t.id = d.id
                t.name = d.name
                t.args = args
                t._rawArgs = d.args || ''
                t.status = 'running'
                // web_search（服务端搜索）默认展开——引用来源就是它的价值，
                // 折叠起来用户看不到搜到了什么；其它工具执行时收起。
                t.expanded = d.name === 'web_search'
            } else {
                flow.blocks.push({ type: 'tool', id: d.id, name: d.name, args, _rawArgs: d.args || '', status: 'running', output: '', expanded: d.name === 'web_search', startTime: Date.now(), elapsedMs: 0 })
            }
            onStreamUpdate?.()
        })

        es.addEventListener('result', e => {
            const d = JSON.parse(e.data)
            // 从后往前找，同一 id 只可能对应最近一条 running 的动作
            const t = findPendingTool(d.id, d.name)
            if (t) {
                t.id = d.id
                // web_search 的 result 事件后端会带 status（searching/completed）：
                // 搜索中必须保持 running，前端才有扫描线（否则被 ok 映射成 'ok'，
                // 搜索进行中会错误显示「无输出」）。
                t.status = (t.name === 'web_search' && d.status === 'searching')
                    ? 'running'
                    : (d.ok ? 'ok' : 'error')
                t.output = d.output || ''
                t.elapsedMs = t.startTime ? (Date.now() - t.startTime) : 0
                if (d.ok && /^(write_file|edit_file|apply_patch|mcp__fs__(write_file|edit_file|create_file))$/.test(d.name || t.name)) {
                    window.dispatchEvent(new CustomEvent('agent-working-diff-changed'))
                }
            }
            // Agent 调用了 generate_pptx → 自动渲染 .pptx + 预览窗口
            maybeAutoPptx(d)
            onStreamUpdate?.()
        })

        // 截图紧跟触发它的工具步骤插进同一条 Agent 工作流；不能另起消息追加到
        // 整段工作流之后，否则视觉上会“永远卡在聊天底部”，打乱后续回复顺序。
        es.addEventListener('artifact', e => {
            const d = JSON.parse(e.data)
            if (d.kind !== 'image' || !d.image) return
            flow.blocks.push({
                type: 'image',
                id: d.id || `artifact_${Date.now()}_${msgSeq++}`,
                image: d.image,
                sourceUrl: d.source_url || '',
                content: d.caption || 'Agent 已发布截图交付。',
                expanded: false
            })
            onStreamUpdate?.()
        })

        // 工具审批请求（Ask/Plan 模式）：后端在执行危险工具前推来，前端弹批准条等人点。
        // 把整条请求（含 id/tool/args）压入 approvalState.pending，弹窗据此渲染。
        es.addEventListener('approval_request', e => {
            const d = JSON.parse(e.data)
            const item = reactive({
                id: d.id,
                tool: d.tool,
                args: d.args || '',
                mode: d.mode || 'ask',
                // reason='path_outside_workdir' 表示这次拦截是因为路径在工作目录之外
                // （不是危险工具本身），批准条据此换文案；path/workdir 用于提示细节
                reason: d.reason || '',
                path: d.path || '',
                workdir: d.workdir || '',
                remember: false,              // 默认不勾选「不再询问」
                remain: APPROVAL_TIMEOUT_SEC, // 60s 倒计时，归零自动同意
                total: APPROVAL_TIMEOUT_SEC
            })
            approvalState.pending.push(item)
            startApprovalCountdown(item)
            onStreamUpdate?.()
        })

        // 雨燕子代理生命周期：start → progress(每次工具调用) → done
        es.addEventListener('subagent_start', e => {
            const d = JSON.parse(e.data)
            flow.subagents.push({
                id: d.id, task: d.task, status: 'running',
                rounds: 0, tools: [], output: '',
                startTime: Date.now(), endTime: null
            })
            onStreamUpdate?.()
        })
        es.addEventListener('subagent_progress', e => {
            const d = JSON.parse(e.data)
            const sa = flow.subagents.find(s => s.id === d.id)
            if (sa) {
                sa.rounds = Math.max(sa.rounds, (d.round || 0) + 1)
                sa.tools.push({ tool: d.tool, preview: d.args_preview || '' })
            }
            onStreamUpdate?.()
        })
        es.addEventListener('subagent_done', e => {
            const d = JSON.parse(e.data)
            const sa = flow.subagents.find(s => s.id === d.id)
            if (sa) {
                sa.status = d.ok ? 'completed' : 'failed'
                sa.rounds = d.rounds ?? sa.rounds
                sa.output = d.output || ''
                sa.endTime = Date.now()
            }
            onStreamUpdate?.()
        })

        es.addEventListener('flow_error', e => {
            const d = JSON.parse(e.data)
            appendText('intent', `\n\n⚠️ ${d.message}`)
            // 后端通常紧随其后发送 workflow_done；这里先收尾，避免网络在两事件之间
            // 断开时把“生成预览”永远留在界面上。
            settlePendingTools('error', d.message || '工作流已中断')
        })

        // 中途插话已被下一轮采纳：插一个轻量块，让用户看到"我刚才那句话生效了"，
        // 而不是发出去之后什么反馈都没有。
        es.addEventListener('steering_injected', e => {
            const d = JSON.parse(e.data)
            flow.blocks.push({ type: 'steer', text: d.message || '' })
            onStreamUpdate?.()
        })

        // agent 改了前端文件：自动弹预览面板并导航过去。真正的开面板/导航动作
        // 分别在 ChatWidget 和 PreviewBrowser 里做，这里只负责把地址广播出去。
        es.addEventListener('preview_open', e => {
            const d = JSON.parse(e.data)
            if (!d.url) return
            // 把后端给的 cdp_ws 一并传下去——open_browser_preview 会带真实
            // Chromium target 的 ws，PreviewBrowser 据此走 CDP screencast 渲染。
            requestPreview(d.url, d.cdp_ws, d.cdp_error)
            flow.blocks.push({ type: 'preview', url: d.url })
            onStreamUpdate?.()
        })

        // ask_user 提问：后端推来一个待用户回答的问题，弹窗据此渲染。
        // 把整条（含 id/question/options/multi/allow_other）压入 questionState.pending。
        es.addEventListener('question', e => {
            const d = JSON.parse(e.data)
            const options = Array.isArray(d.options) ? d.options : []
            const q = reactive({
                id: d.id,
                workflowId: d.workflow_id || flow.workflowId,
                question: d.question || '',
                options,
                multi: !!d.multi,
                allowOther: !!d.allow_other || options.some(option => /其他|自由输入/.test(option.label)),
                answer: '',
                answered: false,
                submitting: false,
                error: '',
            })
            questionState.pending = q
            // 同步压入当前工作流轨迹：和 tool/think 同一机制，刷新前即可见（刷新后由后端 FlowBlock 兜底）。
            flow.blocks.push({
                type: 'question', id: q.id, workflowId: q.workflowId,
                question: q.question, options, multi: q.multi, allowOther: q.allowOther,
                answer: '', answered: false
            })
            onStreamUpdate?.()
        })
        es.addEventListener('question_answered', e => {
            const d = JSON.parse(e.data)
            const pending = questionState.pending
            if (pending?.id === d.id) questionState.pending = null
            for (const b of flow.blocks) {
                if (b.type === 'question' && b.id === d.id) {
                    b.answer = d.answer || b.answer || ''
                    b.answered = true
                    break
                }
            }
            onStreamUpdate?.()
        })

        es.addEventListener('workflow_done', e => {
            const d = JSON.parse(e.data)
            flow.status = d.status || 'completed'
            flow.endTime = Date.now()
            settlePendingTools('error', d.final_output || '工具调用未完成')
            flow.inputTokens = d.input_tokens || 0
            flow.outputTokens = d.output_tokens || 0
            // 对话类 token：用后端已扣除静态部分的 conversation_tokens。
            // 绝不能再用 input_tokens——它是上游真实 prompt_tokens，本身已包含
            // system/tools/skill/subagent/memory，面板再加一遍静态分类就是双重计算
            // （这正是"外显和点开总量对不上"的根因）。老后端没这个字段时前端自己减。
            const cb = contextBreakdown.value
            const statics = (cb.system || 0) + (cb.subagent || 0) + (cb.skill || 0) + (cb.memory || 0) + (cb.tools || 0)
            const conv = d.conversation_tokens != null
                ? d.conversation_tokens
                : Math.max(0, (d.input_tokens || 0) - statics)
            setConversationTokens(conv, localStorage.getItem('prism_session_id') || '')
            // 把本轮 agentflow 的真实 input/output token 按 sessionId 持久化，
            // 这样刷新后底部 context 横条（liveContextStats）仍能显示实际值，不归零。
            persistSessionTokens({
                inputTokens: d.input_tokens || 0,
                outputTokens: d.output_tokens || 0,
                contextWindow: flow.modelInfo?.context_window || 0,
                contextPct: flow.modelInfo?.context_window ? Math.min(((d.input_tokens + d.output_tokens) / flow.modelInfo.context_window) * 100, 100) : 0,
                latencyMs: 0
            }, localStorage.getItem('prism_session_id') || '')
            currentFlow = null
            closeStream()
            // 上游报错时后端留了检查点（workflow_done.resumable），拉出来给续跑条用；
            // 正常完成则检查点已被后端删掉，这次查询自然返回空。
            if (d.resumable) refreshResumable()
        })

        // Hermes 式后台任务：agent 回答已送达但后台任务还在跑——不关流，
        // 卡片进入「等待后台任务」状态，任务完成时后端发 bg_task_done 唤醒同一工作流。
        es.addEventListener('workflow_paused', e => {
            const d = JSON.parse(e.data)
            flow.status = 'waiting'
            flow.endTime = null
            flow.blocks.push({
                type: 'steer',
                text: `⏳ ${d.pending_tasks || 1} 个后台任务运行中，完成时自动继续…`
            })
            onStreamUpdate?.()
        })

        // 后台任务完成：恢复 running 状态 + 在后台任务面板登记一条完成记录。
        es.addEventListener('bg_task_done', e => {
            const d = JSON.parse(e.data)
            flow.status = 'running'
            flow.endTime = null
            flow.subagents.push({
                id: d.task_id,
                task: `[后台任务] ${d.command || ''}`,
                status: d.exit_code === 0 ? 'completed' : 'failed',
                rounds: 0,
                tools: [],
                output: d.output || '',
                startTime: Date.now(),
                endTime: Date.now()
            })
            onStreamUpdate?.()
        })

        // 服务端正常结束响应也会触发 onerror（EventSource 会尝试重连），
        // workflow_done 已把 es 置 null，这里只兜底异常断开
        es.onerror = () => {
            if (currentFlow && (currentFlow.status === 'running' || currentFlow.status === 'waiting')) {
                currentFlow.status = 'failed'
                currentFlow.endTime = Date.now()
                settlePendingTools('error', '连接已断开，工具调用未完成')
                settleSubagents(currentFlow, 'failed')
                currentFlow = null
                // 断在半路（后端被重启 / 网络断）—— 这正是检查点存在的意义，
                // 查一下断点，输入框上方出续跑条。
                refreshResumable()
            }
            closeStream()
        }
    }

    // 流异常/手动停止时，把还挂着 running 的子代理一并收尾
    function settleSubagents(flow, status) {
        for (const sa of flow.subagents || []) {
            if (sa.status === 'running') {
                sa.status = status
                sa.endTime = Date.now()
            }
        }
    }

    async function stopCodeWorkflow() {
        const flow = currentFlow
        if (!flow || (flow.status !== 'running' && flow.status !== 'waiting')) {
            closeStream()
            return
        }
        const workflowId = flow.workflowId
        flow.status = 'stopped'
        flow.endTime = Date.now()
        settleSubagents(flow, 'stopped')
        for (const block of flow.blocks || []) {
            if (block.type === 'tool' && (block.status === 'generating' || block.status === 'running')) {
                block.status = 'error'
                block.output ||= '用户停止了工作流'
                block.elapsedMs = block.startTime ? (Date.now() - block.startTime) : 0
            }
        }
        questionState.pending = null
        currentFlow = null
        onStreamUpdate?.()

        // 必须先通知后端收尾再关 EventSource。只关流时后端只能看到“网络断线”，
        // 无法区分用户停止，也可能来不及把失败状态和部分工具轨迹写进会话上下文。
        if (workflowId) {
            try {
                await fetch('/api/code/workflow/stop', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ workflow_id: workflowId })
                })
            } catch (err) {
                console.error('停止工作流请求失败', err)
            }
        }
        closeStream()
        refreshResumable()
    }

    // 中途插话：工作流跑着的时候塞一条消息，不用等它完全停下。
    // 依赖 currentFlow.workflowId（由 workflow_start 事件回填）定位正在跑的那个工作流；
    // 还没拿到 workflow_id（第一轮模型响应之前的极短窗口）就直接失败，调用方据此提示重试。
    async function sendSteerMessage(message) {
        message = (message || '').trim()
        const wfId = currentFlow?.workflowId
        if (!message || !wfId) return false
        try {
            const res = await fetch('/api/code/workflow/steer', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ workflow_id: wfId, message })
            })
            return res.ok
        } catch {
            return false
        }
    }

    // 回答 ask_user 提问：把用户选中的选项/自由输入 POST 回后端，后端唤醒阻塞的循环。
    // selected 为空且 answer 为空表示「取消」——仍发请求让后端用 fallback/空答案继续，
    // 不卡死工作流（与审批超时自动放行同一思路：宁可继续，不留半吊子）。
    async function answerQuestion({ id, answer = '', selected = [] }) {
        const item = questionState.pending
        if (!item || item.id !== id) return
        if (item.submitting) return
        item.submitting = true
        item.error = ''
        const wfId = item.workflowId
        try {
            const res = await fetch('/api/code/workflow/answer', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id, workflow_id: wfId, answer: answer || '', selected })
            })
            const data = await res.json().catch(() => ({}))
            if (!res.ok) throw new Error(data.error || `提交失败 (${res.status})`)
            for (const b of flow.blocks) {
                if (b.type === 'question' && b.id === item.id) {
                    b.answer = answer || selected.join('、')
                    b.answered = true
                    break
                }
            }
            questionState.pending = null
            onStreamUpdate?.()
        } catch (err) {
            console.error('answer 请求失败', err)
            item.error = err.message || '回答提交失败，请重试'
        } finally {
            item.submitting = false
        }
    }

    return {
        flowState, approvalState, respondApproval, startCodeWorkflow, stopCodeWorkflow,
        resumeState, refreshResumable, resumeCodeWorkflow, dismissResumable,
        todoState, sendSteerMessage, questionState, answerQuestion
    }
}

// Agent 调用 generate_pptx 工具后，后端在 result 事件里回传结构化 JSON；
// 这里检测到该工具后自动渲染 .pptx（下载）并打开预览窗口，不需要用户点按钮。
async function maybeAutoPptx(d) {
    const name = d.name || ''
    if (name !== 'generate_pptx') return
    try {
        let slideData = {}
        try { slideData = JSON.parse(d.output || '{}') } catch { slideData = { title: 'PPT', slides: [] } }
        const html = generatePptxHtml(slideData)
        const blob = new Blob([html], { type: 'text/html;charset=utf-8' })
        const url = URL.createObjectURL(blob)
        requestPreview(url)
        const filename = `rescene-${Date.now()}.pptx`
        await generatePptxFile(slideData, filename)
    } catch (e) {
        console.error('auto pptx failed', e)
    }
}
