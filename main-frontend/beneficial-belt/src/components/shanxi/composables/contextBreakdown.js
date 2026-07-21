// 上下文用量分类明细的共享 store + 持久化。
// 后端四态机在 model_info 事件里回传 context_breakdown（system/subagent/skill/memory/tools
// 各类 token 估算，口径=字符数/4），对话类 token 由 workflow_done 的 conversation_tokens 补
// （后端已从真实 prompt_tokens 里扣掉上述静态部分，避免双重计算）。
// 分类之和 ≈ 真实 prompt_tokens，底部横条与展开面板共用这一个口径。
// 整个 app 共享同一份；按 sessionId 落地 localStorage，刷新/切会话都不丢。
import { ref } from 'vue'

const KEY_PREFIX = 'aurora_ctx_breakdown_'

const EMPTY = { system: 0, subagent: 0, skill: 0, memory: 0, tools: 0, conversation: 0, contextWindow: 0 }

export const contextBreakdown = ref({ ...EMPTY })

export function loadContextBreakdown(sessionId) {
  if (!sessionId) return
  try {
    const raw = localStorage.getItem(KEY_PREFIX + sessionId)
    // 没有存档就必须清零：否则切到一个还没跑过工作流的新会话时，
    // 面板会继续显示上一个会话的分类数值（跟"切会话不重载"是同一类 bug）
    contextBreakdown.value = raw ? { ...EMPTY, ...JSON.parse(raw) } : { ...EMPTY }
  } catch (e) {
    contextBreakdown.value = { ...EMPTY }
  }
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
