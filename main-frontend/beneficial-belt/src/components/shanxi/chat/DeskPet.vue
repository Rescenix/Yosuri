<template>
  <div
    class="deskpet"
    :class="{ dragging, expanded }"
    :style="petStyle"
    @mousedown.prevent="startDrag"
  >
    <!-- 角色本体 -->
    <div class="deskpet-body">
      <img class="deskpet-avatar" src="/src/assets/duro-pet.png" alt="Duro" draggable="false" />
      <!-- 等级徽章 -->
      <span class="deskpet-level-badge">Lv.{{ level }}</span>
    </div>

    <!-- 互动面板 -->
    <Transition name="pet-card">
      <div v-if="expanded" class="deskpet-card" @mousedown.stop @click.stop>
        <div class="pet-card-head">
          <span class="pet-card-name">Duro</span>
          <span class="pet-card-lv">Lv.{{ level }}</span>
        </div>
        <div class="pet-card-xp-track">
          <div class="pet-card-xp-fill" :style="{ width: xpPct + '%' }"></div>
        </div>
        <div class="pet-card-xp-label">{{ xp }} / {{ xpNext }} XP</div>
        <div class="pet-card-quote">{{ quote }}</div>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const expanded = ref(false)
const dragging = ref(false)
const x = ref(24)
const y = ref(120)
const dragOffset = { x: 0, y: 0 }

// ── 等级 ──
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

const petStyle = computed(() => ({
  left: x.value + 'px',
  top: y.value + 'px',
  cursor: dragging.value ? 'grabbing' : 'grab',
}))

// ── 拖拽逻辑 ──
function startDrag(e) {
  dragging.value = true
  expanded.value = false
  dragOffset.x = e.clientX - x.value
  dragOffset.y = e.clientY - y.value
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
}

function onDrag(e) {
  if (!dragging.value) return
  const maxX = window.innerWidth - 64
  const maxY = window.innerHeight - 76
  x.value = Math.max(0, Math.min(e.clientX - dragOffset.x, maxX))
  y.value = Math.max(0, Math.min(e.clientY - dragOffset.y, maxY))
}

function stopDrag() {
  if (!dragging.value) return
  dragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
}

// ── 点击切换面板 ──
function toggleExpanded() {
  if (dragging.value) return
  x.value = Math.max(0, Math.min(x.value, window.innerWidth - 180))
  expanded.value = !expanded.value
  if (expanded.value) {
    quote.value = quotes[Math.floor(Math.random() * quotes.length)]
  }
}

// ── 空闲时小飘动画 ──
let floatTimer = null
function idleFloat() {
  if (dragging.value || expanded.value) return
  const sway = Math.sin(Date.now() / 600) * 3
  document.querySelector('.deskpet-body')?.style.setProperty('--float-y', sway + 'px')
  floatTimer = requestAnimationFrame(idleFloat)
}

onMounted(() => {
  floatTimer = requestAnimationFrame(idleFloat)
})

onUnmounted(() => {
  cancelAnimationFrame(floatTimer)
})
</script>

<style scoped>
.deskpet {
  position: fixed;
  z-index: 99999;
  width: 64px;
  height: 76px;
  user-select: none;
  touch-action: none;
  transition: filter 0.2s ease;
}
.deskpet:hover {
  filter: brightness(1.08);
}
.deskpet.dragging {
  transition: none;
  filter: brightness(1.12);
}

/* ── 角色 ── */
.deskpet-body {
  position: relative;
  width: 64px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  transform: translateY(var(--float-y, 0));
  transition: transform 0.15s ease-out;
}
.deskpet-avatar {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid color-mix(in srgb, var(--app-accent) 38%, transparent);
  box-shadow:
    0 3px 12px color-mix(in srgb, var(--app-accent) 18%, transparent),
    0 0 0 1px color-mix(in srgb, var(--app-accent) 10%, transparent);
  background: var(--app-surface);
  pointer-events: none;
  -webkit-user-drag: none;
}
.deskpet:hover .deskpet-avatar {
  border-color: color-mix(in srgb, var(--app-accent) 52%, transparent);
}
.deskpet-level-badge {
  position: absolute;
  bottom: -2px;
  right: -4px;
  padding: 1px 6px;
  font-size: 10px;
  font-weight: 800;
  color: #fff;
  background: linear-gradient(135deg, var(--app-accent), color-mix(in srgb, var(--app-accent) 60%, #c084fc));
  border-radius: 8px;
  border: 1.5px solid var(--app-surface);
  line-height: 1.4;
  pointer-events: none;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.12);
}

/* ── 信息卡片 ── */
.deskpet-card {
  position: absolute;
  left: 72px;
  top: 0;
  width: 180px;
  padding: 12px 14px;
  border-radius: 12px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  box-shadow: 0 8px 32px rgba(15, 23, 42, 0.14);
}
.pet-card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.pet-card-name {
  font-weight: 700;
  font-size: 13.5px;
  color: var(--app-text);
}
.pet-card-lv {
  font-size: 11px;
  font-weight: 700;
  color: var(--app-accent);
  padding: 2px 7px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-accent) 12%, transparent);
}
.pet-card-xp-track {
  height: 5px;
  border-radius: 3px;
  background: var(--app-surface-3);
  overflow: hidden;
  margin-bottom: 3px;
}
.pet-card-xp-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, var(--app-accent), color-mix(in srgb, var(--app-accent) 70%, #c084fc));
  transition: width 0.5s ease;
}
.pet-card-xp-label {
  font-size: 9.5px;
  color: var(--app-text-faint);
  margin-bottom: 6px;
}
.pet-card-quote {
  font-size: 11px;
  color: var(--app-text-soft);
  line-height: 1.45;
  font-style: italic;
}

/* ── 过渡 ── */
.pet-card-enter-active,
.pet-card-leave-active {
  transition: opacity 0.18s ease, transform 0.2s cubic-bezier(.2,.8,.2,1);
}
.pet-card-enter-from,
.pet-card-leave-to {
  opacity: 0;
  transform: translateX(-6px) scale(0.92);
}
</style>