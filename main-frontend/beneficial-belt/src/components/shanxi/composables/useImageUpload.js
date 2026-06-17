import { ref } from 'vue'

export function useImageUpload({ messages, sessionId, saveMemory }) {
  const imageInput = ref(null)
  let msgId = 0
// 在 useChatWidget.js 中，替换或新增 handleImageUpload
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
    if (onNewMessage) onNewMessage()

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
                    if (!line.startsWith('data: ')) continue
                    const dataStr = line.slice(6)
                    if (!dataStr) continue

                    try {
                        const payload = JSON.parse(dataStr)
                        switch (payload.type) {
                            case 'reasoning':
                                botMsg.recalling = false       // 开始思考，结束回忆
                                botMsg.reasoning += payload.content || ''
                                break
                            case 'content':
                                botMsg.recalling = false       // 开始回答，结束回忆
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
                        if (onStreamUpdate) onStreamUpdate()
                    } catch (e) {}
                }
            }
            // 确保最终状态
            botMsg.isStreaming = false
            botMsg.recalling = false
            if (onStreamUpdate) onStreamUpdate()
        } catch (err) {
            botMsg.content = '杉汐：抱歉，我的灵魂好像被风吹散了…稍等片刻可好？'
            botMsg.isStreaming = false
            botMsg.recalling = false
            if (onStreamUpdate) onStreamUpdate()
        }
    }
    reader.readAsDataURL(file)
}


  return { imageInput, handleImageUpload }
}