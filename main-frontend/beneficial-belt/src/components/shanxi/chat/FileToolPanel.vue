<template>
  <div v-if="embedded" class="file-tool-shell" @keydown="onKeydown">
    <div class="file-tool-card embedded-mode">
      <div class="file-tool-body" :style="{ gridTemplateColumns: `minmax(0,1fr) ${treeCollapsed ? 0 : treeWidth}px` }">
        <section class="file-tool-editor-pane">
          <CodeEditor
            :tabs="tabs"
            :active-file-path="activeFilePath"
            :file-content="activeTab?.content ?? ''"
            :language="activeTab ? languageOf(activeTab.name) : 'plaintext'"
            :pinned-paths="pinnedPaths"
            :external-changes="[...externalChanges]"
            @update:content="onUpdateContent"
            @switch-file="switchFile"
            @close-file="closeFile"
            @pin-file="pinFile"
            @unpin-file="unpinFile"
          >
            <template #tab-actions>
              <button class="file-tool-run-btn" type="button" :disabled="!activeTab || !runnableCommand" :title="runnableCommand ? '保存并在终端运行当前文件' : '当前文件暂不支持一键运行'" @click="runActiveFile">
                <Icon icon="mdi:play" width="15" />
                <span>运行</span>
              </button>
              <span v-if="saveState" class="file-tool-save-state" :class="saveState">
                {{ saveState === 'saving' ? '保存中…' : saveState === 'saved' ? '已保存' : '保存失败' }}
              </span>
              <button class="file-tool-icon-btn" type="button" @click="treeCollapsed = !treeCollapsed" :title="treeCollapsed ? '显示文件树' : '隐藏文件树'">
                <Icon icon="lucide:sidebar" width="16" />
              </button>
            </template>
          </CodeEditor>
        </section>

        <aside v-if="!treeCollapsed" class="file-tool-tree-pane">
          <div class="file-tool-tree-resize" title="拖拽调整宽度" @mousedown="startTreeResize"></div>
          <div class="file-tool-tree-header">
            <span class="file-tool-tree-project">
              <Icon icon="mdi:folder-outline" width="14" />
              <span>{{ workdirName || '项目' }}</span>
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
                @create-file="createFile"
                @create-folder="createFolder"
                @rename="renameNode"
                @delete="deleteNode"
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
              @create-file="createFile"
              @create-folder="createFolder"
              @rename="renameNode"
              @delete="deleteNode"
            />
          </div>
        </aside>
      </div>
    </div>
  </div>

  <Teleport v-else to="body">
    <div class="file-tool-backdrop" @click="requestClose" @keydown.esc="requestClose">
      <div class="file-tool-card" @click.stop @keydown="onKeydown">
        <div class="file-tool-body" :style="{ gridTemplateColumns: `minmax(0,1fr) ${treeCollapsed ? 0 : treeWidth}px` }">
          <section class="file-tool-editor-pane">
            <CodeEditor
              :tabs="tabs"
              :active-file-path="activeFilePath"
              :file-content="activeTab?.content ?? ''"
              :language="activeTab ? languageOf(activeTab.name) : 'plaintext'"
              :pinned-paths="pinnedPaths"
              :external-changes="[...externalChanges]"
              @update:content="onUpdateContent"
              @switch-file="switchFile"
              @close-file="closeFile"
              @pin-file="pinFile"
              @unpin-file="unpinFile"
            >
              <template #tab-actions>
                <button class="file-tool-run-btn" type="button" :disabled="!activeTab || !runnableCommand" :title="runnableCommand ? '保存并在终端运行当前文件' : '当前文件暂不支持一键运行'" @click="runActiveFile">
                  <Icon icon="mdi:play" width="15" />
                  <span>运行</span>
                </button>
                <span v-if="saveState" class="file-tool-save-state" :class="saveState">
                  {{ saveState === 'saving' ? '保存中…' : saveState === 'saved' ? '已保存' : '保存失败' }}
                </span>
                <button class="file-tool-icon-btn" type="button" @click="treeCollapsed = !treeCollapsed" :title="treeCollapsed ? '显示文件树' : '隐藏文件树'">
                  <Icon icon="lucide:sidebar" width="16" />
                </button>
                <button class="file-tool-icon-btn" type="button" @click="requestClose" title="关闭 (Esc)">
                  <Icon icon="mdi:close" width="18" />
                </button>
              </template>
            </CodeEditor>
          </section>

          <aside v-if="!treeCollapsed" class="file-tool-tree-pane">
            <div class="file-tool-tree-resize" title="拖拽调整宽度" @mousedown="startTreeResize"></div>
            <div class="file-tool-tree-header">
              <span class="file-tool-tree-project">
                <Icon icon="mdi:folder-outline" width="14" />
                <span>{{ workdirName || '项目' }}</span>
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
                  @create-file="createFile"
                  @create-folder="createFolder"
                  @rename="renameNode"
                  @delete="deleteNode"
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
                @create-file="createFile"
                @create-folder="createFolder"
                @rename="renameNode"
                @delete="deleteNode"
              />
            </div>
          </aside>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch, defineAsyncComponent } from 'vue'
import { Icon } from '@iconify/vue'
import { useEditorPrefs } from '../composables/useEditorPrefs.js'

// CodeEditor 是 Monaco 编辑器（5MB+ JS + 4 个 worker），默认按需加载（懒加载），
// 用户关闭懒加载时应用启动后后台预取，打开文件面板秒开。
const { editorLazy } = useEditorPrefs()
const CodeEditor = defineAsyncComponent(() => import('./CodeEditor.vue'))
let editorPrefetchStarted = false
function prefetchEditor() {
  if (editorPrefetchStarted) return
  editorPrefetchStarted = true
  import('./CodeEditor.vue').catch(() => {})
}
// 如果关闭了懒加载，立即预取编辑器
watch(editorLazy, (lazy) => { if (!lazy) prefetchEditor() }, { immediate: true })
import FileTreeNode from './FileTreeNode.vue'
import { useResizableWidth } from './useResizable.js'

const emit = defineEmits(['close', 'run-command'])
const props = defineProps({
  embedded: { type: Boolean, default: false },
  workdirPath: { type: String, default: '' },
  workdirName: { type: String, default: '' }
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
let saveStateTimer = null
const externalChanges = ref(new Set())
let fileChangesStream = null

const activeTab = computed(() => tabs.value.find(t => t.path === activeFilePath.value) || null)
const runnableCommand = computed(() => activeTab.value ? commandFor(activeTab.value) : '')

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

function validNewPath(path) {
  return path && !path.startsWith('/') && !path.startsWith('\\') && !path.split(/[\\/]/).includes('..')
}

async function createFile(folder) {
  const name = window.prompt(`在「${folder.name}」中新建文件（例如 hello.py）`)?.trim()
  if (!name) return
  if (!validNewPath(name) || name.includes('/')) return window.alert('请输入文件名，不要包含路径')
  const path = `${folder.path}/${name}`
  try {
    const res = await fetch('/api/file', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path, content: '' }) })
    if (!res.ok) throw new Error(await res.text())
    await loadTree()
    const tab = { path, name: path.split('/').pop(), content: '', savedContent: '' }
    tabs.value.push(tab)
    activeFilePath.value = path
    selectedNode.value = { type: 'file', path, name: tab.name }
  } catch (e) { window.alert('新建文件失败：' + (e.message || '未知错误')) }
}

async function createFolder(folder) {
  const name = window.prompt(`在「${folder.name}」中新建文件夹`)?.trim()
  if (!name) return
  if (!validNewPath(name) || name.includes('/')) return window.alert('请输入文件夹名，不要包含路径')
  const path = `${folder.path}/${name}`
  try {
    const res = await fetch('/api/folder', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path }) })
    if (!res.ok) throw new Error(await res.text())
    await loadTree()
  } catch (e) { window.alert('新建文件夹失败：' + (e.message || '未知错误')) }
}

async function renameNode(node) {
  const name = window.prompt(`重命名「${node.name}」`, node.name)?.trim()
  if (!name || name === node.name) return
  if (!validNewPath(name) || name.includes('/')) return window.alert('请输入名称，不要包含路径')
  const parent = node.path.includes('/') ? node.path.slice(0, node.path.lastIndexOf('/')) : ''
  const newPath = parent ? `${parent}/${name}` : name
  try {
    const res = await fetch('/api/file/rename', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path: node.path, newPath }) })
    if (!res.ok) throw new Error(await res.text())
    tabs.value.filter(tab => tab.path === node.path || tab.path.startsWith(node.path + '/')).forEach(tab => closeFile(tab.path))
    await loadTree()
  } catch (e) { window.alert('重命名失败：' + (e.message || '未知错误')) }
}

async function deleteNode(node) {
  const detail = node.type === 'folder' ? '及其所有内容' : ''
  if (!window.confirm(`确定删除「${node.name}」${detail}？此操作不可恢复。`)) return
  try {
    const res = await fetch('/api/file?path=' + encodeURIComponent(node.path), { method: 'DELETE' })
    if (!res.ok) throw new Error(await res.text())
    tabs.value.filter(tab => tab.path === node.path || tab.path.startsWith(node.path + '/')).forEach(tab => closeFile(tab.path))
    await loadTree()
  } catch (e) { window.alert('删除失败：' + (e.message || '未知错误')) }
}

function quotePowerShell(value) { return "'" + value.replaceAll("'", "''") + "'" }
function commandFor(tab) {
  const path = quotePowerShell(tab.path)
  switch (languageOf(tab.name)) {
    case 'typescript': return `npx --no-install tsx ${path}`
    case 'python': return `python ${path}`
    case 'go': return `go run ${path}`
    case 'rust': return `rustc ${path} -o \"$env:TEMP\\rescene-lesson.exe\"; if ($LASTEXITCODE -eq 0) { & \"$env:TEMP\\rescene-lesson.exe\" }`
    default: return ''
  }
}

async function runActiveFile() {
  if (!runnableCommand.value) return
  clearTimeout(autoSaveTimer)
  // 一键运行必须等保存成功；否则终端很容易读到上一版文件，录教学视频会显得像“代码没生效”。
  if (await saveActiveFile() === false) return
  emit('run-command', runnableCommand.value)
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

async function refreshChangedFile(path) {
  const tab = tabs.value.find(t => t.path === path)
  if (!tab) return
  try {
    const res = await fetch('/api/file?path=' + encodeURIComponent(path))
    if (!res.ok) throw new Error(await res.text())
    const content = await res.text()
    // 保留用户正输入但尚未落盘的内容，绝不让另一个 agent 的写入静默覆盖它。
    if (isDirty(tab)) {
      if (content !== tab.content) externalChanges.value = new Set([...externalChanges.value, path])
      return
    }
    if (content !== tab.content) {
      tab.content = content
      tab.savedContent = content
    }
    const next = new Set(externalChanges.value)
    next.delete(path)
    externalChanges.value = next
  } catch {
    // 文件可能在 agent 重命名/删除的中间态；下一次事件或手动打开会给出具体错误。
  }
}

function connectFileChanges() {
  fileChangesStream?.close()
  fileChangesStream = null
  const paths = tabs.value.map(tab => tab.path)
  if (!paths.length) return
  fileChangesStream = new EventSource('/api/file/changes?paths=' + encodeURIComponent(paths.join(',')))
  fileChangesStream.addEventListener('changed', (event) => {
    try { refreshChangedFile(JSON.parse(event.data).path) } catch {}
  })
}

watch(() => tabs.value.map(tab => tab.path).join('\u0000'), connectFileChanges)

// 对话切换项目时，文件 API 的根目录也随之切换。清掉旧标签，避免同名相对路径
// 在新项目里被误保存或继续展示；文件树则立即从新的根目录重载。
watch(() => props.workdirPath, (next, previous) => {
  if (!next || next === previous) return
  fileChangesStream?.close()
  fileChangesStream = null
  clearTimeout(autoSaveTimer)
  tabs.value = []
  activeFilePath.value = ''
  selectedNode.value = null
  externalChanges.value = new Set()
  treeQuery.value = ''
  loadTree()
})

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
  const nextChanges = new Set(externalChanges.value)
  nextChanges.delete(path)
  externalChanges.value = nextChanges
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
  if (!tab || !isDirty(tab)) return true
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
    return true
  } catch (e) {
    saveState.value = 'error'
    window.alert('保存失败，未运行当前文件。')
    return false
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

onMounted(loadTree)
onUnmounted(() => {
  clearTimeout(autoSaveTimer)
  clearTimeout(saveStateTimer)
  fileChangesStream?.close()
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

.file-tool-save-state {
  font-size: 11px;
  color: var(--app-text-faint);
  padding: 0 4px;
}
.file-tool-run-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: 25px;
  margin-right: 2px;
  padding: 0 7px;
  border: 1px solid color-mix(in srgb, var(--app-accent) 46%, var(--app-border));
  border-radius: 5px;
  background: color-mix(in srgb, var(--app-accent) 12%, var(--app-surface));
  color: var(--app-accent);
  font-size: 11px;
  font-weight: 650;
  cursor: pointer;
}
.file-tool-run-btn:hover:not(:disabled) { background: color-mix(in srgb, var(--app-accent) 20%, var(--app-surface)); }
.file-tool-run-btn:disabled { opacity: .4; cursor: not-allowed; }

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
