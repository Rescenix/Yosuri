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
import { BUILTIN_AGENT_CARDS, MASCOT_AVATAR_URL } from './agentPresets.js'

const GROUP_KEY_PREFIX = 'agentGroup:'
// 内置角色卡只播种一次；播过之后用户删掉的卡不会自己长回来。
const SEEDED_KEY = 'builtinAgentCardsSeeded'

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
    await migrateLegacyPersona()
    await seedBuiltinAgents()
    await backfillBuiltinAgentColors()
  } catch (e) {
    console.error('加载 Agent 失败', e)
  } finally {
    agentsLoaded.value = true
  }
}

// ── 旧版人设迁移（2026-09-11 人设→角色卡）────────────────────
// 老用户的人设活在 localStorage：myPersonas（我的预设）、persona（当前生效）。
// 改造后这套 UI 删了，必须把它们落成真实角色卡，否则数据留着也永远看不到。
// 幂等设计：我的预设只在首跑迁移；「当前生效人设」每次启动都补漏一次
// （键还在就对下落卡/设 activeAgentId），后端失败不置位，下次再试。绝不删键。
const PERSONA_MIGRATED_KEY = 'personaMigratedV1'

const readLegacyMyPersonas = () => {
  try {
    const raw = localStorage.getItem('myPersonas')
    const arr = raw ? JSON.parse(raw) : []
    return Array.isArray(arr) ? arr.filter((p) => p && p.name && p.prompt) : []
  } catch { return [] }
}

export async function migrateLegacyPersona() {
  const already = localStorage.getItem(PERSONA_MIGRATED_KEY) === '1'
  const createCard = async (name, persona) => {
    if (agents.value.some((a) => a.name === name && a.persona === persona)) return null
    return saveAgent({ name, persona })
  }
  try {
    // 1) 我的预设 → 各一张角色卡（只在首次跑；同名同文案的跳过，不覆盖用户编辑）
    if (!already) {
      for (const p of readLegacyMyPersonas()) {
        await createCard(p.name, p.prompt)
      }
    }
    // 2) 当前生效人设 → 对下落成角色卡（每次都补漏，防旧版清键/中途抖动丢数据）。
    //    但 activeAgentId 已有值 = 用户在角色卡里明确选过，绝不覆盖——
    //    旧 persona 键是迁移前的残留，拿它冲掉用户新选择会导致「刷新回到旧角色卡」。
    const active = (localStorage.getItem('persona') || '').trim()
    const userPicked = !!localStorage.getItem('activeAgentId')
    if (active) {
      const hit = agents.value.find((a) => a.persona === active)
      const builtin = BUILTIN_AGENT_CARDS.find((c) => c.persona === active)
      if (hit && !userPicked) {
        // 已有同名同文案的卡（用户自己建的）：直接用
        localStorage.setItem('activeAgentId', hit.id)
      } else if (builtin && !userPicked) {
        // 内置预设原文：指到内置卡 id，播种紧接着会建（顺序：迁移→播种）
        localStorage.setItem('activeAgentId', builtin.id)
      } else if (!hit && !builtin) {
        // 自定义人设：落成「我的人设」卡（数据不丢），仅用户还没选过卡时才顺带选中
        const card = await createCard('我的人设', active)
        const mine = card || agents.value.find((a) => a.name === '我的人设')
        if (mine && !userPicked) localStorage.setItem('activeAgentId', mine.id)
      }
    }
    // 3) 数据已保证落进 agents.json 才标记完成。
    //    旧 localStorage 键一律不删——值与对应角色卡文案一致，留着无害，
    //    且任何意外都能据此回滚。等下一个版本确认无人异常再清。
    localStorage.setItem(PERSONA_MIGRATED_KEY, '1')
  } catch (e) {
    console.error('旧人设迁移失败，保留原状下次再试', e)
  }
}

// 本地图片转 dataURL（头像走 /api/agents/:id/avatar 的 dataURL 契约）。
async function imageToDataURL(url) {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const blob = await res.blob()
  return await new Promise((resolve, reject) => {
    const fr = new FileReader()
    fr.onload = () => resolve(String(fr.result || ''))
    fr.onerror = () => reject(new Error('头像读取失败'))
    fr.readAsDataURL(blob)
  })
}

// 内置角色卡播种：第一次启动把内置的几张角色卡落进 agents.json，
// 默认那张（Yosuri酱）带头像——白发看板娘。
// 只播一次：播过之后用户删掉的卡不会自己长回来；后端没起时静默跳过，下次启动再试。
export async function seedBuiltinAgents() {
  if (localStorage.getItem(SEEDED_KEY) === '1') return
  try {
    let mascot = ''
    for (const c of BUILTIN_AGENT_CARDS) {
      // 按 id 或按名字跳过：旧版人设里用户自己建过同名卡（中文名 slugify 成
      // agent/agent-2），播种只查 id 会再建一张一模一样的 → 双卡。同名不播。
      if (agents.value.some(a => a.id === c.id || a.name === c.name)) continue
      const saved = await saveAgent({ id: c.id, name: c.name, persona: c.persona, color: c.color || '' })
      if (c.id === BUILTIN_AGENT_CARDS[0].id) {
        if (!mascot) mascot = await imageToDataURL(MASCOT_AVATAR_URL).catch(() => '')
        if (mascot) await saveAgentAvatar(saved.id, mascot)
      }
    }
    localStorage.setItem(SEEDED_KEY, '1')
  } catch (e) {
    console.error('内置角色卡播种失败', e)
  }
}

// 内置卡补色（2026-09-12 发言颜色功能上线）：老用户已播种的内置卡 color 为空，
// 播种只跑一次不会重来——这里把缺色的内置卡补上各自默认色。仅补内置 id，
// 用户自定义卡不碰；用户自己改过色的不覆盖（color 已有值 = 用户选过）。
export async function backfillBuiltinAgentColors() {
  try {
    for (const c of BUILTIN_AGENT_CARDS) {
      const hit = agents.value.find(a => a.id === c.id)
      if (hit && !hit.color && c.color) {
        await saveAgent({ ...hit, color: c.color })
      }
    }
  } catch (e) {
    console.error('内置角色卡补色失败', e)
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

