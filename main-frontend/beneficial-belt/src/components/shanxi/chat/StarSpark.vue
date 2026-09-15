<template>
  <!-- StarSpark：原创「星尘凝星」状态图标。
       running：3 颗光点绕中心螺旋汇聚（持续循环=进行中）；
       done：定格成双层星芒 + 光晕呼吸（外晕缓慢明灭，不是死实心块）。
       viewBox 0 0 100 100 矢量为王，颜色走 currentColor。 -->
  <svg
    class="star-spark"
    :width="size"
    :height="size"
    viewBox="0 0 100 100"
    fill="none"
    role="img"
    :aria-label="done ? '完成' : '进行中'"
  >
    <!-- 定格态：外晕 + 主星芒 + 中心亮点 -->
    <g v-if="done" class="ss-frozen">
      <path :d="starD" class="ss-halo" fill="currentColor"
        transform="translate(50 50) scale(1.42) translate(-50 -50)" />
      <path :d="starD" fill="currentColor" />
      <circle cx="50" cy="50" r="4.6" fill="#fff" opacity="0.85" />
    </g>
    <!-- 运行态：轨道 + 汇聚光点 + 中心核 -->
    <g v-else :transform="`rotate(${rot} 50 50)`">
      <circle cx="50" cy="50" r="30" stroke="currentColor" stroke-opacity="0.12" stroke-width="2" />
      <circle
        v-for="(s, i) in sparks"
        :key="i"
        :cx="s.x"
        :cy="s.y"
        :r="s.r"
        :opacity="s.op"
        fill="currentColor"
      />
      <circle cx="50" cy="50" :r="coreR" :opacity="coreOp" fill="currentColor" />
    </g>
  </svg>
</template>

<script setup>
import { onUnmounted, ref, watch } from 'vue'

const props = defineProps({
  size: { type: Number, default: 16 },
  done: { type: Boolean, default: false },
})

// 四角星芒：外尖半径 40、内凹半径 15，尖角之间用二次贝塞尔拉成内凹弧
const starD = (() => {
  const pts = []
  const R = 40, r = 15
  for (let i = 0; i < 8; i++) {
    const rad = i % 2 === 0 ? R : r
    const a = (Math.PI / 4) * i - Math.PI / 2
    pts.push([50 + Math.cos(a) * rad, 50 + Math.sin(a) * rad])
  }
  let d = `M ${pts[0][0].toFixed(1)} ${pts[0][1].toFixed(1)}`
  for (let i = 1; i <= 8; i++) {
    const cur = pts[i % 8], prev = pts[i - 1]
    d += ` Q ${prev[0].toFixed(1)} ${prev[1].toFixed(1)} ${((cur[0] + prev[0]) / 2).toFixed(1)} ${((cur[1] + prev[1]) / 2).toFixed(1)}`
  }
  return d + ' Z'
})()

// ── 运行态：光点由外向内螺旋汇聚，到心后重新甩出（循环=持续进行）──
const N = 3
const rot = ref(0)
const phase = ref(0)
const sparks = ref(Array.from({ length: N }, () => ({ x: 80, y: 50, r: 3, op: 1 })))
const coreR = ref(3)
const coreOp = ref(0.5)

let raf = 0, last = 0
function tick(t) {
  if (!last) last = t
  const dt = Math.min(64, t - last)
  last = t
  phase.value = (phase.value + dt / 1400) % 1
  rot.value = (rot.value + dt * 0.12) % 360
  const p = phase.value
  for (let i = 0; i < N; i++) {
    const q = (p + i / N) % 1
    const ease = q * q
    const rad = 34 - 30 * ease
    const ang = q * Math.PI * 1.6
    sparks.value[i] = {
      x: (50 + Math.cos(ang) * rad).toFixed(1),
      y: (50 + Math.sin(ang) * rad).toFixed(1),
      r: (1.4 + (1 - ease) * 2.4).toFixed(2),
      op: (0.35 + (1 - ease) * 0.6).toFixed(2),
    }
  }
  coreR.value = (2.5 + Math.sin(p * Math.PI) * 2.5).toFixed(2)
  coreOp.value = (0.4 + Math.sin(p * Math.PI) * 0.5).toFixed(2)
  raf = requestAnimationFrame(tick)
}
if (!props.done) raf = requestAnimationFrame(tick)
watch(() => props.done, (d) => {
  if (d) { if (raf) cancelAnimationFrame(raf); raf = 0 }
  else if (!raf) { last = 0; raf = requestAnimationFrame(tick) }
})
onUnmounted(() => { if (raf) cancelAnimationFrame(raf) })
</script>

<style scoped>
.star-spark { display: inline-block; overflow: visible; flex: none; line-height: 0; }
.ss-frozen { transform-origin: 50% 50%; animation: ss-pop 0.34s cubic-bezier(0.34, 1.56, 0.64, 1); }
.ss-halo { opacity: 0.22; transform-origin: 50% 50%; animation: ss-breathe 2.4s ease-in-out infinite; }
@keyframes ss-pop { 0% { transform: scale(0.4); opacity: 0; } 100% { transform: scale(1); opacity: 1; } }
@keyframes ss-breathe { 0%, 100% { opacity: 0.12; } 50% { opacity: 0.3; } }
</style>
