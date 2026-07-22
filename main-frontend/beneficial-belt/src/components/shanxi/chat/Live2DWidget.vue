<template>
  <div class="live2d-widget" v-show="ready">
    <canvas ref="canvasRef" class="live2d-canvas"></canvas>
  </div>
</template>

<script setup>
import { onMounted, onBeforeUnmount, ref } from 'vue'

// 看板娘:pixi-live2d-display + Cubism4 Core。资源放在 public/live2d/ 下:
//   /live2d/core/live2dcubismcore.min.js   (Cubism Core 运行时)
//   /live2d/model/model.model3.json        (Cubism4 模型入口,连同 moc3/贴图/动作)
// 资源缺失时静默降级(不渲染、不报错),不影响其余 UI。换模型只需替换 public 下的文件。

const MODEL_URL = '/live2d/model/model.model3.json'
const CORE_URL = '/live2d/core/live2dcubismcore.min.js'

const canvasRef = ref(null)
const ready = ref(false)
let app = null
let model = null

function loadCore() {
  return new Promise((resolve, reject) => {
    if (window.Live2DCubismCore) return resolve()
    const s = document.createElement('script')
    s.src = CORE_URL
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => reject(new Error('Cubism Core 加载失败'))
    document.head.appendChild(s)
  })
}

onMounted(async () => {
  try {
    // 先探模型在不在,不在就别折腾 pixi/core(常见于还没放资源)
    const exists = await fetch(MODEL_URL, { method: 'HEAD' }).then(r => r.ok).catch(() => false)
    if (!exists) { console.info('[Live2D] 未找到模型资源，看板娘暂不显示'); return }

    await loadCore()
    const PIXI = await import('pixi.js')
    window.PIXI = PIXI // pixi-live2d-display 依赖全局 PIXI
    const { Live2DModel } = await import('pixi-live2d-display/cubism4')

    // 用显式宽高建 renderer——不能靠 resizeTo,它是下一帧才生效,
    // 紧接着 fit() 读到的 renderer.width 还是 0,模型会被缩放到 0(看不见,正是"没显示"的根因)
    const parent = canvasRef.value.parentElement
    const W = parent?.clientWidth || 220
    const H = parent?.clientHeight || 300
    app = new PIXI.Application({
      view: canvasRef.value,
      backgroundAlpha: 0,
      autoStart: true,
      antialias: true,
      width: W,
      height: H,
    })
    model = await Live2DModel.from(MODEL_URL, { autoInteract: false })
    app.stage.addChild(model)

    // model.width/height 是 scale=1 时的显示尺寸;取不到就退回模型设计尺寸,再兜底 1024
    const mw = model.width || model.internalModel?.originalWidth || 1024
    const mh = model.height || model.internalModel?.originalHeight || 1024
    // min 同时约束宽高 → 整只都在框内不裁;留 8% 边,避免贴边被画布裁掉腿/头。
    // 居中放(anchor 0.5,0.5 + 画布正中),不再底部对齐——底部对齐时整只偏上、
    // 下半身容易压到画布下沿被裁,居中最稳。
    const s = Math.min(W / mw, H / mh) * 0.92
    model.scale.set(s)
    model.anchor.set(0.5, 0.5)
    model.x = W / 2
    model.y = H / 2
    ready.value = true
  } catch (e) {
    console.warn('[Live2D] 初始化失败，看板娘跳过:', e)
    ready.value = false
  }
})

onBeforeUnmount(() => {
  try {
    if (model) model.destroy()
    if (app) app.destroy(true, { children: true, texture: true, baseTexture: true })
  } catch { /* 忽略销毁噪声 */ }
  app = null
  model = null
})
</script>

<style scoped>
.live2d-widget {
  width: 240px;
  height: 380px; /* 高一点给全身留位,别把下半身裁了 */
  filter: drop-shadow(0 6px 14px rgba(0, 0, 0, 0.18));
}
.live2d-canvas {
  width: 100%;
  height: 100%;
  display: block;
}
</style>
