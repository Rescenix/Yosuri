import { ref, onMounted } from 'vue'

export function useWelcome() {
  const welcomeMessage = ref('你好！我是杉汐，你的数字伙伴。')
  const welcomeLoading = ref(false)

  const loadWelcome = async () => {
  const token = localStorage.getItem('token')
  if (!token) return // 未登录，使用默认欢迎语

  welcomeLoading.value = true
  try {
    const res = await fetch('/api/memory/welcome', {
      headers: { 'Authorization': `Bearer ${token}` }
    })
    if (res.ok) {
      const data = await res.json()
      welcomeMessage.value = data.message
    }
  } catch { /* 失败则保持默认 */ }
  finally {
    welcomeLoading.value = false
  }
}

  onMounted(loadWelcome)

  return { welcomeMessage, welcomeLoading }
}