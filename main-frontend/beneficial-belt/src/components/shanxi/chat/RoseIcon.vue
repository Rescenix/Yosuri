<template>
  <!-- RoseIcon：静态玫瑰曲线图标（思考行/时间线的思考标识）。
       与动态 RoseParticleLoader 同源：5 瓣玫瑰曲线 + 粒子点，但静止不转。
       颜色走 currentColor —— 外层 .icon-think 定色(紫)。 -->
  <svg
    class="rose-icon"
    :width="size"
    :height="size"
    viewBox="0 0 100 100"
    :fill="filled ? 'currentColor' : 'none'"
    aria-hidden="true"
  >
    <!-- filled：5 瓣玫瑰曲线闭合涂满（实心矢量玫瑰，收束概要栏用）。
         曲线本身只占 viewBox ~61%，放大 1.55 倍让花瓣顶到边缘，避免显小。 -->
    <path
      v-if="filled"
      :d="skeleton + ' Z'"
      fill="currentColor"
      transform="translate(50 50) scale(1.35) translate(-50 -50)"
    />
    <template v-else>
      <path :d="skeleton" stroke="currentColor" stroke-opacity="0.18" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
      <g>
        <circle
          v-for="(c, i) in particles"
          :key="i"
          :cx="c.x"
          :cy="c.y"
          :r="c.r"
          fill="currentColor"
          :opacity="c.op"
        />
      </g>
      <circle cx="50" cy="50" r="4" fill="currentColor" opacity="0.9" />
    </template>
  </svg>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  size: { type: Number, default: 14 },
  // filled：曲线闭合后整体涂满（实心矢量玫瑰），不画骨架线与粒子
  filled: { type: Boolean, default: false },
})

// 5 瓣玫瑰曲线（与 RoseParticleLoader / Hermes loader.tsx 同参数），静态采样
const TWO_PI = Math.PI * 2
const DETAIL = 0.9
const K = 5
const STEPS = 140
const PARTICLE_N = 40

function rosePoint(p, d = DETAIL, k = K) {
  const t = p * TWO_PI
  const a = 9.2 + d * 0.6
  const r = a * (0.72 + d * 0.28) * Math.cos(k * t)
  return {
    x: 50 + Math.cos(t) * r * 3.25,
    y: 50 + Math.sin(t) * r * 3.25,
  }
}

const skeleton = computed(() => {
  const pts = Array.from({ length: STEPS + 1 }, (_, i) => {
    const p = rosePoint(i / STEPS)
    return `${p.x.toFixed(2)},${p.y.toFixed(2)}`
  })
  return 'M' + pts.join(' ')
})

// 沿轨迹取一组粒子，头大尾小，铺满整条曲线（静态呈现完整形态）
const particles = computed(() => {
  const out = []
  for (let i = 0; i < PARTICLE_N; i++) {
    const tail = i / (PARTICLE_N - 1)
    const fade = (1 - tail) ** 0.56
    const p = rosePoint((i / PARTICLE_N) % 1.0)
    out.push({
      x: p.x.toFixed(2),
      y: p.y.toFixed(2),
      r: ((0.9 + fade * 2.6) * 0.72).toFixed(2),
      op: (0.22 + fade * 0.78).toFixed(3),
    })
  }
  return out
})
</script>

<style scoped>
.rose-icon {
  display: inline-block;
  overflow: visible;
  flex: none;
  line-height: 0;
}
</style>