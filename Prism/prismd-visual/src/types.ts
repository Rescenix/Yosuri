// src/types.ts
export type ClusterKey = 'CodeWork' | 'UserBase' | 'ToolLog' | 'Session'
export type Emotion = 'neutral' | 'happy' | 'angry' | 'sad' | 'anxious' | 'excited'
export type EventType = 'conflict' | 'achievement' | 'decision' | 'chat' | 'compilation'

export interface NeuronData {
  id: number
  role: string
  content: string
  cluster: ClusterKey | string
  emotion: Emotion | string
  intensity: number
  eventType: EventType | string
  energy: number
  decayRate: number
  lastAccessAt: string
}

export interface GraphEdge {
  from: number
  to: number
  kind: number
  weight: number
  decayRate: number
  lastUsed: string
}
