export function useChatLogic({ messages, userInput, sessionId, updateEmotion, saveMemory, lastTokenUsage, lastLatency }) {
  let msgId = 0

  const sendMessage = async () => {
    const question = userInput.value.trim()
    if (!question) return

    // 1. 立即添加用户消息（明确 sender）
    messages.value.push({ 
      id: msgId++, 
      content: question, 
      sender: 'user'
    })
    userInput.value = ''

    // 2. 构造请求体
    const requestBody = {
      message: question,
      sessionId: sessionId.value,
      temperature: parseFloat(localStorage.getItem('debugTemp') || 0.7),
      top_p: parseFloat(localStorage.getItem('debugTopP') || 0.9),
      max_tokens: parseInt(localStorage.getItem('debugMaxTokens') || 2000),
      reasoning_effort: localStorage.getItem('debugReasoning') || undefined
    }

    // 切歌检测（保持不变）
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

    try {
      const token = localStorage.getItem('token')
      const headers = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      const res = await fetch('/api/chat', {
        method: 'POST',
        headers,
        body: JSON.stringify(requestBody)
      })
      if (!res.ok) throw new Error('Network error')
      const data = await res.json()

      if (!data.reply) {
        messages.value.push({ id: msgId++, content: '杉汐：抱歉，后端返回的数据格式有误…', sender: 'bot' })
        return
      }

      // 3. 添加杉汐回复
      messages.value.push({ id: msgId++, content: data.reply, sender: 'bot' })

      // 更新调试面板
      if (lastTokenUsage) lastTokenUsage.value = data.token_usage || '--'
      if (lastLatency) lastLatency.value = data.latency || '--'
      if (data.emotion) updateEmotion(data.emotion)

      if (data.action) {
        window.dispatchEvent(new CustomEvent('shanxi-action', { detail: { action: data.action } }))
      }

      // 记忆归档
      saveMemory('leader', question)
      saveMemory('shanshi', data.reply)
    } catch (e) {
      messages.value.push({
        id: msgId++,
        content: '杉汐：抱歉，我的灵魂好像被风吹散了…稍等片刻可好？',
        sender: 'bot'
      })
    }
  }

  return { sendMessage }
}