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
        @mouseenter="hoveredId = s.id"
        @mouseleave="onRowLeave(s.id)"
        @click="onRowClick(s)"
      >
        <input
          v-if="editingId === s.id"
          ref="renameInputRef"
          v-model="editingValue"
          class="session-name-input"
          @click.stop
          @keydown.enter="commitRename"
          @keydown.esc="cancelRename"
          @blur="commitRename"
        />
        <span v-else class="session-name">{{ s.name }}</span>

        <div v-if="editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="session-row-menu-wrap">
          <button class="session-row-menu-btn" @click.stop="toggleMenu(s.id)" title="更多">
            <Icon icon="mdi:dots-horizontal" width="16" />
          </button>
          <div v-if="openMenuId === s.id" class="session-row-dropdown" @click.stop>
            <div class="session-row-dropdown-item" @click="startRename(s)">重命名</div>
            <div class="session-row-dropdown-item danger" @click="onDelete(s)">删除</div>
          </div>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { ref, nextTick, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'

defineProps({
  sessions: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' }
})
const emit = defineEmits(['select', 'new-session', 'rename', 'delete'])

const hoveredId = ref(null)
const openMenuId = ref(null)
const editingId = ref(null)
const editingValue = ref('')
const renameInputRef = ref(null)

function onRowLeave(id) {
  if (openMenuId.value !== id) hoveredId.value = null
}
function toggleMenu(id) {
  openMenuId.value = openMenuId.value === id ? null : id
}
function onRowClick(s) {
  if (editingId.value === s.id) return
  emit('select', s.id)
}

function startRename(s) {
  openMenuId.value = null
  editingId.value = s.id
  editingValue.value = s.name
  nextTick(() => {
    const el = Array.isArray(renameInputRef.value) ? renameInputRef.value[0] : renameInputRef.value
    el?.focus()
    el?.select()
  })
}
function commitRename() {
  if (editingId.value) {
    const name = editingValue.value.trim()
    if (name) emit('rename', { id: editingId.value, name })
  }
  editingId.value = null
}
function cancelRename() {
  editingId.value = null
}
function onDelete(s) {
  openMenuId.value = null
  hoveredId.value = null
  emit('delete', s.id)
}

// 点击行内菜单外部时关闭悬浮菜单
function onDocClick() {
  openMenuId.value = null
}
onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<style scoped>
.session-panel {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #fafafa;
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
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  background: #fcfcfc;
  color: #1a1a1a;
  cursor: pointer;
  font-family: inherit;
  transition: background 0.15s ease;
}
.session-new-btn:hover { background: #f0f0f0; }
.session-new-btn .plus { font-size: 14px; line-height: 1; }

.session-recent-label {
  padding: 6px 14px 4px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #a3a3a3;
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
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 10px;
  border-radius: 9px;
  margin: 2px 2px;
  cursor: pointer;
  transition: background 0.15s ease;
}
.session-row:hover { background: #f0f0f0; }
.session-row.active { background: #ececec; }

.session-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #1a1a1a;
}
.session-name-input {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: #1a1a1a;
  font-family: inherit;
  background: #ffffff;
  border: 1px solid #c96442;
  border-radius: 6px;
  padding: 2px 6px;
  outline: none;
}

.session-row-menu-wrap { position: relative; flex-shrink: 0; }
.session-row-menu-btn {
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
.session-row-menu-btn:hover { background: rgba(0, 0, 0, 0.06); }

.session-row-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 4px;
  width: 130px;
  background: #ffffff;
  border: 1px solid #e5e5e5;
  border-radius: 10px;
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.12);
  padding: 6px 0;
  z-index: 60;
}
.session-row-dropdown-item {
  padding: 7px 14px;
  font-size: 12.5px;
  font-weight: 500;
  color: #262626;
  cursor: pointer;
}
.session-row-dropdown-item:hover { background: #f5f5f5; }
.session-row-dropdown-item.danger { color: #d94834; }
</style>
