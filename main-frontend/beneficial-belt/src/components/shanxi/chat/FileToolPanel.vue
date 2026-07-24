<template>
  <div v-if="embedded" class="file-tool-shell" @keydown="onKeydown">
    <div class="file-tool-card embedded-mode">
      <div class="file-tool-mainbar">
        <div class="file-tool-breadcrumb">
          <span class="crumb-root">re0</span>
          <span class="crumb-sep">›</span>
          <span class="crumb-path">{{ activeTab?.path || selectedNode?.path || '选择文件开始编辑' }}</span>
        </div>
        <div class="file-tool-mainbar-actions">
          <span v-if="saveState" class="file-tool-save-state" :class="saveState">
            {{ saveState === 'saving' ? '保存中…' : saveState === 'saved' ? '已保存' : '保存失败' }}
          </span>
          <button
            class="file-tool-main-btn"
            type="button"
            :disabled="!selectedFileNode"
            @click="openSelectedNode"
          >
            <Icon icon="mdi:open-in-new" width="14" />
            打开
          </button>
          <button
            class="file-tool-main-btn primary"
            type="button"
            :disabled="!activeTab || !isDirty(activeTab) || saveState === 'saving'"
            @click="saveActiveFile"
          >
            <Icon icon="mdi:content-save-outline" width="14" />
            保存
          </button>
        </div>
      </div>

      <div class="file-tool-body">
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

        <aside class="file-tool-tree-pane">
          <div class="file-tool-tree-topbar">
            <button class="file-tool-icon-btn" type="button" title="刷新" @click="loadTree">
              <Icon icon="mdi:refresh" width="15" :class="{ spin: treeLoading }" />
            </button>
            <div class="file-tool-tree-search">
              <Icon icon="mdi:magnify" width="14" />
              <input v-model="treeQuery" type="text" placeholder="筛选文件…" />
            </div>
          </div>

          <div class="file-tool-tree-body">
            <div v-if="treeLoading" class="file-tool-tree-msg">加载中…</div>
            <div v-else-if="treeError" class="file-tool-tree-msg error">{{ treeError }}</div>
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
            <span class="crumb-path">{{ activeTab?.path || selectedNode?.path || '选择文件开始编辑' }}</span>
          </div>
          <div class="file-tool-mainbar-actions">
            <span v-if="saveState" class="file-tool-save-state" :class="saveState">
              {{ saveState === 'saving' ? '保存中…' : saveState === 'saved' ? '已保存' : '保存失败' }}
            </span>
            <button class="file-tool-main-btn" type="button" :disabled="!selectedFileNode" @click="openSelectedNode">
              <Icon icon="mdi:open-in-new" width="14" />
              打开
            </button>
            <button class="file-tool-main-btn primary" type="button" :disabled="!activeTab || !isDirty(activeTab) || saveState === 'saving'" @click="saveActiveFile">
              <Icon icon="mdi:content-save-outline" width="14" />
              保存
            </button>
            <button class="file-tool-icon-btn" type="button" @click="requestClose" title="关闭 (Esc)">
              <Icon icon="mdi:close" width="18" />
            </button>
          </div>
        </div>

        <div class="file-tool-body">
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

          <aside class="file-tool-tree-pane">
            <div class="file-tool-tree-topbar">
              <button class="file-tool-icon-btn" type="button" title="刷新" @click="loadTree">
                <Icon icon="mdi:refresh" width="15" :class="{ spin: treeLoading }" />
              </button>
              <div class="file-tool-tree-search">
                <Icon icon="mdi:magnify" width="14" />
                <input v-model="treeQuery" type="text" placeholder="筛选文件…" />
              </div>
            </div>

            <div class="file-tool-tree-body">
              <div v-if="treeLoading" class="file-tool-tree-msg">加载中…</div>
              <div v-else-if="treeError" class="file-tool-tree-msg error">{{ treeError }}</div>
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
          </aside>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import CodeEditor from './CodeEditor.vue'
import FileTreeNode from './FileTreeNode.vue'

const emit = defineEmits(['close'])
const props = defineProps({
  embedded: { type: Boolean, default: false }
})

const treeNodes = ref([])
const treeLoading = ref(false)
const treeError = ref('')
const treeQuery = ref('')
const selectedNode = ref(null)
const tabs = ref([])
const activeFilePath = ref('')
const openError = ref('')
const pinnedPaths = ref([])
const saveState = ref('')
let saveStateTimer = null

const activeTab = computed(() => tabs.value.find(t => t.path === activeFilePath.value) || null)
const selectedFileNode = computed(() => (selectedNode.value?.type === 'file' ? selectedNode.value : null))

const filteredTreeNodes = computed(() => {
  const q = treeQuery.value.trim().toLowerCase()
  if (!q) return treeNodes.value
  return filterTree(treeNodes.value, q)
})

function filterTree(nodes, query) {
  const result = []
  for (const node of nodes || []) {
    const selfMatch = (node.name || '').toLowerCase().includes(query) || (node.path || '').toLowerCase().includes(query)
    if (node.type === 'folder') {
      const children = filterTree(node.children || [], query)
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

function onSelectNode(node) {
  const wasSameFile = selectedNode.value?.type === 'file' && selectedNode.value?.path === node.path
  selectedNode.value = node
  if (node.type === 'folder') {
    node.expanded = !node.expanded
    return
  }
  if (activeFilePath.value === node.path) return
  if (wasSameFile) {
    openSelectedNode()
  }
}

async function openSelectedNode() {
  const node = selectedFileNode.value
  if (!node) return
  openError.value = ''
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
  activeFilePath.value = path
  const matched = tabs.value.find(t => t.path === path)
  if (matched) selectedNode.value = { type: 'file', path: matched.path, name: matched.name }
}

function closeFile(path) {
  const tab = tabs.value.find(t => t.path === path)
  if (tab && isDirty(tab)) {
    if (!window.confirm(`「${tab.name}」还有未保存的修改，确定要关闭吗？`)) return
  }
  const idx = tabs.value.findIndex(t => t.path === path)
  if (idx === -1) return
  tabs.value.splice(idx, 1)
  if (activeFilePath.value === path) {
    activeFilePath.value = tabs.value[Math.max(0, idx - 1)]?.path || ''
  }
}

function onUpdateContent(val) {
  const tab = activeTab.value
  if (tab) tab.content = val
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

function onKeydown(e) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    saveActiveFile()
  }
}

function requestClose() {
  const dirtyTabs = tabs.value.filter(isDirty)
  if (dirtyTabs.length && !window.confirm(`还有 ${dirtyTabs.length} 个文件未保存，确定要关闭吗？（改动不会保留）`)) {
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

onMounted(loadTree)
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

.file-tool-mainbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 44px;
  padding: 0 12px;
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
  gap: 6px;
  flex-shrink: 0;
}

.file-tool-save-state {
  font-size: 11px;
  color: var(--app-text-faint);
}

.file-tool-save-state.saved { color: #12b76a; }
.file-tool-save-state.error { color: #d94834; }

.file-tool-main-btn,
.file-tool-icon-btn {
  border: 1px solid var(--app-border);
  background: var(--app-surface);
  color: var(--app-text);
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.file-tool-main-btn {
  height: 28px;
  padding: 0 10px;
  font-size: 12px;
  font-weight: 600;
}

.file-tool-main-btn.primary {
  background: var(--app-accent);
  color: #fff;
  border-color: var(--app-accent);
}

.file-tool-icon-btn {
  width: 28px;
  height: 28px;
}

.file-tool-main-btn:hover:not(:disabled),
.file-tool-icon-btn:hover:not(:disabled) {
  background: var(--app-surface-3);
}

.file-tool-main-btn.primary:hover:not(:disabled) {
  background: var(--app-accent-hover);
  border-color: var(--app-accent-hover);
}

.file-tool-main-btn:disabled,
.file-tool-icon-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

/* 独立弹窗（Teleport 到 body）里空间宽裕，树占比可以大方一点；
   嵌入右侧工具坞时整个面板可能只有 ~380px 宽，320px 的固定树宽会把编辑区
   挤到只剩几十像素——这正是"文件树占大部分、编辑区被截断"那个问题的根源。 */
.file-tool-body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 260px;
  background: var(--app-surface);
}

.file-tool-card.embedded-mode .file-tool-body {
  grid-template-columns: minmax(0, 1fr) 150px;
}

.file-tool-editor-pane {
  min-width: 0;
  min-height: 0;
  border-right: 1px solid var(--app-border);
}

.file-tool-tree-pane {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--app-surface-2);
}

.file-tool-tree-topbar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-surface);
  flex-shrink: 0;
}

.file-tool-tree-search {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--app-border);
  border-radius: 7px;
  background: var(--app-surface-2);
  color: var(--app-text-faint);
}

.file-tool-tree-search input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12px;
  color: var(--app-text);
}

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
