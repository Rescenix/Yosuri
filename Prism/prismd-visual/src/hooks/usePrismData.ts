import { useState, useEffect, useRef } from 'react'
import type { NeuronData, GraphEdge } from '../types'
import { parseStatsFull } from '../utils/parseStatsFull'

const PRISM_URL = 'http://localhost:5666'

async function query(body: string): Promise<string> {
  const res = await fetch(PRISM_URL, { method: 'POST', headers: { 'Content-Type': 'text/plain' }, body })
  return res.text()
}

export function usePrismData(pollInterval = 15000) {
  const [neurons, setNeurons] = useState<NeuronData[]>([])
  const [edges, setEdges] = useState<GraphEdge[]>([])

  // 使用 ref 存储上一次的数据副本，用于深度比较
  const prevDataRef = useRef<{ neurons: NeuronData[]; edges: GraphEdge[] }>({ neurons: [], edges: [] })

  useEffect(() => {
    const fetchData = async () => {
      const stats = await query('STATS FULL')
      let newNeurons: NeuronData[] = []
      if (!stats.startsWith('ERROR')) {
        newNeurons = parseStatsFull(stats)
      }

      const graph = await query('GRAPH')
      let newEdges: GraphEdge[] = []
      if (!graph.startsWith('ERROR')) {
        try {
          const json = JSON.parse(graph.replace('OK ', ''))
          newEdges = json.edges || []
        } catch {}
      }

      // ★ 核心修复：若内容完全相同，则跳过 setState，避免强制重绘组件
      const currentJSON = JSON.stringify({ neurons: newNeurons, edges: newEdges })
      const prevJSON = JSON.stringify(prevDataRef.current)
      
      if (currentJSON !== prevJSON) {
        prevDataRef.current = { neurons: newNeurons, edges: newEdges }
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