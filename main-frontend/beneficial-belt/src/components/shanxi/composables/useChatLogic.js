import { nextTick } from 'vue'

const TOOL_NAME_MAP = {
    web_search: '联网搜索',
    write_blog: '撰写博客',
    clean_memories: '清理记忆',
    search_codebase: '搜索代码库',
    read_file: '读取文件',
    write_file: '写入文件',
    execute_command: '执行命令'
}

export function useChatLogic({
    messages, userInput, sessionId,
    updateEmotion, saveMemory, lastTokenUsage, lastLatency,
    onNewMessage,
    onStreamUpdate
}) {
    let msgId = 0
    let lastToolStartTime = 0

    const sendMessage = async () => {
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
            recalling: true,            // ← 新增：正在回忆状态
            toolCallName: null,
            toolCallDetail: '',         // 新增字段
            sender: 'bot',
            isStreaming: true,
            timestamp: new Date()
        }
        messages.value.push(botMsg)

        nextTick(() => {
            if (onNewMessage) onNewMessage()
        })

        const requestBody = {
            message: question,
            sessionId: sessionId.value,
            temperature: parseFloat(localStorage.getItem('debugTemp') || 0.7),
            top_p: parseFloat(localStorage.getItem('debugTopP') || 0.9),
            max_tokens: parseInt(localStorage.getItem('debugMaxTokens') || 2000),
            reasoning_effort: localStorage.getItem('debugReasoning') || undefined
        }

        const musicPattern = /(换首歌|切歌|下一首|来首|放点|放一首|切换|换一首|切一下|切个歌|换歌|换一个)/
        if (musicPattern.test(question)) {
            const musicState = window.__musicState
            if (musicState) {
                const nextIndex = (musicState.currentIndex + 1) % musicState.playlist.length
                requestBody.nextSong = {
                    name: musicState.playlist[nextIndex].title,
                    src: musicState.playlist[nextIndex].src
                }
            }
        }

        const token = localStorage.getItem('token')
        const headers = { 'Content-Type': 'application/json' }
        if (token) headers['Authorization'] = `Bearer ${token}`

        let charQueue = []
        let typingTimer = null

        const processCharQueue = () => {
            if (charQueue.length === 0) {
                typingTimer = null
                return
            }
            const { type, char } = charQueue.shift()
            if (type === 'reasoning') {
                botMsg.reasoning += char
            } else if (type === 'content') {
                botMsg.content += char
            }
            if (onStreamUpdate) onStreamUpdate()
            typingTimer = setTimeout(processCharQueue, 20)
        }

        const enqueueChars = (type, text) => {
            if (!text) return
            const chars = [...text]
            chars.forEach(c => charQueue.push({ type, char: c }))
            if (!typingTimer) {
                processCharQueue()
            }
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
                    if (!line) continue
                    if (line.startsWith('event: ')) continue

                    if (line.startsWith('data: ')) {
                        const data = line.slice(6)
                        if (!data) continue

                        try {
                            const payload = JSON.parse(data)

                            if (payload.type === 'reasoning') {
                                botMsg.recalling = false   // ← 开始思考，结束回忆
                                enqueueChars('reasoning', payload.content)
                            } else if (payload.type === 'content') {
                                botMsg.recalling = false   // ← 开始回答，结束回忆
                                enqueueChars('content', payload.content)
                            } else if (payload.type === 'tool_call_start') {
                                botMsg.toolCallName = TOOL_NAME_MAP[payload.name] || payload.name
                                // 解析具体的指令参数
                                try {
                                    const argsObj = JSON.parse(payload.args)
                                    if (payload.name === 'execute_command') {
                                        botMsg.toolCallDetail = argsObj.command
                                    } else if (payload.name === 'read_file' || payload.name === 'write_file') {
                                        botMsg.toolCallDetail = argsObj.path
                                    } else if (payload.name === 'search_codebase' || payload.name === 'web_search') {
                                        botMsg.toolCallDetail = argsObj.query
                                    } else {
                                        botMsg.toolCallDetail = ''
                                    }
                                } catch (e) {
                                    botMsg.toolCallDetail = ''
                                }
                                lastToolStartTime = Date.now()
                                if (onStreamUpdate) onStreamUpdate()
                            } else if (payload.type === 'tool_call_result' || payload.type === 'tool_call_error') {
                                const elapsed = Date.now() - lastToolStartTime
                                const delay = Math.max(800 - elapsed, 0)
                                setTimeout(() => {
                                    botMsg.toolCallName = null
                                    botMsg.toolCallDetail = ''   // ← 清空详细指令
                                    if (onStreamUpdate) onStreamUpdate()
                                }, delay)
                                if (payload.type === 'tool_call_error') {
                                    botMsg.content += `\n[工具调用失败: ${payload.error}]\n`
                                }
                            } else if (payload.type === 'done') {
                                const waitForQueue = () => {
                                    if (charQueue.length > 0 || typingTimer) {
                                        setTimeout(waitForQueue, 100)
                                    } else {
                                        botMsg.content = payload.content || botMsg.content
                                        botMsg.reasoning = payload.reasoning || botMsg.reasoning
                                        botMsg.isStreaming = false
                                        botMsg.recalling = false    // ← 确保清除
                                        botMsg.toolCallName = null
                                        botMsg.toolCallDetail = ''   // ← 清空详细指令
                                        if (payload.emotion) updateEmotion(payload.emotion)
                                        if (payload.token_usage) lastTokenUsage.value = payload.token_usage
                                        if (payload.latency) lastLatency.value = payload.latency
                                        saveMemory('leader', question)
                                        saveMemory('shanshi', botMsg.content)
                                        if (onStreamUpdate) onStreamUpdate()
                                    }
                                }
                                waitForQueue()
                            } else if (payload.type === 'error') {
                                if (typingTimer) clearTimeout(typingTimer)
                                botMsg.content = `杉汐：抱歉，流式传输失败：${payload.message || '未知错误'}`
                                botMsg.isStreaming = false
                                botMsg.recalling = false    // ← 确保清除
                                botMsg.toolCallName = null
                                botMsg.toolCallDetail = ''   // ← 清空详细指令
                                if (onStreamUpdate) onStreamUpdate()
                            }
                        } catch (e) {
                            // ignore
                        }
                    }
                }
            }

            const finalize = () => {
                if (charQueue.length > 0 || typingTimer) {
                    setTimeout(finalize, 100)
                } else {
                    botMsg.isStreaming = false
                    botMsg.recalling = false    // ← 确保清除
                    botMsg.toolCallName = null
                    botMsg.toolCallDetail = ''   // ← 清空详细指令
                    if (onStreamUpdate) onStreamUpdate()
                }
            }
            finalize()
        } catch (e) {
            if (typingTimer) clearTimeout(typingTimer)
            botMsg.content = '杉汐：抱歉，我的灵魂好像被风吹散了…稍等片刻可好？'
            botMsg.isStreaming = false
            botMsg.recalling = false    // ← 确保清除
            botMsg.toolCallName = null
            botMsg.toolCallDetail = ''   // ← 清空详细指令
            if (onStreamUpdate) onStreamUpdate()
        }
    }

    return { sendMessage }
}