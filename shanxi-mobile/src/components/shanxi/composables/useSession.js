import { ref } from 'vue'

export function useSession() {
  const sessionId = ref(localStorage.getItem('sessionId') || Date.now().toString(36))
  localStorage.setItem('sessionId', sessionId.value)

  return { sessionId }
}