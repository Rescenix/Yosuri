<template>
  <Teleport to="body">
    <Transition name="heatmap-fade">
      <div v-if="visible" class="heatmap-popup" @click.stop @click.self="close">
        <div class="heatmap-popup-header">
          <span class="heatmap-popup-title">活动热力图</span>
          <button class="heatmap-popup-close" @click="close">
            <Icon icon="mdi:close" width="16" />
          </button>
        </div>
        <div class="heatmap-popup-body">
          <div class="heatmap-popup-grid">
            <div
              v-for="(cell, i) in heatmapCells"
              :key="i"
              class="heatmap-popup-cell"
              :style="{ background: cellColor(cell.level) }"
            ></div>
          </div>
          <div class="heatmap-popup-caption" v-if="heatmapCaption">{{ heatmapCaption }}</div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({ visible: Boolean })
const emit = defineEmits(['close', 'update:visible'])

const HEATMAP_ROWS = 7
const HEATMAP_DAYS = 26 * HEATMAP_ROWS

const dailyStats = ref([])

function close() {
  emit('update:visible', false)
  emit('close')
}

async function fetchDailyStats() {
  try {
    const apiBase = import.meta.env.VITE_API_BASE || ''
    const res = await fetch(`${apiBase}/api/stats/daily?days=${HEATMAP_DAYS}`)
    if (!res.ok) throw new Error(`status ${res.status}`)
    dailyStats.value = await res.json()
  } catch (err) {
    console.error('加载热力图数据失败:', err)
  }
}

function heatmapLevelForCount(count) {
  if (count <= 0) return 0
  if (count <= 2) return 1
  if (count <= 6) return 2
  return 3
}

const heatmapCells = computed(() => {
  const data = dailyStats.value
  if (!data.length) return []
  return data.map((d, i) => ({
    c: Math.floor(i / HEATMAP_ROWS),
    r: i % HEATMAP_ROWS,
    level: heatmapLevelForCount(d.count)
  }))
})

function cellColor(level) {
  if (level === 3) return 'var(--app-accent)'
  if (level === 2) return 'color-mix(in srgb, var(--app-accent) 60%, transparent)'
  if (level === 1) return 'color-mix(in srgb, var(--app-accent) 30%, transparent)'
  return 'var(--app-surface-3)'
}

const totalTokens = computed(() => {
  const data = dailyStats.value
  if (!data.length) return 0
  return data.reduce((s, d) => s + (d.tokens || 0), 0)
})

const heatmapCaption = computed(() => {
  const tokens = totalTokens.value
  if (!tokens) return ''
  const books = [
    { title: '《局外人》', chars: 30_000 },
    { title: '《小王子》', chars: 20_000 },
    { title: '《1984》', chars: 90_000 },
  ]
  let best = books[0]
  let bestScore = Infinity
  for (const b of books) {
    const score = Math.abs(tokens / b.chars - 0.5)
    if (score < bestScore) { best = b; bestScore = score }
  }
  const copies = (tokens / best.chars).toFixed(2)
  return `这些对话消耗的 token，抵得上手抄了${best.title}的 ${copies}%。`
})

watch(() => props.visible, (v) => {
  if (v) fetchDailyStats()
})
</script>

<style scoped>
.heatmap-popup {
  position: fixed;
  bottom: 72px;
  right: 180px;
  z-index: 1000;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 14px;
  padding: 14px 16px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.18);
  min-width: 340px;
}

.heatmap-popup-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.heatmap-popup-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--app-text);
}

.heatmap-popup-close {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--app-text-faint);
  padding: 2px;
  border-radius: 4px;
}
.heatmap-popup-close:hover { background: var(--app-surface-2); }

.heatmap-popup-grid {
  display: grid;
  grid-template-columns: repeat(26, 1fr);
  gap: 2px;
}

.heatmap-popup-cell {
  aspect-ratio: 1;
  border-radius: 2px;
  width: 8px;
  height: 8px;
}

.heatmap-popup-caption {
  font-size: 10.5px;
  color: var(--app-text-faint);
  margin-top: 10px;
  text-align: center;
}

.heatmap-fade-enter-active,
.heatmap-fade-leave-active {
  transition: opacity 0.15s ease;
}
.heatmap-fade-enter-from,
.heatmap-fade-leave-to {
  opacity: 0;
}
</style>
