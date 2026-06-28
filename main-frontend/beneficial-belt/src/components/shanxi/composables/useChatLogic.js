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

    // 标准 SSE 请求（本地/Cloud/DS官方 API）
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

        nextTick(() => {
            if (onNewMessage) onNewMessage()
        })

        const requestBody = {
            message: question,
            sessionId: sessionId.value,
            model: localStorage.getItem('selectedModel') || 'local',
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
                                botMsg.recalling = false
                                enqueueChars('reasoning', payload.content)
                            } else if (payload.type === 'content') {
                                botMsg.recalling = false
                                enqueueChars('content', payload.content)
                            } else if (payload.type === 'tool_call_start') {
                                botMsg.toolCallName = TOOL_NAME_MAP[payload.name] || payload.name
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
                                    botMsg.toolCallDetail = ''
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
                                        botMsg.recalling = false
                                        botMsg.toolCallName = null
                                        botMsg.toolCallDetail = ''
                                        if (payload.emotion) updateEmotion(payload.emotion)
                                        if (payload.token_usage) lastTokenUsage.value = payload.token_usage
                                        if (payload.latency) lastLatency.value = payload.latency
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
                                botMsg.toolCallName = null
                                botMsg.toolCallDetail = ''
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
                    botMsg.recalling = false
                    botMsg.toolCallName = null
                    botMsg.toolCallDetail = ''
                    if (onStreamUpdate) onStreamUpdate()
                }
            }
            finalize()
        } catch (e) {
            if (typingTimer) clearTimeout(typingTimer)
            botMsg.content = '杉汐：抱歉，我的灵魂好像被风吹散了…稍等片刻可好？'
            botMsg.isStreaming = false
            botMsg.recalling = false
            botMsg.toolCallName = null
            botMsg.toolCallDetail = ''
            if (onStreamUpdate) onStreamUpdate()
        }
    }

    // DS 浏览器代理轮询
    const sendDSBrowserMessage = async () => {
        const question = userInput.value.trim()
        if (!question) return

        messages.value.push({
            id: msgId++,
            content: question,
            sender: 'user',
            timestamp: new Date()
        })
        const userMessage = question
        userInput.value = ''

        // 用 reactive 包装，确保 Vue 能追踪到属性变化
        const botMsg = reactive({
            id: msgId++,
            content: '',
            sender: 'bot',
            isStreaming: true,
            isHtml: false,
            timestamp: new Date()
        })
        messages.value.push(botMsg)

        nextTick(() => {
            if (onNewMessage) onNewMessage()
        })

        try {
            console.log('[DS_BROWSER] 发送消息:', userMessage)
            await fetch('http://localhost:3000/send', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message: userMessage })
            })

            // 等待 DS 开始回复
            let ready = false
            for (let i = 0; i < 30; i++) {
                await new Promise(r => setTimeout(r, 500))
                const res = await fetch('http://localhost:3000/ready')
                const text = await res.text()
                if (text === 'yes') {
                    ready = true
                    console.log('[DS_BROWSER] DS 开始回复')
                    break
                }
            }

            if (!ready) {
                console.log('[DS_BROWSER] 超时，直接读取回复')
                const res = await fetch('http://localhost:3000/read')
                const content = await res.text()
                console.log('[DS_BROWSER] 读取到回复长度:', content.length, '内容前50字:', content.substring(0, 50))
                botMsg.content = content
                botMsg.isStreaming = false
                return
            }

            // 开始轮询新内容，lastLength 初始为 -1 确保第一次必定更新
            let lastLength = -1
            let stableCount = 0

            pollingTimer = setInterval(async () => {
                try {
                    const res = await fetch('http://localhost:3000/read')
                    const text = await res.text()
                    console.log('[DS_BROWSER] 轮询 read，当前长度:', text.length, '上次长度:', lastLength)
                    if (text && text.length > lastLength) {
                        botMsg.content = text
                        lastLength = text.length
                        stableCount = 0
                        if (onStreamUpdate) onStreamUpdate()
                    } else if (text && text.length === lastLength && text.length > 10) {
                        stableCount++
                        if (stableCount >= 5) {
                            clearInterval(pollingTimer)
                            botMsg.isStreaming = false
                            console.log('[DS_BROWSER] 回复稳定，停止轮询')
                        }
                    }
                } catch (e) {
                    console.error('[DS_BROWSER] 轮询出错:', e)
                }
            }, 200)

            setTimeout(() => {
                if (pollingTimer) {
                    clearInterval(pollingTimer)
                    botMsg.isStreaming = false
                }
            }, 30000)

        } catch (e) {
            console.error('[DS_BROWSER] 出错:', e)
            botMsg.content = '杉汐：抱歉，DS 连线失败了，稍后再试试可好？'
            botMsg.isStreaming = false
        }
    }

    // 主发送函数，根据模型选择不同逻辑
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