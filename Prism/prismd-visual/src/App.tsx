// src/App.tsx — 星核记忆（使用 STATS FULL 获取完整记忆卡片）
import { useEffect, useState, useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { Stars, OrbitControls } from '@react-three/drei'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import * as THREE from 'three'

const API_BASE = '/api'

async function queryPrimQL(body: string): Promise<string> {
  const res = await fetch(API_BASE, { method: 'POST', headers: { 'Content-Type': 'text/plain' }, body })
  return await res.text()
}

interface NeuronData {
  id: number
  role: string
  content: string
  conductance: number
  emotion?: string
  intensity?: number
  eventType?: string
}

function parseStatsFull(text: string): NeuronData[] {
  const neurons: NeuronData[] = []
  const blocks = text.split('── ID:')
  // 跳过分隔符前的空块，从第一个 '── ID:' 开始解析
  for (let i = 1; i < blocks.length; i++) {
    const block = '── ID:' + blocks[i]
    const lines = block.split('\n').map(l => l.trim()).filter(l => l !== '')

    let id = 0, role = '', content = '', conductance = 0.5, emotion = '', intensity = 0, eventType = ''

    for (const line of lines) {
      if (line.startsWith('── ID:')) {
        id = parseInt(line.match(/\d+/)?.[0] ?? '0') || 0
      } else if (line.startsWith('Role:')) {
        role = line.slice(5).trim() || 'memory'
      } else if (line.startsWith('Content:')) {
        content = line.slice(8).trim() || ''
      } else if (line.startsWith('Energy:')) {
        // Energy: 0.50 | Emotion: angry | Intensity: 0.80 | EventType: conflict
        const energyMatch = line.match(/Energy:\s*([\d.]+)/)
        const emotionMatch = line.match(/Emotion:\s*(\S+)/)
        const intensityMatch = line.match(/Intensity:\s*([\d.]+)/)
        const eventMatch = line.match(/EventType:\s*(\S+)/)
        conductance = parseFloat(energyMatch?.[1] ?? '0.5')
        emotion = emotionMatch?.[1] ?? '-'
        intensity = parseFloat(intensityMatch?.[1] ?? '0')
        eventType = eventMatch?.[1] ?? '-'
      }
    }

    if (id) {
      neurons.push({ id, role, content, conductance, emotion, intensity, eventType })
    }
  }

  return neurons
}

function StarCore({ onClick }: { onClick: () => void }) {
  const meshRef = useRef<THREE.Mesh>(null)
  useFrame((state) => {
    if (meshRef.current) {
      meshRef.current.rotation.y = state.clock.getElapsedTime() * 0.2
      meshRef.current.rotation.x = Math.sin(state.clock.getElapsedTime() * 0.1) * 0.05
    }
  })
  return (
    <mesh ref={meshRef} onClick={onClick}>
      <sphereGeometry args={[0.8, 32, 32]} />
      <meshStandardMaterial color="#ffffff" emissive="#7aa2f7" emissiveIntensity={2.5} roughness={0.1} metalness={0.1} />
    </mesh>
  )
}

function MemoryPanel({ neurons, onClose }: { neurons: NeuronData[]; onClose: () => void }) {
  const [selected, setSelected] = useState<NeuronData | null>(null)
  return (
    <div style={{
      position: 'absolute', right: 20, top: 20, bottom: 20, width: 340,
      background: 'rgba(10, 14, 30, 0.55)', backdropFilter: 'blur(28px)',
      border: '1px solid rgba(122, 162, 247, 0.25)', borderRadius: 20,
      boxShadow: '0 20px 60px rgba(0,0,0,0.7)', color: '#edf2ff',
      fontFamily: 'monospace', display: 'flex', flexDirection: 'column', overflow: 'hidden', zIndex: 10
    }}>
      <div style={{ padding: 16, borderBottom: '1px solid rgba(122,162,247,0.2)', fontWeight: 600 }}>
        星核记忆 ({neurons.length})
        <button onClick={onClose} style={{ float: 'right', background: 'none', border: 'none', color: '#7aa2f7', cursor: 'pointer', fontSize: 16 }}>✕</button>
      </div>
      <div style={{ flex: 1, overflowY: 'auto', padding: 12 }}>
        {neurons.map((n) => (
          <div key={n.id} onClick={() => setSelected(n)} style={{
            padding: 10, marginBottom: 6, borderRadius: 12,
            background: selected?.id === n.id ? 'rgba(122,162,247,0.15)' : 'rgba(255,255,255,0.03)',
            border: selected?.id === n.id ? '1px solid rgba(122,162,247,0.4)' : '1px solid transparent',
            cursor: 'pointer', transition: 'all 0.2s'
          }}>
            <div style={{ fontSize: 12, color: '#90a4d4' }}>ID:{n.id} · {n.role} · ⚡{n.conductance.toFixed(2)}</div>
            <div style={{ fontSize: 13, marginTop: 4, lineHeight: 1.5 }}>{n.content}</div>
            {n.emotion && n.emotion !== '-' && (
              <div style={{ marginTop: 4, fontSize: 11, color: '#e0af68' }}>
                {n.emotion} · 强度{n.intensity?.toFixed(2)} · {n.eventType}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export default function App() {
  const [neurons, setNeurons] = useState<NeuronData[]>([])
  const [panelOpen, setPanelOpen] = useState(false)

  const fetchNeurons = async () => {
    try {
      // 关键修复：请求 STATS FULL 获取完整记忆内容
      const raw = await queryPrimQL('STATS FULL')
      console.log('PrismD 原始响应:', raw)
      if (raw.startsWith('ERROR') || raw.startsWith('UNKNOWN')) {
        console.warn('PrismD 返回错误:', raw)
        return
      }
      const list = parseStatsFull(raw)
      setNeurons(list)
    } catch (e) {
      console.error('无法连接 PrismD:', e)
    }
  }

  useEffect(() => {
    fetchNeurons()
    const timer = setInterval(fetchNeurons, 15000)
    return () => clearInterval(timer)
  }, [])

  return (
    <div style={{ width: '100vw', height: '100vh', overflow: 'hidden', background: '#0a0b14', position: 'relative' }}>
      <Canvas camera={{ position: [0, 0, 7], fov: 45 }}>
        <color attach="background" args={['#0a0b14']} />
        <ambientLight intensity={0.3} />
        <pointLight position={[10, 10, 10]} intensity={1} color="#7aa2f7" />
        <Stars radius={50} depth={50} count={2000} factor={5} saturation={0} fade speed={0.6} />
        <StarCore onClick={() => { setPanelOpen(true); fetchNeurons() }} />
        <OrbitControls enableDamping dampingFactor={0.05} autoRotate autoRotateSpeed={0.4} minDistance={3} maxDistance={15} />
        <EffectComposer>
          <Bloom luminanceThreshold={0.2} intensity={1.2} levels={7} mipmapBlur />
        </EffectComposer>
      </Canvas>

      {panelOpen && <MemoryPanel neurons={neurons} onClose={() => setPanelOpen(false)} />}

      {!panelOpen && (
        <div style={{
          position: 'absolute', bottom: 30, left: '50%', transform: 'translateX(-50%)',
          color: '#565f89', fontSize: 13, fontFamily: 'monospace',
          pointerEvents: 'none', background: 'rgba(10,14,30,0.6)', padding: '6px 16px', borderRadius: 20
        }}>
          ⚡ 点击星核，翻阅记忆
        </div>
      )}
    </div>
  )
}