import { ref } from 'vue'

export function useImageUpload({ messages, sessionId, saveMemory }) {
  const imageInput = ref(null)
  let msgId = 0

  function handleImageUpload(event) {
    const file = event.target.files[0]
    if (!file) return

messages.value.push({
    id: msgId++,
    type: 'image',           // 新增类型字段
    image: URL.createObjectURL(file), // 本地预览URL
    sender: 'user'
})

    const reader = new FileReader()
    reader.onload = async (e) => {
      const base64 = e.target.result.split(',')[1]

      const requestBody = {
        message: '帮我看看这张图片',
        sessionId: sessionId.value,
        image: base64
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
        const data = await res.json()
        messages.value.push({ id: msgId++, content: data.reply, sender: 'bot' })
        saveMemory('shanshi', data.reply)
      } catch {
        messages.value.push({
          id: msgId++,
          content: '杉汐：抱歉，我暂时看不清这张图片...',
          sender: 'bot'
        })
      }
    }
    reader.readAsDataURL(file)
  }

  return { imageInput, handleImageUpload }
}