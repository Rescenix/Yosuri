// useAuth —— 全局登录态单例：统一验真 + 缓存 GitHub 用户名/头像 + cloud 分发的 UID。
// 取代各组件零散的 localStorage.getItem('token') 判断，杜绝“任意字符串即登录”。
// 所有组件 import 同一个模块实例，响应式 state 共享，auth-change 时自动刷新。
//
// UID 账号体系：UID 由 ResceneCloud 验证并分发（前端不可伪造）。
//   - 游客：设备指纹（device_id）向 /api/auth/uid 换 UID，同一设备恒定；
//   - 登录：refresh 成功后调 /api/auth/uid/bind，把游客 UID 升级为正式账号，
//     此后换设备/清缓存重新登录即恢复同一 UID —— 用户才会珍惜账号。
import { ref, computed, readonly } from 'vue'
import { computeHardwareFingerprint } from '../utils/hardwareFingerprint.js'

const isLoggedIn = ref(false)
const login = ref('')   // Rescene Cloud 账号名
const name = ref('')    // 账号显示名
const avatar = ref('')  // 账号头像 URL
const customAvatar = ref('') // 用户在本机设置的头像（优先于账号头像）
const uid = ref(null)   // cloud 分发的账号 UID（游客/登录都有）
const isVip = ref(false) // 会员标识：仅由服务端 JWT 的 is_vip 决定，绝不读 localStorage（堵住游客伪造）
const intimacy = ref(0) // 亲密度：无上限互动值（云端权威，随 UID 账号存储、跨设备保留）
const email = ref('')   // 账号绑定的邮箱（云端脱敏后返回：前3后3、不含后缀；空=未绑定）
// authError：/api/auth/me 请求本身失败（网络/云端 502 等，不是「token 无效」）时的提示。
// 之前这类瞬时故障会被当成 token 无效直接清掉、悄悄退回未登录态，用户毫无提示——
// 看起来就是「login过后过一会又要求登录」（2026-08-20 用户反馈：前端反馈不鲁棒）。
const authError = ref('')
let refreshing = false

const DEVICE_KEY = 'aurora_device_id'
const UID_KEY = 'aurora_uid'
const INTIMACY_KEY = 'aurora_intimacy'
const USER_AVATAR_KEY = 'rescene_user_avatar_v1'
// 游客 token：uid 分发时云端签发（guest JWT，含 uid claim），用于查询本人统计/亲密值等
// 内部数据（云端 AuthRequired + uid 匹配，只放行「查自己」）。独立 key，不覆盖登录 token。
const GUEST_TOKEN_KEY = 'aurora_guest_token'

// 设备指纹：首次生成 UUID 存 localStorage（同一设备恒定；换设备/清缓存 = 新游客号）
function getDeviceId() {
  let d = localStorage.getItem(DEVICE_KEY)
  if (!d) {
    d = (crypto.randomUUID && crypto.randomUUID())
      || ('d-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 10))
    localStorage.setItem(DEVICE_KEY, d)
  }
  return d
}

// 请求鉴权头：优先登录 token，游客（未登录）用云端签发的 guest token。
function authHeaders(extra) {
  const token = localStorage.getItem('token') || localStorage.getItem(GUEST_TOKEN_KEY) || ''
  return Object.assign({}, extra, token ? { Authorization: 'Bearer ' + token } : {})
}

// 向 cloud 请求/取回本设备 UID：同一 device_id 恒定返回同一 UID（幂等）。
// 附带硬件指纹（清缓存不变，云端可识别同一台机器）。
async function fetchUid() {
  try {
    const res = await fetch('/api/auth/uid', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        device_id: getDeviceId(),
        fingerprint: computeHardwareFingerprint()
      })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.uid) {
        uid.value = data.uid
        localStorage.setItem(UID_KEY, String(data.uid))
      }
      // 签发的游客 JWT 存起来（用于查询本人统计/亲密值等内部数据鉴权）
      if (data.token) {
        localStorage.setItem(GUEST_TOKEN_KEY, data.token)
        // 交给本地后端缓存：记忆同步（push/pull）是 Go 后端在跑，需要这份 token
        try {
          fetch('/api/auth/guest-token', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token: data.token })
          })
        } catch { /* 本地后端不可达时忽略，下次启动再补 */ }
      }
    }
  } catch {
    // cloud 不可达：保留本地已有 UID，下次启动再校准
  }
}

// 登录成功后把 member token（登录账号 JWT）交给本地后端缓存，供云端记忆同步 push/pull 鉴权。
// 关键：登录用户必须用登录 token 推记忆——guest token 的 uid 是游客号，与登录账号 uid
// 不匹配会被云端 requireUIDMatch 拒掉（403 静默失败 = 登录用户零上传，2026-08-30 实锤）。
async function storeLoginToken() {
  const token = localStorage.getItem('token')
  if (!token) return
  try {
    await fetch('/api/auth/login-token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token })
    })
  } catch { /* 本地后端不可达时忽略，下次 refresh 再补 */ }
}

// 从后端持久化文件恢复登录 token（2026-08-30：WebView2 数据目录随 exe 路径变化，
// localStorage 会被清空导致登录身份丢失——token 已由 storeLoginToken 写到
// rescene_data/cloud_login_token，重启后从这里捞回来，登录态不再依赖浏览器存储）。
// ⚠️ 不能在模块顶层执行时依赖 fetch 桥（desktopTransport 的 /api 改写要在
// installDesktopTransport() 之后才生效）——直接用 Wails 注入的 BackendURL 拼绝对地址，
// 否则启动即执行时 fetch('/api/...') 打到 AssetServer 返回 404，恢复静默失败。
async function restoreLoginToken() {
  // 不以「本地有无 token」为准——localStorage 里可能残留旧/失效 token（WebView2 数据
  // 目录变化时 localStorage 未必清空），那会跳过恢复导致 refresh 401。后端 cloud_login_token
  // 是最近一次登录成功写入的（登出会删除），比 localStorage 更权威：有就覆盖。
  let base = ''
  try {
    const binding = globalThis.go?.main?.DesktopApp?.BackendURL
    if (typeof binding === 'function') base = String(await binding()).replace(/\/+$/, '')
  } catch { /* 非 Wails 环境 */ }
  if (!base) base = globalThis.__RESCENE_BACKEND_URL__ || ''
  try {
    const res = await fetch(base + '/api/auth/login-token')
    if (!res.ok) return
    const data = await res.json()
    if (data && data.token) {
      localStorage.setItem('token', data.token)
    }
  } catch { /* 本地后端不可达：本次不恢复，保持未登录 */ }
}

// 登录成功后把游客 UID 升级为正式账号：UID 不变，永久保留（换设备可恢复）。
async function bindUid() {
  try {
    const token = localStorage.getItem('token')
    if (!token) return
    const res = await fetch('/api/auth/uid/bind', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
      body: JSON.stringify({ device_id: getDeviceId() })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.uid) {
        uid.value = data.uid
        localStorage.setItem(UID_KEY, String(data.uid))
        // UID 变化（并入正式账号）后重新拉取亲密度
        fetchIntimacy()
      }
    }
  } catch {
    // 绑定失败不阻断登录，下次启动 refresh 时重试
  }
}

// 亲密度：无上限互动值，云端权威（随 UID 账号存储，跨设备保留）。
// 每次用户发消息 +1（incIntimacy），启动时 fetchIntimacy 校准。
// 拉取当前亲密度：失败保留本地缓存值，不阻断启动。
async function fetchIntimacy() {
  const u = uid.value
  if (!u) return
  try {
    const res = await fetch('/api/auth/intimacy?uid=' + u, { headers: authHeaders() })
    if (res.ok) {
      const data = await res.json()
      if (typeof data.intimacy === 'number') {
        intimacy.value = data.intimacy
        localStorage.setItem(INTIMACY_KEY, String(data.intimacy))
      }
    }
  } catch {
    // 云端不可达：保留本地缓存值（离线也能展示最近一次）
  }
}

// 亲密度 +1（用户发消息时调用）。fire-and-forget：失败静默，绝不阻断发送。
async function incIntimacy() {
  const u = uid.value
  if (!u) return
  try {
    const res = await fetch('/api/auth/intimacy/inc', {
      method: 'POST',
      headers: authHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ uid: u, delta: 1 })
    })
    if (res.ok) {
      const data = await res.json()
      if (typeof data.intimacy === 'number') {
        intimacy.value = data.intimacy
        localStorage.setItem(INTIMACY_KEY, String(data.intimacy))
      }
    }
  } catch {
    // 离线也照常聊天，亲密度下次在线时校准
  }
}

// 亲密等级：外显 Lv.N（无上限，QQ 宠物式曲线——越高越难升）。
// 与后端同公式：升到 Lv.N 需总亲密值 100*N*(N-1)/2。
const intimacyLevel = computed(() => {
  const v = intimacy.value
  if (!v) return 0
  const x = v / 100
  return Math.floor((1 + Math.sqrt(1 + 8 * x)) / 2)
})

// 云端记忆同步（可选）：uid 到位后显式拉取一次，跨设备即时恢复记忆。
// 与 re0 启动自动拉取互补：启动拉覆盖重启场景，这里覆盖首次登录场景。
async function pullCloudMemory() {
  const u = uid.value
  if (!u) return
  try {
    await fetch('/api/memory/sync/pull', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ uid: u })
    })
  } catch {
    // 云端不可达：下次启动自动拉取再补
  }
}

// 展示名：登录用账号名，未登录用 cloud 分发的 UID
const displayName = computed(() => {
  if (isLoggedIn.value) return name.value || login.value || 'Ameko 用户'
  const u = uid.value
  return u ? 'UID ' + u : '未登录'
})

// 展示头像：仅登录账号可见（本机自定义头像优先，其次账号头像）。
// 2026-09-04 修复：退出登录后必须回到游客态——此前自定义头像不受登录态约束，
// 登出后头像仍显示，看起来像"没退出去"。自定义头像数据本身保留在本地与后端，
// 重新登录即恢复，不做删除。
const displayAvatar = computed(() => {
  if (!isLoggedIn.value) return ''
  if (customAvatar.value) return customAvatar.value
  return avatar.value || ''
})
const hasCustomAvatar = computed(() => isLoggedIn.value && Boolean(customAvatar.value))

// —— 自定义头像持久化（2026-09-01）——
// 头像此前只存 localStorage，而 WebView2 数据目录随 exe 路径变化会被清空 →
// 重开应用头像丢失。参照 login token 模式：写盘到后端 rescene_data/custom_avatar，
// 启动时从后端恢复，不再依赖浏览器存储。fire-and-forget：落盘失败静默，localStorage 仍兜底。
let cachedBackendBase = ''
async function backendBase() {
  if (cachedBackendBase) return cachedBackendBase
  try {
    const binding = globalThis.go?.main?.DesktopApp?.BackendURL
    if (typeof binding === 'function') cachedBackendBase = String(await binding()).replace(/\/+$/, '')
  } catch { /* 非 Wails 环境 */ }
  if (!cachedBackendBase) cachedBackendBase = globalThis.__RESCENE_BACKEND_URL__ || ''
  return cachedBackendBase
}

async function persistCustomAvatar(dataUrl) {
  const b = await backendBase()
  try {
    await fetch(b + '/api/profile/avatar', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ data: dataUrl })
    })
  } catch { /* 本地后端不可达：头像仍在 localStorage，下次再补 */ }
}

async function clearPersistedAvatar() {
  const b = await backendBase()
  try {
    await fetch(b + '/api/profile/avatar', { method: 'DELETE' })
  } catch { /* 忽略 */ }
}

async function restoreCustomAvatar() {
  const b = await backendBase()
  try {
    const res = await fetch(b + '/api/profile/avatar')
    if (!res.ok) return
    const data = await res.json()
    if (data && data.avatar) {
      customAvatar.value = data.avatar
      localStorage.setItem(USER_AVATAR_KEY, data.avatar)
    }
  } catch { /* 本地后端不可达：保留 localStorage 兜底 */ }
}

function setCustomAvatar(dataUrl) {
  const value = typeof dataUrl === 'string' ? dataUrl.trim() : ''
  if (!value) {
    clearCustomAvatar()
    return
  }
  localStorage.setItem(USER_AVATAR_KEY, value)
  customAvatar.value = value
  persistCustomAvatar(value)
}

function clearCustomAvatar() {
  localStorage.removeItem(USER_AVATAR_KEY)
  customAvatar.value = ''
  clearPersistedAvatar()
}

async function refresh() {
  if (refreshing) return
  refreshing = true
  const token = localStorage.getItem('token')
  try {
    if (!token) {
      isLoggedIn.value = false
      login.value = name.value = avatar.value = email.value = ''
      authError.value = ''
      return
    }
    const res = await fetch('/api/auth/me', {
      headers: { Authorization: 'Bearer ' + token }
    })
    if (res.ok) {
      const data = await res.json()
      isLoggedIn.value = true
      login.value = data.login || data.openid || ''
      name.value = data.name || login.value
      avatar.value = data.avatar || ''
      email.value = data.email || '' // 云端脱敏邮箱；空=未绑定
      // is_vip 以服务端 JWT 为准：GitHub 登录或管理员密码登录才会是 true，游客恒为 false
      isVip.value = data.is_vip === true
      authError.value = ''
      if (data.uid) {
        // JWT 已带账号 UID：直接采用
        uid.value = data.uid
        localStorage.setItem(UID_KEY, String(data.uid))
      } else {
        // 首次登录（JWT 尚无 uid）：把游客 UID 升级为正式账号
        bindUid()
      }
      // 登录成功后缓存 member token 给本地后端，供云端记忆同步鉴权
      // （2026-08-30：之前 push 只用 guest token，UID 是游客号≠登录号→403→零上传）
      storeLoginToken()
    } else if (res.status === 401) {
      // 云端明确说 token 无效/过期：才是真的要清掉
      localStorage.removeItem('token')
      isLoggedIn.value = false
      login.value = name.value = avatar.value = email.value = ''
      isVip.value = false
      authError.value = ''
      // 后端缓存的登录 token 也过期了，一并清理（避免 memory-sync 用旧 token + 旧 uid 错配 403，2026-09-02 实锤）
      try { fetch('/api/auth/login-token', { method: 'DELETE' }) } catch {}
    } else {
      // 501/502/503/504 等：ResceneCloud 暂时连不上，不代表 token 无效。
      // 之前这里和 401 一样清 token+退回未登录态，用户会莫名其妙被"登出"且毫无提示；
      // 现在保留本地已有登录态，只把错误暴露出来供 UI 提示"云端暂时连不上，稍后重试"。
      const data = await res.json().catch(() => ({}))
      authError.value = data.error || ('验证登录状态失败（HTTP ' + res.status + '），请稍后重试')
    }
  } catch (e) {
    // fetch 级网络异常（本地后端都连不上）：同样不清 token，只报错
    authError.value = '无法连接到本地服务，请稍后重试'
  } finally {
    refreshing = false
  }
}

function logout() {
  localStorage.removeItem('token')
  isLoggedIn.value = false
  email.value = ''
  login.value = name.value = avatar.value = ''
  // 登出后清掉后端缓存的登录 token，避免重启时 restoreLoginToken 又自动恢复登录态
  try {
    fetch('/api/auth/login-token', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' }
    })
  } catch { /* 本地后端不可达：忽略 */ }
  // 登出后仍是游客身份：UID 保留（本设备账号），仅清登录态
  window.dispatchEvent(new Event('auth-change'))
}

// 首帧先读本地缓存的 UID（避免闪烁"未登录"），再向 cloud 校准
const cachedUid = localStorage.getItem(UID_KEY)
if (cachedUid) uid.value = Number(cachedUid)
// 首帧先读本地缓存的亲密度（避免闪烁 0），UID 到位后再向 cloud 校准
const cachedIntimacy = localStorage.getItem(INTIMACY_KEY)
if (cachedIntimacy) intimacy.value = Number(cachedIntimacy)
const cachedCustomAvatar = localStorage.getItem(USER_AVATAR_KEY)
if (cachedCustomAvatar) customAvatar.value = cachedCustomAvatar

// 首次加载：先从后端持久化文件恢复登录 token（WebView2 localStorage 可能因 exe
// 路径变化被清空），再验真登录态；随后分发/校准 UID。
restoreLoginToken().then(() => {
  refresh()
  fetchUid().then(() => {
    fetchIntimacy()
    pullCloudMemory()
  })
  restoreCustomAvatar()
})
if (typeof window !== 'undefined') {
  window.addEventListener('auth-change', refresh)
}

export function useAuth() {
  return {
    isLoggedIn: readonly(isLoggedIn),
    login: readonly(login),
    name: readonly(name),
    avatar: readonly(avatar),
    uid: readonly(uid),
    isVip: readonly(isVip),
        intimacy: readonly(intimacy),
        intimacyLevel: readonly(intimacyLevel),
        email: readonly(email),
        displayName: readonly(displayName),
    displayAvatar: readonly(displayAvatar),
    hasCustomAvatar: readonly(hasCustomAvatar),
    authError: readonly(authError),
    setCustomAvatar,
    clearCustomAvatar,
    refresh,
    logout,
    fetchIntimacy,
    incIntimacy
  }
}
