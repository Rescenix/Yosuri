<template>
  <main class="sync-view">
    <header class="sv-header">
      <div class="sv-title-row">
        <h1>AI 公司 · 部门协同工作台</h1>
        <div class="sv-live" :class="{ on: anyWorking }">
          <span class="live-dot"></span> LIVE · {{ workingCount }}/{{ agents.length }} 工作中
        </div>
      </div>
      <p class="sub">分部门协作 · 实时同步 · 点击产出实时预览</p>
    </header>

    <!-- 全局统计 -->
    <section class="sv-stats">
      <div class="sv-stat"><Icon icon="mdi:account-group" width="22" class="st-ico st-blue" /><div class="sv-stat-num">{{ agents.length }}</div><div class="sv-stat-lbl">AI 员工</div></div>
      <div class="sv-stat"><Icon icon="mdi:access-point" width="22" class="st-ico st-green" /><div class="sv-stat-num">{{ workingCount }}</div><div class="sv-stat-lbl">正在工作</div></div>
      <div class="sv-stat"><Icon icon="mdi:file-document-outline" width="22" class="st-ico st-orange" /><div class="sv-stat-num">{{ totalOutputs }}</div><div class="sv-stat-lbl">产出文档</div></div>
      <div class="sv-stat"><Icon icon="mdi:brain" width="22" class="st-ico st-purple" /><div class="sv-stat-num">{{ totalSkills }}</div><div class="sv-stat-lbl">掌握技能</div></div>
    </section>

    <!-- 按部门分组 -->
    <section v-for="dept in departments" :key="dept.key" class="sv-dept">
      <div class="sv-dept-head">
        <Icon :icon="dept.icon" width="20" class="dept-ico" :style="{ color: dept.color }" />
        <span class="sv-dept-name">{{ dept.name }}</span>
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
            <div class="sv-agent-avatar" :style="{ background: dept.color + '1a', border: '1px solid ' + dept.color + '44' }">
              <Icon :icon="dept.icon" width="22" :style="{ color: dept.color }" />
              <span class="sv-status-dot" :class="{ on: isWorking(a) }"></span>
            </div>
            <div class="sv-agent-info">
              <div class="sv-agent-name">{{ a.name }}</div>
              <div class="sv-agent-role">{{ dept.name }}</div>
            </div>
          </div>
          <div class="sv-agent-doing">
            <Icon v-if="isWorking(a)" icon="mdi:loading" width="14" class="spin" />
            {{ doingText(a) }}
          </div>
          <div v-if="a.files && a.files.length" class="sv-files">
            <button v-for="f in a.files" :key="f" class="sv-file" :title="f" @click="previewFile(a, f)">
              <Icon icon="mdi:file-document-outline" width="12" /> {{ f }}
            </button>
          </div>
          <div class="sv-agent-stats">
            <span><Icon icon="mdi:file-document-outline" width="13" /> {{ a.outputs }}</span>
            <span><Icon icon="mdi:toolbox-outline" width="13" /> {{ a.skills }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- 同步活动时间线 -->
    <section class="sv-timeline">
      <h3>同步活动流</h3>
      <div class="sv-events">
        <div v-for="(e, i) in events" :key="i" class="sv-event">
          <div class="ev-time-line">
            <Icon :icon="deptIcon(e.role)" width="14" class="ev-ico" />
            <span v-if="i < events.length - 1" class="ev-line"></span>
          </div>
          <div class="ev-card">
            <div class="ev-head">
              <span class="ev-name">{{ e.name }}</span>
              <span class="ev-time">{{ e.time }}</span>
            </div>
            <div class="ev-text">{{ e.text }}</div>
          </div>
        </div>
        <div v-if="!events.length" class="sv-noevents">她们正在赶来…</div>
      </div>
    </section>

    <!-- 产出预览弹窗 -->
    <div v-if="preview" class="pv-backdrop" @click.self="preview = null">
      <div class="pv-card">
        <div class="pv-head">
          <span class="pv-title">{{ preview.agent }} · {{ preview.file }}</span>
          <button class="pv-close" @click="preview = null"><Icon icon="mdi:close" width="16" /></button>
        </div>
        <pre class="pv-content">{{ preview.content }}</pre>
      </div>
    </div>
  </main>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'

const agents = ref([])
const events = ref([])
const seen = new Set()
const preview = ref(null)

const deptMeta = {
  writer: { key: 'writer', name: '作者部', icon: 'ph:pen-nib-bold', color: '#f59e0b' },
  researcher: { key: 'researcher', name: '研究部', icon: 'mdi:microscope', color: '#3b82f6' },
  coder: { key: 'coder', name: '程序部', icon: 'mdi:code-tags', color: '#8b5cf6' },
  publisher: { key: 'publisher', name: '发布部', icon: 'mdi:bullhorn', color: '#ef4444' },
}

const departments = computed(() => {
  const map = {}
  for (const k in deptMeta) map[k] = { ...deptMeta[k], agents: [], working: false }
  for (const a of agents.value) {
    const d = deptMeta[a.role]
    if (d && map[a.role]) {
      map[a.role].agents.push(a)
      if (isWorking(a)) map[a.role].working = true
    }
  }
  return Object.values(map)
})
const deptIcon = role => (deptMeta[role] || {}).icon || 'mdi:robot'
const deptColor = role => (deptMeta[role] || {}).color || '#64748b'

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

// 点击产出文件 → 实时预览（读 /api/company/file）
async function previewFile(a, f) {
  try {
    const r = await fetch('/api/company/file?agent=' + encodeURIComponent(a.name) + '&name=' + encodeURIComponent(f))
    const d = await r.json()
    preview.value = { agent: a.name, file: f, content: d.content || d.error || '' }
  } catch (e) {
    preview.value = { agent: a.name, file: f, content: '读取失败：' + e.message }
  }
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
  min-height: 100vh;
  padding: 24px 24px 60px;
  font-family: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif;
  background: #f5f7fa;
  color: #333;
}
.sv-header h1 { margin: 0; font-size: 24px; color: #1a1a2e; }
.sv-title-row { display: flex; align-items: center; justify-content: space-between; }
.sv-live { display: flex; align-items: center; gap: 6px; font-size: 12px; color: #666; background: #fff; border: 1px solid #e5e7eb; padding: 4px 12px; border-radius: 999px; }
.sv-live.on { color: #16a34a; border-color: #86efac; }
.live-dot { width: 7px; height: 7px; border-radius: 50%; background: #9ca3af; }
.sv-live.on .live-dot { background: #22c55e; animation: pulse 1.2s infinite; }
@keyframes pulse { 0%,100% { opacity: 1 } 50% { opacity: .3 } }
.sub { color: #6b7280; font-size: 13px; margin: 6px 0 18px; }

.sv-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 24px; }
.sv-stat { background: #fff; border: 1px solid #e5e7eb; border-radius: 14px; padding: 14px; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,.04); }
.st-ico { display: block; margin: 0 auto 4px; }
.st-blue { color: #3b82f6; } .st-green { color: #22c55e; } .st-orange { color: #f59e0b; } .st-purple { color: #8b5cf6; }
.sv-stat-num { font-size: 28px; font-weight: 800; color: #1a1a2e; }
.sv-stat-lbl { font-size: 12px; color: #6b7280; margin-top: 2px; }

.sv-dept { margin-bottom: 26px; }
.sv-dept-head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.sv-dept-ico { display: inline-flex; }
.sv-dept-name { font-size: 17px; font-weight: 700; color: #1a1a2e; }
.sv-dept-count { font-size: 12px; color: #6b7280; background: #eef2f7; padding: 2px 10px; border-radius: 999px; }
.sv-dept-working { font-size: 11px; color: #16a34a; background: #dcfce7; padding: 2px 10px; border-radius: 999px; }
.sv-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }

.sv-agent { background: #fff; border: 1px solid #e5e7eb; border-radius: 14px; padding: 14px; transition: all .2s; animation: fadeUp .4s both; box-shadow: 0 1px 3px rgba(0,0,0,.04); }
@keyframes fadeUp { from { opacity: 0; transform: translateY(8px) } to { opacity: 1 } }
.sv-agent.working { border-color: #86efac; box-shadow: 0 0 12px rgba(34,197,94,.1); }
.sv-agent-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.sv-agent-avatar { position: relative; width: 44px; height: 44px; border-radius: 12px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.sv-status-dot { position: absolute; right: -2px; top: -2px; width: 11px; height: 11px; border-radius: 50%; background: #cbd5e1; border: 2px solid #fff; }
.sv-status-dot.on { background: #22c55e; animation: pulse 1s infinite; }
.sv-agent-name { font-size: 14px; font-weight: 700; color: #1a1a2e; }
.sv-agent-role { font-size: 11px; color: #6b7280; }
.sv-agent-doing { display: flex; align-items: center; gap: 4px; font-size: 12px; color: #4b5563; background: #f9fafb; border: 1px solid #f3f4f6; border-radius: 8px; padding: 7px; margin: 0 0 8px; min-height: 32px; line-height: 1.5; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg) } }
.sv-files { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 8px; }
.sv-file { display: inline-flex; align-items: center; gap: 3px; font-size: 10px; color: #1d4ed8; background: #eff6ff; border: 1px solid #bfdbfe; padding: 2px 8px; border-radius: 999px; cursor: pointer; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sv-file:hover { background: #dbeafe; }
.sv-agent-stats { display: flex; gap: 12px; font-size: 12px; color: #6b7280; }

.sv-timeline h3 { font-size: 16px; margin: 0 0 14px; color: #1a1a2e; }
.sv-events { display: flex; flex-direction: column; }
.sv-event { display: flex; gap: 12px; }
.ev-time-line { display: flex; flex-direction: column; align-items: center; width: 18px; flex-shrink: 0; }
.ev-ico { margin-top: 4px; }
.ev-line { flex: 1; width: 2px; background: #e5e7eb; }
.ev-card { flex: 1; background: #fff; border: 1px solid #e5e7eb; border-radius: 12px; padding: 10px 12px; margin-bottom: 10px; box-shadow: 0 1px 3px rgba(0,0,0,.03); }
.ev-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }
.ev-name { font-weight: 600; font-size: 13px; color: #1a1a2e; }
.ev-time { font-size: 11px; color: #9ca3af; }
.ev-text { font-size: 12px; color: #4b5563; line-height: 1.5; }
.sv-noevents { color: #9ca3af; font-size: 14px; text-align: center; padding: 30px 0; }

/* 预览弹窗 */
.pv-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.4); display: flex; align-items: center; justify-content: center; z-index: 30000; }
.pv-card { width: min(680px, 92vw); max-height: 80vh; background: #fff; border-radius: 14px; box-shadow: 0 20px 60px rgba(0,0,0,.25); display: flex; flex-direction: column; overflow: hidden; }
.pv-head { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; border-bottom: 1px solid #e5e7eb; }
.pv-title { font-size: 14px; font-weight: 600; color: #1a1a2e; }
.pv-close { border: none; background: transparent; color: #6b7280; cursor: pointer; padding: 4px; border-radius: 6px; }
.pv-close:hover { background: #f3f4f6; }
.pv-content { flex: 1; overflow-y: auto; padding: 16px; font-size: 13px; line-height: 1.7; color: #374151; white-space: pre-wrap; margin: 0; }
</style>