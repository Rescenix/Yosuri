import { reactive } from 'vue'
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

    // 审批弹窗状态：Ask/Plan 模式下后端推 approval_request 时压入；用户点允许/拒绝后弹窗消失。
    // 同一次工作流可能连续多个危险工具待批，所以用数组挂多个。
    const approvalState = reactive({ pending: [] })

    function respondApproval(item, allow) {
        const idx = approvalState.pending.indexOf(item)
        if (idx >= 0) approvalState.pending.splice(idx, 1)
        const sid = localStorage.getItem('prism_session_id') || ''
        // remember: 仅允许时勾选「不再询问」才生效，把工具签名写进会话规则
        const body = { id: item.id, allow, remember: allow && !!item.remember, tool: item.tool }
        fetch('/api/code/workflow/approve?session_id=' + encodeURIComponent(sid), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        }).catch(err => console.error('approve 请求失败', err))
    }

    function closeStream() {
        if (es) { es.close(); es = null }
        flowState.active = false
        approvalState.pending.length = 0 // 流结束清掉残留审批弹窗
        onStreamUpdate?.()
    }

    // display 可选：{ text, attachments } —— 气泡展示用的"用户实际打的字 + 附件 chip"，
    // 跟真正发给模型的 task（附件内容已经拍平拼接）分开，不然气泡里会把图片解析原文/
    // 文件全文都摊开显示，等于把输入框背后的东西又倒回来给用户看一遍
    function startCodeWorkflow(task, display) {
        task = (task || '').trim()
        if (!task || flowState.active) return
        flowState.active = true

        messages.value.push({
            id: `afu_${Date.now()}_${msgSeq++}`,
            sender: 'user',
            content: display?.text ?? task,
            attachments: display?.attachments ?? [],
            timestamp: new Date()
        })

        const flow = reactive({
            id: `af_${Date.now()}_${msgSeq++}`,
            kind: 'agentflow',
            sender: 'bot',
            status: 'running', // running | completed | failed | stopped
            task,
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
        es = new EventSource(`/api/code/workflow?task=${encodeURIComponent(task)}&session_id=${encodeURIComponent(sid)}&model=${encodeURIComponent(model)}&effort=${encodeURIComponent(effort)}&mode=${encodeURIComponent(mode)}`)

        // thinking / intent 是文本增量：追加到同类型的最后一个块，类型切换时开新块
        const appendText = (type, text) => {
            if (!text) return
            const last = flow.blocks[flow.blocks.length - 1]
            if (last && last.type === type) last.text += text
            else flow.blocks.push({ type, text })
            onStreamUpdate?.()
        }

        es.addEventListener('model_info', e => {
            flow.modelInfo = JSON.parse(e.data)
            // 后端回传的分类上下文占用（system/subagent/skill/memory/tools），落盘持久化
            setContextBreakdownFromBackend(flow.modelInfo.context_breakdown, flow.modelInfo.context_window)
        })
        es.addEventListener('thinking', e => appendText('thinking', JSON.parse(e.data).content))
        es.addEventListener('intent', e => appendText('intent', JSON.parse(e.data).content))

        es.addEventListener('action', e => {
            const d = JSON.parse(e.data)
            let args = {}
            try { args = JSON.parse(d.args || '{}') } catch { /* 参数留空对象，卡片仍可显示工具名 */ }
            flow.blocks.push({ type: 'tool', id: d.id, name: d.name, args, status: 'running', output: '', expanded: false })
            onStreamUpdate?.()
        })

        es.addEventListener('result', e => {
            const d = JSON.parse(e.data)
            // 从后往前找，同一 id 只可能对应最近一条 running 的动作
            const t = [...flow.blocks].reverse().find(b => b.type === 'tool' && b.id === d.id)
            if (t) {
                t.status = d.ok ? 'ok' : 'error'
                t.output = d.output || ''
            }
            onStreamUpdate?.()
        })

        // 工具审批请求（Ask/Plan 模式）：后端在执行危险工具前推来，前端弹批准条等人点。
        // 把整条请求（含 id/tool/args）压入 approvalState.pending，弹窗据此渲染。
        es.addEventListener('approval_request', e => {
            const d = JSON.parse(e.data)
            approvalState.pending.push({
                id: d.id,
                tool: d.tool,
                args: d.args || '',
                mode: d.mode || 'ask',
                remember: false // 默认不勾选「不再询问」
            })
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
        })

        es.addEventListener('workflow_done', e => {
            const d = JSON.parse(e.data)
            flow.status = d.status || 'completed'
            flow.endTime = Date.now()
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
        })

        // 服务端正常结束响应也会触发 onerror（EventSource 会尝试重连），
        // workflow_done 已把 es 置 null，这里只兜底异常断开
        es.onerror = () => {
            if (currentFlow && currentFlow.status === 'running') {
                currentFlow.status = 'failed'
                currentFlow.endTime = Date.now()
                settleSubagents(currentFlow, 'failed')
                currentFlow = null
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

    function stopCodeWorkflow() {
        if (currentFlow && currentFlow.status === 'running') {
            currentFlow.status = 'stopped'
            currentFlow.endTime = Date.now()
            settleSubagents(currentFlow, 'stopped')
            currentFlow = null
        }
        closeStream()
    }

    return { flowState, approvalState, respondApproval, startCodeWorkflow, stopCodeWorkflow }
}
