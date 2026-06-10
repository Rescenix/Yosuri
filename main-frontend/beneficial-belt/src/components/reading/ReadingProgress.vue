<template>
  <div class="progress-panel">
    <div class="current-book" v-if="bookTitle">
      <h4>{{ bookTitle }}</h4>
      <div v-if="totalPages <= 1" class="loading-hint">排版中…</div>
      <template v-else>
        <div class="circle-wrap">
          <svg class="circle" width="100" height="100">
            <circle cx="50" cy="50" r="45" fill="none" stroke="#e2e8f0" stroke-width="8" />
            <circle
              cx="50" cy="50" r="45"
              fill="none"
              stroke="#3b82f6"
              stroke-width="8"
              stroke-linecap="round"
              :stroke-dasharray="circumference"
              :stroke-dashoffset="progressOffset"
              transform="rotate(-90 50 50)"
            />
          </svg>
          <div class="percent-text">{{ percent }}%</div>
        </div>
        <div class="page-info">已读 {{ currentPage + 1 }} / {{ totalPages }} 页</div>
      </template>
    </div>
    <div class="history">
      <h5>近7日阅读记录</h5>
      <div class="bar-chart" v-if="last7Days.length > 0">
        <div v-for="day in last7Days" :key="day.date" class="bar-col">
          <div class="bar-label">{{ day.label }}</div>
          <div class="bar-container">
            <div class="bar" :style="{ height: day.height + '%' }"></div>
          </div>
          <div class="bar-pages">{{ day.pages }}p</div>
        </div>
      </div>
      <div v-else class="empty">暂无记录，开始阅读吧</div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  bookTitle: String,
  totalPages: Number,
  currentPage: Number,
})

// 正文页数（排除封面封底）
const bodyPages = computed(() => Math.max(0, (props.totalPages || 0) - 2))

const readPages = computed(() => Math.max(0, props.currentPage || 0))

const percent = computed(() => {
  if (bodyPages.value <= 0) return 0
  return Math.min(100, Math.round((readPages.value / bodyPages.value) * 100))
})

const circumference = 2 * Math.PI * 45
const progressOffset = computed(() => circumference * (1 - percent.value / 100))

const last7Days = ref([])

function loadHistory() {
  const stored = localStorage.getItem('shanxi_reading_progress')
  let records = stored ? JSON.parse(stored) : []
  const result = []
  let maxPages = 1
  for (let i = 6; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    const dateStr = d.toLocaleDateString()
    const found = records.find(r => r.date === dateStr)
    maxPages = Math.max(maxPages, found ? found.pages : 0)
  }
  for (let i = 6; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    const dateStr = d.toLocaleDateString()
    const found = records.find(r => r.date === dateStr)
    const pages = found ? found.pages : 0
    result.push({
      date: dateStr,
      label: d.getDate() + '日',
      pages,
      height: maxPages > 0 ? (pages / maxPages) * 100 : 0,
    })
  }
  last7Days.value = result
}

onMounted(loadHistory)
watch(() => props.currentPage, () => loadHistory())
</script>

<style scoped>
.progress-panel { padding: 12px; }
.current-book { text-align: center; margin-bottom: 20px; }
.current-book h4 { margin: 0 0 12px; font-size: 1rem; }
.loading-hint { color: #94a3b8; font-size: 0.9rem; margin: 20px 0; }
.circle-wrap { position: relative; display: inline-block; }
.circle { display: block; }
.percent-text { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); font-size: 1.2rem; font-weight: bold; color: #3b82f6; }
.page-info { margin-top: 8px; font-size: 0.85rem; color: #64748b; }
.history h5 { margin: 0 0 12px; font-size: 0.9rem; color: #334155; }
.bar-chart { display: flex; justify-content: space-between; align-items: flex-end; }
.bar-col { display: flex; flex-direction: column; align-items: center; width: 12%; }
.bar-label { font-size: 0.7rem; color: #64748b; margin-bottom: 4px; }
.bar-container { width: 100%; height: 80px; background: #f1f5f9; border-radius: 4px; position: relative; }
.bar { position: absolute; bottom: 0; width: 100%; background: #3b82f6; border-radius: 4px; transition: height 0.3s; }
.bar-pages { font-size: 0.65rem; color: #64748b; margin-top: 4px; }
.empty { text-align: center; color: #94a3b8; margin-top: 20px; }
</style>