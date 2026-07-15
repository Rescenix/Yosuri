<template>
  <div class="smc-root" :class="{ fill }">
    <!-- 新对话入口（顶部） -->
    <button class="smc-new-btn" @click="$emit('new-session')">
      <Icon icon="mdi:plus" width="16" />
      <span>新对话</span>
    </button>

    <!-- 项目 -->
    <div class="smc-group">
      <div class="smc-group-label">
        <Icon icon="mdi:folder-outline" width="15" color="#4a4a4a" />
        <span>项目</span>
      </div>
    </div>

    <!-- 产物 -->
    <div class="smc-group">
      <div class="smc-group-label">
        <Icon icon="ph:stack-fill" width="14" color="#4a4a4a" />
        <span>产物</span>
      </div>
    </div>

    <!-- 搜索对话框 -->
    <div class="smc-search-wrap">
      <Icon icon="mdi:magnify" width="15" color="#9a9a9a" />
      <input
        v-model="q"
        class="smc-search-input"
        type="text"
        placeholder="搜索会话..."
      />
    </div>

    <!-- 置顶列表 -->
    <div v-if="pinnedSessions.length" class="smc-section">
      <div class="smc-section-label">
        <Icon icon="mdi:pin" width="13" color="#4a4a4a" />
        <span>置顶</span>
      </div>
      <div class="smc-session-area">
      <div
        v-for="s in pinnedSessions"
        :key="s.id"
        class="smc-session-row"
        :class="{ active: s.id === activeSession, running: s.id === runningSession }"
        @mouseenter="hoveredId = s.id"
        @mouseleave="onRowLeave(s.id)"
        @click="onRowClick(s)"
      >
        <span class="smc-dot" :class="{ on: s.id === runningSession }"></span>
        <span class="smc-name">{{ s.name }}</span>
        <div v-if="hoveredId === s.id || openMenuId === s.id" class="smc-row-menu-wrap">
          <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
            <Icon icon="mdi:dots-horizontal" width="16" />
          </button>
        </div>
      </div>
      </div>
    </div>

    <!-- 最近会话 -->
    <div class="smc-section">
      <div class="smc-section-label">
        <Icon icon="mdi:chat-outline" width="13" color="#4a4a4a" />
        <span>最近会话</span>
        <span class="smc-count">{{ recentSessions.length }}/{{ sessions.length }}</span>
      </div>
      <div class="smc-session-area">
        <div
          v-for="s in recentSessions"
          :key="s.id"
          class="smc-session-row"
          :class="{ active: s.id === activeSession, running: s.id === runningSession }"
          @mouseenter="hoveredId = s.id"
          @mouseleave="onRowLeave(s.id)"
          @click="onRowClick(s)"
        >
          <span class="smc-dot" :class="{ on: s.id === runningSession }"></span>
          <span class="smc-name">{{ s.name }}</span>
          <div v-if="hoveredId === s.id || openMenuId === s.id" class="smc-row-menu-wrap">
            <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
              <Icon icon="mdi:dots-horizontal" width="16" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- footer：账号 + 齿轮 -->
    <div class="fm-footer">
      <Icon icon="mdi:account-circle" width="20" color="#6b6b6b" />
      <span>Prometheus · Pro</span>
      <Icon
        class="fm-footer-settings"
        icon="mdi:cog-outline"
        width="18"
        color="#6b6b6b"
        @click.stop="$emit('open-settings')"
      />
    </div>

    <!-- 行内三点菜单的悬浮卡：Teleport 到 body，脱离会话列表的 overflow 滚动容器，
         避免被 smc-session-area(auto/visible 塌缩) 裁切。位置由触发按钮的 getBoundingClientRect 计算。 -->
    <Teleport to="body">
      <div
        v-if="openMenuId"
        class="smc-row-dropdown"
        :style="dropdownStyle"
        @click.stop
      >
        <div class="smc-dropdown-item" @click="startRename(openMenuSession)">重命名</div>
        <div class="smc-dropdown-item" @click="togglePin(openMenuSession)">{{ isPinned(openMenuId) ? '取消置顶' : '置顶' }}</div>
        <div class="smc-dropdown-item danger" @click="onDelete(openMenuSession)">删除</div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'

const PIN_KEY = 'pinnedSessions'

const props = defineProps({
  sessions: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' },
  runningSession: { type: String, default: '' },
  fill: { type: Boolean, default: false }
})
const emit = defineEmits([
  'select-session', 'new-session', 'rename-session', 'delete-session', 'open-settings'
])

// 置顶 id 集合：localStorage 持久化，不碰后端
const pinnedIds = ref([])
function loadPinned() {
  try { pinnedIds.value = JSON.parse(localStorage.getItem(PIN_KEY) || '[]') } catch { pinnedIds.value = [] }
}
function savePinned() {
  try { localStorage.setItem(PIN_KEY, JSON.stringify(pinnedIds.value)) } catch {}
}
function isPinned(id) { return pinnedIds.value.includes(id) }
function togglePin(s) {
  const id = s.id
  pinnedIds.value = isPinned(id)
    ? pinnedIds.value.filter(x => x !== id)
    : [id, ...pinnedIds.value]
  savePinned()
  openMenuId.value = null
  hoveredId.value = null
}

const q = ref('')
function match(s) {
  if (!q.value.trim()) return true
  return s.name.toLowerCase().includes(q.value.trim().toLowerCase())
}

const pinnedSessions = computed(() =>
  props.sessions.filter(s => isPinned(s.id) && match(s))
)
const recentSessions = computed(() =>
  props.sessions.filter(s => !isPinned(s.id) && match(s))
)

const hoveredId = ref(null)
const openMenuId = ref(null)
const openMenuSession = ref(null)
const dropdownStyle = ref({})
const editingId = ref(null)
const editingValue = ref('')
const renameInputRef = ref(null)

function toggleMenu(s, ev) {
  if (openMenuId.value === s.id) { openMenuId.value = null; return }
  openMenuId.value = s.id
  openMenuSession.value = s
  const rect = ev.currentTarget.getBoundingClientRect()
  const menuW = 140
  const menuH = 116 // 3 项 ~ 每项 32 + padding
  // 默认向左上弹出：右对齐到按钮右边，底边贴按钮顶；空间不够则向下/向右兜底
  let left = rect.right - menuW
  let top = rect.bottom + 6
  if (top + menuH > window.innerHeight) top = rect.top - menuH - 6
  if (left < 8) left = 8
  dropdownStyle.value = { position: 'fixed', left: left + 'px', top: top + 'px', width: menuW + 'px' }
}

function onRowLeave(id) { if (openMenuId.value !== id) hoveredId.value = null }
function onRowClick(s) { if (editingId.value === s.id) return; emit('select-session', s.id) }
function startRename(s) {
  openMenuId.value = null
  editingId.value = s.id
  editingValue.value = s.name
  nextTick(() => {
    const el = Array.isArray(renameInputRef.value) ? renameInputRef.value[0] : renameInputRef.value
    el?.focus(); el?.select()
  })
}
function commitRename() {
  if (editingId.value) {
    const name = editingValue.value.trim()
    if (name) emit('rename-session', { id: editingId.value, name })
  }
  editingId.value = null
}
function cancelRename() { editingId.value = null }
function onDelete(s) {
  openMenuId.value = null
  hoveredId.value = null
  emit('delete-session', s.id)
}
function onDocClick() { openMenuId.value = null }
onMounted(() => { loadPinned(); document.addEventListener('click', onDocClick) })
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<style scoped>
.smc-root { display: flex; flex-direction: column; }
.smc-root.fill { height: 100%; min-height: 0; }
.smc-root.fill .smc-session-area { flex: 1; max-height: none; }

/* 新对话入口（顶部） */
.smc-new-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin: 12px 14px 14px;
  padding: 10px 14px;
  border: 1px solid #d0d5dd;
  border-radius: 10px;
  background: #ffffff;
  color: #1a1a1a;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}
.smc-new-btn:hover { background: #f5f5f5; border-color: #b8bdc7; }
.smc-new-btn:active { background: #eee; }

.smc-group { padding: 10px 16px 4px; flex-shrink: 0; }
.smc-group-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #2a2a2a;
}

.smc-search-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 8px 12px;
  padding: 7px 10px;
  background: #ffffff;
  border: 1px solid #e3e3e3;
  border-radius: 8px;
  flex-shrink: 0;
}
.smc-search-input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  outline: none;
  font-size: 12.5px;
  color: #1a1a1a;
  font-family: inherit;
}
.smc-search-input::placeholder { color: #6b6b6b; }

.smc-section { flex-shrink: 0; }
.smc-section-label {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 16px 6px;
  font-size: 12px;
  font-weight: 600;
  color: #4a4a4a;
}
.smc-count { margin-left: auto; font-weight: 400; color: #a3a3a3; font-size: 11px; }

.smc-session-area {
  overflow-y: auto;
  overflow-x: visible;
  min-height: 0;
  padding: 4px 6px 14px;
}
/* 细滚动条，避免原生粗滚动条破坏呼吸感 */
.smc-session-area::-webkit-scrollbar { width: 6px; }
.smc-session-area::-webkit-scrollbar-track { background: transparent; }
.smc-session-area::-webkit-scrollbar-thumb { background: #d8d8d8; border-radius: 3px; }
.smc-session-area::-webkit-scrollbar-thumb:hover { background: #c0c0c0; }
.smc-session-area { scrollbar-width: thin; scrollbar-color: #d8d8d8 transparent; }

.smc-session-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 12px;
  border-radius: 10px;
  margin: 4px 2px;
  cursor: pointer;
  transition: background 0.15s ease;
}
.smc-session-row:hover { background: #f7f7f7; }
.smc-session-row.active { background: #f3f3f3; box-shadow: inset 0 0 0 1px #e8e8e8; }
.smc-session-row.running { background: rgba(59,130,246,0.06); box-shadow: inset 0 0 0 1px rgba(59,130,246,0.5); }

.smc-dot {
  flex-shrink: 0;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #c4c4c4;
  transition: background 0.2s ease;
}
.smc-dot.on { background: #3b82f6; animation: smc-dot-pulse 1.4s ease-in-out infinite; }
@keyframes smc-dot-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(59,130,246,0.55); }
  50% { box-shadow: 0 0 0 4px rgba(59,130,246,0); }
}

.smc-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 400;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #1a1a1a;
}

.smc-row-menu-wrap { position: relative; flex-shrink: 0; }
.smc-row-menu-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #6b6b6b;
  cursor: pointer;
}
.smc-row-menu-btn:hover { background: rgba(0,0,0,0.06); }

.smc-row-dropdown {
  background: #ffffff;
  border: 1px solid #e5e5e5;
  border-radius: 10px;
  box-shadow: 0 12px 28px rgba(0,0,0,0.16);
  padding: 6px 0;
  z-index: 9999;
}
.smc-dropdown-item {
  padding: 7px 14px;
  font-size: 12.5px;
  font-weight: 500;
  color: #262626;
  cursor: pointer;
}
.smc-dropdown-item:hover { background: #f5f5f5; }
.smc-dropdown-item.danger { color: #d94834; }

.fm-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: auto;
  padding: 8px 16px 14px;
  font-size: 13px;
  color: #1a1a1a;
  font-weight: 500;
  flex-shrink: 0;
}
.fm-footer-settings {
  margin-left: auto;
  cursor: pointer;
  transition: color 0.15s ease;
}
.fm-footer-settings:hover { color: #1a1a1a; }
</style>
