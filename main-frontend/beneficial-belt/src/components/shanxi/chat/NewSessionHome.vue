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
        <div v-if="modelUsageItems.length" class="home-model-list">
          <div v-for="m in modelUsageItems" :key="m.label" class="home-model-row">
            <div class="home-model-top">
              <span class="home-model-name">{{ m.label }}</span>
              <span class="home-model-pct">{{ m.pct }}%</span>
            </div>
            <div class="home-model-bar-track">
              <div class="home-model-bar-fill" :style="{ width: m.pct + '%' }"></div>
            </div>
          </div>
        </div>
        <div v-else class="home-model-empty">暂无模型使用数据</div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { Icon } from '@iconify/vue'
import { useAuth } from '../../../composables/useAuth.js'

const apiBase = import.meta.env.VITE_API_BASE || ''
const auth = useAuth()

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
  return currentGreeting.value.replace('{name}', auth.displayName.value)
})

// 触发图标动画的函数
// 触发图标动画的函数（兜底 DOM 查找）


onMounted(() => {
  fetchOverview()
  fetchDailyStats()

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

// 后端真实统计数据（/api/stats/overview、/api/stats/daily）
const overviewData = ref(null)
const dailyStats = ref([])

const HEATMAP_ROWS = 7
const HEATMAP_DAYS = 26 * HEATMAP_ROWS // 26 列 x 7 行，约半年的对话活动窗口

const RANGE_TO_WINDOW = { all: 'total', '30d': 'last_30d', '7d': 'last_7d' }

async function fetchOverview() {
  try {
    const res = await fetch(`${apiBase}/api/stats/overview`)
    if (!res.ok) throw new Error(`status ${res.status}`)
    overviewData.value = await res.json()
  } catch (err) {
    console.error('加载统计总览失败:', err)
  }
}

async function fetchDailyStats() {
  try {
    const res = await fetch(`${apiBase}/api/stats/daily?days=${HEATMAP_DAYS}`)
    if (!res.ok) throw new Error(`status ${res.status}`)
    dailyStats.value = await res.json()
  } catch (err) {
    console.error('加载每日活动数据失败:', err)
  }
}

function formatCount(n) {
  return (n ?? 0).toLocaleString('zh-CN')
}

function formatTokens(n) {
  const v = n ?? 0
  if (v >= 1_000_000) return (v / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (v >= 1_000) return (v / 1_000).toFixed(1).replace(/\.0$/, '') + 'K'
  return String(v)
}

function heatmapLevelColor(level) {
  if (level === 3) return 'var(--app-accent)'
  if (level === 2) return 'color-mix(in srgb, var(--app-accent) 60%, transparent)'
  if (level === 1) return 'color-mix(in srgb, var(--app-accent) 30%, transparent)'
  return 'var(--app-surface-3)'
}

// 当前所选时间窗口（总共 / 30 天 / 7 天）对应的后端聚合数据
const currentWindow = computed(() => {
  const key = RANGE_TO_WINDOW[homeRange.value] || 'total'
  return overviewData.value ? overviewData.value[key] : null
})

const statsGridItems = computed(() => {
  const w = currentWindow.value
  if (!w) {
    return [
      { label: '会话数', value: '-' },
      { label: '消息数', value: '-' },
      { label: '总 Token 数', value: '-' },
      { label: '活跃天数', value: '-' },
      { label: '当前连续', value: '-' },
      { label: '最长连续', value: '-' },
      { label: '高峰时段', value: '-' },
      { label: '常用模型', value: '-' }
    ]
  }
  return [
    { label: '会话数', value: formatCount(w.total_sessions) },
    { label: '消息数', value: formatCount(w.total_messages) },
    { label: '总 Token 数', value: formatTokens(w.total_tokens) },
    { label: '活跃天数', value: `${w.active_days} 天` },
    { label: '当前连续', value: `${w.current_streak} 天` },
    { label: '最长连续', value: `${w.longest_streak} 天` },
    { label: '高峰时段', value: w.peak_hour },
    { label: '常用模型', value: w.favorite_model }
  ]
})

const modelUsageItems = computed(() => {
  const tokens = currentWindow.value?.model_tokens || []
  const total = tokens.reduce((sum, m) => sum + m.tokens, 0)
  if (total <= 0) return []
  return tokens.map(m => ({
    label: m.model,
    pct: Math.round((m.tokens / total) * 100)
  }))
})

// 按绝对消息数分档，而不是相对当前窗口内的最大值——
// 否则数据稀疏时（比如只活跃过一天），那一天会永远被判成"最深"档
function heatmapLevelForCount(count) {
  if (count <= 0) return 0
  if (count <= 2) return 1
  if (count <= 6) return 2
  return 3
}

// 按日历周排布成 26 列 x 7 行的热力图格子
const heatmapCells = computed(() => {
  const data = dailyStats.value
  if (!data.length) return []
  return data.map((d, i) => ({
    c: Math.floor(i / HEATMAP_ROWS),
    r: i % HEATMAP_ROWS,
    level: heatmapLevelForCount(d.count)
  }))
})

// 世界名著大致字数（用于将 token 消耗量换算为"手抄X遍"的趣味类比）
const CLASSIC_BOOKS = [
  { title: '《局外人》', author: '加缪', chars: 30_000 },
  { title: '《查拉图斯特拉如是说》', author: '尼采', chars: 70_000 },
  { title: '《小王子》', author: '圣埃克苏佩里', chars: 20_000 },
  { title: '《动物农场》', author: '奥威尔', chars: 30_000 },
  { title: '《1984》', author: '奥威尔', chars: 90_000 },
  { title: '《了不起的盖茨比》', author: '菲茨杰拉德', chars: 50_000 },
  { title: '《老人与海》', author: '海明威', chars: 27_000 },
  { title: '《瓦尔登湖》', author: '梭罗', chars: 75_000 },
]

// 根据 token 总量选取最合适的书，使 "copies" 尽量落在 0.01~9 之间
function pickBook(tokenCount) {
  // 优先选 copies 在 0.1~5 范围内的书；若全部不在则取最接近 1 的
  let best = CLASSIC_BOOKS[0]
  let bestScore = Infinity
  for (const book of CLASSIC_BOOKS) {
    const ratio = tokenCount / book.chars
    // 理想区间 [0.1, 5] 内得分越低越好；区间外用距离惩罚
    const inRange = ratio >= 0.1 && ratio <= 5
    const score = inRange ? Math.abs(Math.log2(ratio)) : 100 + Math.abs(Math.log2(ratio))
    if (score < bestScore) {
      bestScore = score
      best = book
    }
  }
  return best
}

const heatmapCaption = computed(() => {
  const totalTokens = dailyStats.value.reduce((sum, d) => sum + (d.tokens || 0), 0)
  if (totalTokens <= 0) return '最近还没有对话记录，开始聊点什么吧。'

  const book = pickBook(totalTokens)
  const copies = totalTokens / book.chars
  if (copies >= 1) {
    const copiesText = copies >= 10 ? copies.toFixed(0) : copies.toFixed(1)
    return `这些对话消耗的 token，抵得上手抄 ${copiesText} 遍${book.title}。`
  }
  const percent = copies * 100
  const percentText = percent < 0.01 ? '不到 0.01' : percent.toFixed(2)
  return `这些对话消耗的 token，抵得上手抄了${book.title}的 ${percentText}%。`
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
  color: var(--app-text);
}

.home-stats-card {
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: var(--app-surface-2);
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
  color: var(--app-text-faint);
  cursor: pointer;
}
.home-tab.active { font-weight: 700; color: var(--app-text); }

.home-range-group {
  display: flex;
  gap: 2px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 2px;
}
.home-range-btn {
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--app-text-faint);
  cursor: pointer;
}
.home-range-btn.active { background: var(--app-surface); color: var(--app-text); }

.home-stats-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: 12px 16px;
  margin-bottom: 14px;
}
.home-stat-label { font-size: 11px; color: var(--app-text-faint); margin-bottom: 2px; }
.home-stat-value { font-size: 15.5px; font-weight: 700; color: var(--app-text); }

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
.home-heatmap-caption { font-size: 11px; color: var(--app-text-faint); }

.home-model-empty { font-size: 12.5px; color: var(--app-text-faint); padding: 8px 0; }
.home-model-list { display: flex; flex-direction: column; gap: 14px; }
.home-model-row { display: flex; flex-direction: column; gap: 6px; }
.home-model-top { display: flex; justify-content: space-between; gap: 10px; }
.home-model-name {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--app-text);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.home-model-pct {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  color: var(--app-text-faint);
  white-space: nowrap;
  flex-shrink: 0;
}
.home-model-bar-track {
  height: 6px;
  border-radius: 3px;
  background: var(--app-surface-2);
  overflow: hidden;
}
.home-model-bar-fill {
  height: 100%;
  border-radius: 3px;
  background: var(--app-accent);
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

/* 2. 图标：默认跟随主题色，不再强制硬编码 */
.greeting-icon {
  color: var(--app-accent);
  transition: transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1), filter 0.4s ease;
  transform-origin: center;
}

/* 图标闪烁发光状态 */
.greeting-icon.active {
  transform: rotate(15deg) scale(1.1);
  /* 纯粹的发光阴影，不会覆盖或改变图标原来的颜色 */
  filter: drop-shadow(0 0 6px color-mix(in srgb, var(--app-accent) 50%, transparent));
}
</style>
