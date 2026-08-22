// 全局多主题：色板（蓝/粉/紫/橙…）× 亮度（亮/暗/跟随系统）。
// 主题数据来自代码对象，applyTheme 运行时把完整变量集注入 <html> inline style，
// 不再依赖 CSS 里写死的 :root[data-theme=...] 分组——加一套主题只需往 THEME_PRESETS 加一行。
import { ref, watch } from 'vue'

const THEME_KEY = 'aurora_theme'      // 色板名：blue/pink/purple/orange
const MODE_KEY = 'aurora_mode'        // 亮度：light/dark/system

const NOVEL_SKIN_LIGHT = {
  bg: '#f5f2f3', surface: '#fffaf8', surface2: '#faf4f5', surface3: '#f2e9ed',
  text: '#46363e', textSoft: '#806d76', textFaint: '#a8989f',
  border: '#e7d9df', borderSoft: '#eee4e8', codeBg: '#f4ecef',
  shadow: '0 24px 64px rgba(86,58,72,0.16)', surfaceRgb: '255, 250, 248',
  stickyPaper: '#fffdf8', stickyRule: 'rgba(160,79,116,0.06)',
  stickyInk: '#493941', stickyInkSoft: '#75636c', stickyInkFaint: '#aa959f',
}

const NOVEL_SKIN_DARK = {
  bg: '#241a20', surface: '#30242a', surface2: '#392b32', surface3: '#45343c',
  text: '#f7e9ef', textSoft: '#d3b8c5', textFaint: '#9e7f8d',
  border: '#604453', borderSoft: '#4b3540', codeBg: '#21171c',
  shadow: '0 24px 64px rgba(14,7,11,0.55)', surfaceRgb: '48, 36, 42',
  stickyPaper: '#35282e', stickyRule: 'rgba(225,160,190,0.08)',
  stickyInk: '#f7e9ef', stickyInkSoft: '#d3b8c5', stickyInkFaint: '#9e7f8d',
}

const COMIC_SKIN_LIGHT = {
  bg: '#eef1f3', surface: '#fffefa', surface2: '#f5f4ef', surface3: '#e9eef0',
  text: '#243239', textSoft: '#5f6f76', textFaint: '#92a0a5',
  border: '#d6dfe2', borderSoft: '#e2e8ea', codeBg: '#edf1f2',
  shadow: '0 24px 64px rgba(41,61,70,0.16)', surfaceRgb: '255, 254, 250',
  stickyPaper: '#fffef8', stickyRule: 'rgba(80,116,130,0.07)',
  stickyInk: '#27363d', stickyInkSoft: '#617177', stickyInkFaint: '#96a2a6',
}

const COMIC_SKIN_DARK = {
  bg: '#172126', surface: '#202d32', surface2: '#27363c', surface3: '#304249',
  text: '#edf4f5', textSoft: '#b4c5ca', textFaint: '#7f969e',
  border: '#405860', borderSoft: '#34484f', codeBg: '#131c20',
  shadow: '0 24px 64px rgba(5,13,16,0.55)', surfaceRgb: '32, 45, 50',
  stickyPaper: '#29383d', stickyRule: 'rgba(150,190,202,0.08)',
  stickyInk: '#edf4f5', stickyInkSoft: '#b4c5ca', stickyInkFaint: '#7f969e',
}

// 普通配色只改强调色；完整皮肤会同时接管中性面、字体和氛围样式。
export const THEME_PRESETS = {
  blue:   { label: '矢车菊',  accent: '#3b82f6', accentHover: '#2563eb', accentSoft: 'rgba(59,130,246,0.12)' },
  pink:   { label: '樱花', accent: '#ec4899', accentHover: '#db2777', accentSoft: 'rgba(236,72,153,0.12)' },
  purple: { label: '薰衣草', accent: '#a855f7', accentHover: '#9333ea', accentSoft: 'rgba(168,85,247,0.12)' },
  orange: { label: '金盏花',  accent: '#c96442', accentHover: '#b85737', accentSoft: 'rgba(201,100,66,0.12)' },
  witchtrial: {
    label: '花笺物语', series: '创作主题',
    accent: '#a04f74', accentHover: '#8d4265', accentSoft: 'rgba(160,79,116,0.13)',
    fullSkin: true, skin: { light: NOVEL_SKIN_LIGHT, dark: NOVEL_SKIN_DARK },
  },
  witchtrial_hiiro: {
    label: '漫画工房', series: '创作主题',
    accent: '#587985', accentHover: '#456773', accentSoft: 'rgba(88,121,133,0.14)',
    fullSkin: true, skin: { light: COMIC_SKIN_LIGHT, dark: COMIC_SKIN_DARK },
  },
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
const savedMode = MODE_OPTIONS.some(o => o.value === localStorage.getItem(MODE_KEY)) ? localStorage.getItem(MODE_KEY) : 'light'
// 旧版完整皮肤会强制切暗色；首次升级到新皮肤时恢复亮色，让新视觉立即可见。
const shouldResetLegacySkinMode = THEME_PRESETS[initialTheme]?.fullSkin && !localStorage.getItem('aurora_skin_visual_v2')
if (shouldResetLegacySkinMode) {
  localStorage.setItem(MODE_KEY, 'light')
  localStorage.setItem('aurora_skin_visual_v2', '1')
}
export const mode = ref(shouldResetLegacySkinMode ? 'light' : savedMode)

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
  const n = preset.fullSkin ? preset.skin[dark ? 'dark' : 'light'] : (dark ? NEUTRAL_DARK : NEUTRAL_LIGHT)
  const root = document.documentElement
  root.setAttribute('data-theme', dark ? 'dark' : 'light')
  if (preset.fullSkin) root.setAttribute('data-skin', theme.value)
  else root.removeAttribute('data-skin')
  const appFont = theme.value === 'witchtrial'
    ? "Georgia, 'Noto Serif SC', 'Songti SC', 'Microsoft YaHei', serif"
    : theme.value === 'witchtrial_hiiro'
      ? "'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif"
      : "'Inter', system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif"
  const vars = {
    '--app-accent': preset.accent,
    '--app-accent-hover': preset.accentHover,
    '--app-accent-soft': preset.accentSoft,
    '--app-font': appFont,
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
