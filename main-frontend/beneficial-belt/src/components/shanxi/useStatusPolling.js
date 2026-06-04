// src/components/shanxi/useStatusPolling.js
import { ref } from 'vue'

export function useStatusPolling() {
  const status = ref('活跃中，随时准备帮忙')
  // 后续可改为真实轮询逻辑
  return { status }
}