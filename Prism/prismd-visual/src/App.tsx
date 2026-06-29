import { useEffect, useRef, useState, useMemo, useCallback } from 'react'
import * as echarts from 'echarts'
import { usePrismData } from './hooks/usePrismData'
import { useUIState } from './store/useUIState'
import MemoryPanel from './components/MemoryPanel'

const CLUSTER_COLORS: Record<string, string> = {
  UserBase: '#7aa2f7',
  CodeWork: '#2dd4bf',
  ToolLog: '#555555',
  Session: '#f59e0b',
}

export default function App() {
  const { selectedNode, closePanel } = useUIState()
  const { neurons, edges } = usePrismData()
  const chartRef = useRef<HTMLDivElement>(null)
  const [selectedCluster, setSelectedCluster] = useState<string | null>(null)

  // 簇列表
  const clusters = useMemo(() => {
    const set = new Set(neurons.map(n => n.cluster || 'Unknown'))
    return Array.from(set)
  }, [neurons])

  // 按簇筛选后的数据
  const filteredData = useMemo(() => {
    if (!selectedCluster) return { nodes: neurons, edges }
    const nodeIds = new Set(neurons.filter(n => n.cluster === selectedCluster).map(n => n.id))
    return {
      nodes: neurons.filter(n => n.cluster === selectedCluster),
      edges: edges.filter(e => nodeIds.has(e.from) && nodeIds.has(e.to)),
    }
  }, [neurons, edges, selectedCluster])

  // ------ PrimQL 控制台状态 ------
  const [consoleOpen, setConsoleOpen] = useState(false)
  const [primqlInput, setPrimqlInput] = useState('')
  const [primqlResult, setPrimqlResult] = useState('')
  const [primqlLoading, setPrimqlLoading] = useState(false)

  const sendPrimQL = useCallback(async (cmd?: string) => {
    const payload = (cmd ?? primqlInput).trim()
    if (!payload) return
    setPrimqlLoading(true)
    setPrimqlResult('')
    try {
      const res = await fetch('http://localhost:5666', {
        method: 'POST',
        body: payload,
      })
      const text = await res.text()
      setPrimqlResult(text)
    } catch (err: any) {
      setPrimqlResult(`Error: ${err.message}`)
    } finally {
      setPrimqlLoading(false)
    }
  }, [primqlInput])

  // 键盘 Enter 发送
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendPrimQL()
    }
  }, [sendPrimQL])

  useEffect(() => {
    if (!chartRef.current) return
    const chart = echarts.init(chartRef.current)

    const { nodes, edges } = filteredData

    chart.setOption({
      tooltip: {
        formatter: (params: any) => {
          if (params.dataType === 'node') {
            return `<b>${params.data.label}</b><br/>${params.data.content}<br/>Energy: ${params.data.energy}`
          }
          return `Weight: ${params.data.weight}`
        },
      },
      series: [
        {
          type: 'graph',
          layout: 'force',
          force: {
            repulsion: 200,
            edgeLength: [100, 300],
            gravity: 0.1,
          },
          roam: true,
          draggable: true,
          data: nodes.map(n => ({
            id: n.id,
            name: n.content?.substring(0, 30) || '...',
            label: { show: true, fontSize: 11 },
            symbolSize: 8 + (n.conductance || 0.5) * 12,
            itemStyle: { color: CLUSTER_COLORS[n.cluster || ''] || '#cccccc' },
            energy: n.conductance,
            content: n.content,
          })),
          edges: edges.map(e => ({
            source: e.from,
            target: e.to,
            lineStyle: {
              opacity: (e.weight || 0.1) * 0.7,
              curveness: 0.1,
              width: Math.max(1, (e.weight || 0.1) * 2),
            },
          })),
        },
      ],
    })

    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.dispose()
    }
  }, [filteredData])

  return (
    <div style={{ width: '100vw', height: '100vh', overflow: 'hidden', background: '#0a0b14', position: 'relative' }}>
      {/* 顶部控制栏 */}
      <div style={{ position: 'absolute', top: 12, left: 12, zIndex: 10, display: 'flex', gap: 8 }}>
        {/* 簇筛选按钮 */}
        {clusters.map(c => (
          <button
            key={c}
            onClick={() => setSelectedCluster(selectedCluster === c ? null : c)}
            style={{
              padding: '4px 12px',
              border: '1px solid #e4dfd4',
              borderRadius: 6,
              background: selectedCluster === c ? '#e8e3d8' : '#fff',
              fontWeight: selectedCluster === c ? 600 : 400,
              fontSize: 12,
              cursor: 'pointer',
            }}
          >
            {c}
          </button>
        ))}
        {/* 控制台开关 */}
        <button
          onClick={() => setConsoleOpen(v => !v)}
          style={{
            padding: '4px 12px',
            border: '1px solid #e4dfd4',
            borderRadius: 6,
            background: consoleOpen ? '#e8e3d8' : '#fff',
            fontSize: 12,
            cursor: 'pointer',
            marginLeft: 16,
          }}
        >
          PrimQL
        </button>
      </div>

      {/* ECharts 图表容器 */}
      <div ref={chartRef} style={{ width: '100%', height: '100%' }} />

      {/* PrimQL 控制台 */}
      {/* PrimQL 控制台 */}
{consoleOpen && (
  <div style={{
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    background: 'rgba(10, 11, 20, 0.92)',
    backdropFilter: 'blur(12px)',
    borderTop: '1px solid #2f3341',
    padding: '12px 16px',
    maxHeight: '35vh',
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    fontFamily: 'monospace',
    fontSize: 13,
    color: '#c0caf5',
  }}>
    {/* 结果区域（现在在上面） */}
    {primqlResult && (
      <div style={{
        flex: 1,
        overflowY: 'auto',
        background: '#0d0e16',
        borderRadius: 6,
        padding: '8px 12px',
        whiteSpace: 'pre-wrap',
        border: '1px solid #2f3341',
        maxHeight: '20vh',
      }}>
        {primqlResult}
      </div>
    )}

    {/* 输入行 */}
    <div style={{ display: 'flex', gap: 8 }}>
      <input
        value={primqlInput}
        onChange={e => setPrimqlInput(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="输入 PrimQL 原语，例如 STATS FULL"
        style={{
          flex: 1,
          padding: '6px 10px',
          borderRadius: 4,
          border: '1px solid #444',
          background: '#1a1b26',
          color: '#c0caf5',
          outline: 'none',
        }}
        autoFocus
      />
      <button
        onClick={() => sendPrimQL()}
        disabled={primqlLoading}
        style={{
          padding: '6px 16px',
          borderRadius: 4,
          border: '1px solid #5a6e8a',
          background: primqlLoading ? '#2a2b36' : '#1e2233',
          color: '#c0caf5',
          cursor: primqlLoading ? 'not-allowed' : 'pointer',
          fontWeight: 600,
        }}
      >
        {primqlLoading ? '⏳' : '▶'}
      </button>
    </div>
  </div>
)}
      {/* 记忆面板（点击星核或节点时显示） */}
      {selectedNode && <MemoryPanel onClose={closePanel} />}
    </div>
  )
}