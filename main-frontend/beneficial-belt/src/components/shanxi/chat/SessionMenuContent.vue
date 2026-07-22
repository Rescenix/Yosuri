<template>
  <div class="smc-root" :class="{ fill }">
    <!-- 顶部功能项（仿 Gemini：图标 + 文字行） -->
    <div class="smc-nav">
      <button class="smc-nav-item" @click="$emit('new-session')">
        <Icon icon="mdi:pencil-plus-outline" width="18" />
        <span>发起新对话</span>
      </button>
      <button class="smc-nav-item" :class="{ on: showSearch }" @click="toggleSearch">
        <Icon icon="mdi:magnify" width="18" />
        <span>搜索对话内容</span>
      </button>
      <div v-if="showSearch" class="smc-search-wrap">
        <Icon icon="mdi:magnify" width="15" color="#9a9a9a" />
        <input
          ref="searchInput"
          v-model="q"
          class="smc-search-input"
          type="text"
          placeholder="搜索会话..."
        />
      </div>
      <button class="smc-nav-item" @click="$emit('open-projects')">
        <Icon icon="mdi:folder-outline" width="18" />
        <span>项目</span>
      </button>
      <button class="smc-nav-item" @click="$emit('open-attachments')">
        <Icon icon="hugeicons:file-attachment" width="18" />
        <span>附件</span>
      </button>
    </div>

    <!-- 笔记本 = 置顶会话 -->
    <div v-if="pinnedNodes.length" class="smc-section">
      <div class="smc-section-label">
        <Icon icon="lucide:notebook" width="14" color="#4a4a4a" />
        <span>笔记本</span>
      </div>
      <div class="smc-session-area">
        <!-- 置顶的若是某条分支，这里平铺一份（不带祖先），它同时仍嵌套在下面的最近区里
             —— 置顶是快捷方式，不是把分支从血缘里搬走。key 加前缀避免跨区重复 -->
        <SessionTreeNode
          v-for="s in pinnedNodes"
          :key="'pin_' + s.id"
          :node="s"
          :active-session="activeSession"
          :running-session="runningSession"
          :hovered-id="hoveredId"
          :open-menu-id="openMenuId"
          :is-expanded="isExpanded"
          @select="onRowClick"
          @toggle="toggleCollapse"
          @menu="toggleMenu"
          @hover="hoveredId = $event"
          @hover-leave="onRowLeave"
        />
      </div>
    </div>

    <!-- 最近会话（分支嵌套在各自父会话下面） -->
    <div class="smc-section">
      <div class="smc-section-label">
        <span>最近</span>
        <span class="smc-count">{{ rootNodes.length }}/{{ sessions.length }}</span>
      </div>
      <div class="smc-session-area">
        <SessionTreeNode
          v-for="s in rootNodes"
          :key="s.id"
          :node="s"
          :active-session="activeSession"
          :running-session="runningSession"
          :hovered-id="hoveredId"
          :open-menu-id="openMenuId"
          :is-expanded="isExpanded"
          @select="onRowClick"
          @toggle="toggleCollapse"
          @menu="toggleMenu"
          @hover="hoveredId = $event"
          @hover-leave="onRowLeave"
        />
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
import SessionTreeNode from './SessionTreeNode.vue'

const PIN_KEY = 'pinnedSessions'

const props = defineProps({
  sessions: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' },
  runningSession: { type: String, default: '' },
  fill: { type: Boolean, default: false }
})
const emit = defineEmits([
  'select-session', 'new-session', 'rename-session', 'delete-session', 'open-settings',
  'open-projects', 'open-attachments'
])

// 搜索：点"搜索对话内容"才展开输入框（仿 Gemini 的行式入口）
const showSearch = ref(false)
const searchInput = ref(null)
function toggleSearch() {
  showSearch.value = !showSearch.value
  if (showSearch.value) nextTick(() => searchInput.value?.focus())
  else q.value = ''
}

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

// ---- 分支树 ----
// 折叠态记的是「折叠」而不是「展开」：这样默认全展开，新分叉出来的分支天然可见，
// 不用每次分叉都记得去把父节点加进展开集合（否则就会有"我的分支没出现"的怪 bug）。
const COLLAPSE_KEY = 'collapsedSessionBranches'
const collapsedIds = ref({})
function loadCollapsed() {
  try { collapsedIds.value = JSON.parse(localStorage.getItem(COLLAPSE_KEY) || '{}') } catch { collapsedIds.value = {} }
}
function saveCollapsed() {
  try { localStorage.setItem(COLLAPSE_KEY, JSON.stringify(collapsedIds.value)) } catch {}
}
function toggleCollapse(id) {
  if (collapsedIds.value[id]) delete collapsedIds.value[id]
  else collapsedIds.value[id] = true
  saveCollapsed()
}
function subtreeHasActive(node) {
  if (node.id === props.activeSession) return true
  return node.children.some(subtreeHasActive)
}
function isExpanded(node) {
  if (q.value.trim()) return true                 // 搜索时全展开，免得匹配项藏在折叠节点里
  if (!collapsedIds.value[node.id]) return true
  return node.children.some(subtreeHasActive)     // 当前会话在里面就必须展开，否则选中的那条看不见
}

// 先对全量列表建树，再按置顶分区——这样置顶一个根会连整棵子树一起进笔记本
const forest = computed(() => {
  const byId = new Map(props.sessions.map(s => [s.id, { ...s, children: [] }]))
  const roots = []
  for (const node of byId.values()) {
    const parent = node.parentId ? byId.get(node.parentId) : null
    // 找不到父节点的挂到根：后端删父会话时子分支会被提升为根，
    // 这里兜住"列表还没刷新"的那一小段竞态，不让分支凭空消失
    if (parent) parent.children.push(node)
    else roots.push(node)
  }
  return { byId, roots }
})

// 自身匹配 或 任一后代匹配 就保留：扁平过滤会把匹配的子分支连同被过滤掉的父节点
// 一起弄没，搜索反而找不到东西
function filterTree(nodes) {
  const out = []
  for (const n of nodes) {
    const kids = filterTree(n.children)
    if (match(n) || kids.length) out.push({ ...n, children: kids })
  }
  return out
}

// 最近区：没被置顶的根会话，各自带整棵子树（置顶的子分支仍然嵌套在这里，
// 因为置顶只是加个快捷方式，不该让树谎报血缘）
const rootNodes = computed(() => filterTree(forest.value.roots.filter(s => !isPinned(s.id))))

// 笔记本区：置顶的根带着整棵子树进来；置顶的是子分支则平铺一份（不带祖先）
const pinnedNodes = computed(() => {
  const { byId, roots } = forest.value
  const rootIds = new Set(roots.map(r => r.id))
  const picked = []
  for (const s of props.sessions) {
    if (!isPinned(s.id)) continue
    const node = byId.get(s.id)
    if (!node) continue
    picked.push(rootIds.has(s.id) ? node : { ...node, children: [] })
  }
  return filterTree(picked)
})

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
onMounted(() => { loadPinned(); loadCollapsed(); document.addEventListener('click', onDocClick) })
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<style scoped>
.smc-root { display: flex; flex-direction: column; }
.smc-root.fill { height: 100%; min-height: 0; }
.smc-root.fill .smc-session-area { flex: 1; max-height: none; }

/* 新对话入口（顶部） */
/* 顶部功能项（仿 Gemini：图标 + 文字行，胶囊 hover） */
.smc-nav { flex-shrink: 0; padding: 10px 8px 6px; display: flex; flex-direction: column; gap: 2px; }
.smc-nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 12px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: #1f1f1f;
  font-size: 13.5px;
  font-weight: 500;
  cursor: pointer;
  text-align: left;
  transition: background 0.12s ease;
}
.smc-nav-item:hover { background: rgba(0, 0, 0, 0.06); }
.smc-nav-item.on { background: rgba(0, 0, 0, 0.08); }

.smc-search-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 2px 10px 6px;
  padding: 7px 10px;
  background: #ffffff;
  border: 1px solid #e3e3e3;
  border-radius: 10px;
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

/* 会话行的样式已随分支树迁到 SessionTreeNode.vue（那边用主题变量，暗色模式也对） */

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
