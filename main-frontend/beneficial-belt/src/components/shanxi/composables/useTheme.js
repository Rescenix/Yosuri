// 全局主题：亮 / 暗 / 跟随系统。data-theme 打在 <html> 上，CSS 变量在 global.css
// 里按 :root[data-theme='...'] 定义。切换即时生效 + localStorage 持久化。
// 统一色调：强调色收敛到品牌陶土橙 #c96442（原先蓝 #2563eb 与橙混用）。
import { ref, watch } from 'vue'

const THEME_KEY = 'aurora_theme'
export const THEME_OPTIONS = [
  { value: 'light', label: '亮色' },
  { value: 'dark', label: '暗色' },
  { value: 'system', label: '跟随系统' },
]

export const theme = ref(localStorage.getItem(THEME_KEY) || 'light')

function systemPrefersDark() {
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
}

// 把 theme 解析成实际的 light/dark 并写到 <html data-theme>
function applyTheme(val) {
  const resolved = val === 'system' ? (systemPrefersDark() ? 'dark' : 'light') : val
  document.documentElement.setAttribute('data-theme', resolved)
}

let mediaListenerBound = false
export function initTheme() {
  applyTheme(theme.value)
  if (!mediaListenerBound && window.matchMedia) {
    mediaListenerBound = true
    // 系统主题变化时，若当前是"跟随系统"则实时跟随
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (theme.value === 'system') applyTheme('system')
    })
  }
}

watch(theme, (val) => {
  localStorage.setItem(THEME_KEY, val)
  applyTheme(val)
})

// 当前解析后的主题（light/dark），供组件按需读取
export function resolvedTheme() {
  return theme.value === 'system' ? (systemPrefersDark() ? 'dark' : 'light') : theme.value
}
