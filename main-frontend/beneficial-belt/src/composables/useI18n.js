// 轻量 i18n：不引 vue-i18n 重依赖。全局一个响应式 locale（localStorage 持久化）。
// 约定：中文原文即 key。t('发送消息') 在 zh 下原样返回，在 en 下查英文表；
// 带变量的句子用占位符 t('共 {n} 条', { n: 5 })。新增界面文案直接写中文并包 t() 即可，
// 未登记英文的 key 自动回退中文，不会渲染成空白。
import { reactive, computed } from 'vue'
import enMessages from '../locales/en/index.js'

const LOCALE_KEY = 'ameko_locale_v1'

// 历史遗留的符号 key（早期界面在用），保留映射避免老调用点失效。
const LEGACY = {
  zh: {
    'nav.newSession': '新建会话',
    'nav.scheduledTasks': '定时任务',
    'nav.sites': '站点',
    'nav.projects': '项目',
    'nav.pinned': '置顶',
    'nav.ungrouped': '未分组',
    'account.menu.profileCard': '角色卡',
    'account.menu.login': '登录',
    'account.menu.register': '注册',
    'account.menu.language': '语言',
    'account.menu.help': '帮助',
    'account.menu.logout': '退出登录',
    'account.notLoggedIn': '未登录',
    'login.title': '登录',
    'login.registerTitle': '注册',
    'login.account': 'Rescene Cloud 账号',
    'login.username': '用户名（3-32 字符）',
    'login.password': '密码',
    'login.passwordHint': '密码（6-64 字符）',
    'login.confirmPassword': '确认密码',
    'login.confirmPasswordHint': '再输一次密码',
    'login.captcha': '验证码',
    'login.captchaRefresh': '点击刷新',
    'login.submit': '登录',
    'login.registerSubmit': '注册',
    'login.noAccount': '没有账号？',
    'login.registerLink': '注册一个',
    'login.hasAccount': '已有账号？',
    'login.loginLink': '去登录',
    'login.footer': '本地加密存储，不泄露第三方',
    'login.emptyInput': '请输入账号和密码',
    'login.emptyRegister': '请输入用户名和密码',
    'login.pwdMismatch': '两次输入的密码不一致',
    'login.captchaInput': '请输入验证码',
    'login.captchaLoadFail': '验证码加载失败，点击刷新',
    'login.fail': '登录失败，请检查账号密码',
    'login.netError': '网络错误，请稍后再试',
    'login.localGuest': '本地访客',
    'lang.zh': '简体中文',
    'lang.en': 'English',
    'session.time.today': '今天',
  },
  en: {
    'nav.newSession': 'New session',
    'nav.scheduledTasks': 'Scheduled tasks',
    'nav.sites': 'Sites',
    'nav.projects': 'Projects',
    'nav.pinned': 'Pinned',
    'nav.ungrouped': 'Ungrouped',
    'account.menu.profileCard': 'Profile card',
    'account.menu.login': 'Log in',
    'account.menu.register': 'Sign up',
    'account.menu.language': 'Language',
    'account.menu.help': 'Help',
    'account.menu.logout': 'Log out',
    'account.notLoggedIn': 'Not logged in',
    'login.title': 'Log in',
    'login.registerTitle': 'Sign up',
    'login.account': 'Rescene Cloud account',
    'login.username': 'Username (3-32 chars)',
    'login.password': 'Password',
    'login.passwordHint': 'Password (6-64 chars)',
    'login.confirmPassword': 'Confirm password',
    'login.confirmPasswordHint': 'Type password again',
    'login.captcha': 'Captcha',
    'login.captchaRefresh': 'Refresh',
    'login.submit': 'Log in',
    'login.registerSubmit': 'Sign up',
    'login.noAccount': 'No account?',
    'login.registerLink': 'Sign up',
    'login.hasAccount': 'Already have an account?',
    'login.loginLink': 'Log in',
    'login.footer': 'Stored encrypted locally, never shared',
    'login.emptyInput': 'Enter account and password',
    'login.emptyRegister': 'Enter username and password',
    'login.pwdMismatch': 'Passwords do not match',
    'login.captchaInput': 'Enter the captcha',
    'login.captchaLoadFail': 'Captcha failed to load, click to refresh',
    'login.fail': 'Login failed, check credentials',
    'login.netError': 'Network error, try again later',
    'login.localGuest': 'Guest',
    'lang.zh': '简体中文',
    'lang.en': 'English',
    'session.time.today': 'Today',
  },
}

const state = reactive({
  locale: localStorage.getItem(LOCALE_KEY) === 'en' ? 'en' : 'zh', // 默认中文
})

// 切换界面语言时广播，供需要重算文案的地方（如日期格式化）监听
export const LOCALE_EVENT = 'ameko-locale-change'

function fill(str, params) {
  if (!params) return str
  return str.replace(/\{(\w+)\}/g, (whole, name) =>
    (params[name] !== undefined && params[name] !== null ? String(params[name]) : whole))
}

// 模块级 t：不依赖 useI18n() 解构顺序，顶层 const 数组也能安全调用（避免 TDZ）。
export function tr(key, params) {
  if (typeof key !== 'string') return key
  if (LEGACY.zh[key] !== undefined) {
    return fill(state.locale === 'en' ? (LEGACY.en[key] ?? key) : LEGACY.zh[key], params)
  }
  if (state.locale === 'en') return fill(enMessages[key] ?? key, params)
  return fill(key, params)
}

export function useI18n() {
  function setLocale(locale) {
    const next = locale === 'en' ? 'en' : 'zh'
    if (next === state.locale) return
    state.locale = next
    localStorage.setItem(LOCALE_KEY, next)
    window.dispatchEvent(new CustomEvent(LOCALE_EVENT, { detail: { locale: next } }))
  }
  // t := 模块级 tr
  const t = tr
  const locale = computed(() => state.locale)
  const isZh = computed(() => state.locale === 'zh')
  return { t, locale, isZh, setLocale }
}
