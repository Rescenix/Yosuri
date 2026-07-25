// 全局多主题：色板（蓝/粉/紫/橙…）× 亮度（亮/暗/跟随系统）。
// 主题数据来自代码对象，applyTheme 运行时把完整变量集注入 <html> inline style，
// 不再依赖 CSS 里写死的 :root[data-theme=...] 分组——加一套主题只需往 THEME_PRESETS 加一行。
import { ref, watch } from 'vue'

const THEME_KEY = 'aurora_theme'      // 色板名：blue/pink/purple/orange/witchtrial
const MODE_KEY = 'aurora_mode'        // 亮度：light/dark/system

// 色板预设：普通主题只需定义 accent 三件套，中性面由亮度轴提供（见下方常量）。
// 完整皮肤（fullSkin）可自带亮/暗两套中性面，实现像「魔女审判」这种整体氛围。
// 加几十套主题 = 在此追加一个对象，零 CSS 改动。
export const THEME_PRESETS = {
  blue:   { label: '矢车菊',  accent: '#3b82f6', accentHover: '#2563eb', accentSoft: 'rgba(59,130,246,0.12)' },
  pink:   { label: '樱花', accent: '#ec4899', accentHover: '#db2777', accentSoft: 'rgba(236,72,153,0.12)' },
  purple: { label: '薰衣草', accent: '#a855f7', accentHover: '#9333ea', accentSoft: 'rgba(168,85,247,0.12)' },
  orange: { label: '金盏花',  accent: '#c96442', accentHover: '#b85737', accentSoft: 'rgba(201,100,66,0.12)' },
  witchtrial: {
    label: '原初审判',
    series: '魔女审判',
    accent: '#c73e3e',
    accentHover: '#d94c4c',
    accentSoft: 'rgba(199,62,62,0.15)',
    vip: true,
    fullSkin: true,
    skin: {
      // 魔女审判只有暗色：审判厅、烛火、铁链与血迹
      dark: {
        bg: '#0c0a0d',
        surface: '#15101a',
        surface2: '#1c1522',
        surface3: '#261c2e',
        text: '#e8e0e0',
        textSoft: '#a8989e',
        textFaint: '#6b5862',
        border: '#3e2831',
        borderSoft: '#2a1b21',
        codeBg: '#110e13',
        shadow: '0 24px 64px rgba(0,0,0,0.72)',
        surfaceRgb: '21, 16, 26',
        stickyPaper: '#1c1713',
        stickyRule: 'rgba(199,62,62,0.08)',
        stickyInk: '#e8ddd0',
        stickyInkSoft: '#c9bba8',
        stickyInkFaint: '#8f7f6e',
      },
      // 亮色兜底（与暗色一致，避免逻辑分支）
      light: {
        bg: '#0c0a0d',
        surface: '#15101a',
        surface2: '#1c1522',
        surface3: '#261c2e',
        text: '#e8e0e0',
        textSoft: '#a8989e',
        textFaint: '#6b5862',
        border: '#3e2831',
        borderSoft: '#2a1b21',
        codeBg: '#110e13',
        shadow: '0 24px 64px rgba(0,0,0,0.72)',
        surfaceRgb: '21, 16, 26',
        stickyPaper: '#1c1713',
        stickyRule: 'rgba(199,62,62,0.08)',
        stickyInk: '#e8ddd0',
        stickyInkSoft: '#c9bba8',
        stickyInkFaint: '#8f7f6e',
      }
    }
  },
  witchtrial_hiiro: {
    label: '二阶堂希罗',
    series: '魔女审判',
    accent: '#e91e63',
    accentHover: '#f06292',
    accentSoft: 'rgba(233,30,99,0.18)',
    vip: true,
    fullSkin: true,
    skin: {
      // 红黑洛丽塔：黑缎带、彼岸花、蕾丝与蝴蝶结
      dark: {
        bg: '#0d0709',
        surface: '#181014',
        surface2: '#22151b',
        surface3: '#2f1b24',
        text: '#ffeef3',
        textSoft: '#f0c6d0',
        textFaint: '#b0808e',
        border: '#5a1f35',
        borderSoft: '#35141f',
        codeBg: '#150c10',
        shadow: '0 24px 64px rgba(0,0,0,0.62)',
        surfaceRgb: '24, 16, 20',
        stickyPaper: '#24181c',
        stickyRule: 'rgba(233,30,99,0.08)',
        stickyInk: '#ffeef3',
        stickyInkSoft: '#f0c6d0',
        stickyInkFaint: '#b0808e',
      },
      light: {
        bg: '#0d0709',
        surface: '#181014',
        surface2: '#22151b',
        surface3: '#2f1b24',
        text: '#ffeef3',
        textSoft: '#f0c6d0',
        textFaint: '#b0808e',
        border: '#5a1f35',
        borderSoft: '#35141f',
        codeBg: '#150c10',
        shadow: '0 24px 64px rgba(0,0,0,0.62)',
        surfaceRgb: '24, 16, 20',
        stickyPaper: '#24181c',
        stickyRule: 'rgba(233,30,99,0.08)',
        stickyInk: '#ffeef3',
        stickyInkSoft: '#f0c6d0',
        stickyInkFaint: '#b0808e',
      }
    }
  }
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

export const theme = ref(THEME_PRESETS[localStorage.getItem(THEME_KEY)] ? localStorage.getItem(THEME_KEY) : 'orange')
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
  const n = preset.fullSkin ? preset.skin[dark ? 'dark' : 'light'] : (dark ? NEUTRAL_DARK : NEUTRAL_LIGHT)
  const root = document.documentElement
  // 完整皮肤固定为暗色氛围，data-theme 仍按 resolvedMode 走保证组件选择器兼容
  root.setAttribute('data-theme', dark ? 'dark' : 'light')
  // data-skin 供皮肤专属 CSS 使用；普通配色为空字符串
  root.setAttribute('data-skin', preset.fullSkin ? theme.value : '')
  const skinFont =
    theme.value === 'witchtrial'
      ? "'ZCOOL XiaoWei', 'Noto Serif SC', 'Source Han Serif SC', 'STSong', 'SimSun', 'Cinzel', serif"
      : theme.value === 'witchtrial_hiiro'
        ? "'ZCOOL QingKe HuangYou', 'PingFang SC', 'Microsoft YaHei', 'STHeiti', cursive"
        : "'Inter', system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif"
  const vars = {
    '--app-accent': preset.accent,
    '--app-accent-hover': preset.accentHover,
    '--app-accent-soft': preset.accentSoft,
    '--app-font': skinFont,
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
