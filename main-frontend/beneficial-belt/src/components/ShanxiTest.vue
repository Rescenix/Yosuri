<template>
  <div ref="container" style="width: 100vw; height: 100vh;"></div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import * as THREE from 'three'
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js'
import { VRMLoaderPlugin } from '@pixiv/three-vrm'

const container = ref(null)

onMounted(() => {
  const scene = new THREE.Scene()
  scene.background = new THREE.Color('#333')

  const camera = new THREE.PerspectiveCamera(45, window.innerWidth / window.innerHeight, 0.1, 100)
  camera.position.set(0, 2, 4)

  const renderer = new THREE.WebGLRenderer({ antialias: true })
  renderer.setSize(window.innerWidth, window.innerHeight)
  container.value.appendChild(renderer.domElement)

  scene.add(new THREE.AmbientLight(0xffffff, 1.5))

  const loader = new GLTFLoader()
  loader.register(parser => new VRMLoaderPlugin(parser))

  loader.load('/models/shanxi.vrm', (gltf) => {
    const vrm = gltf.userData.vrm
    if (!vrm) return console.error('VRM 数据为空')

    const model = vrm.scene
    model.position.set(0, 0, 0)
    scene.add(model)

    let walkTime = 0
    if (vrm.humanoid) vrm.humanoid.autoUpdateHumanBones = true

    function animate() {
      requestAnimationFrame(animate)
      walkTime += 0.1
      const swing = Math.sin(walkTime) * 0.3
      const h = vrm.humanoid
      if (h) {
        const l = h.getNormalizedBoneNode('leftUpperLeg')
        const r = h.getNormalizedBoneNode('rightUpperLeg')
        if (l) l.rotation.x = swing
        if (r) r.rotation.x = -swing
      }
      if (vrm.update) vrm.update(0.016)
      renderer.render(scene, camera)
    }
    animate()
    console.log('杉汐已加载')
  }, undefined, (err) => console.error('加载失败', err))
})
</script>