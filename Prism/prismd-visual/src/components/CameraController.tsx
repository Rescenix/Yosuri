import { useRef } from 'react'
import { useFrame, useThree } from '@react-three/fiber'
import { useUIState } from '../store/useUIState'
import * as THREE from 'three'

export default function CameraController() {
  const { camera } = useThree()
  const { cameraMode, targetPosition, targetLookAt, isAnimating, setFreeMode } = useUIState()

  // ★ 所有动画状态全部放进 useRef，防止 React 渲染重建导致状态丢失
  const progressRef = useRef(1)
  const startPosRef = useRef(new THREE.Vector3())
  const startTargetRef = useRef(new THREE.Vector3())

  useFrame((state, delta) => {
    const controls = state.controls as any

    // 实时同步 controls 权限
    if (controls) {
      controls.enabled = !isAnimating
      controls.autoRotate = !isAnimating
    }

    // 自由或非动画状态，重置进度
    if (cameraMode === 'free' || !isAnimating) {
      progressRef.current = 1
      return
    }

    // 飞行动画独占模式
    if (cameraMode === 'flying') {
      // 只有刚触发飞行时，记录一次起点
      if (progressRef.current >= 1) {
        startPosRef.current.copy(camera.position)
        startTargetRef.current.copy(controls ? controls.target : new THREE.Vector3(0, 0, 0))
        progressRef.current = 0
      }

      progressRef.current += delta * 1.8
      if (progressRef.current > 1) progressRef.current = 1

      const t = 1 - Math.pow(1 - progressRef.current, 3)

      // 单独控制相机位置
      camera.position.lerpVectors(startPosRef.current, targetPosition, t)

      // 直接在稳定实例上修改 controls.target (符合 Three.js 规范)
      if (controls) {
        controls.target.lerpVectors(startTargetRef.current, targetLookAt, t)
        controls.update() // ★ 强制更新内部状态，防止姿态冲突
      }

      // 飞行结束，解除独占，释放控制权
      if (progressRef.current >= 1) {
        // 最后一次强制把 target 精准定死，防止任何残余的惯性偏移
        if (controls) {
          controls.target.copy(targetLookAt)
          controls.update()
        }
        setFreeMode()
      }
    }
  })

  return null
}