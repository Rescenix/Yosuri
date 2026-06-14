<template>
  
  <div class="emotion-glow" :style="glowStyle"></div>
</template>

<script setup>
import { computed } from 'vue'
import { useEmotion } from '../composables/useEmotion.js'

const { currentEmotion } = useEmotion()

/**
 * 解析 glowColor 为 { r, g, b } 对象
 * 支持 hex (#RGB, #RRGGBB) 和 rgba(r, g, b, a) 两种格式
 */
function parseColor(glowColor) {
  // 处理 rgba 格式
  const rgbaMatch = glowColor.match(
    /rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*[\d.]+)?\s*\)/
  )
  if (rgbaMatch) {
    return {
      r: parseInt(rgbaMatch[1], 10),
      g: parseInt(rgbaMatch[2], 10),
      b: parseInt(rgbaMatch[3], 10)
    }
  }
  // 处理 hex 格式
  const hexMatch = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(glowColor)
  if (hexMatch) {
    return {
      r: parseInt(hexMatch[1], 16),
      g: parseInt(hexMatch[2], 16),
      b: parseInt(hexMatch[3], 16)
    }
  }
  // fallback 默认 calm 色
  return { r: 240, g: 160, b: 100 }
}

const glowStyle = computed(() => {
  const emo = currentEmotion.value || {}
  const glowColor = emo.glowColor || '#f0a040'
  const speed = emo.speed || 3.5
  const intensity = emo.intensity || 1.0
  const { r, g, b } = parseColor(glowColor)
  return {
    '--glow-r': r,
    '--glow-g': g,
    '--glow-b': b,
    '--glow-speed': `${speed}s`,
    '--glow-intensity': intensity
  }
})
</script>

<style scoped>

.emotion-glow {
  
  position: absolute;
  inset: -2px;
  border-radius: inherit;
  pointer-events: none;
  z-index: 0;
  box-shadow:
    0 0 8px 2px rgba(var(--glow-r), var(--glow-g), var(--glow-b), 0.3),
    0 0 20px 6px rgba(var(--glow-r), var(--glow-g), var(--glow-b), calc(0.15 * var(--glow-intensity)));
  animation: emotion-breathe var(--glow-speed) ease-in-out infinite;
}

@keyframes emotion-breathe {
  0%, 100% { opacity: 0.4; transform: scale(1); }
  50% { opacity: calc(0.8 * var(--glow-intensity)); transform: scale(1.005); }
}
</style>