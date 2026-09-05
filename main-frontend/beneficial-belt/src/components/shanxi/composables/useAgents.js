// useAgents.js —— 多 Agent 角色卡：列表、当前发言者、群聊成员。
//
// 数据分两层：
//   - 角色卡（后端 ~/rescene_data/agents.json + agents/<id>/avatar）：
//     名字 / 人设文案 / 头像 / 名牌色，跨会话持久，走 /api/agents。
//   - 群聊成员（localStorage，按会话存）：这个对话里挂了哪几个 Agent。
//     一个对话可以挂多个 Agent，发消息时按成员顺序依次点名发言。
//
// 记忆分两层（后端负责，这里只是概念说明）：
//   - 通用记忆 ~/rescene_data/memory/ —— 所有 Agent 共享
//   - 私有记忆 ~/rescene_data/agents/<id>/memory/ —— 每个 Agent 各一份

import { ref, computed } from 'vue'

const GROUP_KEY_PREFIX = 'agentGroup:'

export const agents = ref([])
export const agentsLoaded = ref(false)

async function req(url, opts) {
  const res = await fetch(url, opts)
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try { msg = (await res.json()).error || msg } catch { /* 非 JSON 响应保留状态码 */ }
    throw new Error(msg)
  }
  return res.json()
}

export async function loadAgents() {
  try {
    const d = await req('/api/agents')
    agents.value = d.agents || []
  } catch (e) {
    console.error('加载 Agent 失败', e)
  } finally {
    agentsLoaded.value = true
  }
}

export async function saveAgent(card) {
  const d = await req('/api/agents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(card),
  })
  if (d.agent) {
    const i = agents.value.findIndex(a => a.id === d.agent.id)
    if (i >= 0) agents.value[i] = d.agent
    else agents.value.push(d.agent)
  }
  return d.agent
}

export async function deleteAgent(id) {
  await req(`/api/agents/${encodeURIComponent(id)}`, { method: 'DELETE' })
  agents.value = agents.value.filter(a => a.id !== id)
}

export async function saveAgentAvatar(id, dataURL) {
  await req(`/api/agents/${encodeURIComponent(id)}/avatar`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ data: dataURL }),
  })
  const a = agents.value.find(x => x.id === id)
  if (a) a.avatar = dataURL
}

export async function loadAgentMemory(id) {
  const d = await req(`/api/agents/${encodeURIComponent(id)}/memory`)
  return d.files || []
}

// ── 群聊成员：按会话存 localStorage，一个对话挂多个 Agent ──

export function groupOf(sessionId) {
  if (!sessionId) return []
  try {
    const raw = localStorage.getItem(GROUP_KEY_PREFIX + sessionId)
    const ids = raw ? JSON.parse(raw) : []
    return Array.isArray(ids) ? ids : []
  } catch {
    return []
  }
}

export function setGroup(sessionId, ids) {
  if (!sessionId) return
  localStorage.setItem(GROUP_KEY_PREFIX + sessionId, JSON.stringify(ids))
}

export function useAgentsStore() {
  const currentAgentId = ref(localStorage.getItem('activeAgentId') || '')
  const currentAgent = computed(() =>
    agents.value.find(a => a.id === currentAgentId.value) || null)

  function selectAgent(id) {
    currentAgentId.value = id || ''
    if (id) localStorage.setItem('activeAgentId', id)
    else localStorage.removeItem('activeAgentId')
  }

  function agentById(id) {
    return agents.value.find(a => a.id === id) || null
  }

  return {
    agents, agentsLoaded, currentAgentId, currentAgent, selectAgent, agentById,
    loadAgents, saveAgent, deleteAgent, saveAgentAvatar, loadAgentMemory,
    groupOf, setGroup,
  }
}

