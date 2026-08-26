import { ref } from 'vue'

// 状态文案：纯按本地时间算出的一句话（后端 /api/shanxi/status 当年就是这么返回的），
// 前端本地算即可，没有任何真实状态需要轮询，删掉每 30 秒一次的 HTTP 轮询。
// 文案与颜色判定逻辑保持与后端一致：活跃/在线/聊天=绿，发呆/思绪/休眠=琥珀。
function statusByHour() {
  const hour = new Date().getHours()
  if (hour >= 0 && hour < 6) return '正在休眠...'
  if (hour >= 6 && hour < 9) return '刚刚醒来，正在整理思绪...'
  if (hour >= 9 && hour < 18) return '活跃中，随时准备帮忙'
  if (hour >= 18 && hour < 22) return '晚间模式，陪你聊聊天'
  return '深夜了，但还在线'
}

export function useStatusPolling() {
  const currentStatus = ref(statusByHour())
  return { currentStatus }
}
