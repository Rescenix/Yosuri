<template>
  <!-- 运行态波浪环：一条连续正弦波沿整行边框环绕流动。
       路径按真实像素生成（拐角相位连续、不被拉伸），dashoffset 驱动波头沿周长流动。 -->
  <svg
    class="running-wave-ring"
    width="100%"
    height="100%"
    viewBox="0 0 300 40"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <path :d="path" class="running-wave-path" :style="{ stroke: accent }" />
    <path :d="path" class="running-wave-glow" :style="{ stroke: accent }" />
  </svg>
</template>

<script setup>
defineProps({
  accent: { type: String, default: 'var(--app-accent)' },
})

// 连续正弦环绕矩形路径（真实像素 300×40，内缩 4px，振幅 5、波长 13）。
// 每边起点相位都按走过的周长累加，拐角处波形自然衔接、无跳变。
// 生成 1000 个采样点，SVG 画成平滑折线，视觉等同真曲线。
function buildWavePath(W = 296, H = 36, A = 5, LAM = 13, TOP = 4, n = 1000) {
  const perim = 2 * (W + H)
  const pts = []
  for (let k = 0; k <= n; k++) {
    const u = (perim * k) / n
    let x, y
    if (u <= W) {
      x = u
      y = TOP + A * Math.sin((2 * Math.PI * u) / LAM)
    } else if (u <= W + H) {
      const v = u - W
      x = W
      y = TOP + v + A * Math.sin((2 * Math.PI * v) / LAM + (2 * Math.PI * W) / LAM)
    } else if (u <= 2 * W + H) {
      const v = u - W - H
      x = W - v
      y = TOP + H - A * Math.sin((2 * Math.PI * v) / LAM + (2 * Math.PI * (W + H)) / LAM)
    } else {
      const v = u - 2 * W - H
      x = 0
      y = TOP + H - v + A * Math.sin((2 * Math.PI * v) / LAM + (2 * Math.PI * (2 * W + H)) / LAM)
    }
    pts.push(`${x.toFixed(2)},${y.toFixed(2)}`)
  }
  return 'M' + pts.map((p, i) => (i === 0 ? p : 'L' + p)).join(' ')
}
const path = buildWavePath()
</script>

<style scoped>
.running-wave-ring {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
.running-wave-path {
  fill: none;
  stroke-width: 2.4;
  stroke-linecap: round;
  /* 完整波浪样式：按周长 664 画成整条可见（dash 664 0）。 */
  stroke-dasharray: 664 0;
  opacity: 0.38;
}
/* 亮波头：一段实心高亮沿周长流动，其余透明，营造波浪沿边框传播的呼吸感 */
.running-wave-glow {
  fill: none;
  stroke-width: 2.4;
  stroke-linecap: round;
  stroke-dasharray: 60 604;
  animation: running-wave-flow 3.2s linear infinite;
}
@keyframes running-wave-flow {
  from { stroke-dashoffset: 0; }
  to { stroke-dashoffset: -664; }
}
</style>
