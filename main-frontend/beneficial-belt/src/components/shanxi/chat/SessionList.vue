<template>
  <aside class="session-panel">
    <div class="session-new-wrap">
      <button class="session-new-btn" @click="$emit('new-session')">
        <span class="plus">+</span>
        <span>新建会话</span>
      </button>
    </div>
    <div class="session-recent-label">最近会话</div>
    <div class="session-list-body">
      <div
        v-for="s in sessions"
        :key="s.id"
        class="session-row"
        :class="{ active: s.id === activeSession }"
        @click="$emit('select', s.id)"
      >
        <div class="session-row-top">
          <span class="session-dot" :class="'status-' + s.status"></span>
          <span class="session-name">{{ s.name }}</span>
          <span class="session-time">{{ s.time }}</span>
        </div>
        <div class="session-desc">{{ s.desc }}</div>
        <div class="session-row-bottom">
          <span class="session-branch">{{ s.branch }}</span>
          <span class="session-status" :class="'status-' + s.status">{{ statusLabel(s.status) }}</span>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup>
defineProps({
  sessions: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' }
})
defineEmits(['select', 'new-session'])

const LABELS = { running: '运行中', done: '已完成', idle: '空闲' }
function statusLabel(s) {
  return LABELS[s] || '空闲'
}
</script>

<style scoped>
.session-panel {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #faf9f6;
  min-height: 0;
  overflow: hidden;
}

.session-new-wrap { padding: 12px 12px 8px; flex-shrink: 0; }
.session-new-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 0;
  font-size: 12.5px;
  font-weight: 600;
  border: 1px solid #e4dfd4;
  border-radius: 8px;
  background: #fdfcfa;
  color: #1b1a18;
  cursor: pointer;
  font-family: inherit;
  transition: background 0.15s ease;
}
.session-new-btn:hover { background: #efece4; }
.session-new-btn .plus { font-size: 14px; line-height: 1; }

.session-recent-label {
  padding: 6px 14px 4px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #a39c8f;
  flex-shrink: 0;
}

.session-list-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
  padding: 2px 6px 10px;
}

.session-row {
  display: block;
  padding: 9px 10px;
  border-radius: 9px;
  margin: 2px 2px;
  cursor: pointer;
  transition: background 0.15s ease;
}
.session-row:hover { background: #efece4; }
.session-row.active { background: #e8e3d8; }

.session-row-top { display: flex; align-items: center; gap: 8px; }
.session-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.session-dot.status-running { background: #c96442; animation: sess-pulse 1.6s ease-in-out infinite; }
.session-dot.status-done { background: #12b76a; }
.session-dot.status-idle { background: #a39c8f; }

.session-name {
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #1b1a18;
}
.session-time { font-size: 10.5px; color: #a39c8f; flex-shrink: 0; }

.session-desc {
  margin: 3px 0 0 15px;
  font-size: 11.5px;
  color: #696259;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-row-bottom {
  margin: 5px 0 0 15px;
  display: flex;
  align-items: center;
  gap: 7px;
}
.session-branch {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 10px;
  padding: 1px 7px;
  border-radius: 999px;
  border: 1px solid #e4dfd4;
  color: #696259;
}
.session-status { font-size: 10.5px; }
.session-status.status-running { color: #c96442; }
.session-status.status-done { color: #12b76a; }
.session-status.status-idle { color: #a39c8f; }

@keyframes sess-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(1.25); }
}
</style>
