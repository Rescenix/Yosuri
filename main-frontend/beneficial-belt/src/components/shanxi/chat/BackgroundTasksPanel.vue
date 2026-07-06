<template>
  <div class="bgpanel-backdrop" @click="$emit('close')"></div>
  <div class="bgpanel" @click.stop>
    <div class="bgpanel-header">
      <span class="bgpanel-title">Background tasks</span>
      <button class="bgpanel-close-btn" @click="$emit('close')" title="关闭">
        <Icon icon="mdi:close" width="16" color="#8a8a8a" />
      </button>
    </div>

    <div class="bgpanel-toolbar">
      <span class="bgpanel-filter">Finished {{ finishedCount }}</span>
    </div>

    <div class="bgpanel-list">
      <div v-if="tasks.length === 0" class="bgpanel-empty">
        <Icon icon="mdi:tray-outline" width="24" color="#c4c4c4" />
        <span>暂无后台任务</span>
      </div>

      <!-- 每条任务只是标题/状态/耗时/Token 的轻量摘要；步骤明细已下沉进消息流里的
           MessageStepGroup，这里点击一条 = 跳转+展开消息流里对应的那个 group -->
      <div v-for="t in tasks" :key="t.id" class="bgtask-card" @click="$emit('select-task', t.id)">
        <Icon :icon="statusIcon(t.status)" :color="statusColor(t.status)" :spin="t.status === 'running'" width="16" class="bgtask-status-icon" />
        <div class="bgtask-summary-body">
          <div class="bgtask-desc">{{ t.description }}</div>
          <div class="bgtask-meta-row">
            <span>Agent</span>
            <span class="bgtask-dot">·</span>
            <span :class="'bgtask-status-' + t.status">{{ statusLabel(t.status) }}</span>
            <span class="bgtask-dot">·</span>
            <span>{{ formatDuration(t) }}</span>
          </div>
          <div class="bgtask-meta-row secondary">
            {{ formatTok(t.totalTokens) }} tokens · {{ t.toolUseCount }} tool use{{ t.toolUseCount === 1 ? '' : 's' }}
          </div>
        </div>
        <Icon icon="mdi:arrow-right" width="16" class="bgtask-jump-icon" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  tasks: { type: Array, default: () => [] }
})
defineEmits(['close', 'select-task'])

const finishedCount = computed(() => props.tasks.filter(t => t.status !== 'running').length)

// 运行中任务的时长需要每秒刷新一次显示
const nowTick = ref(Date.now())
let tickTimer = null
onMounted(() => {
  tickTimer = setInterval(() => { nowTick.value = Date.now() }, 1000)
})
onUnmounted(() => { clearInterval(tickTimer) })

function formatTok(n) {
  n = n || 0
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
  return String(n)
}

function formatDuration(t) {
  const end = t.status === 'running' ? nowTick.value : (t.endTime || nowTick.value)
  const totalSeconds = Math.max(0, Math.floor((end - t.startTime) / 1000))
  const m = Math.floor(totalSeconds / 60)
  const s = totalSeconds % 60
  if (m >= 60) return `${Math.floor(m / 60)}h ${m % 60}m`
  return `${m}m ${s}s`
}

function statusIcon(status) {
  if (status === 'running') return 'mdi:loading'
  if (status === 'failed') return 'mdi:alert-circle'
  if (status === 'stopped') return 'mdi:stop-circle'
  return 'mdi:check-circle'
}
function statusColor(status) {
  if (status === 'running') return '#c96442'
  if (status === 'failed') return '#d94834'
  if (status === 'stopped') return '#a3a3a3'
  return '#12b76a'
}
function statusLabel(status) {
  if (status === 'running') return 'Running'
  if (status === 'failed') return 'Failed'
  if (status === 'stopped') return 'Stopped'
  return 'Completed'
}
</script>

<style scoped>
.bgpanel-backdrop {
  position: fixed;
  inset: 0;
  z-index: 9990;
}
.bgpanel {
  position: absolute;
  top: 52px;
  right: 12px;
  width: 380px;
  max-height: min(600px, calc(100vh - 90px));
  background: #ffffff;
  border: 1px solid #e5e5e5;
  border-radius: 14px;
  box-shadow: 0 20px 48px rgba(0, 0, 0, 0.18);
  z-index: 9999;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.bgpanel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid #e5e5e5;
  flex-shrink: 0;
}
.bgpanel-title { font-size: 14px; font-weight: 700; color: #1a1a1a; }
.bgpanel-close-btn {
  border: none; background: transparent; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  width: 26px; height: 26px; border-radius: 6px;
}
.bgpanel-close-btn:hover { background: rgba(0, 0, 0, 0.06); }

.bgpanel-toolbar {
  display: flex;
  align-items: center;
  padding: 8px 14px;
  border-bottom: 1px solid #ececec;
  flex-shrink: 0;
}
.bgpanel-filter { font-size: 12px; color: #a3a3a3; }

.bgpanel-list {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
}
.bgpanel-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 0;
  color: #a3a3a3;
  font-size: 12.5px;
}

.bgtask-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e5e5e5;
  border-radius: 12px;
  background: #fafafa;
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.15s ease;
}
.bgtask-card:hover { background: rgba(0, 0, 0, 0.02); }
.bgtask-status-icon { flex-shrink: 0; margin-top: 2px; }
.bgtask-summary-body { flex: 1; min-width: 0; }
.bgtask-desc {
  font-size: 13px;
  font-weight: 600;
  color: #1a1a1a;
  line-height: 1.4;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.bgtask-meta-row {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  color: #8a8a8a;
  margin-top: 4px;
}
.bgtask-meta-row.secondary { color: #a3a3a3; margin-top: 2px; }
.bgtask-dot { color: #c4c4c4; }
.bgtask-status-running { color: #c96442; font-weight: 600; }
.bgtask-status-completed { color: #12b76a; font-weight: 600; }
.bgtask-status-failed { color: #d94834; font-weight: 600; }
.bgtask-status-stopped { color: #a3a3a3; font-weight: 600; }
.bgtask-jump-icon { flex-shrink: 0; margin-top: 2px; color: #c4c4c4; }
</style>
