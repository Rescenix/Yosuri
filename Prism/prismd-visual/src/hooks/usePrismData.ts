import { useState, useEffect, useRef } from 'react'
import type { NeuronData, GraphEdge } from '../types'

export const PRISM_URL = 'http://localhost:5666'

export async function queryPrimQL(body: string): Promise<string> {
  const res = await fetch(PRISM_URL, { method: 'POST', headers: { 'Content-Type': 'text/plain' }, body })
  return res.text()
}

interface RawGraphNode {
  id: number
  role: string
  text: string
  energy: number
  decay_rate: number
  last_access_at: string
  cluster: string
  emotion: string
  intensity: number
  event_type: string
}

interface RawGraphEdge {
  from: number
  to: number
  kind: number
  weight: number
  decay_rate: number
  last_used: string
}

export function usePrismData(pollInterval = 15000) {
  const [neurons, setNeurons] = useState<NeuronData[]>([])
  const [edges, setEdges] = useState<GraphEdge[]>([])
  const prevJSONRef = useRef<string>('')

  useEffect(() => {
    const fetchData = async () => {
      const text = await queryPrimQL('GRAPH')
      if (text.startsWith('ERROR')) return

      let parsed: { nodes?: RawGraphNode[]; edges?: RawGraphEdge[] }
      try {
        parsed = JSON.parse(text.replace(/^OK /, ''))
      } catch {
        return
      }

      // 后端 GRAPH 是遍历 Go map 拼出来的，节点/边的顺序在每次请求之间不保证一致——
      // 排序一下，避免"顺序变了但内容没变"被误判成数据变化，导致每次 poll 都重建力导向布局。
      const newNeurons: NeuronData[] = (parsed.nodes || [])
        .map((n) => ({
          id: n.id,
          role: n.role,
          content: n.text,
          cluster: n.cluster,
          emotion: n.emotion || 'neutral',
          intensity: n.intensity,
          eventType: n.event_type,
          energy: n.energy,
          decayRate: n.decay_rate,
          lastAccessAt: n.last_access_at,
        }))
        .sort((a, b) => a.id - b.id)
      const newEdges: GraphEdge[] = (parsed.edges || [])
        .map((e) => ({
          from: e.from,
          to: e.to,
          kind: e.kind,
          weight: e.weight,
          decayRate: e.decay_rate,
          lastUsed: e.last_used,
        }))
        .sort((a, b) => a.from - b.from || a.to - b.to || a.kind - b.kind)

      const currentJSON = JSON.stringify({ neurons: newNeurons, edges: newEdges })
      if (currentJSON !== prevJSONRef.current) {
        prevJSONRef.current = currentJSON
        setNeurons(newNeurons)
        setEdges(newEdges)
      }
    }

    fetchData()
    const timer = setInterval(fetchData, pollInterval)
    return () => clearInterval(timer)
  }, [pollInterval])

  return { neurons, edges }
}
