<template>
  <main class="sync-view">
    <div class="bg-stars">
      <span v-for="(s, i) in stars" :key="i" class="star" :style="{ left: s.x + '%', top: s.y + '%', animationDelay: s.d + 's' }"></span>
    </div>

    <header class="sv-header">
      <div class="sv-title-row">
        <h1>👥 AI 公司 · <span class="grad-text">部门协同工作台</span></h1>
        <div class="sv-live" :class="{ on: anyWorking }">
          <span class="live-dot"></span> LIVE · {{ workingCount }}/{{ agents.length }} 工作中
        </div>
      </div>
      <p class="sub">分部门协作 · 实时同步 · 产出可见</p>
    </header>

    <!-- 全局统计 -->
    <section class="sv-stats">
      <div class="sv-stat"><div class="sv-stat-num grad-text">{{ agents.length }}</div><div class="sv-stat-lbl">AI 员工</div></div>
      <div class="sv-stat"><div class="sv-stat-num grad2-text">{{ workingCount }}</div><div class="sv-stat-lbl">正在工作</div></div>
      <div class="sv-stat"><div class="sv-stat-num grad3-text">{{ totalOutputs }}</div><div class="sv-stat-lbl">产出文档</div></div>
      <div class="sv-stat"><div class="sv-stat-num grad-text">{{ totalSkills }}</div><div class="sv-stat-lbl">掌握技能</div></div>
    </section>

    <!-- 按部门分组 -->
    <section v-for="dept in departments" :key="dept.key" class="sv-dept">
      <div class="sv-dept-head">
        <span class="sv-dept-icon">{{ dept.icon }}</span>
        <span class="sv-dept-name grad-text">{{ dept.name }}</span>
        <span class="sv-dept-count">{{ dept.agents.length }} 人</span>
        <span v-if="dept.working" class="sv-dept-working">工作中</span>
      </div>
      <div class="sv-grid">
        <div
          v-for="(a, i) in dept.agents"
          :key="a.name"
          class="sv-agent"
          :class="{ working: isWorking(a) }"
          :style="{ animationDelay: (i * 0.08) + 's' }"
        >
          <div class="sv-agent-head">
            <div class="sv-agent-avatar" :style="{ background: dept.grad }">
              <span class="sv-emoji">{{ dept.icon }}</span>
              <span class="sv-status-dot" :class="{ on: isWorking(a) }"></span>
            </div>
            <div class="sv-agent-info">
              <div class="sv-agent-name">{{ a.name }}</div>
              <div class="sv-agent-role">{{ dept.name }}</div>
            </div>
          </div>
          <div class="sv-agent-doing">
            <span v-if="isWorking(a)" class="wave">〰</span>
            {{ doingText(a) }}
          </div>
          <div v-if="a.files && a.files.length" class="sv-files">
            <div v-for="f in a.files" :key="f" class="sv-file" :title="f">{{ f }}</div>
          </div>
          <div class="sv-agent-stats">
            <span>📄 {{ a.outputs }}</span>
            <span>🛠️ {{ a.skills }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- 同步活动时间线 -->
    <section class="sv-timeline">
      <h3><span class="grad-text">📡 同步活动流</span></h3>
      <div class="sv-events">
        <div v-for="(e, i) in events" :key="i" class="sv-event">
          <div class="ev-time-line">
            <span class="ev-dot" :style="{ background: deptGrad(e.role) }"></span>
            <span v-if="i < events.length - 1" class="ev-line"></span>
          </div>
          <div class="ev-card">
            <div class="ev-head">
              <span class="ev-emoji">{{ deptIcon(e.role) }}</span>
              <span class="ev-name">{{ e.name }}</span>
              <span class="ev-time">{{ e.time }}</span>
            </div>
            <div class="ev-text">{{ e.text }}</div>
          </div>
        </div>
        <div v-if="!events.length" class="sv-noevents">她们正在赶来…</div>
      </div>
    </section>
  </main>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const agents = ref([])
const events = ref([])
const seen = new Set()

const stars = Array.from({ length: 40 }, () => ({ x: Math.random() * 100, y: Math.random() * 100, d: Math.random() * 3 }))

const deptMeta = {
  writer: { key: 'writer', name: '作者部', icon: '✍️', grad: 'linear-gradient(135deg,#f59e0b,#f97316)' },
  researcher: { key: 'researcher', name: '研究部', icon: '🔬', grad: 'linear-gradient(135deg,#22d3ee,#3b82f6)' },
  coder: { key: 'coder', name: '程序部', icon: '💻', grad: 'linear-gradient(135deg,#a855f7,#6366f1)' },
  publisher: { key: 'publisher', name: '发布部', icon: '📡', grad: 'linear-gradient(135deg,#f43f5e,#fb7185)' },
}

const departments = computed(() => {
  const map = {}
  for (const k in deptMeta) map[k] = { ...deptMeta[k], agents: [] }
  for (const a of agents.value) {
    const d = deptMeta[a.role]
    if (d && map[a.role]) map[a.role].agents.push(a)
  }
  return Object.values(map)
})
const deptIcon = role => (deptMeta[role] || {}).icon || '🤖'
const deptGrad = role => (deptMeta[role] || {}).grad || '#8b5cf6'

const workingCount = computed(() => agents.value.filter(a => isWorking(a)).length)
const anyWorking = computed(() => workingCount.value > 0)
const totalOutputs = computed(() => agents.value.reduce((s, a) => s + (a.outputs || 0), 0))
const totalSkills = computed(() => agents.value.reduce((s, a) => s + (a.skills || 0), 0))

function isWorking(a) {
  const log = a.recentLog || ''
  return /🧠|✍️|🔬|💻|📡|⚙️|调研|学习|写|精读|项目|任务/.test(log) && !/失败|未完成|未成功|熔断|429/.test(log)
}

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
    for (const a of agents.value) {
      const lines = (a.recentLog || '').split('\n').filter(Boolean)
      for (const line of lines.slice(-2)) {
        const key = a.name + '|' + line
        if (!seen.has(key)) {
          seen.add(key)
          const t = (line.match(/\[([^\]]*)\]/) || [])[1] || ''
          const text = line.replace(/^\[[^\]]*\]\s*/, '').slice(0, 46)
          events.value.unshift({ role: a.role, name: a.name, time: t, text })
        }
      }
    }
    if (events.value.length > 30) events.value = events.value.slice(0, 30)
  } catch (e) { /* 静默 */ }
}

let timer
onMounted(() => {
  document.title = '杉汐 | 部门协同工作台'
  loadAgents()
  timer = setInterval(loadAgents, 3000)
})
onUnmounted(() => { clearInterval(timer) })
</script>

<style scoped>
.sync-view {
  min-height: 100vh; padding: 28px 24px 60px;
  font-family: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif;
  background: radial-gradient(1200px 600px at 80% -10%, rgba(139,92,246,.25), transparent),
              radial-gradient(900px 500px at 10% 110%, rgba(34,211,238,.15), transparent),
              linear-gradient(180deg, #0b0b1e 0%, #141433 60%, #1a1a3a 100%);
  color: #e8e8f5; position: relative; overflow: hidden;
}
.bg-stars { position: fixed; inset: 0; pointer-events: none; }
.star { position: absolute; width: 2px; height: 2px; background: #fff; border-radius: 50%; opacity: .5; animation: twinkle 3s infinite; }
@keyframes twinkle { 0%,100% { opacity: .2 } 50% { opacity: .8 } }

.sv-header { position: relative; z-index: 1; }
.sv-title-row { display: flex; align-items: center; justify-content: space-between; }
.sv-header h1 { margin: 0; font-size: 26px; color: #fff; }
.grad-text { background: linear-gradient(135deg, #a78bfa, #60a5fa); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
.grad2-text { background: linear-gradient(135deg, #34d399, #22d3ee); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
.grad3-text { background: linear-gradient(135deg, #fb923c, #f43f5e); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
.sv-live { display: flex; align-items: center; gap: 6px; font-size: 12px; color: #94a3b8; background: rgba(255,255,255,.06); padding: 4px 12px; border-radius: 999px; }
.sv-live.on { color: #34d399; border: 1px solid rgba(52,211,153,.3); }
.live-dot { width: 7px; height: 7px; border-radius: 50%; background: #64748b; }
.sv-live.on .live-dot { background: #34d399; animation: pulse 1.2s infinite; }
@keyframes pulse { 0%,100% { opacity: 1 } 50% { opacity: .3 } }
.sub { color: #64748b; font-size: 13px; margin: 6px 0 18px; }

.sv-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 24px; position: relative; z-index: 1; }
.sv-stat { background: rgba(255,255,255,.05); border: 1px solid rgba(255,255,255,.08); border-radius: 16px; padding: 14px; text-align: center; backdrop-filter: blur(8px); }
.sv-stat-num { font-size: 30px; font-weight: 800; }
.sv-stat-lbl { font-size: 12px; color: #94a3b8; margin-top: 2px; }

.sv-dept { margin-bottom: 26px; position: relative; z-index: 1; }
.sv-dept-head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.sv-dept-icon { font-size: 20px; }
.sv-dept-name { font-size: 18px; font-weight: 700; color: #fff; }
.sv-dept-count { font-size: 12px; color: #94a3b8; background: rgba(255,255,255,.06); padding: 2px 10px; border-radius: 999px; }
.sv-dept-working { font-size: 11px; color: #34d399; border: 1px solid rgba(52,211,153,.3); padding: 2px 10px; border-radius: 999px; animation: pulse 1.5s infinite; }
.sv-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }

.sv-agent { background: rgba(255,255,255,.05); border: 1px solid rgba(255,255,255,.08); border-radius: 16px; padding: 14px; backdrop-filter: blur(10px); transition: all .3s; animation: fadeUp .5s both; }
@keyframes fadeUp { from { opacity: 0; transform: translateY(10px) } to { opacity: 1 } }
.sv-agent.working { border-color: rgba(52,211,153,.5); box-shadow: 0 0 24px rgba(52,211,153,.12); }
.sv-agent-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.sv-agent-avatar { position: relative; width: 44px; height: 44px; border-radius: 14px; display: flex; align-items: center; justify-content: center; font-size: 20px; flex-shrink: 0; }
.sv-status-dot { position: absolute; right: -3px; top: -3px; width: 11px; height: 11px; border-radius: 50%; background: #64748b; border: 2px solid #141433; }
.sv-status-dot.on { background: #34d399; animation: pulse 1s infinite; }
.sv-agent-name { font-size: 14px; font-weight: 700; color: #fff; }
.sv-agent-role { font-size: 11px; color: #94a3b8; }
.sv-agent-doing { font-size: 12px; color: #cbd5e1; background: rgba(0,0,0,.25); border-radius: 8px; padding: 7px; margin: 0 0 8px; min-height: 32px; line-height: 1.5; }
.wave { color: #34d399; margin-right: 4px; animation: pulse 1s infinite; }
.sv-files { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 8px; }
.sv-file { font-size: 10px; color: #a5b4fc; background: rgba(139,92,246,.15); border: 1px solid rgba(139,92,246,.2); padding: 1px 8px; border-radius: 999px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sv-agent-stats { display: flex; gap: 12px; font-size: 12px; color: #94a3b8; }

.sv-timeline { position: relative; z-index: 1; }
.sv-timeline h3 { font-size: 17px; margin: 0 0 14px; color: #fff; }
.sv-events { display: flex; flex-direction: column; }
.sv-event { display: flex; gap: 12px; }
.ev-time-line { display: flex; flex-direction: column; align-items: center; width: 14px; flex-shrink: 0; }
.ev-dot { width: 10px; height: 10px; border-radius: 50%; margin-top: 6px; box-shadow: 0 0 8px rgba(139,92,246,.5); }
.ev-line { flex: 1; width: 2px; background: rgba(139,92,246,.25); }
.ev-card { flex: 1; background: rgba(255,255,255,.04); border: 1px solid rgba(255,255,255,.07); border-radius: 12px; padding: 10px 12px; margin-bottom: 10px; backdrop-filter: blur(8px); }
.ev-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.ev-name { font-weight: 600; font-size: 13px; color: #fff; }
.ev-time { font-size: 11px; color: #64748b; margin-left: auto; }
.ev-text { font-size: 12px; color: #cbd5e1; line-height: 1.5; }
.sv-noevents { color: #64748b; font-size: 14px; text-align: center; padding: 30px 0; }
</style>