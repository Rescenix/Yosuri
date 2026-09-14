// useRoleplay.js —— 会话级「角色扮演」模式。
//
// 每个会话独立记一个状态：
//   cast: string[] —— 本场戏的角色卡 id 列表（可多角色同框）
//   ''             —— 默认 Agent（工程/助手链路，带工具，走 yolo 模式）
//
// 存 localStorage（rpCast:<sessionId> = 逗号分隔 id 串），跟群聊成员一样归
// 前端管——切模式不改历史，后端不存，所以老会话不会因为升级凭空变成 RP。
//
// @ 呼人：RP 会话输入框打 @ 弹出场上角色（cast 成员 + Yosuri 导演），选中后
// 该条消息带 rpTarget 参数发给后端，导演据此让被点名者优先回应。

import { ref, computed } from 'vue'

const CAST_KEY_PREFIX = 'rpCast:'

// 按会话 id 记扮演角色卡 id 数组；空数组/无键 = 默认 Agent。
const rpMap = ref({})

function castKey(sid) {
  return CAST_KEY_PREFIX + (sid || '')
}

function readAll() {
  const out = {}
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i)
      if (k && k.startsWith(CAST_KEY_PREFIX)) {
        const raw = localStorage.getItem(k) || ''
        out[k.slice(CAST_KEY_PREFIX.length)] = raw ? raw.split(',') : []
      }
    }
  } catch { /* 隐私模式等环境 localStorage 不可用，静默当作全默认 */ }
  rpMap.value = out
}
readAll()

export function reloadRpMap() {
  readAll()
}

// rpCast 取某会话的扮演角色卡 id 列表（空数组 = 默认 Agent）。
export function rpCast(sid) {
  return rpMap.value[sid || ''] || []
}

// isRp 该会话是否处于角色扮演模式。
export function isRp(sid) {
  return rpCast(sid).length > 0
}

// setRp 设定某会话的扮演角色（可多张卡）；传空数组 = 切回默认 Agent。
// 切回 Agent 前把当前 cast 备份到 rpCastLast:<sid>，下次再进角色扮演直接恢复，
// 不用重新选卡（2026-09-14 用户嫌每次都要选太蠢）。
export function setRp(sid, agentIds) {
  sid = sid || ''
  const ids = Array.isArray(agentIds) ? agentIds.filter(Boolean) : (agentIds ? [agentIds] : [])
  try {
    if (ids.length) localStorage.setItem(castKey(sid), ids.join(','))
    else {
      // 退出前备份当前选卡；上次本来就有备份的话保留（先退再进不丢记忆）
      const cur = rpMap.value[sid]
      if (cur && cur.length) localStorage.setItem(CAST_KEY_PREFIX + 'last:' + sid, cur.join(','))
      localStorage.removeItem(castKey(sid))
    }
  } catch { /* 写不进去也不影响本次会话内存态 */ }
  rpMap.value = { ...rpMap.value, [sid]: ids }
}

// rpLast 该会话上一次的选卡（退出角色扮演时备份的），没有就空数组。
export function rpLast(sid) {
  try {
    const raw = localStorage.getItem(CAST_KEY_PREFIX + 'last:' + (sid || ''))
    return raw ? raw.split(',') : []
  } catch { return [] }
}

// rpPrimary 主扮演角色（第一张卡；@Yosuri 之外的默认回应者）。
export function rpPrimary(sid) {
  return rpCast(sid)[0] || ''
}

// rpCount 有多少会话开着 RP（侧栏小徽标用得上）。
export const rpCount = computed(() => Object.values(rpMap.value).filter(c => c && c.length).length)

export function useRoleplay() {
  return { rpMap, rpCast, rpPrimary, isRp, setRp, rpCount, reloadRpMap }
}
