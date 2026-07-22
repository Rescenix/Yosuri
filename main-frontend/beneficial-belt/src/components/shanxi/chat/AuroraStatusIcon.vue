<template>
  <span class="aurora-icon-wrapper">
    <svg class="aurora-icon" :class="state" viewBox="0 0 32 32" width="15" height="15">
      <!-- 底层：光晕/背景，负责淡入淡出 + 缩放 -->
      <circle class="aurora-layer aurora-glow" cx="16" cy="16" r="9" />

      <!-- 外层：拖尾/轨道，负责表现动感（processing 态做类 3D 翻转） -->
      <g class="aurora-layer aurora-trail">
        <line x1="10" y1="22" x2="3" y2="29" stroke-width="2" stroke-linecap="round" />
        <line x1="14.5" y1="25.5" x2="9" y2="31" stroke-width="1.4" stroke-linecap="round" />
      </g>

      <!-- 中层：核心星体（四角闪烁星形） -->
      <path
        class="aurora-layer aurora-core"
        d="M16 3 C17 10, 22 15, 29 16 C22 17, 17 22, 16 29 C15 22, 10 17, 3 16 C10 15, 15 10, 16 3 Z"
      />
    </svg>
  </span>
</template>

<script setup>
defineProps({
  // idle：静止/极轻微呼吸；processing：工作流处理中；commit：任务完成的一次性回弹反馈
  state: { type: String, default: 'idle' }
})
</script>

<style scoped>
.aurora-icon-wrapper {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  perspective: 60px; /* 给拖尾层的 rotateY 一点透视深度，做出类 3D 翻转感 */
}
.aurora-icon { overflow: visible; }
.aurora-layer { transform-origin: 16px 16px; transform-box: fill-box; }
.aurora-glow { fill: var(--app-accent); }
.aurora-core { fill: var(--app-accent); }
.aurora-trail { stroke: var(--app-accent); }

/* ==================== idle：静止或极轻微呼吸，不抢视线 ==================== */
.aurora-icon.idle .aurora-glow {
  opacity: 0.18;
  animation: aurora-glow-idle 4.5s ease-in-out infinite;
}
.aurora-icon.idle .aurora-core {
  opacity: 0.75;
  transform: scale(0.94);
}
.aurora-icon.idle .aurora-trail { opacity: 0; }

@keyframes aurora-glow-idle {
  0%, 100% { opacity: 0.14; transform: scale(0.9); }
  50% { opacity: 0.26; transform: scale(1.02); }
}

/* ==================== processing：核心偏转+旋转，光晕脉冲，拖尾类 3D 翻转 ==================== */
.aurora-icon.processing .aurora-core {
  animation: aurora-core-processing 1.1s ease-in-out infinite;
}
@keyframes aurora-core-processing {
  0%, 100% { transform: translateY(0) rotate(0deg); opacity: 0.9; }
  50% { transform: translateY(-3px) rotate(15deg); opacity: 1; }
}

.aurora-icon.processing .aurora-glow {
  animation: aurora-glow-processing 1.1s ease-in-out infinite;
}
@keyframes aurora-glow-processing {
  0%, 100% { opacity: 0.32; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.4); }
}

.aurora-icon.processing .aurora-trail {
  animation: aurora-trail-flip 1.1s ease-in-out infinite;
}
@keyframes aurora-trail-flip {
  0%, 100% { transform: rotateY(0deg); opacity: 0.35; }
  50% { transform: rotateY(180deg); opacity: 0.85; }
}

/* ==================== commit：150ms 一次性物理回弹（不循环） ==================== */
.aurora-icon.commit .aurora-core {
  animation: aurora-core-commit 0.15s cubic-bezier(0.34, 1.56, 0.64, 1) 1;
}
@keyframes aurora-core-commit {
  0% { transform: scale(1) skew(0deg); }
  55% { transform: scale(1.1) skew(-5deg); }
  100% { transform: scale(1) skew(0deg); }
}
.aurora-icon.commit .aurora-glow {
  animation: aurora-glow-commit 0.15s ease-out 1;
}
@keyframes aurora-glow-commit {
  0% { opacity: 0.3; transform: scale(1); }
  50% { opacity: 0.7; transform: scale(1.3); }
  100% { opacity: 0.18; transform: scale(0.94); }
}
.aurora-icon.commit .aurora-trail { opacity: 0; }
</style>
