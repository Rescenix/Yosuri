<template>
  <div class="smc-root" :class="{ fill }">
    <!-- 顶部操作：新建会话 + 定时任务 -->
    <div class="smc-nav">
      <button class="smc-nav-item primary" type="button" @click="$emit('new-session')">
        <Icon icon="mdi:plus" width="18" />
        <span>新建会话</span>
      </button>
      <button class="smc-nav-item" type="button" @click="$emit('open-scheduled-tasks')">
        <Icon icon="mdi:clock-outline" width="18" />
        <span>定时任务</span>
      </button>
    </div>

    <!-- 搜索框 -->
    <div class="smc-search-bar" @click="focusSearch">
      <Icon icon="mdi:magnify" width="16" class="smc-search-icon" />
      <input
        ref="searchInputRef"
        class="smc-search-input"
        type="text"
        placeholder="搜索会话..."
        @focus="onSearchFocus"
        @keydown.esc="onSearchBlur"
      />
    </div>

    <!-- 会话列表区 -->
    <div class="smc-session-area">

      <!-- 置顶项目文件夹 -->
      <div v-if="pinnedFolders.length" class="smc-section">
        <div class="smc-section-label">
          <Icon icon="mdi:pin" width="14" color="var(--app-accent)" />
          <span>置顶</span>
        </div>
        <div v-for="f in pinnedFolders" :key="'pin_' + f.name" class="smc-folder">
          <div class="smc-folder-head" @click="togglePinnedFolder(f.name)">
            <span class="smc-folder-chevron" :class="{ open: expandedPinned[f.name] }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-accent)" />
            <span class="smc-folder-name">{{ f.name }}</span>
          </div>
          <div v-if="expandedPinned[f.name]" class="smc-folder-children">
            <div
              v-for="s in f.sessions"
              :key="s.id"
              class="smc-session-row"
              :class="{ active: s.id === activeSession, running: s.id === runningSession }"
              @mouseenter="hoveredId = s.id"
              @mouseleave="onRowLeave(s.id)"
              @click="onRowClick(s)"
            >
              <span v-if="bulkMode" class="smc-bulk-check" @click.stop="toggleBulkSelect(s)">
                <Icon :icon="bulkSelected.has(s.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
              </span>
              <span v-else class="smc-session-dot" :class="dotClass(s)"></span>
              <input
                v-if="editingId === s.id"
                ref="renameInputRef"
                v-model="editingValue"
                class="smc-name-input"
                @click.stop
                @keydown.enter="commitRename"
                @keydown.esc="cancelRename"
                @blur="commitRename"
              />
              <Transition name="smc-title-swap" mode="out-in"><span v-if="editingId !== s.id" :key="s.name" class="smc-session-name">{{ s.name }}</span></Transition>
              <div v-if="!bulkMode && editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="smc-row-menu-wrap">
                <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
                  <Icon icon="mdi:dots-horizontal" width="16" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 项目：按工作目录分组 -->
      <div class="smc-section">
        <div class="smc-section-label">
          <span>项目</span>
          <button class="smc-project-bulk" type="button" :title="bulkMode ? '退出批量管理' : '批量管理会话'" :class="{ active: bulkMode }" @click="toggleBulkMode">
            <Icon icon="mdi:playlist-edit" width="18" />
          </button>
          <button class="smc-project-add" type="button" title="创建项目" @click="openCreateProject">
            <Icon icon="mdi:plus" width="18" />
          </button>
        </div>
        <div v-for="grp in taskGroups" :key="'wd_' + grp.name" class="smc-folder">
          <div class="smc-folder-head" @click="toggleGroup(grp.name)">
            <span class="smc-folder-chevron" :class="{ open: isGroupOpen(grp.name) }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-accent)" />
            <span class="smc-folder-name">{{ grp.name }}</span>
          </div>
          <div v-if="isGroupOpen(grp.name)" class="smc-folder-children">
            <div
              v-for="s in grp.sessions"
              :key="s.id"
              class="smc-session-row"
              :class="{ active: s.id === activeSession, running: s.id === runningSession }"
              @mouseenter="hoveredId = s.id"
              @mouseleave="onRowLeave(s.id)"
              @click="onRowClick(s)"
            >
              <span v-if="bulkMode" class="smc-bulk-check" @click.stop="toggleBulkSelect(s)">
                <Icon :icon="bulkSelected.has(s.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
              </span>
              <span v-else class="smc-session-dot" :class="dotClass(s)"></span>
              <input
                v-if="editingId === s.id"
                ref="renameInputRef"
                v-model="editingValue"
                class="smc-name-input"
                @click.stop
                @keydown.enter="commitRename"
                @keydown.esc="cancelRename"
                @blur="commitRename"
              />
              <Transition name="smc-title-swap" mode="out-in"><span v-if="editingId !== s.id" :key="s.name" class="smc-session-name">{{ s.name }}</span></Transition>
              <div v-if="!bulkMode && editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="smc-row-menu-wrap">
                <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
                  <Icon icon="mdi:dots-horizontal" width="16" />
                </button>
              </div>
            </div>
          </div>
        </div>
        <!-- 未分组 -->
        <div v-if="orphanSessions.length" class="smc-folder">
          <div class="smc-folder-head" @click="toggleOrphan">
            <span class="smc-folder-chevron" :class="{ open: showOrphan }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-text-faint)" />
            <span class="smc-folder-name" style="color:var(--app-text-faint)">未分组</span>
          </div>
          <div v-if="showOrphan" class="smc-folder-children">
            <div
              v-for="s in orphanSessions"
              :key="s.id"
              class="smc-session-row"
              :class="{ active: s.id === activeSession, running: s.id === runningSession }"
              @mouseenter="hoveredId = s.id"
              @mouseleave="onRowLeave(s.id)"
              @click="onRowClick(s)"
            >
              <span v-if="bulkMode" class="smc-bulk-check" @click.stop="toggleBulkSelect(s)">
                <Icon :icon="bulkSelected.has(s.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
              </span>
              <span v-else class="smc-session-dot" :class="dotClass(s)"></span>
              <input
                v-if="editingId === s.id"
                ref="renameInputRef"
                v-model="editingValue"
                class="smc-name-input"
                @click.stop
                @keydown.enter="commitRename"
                @keydown.esc="cancelRename"
                @blur="commitRename"
              />
              <Transition name="smc-title-swap" mode="out-in"><span v-if="editingId !== s.id" :key="s.name" class="smc-session-name">{{ s.name }}</span></Transition>
              <div v-if="!bulkMode && editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="smc-row-menu-wrap">
                <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
                  <Icon icon="mdi:dots-horizontal" width="16" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 时间分组会话（无工作目录的独立会话） -->
      <template v-if="looseGroupedSessions.length">
        <div v-for="group in looseGroupedSessions" :key="'t_' + group.label">
          <div class="smc-time-label">{{ group.label }}</div>
          <div
            v-for="s in group.sessions"
            :key="s.id"
            class="smc-session-row"
            :class="{ active: s.id === activeSession, running: s.id === runningSession }"
            @mouseenter="hoveredId = s.id"
            @mouseleave="onRowLeave(s.id)"
            @click="onRowClick(s)"
            >
            <span v-if="bulkMode" class="smc-bulk-check" @click.stop="toggleBulkSelect(s)">
              <Icon :icon="bulkSelected.has(s.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
            </span>
            <span v-else class="smc-session-dot" :class="dotClass(s)"></span>
            <input
              v-if="editingId === s.id"
              ref="renameInputRef"
              v-model="editingValue"
              class="smc-name-input"
              @click.stop
              @keydown.enter="commitRename"
              @keydown.esc="cancelRename"
              @blur="commitRename"
            />
            <Transition name="smc-title-swap" mode="out-in"><span v-if="editingId !== s.id" :key="s.name" class="smc-session-name">{{ s.name }}</span></Transition>
            <div v-if="!bulkMode && editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="smc-row-menu-wrap">
              <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
                <Icon icon="mdi:dots-horizontal" width="16" />
              </button>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- 批量管理操作条 -->
    <div v-if="bulkMode" class="smc-bulk-bar">
      <button class="smc-bulk-action" type="button" @click="toggleSelectAllBulk">
        {{ allBulkSelected ? '取消全选' : '全选' }}
      </button>
      <span class="smc-bulk-count">{{ bulkSelected.size }} 个已选</span>
      <button class="smc-bulk-action danger" type="button" :disabled="!bulkSelected.size" @click="onBulkDelete">删除</button>
      <button class="smc-bulk-action" type="button" @click="toggleBulkMode">完成</button>
    </div>

    <!-- footer -->
    <div class="fm-footer" ref="footerRef">
      <div class="fm-user" ref="userRef" @click.stop="toggleUserMenu" title="点击查看账户">
        <img v-if="auth.displayAvatar.value" :src="auth.displayAvatar.value" class="fm-user-avatar" alt="avatar" />
        <Icon v-else icon="mdi:account-circle" width="20" color="#6b6b6b" />
        <span>{{ auth.displayName.value }}</span>
      </div>
      <button class="fm-footer-settings" type="button" title="设置" @click.stop="$emit('open-settings')">
        <Icon icon="mdi:cog-outline" width="18" />
      </button>
    </div>

    <!-- 用户卡片菜单 -->
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
          <a class="smc-user-card-item" :href="githubAuthUrl">使用 GitHub 登录</a>
        </template>
      </div>
    </Teleport>

    <!-- 会话三点菜单 -->
    <Teleport to="body">
      <div v-if="openMenuId" class="smc-row-dropdown" :style="dropdownStyle" @click.stop>
        <div class="smc-dropdown-item" @click="startRename(openMenuSession)">重命名</div>
        <div class="smc-dropdown-item danger" @click="onDelete(openMenuSession)">删除</div>
      </div>
    </Teleport>

    <!-- 创建项目 -->
    <Teleport to="body">
      <Transition name="smc-modal">
        <div v-if="showCreateProject" class="smc-modal-backdrop" @click.self="closeCreateProject">
          <form class="smc-create-project" @submit.prevent="createProject">
            <div class="smc-create-project-head">
              <h2>创建项目</h2>
              <button type="button" class="smc-modal-close" title="关闭" @click="closeCreateProject">
                <Icon icon="mdi:close" width="20" />
              </button>
            </div>
            <label class="smc-project-name-field">
              <Icon icon="mdi:folder-outline" width="20" />
              <input ref="projectNameInput" v-model="newProjectName" placeholder="项目名称" maxlength="80" />
            </label>
            <div class="smc-source-label">源文件夹</div>
            <button type="button" class="smc-source-picker" @click="pickSourceFolder">
              <Icon icon="mdi:folder-plus-outline" width="25" />
              <span v-if="selectedSourceFolder">{{ selectedSourceFolder.name }}</span>
              <span v-else>添加可读取和编辑的文件夹</span>
            </button>
            <div class="smc-create-project-actions">
              <button type="button" class="smc-cancel-btn" @click="closeCreateProject">取消</button>
              <button type="submit" class="smc-create-btn" :disabled="!newProjectName.trim() || !selectedSourceFolder">创建项目</button>
            </div>
          </form>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, reactive, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { useAuth } from '../../../composables/useAuth.js'

const auth = useAuth()

// GitHub 登录链接：桌面发行版需带后端端口前缀（否则 Wails AssetServer 不认识 /api 路由）
const githubAuthUrl = computed(() => {
  const base = globalThis.__RESCENE_BACKEND_URL__ || ''
  return base + '/api/auth/github'
})

const PIN_KEY = 'shanxi_pinned_projects'

const props = defineProps({
  sessions: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' },
  runningSession: { type: String, default: '' },
  completedSessions: { type: Set, default: () => new Set() },
  questionSession: { type: String, default: '' },
  fill: { type: Boolean, default: false }
})
const emit = defineEmits(['select-session', 'new-session', 'rename-session', 'delete-session', 'delete-sessions', 'open-settings', 'open-search', 'open-plugins', 'create-project', 'open-scheduled-tasks'])

// ========== 搜索框 ==========
const searchInputRef = ref(null)
function focusSearch() { searchInputRef.value?.focus() }
function onSearchFocus() { emit('open-search') }
function onSearchBlur() { searchInputRef.value?.blur() }

// ========== 创建项目 ==========
const showCreateProject = ref(false)
const newProjectName = ref('')
const selectedSourceFolder = ref(null)
const projectNameInput = ref(null)

function openCreateProject() {
  showCreateProject.value = true
  newProjectName.value = ''
  selectedSourceFolder.value = null
  nextTick(() => projectNameInput.value?.focus())
}
function closeCreateProject() { showCreateProject.value = false }
async function pickSourceFolder() {
  try {
    const res = await fetch('/api/workdir/pick', { method: 'POST' })
    if (!res.ok) throw new Error('无法打开文件夹选择器')
    const data = await res.json()
    if (!data.cancelled && data.path) selectedSourceFolder.value = { name: data.name || data.path, path: data.path }
  } catch {}
}
function createProject() {
  const name = newProjectName.value.trim()
  if (!name || !selectedSourceFolder.value) return
  emit('create-project', { name, sourceFolder: selectedSourceFolder.value })
  closeCreateProject()
}
defineExpose({ openCreateProject })

// ========== 置顶项目 ==========
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

// ========== 文件夹展开 ==========
const expandedPinned = reactive({})
const expandedGroups = reactive({})
const showOrphan = ref(true)

function togglePinnedFolder(name) { expandedPinned[name] = !expandedPinned[name] }
function isGroupOpen(name) {
  if (expandedGroups[name] !== undefined) return expandedGroups[name]
  return true
}
function toggleGroup(name) { expandedGroups[name] = !isGroupOpen(name) }
function toggleOrphan() { showOrphan.value = !showOrphan.value }

// ========== 按 workdir 分组 ==========
const workdirMap = computed(() => {
  const map = new Map()
  for (const s of props.sessions) {
    const wd = s.workdir || ''
    if (!wd) continue
    if (!map.has(wd)) map.set(wd, [])
    map.get(wd).push(s)
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
  return props.sessions.filter(s => !have.has(s.id))
})

// ========== 时间分组（无 workdir 的会话） ==========
function getTimeLabel(ts) {
  if (!ts) return '更早'
  const d = new Date(ts).getTime()
  const todayStart = new Date().setHours(0, 0, 0, 0)
  const yesterdayStart = todayStart - 86400000
  if (d >= todayStart) return '今天'
  if (d >= yesterdayStart) return '昨天'
  return '更早'
}

const looseGroupedSessions = computed(() => {
  const groups = new Map()
  for (const s of orphanSessions.value) {
    const label = getTimeLabel(s.updatedAt || s.createdAt)
    if (!groups.has(label)) groups.set(label, [])
    groups.get(label).push(s)
  }
  const order = ['今天', '昨天', '更早']
  return order.filter(l => groups.has(l)).map(label => ({ label, sessions: groups.get(label) }))
})

// ========== 用户卡片 ==========
const footerRef = ref(null)
const userRef = ref(null)
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
    const el = userRef.value || footerRef.value
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

// ========== 会话交互 ==========
const hoveredId = ref(null)
const openMenuId = ref(null)
const openMenuSession = ref(null)
const dropdownStyle = ref({})
const editingId = ref(null)
const editingValue = ref('')
const renameInputRef = ref(null)

function dotClass(s) {
  if (s.id === props.runningSession) return 'running'
  if (props.completedSessions.has(s.id)) return 'completed'
  if (s.id === props.questionSession) return 'question'
  return ''
}

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
function onRowClick(s) {
  if (editingId.value === s.id) return
  if (bulkMode.value) { toggleBulkSelect(s); return }
  emit('select-session', s.id)
}
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

// ========== 批量管理 ==========
const bulkMode = ref(false)
const bulkSelected = ref(new Set())

function toggleBulkMode() {
  bulkMode.value = !bulkMode.value
  bulkSelected.value = new Set()
  openMenuId.value = null
  hoveredId.value = null
  editingId.value = null
}
function toggleBulkSelect(s) {
  const set = new Set(bulkSelected.value)
  if (set.has(s.id)) set.delete(s.id)
  else set.add(s.id)
  bulkSelected.value = set
}
const allBulkSelected = computed(() => props.sessions.length > 0 && bulkSelected.value.size === props.sessions.length)
function toggleSelectAllBulk() {
  bulkSelected.value = allBulkSelected.value ? new Set() : new Set(props.sessions.map(s => s.id))
}
function onBulkDelete() {
  const ids = [...bulkSelected.value]
  if (!ids.length) return
  emit('delete-sessions', ids)
  toggleBulkMode()
}
function onDocClick() { openMenuId.value = null; showUserMenu.value = false }

onMounted(() => { loadPinned(); document.addEventListener('click', onDocClick); window.addEventListener('auth-change', refreshLoginState) })
onUnmounted(() => { document.removeEventListener('click', onDocClick); window.removeEventListener('auth-change', refreshLoginState) })
</script>

<style scoped>
.smc-root {
  display: flex;
  flex-direction: column;
  color: var(--app-text);
}
.smc-root.fill { height: 100%; min-height: 0; }

/* ===== Nav ===== */
.smc-nav {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 10px 4px;
}
.smc-nav-item {
  width: 100%;
  min-height: 40px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--app-text);
  font: inherit;
  font-size: 13.5px;
  font-weight: 500;
  line-height: 1.2;
  cursor: pointer;
  text-align: left;
  transition: background .16s ease, color .16s ease;
}
.smc-nav-item:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 94%);
}
.smc-nav-item.primary {
  background: color-mix(in srgb, var(--app-accent), transparent 90%);
  font-weight: 620;
}
.smc-nav-item.primary:hover {
  background: color-mix(in srgb, var(--app-accent), transparent 80%);
}
.smc-nav-item .iconify {
  flex: 0 0 auto;
  color: var(--app-text-soft);
}
.smc-nav-item.primary .iconify { color: var(--app-accent); }

/* ===== Search ===== */
.smc-search-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 4px 10px 6px;
  padding: 8px 12px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--app-text, #202124), transparent 94%);
  cursor: text;
  transition: background .16s ease;
}
.smc-search-bar:focus-within {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 90%);
}
.smc-search-icon {
  flex: 0 0 auto;
  color: var(--app-text-faint);
}
.smc-search-input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--app-text);
  font: inherit;
  font-size: 13px;
  line-height: 1;
}
.smc-search-input::placeholder {
  color: var(--app-text-faint);
}

/* ===== Section labels ===== */
.smc-section { flex-shrink: 0; }
.smc-section-label {
  min-height: 32px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 16px 16px 6px;
  color: var(--app-text-soft);
  font-size: 12px;
  font-weight: 650;
  letter-spacing: .015em;
}
.smc-project-add {
  width: 28px;
  height: 28px;
  margin-right: -6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-soft);
  cursor: pointer;
  transition: background .15s ease, color .15s ease, transform .15s ease;
}
.smc-project-add:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 93%);
  color: var(--app-text);
  transform: rotate(90deg);
}
.smc-project-bulk {
  width: 28px;
  height: 28px;
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-soft);
  cursor: pointer;
  transition: background .15s ease, color .15s ease;
}
.smc-project-bulk:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 93%);
  color: var(--app-text);
}
.smc-project-bulk.active {
  background: color-mix(in srgb, var(--app-accent), transparent 88%);
  color: var(--app-accent);
}

/* ===== 批量管理 ===== */
.smc-bulk-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 2px 8px 6px;
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--app-accent), transparent 62%);
  background: color-mix(in srgb, var(--app-accent), transparent 92%);
}
.smc-bulk-count {
  flex: 1;
  min-width: 0;
  color: var(--app-text);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.smc-bulk-action {
  flex-shrink: 0;
  border: 0;
  border-radius: 7px;
  padding: 5px 10px;
  background: transparent;
  color: var(--app-accent);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background .15s ease;
}
.smc-bulk-action:hover { background: color-mix(in srgb, var(--app-accent), transparent 86%); }
.smc-bulk-action.danger { color: #d94834; }
.smc-bulk-action.danger:hover { background: rgba(217, 72, 52, 0.1); }
.smc-bulk-action:disabled { color: var(--app-text-faint); cursor: default; background: transparent; }
.smc-bulk-check {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--app-text-soft);
}

/* ===== Folder ===== */
.smc-folder { margin-bottom: 4px; }
.smc-folder-head {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 9px;
  border-radius: 10px;
  cursor: pointer;
  transition: background .15s ease;
}
.smc-folder-head:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 94%);
}
.smc-folder-chevron {
  width: 14px;
  flex-shrink: 0;
  color: var(--app-text-faint);
  font-size: 14px;
  text-align: center;
  transition: transform .18s ease;
}
.smc-folder-chevron.open { transform: rotate(90deg); }
.smc-folder-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  font-weight: 620;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.smc-folder-children {
  /* 不要缩进 —— 会话与文件夹平级 */
  margin-top: 1px;
}

/* ===== Session list area ===== */
.smc-session-area {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
  padding: 2px 6px 10px;
  /* 隐藏滚动条 */
  scrollbar-width: none;
  -ms-overflow-style: none;
}
.smc-session-area::-webkit-scrollbar { display: none; }

/* ===== Session row ===== */
.smc-session-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 10px;
  border-radius: 9px;
  margin: 1px 2px;
  cursor: pointer;
  transition: background 0.15s ease;
}
.smc-session-row:hover { background: color-mix(in srgb, var(--app-text, #202124), transparent 95%); }
.smc-session-row.active {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 92%);
  font-weight: 600;
}
.smc-session-row.running {
  background: color-mix(in srgb, var(--app-accent), transparent 94%);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--app-accent), transparent 50%);
}

/* 运行指示灯：灰色空闲 → running(accent脉冲) / completed(绿) / question(橙) */
.smc-session-dot {
  flex-shrink: 0;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #c4c4c4;
  transition: background 0.2s ease;
}
.smc-session-dot.running {
  background: var(--app-accent);
  animation: smc-dot-pulse 1.4s ease-in-out infinite;
}
.smc-session-dot.completed {
  background: #22c55e;
  box-shadow: 0 0 0 0 #22c55e;
}
.smc-session-dot.question {
  background: #f59e0b;
  box-shadow: 0 0 0 0 #f59e0b;
}
@keyframes smc-dot-pulse {
  0%, 100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--app-accent), transparent 45%); }
  50% { box-shadow: 0 0 0 4px color-mix(in srgb, var(--app-accent), transparent 100%); }
}

.smc-session-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 400;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--app-text);
}
/* 标题渐变替换：旧标题淡出上滑，新标题淡入下滑（AI 生成标题替换默认标题时） */
.smc-title-swap-enter-active,
.smc-title-swap-leave-active {
  transition: opacity 0.28s ease, transform 0.28s ease;
}
.smc-title-swap-enter-from {
  opacity: 0;
  transform: translateY(4px);
}
.smc-title-swap-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
.smc-name-input {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 400;
  color: var(--app-text);
  font-family: inherit;
  background: var(--app-surface);
  border: 1px solid #3b82f6;
  border-radius: 6px;
  padding: 2px 6px;
  outline: none;
}

.smc-row-menu-wrap { position: relative; flex-shrink: 0; display: flex; }
.smc-row-menu-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  border-radius: 7px;
  color: var(--app-text-soft);
  cursor: pointer;
}
.smc-row-menu-btn:hover {
  color: var(--app-text);
  background: color-mix(in srgb, var(--app-text, #1a1a1a), transparent 91%);
}

/* ===== Time label ===== */
.smc-time-label {
  padding: 10px 14px 4px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--app-text-faint);
  flex-shrink: 0;
}

/* ===== Dropdown ===== */
.smc-row-dropdown {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
  box-shadow: 0 12px 28px rgba(0,0,0,0.16);
  padding: 6px 0;
  z-index: 9999;
}
.smc-dropdown-item {
  padding: 7px 14px;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--app-text);
  cursor: pointer;
}
.smc-dropdown-item:hover { background: var(--app-surface-3); }
.smc-dropdown-item.danger { color: #d94834; }

/* ===== Footer ===== */
.fm-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: auto 8px 8px;
  padding: 8px;
  border-top: 1px solid color-mix(in srgb, var(--app-border), transparent 25%);
  color: var(--app-text);
  font-size: 13px;
  font-weight: 500;
}
.fm-footer-settings {
  width: 36px;
  height: 36px;
  margin-left: auto;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border: 0;
  border-radius: 10px;
  color: var(--app-text-soft);
  background: transparent;
  cursor: pointer;
  transition: background .15s ease, color .15s ease;
}
.fm-footer-settings:hover {
  color: var(--app-text);
  background: color-mix(in srgb, var(--app-text, #202124), transparent 94%);
}
.fm-user {
  min-width: 0;
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 6px;
  border-radius: 10px;
  cursor: pointer;
}
.fm-user:hover { background: color-mix(in srgb, var(--app-text, #202124), transparent 95%); }
.fm-user span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fm-user-avatar {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  border: 1px solid color-mix(in srgb, var(--app-accent), transparent 42%);
  border-radius: 50%;
  object-fit: cover;
}

/* ===== User card ===== */
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

/* ===== Create project modal ===== */
.smc-modal-backdrop {
  position: fixed; inset: 0; z-index: 10020;
  display: flex; align-items: center; justify-content: center;
  padding: 24px; background: rgba(15, 23, 42, 0.28); backdrop-filter: blur(3px);
}
.smc-create-project {
  width: min(100%, 620px); box-sizing: border-box; padding: 28px;
  border: 1px solid rgba(255, 255, 255, 0.7); border-radius: 22px;
  background: var(--app-surface, #fff); color: var(--app-text, #202124);
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.2);
}
.smc-create-project-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.smc-create-project-head h2 { margin: 0; font-size: 24px; line-height: 1.2; letter-spacing: -0.02em; }
.smc-modal-close {
  display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px;
  border: 0; border-radius: 8px; background: transparent; color: var(--app-text-soft); cursor: pointer;
}
.smc-modal-close:hover { background: var(--app-surface-3); color: var(--app-text); }
.smc-project-name-field {
  display: flex; align-items: center; gap: 12px; height: 48px; box-sizing: border-box;
  padding: 0 14px; border: 1.5px solid var(--app-accent, #6366f1); border-radius: 14px;
  color: var(--app-text-soft); background: var(--app-surface);
}
.smc-project-name-field input {
  width: 100%; min-width: 0; border: 0; outline: 0; background: transparent;
  color: var(--app-text); font: inherit; font-size: 15px;
}
.smc-project-name-field input::placeholder { color: var(--app-text-faint); }
.smc-source-label { margin: 20px 0 10px; font-size: 14px; font-weight: 650; }
.smc-source-picker {
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px;
  width: 100%; min-height: 120px; padding: 18px; border: 1px solid var(--app-border); border-radius: 14px;
  background: var(--app-surface); color: var(--app-text); font: inherit; font-size: 14px; cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}
.smc-source-picker:hover { border-color: var(--app-accent); background: var(--app-surface-2); }
.smc-source-picker .iconify { color: var(--app-text-soft); }
.smc-create-project-actions { display: flex; justify-content: flex-end; gap: 12px; margin-top: 24px; }
.smc-cancel-btn, .smc-create-btn {
  min-height: 40px; padding: 0 16px; border: 0; border-radius: 11px; font: inherit; font-weight: 600; cursor: pointer;
}
.smc-cancel-btn { background: transparent; color: var(--app-text-soft); }
.smc-cancel-btn:hover { background: var(--app-surface-3); color: var(--app-text); }
.smc-create-btn { background: #202124; color: #fff; }
.smc-create-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.smc-create-btn:not(:disabled):hover { background: #000; }
.smc-modal-enter-active, .smc-modal-leave-active { transition: opacity 0.18s ease; }
.smc-modal-enter-active .smc-create-project, .smc-modal-leave-active .smc-create-project { transition: transform 0.18s ease, opacity 0.18s ease; }
.smc-modal-enter-from, .smc-modal-leave-to { opacity: 0; }
.smc-modal-enter-from .smc-create-project, .smc-modal-leave-to .smc-create-project { transform: translateY(10px) scale(0.98); opacity: 0; }
</style>