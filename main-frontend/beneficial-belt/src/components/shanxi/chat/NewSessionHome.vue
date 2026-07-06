<template>
  <div class="session-home">
    <div class="home-greeting">
      <!-- 给图标加一个专门的 ref，方便我们在 JS 里操控它 -->
      <Icon ref="greetingIconRef" icon="majesticons:shooting-star-line" width="22" class="greeting-icon" />
      
      <!-- 加入 Vue 的 Transition 过渡组件，负责文字的上下浮入淡出 -->
      <Transition name="slide-fade" mode="out-in">
        <span class="home-greeting-text" :key="currentGreeting">{{ displayGreeting }}</span>
      </Transition>
    </div>
    <div class="home-stats-card">
      <div class="home-stats-header">
        <div class="home-tabs">
          <span class="home-tab" :class="{ active: homeTab === 'overview' }" @click="homeTab = 'overview'">总览</span>
          <span class="home-tab" :class="{ active: homeTab === 'models' }" @click="homeTab = 'models'">模型</span>
        </div>
        <div class="home-range-group">
          <span
            v-for="opt in HOME_RANGES"
            :key="opt.value"
            class="home-range-btn"
            :class="{ active: homeRange === opt.value }"
            @click="homeRange = opt.value"
          >{{ opt.label }}</span>
        </div>
      </div>

      <template v-if="homeTab === 'overview'">
        <div class="home-stats-grid">
          <div v-for="item in statsGridItems" :key="item.label" class="home-stat-cell">
            <div class="home-stat-label">{{ item.label }}</div>
            <div class="home-stat-value">{{ item.value }}</div>
          </div>
        </div>
        <div class="home-heatmap">
          <div
            v-for="(cell, i) in heatmapCells"
            :key="i"
            class="home-heatmap-cell"
            :style="{ gridColumn: cell.c + 1, gridRow: cell.r + 1, background: heatmapLevelColor(cell.level) }"
          ></div>
        </div>
        <div class="home-heatmap-caption">{{ heatmapCaption }}</div>
      </template>

      <template v-else>
        <div class="home-model-list">
          <div v-for="m in MODEL_USAGE" :key="m.label" class="home-model-row">
            <div class="home-model-top">
              <span class="home-model-name">{{ m.label }}</span>
              <span class="home-model-pct">{{ m.pct }}%</span>
            </div>
            <div class="home-model-bar-track">
              <div class="home-model-bar-fill" :style="{ width: m.pct + '%' }"></div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'


const props = defineProps({
  userName: { type: String, default: 'Prometheus' }
})

const greetingMessages = [
  "接下来做什么，{name}？",
  "欢迎回家，{name}！",
  "今天想重构什么，{name}？",
  "你好，{name}，一切就绪。",
  "准备开始了吗，{name}？",
  "PrismD 已就绪，{name}。"
]

const currentGreeting = ref(greetingMessages[0])
const greetingIconRef = ref(null) // 用来获取图标的 DOM

const displayGreeting = computed(() => {
  return currentGreeting.value.replace('{name}', props.userName)
})

// 触发图标动画的函数
// 触发图标动画的函数（兜底 DOM 查找）


onMounted(() => {
  // 确保初始化前 DOM 已存在
  setTimeout(() => {
    const randomIndex = Math.floor(Math.random() * greetingMessages.length)
    currentGreeting.value = greetingMessages[randomIndex]
  
  }, 100)

  // 每隔 20 秒切换，并带动画
  setInterval(async () => {
    const nextIndex = Math.floor(Math.random() * greetingMessages.length)
    currentGreeting.value = greetingMessages[nextIndex]
    // 等待 Vue 更新 DOM 后再触发图标特效
    await nextTick()
  
  }, 20000)
})
const homeTab = ref('overview')
const homeRange = ref('all')

const HOME_RANGES = [
  { value: 'all', label: '全部' },
  { value: '30d', label: '30 天' },
  { value: '7d', label: '7 天' }
]

// 仅作占位的演示数据，接入真实用量接口时替换，结构不变
const ACTIVITY_STATS = {
  all: { sessions: 32, messages: '6,947', tokens: '9.8M', activeDays: '5 天', currentStreak: '0 天', longestStreak: '4 天', peakHour: '凌晨 1 点', favoriteModel: 'DS 官方' },
  '30d': { sessions: 21, messages: '4,380', tokens: '6.1M', activeDays: '5 天', currentStreak: '0 天', longestStreak: '4 天', peakHour: '凌晨 1 点', favoriteModel: 'DS 官方' },
  '7d': { sessions: 6, messages: '980', tokens: '1.4M', activeDays: '3 天', currentStreak: '0 天', longestStreak: '2 天', peakHour: '晚上 11 点', favoriteModel: 'Cloud 480B' }
}

const MODEL_USAGE = [
  { label: 'DS 官方', pct: 46 },
  { label: 'Cloud 480B', pct: 28 },
  { label: '本地 7B', pct: 18 },
  { label: 'DS 浏览器', pct: 8 }
]

function buildHeatmap() {
  const cols = 26, rows = 7
  const active = { '25,2': 2, '25,3': 3, '25,4': 2, '25,5': 1, '24,3': 1, '20,4': 1, '14,2': 1, '9,5': 1 }
  const cells = []
  for (let c = 0; c < cols; c++) {
    for (let r = 0; r < rows; r++) {
      cells.push({ c, r, level: active[c + ',' + r] || 0 })
    }
  }
  return cells
}
const heatmapCells = buildHeatmap()
const heatmapCaption = '这些对话消耗的 token，抵得上手抄 12 遍《逆天邪神》全本。'

function heatmapLevelColor(level) {
  if (level === 3) return '#c96442'
  if (level === 2) return 'rgba(201, 100, 66, 0.6)'
  if (level === 1) return 'rgba(201, 100, 66, 0.3)'
  return '#ececec'
}

const statsGridItems = computed(() => {
  const s = ACTIVITY_STATS[homeRange.value] || ACTIVITY_STATS.all
  return [
    { label: '会话数', value: s.sessions },
    { label: '消息数', value: s.messages },
    { label: '总 Token 数', value: s.tokens },
    { label: '活跃天数', value: s.activeDays },
    { label: '当前连续', value: s.currentStreak },
    { label: '最长连续', value: s.longestStreak },
    { label: '高峰时段', value: s.peakHour },
    { label: '常用模型', value: s.favoriteModel }
  ]
})
</script>

<style scoped>
.session-home {
  max-width: 640px;
  margin: 28px auto 0;
  padding: 0 24px;
  font-family: "Inter", system-ui, sans-serif;
  transform: scale(0.88);
}

.home-greeting {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}
.home-greeting-text {
  font-size: 24px;
  font-weight: 600;
  color: #1a1a1a;
}

.home-stats-card {
  border: 1px solid #e5e5e5;
  border-radius: 14px;
  background: #fafafa;
  padding: 14px 18px;
  
  /* ✅ 核心修复：固定卡片的最小高度 */
  min-height: 330px; 
  
  /* ✅ 加这两行，确保卡片内部的模型列表能撑满或顶对齐 */
  display: flex;
  flex-direction: column;
}

.home-stats-header {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}
.home-tabs { display: flex; gap: 14px; flex: 1; }
.home-tab {
  font-size: 13px;
  font-weight: 500;
  color: #a3a3a3;
  cursor: pointer;
}
.home-tab.active { font-weight: 700; color: #1a1a1a; }

.home-range-group {
  display: flex;
  gap: 2px;
  background: #f5f5f5;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  padding: 2px;
}
.home-range-btn {
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11.5px;
  font-weight: 600;
  color: #a3a3a3;
  cursor: pointer;
}
.home-range-btn.active { background: #ffffff; color: #1a1a1a; }

.home-stats-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: 12px 16px;
  margin-bottom: 14px;
}
.home-stat-label { font-size: 11px; color: #a3a3a3; margin-bottom: 2px; }
.home-stat-value { font-size: 15.5px; font-weight: 700; color: #1a1a1a; }

/* 热力图不再用 aspect-ratio 跟宽度联动——26 列平铺时格子宽度本来就有 ~20px，
   如果高度也跟着变成正方形，整块热力图会偏高，首页超出可视高度出现滚动条。
   固定一个矮一些的格子高度，横向依然铺满 26 列 */
/* 热力图改为标准的网格正方形 */
.home-heatmap {
  display: grid;
  grid-template-columns: repeat(26, 1fr); /* 保持26列 */
  grid-auto-rows: 1fr; /* 关键：让行高自动跟随列宽 */
  gap: 3px;             /* 略微拉开间距，更像 Claude 的质感 */
  margin-bottom: 8px;
}

.home-heatmap-cell {
  border-radius: 3px;
  aspect-ratio: 1 / 1; /* 强制每一个格子都是标准正方形 */
}
.home-heatmap-caption { font-size: 11px; color: #a3a3a3; }

.home-model-list { display: flex; flex-direction: column; gap: 14px; }
.home-model-row { display: flex; flex-direction: column; gap: 6px; }
.home-model-top { display: flex; justify-content: space-between; gap: 10px; }
.home-model-name {
  font-size: 12.5px;
  font-weight: 600;
  color: #1a1a1a;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.home-model-pct {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  color: #a3a3a3;
  white-space: nowrap;
  flex-shrink: 0;
}
.home-model-bar-track {
  height: 6px;
  border-radius: 3px;
  background: #f5f5f5;
  overflow: hidden;
}
.home-model-bar-fill {
  height: 100%;
  border-radius: 3px;
  background: #c96442;
}

/* ================== 新加入的炫酷入场动画 ================== */
/* 1. 文字：上升+淡入动画 */
.slide-fade-enter-active,
.slide-fade-leave-active {
  transition: all 0.4s cubic-bezier(0.25, 1, 0.5, 1);
}
.slide-fade-enter-from {
  transform: translateY(20px);
  opacity: 0;
}
.slide-fade-leave-to {
  transform: translateY(-20px);
  opacity: 0;
}

/* 2. 图标：默认状态锁定颜色防止变黑 */
.greeting-icon {
  color: #c96442 !important; /* 强制指定橘红色，绝不再黑 */
  transition: transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1), filter 0.4s ease;
  transform-origin: center;
}

/* 图标闪烁发光状态 */
.greeting-icon.active {
  transform: rotate(15deg) scale(1.1);
  /* 纯粹的发光阴影，不会覆盖或改变图标原来的颜色 */
  filter: drop-shadow(0 0 6px rgba(201, 100, 66, 0.5));
}
</style>
