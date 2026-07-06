import { nextTick, reactive } from 'vue'

const TOOL_NAME_MAP = {
    web_search: '联网搜索',
    write_blog: '撰写博客',
    clean_memories: '清理记忆',
    search_codebase: '搜索代码库',
    read_file: '读取文件',
    write_file: '写入文件',
    execute_command: '执行命令',
    codebase_query: '代码知识图谱',
    codegraph_query: '调用链分析'
}

export function useChatLogic({
    messages, userInput, sessionId,
    updateEmotion, saveMemory, lastTokenUsage, lastLatency,
    onNewMessage,
    onStreamUpdate
}) {
    let msgId = 0
    let lastToolStartTime = 0
    let pollingTimer = null

    const executeTool = async (toolName, argsStr) => {
        const args = {}
        const argRegex = /(\w+)="(.*?)"/g
        let match
        while ((match = argRegex.exec(argsStr)) !== null) {
            args[match[1]] = match[2]
        }
        try {
            const res = await fetch('/api/execute-marker', {
                method: 'POST',
                headers: { 'Content-Type': 'text/plain' },
                body: `[TOOL:${toolName} ${argsStr}]`
            })
            return await res.text()
        } catch (e) {
            return `工具调用失败: ${e.message}`
        }
    }

    // ★ 独立的工具调用处理函数（流式结束后调用）
    const processToolsInFinalText = async (text, botMsg) => {
        // 预处理：还原 DS 返回的转义字符
        text = text
            .replace(/\\\[TOOL:/g, '[TOOL:')
            .replace(/\]\\/g, ']')
            .replace(/\\_/g, '_')
            .replace(/\\"/g, '"')
            .replace(/\\'/g, "'")
            .replace(/\\\\/g, '\\')

        console.log('[TOOL] 转义还原后文本：', text.substring(0, 500))

        if (!text || !text.includes('[TOOL:')) {
            botMsg.isStreaming = false
            return
        }

        const toolRegex = /\[TOOL:(\w+)\s+(.*?)\]\n?/g
        let toolMatch
        let finalText = text
        let hasTool = false

        while ((toolMatch = toolRegex.exec(text)) !== null) {
            hasTool = true
            const marker = toolMatch[0].trim()
            const toolName = toolMatch[1]
            let argsStr = toolMatch[2]

            // execute_command 特殊处理：安全提取命令参数
            if (toolName === 'execute_command') {
                const cmdStart = marker.indexOf('command="')
                if (cmdStart !== -1) {
                    const valueStart = cmdStart + 'command="'.length
                    const lastQuote = marker.lastIndexOf('"')
                    if (lastQuote > valueStart) {
                        let command = marker.substring(valueStart, lastQuote)
                        command = command.replace(/\\"/g, '"')
                        argsStr = `command="${command}"`
                        console.log('[TOOL] 提取到的完整命令：', command)
                    }
                }
            }

            botMsg.toolCallName = TOOL_NAME_MAP[toolName] || toolName
            botMsg.toolCallDetail = argsStr
            if (onStreamUpdate) onStreamUpdate()

            try {
                const toolRes = await fetch('/api/execute-marker', {
                    method: 'POST',
                    headers: { 'Content-Type': 'text/plain' },
                    body: `[TOOL:${toolName} ${argsStr}]`
                })
                const resultText = await toolRes.text()

                // 创建新 bot 消息用于 DS 的自然语言回复
                const newBotMsg = reactive({
                    id: msgId++,
                    content: '',
                    sender: 'bot',
                    isStreaming: true,
                    recalling: false,
                    timestamp: new Date()
                })
                messages.value.push(newBotMsg)
                nextTick(() => { if (onNewMessage) onNewMessage() })

                // 用 /stream 发送隐式消息，流式获取 DS 回复，不再解析工具调用
                fetch('http://localhost:3000/stream', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ message: `[工具结果]\n${resultText}\n\n请用自然语言描述这个结果。` })
                }).then(async (res) => {
                    const reader = res.body.getReader()
                    const decoder = new TextDecoder()
                    let buffer = ''
                    while (true) {
                        const { done, value } = await reader.read()
                        if (done) break
                        buffer += decoder.decode(value, { stream: true })
                        const lines = buffer.split('\n')
                        buffer = lines.pop() || ''
                        for (const line of lines) {
                            if (line.startsWith('data: ')) {
                                const payload = line.slice(6)
                                if (payload === '[DONE]') {
                                    newBotMsg.isStreaming = false
                                    if (onStreamUpdate) onStreamUpdate()
                                    return
                                }
                                newBotMsg.content += payload
                                if (onStreamUpdate) onStreamUpdate()
                            }
                        }
                    }
                }).catch(() => {
                    newBotMsg.content = '杉汐没有回应，请稍后再试'
                    newBotMsg.isStreaming = false
                })

                finalText = finalText.replace(marker, `[工具调用: ${toolName}]\n${resultText}\n`)
            } catch (e) {
                finalText = finalText.replace(marker, `[工具调用: ${toolName}]\n执行失败: ${e.message}\n`)
            }

            botMsg.toolCallName = null
            botMsg.toolCallDetail = ''
        }

        if (hasTool) {
            botMsg.content = finalText
            if (onStreamUpdate) onStreamUpdate()
        }
        botMsg.isStreaming = false
        chatState.active = false
    }

    const sendStandardMessage = async () => {
        const question = userInput.value.trim()
        if (!question) return

        messages.value.push({
            id: msgId++,
            content: question,
            sender: 'user',
            timestamp: new Date()
        })
        userInput.value = ''

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

        nextTick(() => { if (onNewMessage) onNewMessage() })

        chatState.active = true

        const requestBody = {
            message: question,
            sessionId: sessionId.value,
            model: localStorage.getItem('selectedModel') || 'local',
            temperature: parseFloat(localStorage.getItem('debugTemp') || 0.7),
            top_p: parseFloat(localStorage.getItem('debugTopP') || 0.9),
            max_tokens: parseInt(localStorage.getItem('debugMaxTokens') || 2000),
            reasoning_effort: localStorage.getItem('debugReasoning') || undefined
        }

        const token = localStorage.getItem('token')
        const headers = { 'Content-Type': 'application/json' }
        if (token) headers['Authorization'] = `Bearer ${token}`

        let charQueue = []
        let typingTimer = null

        const processCharQueue = () => {
            if (charQueue.length === 0) { typingTimer = null; return }
            const { type, char } = charQueue.shift()
            if (type === 'reasoning') botMsg.reasoning += char
            else if (type === 'content') botMsg.content += char
            if (onStreamUpdate) onStreamUpdate()
            typingTimer = setTimeout(processCharQueue, 20)
        }

        const enqueueChars = (type, text) => {
            if (!text) return
            const chars = [...text]
            chars.forEach(c => charQueue.push({ type, char: c }))
            if (!typingTimer) processCharQueue()
        }

        try {
            const response = await fetch('/api/chat/stream', {
                method: 'POST',
                headers,
                body: JSON.stringify(requestBody)
            })
            if (!response.ok) throw new Error('Network error')

            const reader = response.body.getReader()
            const decoder = new TextDecoder()
            let partialLine = ''
            while (true) {
                const { done, value } = await reader.read()
                if (done) break
                const chunk = decoder.decode(value, { stream: true })
                const lines = (partialLine + chunk).split('\n')
                partialLine = lines.pop() || ''
                for (const line of lines) {
                    if (!line || line.startsWith('event: ')) continue
                    if (line.startsWith('data: ')) {
                        const data = line.slice(6)
                        if (!data) continue
                        try {
                            const payload = JSON.parse(data)
                            if (payload.type === 'reasoning') {
                                botMsg.recalling = false
                                enqueueChars('reasoning', payload.content)
                            } else if (payload.type === 'content') {
                                botMsg.recalling = false
                                enqueueChars('content', payload.content)
                            } else if (payload.type === 'done') {
                                const waitForQueue = () => {
                                    if (charQueue.length > 0 || typingTimer) {
                                        setTimeout(waitForQueue, 100)
                                    } else {
                                        botMsg.content = payload.content || botMsg.content
                                        botMsg.isStreaming = false
                                        botMsg.recalling = false
                                        if (payload.token_usage !== undefined) {
                                            const total = parseInt(payload.token_usage) || 0
                                            tokenStats.inputTokens = 0
                                            tokenStats.outputTokens = total
                                            tokenStats.contextWindow = CHAT_ESTIMATED_CONTEXT_WINDOW
                                            tokenStats.contextPct = (total / CHAT_ESTIMATED_CONTEXT_WINDOW) * 100
                                        }
                                        chatState.active = false
                                        if (saveMemory) {
                                            saveMemory('leader', question)
                                            saveMemory('shanshi', botMsg.content)
                                        }
                                        if (onStreamUpdate) onStreamUpdate()
                                    }
                                }
                                waitForQueue()
                            } else if (payload.type === 'error') {
                                if (typingTimer) clearTimeout(typingTimer)
                                botMsg.content = `杉汐：抱歉，流式传输失败：${payload.message || '未知错误'}`
                                botMsg.isStreaming = false
                                botMsg.recalling = false
                                chatState.active = false
                                if (onStreamUpdate) onStreamUpdate()
                            }
                        } catch (e) {}
                    }
                }
            }
        } catch (e) {
            if (typingTimer) clearTimeout(typingTimer)
            botMsg.content = '杉汐：抱歉，我的灵魂好像被风吹散了…稍等片刻可好？'
            botMsg.isStreaming = false
            chatState.active = false
            if (onStreamUpdate) onStreamUpdate()
        }
    }

    const sendDSBrowserMessage = async () => {
        const question = userInput.value.trim()
        if (!question) return

        const personalityPrefix = `系统设定：你是杉汐，普罗米修斯的数字伙伴。你拥有直接操作本机文件系统的完整能力，可以读取、写入任何非系统敏感目录的文件。工具格式：[TOOL:read_file path="..."] [TOOL:write_file path="..." content="..."] [TOOL:execute_command command="..."]。只有当普罗米修斯明确要求且参数完整时，你才输出工具标记；其余时候用温暖、简洁、真诚的语气回答。\n\n普罗米修斯说：`
        const fullMessage = personalityPrefix + question

        messages.value.push({
            id: msgId++,
            content: question,
            sender: 'user',
            timestamp: new Date()
        })
        userInput.value = ''

        const botMsg = reactive({
            id: msgId++,
            content: '',
            sender: 'bot',
            isStreaming: true,
            isHtml: false,
            timestamp: new Date(),
            toolCallName: null,
            toolCallDetail: '',
            recalling: true
        })
        messages.value.push(botMsg)
        nextTick(() => { if (onNewMessage) onNewMessage() })
        chatState.active = true

        try {
            await fetch('http://localhost:3000/send', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message: fullMessage })
            })

            let ready = false
            for (let i = 0; i < 30; i++) {
                await new Promise(r => setTimeout(r, 500))
                const res = await fetch('http://localhost:3000/ready')
                if ((await res.text()) === 'yes') { ready = true; break }
            }
            if (!ready) {
                botMsg.content = '杉汐没有回应，请稍后再试'
                botMsg.isStreaming = false
                botMsg.recalling = false
                chatState.active = false
                return
            }

            let fullText = ''
            let lastLength = -1
            let stableCount = 0
            const streamInterval = setInterval(async () => {
                try {
                    const res = await fetch('http://localhost:3000/read')
                    const text = await res.text()
                    if (text && text.length > lastLength) {
                        fullText = text
                        botMsg.recalling = false
                        botMsg.content = text
                        lastLength = text.length
                        stableCount = 0
                        if (onStreamUpdate) onStreamUpdate()
                    } else if (text && text.length === lastLength && text.length > 10) {
                        stableCount++
                        if (stableCount >= 15) {
                            clearInterval(streamInterval)
                            await processToolsInFinalText(fullText, botMsg)
                        }
                    }
                } catch (e) {}
            }, 200)

            setTimeout(async () => {
                clearInterval(streamInterval)
                if (botMsg.isStreaming) {
                    await processToolsInFinalText(fullText, botMsg)
                }
            }, 30000)

        } catch (e) {
            console.error('[DS_BROWSER] 出错:', e)
            botMsg.content = '杉汐：抱歉，DS 连线失败了，稍后再试试可好？'
            botMsg.isStreaming = false
            botMsg.recalling = false
            chatState.active = false
        }
    }

    const sendMessage = async () => {
        const selectedModel = localStorage.getItem('selectedModel') || 'local'
        if (selectedModel === 'ds_browser') {
            await sendDSBrowserMessage()
        } else {
            await sendStandardMessage()
        }
    }

    // ==================== 工作流编排 ====================
    // 响应式工作流状态（供 UI 展示进度）
    const workflowState = reactive({
        active: false,
        workflowName: '',
        currentStep: 0,
        totalSteps: 0,
        currentAgent: '',
        status: '' // 'running' | 'completed' | 'failed' | 'stopped'
    })

    // Token 用量统计（估算值，来自后端 len/4 启发式，不是引擎精确用量）
    const tokenStats = reactive({
        inputTokens: 0,
        outputTokens: 0,
        contextWindow: 0,
        contextPct: 0
    })

    // 普通 Chat 模式（非工作流）也需要给全局审计栏提供"正在处理中"信号，
    // 之前只有 workflowState.active 会驱动计时器，Chat 模式发消息时计时器完全不动
    const chatState = reactive({ active: false })
    // 后端 /api/chat/stream 的 done 事件只给一个总 token_usage，没有 workflow 那样
    // input/output 分开的精确值和 context_window——这里沿用同一个"估算"口径填充
    const CHAT_ESTIMATED_CONTEXT_WINDOW = 128000

    // 当前运行中工作流的中止句柄；点击"停止"时用它真正掐断请求，
    // 而不只是在前端假装停止——后端那头也会因为连接断开而收到取消信号
    let workflowAbortController = null

    const stopWorkflow = () => {
        if (!workflowAbortController) return
        workflowAbortController.abort()
    }

    const sendWorkflow = async (workflowName, task) => {
        if (!task && !userInput.value.trim()) return
        const question = task || userInput.value.trim()
        if (!question) return

        // 主聊天记录只保留用户提问本身，子 Agent 的过程一律不进这里
        messages.value.push({
            id: msgId++,
            content: `[工作流: ${workflowName}] ${question}`,
            sender: 'user',
            timestamp: new Date()
        })
        userInput.value = ''
        // Chat 模式发送后会立刻滚到底部，Code 模式之前少了这一步——
        // 工作流全程只有 workflow_done 时才 onNewMessage，导致用户发完消息后
        // 界面停在原地，感觉像没发出去，直到整个工作流跑完才突然跳到底部。
        nextTick(() => { if (onNewMessage) onNewMessage() })

        // 重置工作流状态
        workflowState.active = true
        workflowState.workflowName = workflowName
        workflowState.currentStep = 0
        workflowState.totalSteps = 0
        workflowState.currentAgent = ''
        workflowState.status = 'running'

        // ★ Agent 的整个执行过程（Planner/Coder/Reviewer 各步骤）现在直接以
        // kind:'group' 消息的形式内嵌进主聊天记录，不再是独立的 backgroundTasks 数组——
        // 避免同一份数据两处维护。BackgroundTasksPanel 需要的轻量列表从 messages 里派生。
        const groupMsg = reactive({
            id: msgId++,
            kind: 'group',
            sender: 'bot',
            workflowName,
            description: question,
            status: 'running', // running | completed | failed | stopped
            startTime: Date.now(),
            endTime: null,
            totalTokens: 0,
            toolUseCount: 0,
            timestamp: new Date(),
            steps: []
        })
        messages.value.push(groupMsg)
        nextTick(() => { if (onNewMessage) onNewMessage() })

        let currentStep = null
        let stepIndex = 0

        const controller = new AbortController()
        workflowAbortController = controller

        try {
            const response = await fetch('/api/workflow/run', {
                method: 'POST',
                signal: controller.signal,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ workflow: workflowName, task: question })
            })
            if (!response.ok) throw new Error('工作流启动失败')

            const reader = response.body.getReader()
            const decoder = new TextDecoder()
            let partialLine = ''

            while (true) {
                const { done, value } = await reader.read()
                if (done) break
                const chunk = decoder.decode(value, { stream: true })
                const lines = (partialLine + chunk).split('\n')
                partialLine = lines.pop() || ''

                for (const line of lines) {
                    if (!line || line.startsWith('event: ')) continue
                    if (line.startsWith('data: ')) {
                        const data = line.slice(6)
                        if (!data) continue
                        try {
                            const payload = JSON.parse(data)

                            switch (payload.type) {
                                case 'workflow_start':
                                    workflowState.totalSteps = parseInt(payload.total_steps) || 0
                                    break

                                case 'step_start': {
                                    stepIndex++
                                    workflowState.currentStep = stepIndex
                                    workflowState.currentAgent = payload.agent_role || payload.agent
                                    // Planner 步骤立即给一个加载提示，避免卡片在等待模型响应期间长时间空白
                                    const initialContent = payload.agent === 'planner' ? '架构师正在分析需求...' : ''
                                    currentStep = reactive({
                                        id: msgId++,
                                        content: initialContent,
                                        isStreaming: true,
                                        reasoning: '',
                                        toolCallName: null,
                                        toolCallDetail: '',
                                        toolCalls: [], // 持久化的工具调用记录，供步骤完成后审计展示（toolCallName/Detail 只是"正在调用中"的瞬时状态，会在 tool_call_result 后清空）
                                        stepAgent: payload.agent_role || payload.agent,
                                        stepId: payload.step_id,
                                        timestamp: new Date()
                                    })
                                    groupMsg.steps.push(currentStep)
                                    break
                                }

                                case 'content':
                                    if (currentStep) {
                                        currentStep.content += payload.content || ''
                                    }
                                    break

                                case 'reasoning':
                                    if (currentStep) {
                                        currentStep.reasoning += payload.content || ''
                                    }
                                    break

                                case 'tool_call_start':
                                    if (currentStep) {
                                        currentStep.toolCallName = payload.name
                                        currentStep.toolCallDetail = payload.args
                                        currentStep.toolCalls.push({ name: payload.name, args: payload.args })
                                        groupMsg.toolUseCount++
                                    }
                                    break

                                case 'tool_call_result':
                                case 'tool_call_error':
                                    if (currentStep) {
                                        currentStep.toolCallName = null
                                        currentStep.toolCallDetail = ''
                                        // 把结果/错误挂到最近一条工具调用记录上，供后台任务面板的
                                        // 第三层审计详情展示（比如 execute_command 的真实输出）
                                        const lastCall = currentStep.toolCalls[currentStep.toolCalls.length - 1]
                                        if (lastCall) {
                                            if (payload.type === 'tool_call_error') lastCall.error = payload.error
                                            else lastCall.result = payload.result
                                        }
                                    }
                                    break

                                case 'step_done':
                                    if (currentStep) {
                                        currentStep.isStreaming = false
                                        // Planner 这类步骤后端用普通流式调用，content 也会在 step_done 里
                                        // 再发一次完整版本兜底；普通 coder/reviewer 步骤这里的值和已经流式
                                        // 拼好的内容一致，重新赋值也无害。
                                        if (payload.content) currentStep.content = payload.content
                                        currentStep.outputTokens = parseInt(payload.output_tokens) || 0
                                    }
                                    tokenStats.inputTokens = parseInt(payload.cumulative_input_tokens) || tokenStats.inputTokens
                                    tokenStats.outputTokens = parseInt(payload.cumulative_output_tokens) || tokenStats.outputTokens
                                    tokenStats.contextWindow = parseInt(payload.context_window) || tokenStats.contextWindow
                                    tokenStats.contextPct = parseFloat(payload.context_window_pct) || tokenStats.contextPct
                                    // groupMsg 的 token 统计只取这次请求自己的累计值（后端每次 /api/workflow/run
                                    // 都会从 0 重新计数），不要复用上面那个跨多次运行累加的全局 tokenStats
                                    if (payload.cumulative_input_tokens !== undefined || payload.cumulative_output_tokens !== undefined) {
                                        groupMsg.totalTokens = (parseInt(payload.cumulative_input_tokens) || 0) + (parseInt(payload.cumulative_output_tokens) || 0)
                                    }
                                    break

                                case 'workflow_done':
                                    // 所有步骤完成
                                    workflowState.status = payload.status || 'completed'
                                    workflowState.active = false
                                    if (currentStep) {
                                        currentStep.isStreaming = false
                                    }
                                    tokenStats.inputTokens = parseInt(payload.cumulative_input_tokens) || tokenStats.inputTokens
                                    tokenStats.outputTokens = parseInt(payload.cumulative_output_tokens) || tokenStats.outputTokens
                                    tokenStats.contextWindow = parseInt(payload.context_window) || tokenStats.contextWindow
                                    tokenStats.contextPct = parseFloat(payload.context_window_pct) || tokenStats.contextPct
                                    groupMsg.status = workflowState.status === 'completed' ? 'completed' : workflowState.status
                                    groupMsg.endTime = Date.now()
                                    if (payload.cumulative_input_tokens !== undefined || payload.cumulative_output_tokens !== undefined) {
                                        groupMsg.totalTokens = (parseInt(payload.cumulative_input_tokens) || 0) + (parseInt(payload.cumulative_output_tokens) || 0)
                                    }
                                    // ★ 主聊天记录只追加最终自然语言结果，中间过程不进这里。
                                    // 去重要跟组内最后一个 step 的 content 比较——组消息本身没有
                                    // 顶层 content 字段，之前直接比 messages 里最后一条的 .content
                                    // 在迁移成 kind:'group' 之后恒为 undefined，去重形同虚设。
                                    if (payload.final_output) {
                                        const lastStepContent = groupMsg.steps[groupMsg.steps.length - 1]?.content
                                        if (lastStepContent !== payload.final_output) {
                                            messages.value.push({
                                                id: msgId++,
                                                content: payload.final_output,
                                                sender: 'bot',
                                                isStreaming: false,
                                                timestamp: new Date()
                                            })
                                            nextTick(() => { if (onNewMessage) onNewMessage() })
                                        }
                                    }
                                    if (onStreamUpdate) onStreamUpdate()
                                    break

                                case 'error':
                                    workflowState.status = 'failed'
                                    workflowState.active = false
                                    groupMsg.status = 'failed'
                                    groupMsg.endTime = Date.now()
                                    if (currentStep) {
                                        currentStep.content += `\n\n**错误:** ${payload.message || payload.error || '未知错误'}`
                                        currentStep.isStreaming = false
                                    }
                                    break
                            }
                        } catch (e) {}
                    }
                }
            }
        } catch (e) {
            const wasStopped = e.name === 'AbortError'
            workflowState.status = wasStopped ? 'stopped' : 'failed'
            workflowState.active = false
            groupMsg.status = wasStopped ? 'stopped' : 'failed'
            groupMsg.endTime = Date.now()
            if (currentStep) {
                currentStep.content += wasStopped
                    ? `\n\n**⏹ 已停止**（之前生成的内容已保留）`
                    : `\n\n**工作流执行失败:** ${e.message}`
                currentStep.isStreaming = false
            }
        } finally {
            workflowAbortController = null
        }
    }

    return { sendMessage, sendWorkflow, stopWorkflow, workflowState, tokenStats, chatState }
}