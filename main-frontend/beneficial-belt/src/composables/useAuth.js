// useAuth —— 全局登录态单例：统一验真 + 缓存 GitHub 用户名/头像 + cloud 分发的 UID。
// 取代各组件零散的 localStorage.getItem('token') 判断，杜绝“任意字符串即登录”。
// 所有组件 import 同一个模块实例，响应式 state 共享，auth-change 时自动刷新。
//
// UID 账号体系：UID 由 ResceneCloud 验证并分发（前端不可伪造）。
//   - 游客：设备指纹（device_id）向 /api/auth/uid 换 UID，同一设备恒定；
//   - 登录：refresh 成功后调 /api/auth/uid/bind，把游客 UID 升级为正式账号，
//     此后换设备/清缓存重新登录即恢复同一 UID —— 用户才会珍惜账号。
import { ref, computed, readonly } from 'vue'

const isLoggedIn = ref(false)
const login = ref('')   // GitHub 登录名
const name = ref('')    // GitHub 显示名
const avatar = ref('')  // GitHub 头像 URL
const uid = ref(null)   // cloud 分发的账号 UID（游客/登录都有）
const isVip = ref(false) // 会员标识：仅由服务端 JWT 的 is_vip 决定，绝不读 localStorage（堵住游客伪造）
let refreshing = false

const DEVICE_KEY = 'aurora_device_id'
const UID_KEY = 'aurora_uid'

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

// 向 cloud 请求/取回本设备 UID：同一 device_id 恒定返回同一 UID（幂等）。
async function fetchUid() {
  try {
    const res = await fetch('/api/auth/uid', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ device_id: getDeviceId() })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.uid) {
        uid.value = data.uid
        localStorage.setItem(UID_KEY, String(data.uid))
      }
    }
  } catch {
    // cloud 不可达：保留本地已有 UID，下次启动再校准
  }
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
      }
    }
  } catch {
    // 绑定失败不阻断登录，下次启动 refresh 时重试
  }
}

// 展示名：登录用 GitHub 名，未登录用 cloud 分发的 UID
const displayName = computed(() => {
  if (isLoggedIn.value) return name.value || login.value || 'GitHub 用户'
  const u = uid.value
  return u ? 'UID ' + u : '未登录'
})

// 展示头像：登录用 GitHub 头像，未登录用 null（UI 回退到默认图标）
const displayAvatar = computed(() => {
  if (isLoggedIn.value) return avatar.value || ''
  return ''
})

async function refresh() {
  if (refreshing) return
  refreshing = true
  const token = localStorage.getItem('token')
  try {
    if (!token) {
      isLoggedIn.value = false
      login.value = name.value = avatar.value = ''
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
      // is_vip 以服务端 JWT 为准：GitHub 登录或管理员密码登录才会是 true，游客恒为 false
      isVip.value = data.is_vip === true
      if (data.uid) {
        // JWT 已带账号 UID：直接采用
        uid.value = data.uid
        localStorage.setItem(UID_KEY, String(data.uid))
      } else {
        // 首次登录（JWT 尚无 uid）：把游客 UID 升级为正式账号
        bindUid()
      }
    } else {
      // token 无效：清掉，避免伪造/过期 token 被当作已登录
      localStorage.removeItem('token')
      isLoggedIn.value = false
      login.value = name.value = avatar.value = ''
      isVip.value = false
    }
  } catch {
    localStorage.removeItem('token')
    isLoggedIn.value = false
    login.value = name.value = avatar.value = ''
  } finally {
    refreshing = false
  }
}

function logout() {
  localStorage.removeItem('token')
  isLoggedIn.value = false
  login.value = name.value = avatar.value = ''
  // 登出后仍是游客身份：UID 保留（本设备账号），仅清登录态
  window.dispatchEvent(new Event('auth-change'))
}

// 首帧先读本地缓存的 UID（避免闪烁"未登录"），再向 cloud 校准
const cachedUid = localStorage.getItem(UID_KEY)
if (cachedUid) uid.value = Number(cachedUid)

// 首次加载即验真；并监听登录态变化事件自动刷新
refresh()
fetchUid()
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
    displayName: readonly(displayName),
    displayAvatar: readonly(displayAvatar),
    refresh,
    logout
  }
}
