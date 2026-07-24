<template>
  <div v-if="embedded" class="file-tool-shell" @keydown="onKeydown">
    <div class="file-tool-card embedded-mode">
      <div class="file-tool-mainbar">
        <div class="file-tool-breadcrumb">
          <span class="crumb-root">re0</span>
          <span class="crumb-sep">›</span>
          <span class="crumb-path">{{ activeTab?.path || '选择文件开始编辑' }}</span>
        </div>
        <div class="file-tool-mainbar-actions">
          <span v-if="saveState" class="file-tool-save-state" :class="saveState">
            {{ saveState === 'saving' ? '保存中…' : saveState === 'saved' ? '已保存' : '保存失败' }}
          </span>
          <!-- 文件树开关常驻在这——不再是内容区里一条会消失的窄栏，图标复用左侧
               Gemini 风侧栏折叠用的同一个 lucide:sidebar，视觉语言对上 -->
          <button class="file-tool-icon-btn" type="button" @click="treeCollapsed = !treeCollapsed" :title="treeCollapsed ? '显示文件树' : '隐藏文件树'">
            <Icon icon="lucide:sidebar" width="16" />
          </button>
          <div class="file-tool-more-wrap">
            <button class="file-tool-icon-btn" type="button" @click.stop="showMoreMenu = !showMoreMenu" title="更多">
              <Icon icon="mdi:dots-horizontal" width="16" />
            </button>
            <div v-if="showMoreMenu" class="file-tool-more-menu" @click.stop>
              <button type="button" @click="loadTree(); showMoreMenu = false">
                <Icon icon="mdi:refresh" width="14" :class="{ spin: treeLoading }" />刷新文件树
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="file-tool-body" :style="{ gridTemplateColumns: `minmax(0,1fr) ${treeCollapsed ? 0 : treeWidth}px` }">
        <section class="file-tool-editor-pane">
          <CodeEditor
            :tabs="tabs"
            :active-file-path="activeFilePath"
            :file-content="activeTab?.content ?? ''"
            :language="activeTab ? languageOf(activeTab.name) : 'plaintext'"
            :pinned-paths="pinnedPaths"
            @update:content="onUpdateContent"
            @switch-file="switchFile"
            @close-file="closeFile"
            @pin-file="pinFile"
            @unpin-file="unpinFile"
          />
        </section>

        <aside v-if="!treeCollapsed" class="file-tool-tree-pane">
          <div class="file-tool-tree-resize" title="拖拽调整宽度" @mousedown="startTreeResize"></div>
          <div class="file-tool-tree-header">
            <span class="file-tool-tree-project">
              <Icon icon="mdi:folder-outline" width="14" />
              <span>re0</span>
            </span>
            <button class="file-tool-icon-btn sm" type="button" @click="toggleTreeSearch" :title="treeSearchOpen ? '关闭搜索' : '搜索文件'">
              <Icon :icon="treeSearchOpen ? 'mdi:close' : 'mdi:magnify'" width="14" />
            </button>
          </div>

          <!-- 搜索：整块替换文件树视图，不是叠加在上面（仿 Cursor） -->
          <template v-if="treeSearchOpen">
            <div class="file-tool-search-bar">
              <Icon icon="mdi:magnify" width="14" />
              <input ref="treeSearchInputRef" v-model="treeQuery" type="text" placeholder="搜索文件…" @keydown.esc="closeTreeSearch" />
              <button class="search-toggle-btn" :class="{ on: searchCaseSensitive }" type="button" title="区分大小写" @click="searchCaseSensitive = !searchCaseSensitive">Aa</button>
              <button class="search-toggle-btn" :class="{ on: searchWholeWord }" type="button" title="全字匹配" @click="searchWholeWord = !searchWholeWord">ab</button>
              <button class="search-toggle-btn" :class="{ on: searchRegex }" type="button" title="正则表达式" @click="searchRegex = !searchRegex">.*</button>
            </div>
            <div class="file-tool-tree-body">
              <div v-if="!treeQuery.trim()" class="file-tool-tree-msg">输入关键字搜索文件名</div>
              <div v-else-if="filteredTreeNodes.length === 0" class="file-tool-tree-msg">没有匹配的文件</div>
              <FileTreeNode
                v-else
                v-for="node in filteredTreeNodes"
                :key="node.path || node.name"
                :node="node"
                :depth="0"
                :selected="selectedNode"
                @select="onSelectNode"
                @toggle="onToggleNode"
              />
            </div>
          </template>
          <div v-else class="file-tool-tree-body">
            <div v-if="treeLoading" class="file-tool-tree-msg">加载中…</div>
            <div v-else-if="treeError" class="file-tool-tree-msg error">{{ treeError }}</div>
            <FileTreeNode
              v-else
              v-for="node in treeNodes"
              :key="node.path || node.name"
              :node="node"
              :depth="0"
              :selected="selectedNode"
              @select="onSelectNode"
              @toggle="onToggleNode"
            />
          </div>
        </aside>
      </div>
    </div>
  </div>

  <Teleport v-else to="body">
    <div class="file-tool-backdrop" @click="requestClose" @keydown.esc="requestClose">
      <div class="file-tool-card" @click.stop @keydown="onKeydown">
        <div class="file-tool-mainbar">
          <div class="file-tool-breadcrumb">
            <span class="crumb-root">re0</span>
            <span class="crumb-sep">›</span>
            <span class="crumb-path">{{ activeTab?.path || '选择文件开始编辑' }}</span>
          </div>
          <div class="file-tool-mainbar-actions">
            <span v-if="saveState" class="file-tool-save-state" :class="saveState">
              {{ saveState === 'saving' ? '保存中…' : saveState === 'saved' ? '已保存' : '保存失败' }}
            </span>
            <button class="file-tool-icon-btn" type="button" @click="treeCollapsed = !treeCollapsed" :title="treeCollapsed ? '显示文件树' : '隐藏文件树'">
              <Icon icon="lucide:sidebar" width="16" />
            </button>
            <div class="file-tool-more-wrap">
              <button class="file-tool-icon-btn" type="button" @click.stop="showMoreMenu = !showMoreMenu" title="更多">
                <Icon icon="mdi:dots-horizontal" width="16" />
              </button>
              <div v-if="showMoreMenu" class="file-tool-more-menu" @click.stop>
                <button type="button" @click="loadTree(); showMoreMenu = false">
                  <Icon icon="mdi:refresh" width="14" :class="{ spin: treeLoading }" />刷新文件树
                </button>
              </div>
            </div>
            <button class="file-tool-icon-btn" type="button" @click="requestClose" title="关闭 (Esc)">
              <Icon icon="mdi:close" width="18" />
            </button>
          </div>
        </div>

        <div class="file-tool-body" :style="{ gridTemplateColumns: `minmax(0,1fr) ${treeCollapsed ? 0 : treeWidth}px` }">
          <section class="file-tool-editor-pane">
            <CodeEditor
              :tabs="tabs"
              :active-file-path="activeFilePath"
              :file-content="activeTab?.content ?? ''"
              :language="activeTab ? languageOf(activeTab.name) : 'plaintext'"
              :pinned-paths="pinnedPaths"
              @update:content="onUpdateContent"
              @switch-file="switchFile"
              @close-file="closeFile"
              @pin-file="pinFile"
              @unpin-file="unpinFile"
            />
          </section>

          <aside v-if="!treeCollapsed" class="file-tool-tree-pane">
            <div class="file-tool-tree-resize" title="拖拽调整宽度" @mousedown="startTreeResize"></div>
            <div class="file-tool-tree-header">
              <span class="file-tool-tree-project">
                <Icon icon="mdi:folder-outline" width="14" />
                <span>re0</span>
              </span>
              <button class="file-tool-icon-btn sm" type="button" @click="toggleTreeSearch" :title="treeSearchOpen ? '关闭搜索' : '搜索文件'">
                <Icon :icon="treeSearchOpen ? 'mdi:close' : 'mdi:magnify'" width="14" />
              </button>
            </div>

            <template v-if="treeSearchOpen">
              <div class="file-tool-search-bar">
                <Icon icon="mdi:magnify" width="14" />
                <input ref="treeSearchInputRef" v-model="treeQuery" type="text" placeholder="搜索文件…" @keydown.esc="closeTreeSearch" />
                <button class="search-toggle-btn" :class="{ on: searchCaseSensitive }" type="button" title="区分大小写" @click="searchCaseSensitive = !searchCaseSensitive">Aa</button>
                <button class="search-toggle-btn" :class="{ on: searchWholeWord }" type="button" title="全字匹配" @click="searchWholeWord = !searchWholeWord">ab</button>
                <button class="search-toggle-btn" :class="{ on: searchRegex }" type="button" title="正则表达式" @click="searchRegex = !searchRegex">.*</button>
              </div>
              <div class="file-tool-tree-body">
                <div v-if="!treeQuery.trim()" class="file-tool-tree-msg">输入关键字搜索文件名</div>
                <div v-else-if="filteredTreeNodes.length === 0" class="file-tool-tree-msg">没有匹配的文件</div>
                <FileTreeNode
                  v-else
                  v-for="node in filteredTreeNodes"
                  :key="node.path || node.name"
                  :node="node"
                  :depth="0"
                  :selected="selectedNode"
                  @select="onSelectNode"
                  @toggle="onToggleNode"
                />
              </div>
            </template>
            <div v-else class="file-tool-tree-body">
              <div v-if="treeLoading" class="file-tool-tree-msg">加载中…</div>
              <div v-else-if="treeError" class="file-tool-tree-msg error">{{ treeError }}</div>
              <FileTreeNode
                v-else
                v-for="node in treeNodes"
                :key="node.path || node.name"
                :node="node"
                :depth="0"
                :selected="selectedNode"
                @select="onSelectNode"
                @toggle="onToggleNode"
              />
            </div>
          </aside>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { Icon } from '@iconify/vue'
import CodeEditor from './CodeEditor.vue'
import FileTreeNode from './FileTreeNode.vue'
import { useResizableWidth } from './useResizable.js'

const emit = defineEmits(['close'])
const props = defineProps({
  embedded: { type: Boolean, default: false }
})

// 文件树左边框拖拽调宽：树贴在右边，手柄在它左边界，edge:'left' 时
// useResizableWidth 会把"手柄往左拖"换算成"变宽"（拖手柄=拖着树的左边界走）。
// 嵌入 dock 面板和独立弹窗宽度差得远（~380px vs 1180px），默认值/持久化 key 分开存，
// 不然嵌入模式记下的窄宽度会把独立弹窗也顶得很挤，反过来也一样别扭。
const treeWidth = ref(props.embedded ? 150 : 260)
const { startDrag: startTreeResize } = useResizableWidth(treeWidth, {
  min: 120,
  max: 480,
  edge: 'left',
  persistKey: props.embedded ? 'fileToolTreeWidthEmbedded' : 'fileToolTreeWidthModal'
})
// 隐藏文件树：编辑区独占宽度；开关按钮常驻顶栏（lucide:sidebar 图标），
// 不再是内容区里一条会跟着消失、只能靠"记得点哪"才能点回来的窄栏
const treeCollapsed = ref(false)

const treeNodes = ref([])
const treeLoading = ref(false)
const treeError = ref('')
const selectedNode = ref(null)
const tabs = ref([])
const activeFilePath = ref('')
const openError = ref('')
const pinnedPaths = ref([])
const saveState = ref('')
const showMoreMenu = ref(false)
let saveStateTimer = null

const activeTab = computed(() => tabs.value.find(t => t.path === activeFilePath.value) || null)

// ---- 搜索：点搜索图标后整块替换树视图（仿 Cursor），不是叠加一层筛选框 ----
const treeSearchOpen = ref(false)
const treeSearchInputRef = ref(null)
const treeQuery = ref('')
const searchCaseSensitive = ref(false)
const searchWholeWord = ref(false)
const searchRegex = ref(false)

function toggleTreeSearch() {
  treeSearchOpen.value = !treeSearchOpen.value
  if (treeSearchOpen.value) {
    nextTick(() => treeSearchInputRef.value?.focus())
  } else {
    treeQuery.value = ''
  }
}
function closeTreeSearch() {
  treeSearchOpen.value = false
  treeQuery.value = ''
}

// Aa/全字/正则三个开关是真的接进匹配逻辑的，不是摆设图标
function buildMatcher(query) {
  if (searchRegex.value) {
    try {
      return new RegExp(query, searchCaseSensitive.value ? '' : 'i')
    } catch {
      return null // 正则写错了：当无匹配处理，不让它把整个搜索炸掉
    }
  }
  let pattern = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') // 转义成字面量再拼
  if (searchWholeWord.value) pattern = `\\b${pattern}\\b`
  try {
    return new RegExp(pattern, searchCaseSensitive.value ? '' : 'i')
  } catch {
    return null
  }
}

const filteredTreeNodes = computed(() => {
  const q = treeQuery.value.trim()
  if (!q) return []
  const matcher = buildMatcher(q)
  if (!matcher) return []
  return filterTree(treeNodes.value, matcher)
})

function filterTree(nodes, matcher) {
  const result = []
  for (const node of nodes || []) {
    const selfMatch = matcher.test(node.name || '')
    if (node.type === 'folder') {
      const children = filterTree(node.children || [], matcher)
      if (selfMatch || children.length) {
        result.push({ ...node, expanded: true, children })
      }
    } else if (selfMatch) {
      result.push(node)
    }
  }
  return result
}

async function loadTree() {
  treeLoading.value = true
  treeError.value = ''
  try {
    const res = await fetch('/api/file-tree')
    if (!res.ok) throw new Error(`加载失败 (${res.status})`)
    treeNodes.value = await res.json()
  } catch (e) {
    treeError.value = e.message || '加载文件树失败'
  } finally {
    treeLoading.value = false
  }
}

function onToggleNode(node) {
  node.expanded = !node.expanded
}

// 单击直接打开——原来是"点一下选中、再点一下（哪怕是同一下）才真正打开"，
// 等于强制双击，体验上跟 Cursor 点文件立刻显示的预期完全对不上。现在选中和
// 打开是同一个动作，selectedNode 只留给 FileTreeNode 做高亮用。
async function onSelectNode(node) {
  selectedNode.value = node
  if (node.type === 'folder') {
    node.expanded = !node.expanded
    return
  }
  await openFile(node)
}

async function openFile(node) {
  openError.value = ''
  // 去重：已经开着就切过去，不再新开一个标签
  const existing = tabs.value.find(t => t.path === node.path)
  if (existing) {
    activeFilePath.value = existing.path
    return
  }
  try {
    const res = await fetch('/api/file?path=' + encodeURIComponent(node.path))
    if (!res.ok) throw new Error(await res.text() || `打开失败 (${res.status})`)
    const content = await res.text()
    tabs.value.push({ path: node.path, name: node.name, content, savedContent: content })
    activeFilePath.value = node.path
  } catch (e) {
    openError.value = e.message || '打开文件失败'
    window.alert('打开失败：' + openError.value)
  }
}

function switchFile(path) {
  flushAutoSave() // 离开当前标签前把没落盘的防抖改动先冲掉，不等 600ms
  activeFilePath.value = path
  const matched = tabs.value.find(t => t.path === path)
  if (matched) selectedNode.value = { type: 'file', path: matched.path, name: matched.name }
}

function closeFile(path) {
  if (path === activeFilePath.value) flushAutoSave()
  const tab = tabs.value.find(t => t.path === path)
  if (tab && isDirty(tab)) {
    // 实时保存下这条基本只在"保存请求本身失败"时才会触发，留着当兜底
    if (!window.confirm(`「${tab.name}」还有未保存的修改（保存失败？），确定要关闭吗？`)) return
  }
  const idx = tabs.value.findIndex(t => t.path === path)
  if (idx === -1) return
  tabs.value.splice(idx, 1)
  if (activeFilePath.value === path) {
    activeFilePath.value = tabs.value[Math.max(0, idx - 1)]?.path || ''
  }
}

// 实时保存：跟 VS Code 一样，输入停顿一下就自动落盘，不再靠手动点"保存"按钮。
// 600ms 防抖——太短会在用户还在敲字的时候就疯狂发请求，太长又显得"没保上"。
let autoSaveTimer = null
function scheduleAutoSave() {
  clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(saveActiveFile, 600)
}
function flushAutoSave() {
  clearTimeout(autoSaveTimer)
  if (activeTab.value && isDirty(activeTab.value)) saveActiveFile()
}
function onUpdateContent(val) {
  const tab = activeTab.value
  if (!tab) return
  tab.content = val
  scheduleAutoSave()
}

function pinFile(tab) {
  if (!pinnedPaths.value.includes(tab.path)) pinnedPaths.value.push(tab.path)
}

function unpinFile(tab) {
  pinnedPaths.value = pinnedPaths.value.filter(p => p !== tab.path)
}

function isDirty(tab) {
  return tab.content !== tab.savedContent
}

async function saveActiveFile() {
  const tab = activeTab.value
  if (!tab || !isDirty(tab)) return
  saveState.value = 'saving'
  try {
    const res = await fetch('/api/file', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: tab.path, content: tab.content })
    })
    if (!res.ok) throw new Error(await res.text())
    tab.savedContent = tab.content
    saveState.value = 'saved'
  } catch (e) {
    saveState.value = 'error'
  } finally {
    clearTimeout(saveStateTimer)
    saveStateTimer = setTimeout(() => { saveState.value = '' }, 2000)
  }
}

// Ctrl+S 仍然管用——立即冲掉防抖直接存，急的时候不用等 600ms
function onKeydown(e) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    flushAutoSave()
  }
}

function requestClose() {
  flushAutoSave()
  const dirtyTabs = tabs.value.filter(isDirty)
  if (dirtyTabs.length && !window.confirm(`还有 ${dirtyTabs.length} 个文件未保存（保存失败？），确定要关闭吗？`)) {
    return
  }
  emit('close')
}

const LANG_MAP = {
  js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
  ts: 'typescript', tsx: 'typescript',
  vue: 'html', html: 'html', htm: 'html',
  css: 'css', scss: 'scss', less: 'less',
  json: 'json', md: 'markdown', yml: 'yaml', yaml: 'yaml',
  py: 'python', go: 'go', rs: 'rust', java: 'java',
  c: 'c', h: 'c', cpp: 'cpp', hpp: 'cpp',
  sh: 'shell', bash: 'shell', ps1: 'powershell',
  sql: 'sql', xml: 'xml', toml: 'ini', ini: 'ini'
}

function languageOf(name) {
  const ext = name.split('.').pop()?.toLowerCase()
  return LANG_MAP[ext] || 'plaintext'
}

function closeMoreMenuOutside() {
  showMoreMenu.value = false
}

onMounted(() => {
  loadTree()
  document.addEventListener('click', closeMoreMenuOutside)
})
onUnmounted(() => {
  document.removeEventListener('click', closeMoreMenuOutside)
})
</script>

<style scoped>
/* 全部改用 var(--app-*)：之前这块是另一份硬编码的暖米色 hex 调色板，跟应用其余
   部分的主题系统（含明暗切换）完全脱节，切个主题这里就是一块不听话的白色死区。 */
.file-tool-shell {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.file-tool-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  z-index: 20000;
  display: flex;
  align-items: center;
  justify-content: center;
}

.file-tool-card {
  width: min(1180px, 95vw);
  height: min(760px, 90vh);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--app-surface);
  border-radius: 14px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
}

.file-tool-card.embedded-mode {
  width: 100%;
  height: 100%;
  border-radius: 0;
  box-shadow: none;
}

/* 简化后的顶栏：面包屑 + 保存状态 + "..." 菜单（+ 独立弹窗多一个关闭）。
   打开/保存两个按钮已经删掉——单击文件即打开，编辑即自动保存，不需要手动触发。 */
.file-tool-mainbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 40px;
  padding: 0 10px;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-surface);
  flex-shrink: 0;
}

.file-tool-breadcrumb {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.crumb-root,
.crumb-sep {
  font-size: 12px;
  color: var(--app-text-faint);
  flex-shrink: 0;
}

.crumb-path {
  min-width: 0;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--app-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-tool-mainbar-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.file-tool-save-state {
  font-size: 11px;
  color: var(--app-text-faint);
}

.file-tool-save-state.saved { color: #12b76a; }
.file-tool-save-state.error { color: #d94834; }

.file-tool-icon-btn {
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  border-radius: 7px;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.file-tool-icon-btn.sm { width: 24px; height: 24px; }
.file-tool-icon-btn:hover { background: var(--app-surface-3); color: var(--app-text); }

.file-tool-more-wrap { position: relative; }
.file-tool-more-menu {
  position: absolute;
  top: 32px;
  right: 0;
  min-width: 150px;
  padding: 4px;
  border: 1px solid var(--app-border);
  border-radius: 9px;
  background: var(--app-surface);
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.14);
  z-index: 30;
}
.file-tool-more-menu button {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  border: none;
  background: transparent;
  padding: 7px 9px;
  border-radius: 6px;
  font-size: 12.5px;
  color: var(--app-text);
  cursor: pointer;
  text-align: left;
}
.file-tool-more-menu button:hover { background: var(--app-surface-3); }

/* 树宽由 treeWidth（拖拽调宽）驱动 grid-template-columns；隐藏树时收到 28px
   只留一条能点回来的窄条。独立弹窗空间宽裕默认给 260px，嵌入右侧工具坞
   （~380px 总宽）给 150px——之前写死 320px，编辑区会被挤到只剩几十像素。 */
.file-tool-body {
  flex: 1;
  min-height: 0;
  display: grid;
  background: var(--app-surface);
}

.file-tool-editor-pane {
  min-width: 0;
  min-height: 0;
  border-right: 1px solid var(--app-border);
}

.file-tool-tree-pane {
  position: relative; /* 给 .file-tool-tree-resize 手柄定位 */
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--app-surface-2);
}

/* 树左边框拖拽调宽手柄：贴着树的左边界，往编辑区那侧多留 2px 命中范围，
   跟 .tool-dock-resize-handle（整个工具坞的调宽手柄）同一套视觉语言。 */
.file-tool-tree-resize {
  position: absolute;
  left: -2px;
  top: 0;
  bottom: 0;
  width: 5px;
  cursor: col-resize;
  z-index: 5;
}
.file-tool-tree-resize:hover,
.file-tool-tree-resize:active {
  background: var(--app-accent-soft);
}

/* 树头：项目名在这——不再放在顶栏面包屑那一行（面包屑现在只显示当前打开的文件），
   右边两个图标按钮换掉原来那条"常驻输入框+刷新按钮"的老 topbar。 */
.file-tool-tree-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 8px 8px 6px;
  flex-shrink: 0;
}
.file-tool-tree-project {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  font-weight: 700;
  letter-spacing: 0.3px;
  color: var(--app-text-soft);
  min-width: 0;
}
/* 搜索：点搜索图标后整块替换树视图，Aa/全字/正则三个小按钮是真开关 */
.file-tool-search-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 30px;
  margin: 0 8px 6px;
  padding: 0 6px;
  border: 1px solid var(--app-border);
  border-radius: 7px;
  background: var(--app-surface);
  color: var(--app-text-faint);
  flex-shrink: 0;
}
.file-tool-search-bar input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12px;
  color: var(--app-text);
}
.search-toggle-btn {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--app-text-faint);
  font-size: 10px;
  font-weight: 700;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  cursor: pointer;
}
.search-toggle-btn:hover { background: var(--app-surface-3); }
.search-toggle-btn.on { background: var(--app-accent-soft); color: var(--app-accent); }

.file-tool-tree-body {
  flex: 1;
  overflow-y: auto;
  padding: 4px 4px 12px;
}

.file-tool-tree-msg {
  padding: 14px 8px;
  font-size: 12px;
  color: var(--app-text-faint);
}

.file-tool-tree-msg.error {
  color: #d94834;
}

.spin {
  animation: file-tool-spin 0.9s linear infinite;
}

@keyframes file-tool-spin {
  to { transform: rotate(360deg); }
}

.file-tool-editor-pane :deep(.code-editor-panel) {
  border-left: none;
}

.file-tool-editor-pane :deep(.editor-tabs) {
  background: var(--app-surface);
  border-bottom: 1px solid var(--app-border);
}

.file-tool-editor-pane :deep(.editor-tab) {
  border-right: none;
  border-radius: 0;
  margin: 0;
  background: transparent;
  color: var(--app-text-faint);
}

.file-tool-editor-pane :deep(.editor-tab.active) {
  background: var(--app-surface);
  color: var(--app-text);
  box-shadow: inset 0 -2px 0 var(--app-accent);
}
</style>
