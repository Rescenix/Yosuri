import { useRef } from 'react'
import { useFrame } from '@react-three/fiber'
import * as THREE from 'three'

export default function StarCore({ onClick }: { onClick: () => void }) {
  const ref = useRef<THREE.Mesh>(null)
  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.y = state.clock.getElapsedTime() * 0.2
      ref.current.rotation.x = Math.sin(state.clock.getElapsedTime() * 0.1) * 0.05
    }
  })
  return (
    <mesh ref={ref} onClick={onClick}>
      <sphereGeometry args={[0.8, 32, 32]} />
      <meshStandardMaterial color="#ffffff" emissive="#7aa2f7" emissiveIntensity={2.5} roughness={0.1} metalness={0.1} />
    </mesh>
  )
}