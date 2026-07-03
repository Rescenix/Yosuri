export interface ClusterDef {
  key: string
  name: string
  color: string
}

export const CLUSTERS: ClusterDef[] = [
  { key: 'CodeWork', name: 'CodeWork', color: '#0fb5a0' },
  { key: 'UserBase', name: 'UserBase', color: '#4f7ef0' },
  { key: 'ToolLog', name: 'ToolLog', color: '#6b7280' },
  { key: 'Session', name: 'Session', color: '#e8920c' },
]
export const CMAP: Record<string, ClusterDef> = {}
CLUSTERS.forEach((c) => (CMAP[c.key] = c))
export const DEFAULT_CLUSTER_COLOR = '#8a93a6'

export const VIOLET = '#6d5efc'

export const EMO: Record<string, { c: string; t: string }> = {
  neutral: { c: '#8a93a6', t: '中性' },
  happy: { c: '#d9a400', t: '积极' },
  excited: { c: '#d9a400', t: '兴奋' },
  angry: { c: '#ef4444', t: '愤怒' },
  sad: { c: '#8b5cf6', t: '低落' },
  anxious: { c: '#f97316', t: '焦虑' },
}

// 真实后端枚举（internal/memory/compiler.go）：conflict/achievement/decision/chat/compilation
// code_work 为实际数据中观察到、未在文档枚举里的值，一并兜底
export const EVT: Record<string, string> = {
  conflict: '冲突',
  achievement: '成就',
  decision: '决策',
  chat: '闲聊',
  compilation: '记忆压缩',
  code_work: '代码工作',
}

export function hex2rgb(h: string) {
  h = h.replace('#', '')
  return { r: parseInt(h.slice(0, 2), 16), g: parseInt(h.slice(2, 4), 16), b: parseInt(h.slice(4, 6), 16) }
}

export function energyFill(hex: string, e: number): string {
  const c = hex2rgb(hex)
  const k = 0.14 + Math.pow(Math.max(0, Math.min(1, e)), 0.85) * 0.82
  const m = (base: number, cc: number) => Math.round(base * (1 - k) + cc * k)
  return `rgb(${m(247, c.r)},${m(248, c.g)},${m(250, c.b)})`
}

export function mixHex(h1: string, h2: string, t: number): string {
  const a = hex2rgb(h1)
  const b = hex2rgb(h2)
  const m = (x: number, y: number) => Math.round(x * (1 - t) + y * t)
  return `rgb(${m(a.r, b.r)},${m(a.g, b.g)},${m(a.b, b.b)})`
}

export function rgba(hex: string, al: number): string {
  const c = hex2rgb(hex)
  return `rgba(${c.r},${c.g},${c.b},${al})`
}

export function ekey(a: number, b: number): string {
  return a < b ? `${a}-${b}` : `${b}-${a}`
}

export function short(s: string | undefined, n: number): string {
  s = s || ''
  return s.length > n ? s.slice(0, n) + '…' : s
}
