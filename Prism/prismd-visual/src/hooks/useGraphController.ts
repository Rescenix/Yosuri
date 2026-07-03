import { useCallback, useEffect, useRef, useState } from 'react'
import * as echarts from 'echarts'
import type { NeuronData, GraphEdge } from '../types'
import { CMAP, DEFAULT_CLUSTER_COLOR, VIOLET, energyFill, mixHex, rgba, ekey, short } from '../lib/colors'
import { effectiveValue } from '../lib/decay'
import { queryPrimQL } from './usePrismData'

export interface MenuState {
  show: boolean
  x: number
  y: number
  id: number | null
}
export interface PhysicsState {
  repulsion: number
  edgeLength: number
  gravity: number
}
interface AdjEntry {
  id: number
  weight: number
  kind: number
}
interface TraceHop {
  from: number
  to: number
  level: number
}
interface TraceState {
  start: number
  byLevel: Record<number, TraceHop[]>
  maxLevel: number
  activated: Set<number>
  activeEdges: Set<string>
}

const DEFAULT_PHYSICS: PhysicsState = { repulsion: 340, edgeLength: 130, gravity: 0.08 }
const HOP_STEP_MS = 620
const FREEZE_DELAY_MS = 1600
// 层级导航：默认只显示每簇能量/度数最高的一小撮"枢纽"节点，其余留在簇内待展开——
// 而不是把几百个节点一次性摊平堆在画布上。
const HUB_PER_CLUSTER = 8
const EXPAND_ON_CLICK = 6
const CLUSTER_EXPAND_CAP = 30
// 4 个不可见的固定锚点，撑大力导向坐标系的可交互（含拖拽平移）范围——
// ECharts graph 的 roam 命中区域是按数据点的包围盒算的，节点全挤在中间时，
// 包围盒以外的空白画布是拖不动的。锚点半径要远大于枢纽子图的自然展开半径。
const ANCHOR_RADIUS = 900

function eff(n: NeuronData): number {
  return effectiveValue(n.energy, n.decayRate, n.lastAccessAt, Date.now())
}

function scoreNode(n: NeuronData, adj: Map<number, AdjEntry[]>): number {
  const d = adj.get(n.id)?.length || 0
  return eff(n) * (1 + d * 0.12)
}

function computeDefaultActiveIds(ns: NeuronData[], adj: Map<number, AdjEntry[]>): Set<number> {
  const byCluster = new Map<string, NeuronData[]>()
  ns.forEach((n) => {
    const arr = byCluster.get(n.cluster) || []
    arr.push(n)
    byCluster.set(n.cluster, arr)
  })
  const ids = new Set<number>()
  byCluster.forEach((arr) => {
    const sorted = [...arr].sort((a, b) => scoreNode(b, adj) - scoreNode(a, adj))
    sorted.slice(0, HUB_PER_CLUSTER).forEach((n) => ids.add(n.id))
  })
  return ids
}

function addAnchors(nodes: any[]): any[] {
  const r = ANCHOR_RADIUS
  const mk = (id: string, x: number, y: number) => ({
    id,
    name: '',
    symbolSize: 0.01,
    itemStyle: { opacity: 0 },
    silent: true,
    label: { show: false },
    tooltip: { show: false },
    fixed: true,
    x,
    y,
  })
  return [...nodes, mk('__a0', -r, -r), mk('__a1', r, -r), mk('__a2', r, r), mk('__a3', -r, r)]
}

export function useGraphController(neurons: NeuronData[], edges: GraphEdge[]) {
  const chartRef = useRef<HTMLDivElement>(null)

  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [menu, setMenu] = useState<MenuState>({ show: false, x: 0, y: 0, id: null })
  const [tracing, setTracing] = useState(false)
  const [traceHopState, setTraceHopState] = useState(0)
  const [traceStart, setTraceStart] = useState<number | null>(null)
  const [traceMaxHop, setTraceMaxHop] = useState(0)
  const [tracePlaying, setTracePlaying] = useState(false)
  const [activeCluster, setActiveCluster] = useState<string | null>(null)
  const [physics, setPhysics] = useState<PhysicsState>(DEFAULT_PHYSICS)
  const [consoleOpen, setConsoleOpen] = useState(false)
  const [primqlInput, setPrimqlInput] = useState('')
  const [primqlResult, setPrimqlResult] = useState(
    'PrismD PrimQL 控制台  ·  输入 HELP 查看原语\n直连 PrismD :5666 · TRACE/CLUSTER/RESET 为本地视图快捷指令'
  )
  const [copied, setCopied] = useState(false)
  const [visibleCount, setVisibleCount] = useState(0)

  // ── 纯运行时可变状态：不经过 React 渲染，直接驱动 ECharts ──
  const R = useRef({
    chart: null as any,
    nodeById: new Map<number, NeuronData>(),
    idxById: new Map<number, number>(),
    adj: new Map<number, AdjEntry[]>(),
    hubs: new Set<number>(),
    activeIds: new Set<number>(),
    traceAddedIds: [] as number[],
    optionNodes: [] as any[],
    optionLinks: [] as any[],
    neurons: [] as NeuronData[],
    edges: [] as GraphEdge[],
    trace: null as TraceState | null,
    zoom: 1,
    labelsAll: false,
    hovering: false,
    interacting: false,
    pulseT: 0,
    pulseTimer: 0 as any,
    traceTimer: 0 as any,
    freezeTimer: 0 as any,
    copyTimer: 0 as any,
    activeCluster: null as string | null,
    selectedId: null as number | null,
    tracing: false,
    tracePlaying: false,
    traceHop: 0,
  }).current

  R.activeCluster = activeCluster
  R.selectedId = selectedId
  R.tracing = tracing
  R.tracePlaying = tracePlaying
  R.traceHop = traceHopState

  // ── 全图索引：始终覆盖完整数据集（供邻居展开/追踪扩散/详情卡使用）──
  const indexData = useCallback((ns: NeuronData[], es: GraphEdge[]) => {
    R.nodeById = new Map(ns.map((n) => [n.id, n]))
    // 后端可能对同一对节点存在多条 synapse（例如两次不同的压缩事件各自建了一条边），
    // 按邻居 id 去重、保留权重更高的那条，避免同一邻居在列表/展开逻辑里重复出现。
    const adjMap = new Map<number, Map<number, AdjEntry>>()
    ns.forEach((n) => adjMap.set(n.id, new Map()))
    const upsert = (from: number, to: number, weight: number, kind: number) => {
      const m = adjMap.get(from)
      if (!m) return
      const existing = m.get(to)
      if (!existing || weight > existing.weight) m.set(to, { id: to, weight, kind })
    }
    es.forEach((e) => {
      upsert(e.from, e.to, e.weight, e.kind)
      upsert(e.to, e.from, e.weight, e.kind)
    })
    const adj = new Map<number, AdjEntry[]>()
    adjMap.forEach((m, id) => adj.set(id, [...m.values()]))
    R.adj = adj
  }, [])

  const baseNodeStyle = useCallback((n: NeuronData) => {
    const c = CMAP[n.cluster]?.color || DEFAULT_CLUSTER_COLOR
    const e = eff(n)
    const dormant = e < 0.24
    const isHub = R.hubs.has(n.id)
    const dimmed = R.activeCluster && n.cluster !== R.activeCluster
    const border = dormant ? mixHex(c, '#aab0bd', 0.55) : c
    return {
      color: energyFill(c, e),
      borderColor: border,
      borderWidth: isHub ? 2.4 : dormant ? 1.2 : 1.8,
      borderType: dormant ? 'dashed' : 'solid',
      shadowBlur: e >= 0.72 ? 10 : 0,
      shadowColor: rgba(c, 0.5),
      opacity: dimmed ? 0.09 : 1,
    }
  }, [])
  const nodeSize = useCallback((n: NeuronData) => {
    const isHub = R.hubs.has(n.id)
    const e = eff(n)
    return Math.max(isHub ? 30 : 10, Math.min(42, 11 + e * 26))
  }, [])

  const buildOptionNodes = useCallback(
    (ns: NeuronData[]) =>
      ns.map((n) => ({
        id: String(n.id),
        name: String(n.id),
        value: eff(n),
        symbol: 'circle',
        symbolSize: nodeSize(n),
        itemStyle: baseNodeStyle(n),
        _label: short(n.content, 11),
        label: { show: R.hubs.has(n.id) },
      })),
    [baseNodeStyle, nodeSize]
  )

  const buildOptionLinks = useCallback((es: GraphEdge[], byId: Map<number, NeuronData>) => {
    const dim = R.activeCluster
    return es.map((e) => {
      const fn = byId.get(e.from)
      const tn = byId.get(e.to)
      const off = !!dim && (fn?.cluster !== dim || tn?.cluster !== dim)
      return {
        source: String(e.from),
        target: String(e.to),
        value: e.weight,
        lineStyle: { color: '#8f97a8', width: 0.6 + e.weight * 2.4, opacity: off ? 0.03 : 0.1 + e.weight * 0.34, curveness: 0 },
      }
    })
  }, [])

  // ── 可见子图索引：只覆盖当前展开的 activeIds ──
  const rebuildActiveIndex = useCallback((activeNeurons: NeuronData[]) => {
    R.idxById = new Map(activeNeurons.map((n, i) => [n.id, i]))
    const scored = [...activeNeurons].sort((a, b) => scoreNode(b, R.adj) - scoreNode(a, R.adj))
    R.hubs = new Set(scored.slice(0, 5).map((n) => n.id))
  }, [])

  const rebuildOptionArrays = useCallback(() => {
    const activeNeurons = R.neurons.filter((n) => R.activeIds.has(n.id))
    rebuildActiveIndex(activeNeurons)
    const activeEdges = R.edges.filter((e) => R.activeIds.has(e.from) && R.activeIds.has(e.to))
    R.optionNodes = addAnchors(buildOptionNodes(activeNeurons))
    R.optionLinks = buildOptionLinks(activeEdges, R.nodeById)
  }, [rebuildActiveIndex, buildOptionNodes, buildOptionLinks])

  const updateStyles = useCallback(() => {
    if (R.chart) R.chart.setOption({ series: [{ id: 'field', data: R.optionNodes, links: R.optionLinks }] })
  }, [])

  const applyBaseStyles = useCallback(() => {
    R.neurons.forEach((n) => {
      const idx = R.idxById.get(n.id)
      if (idx == null) return
      const it = R.optionNodes[idx]
      it.itemStyle = baseNodeStyle(n)
      it.label = { show: R.hubs.has(n.id) }
    })
    const dim = R.activeCluster
    R.optionLinks.forEach((link: any, i: number) => {
      const e = R.edges[i]
      if (!e) return
      const fn = R.nodeById.get(e.from)
      const tn = R.nodeById.get(e.to)
      const off = !!dim && (fn?.cluster !== dim || tn?.cluster !== dim)
      link.lineStyle = { color: '#8f97a8', width: 0.6 + e.weight * 2.4, opacity: off ? 0.03 : 0.1 + e.weight * 0.34, curveness: 0 }
    })
    updateStyles()
  }, [baseNodeStyle, updateStyles])

  const getPos = useCallback((idx: number): [number, number] | null => {
    try {
      const d = R.chart.getModel().getSeriesByIndex(0).getData()
      const l = d.getItemLayout(idx)
      if (l) return (Array.isArray(l) ? [l[0], l[1]] : [l.x, l.y]) as [number, number]
    } catch {
      /* noop */
    }
    return null
  }, [])

  const autoCenterOn = useCallback(
    (id: number) => {
      if (!R.chart) return
      const idx = R.idxById.get(id)
      if (idx == null) return
      const it = R.optionNodes[idx]
      const pos = it.x != null && it.y != null ? [it.x, it.y] : getPos(idx)
      if (!pos) return
      R.chart.setOption({ series: [{ id: 'field', center: pos, zoom: R.zoom || 1 }] })
    },
    [getPos]
  )

  // 力导向初始随机起点 + 较弱的引力(gravity)在 1.6s 内未必能把节点收敛到 (0,0) 附近，
  // 冻结后必须按实际内容的包围盒中心重新定位视口，不能假设内容总是落在原点。
  const fitViewToActive = useCallback(() => {
    if (!R.chart) return
    let minX = Infinity,
      maxX = -Infinity,
      minY = Infinity,
      maxY = -Infinity
    R.optionNodes.forEach((it: any) => {
      if (it.fixed || it.x == null || it.y == null) return
      if (it.x < minX) minX = it.x
      if (it.x > maxX) maxX = it.x
      if (it.y < minY) minY = it.y
      if (it.y > maxY) maxY = it.y
    })
    if (minX > maxX) return
    R.chart.setOption({ series: [{ id: 'field', center: [(minX + maxX) / 2, (minY + maxY) / 2] }] })
  }, [])

  const freezeLayout = useCallback(() => {
    if (!R.chart) return
    try {
      const data = R.chart.getModel().getSeriesByIndex(0).getData()
      R.neurons.forEach((n) => {
        const idx = R.idxById.get(n.id)
        if (idx == null) return
        const it = R.optionNodes[idx]
        const l = data.getItemLayout(idx)
        if (l) {
          const x = Array.isArray(l) ? l[0] : l.x
          const y = Array.isArray(l) ? l[1] : l.y
          if (x != null && y != null) {
            it.x = x
            it.y = y
          }
        }
      })
    } catch {
      /* noop */
    }
    R.chart.setOption({ series: [{ id: 'field', layout: 'none', data: R.optionNodes }] })
    if (R.selectedId != null) autoCenterOn(R.selectedId)
    else fitViewToActive()
  }, [autoCenterOn, fitViewToActive])

  const scheduleFreeze = useCallback(
    (delay?: number) => {
      clearTimeout(R.freezeTimer)
      R.freezeTimer = setTimeout(freezeLayout, delay || FREEZE_DELAY_MS)
    },
    [freezeLayout]
  )

  const wakeForRelayout = useCallback(() => {
    R.optionNodes.forEach((it: any) => {
      if (!it.fixed) {
        delete it.x
        delete it.y
      }
    })
  }, [])

  // ── activeIds 变化后的公共收尾：重建可见子图 + 视图计数 + （如已建图）重新力导向布局 ──
  const applyActiveChange = useCallback(() => {
    setVisibleCount(R.activeIds.size)
    rebuildOptionArrays()
    if (R.chart) {
      wakeForRelayout()
      R.chart.setOption({ series: [{ id: 'field', layout: 'force', data: R.optionNodes, links: R.optionLinks }] })
      scheduleFreeze()
    }
  }, [rebuildOptionArrays, wakeForRelayout, scheduleFreeze])

  // ── 展开：把给定 id 们并入可见子图，返回本次新加入的 id（缺的邻居才会触发重新力导向布局）──
  const ensureActive = useCallback(
    (ids: number[]): number[] => {
      const added: number[] = []
      ids.forEach((id) => {
        if (!R.activeIds.has(id)) {
          R.activeIds.add(id)
          added.push(id)
        }
      })
      if (added.length) applyActiveChange()
      return added
    },
    [applyActiveChange]
  )

  const pulse = useCallback(() => {
    if (!R.chart) return
    if (R.hovering || R.interacting || R.tracing || R.selectedId != null) return
    R.pulseT += 0.16
    const amp = (Math.sin(R.pulseT) + 1) / 2
    let changed = false
    R.neurons.forEach((n) => {
      const idx = R.idxById.get(n.id)
      if (idx == null) return
      if (eff(n) >= 0.72) {
        R.optionNodes[idx].itemStyle.shadowBlur = 7 + amp * 20
        changed = true
      }
    })
    if (changed) updateStyles()
  }, [updateStyles])

  const onRoam = useCallback(() => {
    if (!R.chart) return
    const opt = R.chart.getOption()
    const z = (opt.series && opt.series[0] && opt.series[0].zoom) || 1
    R.zoom = z
    const want = z > 1.7
    if (want !== R.labelsAll) {
      R.labelsAll = want
      R.chart.setOption({ series: [{ id: 'field', label: { show: want } }] })
    }
  }, [])

  const closeMenu = useCallback(() => setMenu((m) => ({ ...m, show: false })), [])

  const selectNode = useCallback(
    (id: number) => {
      const n = R.nodeById.get(id)
      if (!n) return
      const neighborIds = [...(R.adj.get(id) || [])]
        .sort((a, b) => b.weight - a.weight)
        .slice(0, EXPAND_ON_CLICK)
        .map((nb) => nb.id)
      ensureActive([id, ...neighborIds])

      const cur = R.activeCluster
      const nextCluster = cur && n.cluster !== cur ? n.cluster : cur
      const clusterChanged = nextCluster !== cur
      setSelectedId(id)
      setMenu((m) => ({ ...m, show: false }))
      if (clusterChanged) {
        setActiveCluster(nextCluster)
        R.activeCluster = nextCluster
        applyBaseStyles()
      }
      const idx = R.idxById.get(id)
      if (R.chart && idx != null) {
        R.chart.dispatchAction({ type: 'downplay', seriesIndex: 0 })
        R.chart.dispatchAction({ type: 'highlight', seriesIndex: 0, dataIndex: idx })
      }
      autoCenterOn(id)
    },
    [ensureActive, applyBaseStyles, autoCenterOn]
  )

  const closeCard = useCallback(() => {
    setSelectedId(null)
    R.chart?.dispatchAction({ type: 'downplay', seriesIndex: 0 })
  }, [])

  const stopTrace = useCallback(() => {
    R.trace = null
    clearTimeout(R.traceTimer)
    setTracing(false)
    setTraceHopState(0)
    setTracePlaying(false)
    // 追踪扩散只是临时展开去看传播路径的，追踪一结束就把专门为这次追踪拉进来的节点收回雾里，
    // 不然一条 4 跳的追踪很容易把小世界视图重新摊平回几百个节点。
    if (R.traceAddedIds.length) {
      R.traceAddedIds.forEach((id) => R.activeIds.delete(id))
      R.traceAddedIds = []
      applyActiveChange()
    } else {
      applyBaseStyles()
    }
  }, [applyBaseStyles, applyActiveChange])

  const onBlank = useCallback(() => {
    if (R.tracing) {
      stopTrace()
      return
    }
    setMenu((m) => (m.show ? { ...m, show: false } : m))
    if (R.selectedId != null) closeCard()
  }, [stopTrace, closeCard])

  const openMenu = useCallback((id: number, x: number, y: number) => setMenu({ show: true, x, y, id }), [])

  const applyTraceStyles = useCallback(() => {
    const trace = R.trace
    if (!trace) return
    const act = trace.activated
    const ae = trace.activeEdges
    R.neurons.forEach((n) => {
      const idx = R.idxById.get(n.id)
      if (idx == null) return
      const it = R.optionNodes[idx]
      const on = act.has(n.id)
      if (on) {
        const c = CMAP[n.cluster]?.color || DEFAULT_CLUSTER_COLOR
        it.itemStyle = {
          color: energyFill(c, Math.max(0.62, eff(n))),
          borderColor: VIOLET,
          borderWidth: 2.6,
          borderType: 'solid',
          shadowBlur: 22,
          shadowColor: rgba(VIOLET, 0.55),
          opacity: 1,
        }
        it.label = { show: n.id === trace.start || R.hubs.has(n.id) }
      } else {
        it.itemStyle = { ...baseNodeStyle(n), opacity: 0.1, shadowBlur: 0 }
        it.label = { show: false }
      }
    })
    R.optionLinks.forEach((link: any, i: number) => {
      const e = R.edges[i]
      if (!e) return
      const on = ae.has(ekey(e.from, e.to))
      link.lineStyle = on
        ? { color: VIOLET, width: 2.6, opacity: 0.95, curveness: 0, shadowBlur: 6, shadowColor: rgba(VIOLET, 0.5) }
        : { color: '#8f97a8', width: 0.6 + e.weight * 2.4, opacity: 0.03, curveness: 0 }
    })
    updateStyles()
  }, [baseNodeStyle, updateStyles])

  const setTraceHop = useCallback(
    (hop: number) => {
      const trace = R.trace
      if (!trace) return
      hop = Math.max(0, Math.min(trace.maxLevel, hop))
      const activated = new Set<number>([trace.start])
      const activeEdges = new Set<string>()
      for (let l = 1; l <= hop; l++) {
        ;(trace.byLevel[l] || []).forEach((e) => {
          activated.add(e.to)
          activeEdges.add(ekey(e.from, e.to))
        })
      }
      trace.activated = activated
      trace.activeEdges = activeEdges
      setTraceHopState(hop)
      R.traceHop = hop
      applyTraceStyles()
    },
    [applyTraceStyles]
  )

  const autoplayStep = useCallback(() => {
    const trace = R.trace
    if (!trace || !R.tracePlaying) return
    const next = R.traceHop + 1
    if (next > trace.maxLevel) {
      setTracePlaying(false)
      return
    }
    setTraceHop(next)
    if (next < trace.maxLevel) R.traceTimer = setTimeout(autoplayStep, HOP_STEP_MS)
    else setTracePlaying(false)
  }, [setTraceHop])

  const traceDiffusion = useCallback(
    (idArg?: number) => {
      const id = idArg ?? R.selectedId
      if (id == null || !R.nodeById.has(id)) return
      closeMenu()
      R.chart?.dispatchAction({ type: 'downplay', seriesIndex: 0 })
      const level = new Map<number, number>([[id, 0]])
      const q = [id]
      const tree: TraceHop[] = []
      const maxHop = 4
      while (q.length) {
        const cur = q.shift()!
        const L = level.get(cur)!
        if (L >= maxHop) continue
        const nb = [...(R.adj.get(cur) || [])].sort((a, b) => b.weight - a.weight)
        for (const { id: nid } of nb) {
          if (!level.has(nid)) {
            level.set(nid, L + 1)
            tree.push({ from: cur, to: nid, level: L + 1 })
            q.push(nid)
          }
        }
      }
      const maxLevel = Math.max(0, ...Array.from(level.values()))
      const byLevel: Record<number, TraceHop[]> = {}
      tree.forEach((e) => {
        ;(byLevel[e.level] = byLevel[e.level] || []).push(e)
      })
      // 追踪覆盖到的节点可能还在雾里（不在当前可见子图内），先展开进来才有 idx 可供高亮；
      // 记下这次是为追踪新展开的哪些 id，追踪结束时只收回这些，不动用户手动展开过的部分。
      R.traceAddedIds = ensureActive(Array.from(level.keys()))
      R.trace = { start: id, byLevel, maxLevel, activated: new Set([id]), activeEdges: new Set() }
      setTracing(true)
      setTraceHopState(0)
      setTraceStart(id)
      setTraceMaxHop(maxLevel)
      setTracePlaying(true)
      R.tracing = true
      R.tracePlaying = true
      R.traceHop = 0
      applyTraceStyles()
      clearTimeout(R.traceTimer)
      R.traceTimer = setTimeout(autoplayStep, HOP_STEP_MS)
    },
    [closeMenu, ensureActive, applyTraceStyles, autoplayStep]
  )

  const onTraceNext = useCallback(() => {
    clearTimeout(R.traceTimer)
    setTracePlaying(false)
    R.tracePlaying = false
    setTraceHop(R.traceHop + 1)
  }, [setTraceHop])
  const onTracePrev = useCallback(() => {
    clearTimeout(R.traceTimer)
    setTracePlaying(false)
    R.tracePlaying = false
    setTraceHop(R.traceHop - 1)
  }, [setTraceHop])
  const onTracePlayToggle = useCallback(() => {
    if (!R.trace) return
    if (R.tracePlaying) {
      clearTimeout(R.traceTimer)
      setTracePlaying(false)
      R.tracePlaying = false
    } else {
      const atEnd = R.traceHop >= R.trace.maxLevel
      if (atEnd) setTraceHop(0)
      setTracePlaying(true)
      R.tracePlaying = true
      R.traceTimer = setTimeout(autoplayStep, HOP_STEP_MS)
    }
  }, [setTraceHop, autoplayStep])

  const toggleCluster = useCallback(
    (key: string) => {
      const cur = R.activeCluster === key ? null : key
      if (cur) {
        const clusterIds = R.neurons
          .filter((n) => n.cluster === key)
          .sort((a, b) => scoreNode(b, R.adj) - scoreNode(a, R.adj))
          .slice(0, CLUSTER_EXPAND_CAP)
          .map((n) => n.id)
        ensureActive(clusterIds)
      }
      setActiveCluster(cur)
      R.activeCluster = cur
      applyBaseStyles()
    },
    [ensureActive, applyBaseStyles]
  )
  const onClearFilter = useCallback(() => {
    if (R.activeCluster) {
      setActiveCluster(null)
      R.activeCluster = null
      applyBaseStyles()
    }
  }, [applyBaseStyles])

  const applyForce = useCallback(
    (p: PhysicsState) => {
      wakeForRelayout()
      R.chart?.setOption({
        series: [{ id: 'field', layout: 'force', data: R.optionNodes, force: { repulsion: p.repulsion, edgeLength: p.edgeLength, gravity: p.gravity, friction: 0.62, layoutAnimation: true } }],
      })
      scheduleFreeze()
    },
    [wakeForRelayout, scheduleFreeze]
  )
  const setPhysicsAndApply = useCallback(
    (patch: Partial<PhysicsState>) => {
      setPhysics((prev) => {
        const next = { ...prev, ...patch }
        applyForce(next)
        return next
      })
    },
    [applyForce]
  )
  const onRepulsion = useCallback((v: number) => setPhysicsAndApply({ repulsion: v }), [setPhysicsAndApply])
  const onEdgeLength = useCallback((v: number) => setPhysicsAndApply({ edgeLength: v }), [setPhysicsAndApply])
  const onGravity = useCallback((v: number) => setPhysicsAndApply({ gravity: v }), [setPhysicsAndApply])

  const zoomBy = useCallback(
    (f: number) => {
      if (!R.chart) return
      R.zoom = Math.max(0.4, Math.min(4, (R.zoom || 1) * f))
      R.chart.setOption({ series: [{ id: 'field', zoom: R.zoom }] })
      onRoam()
    },
    [onRoam]
  )
  const onZoomIn = useCallback(() => zoomBy(1.25), [zoomBy])
  const onZoomOut = useCallback(() => zoomBy(0.8), [zoomBy])
  const onFit = useCallback(() => {
    R.zoom = 1
    R.chart?.setOption({ series: [{ id: 'field', zoom: 1 }] })
    fitViewToActive()
    onRoam()
  }, [onRoam, fitViewToActive])

  const onCopy = useCallback(() => {
    const n = R.selectedId != null ? R.nodeById.get(R.selectedId) : null
    if (!n) return
    try {
      navigator.clipboard?.writeText(`#${n.id} [${n.cluster}] ${n.content}`)
    } catch {
      /* noop */
    }
    setCopied(true)
    clearTimeout(R.copyTimer)
    R.copyTimer = setTimeout(() => setCopied(false), 1400)
  }, [])

  // ── PrimQL：真实指令直连后端，视图类快捷指令走本地 ──
  const runPrimql = useCallback(async () => {
    const cmd = primqlInput.trim()
    if (!cmd) return
    const up = cmd.toUpperCase()
    let out = ''
    if (up === 'HELP') {
      out =
        '原语列表:\n  STATS FULL     场态摘要（打 :5666）\n  GRAPH          导出节点与边 JSON（打 :5666）\n  TRACE <id>     从节点播放扩散激活（本地视图指令）\n  CLUSTER <name> 隔离某个簇（本地视图指令）\n  RESET          清除筛选与追踪（本地视图指令）\n  其余原语（ENGRAM/LOOM/…）直接透传给 PrismD'
    } else if (up.startsWith('TRACE')) {
      const m = cmd.match(/(\d+)/)
      if (m) {
        const id = +m[1]
        if (R.nodeById.has(id)) {
          traceDiffusion(id)
          out = `OK 从 #${id} 播放扩散激活`
        } else out = `ERROR 节点 ${id} 不存在`
      } else out = 'ERROR 用法: TRACE <id>'
    } else if (up.startsWith('CLUSTER')) {
      const name = cmd.split(/\s+/)[1]
      const c = CMAP[Object.keys(CMAP).find((k) => k.toLowerCase() === (name || '').toLowerCase()) || '']
      if (c) {
        toggleCluster(c.key)
        out = `OK 隔离簇 ${c.key}`
      } else out = 'ERROR 未知簇'
    } else if (up === 'RESET') {
      onClearFilter()
      if (R.tracing) stopTrace()
      out = 'OK 已重置'
    } else {
      out = await queryPrimQL(cmd)
    }
    setPrimqlResult(`› ${cmd}\n${out}`)
    setPrimqlInput('')
    setConsoleOpen(true)
  }, [primqlInput, traceDiffusion, toggleCluster, onClearFilter, stopTrace])

  const toggleConsole = useCallback(() => setConsoleOpen((v) => !v), [])

  const onResetLayout = useCallback(() => {
    clearTimeout(R.freezeTimer)
    R.chart?.dispose()
    R.chart = null
    setSelectedId(null)
    setTracing(false)
    setMenu({ show: false, x: 0, y: 0, id: null })
    R.trace = null
    R.activeIds = computeDefaultActiveIds(R.neurons, R.adj)
    setVisibleCount(R.activeIds.size)
    initChartRef.current?.()
  }, [])

  // ── ECharts 初始化 ──
  const initChartRef = useRef<() => void>(null)
  const bootstrapRO = useRef<ResizeObserver | null>(null)
  initChartRef.current = () => {
    const el = chartRef.current
    if (!el) return
    // 容器首帧可能还是 0×0（flex 布局尚未落定 / 挂载时机早于 layout），
    // 此时初始化 ECharts 会在内部坐标系变换里抛异常。等它有真实尺寸再 init。
    if (el.clientWidth === 0 || el.clientHeight === 0) {
      bootstrapRO.current?.disconnect()
      const ro = new ResizeObserver((entries) => {
        const box = entries[0]?.contentRect
        if (box && box.width > 0 && box.height > 0) {
          ro.disconnect()
          bootstrapRO.current = null
          initChartRef.current?.()
        }
      })
      ro.observe(el)
      bootstrapRO.current = ro
      return
    }
    R.chart = echarts.init(el, null, { renderer: 'canvas' })
    R.zoom = 1
    R.labelsAll = false
    rebuildOptionArrays()
    const p = physics
    R.chart.setOption({
      animation: true,
      animationDuration: 520,
      animationDurationUpdate: 420,
      animationEasingUpdate: 'cubicOut',
      tooltip: { show: false },
      series: [
        {
          id: 'field',
          type: 'graph',
          layout: 'force',
          roam: true,
          scaleLimit: { min: 0.4, max: 4 },
          draggable: true,
          nodeScaleRatio: 0.5,
          zoom: R.zoom,
          center: [0, 0],
          force: { repulsion: p.repulsion, edgeLength: p.edgeLength, gravity: p.gravity, friction: 0.62, layoutAnimation: true },
          edgeSymbol: ['none', 'none'],
          label: {
            show: R.labelsAll,
            position: 'right',
            color: '#3a4150',
            fontFamily: "'IBM Plex Mono',monospace",
            fontSize: 11,
            backgroundColor: 'rgba(255,255,255,.92)',
            padding: [3, 6],
            borderRadius: 5,
            borderColor: '#e2e5ec',
            borderWidth: 1,
            formatter: (o: any) => (o.data && o.data._label) || '',
          },
          emphasis: {
            focus: 'adjacency',
            scale: false,
            label: { show: true },
            itemStyle: { shadowBlur: 16 },
            lineStyle: { color: '#5b6472', opacity: 0.85, width: 2 },
          },
          blur: { itemStyle: { opacity: 0.12 }, lineStyle: { opacity: 0.035 }, label: { show: false } },
          data: R.optionNodes,
          links: R.optionLinks,
        },
      ],
    })

    R.chart.on('click', (p: any) => {
      if (p.dataType === 'node') selectNode(+p.data.id)
    })
    R.chart.on('contextmenu', (p: any) => {
      if (p.dataType === 'node' && p.event?.event) {
        p.event.event.preventDefault()
        openMenu(+p.data.id, p.event.event.clientX, p.event.event.clientY)
      }
    })
    R.chart.on('mouseover', (p: any) => {
      if (p.dataType === 'node') R.hovering = true
    })
    R.chart.on('mouseout', () => {
      R.hovering = false
    })
    R.chart.on('graphroam', () => onRoam())
    const zr = R.chart.getZr()
    zr.on('mousedown', () => {
      R.interacting = true
    })
    zr.on('mouseup', () => {
      R.interacting = false
    })
    zr.on('click', (e: any) => {
      if (!e.target) onBlank()
    })
    zr.on('contextmenu', (e: any) => {
      if (e?.event) e.event.preventDefault()
    })
    el.addEventListener('contextmenu', (e) => e.preventDefault())

    clearInterval(R.pulseTimer)
    R.pulseTimer = setInterval(pulse, 95)
    scheduleFreeze(FREEZE_DELAY_MS)
  }

  // ── 首次挂载 + 卸载清理 ──
  useEffect(() => {
    const onDocDown = () => {
      if (menuRef.current.show) setMenu((m) => ({ ...m, show: false }))
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (R.tracing) stopTrace()
        else if (R.selectedId != null) closeCard()
      }
    }
    document.addEventListener('mousedown', onDocDown)
    document.addEventListener('keydown', onKey)
    const onResize = () => R.chart?.resize()
    window.addEventListener('resize', onResize)
    return () => {
      document.removeEventListener('mousedown', onDocDown)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('resize', onResize)
      clearInterval(R.pulseTimer)
      clearTimeout(R.traceTimer)
      clearTimeout(R.freezeTimer)
      clearTimeout(R.copyTimer)
      bootstrapRO.current?.disconnect()
      bootstrapRO.current = null
      R.chart?.dispose()
      R.chart = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const menuRef = useRef(menu)
  menuRef.current = menu

  // ── 数据到达/刷新：首次建图，之后增量对账，避免每次 poll 都触发力导向重排 ──
  useEffect(() => {
    if (neurons.length === 0) return
    const prevIds = new Set(R.neurons.map((n) => n.id))
    indexData(neurons, edges)
    R.neurons = neurons
    R.edges = edges

    if (R.activeIds.size === 0) {
      R.activeIds = computeDefaultActiveIds(neurons, R.adj)
      setVisibleCount(R.activeIds.size)
    } else {
      const idSet = new Set(neurons.map((n) => n.id))
      let shrank = false
      R.activeIds.forEach((id) => {
        if (!idSet.has(id)) {
          R.activeIds.delete(id)
          shrank = true
        }
      })
      if (shrank) setVisibleCount(R.activeIds.size)
    }

    if (!R.chart) {
      initChartRef.current?.()
      return
    }

    const nextIds = new Set(neurons.map((n) => n.id))
    let topologyChanged = nextIds.size !== prevIds.size
    if (!topologyChanged) {
      for (const id of nextIds) {
        if (!prevIds.has(id)) {
          topologyChanged = true
          break
        }
      }
    }

    // 后端每次轮询返回的节点/边顺序不保证一致，这里整段数组都是重建的——
    // 必须把已冻结(layout:'none')的 x/y 按 id 搬过来，否则新节点对象没有坐标，
    // ECharts 在 layout:'none' 下拿到 undefined 坐标会在内部坐标变换里直接抛异常。
    const prevPos = new Map<number, { x: number; y: number }>()
    R.optionNodes.forEach((it: any) => {
      if (!it.fixed && it.x != null && it.y != null) prevPos.set(+it.id, { x: it.x, y: it.y })
    })
    rebuildOptionArrays()
    R.optionNodes.forEach((it: any) => {
      if (it.fixed) return
      const pos = prevPos.get(+it.id)
      if (pos) {
        it.x = pos.x
        it.y = pos.y
      }
    })
    if (R.tracing && R.trace) {
      applyTraceStyles()
    } else if (topologyChanged) {
      wakeForRelayout()
      R.chart.setOption({ series: [{ id: 'field', layout: 'force', data: R.optionNodes, links: R.optionLinks }] })
      scheduleFreeze()
    } else {
      updateStyles()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [neurons, edges])

  // ── stats ──
  const stats = {
    nodes: neurons.length,
    edges: edges.length,
    active: neurons.filter((n) => eff(n) >= 0.7).length,
    dormant: neurons.filter((n) => eff(n) < 0.24).length,
  }

  return {
    chartRef,
    stats,
    visibleCount,
    selectedId,
    menu,
    tracing,
    traceHop: traceHopState,
    traceStart,
    traceMaxHop,
    tracePlaying,
    activeCluster,
    physics,
    consoleOpen,
    primqlInput,
    primqlResult,
    copied,
    nodeById: R.nodeById,
    adj: R.adj,
    // actions
    selectNode,
    closeCard,
    openMenu,
    closeMenu,
    toggleCluster,
    onClearFilter,
    onRepulsion,
    onEdgeLength,
    onGravity,
    onResetLayout,
    onZoomIn,
    onZoomOut,
    onFit,
    onCopy,
    traceDiffusion,
    onTraceNext,
    onTracePrev,
    onTracePlayToggle,
    stopTrace,
    setPrimqlInput,
    runPrimql,
    toggleConsole,
  }
}

export type GraphController = ReturnType<typeof useGraphController>
