<template>
  <!-- RoseParticleIcon：rose-curve 玫瑰曲线粒子图标。
       矢量为王：viewBox 0 0 100 100，放大不糊。
       颜色走 currentColor —— 外层容器定色，深色底=白/浅色底=黑/品牌区=主题色。
       对比 Hermes 原版：粒子和轨迹改 currentColor（不再硬传 color），pathSteps=180、
       粒子数=64、可配 type 决定花瓣数。 -->
  <svg
    class="rose-particle-icon"
    :width="size"
    :height="size"
    viewBox="0 0 100 100"
    fill="none"
    role="status"
    aria-label="运行中"
  >
    <g :transform="`rotate(${rotation} 50 50)`">
      <path
        ref="pathRef"
        :d="pathD"
        opacity="0.1"
        stroke="currentColor"
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="3.24"
      />
      <circle
        v-for="i in particleCount"
        :key="i"
        ref="particleRefs"
        fill="currentColor"
        :opacity="0"
      />
    </g>
  </svg>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'

// ---------- rose-curve 参数（Hermes loader.tsx 转译，改为可配花瓣数） ----------
const TWO_PI = Math.PI * 2
const DURATION_MS = 5400  // 单圈轨迹时长
const PULSE_MS = 4600     // 呼吸缩放周期
const ROTATION_MS = 28000 // 整图自转周期
const TRAIL_SPAN = 0.32   // 拖尾覆盖的进度区间
const PATH_STEPS = 180
const STROKE_SCALE = 0.72

const props = defineProps({
  size: { type: Number, default: 48 },
  // 花瓣系数：5=5瓣玫瑰(默认)，3=三叶，6=六瓣；奇数花瓣数=对称瓣数，偶数=瓣数×2
  petals: { type: Number, default: 5 },
  particleCount: { type: Number, default: 64 },
})

const pathRef = ref(null)
const particleRefs = ref([])
const pathD = ref('')
const rotation = ref(0)
const K = computed(() => props.petals)
const count = computed(() => props.particleCount)

// ---------- 曲线采样 ----------
function rosePoint(progress, detailScale) {
  const t = progress * TWO_PI
  const a = 9.2 + detailScale * 0.6
  const r = a * (0.72 + detailScale * 0.28) * Math.cos(K.value * t)
  return {
    x: 50 + Math.cos(t) * r * 3.25,
    y: 50 + Math.sin(t) * r * 3.25,
  }
}

function buildPath(detailScale) {
  return Array.from({ length: PATH_STEPS + 1 }, (_, index) => {
    const p = rosePoint(index / PATH_STEPS, detailScale)
    return `${index === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`
  }).join(' ')
}

function detailScaleFor(time, phaseOffset) {
  const pulseProgress = ((time + phaseOffset * PULSE_MS) % PULSE_MS) / PULSE_MS
  const angle = pulseProgress * TWO_PI
  return 0.52 + ((Math.sin(angle + 0.55) + 1) / 2) * 0.48
}

function normalize(aspect) {
  return ((aspect % 1) + 1) % 1
}

function particleFor(index, progress, detailScale) {
  const tail = index / (count.value - 1)
  const p = rosePoint(normalize(progress - tail * TRAIL_SPAN), detailScale)
  const fade = (1 - tail) ** 0.56
  return {
    opacity: 0.04 + fade * 0.96,
    radius: (0.9 + fade * 2.7) * STROKE_SCALE,
    x: p.x,
    y: p.y,
  }
}

// ---------- 动画循环 ----------
let raf = 0
let startedAt = 0
const phaseOffset = Math.random()

function render(time) {
  const elapsed = time - startedAt
  const progress = ((elapsed + phaseOffset * DURATION_MS) % DURATION_MS) / DURATION_MS
  const detail = detailScaleFor(elapsed, phaseOffset)

  rotation.value =
    -(((elapsed + phaseOffset * ROTATION_MS) % ROTATION_MS) / ROTATION_MS) * 360

  pathD.value = buildPath(detail)

  const nodes = particleRefs.value
  for (let i = 0; i < nodes.length && i < count.value; i++) {
    const node = nodes[i]
    if (!node) continue
    const p = particleFor(i, progress, detail)
    node.setAttribute('cx', p.x.toFixed(2))
    node.setAttribute('cy', p.y.toFixed(2))
    node.setAttribute('r', p.radius.toFixed(2))
    node.setAttribute('opacity', p.opacity.toFixed(3))
  }

  raf = window.requestAnimationFrame(render)
}

onMounted(() => {
  startedAt = performance.now()
  raf = window.requestAnimationFrame(render)
})

onUnmounted(() => {
  window.cancelAnimationFrame(raf)
})
</script>

<style scoped>
.rose-particle-icon {
  display: inline-block;
  overflow: visible;
  flex: none;
  line-height: 0;
}
</style>