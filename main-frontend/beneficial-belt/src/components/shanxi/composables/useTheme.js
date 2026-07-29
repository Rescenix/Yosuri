// 全局多主题：色板（蓝/粉/紫/橙…）× 亮度（亮/暗/跟随系统）。
// 主题数据来自代码对象，applyTheme 运行时把完整变量集注入 <html> inline style，
// 不再依赖 CSS 里写死的 :root[data-theme=...] 分组——加一套主题只需往 THEME_PRESETS 加一行。
import { ref, watch } from 'vue'

const THEME_KEY = 'aurora_theme'      // 色板名：blue/pink/purple/orange
const MODE_KEY = 'aurora_mode'        // 亮度：light/dark/system

// 只保留轻量配色；页面氛围交给用户自定义动态壁纸。
export const THEME_PRESETS = {
  blue:   { label: '矢车菊',  accent: '#3b82f6', accentHover: '#2563eb', accentSoft: 'rgba(59,130,246,0.12)' },
  pink:   { label: '樱花', accent: '#ec4899', accentHover: '#db2777', accentSoft: 'rgba(236,72,153,0.12)' },
  purple: { label: '薰衣草', accent: '#a855f7', accentHover: '#9333ea', accentSoft: 'rgba(168,85,247,0.12)' },
  orange: { label: '金盏花',  accent: '#c96442', accentHover: '#b85737', accentSoft: 'rgba(201,100,66,0.12)' },
}

// 亮度选项（沿用旧 UI 语义）
export const MODE_OPTIONS = [
  { value: 'light', label: '亮色' },
  { value: 'dark', label: '暗色' },
  { value: 'system', label: '跟随系统' },
]

// 中性面两套常量（值照搬自 global.css 原 --app-* 中性部分，按亮度轴共用）
const NEUTRAL_LIGHT = {
  bg: '#ffffff', surface: '#ffffff', surface2: '#fafafa', surface3: '#f4f4f5',
  text: '#1a1a1a', textSoft: '#6b6b6b', textFaint: '#a3a3a3',
  border: '#e5e5e5', borderSoft: '#ececec', codeBg: '#f7f7f8',
  shadow: '0 24px 64px rgba(0,0,0,0.24)',
  // surfaceRgb 给"毛玻璃"用：工具栏/关闭按钮/下拉/git 栏都是 rgba(面色, 透明度)，
  // 透明度五花八门(0.5~0.9)，所以不做一堆变量，只给裸 RGB 三元组，
  // 用法 rgba(var(--app-surface-rgb), 0.78)。
  surfaceRgb: '255, 255, 255',
  // 便签是刻意的"纸"，不跟 surface 走（跟了就变成普通面板，纸感没了）。
  // 但纯白纸在暗色下极其刺眼，所以纸/墨单独一套，随亮度切换。
  stickyPaper: '#fffdf5', stickyRule: 'rgba(0,0,0,0.03)',
  stickyInk: '#4a4436', stickyInkSoft: '#5b544a', stickyInkFaint: '#a89f88',
}
const NEUTRAL_DARK = {
  bg: '#1e1e20', surface: '#26262a', surface2: '#2b2b30', surface3: '#313136',
  text: '#ececec', textSoft: '#a8a8b0', textFaint: '#76767e',
  border: '#3a3a40', borderSoft: '#34343a', codeBg: '#17171a',
  shadow: '0 24px 64px rgba(0,0,0,0.55)',
  surfaceRgb: '38, 38, 42', // = #26262a，与 surface 同色
  // 暗色下的"牛皮纸"：暖调深色，保留纸感又不刺眼
  stickyPaper: '#332f28', stickyRule: 'rgba(255,255,255,0.04)',
  stickyInk: '#e8e0cf', stickyInkSoft: '#cfc7b5', stickyInkFaint: '#8f8877',
}

const savedTheme = localStorage.getItem(THEME_KEY)
const initialTheme = THEME_PRESETS[savedTheme] ? savedTheme : 'orange'
if (savedTheme && savedTheme !== initialTheme) localStorage.setItem(THEME_KEY, initialTheme)

export const theme = ref(initialTheme)
export const mode = ref(MODE_OPTIONS.some(o => o.value === localStorage.getItem(MODE_KEY)) ? localStorage.getItem(MODE_KEY) : 'light')

function systemPrefersDark() {
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
}
function resolvedMode() {
  return mode.value === 'system' ? (systemPrefersDark() ? 'dark' : 'light') : mode.value
}

// 运行时注入：解析当前 色板×亮度 → 完整 --app-* 变量集写到 <html> inline style。
// 保留 data-theme 属性供个别依赖属性选择器的样式（如 ChatWidget context 横条）。
function applyTheme() {
  const preset = THEME_PRESETS[theme.value] || THEME_PRESETS.orange
  const dark = resolvedMode() === 'dark'
  const n = dark ? NEUTRAL_DARK : NEUTRAL_LIGHT
  const root = document.documentElement
  root.setAttribute('data-theme', dark ? 'dark' : 'light')
  // 清掉旧版本皮肤留下的属性，保证升级后立即回到普通配色。
  root.removeAttribute('data-skin')
  const vars = {
    '--app-accent': preset.accent,
    '--app-accent-hover': preset.accentHover,
    '--app-accent-soft': preset.accentSoft,
    '--app-font': "'Inter', system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif",
    '--app-bg': n.bg,
    '--app-surface': n.surface,
    '--app-surface-2': n.surface2,
    '--app-surface-3': n.surface3,
    '--app-text': n.text,
    '--app-text-soft': n.textSoft,
    '--app-text-faint': n.textFaint,
    '--app-border': n.border,
    '--app-border-soft': n.borderSoft,
    '--app-code-bg': n.codeBg,
    '--app-shadow': n.shadow,
    '--app-surface-rgb': n.surfaceRgb,
    '--sticky-paper': n.stickyPaper,
    '--sticky-rule': n.stickyRule,
    '--sticky-ink': n.stickyInk,
    '--sticky-ink-soft': n.stickyInkSoft,
    '--sticky-ink-faint': n.stickyInkFaint,
  }
  for (const [k, v] of Object.entries(vars)) root.style.setProperty(k, v)
}

let mediaListenerBound = false
export function initTheme() {
  applyTheme()
  if (!mediaListenerBound && window.matchMedia) {
    mediaListenerBound = true
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (mode.value === 'system') applyTheme()
    })
  }
}

watch(theme, (v) => { localStorage.setItem(THEME_KEY, v); applyTheme() })
watch(mode, (v) => { localStorage.setItem(MODE_KEY, v); applyTheme() })

// 当前解析后的亮度（light/dark），供组件按需读取
export function resolvedTheme() {
  return resolvedMode()
}
