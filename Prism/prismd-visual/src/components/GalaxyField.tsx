import { useMemo, useRef, useEffect } from 'react'
import { useFrame } from '@react-three/fiber'
import { Points, PointMaterial, Text } from '@react-three/drei'
import { useUIState } from '../store/useUIState'
import type { NeuronData, GraphEdge } from '../types'
import * as THREE from 'three'

const CLUSTER_COLORS: Record<string, string> = {
  UserBase: '#7aa2f7', CodeWork: '#2dd4bf',
  ToolLog: '#ff6b6b', Session: '#f59e0b',
}
const EMOTION_COLORS: Record<string, string> = {
  neutral: '#c0d0ff', happy: '#ffd700',
  angry: '#ff4444', sad: '#9966ff', anxious: '#ff9900',
}

function SpaceEnvironment() {
  return <fogExp2 attach="fog" args={['#0a0b14', 0.035]} />
}

function StratifiedNebula({ clusterColor }: { clusterColor: string }) {
  const ref = useRef<THREE.Points>(null)
  const particleData = useMemo(() => {
    const layers = [
      { count: 3000, minR: 0.5, maxR: 1.5 },
      { count: 2500, minR: 1.5, maxR: 3.5 },
      { count: 2000, minR: 3.5, maxR: 6.0 },
    ]
    const positions = new Float32Array(7500 * 3)
    let idx = 0
    layers.forEach(({ count, minR, maxR }) => {
      for (let i = 0; i < count; i++) {
        const r = Math.random() * (maxR - minR) + minR
        const theta = Math.random() * Math.PI * 2
        const phi = Math.random() * Math.PI
        positions[idx * 3] = r * Math.sin(phi) * Math.cos(theta)
        positions[idx * 3 + 1] = r * Math.sin(phi) * Math.sin(theta) - 0.2
        positions[idx * 3 + 2] = r * Math.cos(phi)
        idx++
      }
    })
    return positions
  }, [])

  useFrame(({ clock }) => {
    if (ref.current) {
      ;(ref.current.material as THREE.PointsMaterial).opacity = 
        0.15 + Math.sin(clock.getElapsedTime() * 0.6) * 0.05
    }
  })

  return (
    <Points ref={ref} positions={particleData}>
      <PointMaterial
        transparent color={clusterColor} size={0.08} opacity={0.2}
        sizeAttenuation depthWrite={false} blending={THREE.AdditiveBlending}
      />
    </Points>
  )
}

function GalaxyCluster(props: {
  name: string; nodes: NeuronData[]; allNodes: NeuronData[];
  orbitRadius: number; initialAngle: number; speed: number;
}) {
  const { name, nodes, allNodes, orbitRadius, initialAngle, speed } = props
  const groupRef = useRef<THREE.Group>(null)
  const nodeRefs = useRef<Map<number, THREE.Mesh>>(new Map())
  const coreRef = useRef<THREE.Mesh>(null)
  
  // ★ 核心：拿全局飞行状态，用于暂停公转
  const isFlying = useUIState((state) => state.isAnimating)
  const { flyToNode, focusCluster } = useUIState()

  const centerColor = CLUSTER_COLORS[name] || '#ffffff'
  const baseY = (Math.random() - 0.5) * 2.5

  useEffect(() => {
    if (groupRef.current) {
      const angle = initialAngle
      groupRef.current.position.set(
        Math.cos(angle) * orbitRadius, baseY, Math.sin(angle) * orbitRadius
      )
    }
  }, [initialAngle, orbitRadius, baseY])

  useFrame(({ clock }) => {
    const t = clock.getElapsedTime()
    // ★ 核心修复：只有画面没有在飞行时，星系才继续公转！
    if (groupRef.current && !isFlying) {
      const angle = t * speed + initialAngle
      groupRef.current.position.set(
        Math.cos(angle) * orbitRadius, baseY, Math.sin(angle) * orbitRadius
      )
    }
    if (coreRef.current) {
      const pulse = 1 + Math.sin(t * 1.5 + initialAngle) * 0.15
      coreRef.current.scale.setScalar(pulse)
    }
    nodeRefs.current.forEach((mesh, id) => {
      const node = allNodes.find(n => n.id === id)
      if (node) {
        const scale = 1 + Math.sin(t * 2 + id) * 0.05 * node.conductance
        mesh.scale.set(scale, scale, scale)
      }
    })
  })

  return (
    <group ref={groupRef}>
      <StratifiedNebula clusterColor={centerColor} />
      
      <mesh ref={coreRef} onClick={() => {
        const pos = groupRef.current ? groupRef.current.position.clone() : new THREE.Vector3()
        focusCluster(pos)
      }}>
        <sphereGeometry args={[0.25, 32, 32]} />
        <meshBasicMaterial color={centerColor} />
      </mesh>
      <Text position={[0, 0.7, 0]} fontSize={0.15} color={centerColor} anchorX="center">{name}</Text>

      {nodes.map(n => {
        const idx = nodes.findIndex(x => x.id === n.id)
        const t = idx !== -1 ? idx / Math.max(1, nodes.length) : 0
        const radius = 2 + t * 5
        const angle = radius * 0.8 + t * 4 * Math.PI
        const localPos = new THREE.Vector3(
          Math.cos(angle) * radius,
          Math.sin(n.id * 0.7) * 0.3,
          Math.sin(angle) * radius
        )

        const color = EMOTION_COLORS[n.emotion || 'neutral'] || '#c0d0ff'
        
        // ★ 增加自转
        const rotRef = useRef<THREE.Mesh>(null)
        useFrame(() => {
          if (rotRef.current) {
            rotRef.current.rotation.y += 0.005 * ((n.id % 5) + 1)
          }
        })

        return (
          <mesh
            ref={rotRef}
            key={n.id}
            position={localPos}
            onClick={() => {
              if (groupRef.current) {
                const worldPos = groupRef.current.position.clone().add(localPos)
                flyToNode(n, worldPos)
              }
            }}
          >
            <sphereGeometry args={[0.08 + Math.min(n.conductance, 1) * 0.15, 16, 16]} />
            <meshStandardMaterial color={color} emissive={color} emissiveIntensity={n.conductance * 2} />
          </mesh>
        )
      })}
    </group>
  )
}

export default function GalaxyField({ neurons, edges }: { neurons: NeuronData[]; edges: GraphEdge[] }) {
  const clusters = useMemo(() => {
    const map = new Map<string, NeuronData[]>()
    neurons.forEach(n => {
      const c = n.cluster || 'Unknown'
      if (!map.has(c)) map.set(c, [])
      map.get(c)!.push(n)
    })
    return map
  }, [neurons])

  const clusterConfigs = useMemo(() => {
    const names = Array.from(clusters.keys())
    return names.map((name, i) => ({
      name,
      radius: 6 + Math.random() * 4,
      speed: 0.04 + (1 / (6 + Math.random() * 4)) * 0.3,
      initialAngle: (i / names.length) * Math.PI * 2
    }))
  }, [clusters])

  return (
    <group>
      <SpaceEnvironment />
      {clusterConfigs.map(({ name, radius, speed, initialAngle }) => (
        <GalaxyCluster
          key={name}
          name={name}
          nodes={clusters.get(name) || []}
          allNodes={neurons}
          orbitRadius={radius}
          initialAngle={initialAngle}
          speed={speed}
        />
      ))}
    </group>
  )
}