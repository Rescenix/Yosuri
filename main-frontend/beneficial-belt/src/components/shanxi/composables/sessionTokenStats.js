// 会话级 token 统计的持久化 store（按 sessionId 落 localStorage，刷新不丢）。
// 普通聊天路径的 token 统计以前只存在内存里的 tokenStats，刷新即清零——
// 这里把它按会话落地，满足"绑定会话显示实际数值"。
import { ref } from 'vue'

const KEY_PREFIX = 'aurora_session_tokens_'

export const sessionTokenStats = ref({
  inputTokens: 0,
  outputTokens: 0,
  contextWindow: 0,
  contextPct: 0,
  latencyMs: 0
})

const FALLBACK = { inputTokens: 0, outputTokens: 0, contextWindow: 0, contextPct: 0, latencyMs: 0 }

export function loadSessionTokenStats(sessionId) {
  const sid = sessionId || localStorage.getItem('prism_session_id') || ''
  if (!sid) return { ...FALLBACK }
  try {
    const raw = localStorage.getItem(KEY_PREFIX + sid)
    if (raw) return { ...FALLBACK, ...JSON.parse(raw) }
  } catch (e) {}
  return { ...FALLBACK }
}

export function persistSessionTokens(stats, sessionId) {
  const sid = sessionId || localStorage.getItem('prism_session_id') || ''
  if (!sid) return
  try {
    localStorage.setItem(KEY_PREFIX + sid, JSON.stringify({
      inputTokens: stats.inputTokens || 0,
      outputTokens: stats.outputTokens || 0,
      contextWindow: stats.contextWindow || 0,
      contextPct: stats.contextPct || 0,
      latencyMs: stats.latencyMs || 0
    }))
  } catch (e) {}
}
