import { useMemo } from 'react'
import * as THREE from 'three'
import { Line } from '@react-three/drei'

export default function MemoryEdge({ from, to, weight }: { from: THREE.Vector3; to: THREE.Vector3; weight: number }) {
  const points = useMemo(() => [from, to], [from, to])
  const color = weight > 0.5 ? '#ffffff' : weight > 0.1 ? '#7aa2f7' : '#333333'
  return <Line points={points} color={color} lineWidth={weight * 2} transparent opacity={weight} />
}