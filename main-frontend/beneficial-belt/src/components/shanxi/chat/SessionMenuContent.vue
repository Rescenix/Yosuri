<template>
  <div class="smc-root" :class="{ fill }">
    <!-- 顶部功能项 -->
    <div class="smc-nav">
      <button class="smc-nav-item" @click="$emit('new-session')">
        <Icon icon="mdi:pencil-plus-outline" width="18" />
        <span>发起新对话</span>
      </button>
      <button class="smc-nav-item" @click="$emit('open-search')">
        <Icon icon="mdi:magnify" width="18" />
        <span>搜索对话内容</span>
      </button>
    </div>

    <!-- 置顶 = 收藏的项目文件夹（可展开） -->
    <div v-if="pinnedFolders.length" class="smc-section">
      <div class="smc-section-label">
        <Icon icon="mdi:pin" width="14" color="var(--app-accent)" />
        <span>置顶</span>
      </div>
      <div class="smc-session-area">
        <div v-for="f in pinnedFolders" :key="'pin_' + f.name" class="pin-folder-row" :class="{ open: expandedPinned[f.name] }">
        <div class="pin-folder-head" @click="togglePinnedFolder(f.name)" @mouseenter="folderHover = f.name" @mouseleave="folderHover = null">
          <span class="pin-folder-chevron" :class="{ open: expandedPinned[f.name] }">›</span>
          <Icon icon="mdi:folder-outline" width="15" color="var(--app-accent)" />
          <span class="pin-folder-name">{{ f.name }}</span>
          <div class="pin-folder-menu-wrap" v-if="folderHover === f.name || openFolderMenu === f.name">
            <button class="pin-folder-menu-btn" @click.stop="openFolderMenuFn(f.name, $event)" title="更多">
              <Icon icon="mdi:dots-horizontal" width="16" />
            </button>
          </div>
        </div>
          <div v-if="expandedPinned[f.name]" class="pin-folder-children">
            <SessionTreeNode v-for="s in f.sessions" :key="s.id" :node="s"
              :active-session="activeSession" :running-session="runningSession"
              :hovered-id="hoveredId" :open-menu-id="openMenuId"
              :is-expanded="noopExpanded"
              @select="onRowClick" @toggle="noop" @menu="toggleMenu"
              @hover="hoveredId = $event" @hover-leave="onRowLeave" />
          </div>
        </div>
      </div>
    </div>

    <!-- 任务列表：按工作目录分组的会话 -->
    <div class="smc-section">
      <div class="smc-section-label">
        <span>任务列表</span>
      </div>
      <div class="smc-session-area">
        <div v-for="grp in taskGroups" :key="'wd_' + grp.name" class="wd-group" :class="{ open: isGroupOpen(grp.name) }">
          <div class="wd-group-head" @click="toggleGroup(grp.name)" @mouseenter="folderHover = grp.name" @mouseleave="folderHover = null">
            <span class="wd-group-chevron" :class="{ open: isGroupOpen(grp.name) }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-accent)" />
            <span class="wd-group-name">{{ grp.name }}</span>
            <div class="wd-group-menu-wrap" v-if="folderHover === grp.name || openFolderMenu === grp.name">
              <button class="wd-group-menu-btn" @click.stop="openFolderMenuFn(grp.name, $event)" title="更多">
                <Icon icon="mdi:dots-horizontal" width="16" />
              </button>
            </div>
          </div>
          <div v-if="isGroupOpen(grp.name)" class="wd-group-children">
            <SessionTreeNode v-for="s in grp.sessions" :key="s.id" :node="s"
              :active-session="activeSession" :running-session="runningSession"
              :hovered-id="hoveredId" :open-menu-id="openMenuId"
              :is-expanded="noopExpanded"
              @select="onRowClick" @toggle="noop" @menu="toggleMenu"
              @hover="hoveredId = $event" @hover-leave="onRowLeave" />
          </div>
        </div>
        <!-- 未分组文件夹 -->
        <div v-if="orphanSessions.length" class="wd-group">
          <div class="wd-group-head" @click="toggleOrphan" @mouseenter="folderHover = 'orphan'" @mouseleave="folderHover = null">
            <span class="wd-group-chevron" :class="{ open: showOrphan }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-text-faint)" />
            <span class="wd-group-name" style="color:var(--app-text-faint)">未分组</span>
          </div>
          <div v-if="showOrphan" class="wd-group-children">
            <SessionTreeNode v-for="s in orphanSessions" :key="s.id" :node="s"
              :active-session="activeSession" :running-session="runningSession"
              :hovered-id="hoveredId" :open-menu-id="openMenuId"
              :is-expanded="noopExpanded"
              @select="onRowClick" @toggle="noop" @menu="toggleMenu"
              @hover="hoveredId = $event" @hover-leave="onRowLeave" />
          </div>
        </div>
      </div>
    </div>

    <!-- footer -->
    <div class="fm-footer" ref="footerRef">
      <div class="fm-user" @click.stop="toggleUserMenu" title="点击查看账户">
        <img v-if="auth.displayAvatar.value" :src="auth.displayAvatar.value" class="fm-user-avatar" alt="avatar" />
        <Icon v-else icon="mdi:account-circle" width="20" color="#6b6b6b" />
        <span>{{ auth.displayName.value }}</span>
      </div>
      <Icon class="fm-footer-settings" icon="mdi:cog-outline" width="18" color="#6b6b6b" @click.stop="$emit('open-settings')" />
    </div>

    <!-- 用户卡片菜单：点头像悬浮白色卡片，含登录/退出 -->
    <Teleport to="body">
      <div v-if="showUserMenu" ref="userCardRef" class="smc-user-card" :style="userMenuStyle" @click.stop>
        <template v-if="isLoggedIn">
          <div class="smc-user-card-head">
            <img v-if="auth.avatar.value" :src="auth.avatar.value" class="smc-user-avatar" alt="avatar" />
            <Icon v-else icon="mdi:account-circle" width="26" color="var(--app-accent)" />
            <div class="smc-user-card-name">{{ auth.name.value || auth.login.value || 'GitHub 用户' }}</div>
          </div>
          <button class="smc-user-card-item danger" @click="logout">退出登录</button>
        </template>
        <template v-else>
          <div class="smc-user-card-head">
            <Icon icon="mdi:account-circle" width="26" color="var(--app-accent)" />
            <div class="smc-user-card-name">{{ auth.displayName.value }}</div>
          </div>
          <a class="smc-user-card-item" href="/api/auth/github">使用 GitHub 登录</a>
        </template>
      </div>
    </Teleport>

    <!-- 文件夹三点菜单 -->
    <Teleport to="body">
      <div v-if="openFolderMenu" class="smc-row-dropdown" :style="folderMenuStyle" @click.stop>
        <div class="smc-dropdown-item" @click="doPinFolder">{{ isPinned(openFolderMenu) ? '取消置顶' : '置顶' }}</div>
      </div>
    </Teleport>

    <!-- 会话三点菜单 -->
    <Teleport to="body">
      <div v-if="openMenuId" class="smc-row-dropdown" :style="dropdownStyle" @click.stop>
        <div class="smc-dropdown-item" @click="startRename(openMenuSession)">重命名</div>
        <div class="smc-dropdown-item danger" @click="onDelete(openMenuSession)">删除</div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, reactive, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import SessionTreeNode from './SessionTreeNode.vue'
import { useAuth } from '../../../composables/useAuth.js'

const auth = useAuth()

const PIN_KEY = 'shanxi_pinned_projects'

const props = defineProps({
  sessions: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' },
  runningSession: { type: String, default: '' },
  fill: { type: Boolean, default: false }
})
const emit = defineEmits(['select-session', 'new-session', 'rename-session', 'delete-session', 'open-settings', 'open-search'])

// 用户卡片菜单（登录 / 退出）
const footerRef = ref(null)
const userCardRef = ref(null)
const showUserMenu = ref(false)
const userMenuStyle = ref({})
const isLoggedIn = auth.isLoggedIn
function refreshLoginState() { auth.refresh() }
function toggleUserMenu() {
  if (showUserMenu.value) { showUserMenu.value = false; return }
  showUserMenu.value = true
  refreshLoginState()
  nextTick(() => {
    const el = footerRef.value
    const card = userCardRef.value
    if (!el || !card) return
    const rect = el.getBoundingClientRect()
    const cardRect = card.getBoundingClientRect()
    const cardW = 200
    const cardH = cardRect.height || 100
    const gap = 8
    let left = rect.right - cardW
    let top = rect.top - cardH - gap
    if (left < 8) left = 8
    if (top < 8) top = rect.bottom + gap
    if (top + cardH > window.innerHeight - 8) {
      top = window.innerHeight - cardH - 8
      if (top < 8) top = 8
    }
    userMenuStyle.value = { position: 'fixed', left: left + 'px', top: top + 'px', width: cardW + 'px' }
  })
}
function logout() {
  auth.logout()
  showUserMenu.value = false
  refreshLoginState()
}

// 置顶项目文件夹
const pinnedProjectNames = ref([])
function loadPinned() {
  try { pinnedProjectNames.value = JSON.parse(localStorage.getItem(PIN_KEY) || '[]') } catch { pinnedProjectNames.value = [] }
}
function savePinned() {
  try { localStorage.setItem(PIN_KEY, JSON.stringify(pinnedProjectNames.value)) } catch {}
}
function isPinned(name) { return pinnedProjectNames.value.includes(name) }
function togglePinFolder(name) {
  if (isPinned(name)) pinnedProjectNames.value = pinnedProjectNames.value.filter(f => f !== name)
  else pinnedProjectNames.value.push(name)
  savePinned()
}

// 置顶文件夹展开
const expandedPinned = reactive({})
function togglePinnedFolder(name) { expandedPinned[name] = !expandedPinned[name] }

// 任务列表分组展开：默认全展开
const expandedGroups = reactive({})
function isGroupOpen(name) {
  if (expandedGroups[name] !== undefined) return expandedGroups[name]
  return true
}
function toggleGroup(name) { expandedGroups[name] = !isGroupOpen(name) }

// 未分组展开
const showOrphan = ref(true)
function toggleOrphan() { showOrphan.value = !showOrphan.value }

// 文件夹悬浮+三点菜单
const folderHover = ref(null)
const openFolderMenu = ref(null)
const folderMenuStyle = ref({})
function openFolderMenuFn(name, ev) {
  if (openFolderMenu.value === name) { openFolderMenu.value = null; return }
  openFolderMenu.value = name
  const rect = ev.currentTarget.getBoundingClientRect()
  folderMenuStyle.value = { position: 'fixed', left: (rect.right - 100) + 'px', top: (rect.bottom + 4) + 'px', width: '100px' }
}
function doPinFolder() {
  const name = openFolderMenu.value
  if (name) togglePinFolder(name)
  openFolderMenu.value = null
}

// SessionTreeNode 辅助
function noop() {}
const noopExpanded = () => true

// 按 workdir 分组
const workdirMap = computed(() => {
  const map = new Map()
  for (const s of props.sessions) {
    const wd = s.workdir || ''
    if (!wd) continue
    if (!map.has(wd)) map.set(wd, [])
    map.get(wd).push({ ...s, children: [] })
  }
  return map
})

const taskGroups = computed(() => {
  const map = workdirMap.value
  return Array.from(map.entries())
    .sort((a, b) => {
      const aP = isPinned(a[0]) ? 0 : 1
      const bP = isPinned(b[0]) ? 0 : 1
      if (aP !== bP) return aP - bP
      return b[1].length - a[1].length
    })
    .map(([name, sessions]) => ({ name, sessions }))
})

const pinnedFolders = computed(() => {
  const map = workdirMap.value
  return pinnedProjectNames.value
    .filter(name => map.has(name))
    .map(name => ({ name, sessions: map.get(name) }))
})

const orphanSessions = computed(() => {
  const have = new Set()
  for (const [, sess] of workdirMap.value) {
    for (const s of sess) have.add(s.id)
  }
  return props.sessions.filter(s => !have.has(s.id)).map(s => ({ ...s, children: [] }))
})

// 会话菜单
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
  let left = rect.right - menuW
  let top = rect.bottom + 6
  if (top + 116 > window.innerHeight) top = rect.top - 116 - 6
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
function onDocClick() { openMenuId.value = null; openFolderMenu.value = null; showUserMenu.value = false }

onMounted(() => { loadPinned(); document.addEventListener('click', onDocClick); window.addEventListener('auth-change', refreshLoginState) })
onUnmounted(() => { document.removeEventListener('click', onDocClick); window.removeEventListener('auth-change', refreshLoginState) })
</script>

<style scoped>
.smc-root { display: flex; flex-direction: column; }
.smc-root.fill { height: 100%; min-height: 0; }
.smc-root.fill .smc-session-area { flex: 1; max-height: none; }

.smc-nav { flex-shrink: 0; padding: 10px 8px 6px; display: flex; flex-direction: column; gap: 2px; }
.smc-nav-item {
  display: flex; align-items: center; gap: 12px;
  padding: 9px 12px; border: none; border-radius: 999px;
  background: transparent; color: var(--app-text);
  font-size: 13.5px; font-weight: 500; cursor: pointer; text-align: left;
  transition: background 0.12s ease;
}
.smc-nav-item:hover { background: rgba(0, 0, 0, 0.06); }
.smc-nav-item.on { background: rgba(0, 0, 0, 0.08); }

.smc-section { flex-shrink: 0; }
.smc-section-label {
  display: flex; align-items: center; gap: 8px;
  padding: 16px 16px 6px; font-size: 12px; font-weight: 600; color: var(--app-text);
}
.smc-count { margin-left: auto; font-weight: 400; color: var(--app-text-faint); font-size: 11px; }

.smc-session-area { overflow-y: auto; overflow-x: visible; min-height: 0; padding: 4px 6px 14px; }
.smc-session-area::-webkit-scrollbar { width: 6px; }
.smc-session-area::-webkit-scrollbar-track { background: transparent; }
.smc-session-area::-webkit-scrollbar-thumb { background: var(--app-surface-3); border-radius: 3px; }
.smc-session-area::-webkit-scrollbar-thumb:hover { background: #c0c0c0; }
.smc-session-area { scrollbar-width: thin; scrollbar-color: #d8d8d8 transparent; }

.pin-folder-row { margin-bottom: 2px; }
.pin-folder-head, .wd-group-head {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 12px; border-radius: 8px; cursor: pointer;
  transition: background 0.12s ease; position: relative;
}
.pin-folder-head:hover, .wd-group-head:hover { background: var(--app-surface-3); }
.pin-folder-chevron, .wd-group-chevron {
  font-size: 14px; color: var(--app-text-faint); transition: transform 0.15s ease;
  width: 14px; text-align: center; flex-shrink: 0;
}
.pin-folder-chevron.open, .wd-group-chevron.open { transform: rotate(90deg); }
.pin-folder-name, .wd-group-name {
  flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis;
  white-space: nowrap; font-size: 13px; font-weight: 600; color: var(--app-text);
}
.pin-folder-menu-wrap, .wd-group-menu-wrap {
  flex-shrink: 0; display: flex;
}
.pin-folder-menu-btn, .wd-group-menu-btn {
  border: none; background: none; padding: 0 2px; cursor: pointer;
  color: var(--app-text-soft); display: flex;
}
.pin-folder-menu-btn:hover, .wd-group-menu-btn:hover { color: var(--app-text); }
.wd-group { margin-bottom: 2px; }

.smc-row-dropdown {
  background: var(--app-surface);
  border: 1px solid var(--app-border); border-radius: 10px;
  box-shadow: 0 12px 28px rgba(0,0,0,0.16);
  padding: 6px 0; z-index: 9999;
}
.smc-dropdown-item {
  padding: 7px 14px; font-size: 12.5px; font-weight: 500;
  color: var(--app-text); cursor: pointer;
}
.smc-dropdown-item:hover { background: var(--app-surface-3); }
.smc-dropdown-item.danger { color: #d94834; }

.fm-footer {
  display: flex; align-items: center; gap: 10px; margin-top: auto;
  padding: 8px 16px 14px; font-size: 13px; color: var(--app-text);
  font-weight: 500; flex-shrink: 0;
}
.fm-footer-settings { margin-left: auto; cursor: pointer; transition: color 0.15s ease; }
.fm-footer-settings:hover { color: var(--app-text); }
.fm-user { display: flex; align-items: center; gap: 10px; cursor: pointer; border-radius: 8px; }
.fm-user:hover { opacity: 0.85; }
.fm-user-avatar { width: 20px; height: 20px; border-radius: 50%; object-fit: cover; flex-shrink: 0; border: 1px solid var(--app-accent); }

/* 用户卡片菜单：白色卡片浮层 */
.smc-user-card {
  background: #ffffff;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.18);
  padding: 8px;
  z-index: 10000;
}
.smc-user-card-head {
  display: flex; align-items: center; gap: 10px;
  padding: 6px 8px 8px;
}
.smc-user-avatar {
  width: 26px; height: 26px; border-radius: 50%;
  object-fit: cover; flex-shrink: 0;
  border: 1px solid var(--app-accent);
}
.smc-user-card-name { font-size: 13px; font-weight: 600; color: var(--app-text); }
.smc-user-card-item {
  display: block; width: 100%; box-sizing: border-box;
  padding: 9px 12px; border: none; border-radius: 8px;
  background: transparent; color: var(--app-text);
  font-size: 13px; font-weight: 500; text-align: left;
  text-decoration: none; cursor: pointer;
  transition: background 0.12s ease;
}
.smc-user-card-item:hover { background: var(--app-surface-3); }
.smc-user-card-item.danger { color: #d94834; }
.smc-user-card-item.danger:hover { background: rgba(217, 72, 52, 0.08); }
</style>
