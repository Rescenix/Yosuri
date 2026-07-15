<template>
  <div class="smc-root" :class="{ fill }">
    <div class="fm-session-area">
      <SessionList
        :sessions="sessions"
        :active-session="activeSession"
        :running-session="runningSession"
        @select="$emit('select-session', $event)"
        @new-session="$emit('new-session')"
        @rename="$emit('rename-session', $event)"
        @delete="$emit('delete-session', $event)"
      />
    </div>
    <div class="fm-settings-row" @click="$emit('open-settings')">
      <Icon icon="mdi:cog-outline" width="18" color="#6b6b6b" />
      <span>设置</span>
    </div>
    <div class="fm-footer">
      <Icon icon="mdi:account-circle" width="20" color="#6b6b6b" />
      <span>Prometheus · Pro</span>
    </div>
  </div>
</template>

<script setup>
import { Icon } from '@iconify/vue'
import SessionList from './SessionList.vue'

defineProps({
  sessions: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' },
  // 正在跑 agent 的会话 id，透传给 SessionList 点亮蓝色运行指示灯
  runningSession: { type: String, default: '' },
  // 悬浮预览走内容自身高度（session 区自带 max-height 上限）；
  // 钉住态需要撑满整条侧栏，session 区改成 flex:1 吃掉剩余高度
  fill: { type: Boolean, default: false }
})
defineEmits(['select-session', 'new-session', 'rename-session', 'delete-session', 'open-settings'])
</script>

<style scoped>
.smc-root { display: flex; flex-direction: column; }
.smc-root.fill { height: 100%; min-height: 0; }
.smc-root.fill .fm-session-area { flex: 1; max-height: none; }

.fm-session-area {
  min-height: 0;
  max-height: 400px;
  display: flex;
  padding: 8px 8px 0;
  overflow-y: auto;
}

.fm-settings-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  margin-top: 6px;
  border-top: 1px solid #ececec;
  font-size: 13px;
  color: #262626;
  font-weight: 500;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s ease;
}
.fm-settings-row:hover { background: #f5f5f5; }

.fm-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px 14px;
  font-size: 13px;
  color: #1a1a1a;
  font-weight: 500;
  flex-shrink: 0;
}
</style>
