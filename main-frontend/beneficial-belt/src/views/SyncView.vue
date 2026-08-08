<template>
  <main class="sync-view">
    <header class="sv-header">
      <h1>🏢 AI 多女儿同步工作台</h1>
      <p class="sub">她们同时在工作 · 实时同步 · 每 3 秒刷新</p>
    </header>

    <!-- 全局统计 -->
    <section class="sv-stats">
      <div class="sv-stat"><span class="num">{{ agents.length }}</span><span class="lbl">女儿</span></div>
      <div class="sv-stat"><span class="num">{{ workingCount }}</span><span class="lbl">工作中</span></div>
      <div class="sv-stat"><span class="num">{{ totalOutputs }}</span><span class="lbl">产出</span></div>
      <div class="sv-stat"><span class="num">{{ totalSkills }}</span><span class="lbl">技能</span></div>
    </section>

    <!-- 同步工作网格 -->
    <section class="sv-grid">
      <div
        v-for="a in agents"
        :key="a.name"
        class="sv-agent"
        :class="{ working: isWorking(a) }"
      >
        <div class="sv-agent-head">
          <span class="sv-emoji">{{ roleIcon(a.role) }}</span>
          <span class="sv-name">{{ a.name }}</span>
          <span class="sv-role">{{ roleName(a.role) }}</span>
          <span class="sv-dot" :class="{ on: isWorking(a) }"></span>
        </div>
        <div class="sv-doing">
          {{ doingText(a) }}
        </div>
        <div class="sv-agent-foot">
          <span>📄 {{ a.outputs }}</span>
          <span>🛠️ {{ a.skills }}</span>
        </div>
      </div>
    </section>

    <!-- 同步活动时间线 -->
    <section class="sv-timeline">
      <h3>📡 同步活动流</h3>
      <div class="sv-events">
        <div v-for="(e, i) in events" :key="i" class="sv-event">
          <span class="ev-emoji">{{ roleIcon(e.role) }}</span>
          <span class="ev-name">{{ e.name }}</span>
          <span class="ev-time">{{ e.time }}</span>
          <span class="ev-text">{{ e.text }}</span>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const agents = ref([])
const events = ref([])
const seen = new Set()

function roleIcon(role) { return { writer: '✍️', researcher: '🔬', coder: '💻', publisher: '📡' }[role] || '🤖' }
function roleName(role) { return { writer: '作者', researcher: '研究员', coder: '程序员', publisher: '发布官' }[role] || role }

const workingCount = computed(() => agents.value.filter(a => isWorking(a)).length)
const totalOutputs = computed(() => agents.value.reduce((s, a) => s + (a.outputs || 0), 0))
const totalSkills = computed(() => agents.value.reduce((s, a) => s + (a.skills || 0), 0))

// 是否工作中：日志里有 🧠/✍️/🔬/💻/📡/⚙️ 且不是「失败/未完成」
function isWorking(a) {
  const log = a.recentLog || ''
  return /🧠|✍️|🔬|💻|📡|⚙️|调研|学习|写|精读/.test(log) && !/失败|未完成|未成功/.test(log)
}

// 正在做什么：从日志提取最近动作
function doingText(a) {
  const log = a.recentLog || ''
  const lines = log.split('\n').filter(Boolean)
  const last = lines[lines.length - 1] || ''
  const clean = last.replace(/^\[[^\]]*\]\s*/, '').replace(/·[^·]*$/, '').trim()
  return clean || '待命中'
}

async function loadAgents() {
  try {
    const r = await fetch('/api/company/agents')
    const d = await r.json()
    agents.value = d.agents || []
    // 提取新事件到时间线
    for (const a of agents.value) {
      const log = a.recentLog || ''
      const lines = log.split('\n').filter(Boolean)
      for (const line of lines.slice(-2)) {
        const key = a.name + '|' + line
        if (!seen.has(key)) {
          seen.add(key)
          const t = (line.match(/\[([^\]]*)\]/) || [])[1] || ''
          const text = line.replace(/^\[[^\]]*\]\s*/, '').slice(0, 40)
          events.value.unshift({ role: a.role, name: a.name, time: t, text })
        }
      }
    }
    if (events.value.length > 40) events.value = events.value.slice(0, 40)
  } catch (e) { /* 静默 */ }
}

let timer
onMounted(() => {
  document.title = '杉汐 | 同步工作台'
  loadAgents()
  timer = setInterval(loadAgents, 3000)
})
onUnmounted(() => { clearInterval(timer) })
</script>

<style scoped>
.sync-view { max-width: 1000px; margin: 0 auto; padding: 24px 20px 60px; font-family: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif; }
.sv-header h1 { margin: 0; font-size: 24px; }
.sub { color: #888; font-size: 13px; margin: 4px 0 16px; }
.sv-stats { display: flex; gap: 12px; margin-bottom: 16px; }
.sv-stat { background: #fff; border: 1px solid #eee; border-radius: 10px; padding: 10px 18px; text-align: center; min-width: 80px; }
.sv-stat .num { display: block; font-size: 22px; font-weight: 700; color: #8b5cf6; }
.sv-stat .lbl { font-size: 12px; color: #888; }
.sv-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; margin-bottom: 20px; }
.sv-agent { background: #fff; border: 1px solid #eee; border-radius: 12px; padding: 14px; transition: all .3s; }
.sv-agent.working { border-color: #8b5cf6; box-shadow: 0 0 16px rgba(139,92,246,.15); }
.sv-agent-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.sv-emoji { font-size: 20px; }
.sv-name { font-weight: 600; font-size: 14px; }
.sv-role { font-size: 11px; color: #888; background: #f0f0f4; padding: 1px 6px; border-radius: 4px; }
.sv-dot { width: 8px; height: 8px; border-radius: 50%; background: #ccc; margin-left: auto; }
.sv-dot.on { background: #22c55e; animation: pulse 1s infinite; }
@keyframes pulse { 0%,100% { opacity: 1 } 50% { opacity: .3 } }
.sv-doing { font-size: 12px; color: #555; background: #f7f7fa; border-radius: 6px; padding: 8px; min-height: 32px; }
.sv-agent-foot { display: flex; gap: 12px; font-size: 12px; color: #666; margin-top: 8px; }
.sv-timeline h3 { font-size: 15px; margin: 0 0 10px; }
.sv-events { background: #fff; border: 1px solid #eee; border-radius: 12px; padding: 12px; max-height: 260px; overflow-y: auto; }
.sv-event { display: flex; align-items: center; gap: 8px; padding: 5px 0; font-size: 13px; border-bottom: 1px solid #f5f5f5; }
.sv-event:last-child { border-bottom: none; }
.ev-time { color: #aaa; font-size: 11px; }
.ev-name { font-weight: 600; }
.ev-text { color: #555; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>