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
    // 只还原工具标记的外层转义，不碰内部参数
    text = text
        .replace(/\\\[TOOL:/g, '[TOOL:') // 还原标记开头
        .replace(/\]\\/g, ']');           // 还原标记结尾

    console.log('[TOOL] 转义还原后文本：', text.substring(0, 500));

    if (!text || !text.includes('[TOOL:')) {
        botMsg.isStreaming = false;
        return;
    }

    const toolRegex = /\[TOOL:(\w+)\s+(.*?)\]\n?/g;
    let toolMatch;
    let finalText = text;
    let hasTool = false;

    while ((toolMatch = toolRegex.exec(text)) !== null) {
        hasTool = true;
        const marker = toolMatch[0].trim(); // 完整标记
        const toolName = toolMatch[1];
        let argsStr = toolMatch[2];

        // execute_command 特殊处理：安全提取命令参数
        if (toolName === 'execute_command') {
            const cmdStart = marker.indexOf('command="');
            if (cmdStart !== -1) {
                const valueStart = cmdStart + 'command="'.length;
                const lastQuote = marker.lastIndexOf('"');
                if (lastQuote > valueStart) {
                    let command = marker.substring(valueStart, lastQuote);
                    // 只还原命令内部被转义的引号 \" -> "
                    command = command.replace(/\\"/g, '"');
                    argsStr = `command="${command}"`;
                    console.log('[TOOL] 提取到的完整命令：', command);
                }
            }
        }

        botMsg.toolCallName = TOOL_NAME_MAP[toolName] || toolName;
        botMsg.toolCallDetail = argsStr;
        if (onStreamUpdate) onStreamUpdate();

        try {
            const toolRes = await fetch('/api/execute-marker', {
                method: 'POST',
                headers: { 'Content-Type': 'text/plain' },
                body: `[TOOL:${toolName} ${argsStr}]`
            });
            const resultText = await toolRes.text();

            // 创建新 bot 消息用于 DS 的自然语言回复
            const newBotMsg = reactive({
                id: msgId++,
                content: '',
                sender: 'bot',
                isStreaming: true,
                recalling: false,
                timestamp: new Date()
            });
            messages.value.push(newBotMsg);
            nextTick(() => { if (onNewMessage) onNewMessage(); });

            // 用 /stream 发送隐式消息，流式获取 DS 回复
            fetch('http://localhost:3000/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message: `[工具结果]\n${resultText}\n\n请用自然语言描述这个结果。` })
            }).then(async (res) => {
                const reader = res.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';
                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;
                    buffer += decoder.decode(value, { stream: true });
                    const lines = buffer.split('\n');
                    buffer = lines.pop() || '';
                    for (const line of lines) {
                        if (line.startsWith('data: ')) {
                            const payload = line.slice(6);
                            if (payload === '[DONE]') {
                                newBotMsg.isStreaming = false;
                                if (onStreamUpdate) onStreamUpdate();
                                return;
                            }
                            newBotMsg.content += payload;
                            if (onStreamUpdate) onStreamUpdate();
                        }
                    }
                }
            }).catch(() => {
                newBotMsg.content = '杉汐没有回应，请稍后再试';
                newBotMsg.isStreaming = false;
            });

            finalText = finalText.replace(marker, `[工具调用: ${toolName}]\n${resultText}\n`);
        } catch (e) {
            finalText = finalText.replace(marker, `[工具调用: ${toolName}]\n执行失败: ${e.message}\n`);
        }

        botMsg.toolCallName = null;
        botMsg.toolCallDetail = '';
    }

    if (hasTool) {
        botMsg.content = finalText;
        if (onStreamUpdate) onStreamUpdate();
    }
    botMsg.isStreaming = false;
};

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
            if (onStreamUpdate) onStreamUpdate()
        }
    }

    const sendDSBrowserMessage = async () => {
        const question = userInput.value.trim()
        if (!question) return

        // ★ 有人情味的人格前缀，工具能力软性融入
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

        try {
            // 1. 发送消息（带人格前缀）
            await fetch('http://localhost:3000/send', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message: fullMessage })
            })

            // 2. 等待 DS 回复就绪
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
                return
            }

            // 3. ★ 流式轮询显示（不处理工具调用）
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
                        if (stableCount >= 15) { // 连续 3 秒不变，流式结束
                            clearInterval(streamInterval)
                            await processToolsInFinalText(fullText, botMsg)
                        }
                    }
                } catch (e) {}
            }, 200)

            // 保底：30 秒后强制停止流式并处理工具
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

    return { sendMessage }
}