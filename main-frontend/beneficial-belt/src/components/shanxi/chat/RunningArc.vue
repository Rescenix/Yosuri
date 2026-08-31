<template>
  <!-- RunningArc：Hermes 桌面侧栏会话行的运行状态边框——conic 渐变亮点沿边框流动。
       忠实转译 Hermes styles.css 的 .arc-border.arc-row：
       - mask 抠出边框环（content-box xor 全盒）
       - 160° linear-gradient 三组亮点（每组：透明→c1 亮→c2 衰减尾）铺 300% 背景，
         用 background-position 从 15% 走到 75% 的 2.23s 循环制造流动感
       颜色跟随主题 accent（--arc-c1），尾巴 45% 透明混合做衰减。 -->
  <span class="running-arc" aria-hidden="true"></span>
</template>

<script setup>
// 无逻辑：纯 CSS 动画。颜色默认 var(--app-accent)，可传 color prop 覆盖。
defineProps({
  color: { type: String, default: 'var(--app-accent)' },
})
</script>

<style scoped>
.running-arc {
  --arc-c0: color-mix(in srgb, var(--arc-c1, var(--app-accent)) 0%, transparent);
  --arc-c1: var(--app-accent);
  --arc-c2: color-mix(in srgb, var(--arc-c1) 45%, transparent);
  --arc-angle: 160deg;
  --arc-width: 1.5px;
  --arc-standoff: 0rem;
  --arc-radius: 0.75rem;
  --arc-duration: 2.23s;

  pointer-events: none;
  position: absolute;
  overflow: hidden;
  border-radius: calc(var(--arc-radius) + var(--arc-standoff));
  inset: calc(var(--arc-standoff) * -1);
  padding: var(--arc-width);
  /* 抠出边框环：不用 mask 简写（scoped 下 content-box 易丢），拆成独立属性 + 双前缀。
     第一层 clip 到 content-box（盖掉 padding 环内部），第二层全盒，exclude 后只剩边框。 */
  -webkit-mask-image: linear-gradient(#000 0 0), linear-gradient(#000 0 0);
  mask-image: linear-gradient(#000 0 0), linear-gradient(#000 0 0);
  -webkit-mask-clip: content-box, border-box;
  mask-clip: content-box, border-box;
  -webkit-mask-composite: xor;
  mask-composite: exclude;
}

.running-arc::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(
    var(--arc-angle),
    transparent 0%,
    var(--arc-c0) 15%,
    var(--arc-c1) 20%,
    var(--arc-c2) 25%,
    transparent 35%,
    transparent 40%,
    var(--arc-c0) 55%,
    var(--arc-c1) 60%,
    var(--arc-c2) 65%,
    transparent 75%,
    transparent 80%,
    var(--arc-c0) 95%,
    var(--arc-c1) 100%
  );
  background-size: 300% 300%;
  animation: running-arc-flow var(--arc-duration) linear infinite;
}

@keyframes running-arc-flow {
  0% {
    background-position: 15% 15%;
  }
  100% {
    background-position: 75% 75%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .running-arc::before {
    animation: none;
  }
}
</style>