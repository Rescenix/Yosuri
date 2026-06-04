<template>
  <div class="pulse-wrapper">
    <div class="pulse-ring" :style="pulseStyle" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps({
  color: {
    type: String,
    default: 'rgba(96, 165, 250, 0.15)'
  }
})

const pulseStyle = computed(() => ({
  background: `radial-gradient(circle, ${props.color} 0%, transparent 70%)`,
  boxShadow: `0 0 20px ${props.color.replace('0.15', '0.3')}, 0 0 40px ${props.color.replace('0.15', '0.1')}`
}))
</script>

<style scoped>
.pulse-wrapper {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.pulse-ring {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  animation: heartbeat 2s ease-in-out infinite;
}

@keyframes heartbeat {
  0%, 100% {
    transform: scale(1);
    opacity: 0.6;
  }
  50% {
    transform: scale(1.2);
    opacity: 1;
  }
}
</style>