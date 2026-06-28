import { Canvas } from '@react-three/fiber'
import { OrbitControls } from '@react-three/drei'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import { usePrismData } from './hooks/usePrismData'
import { useUIState } from './store/useUIState'
import Starfield from './components/Starfield'
import StarCore from './components/StarCore'
import GalaxyField from './components/GalaxyField'
import MemoryPanel from './components/MemoryPanel'
import CameraController from './components/CameraController'

export default function App() {
  const { selectedNode, flyToCenter, closePanel } = useUIState() // ★ 使用新的 API
  const { neurons, edges } = usePrismData()

  return (
    <div style={{ width: '100vw', height: '100vh', overflow: 'hidden', background: '#0a0b14', position: 'relative' }}>
      <Canvas camera={{ position: [0, 2, 12], fov: 45 }}>
        <color attach="background" args={['#0a0b14']} />
        <ambientLight intensity={0.3} />
        <Starfield />
        <GalaxyField neurons={neurons} edges={edges} />
        
        {/* 只有主动点击星核时，才会触发飞回中心的动画 */}
        <StarCore onClick={flyToCenter} />
        
        <CameraController />
        
        {/* 坚决不留 target 属性，彻底切断 React 对 controls 状态的重置路径 */}
        <OrbitControls 
          makeDefault
          enableDamping
          dampingFactor={0.08}
          autoRotateSpeed={0.4}
          minDistance={2}
          maxDistance={30}
        />
        
        <EffectComposer>
          <Bloom luminanceThreshold={0.2} intensity={1.2} levels={7} mipmapBlur />
        </EffectComposer>
      </Canvas>

      {/* 点击关闭面板 -> 只清选中不移动相机 */}
      {selectedNode && <MemoryPanel onClose={closePanel} />}
    </div>
  )
}