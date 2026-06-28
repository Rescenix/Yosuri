import { create } from 'zustand'
import * as THREE from 'three'

interface UIState {
  selectedNode: any | null
  cameraMode: 'free' | 'flying'
  isAnimating: boolean
  targetPosition: THREE.Vector3
  targetLookAt: THREE.Vector3
  
  selectNode: (node: any) => void
  flyToNode: (node: any, worldPos: THREE.Vector3) => void
  focusCluster: (worldPos: THREE.Vector3) => void
  flyToCenter: () => void
  closePanel: () => void
  setAnimating: (state: boolean) => void
  setFreeMode: () => void
}

export const useUIState = create<UIState>((set, get) => ({
  selectedNode: null,
  cameraMode: 'free',
  isAnimating: false,
  targetPosition: new THREE.Vector3(0, 2, 12),
  targetLookAt: new THREE.Vector3(0, 0, 0),

  selectNode: (node) => set({ selectedNode: node }),

  flyToNode: (node, worldPos) => set({
    selectedNode: node,
    cameraMode: 'flying',
    isAnimating: true,
    targetPosition: worldPos.clone().add(new THREE.Vector3(0, 2.5, 4.5)),
    targetLookAt: worldPos.clone(),
  }),

  focusCluster: (worldPos) => set({
    cameraMode: 'flying',
    isAnimating: true,
    targetPosition: worldPos.clone().add(new THREE.Vector3(0, 4, 10)),
    targetLookAt: worldPos.clone(),
  }),

  flyToCenter: () => set({
    selectedNode: null,
    cameraMode: 'flying',
    isAnimating: true,
    targetPosition: new THREE.Vector3(0, 2, 12),
    targetLookAt: new THREE.Vector3(0, 0, 0),
  }),

  // ★ 关闭面板：只取消状态，绝对不移动哪怕一像素的画面
  closePanel: () => set({
    selectedNode: null,
    cameraMode: 'free', 
    isAnimating: false
  }),

  setAnimating: (state) => set({ isAnimating: state }),
  
  setFreeMode: () => set({
    cameraMode: 'free',
    isAnimating: false
  }),
}))