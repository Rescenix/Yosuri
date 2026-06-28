// src/types.ts
export interface NeuronData {
  id: number
  role: string
  content: string
  conductance: number
  emotion?: string
  intensity?: number
  eventType?: string
  cluster?: string
}

export interface GraphEdge {
  from: number
  to: number
  kind: number
  weight: number
}