// 衰减公式与后端 internal/memory/graph.go 的 EffectiveEnergy/EffectiveWeight 保持一致：
//   effective = base * exp(-decayRate * elapsedHours)

export function hoursSince(iso: string, now: number): number {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return 0
  const h = (now - t) / 3_600_000
  return h > 0 ? h : 0
}

export function effectiveValue(base: number, decayRate: number, lastAt: string, now: number): number {
  const elapsed = hoursSince(lastAt, now)
  return base * Math.exp(-decayRate * elapsed)
}

export function formatLastAccess(iso: string, now: number): string {
  const h = hoursSince(iso, now)
  if (h < 1) return h <= 0 ? '刚刚' : '<1h'
  return `${Math.floor(h)}h 前`
}
