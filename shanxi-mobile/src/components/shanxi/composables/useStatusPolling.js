import { ref, onMounted, onUnmounted } from 'vue'

export function useStatusPolling() {
  const currentStatus = ref('正在发呆...')
  let timer = null

  onMounted(() => {
    timer = setInterval(async () => {
      try {
        const res = await fetch('/api/shanxi/status')
        if (res.ok) {
          const data = await res.json()
          currentStatus.value = data.status
        }
      } catch {}
    }, 30000)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { currentStatus }
}