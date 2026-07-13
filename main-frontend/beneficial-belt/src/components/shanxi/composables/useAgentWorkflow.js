import { reactive } from 'vue'

// 四态机 Code 工作流的前端传输层。
// 直接用原生 EventSource 连 GET /api/code/workflow，事件契约见后端
// agent_workflow_handler.go 头注释。一次工作流 = messages 里一条
// kind:'agentflow' 的消息，blocks 数组按到达顺序平铺（thinking/intent/tool）。

let msgSeq = 0

export function useAgentWorkflow({ messages, onNewMessage, onStreamUpdate }) {
    const flowState = reactive({ active: false })
    let es = null
    let currentFlow = null

    function closeStream() {
        if (es) { es.close(); es = null }
        flowState.active = false
        onStreamUpdate?.()
    }

    function startCodeWorkflow(task) {
        task = (task || '').trim()
        if (!task || flowState.active) return
        flowState.active = true

        messages.value.push({
            id: `afu_${Date.now()}_${msgSeq++}`,
            sender: 'user',
            content: `[工作流: code] ${task}`,
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
            timestamp: new Date()
        })
        currentFlow = flow
        messages.value.push(flow)
        onNewMessage?.()

        const sid = localStorage.getItem('prism_session_id') || ''
        es = new EventSource(`/api/code/workflow?task=${encodeURIComponent(task)}&session_id=${encodeURIComponent(sid)}`)

        // thinking / intent 是文本增量：追加到同类型的最后一个块，类型切换时开新块
        const appendText = (type, text) => {
            if (!text) return
            const last = flow.blocks[flow.blocks.length - 1]
            if (last && last.type === type) last.text += text
            else flow.blocks.push({ type, text })
            onStreamUpdate?.()
        }

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

    return { flowState, startCodeWorkflow, stopCodeWorkflow }
}
