// 全局多主题：色板（蓝/粉/紫/橙…）× 亮度（亮/暗/跟随系统）。
// 主题数据来自代码对象，applyTheme 运行时把完整变量集注入 <html> inline style，
// 不再依赖 CSS 里写死的 :root[data-theme=...] 分组——加一套主题只需往 THEME_PRESETS 加一行。
import { ref, watch } from 'vue'

const THEME_KEY = 'aurora_theme'      // 色板名：blue/pink/purple/orange/custom
const MODE_KEY = 'aurora_mode'        // 亮度：light/dark/system
const CUSTOM_COLOR_KEY = 'aurora_theme_custom_color' // 自定义主题色 hex
const CUSTOM_NAME_KEY = 'aurora_theme_custom_name'   // 自定义主题色语义名（auto 免费模型起名）

// 两套完整皮肤都以专业 IDE 为骨架。二次元感只来自赛璐璐式双强调色，
// 不再用波点、蕾丝、发光和大圆角破坏教学录屏里的代码可读性。
const KEYFRAME_SKIN_LIGHT = {
  bg: '#e9ecf2', surface: '#fbfaf7', surface2: '#faf4f5', surface3: '#f2e9ed',
  text: '#242938', textSoft: '#626878', textFaint: '#9097a6',
  border: '#cbd0db', borderSoft: '#dfe2e8', codeBg: '#f0f1f4',
  shadow: '0 18px 52px rgba(32,38,55,0.20)', surfaceRgb: '251, 250, 247',
  stickyPaper: '#fffdf5', stickyRule: 'rgba(64,86,161,0.07)',
  stickyInk: '#303544', stickyInkSoft: '#656b79', stickyInkFaint: '#949aa7',
}

const KEYFRAME_SKIN_DARK = {
  bg: '#121722', surface: '#1a202c', surface2: '#202735', surface3: '#293140',
  text: '#eef1f7', textSoft: '#b3bac8', textFaint: '#7f8899',
  border: '#374051', borderSoft: '#2c3443', codeBg: '#111620',
  shadow: '0 18px 52px rgba(3,7,13,0.58)', surfaceRgb: '26, 32, 44',
  stickyPaper: '#2b2a2a', stickyRule: 'rgba(127,152,255,0.08)',
  stickyInk: '#f1eee7', stickyInkSoft: '#c5c2bb', stickyInkFaint: '#8f8d88',
}

const NIGHT_SKIN_LIGHT = {
  bg: '#e8efef', surface: '#f9fcfb', surface2: '#f5f4ef', surface3: '#e9eef0',
  text: '#1f2b33', textSoft: '#5a6c73', textFaint: '#89999d',
  border: '#c4d2d2', borderSoft: '#d9e3e2', codeBg: '#edf3f2',
  shadow: '0 18px 52px rgba(24,48,52,0.20)', surfaceRgb: '249, 252, 251',
  stickyPaper: '#f7fbf7', stickyRule: 'rgba(22,139,143,0.07)',
  stickyInk: '#26353a', stickyInkSoft: '#617176', stickyInkFaint: '#8e9b9e',
}

const NIGHT_SKIN_DARK = {
  bg: '#0d181d', surface: '#142329', surface2: '#192b32', surface3: '#21373f',
  text: '#eaf4f5', textSoft: '#adc0c3', textFaint: '#768d92',
  border: '#315058', borderSoft: '#263f47', codeBg: '#0c1519',
  shadow: '0 18px 52px rgba(2,9,12,0.62)', surfaceRgb: '20, 35, 41',
  stickyPaper: '#233135', stickyRule: 'rgba(66,199,199,0.08)',
  stickyInk: '#e9f2ef', stickyInkSoft: '#b9c8c5', stickyInkFaint: '#81918e',
}

// 普通配色只改强调色；完整皮肤会同时接管中性面、字体和氛围样式。
export const THEME_PRESETS = {
  blue:   { label: '矢车菊',  accent: '#3b82f6', accentHover: '#2563eb', accentSoft: 'rgba(59,130,246,0.12)' },
  pink:   { label: '樱花', accent: '#ec4899', accentHover: '#db2777', accentSoft: 'rgba(236,72,153,0.12)' },
  purple: { label: '薰衣草', accent: '#a855f7', accentHover: '#9333ea', accentSoft: 'rgba(168,85,247,0.12)' },
  orange: { label: '金盏花',  accent: '#c96442', accentHover: '#b85737', accentSoft: 'rgba(201,100,66,0.12)' },
  // 双色渐变主题：gradFrom/gradTo 生成 --app-accent-gradient（主按钮/明显强调用渐变，
  // 普通强调仍回落到 accent 单色，保证浅色易读）
  aurora: {
    label: '极光', gradient: true, gradFrom: '#6366f1', gradTo: '#ec4899',
    accent: '#7c6cf0', accentHover: '#6a5ae0', accentSoft: 'rgba(124,108,240,0.13)',
  },
  witchtrial: {
    label: '原画工房', series: '动画工作台',
    accent: '#df5656', accentHover: '#c84549', accentSoft: 'rgba(223,86,86,0.13)',
    accent2: '#4056a1', accent2Soft: 'rgba(64,86,161,0.12)',
    fullSkin: true, skin: { light: KEYFRAME_SKIN_LIGHT, dark: KEYFRAME_SKIN_DARK },
  },
  witchtrial_hiiro: {
    label: '夜幕放映', series: '动画工作台',
    accent: '#168b8f', accentHover: '#0e7378', accentSoft: 'rgba(22,139,143,0.14)',
    accent2: '#674ea7', accent2Soft: 'rgba(103,78,167,0.13)',
    fullSkin: true, skin: { light: NIGHT_SKIN_LIGHT, dark: NIGHT_SKIN_DARK },
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
const initialTheme = (THEME_PRESETS[savedTheme] || savedTheme === 'custom') ? savedTheme : 'orange'
if (savedTheme && savedTheme !== initialTheme) localStorage.setItem(THEME_KEY, initialTheme)

export const theme = ref(initialTheme)
// 自定义主题色（custom 色板用）：hex 存 localStorage，起名结果也持久化
export const customColor = ref(/^#[0-9a-fA-F]{6}$/.test(localStorage.getItem(CUSTOM_COLOR_KEY) || '') ? localStorage.getItem(CUSTOM_COLOR_KEY) : '#c96442')
export const customThemeName = ref(localStorage.getItem(CUSTOM_NAME_KEY) || '自定义')
const savedMode = MODE_OPTIONS.some(o => o.value === localStorage.getItem(MODE_KEY)) ? localStorage.getItem(MODE_KEY) : 'light'
export const mode = ref(savedMode)

// hex → {r,g,b}，非法值回退橙色
function hexToRgb(hex) {
  const m = /^#?([0-9a-fA-F]{2})([0-9a-fA-F]{2})([0-9a-fA-F]{2})$/.exec(hex || '')
  if (!m) return { r: 201, g: 100, b: 66 }
  return { r: parseInt(m[1], 16), g: parseInt(m[2], 16), b: parseInt(m[3], 16) }
}
// hover 变暗 12%（同现有 preset 的 accentHover 相对 accent 的比例）
function hoverOf(hex) {
  const { r, g, b } = hexToRgb(hex)
  return `rgb(${Math.round(r * 0.88)}, ${Math.round(g * 0.88)}, ${Math.round(b * 0.88)})`
}
// soft 半透明底（12% 浓度，同现有 preset 的 accentSoft）
function softOf(hex) {
  const { r, g, b } = hexToRgb(hex)
  return `rgba(${r}, ${g}, ${b}, 0.12)`
}
export function setCustomColor(hex) {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex || '')) return false
  customColor.value = hex
  localStorage.setItem(CUSTOM_COLOR_KEY, hex)
  return true
}
export function setCustomThemeName(name) {
  const n = String(name || '').trim().slice(0, 12)
  if (!n) return
  customThemeName.value = n
  localStorage.setItem(CUSTOM_NAME_KEY, n)
}

function systemPrefersDark() {
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
}
function resolvedMode() {
  return mode.value === 'system' ? (systemPrefersDark() ? 'dark' : 'light') : mode.value
}

// 运行时注入：解析当前 色板×亮度 → 完整 --app-* 变量集写到 <html> inline style。
// 保留 data-theme 属性供个别依赖属性选择器的样式（如 ChatWidget context 横条）。
function applyTheme() {
  const preset = theme.value === 'custom'
    ? { accent: customColor.value, accentHover: hoverOf(customColor.value), accentSoft: softOf(customColor.value) }
    : (THEME_PRESETS[theme.value] || THEME_PRESETS.orange)
  const dark = resolvedMode() === 'dark'
  const n = preset.fullSkin ? preset.skin[dark ? 'dark' : 'light'] : (dark ? NEUTRAL_DARK : NEUTRAL_LIGHT)
  const root = document.documentElement
  root.setAttribute('data-theme', dark ? 'dark' : 'light')
  if (preset.fullSkin) root.setAttribute('data-skin', theme.value)
  else root.removeAttribute('data-skin')
  const appFont = preset.fullSkin
    ? "'Segoe UI Variable', 'Inter', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif"
    : "'Inter', system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif"
  const vars = {
      '--app-accent': preset.accent,
      '--app-accent-hover': preset.accentHover,
      '--app-accent-soft': preset.accentSoft,
      '--app-accent-2': preset.accent2 || preset.accentHover,
      '--app-accent-2-soft': preset.accent2Soft || preset.accentSoft,
      // 双色渐变：gradFrom/gradTo 拼 135° 渐变；非渐变主题/自定义色回退单色
      '--app-accent-gradient': preset.gradient && preset.gradFrom && preset.gradTo
        ? `linear-gradient(135deg, ${preset.gradFrom}, ${preset.gradTo})`
        : preset.accent,
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
