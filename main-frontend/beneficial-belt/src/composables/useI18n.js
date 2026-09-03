// 轻量 i18n：不引 vue-i18n 重依赖。全局一个响应式 locale（localStorage 持久化），
// t(key) 按当前 locale 查消息表。供侧栏/顶栏/设置等核心界面用。
import { reactive, computed } from 'vue'

const LOCALE_KEY = 'ameko_locale_v1'

// 消息表：目前只维护核心界面所需 key。新增面板加进来即可，locale 结构保持一致。
// zh = 简体中文（默认），en = English
const MESSAGES = {
  zh: {
    // 侧栏 / 账户菜单
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
    'login.submit': '登录',
    'login.registerSubmit': '注册',
    'login.noAccount': '没有账号？',
    'login.registerLink': '注册一个',
    'login.hasAccount': '已有账号？',
    'login.loginLink': '去登录',
    'login.footer': '本地加密存储，不泄露第三方',
    'login.emptyInput': '请输入账号和密码',
    'login.emptyRegister': '请输入用户名和密码',
    'login.fail': '登录失败，请检查账号密码',
    'login.netError': '网络错误，请稍后再试',
    'login.localGuest': '本地访客',
    // 语言选择菜单
    'lang.zh': '简体中文',
    'lang.en': 'English',
    // 会话行
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
    'login.submit': 'Log in',
    'login.registerSubmit': 'Sign up',
    'login.noAccount': 'No account?',
    'login.registerLink': 'Sign up',
    'login.hasAccount': 'Already have an account?',
    'login.loginLink': 'Log in',
    'login.footer': 'Stored encrypted locally, never shared',
    'login.emptyInput': 'Enter account and password',
    'login.emptyRegister': 'Enter username and password',
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

export function useI18n() {
  function setLocale(locale) {
    state.locale = locale === 'en' ? 'en' : 'zh'
    localStorage.setItem(LOCALE_KEY, state.locale)
  }
  // t(key) → 当前 locale 文案；缺 key 回退中文再回退 key 本身
  function t(key) {
    const table = MESSAGES[state.locale] || MESSAGES.zh
    return table[key] !== undefined ? table[key] : (MESSAGES.zh[key] !== undefined ? MESSAGES.zh[key] : key)
  }
  const locale = computed(() => state.locale)
  const isZh = computed(() => state.locale === 'zh')
  return { t, locale, isZh, setLocale }
}
