// 上下文用量分类明细的共享 store + 持久化。
// 后端四态机在 model_info 事件里回传 context_breakdown（system/subagent/skill/memory/tools
// 各类 token 估算，口径=字符数/4），对话类 token 由前端用 workflow_done 的 input_tokens 补。
// 整个 app 共享同一份；按 sessionId 落地 localStorage，刷新不丢。
import { ref } from 'vue'

const KEY_PREFIX = 'aurora_ctx_breakdown_'

export const contextBreakdown = ref({ system: 0, subagent: 0, skill: 0, memory: 0, tools: 0, conversation: 0, contextWindow: 0 })

export function loadContextBreakdown(sessionId) {
  if (!sessionId) return
  try {
    const raw = localStorage.getItem(KEY_PREFIX + sessionId)
    if (raw) contextBreakdown.value = JSON.parse(raw)
  } catch (e) {}
}

// 后端 model_info 回填分类占用（不含对话，对话由 workflow_done 补）
export function setContextBreakdownFromBackend(cb, contextWindow) {
  contextBreakdown.value = {
    ...contextBreakdown.value,
    system: cb?.system || 0,
    subagent: cb?.subagent || 0,
    skill: cb?.skill || 0,
    memory: cb?.memory || 0,
    tools: cb?.tools || 0,
    contextWindow: contextWindow || contextBreakdown.value.contextWindow
  }
  persist()
}

// 对话类 token 随每次工作流结束回填（后端 input_tokens=历史字符/4，与分类口径一致）
export function setConversationTokens(n, sessionId) {
  contextBreakdown.value = { ...contextBreakdown.value, conversation: n || 0 }
  persist(sessionId)
}

function persist(sessionId) {
  const sid = sessionId || localStorage.getItem('prism_session_id') || ''
  if (!sid) return
  try { localStorage.setItem(KEY_PREFIX + sid, JSON.stringify(contextBreakdown.value)) } catch (e) {}
}
