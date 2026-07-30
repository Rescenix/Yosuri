<template>
  <div class="duro-pet" :class="{ expanded: expanded }" @click="toggleExpanded">
    <!-- 角色形象 -->
    <img class="duro-pet-avatar" src="/src/assets/duro-pet.png" alt="Duro" draggable="false" />

    <!-- 展开面板：亲密度 & 等级 -->
    <Transition name="duro-card">
      <div v-if="expanded" class="duro-card" @click.stop>
        <div class="duro-card-head">
          <span class="duro-name">Duro</span>
          <span class="duro-level">Lv.{{ level }}</span>
        </div>
        <div class="duro-xp-bar-track">
          <div class="duro-xp-bar-fill" :style="{ width: xpPct + '%' }"></div>
        </div>
        <div class="duro-xp-label">{{ xp }} / {{ xpNext }} XP</div>
        <div class="duro-quote">{{ quote }}</div>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const expanded = ref(false)

// ── 等级数据（未来接后端，目前客户端模拟） ──
const level = ref(1)
const xp = ref(23)
const xpNext = ref(100)
const xpPct = computed(() => Math.min(100, (xp.value / xpNext.value) * 100))

const quotes = [
  '今天也想帮你写代码呢 ♪',
  '你项目的结构我都记着呢！',
  '嗯…这个 bug 我好像见过…',
  '要不要休息一下？',
  '亲密度越高，我越懂你哦 ✨',
]
const quote = ref(quotes[0])

function toggleExpanded() {
  expanded.value = !expanded.value
  if (expanded.value) {
    quote.value = quotes[Math.floor(Math.random() * quotes.length)]
  }
}
</script>

<style scoped>
.duro-pet {
  position: relative;
  width: 56px;
  height: 56px;
  flex-shrink: 0;
  cursor: pointer;
  animation: duro-float 3.6s ease-in-out infinite;
  transition: transform 0.2s ease;
  z-index: 20;
}
.duro-pet:hover {
  transform: scale(1.08);
  animation-play-state: paused;
}
.duro-pet.expanded {
  animation-play-state: paused;
}

.duro-pet-avatar {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 50%;
  border: 2px solid color-mix(in srgb, var(--app-accent) 34%, transparent);
  box-shadow: 0 2px 10px color-mix(in srgb, var(--app-accent) 16%, transparent);
  background: var(--app-surface);
  pointer-events: none;
  user-select: none;
  -webkit-user-drag: none;
}

/* 悬浮面板 */
.duro-card {
  position: absolute;
  bottom: 64px;
  right: 0;
  width: 180px;
  padding: 12px 14px;
  border-radius: 12px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  box-shadow: 0 8px 28px rgba(15, 23, 42, 0.12);
  cursor: default;
}
.duro-card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.duro-name {
  font-weight: 700;
  font-size: 13.5px;
  color: var(--app-text);
}
.duro-level {
  font-size: 11px;
  font-weight: 700;
  color: var(--app-accent);
  padding: 2px 7px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-accent) 12%, transparent);
}
.duro-xp-bar-track {
  height: 5px;
  border-radius: 3px;
  background: var(--app-surface-3);
  overflow: hidden;
  margin-bottom: 3px;
}
.duro-xp-bar-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, var(--app-accent), color-mix(in srgb, var(--app-accent) 70%, #c084fc));
  transition: width 0.5s ease;
}
.duro-xp-label {
  font-size: 9.5px;
  color: var(--app-text-faint);
  margin-bottom: 6px;
}
.duro-quote {
  font-size: 11px;
  color: var(--app-text-soft);
  line-height: 1.45;
  font-style: italic;
}

/* 浮游动画 */
@keyframes duro-float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-4px); }
}

/* 面板入场 */
.duro-card-enter-active,
.duro-card-leave-active {
  transition: opacity 0.18s ease, transform 0.2s cubic-bezier(.2,.8,.2,1);
}
.duro-card-enter-from,
.duro-card-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.92);
}
</style>