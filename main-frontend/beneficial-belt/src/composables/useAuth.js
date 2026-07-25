// useAuth —— 全局登录态单例：统一验真 + 缓存 GitHub 用户名/头像 + 游客随机名。
// 取代各组件零散的 localStorage.getItem('token') 判断，杜绝“任意字符串即登录”。
// 所有组件 import 同一个模块实例，响应式 state 共享，auth-change 时自动刷新。
import { ref, computed, readonly } from 'vue'

const isLoggedIn = ref(false)
const login = ref('')   // GitHub 登录名
const name = ref('')    // GitHub 显示名
const avatar = ref('')  // GitHub 头像 URL
let refreshing = false

const GUEST_KEY = 'aurora_guest_name'

// 游客名：未登录时显示“游客#随机数字”，首次生成后存 localStorage 保持稳定（刷新不变）。
function getGuestName() {
  let g = localStorage.getItem(GUEST_KEY)
  if (!g) {
    const n = Math.floor(1000 + Math.random() * 9000) // 4 位随机数字
    g = '游客#' + n
    localStorage.setItem(GUEST_KEY, g)
  }
  return g
}

// 展示名：登录用 GitHub 名，未登录用游客名
const displayName = computed(() => {
  if (isLoggedIn.value) return name.value || login.value || 'GitHub 用户'
  return getGuestName()
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
    } else {
      // token 无效：清掉，避免伪造/过期 token 被当作已登录
      localStorage.removeItem('token')
      isLoggedIn.value = false
      login.value = name.value = avatar.value = ''
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
  window.dispatchEvent(new Event('auth-change'))
}

// 首次加载即验真；并监听登录态变化事件自动刷新
refresh()
if (typeof window !== 'undefined') {
  window.addEventListener('auth-change', refresh)
}

export function useAuth() {
  return {
    isLoggedIn: readonly(isLoggedIn),
    login: readonly(login),
    name: readonly(name),
    avatar: readonly(avatar),
    displayName: readonly(displayName),
    displayAvatar: readonly(displayAvatar),
    refresh,
    logout
  }
}
