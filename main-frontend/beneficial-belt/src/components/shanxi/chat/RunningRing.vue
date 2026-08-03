<template>
  <!-- 运行态电流环：纯 SVG 描边，零 conic / 零 mask，任何内核都稳。
       pathLength=100 → 整条路径周长归一化为 100，dashoffset 从 100 匀退到 0
       恰好绕一整圈、无缝且随尺寸自适应（不用算真实周长）。 -->
  <svg
    class="running-ring"
    :width="w"
    :height="h"
    viewBox="0 0 100 40"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <!-- 电流弧：一段主题色沿边框滑 -->
    <rect
      x="0.5" y="0.5" width="99" height="39"
      pathLength="100" fill="none"
      :stroke="accent" stroke-width="2"
      stroke-linecap="butt"
      vector-effect="non-scaling-stroke"
      stroke-dasharray="16 84"
      class="running-ring-current"
    />
  </svg>
</template>

<script setup>
defineProps({
  w: { type: [Number, String], default: 100 },
  h: { type: [Number, String], default: 40 },
  accent: { type: String, default: 'var(--app-accent)' },
})
</script>

<style scoped>
.running-ring-current {
  animation: running-ring-flow 4.2s linear infinite;
}
/* 电流段沿归一化周长从 0 匀滑到 100（回到原点即一整圈），缓慢流动，无缝循环 */
@keyframes running-ring-flow {
  from { stroke-dashoffset: 0; }
  to { stroke-dashoffset: -100; }
}
</style>