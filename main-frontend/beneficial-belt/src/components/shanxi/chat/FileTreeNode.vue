<template>
  <div>
    <div
      class="tree-node"
      :style="{ paddingLeft: depth * 16 + 8 + 'px' }"
      :class="{ active: selected?.name === node.name && node.type === 'file' }"
      @click="handleClick"
      @contextmenu.prevent="onRightClick"
    >
      <Icon
        v-if="node.type === 'folder'"
        :icon="node.expanded ? 'mdi:folder-open-outline' : 'mdi:folder-outline'"
        width="16" class="node-icon"
      />
      <span
        v-else
        class="file-badge"
        :style="{ background: fileBadge(node.name).bg, color: fileBadge(node.name).color }"
      >{{ fileBadge(node.name).label }}</span>
      <span class="node-name" :title="node.name">{{ node.name }}</span>
    </div>

    <!-- 右键菜单 -->
    <Teleport to="body">
      <div
        v-if="showMenu"
        class="file-context-menu"
        :style="{ top: menuY + 'px', left: menuX + 'px' }"
        @click.stop
      >
        <button @click.stop="handleCopyPath">复制路径</button>
        <button @click.stop="handleCopyName">复制文件名</button>
        <button v-if="node.type === 'file'" @click.stop="handleOpenFile">在编辑器中打开</button>
        <template v-if="node.type === 'folder'">
          <div class="menu-separator"></div>
          <button @click.stop="emitAction('create-file')">新建文件</button>
          <button @click.stop="emitAction('create-folder')">新建文件夹</button>
        </template>
        <div class="menu-separator"></div>
        <button @click.stop="emitAction('rename')">重命名</button>
        <button class="danger" @click.stop="emitAction('delete')">删除</button>
      </div>
    </Teleport>

    <template v-if="node.type === 'folder' && node.expanded">
      <FileTreeNode
        v-for="child in node.children"
        :key="child.name"
        :node="child" :depth="depth + 1"
        :selected="selected"
        @select="$emit('select', $event)"
        @toggle="$emit('toggle', $event)"
        @create-file="$emit('create-file', $event)"
        @create-folder="$emit('create-folder', $event)"
        @rename="$emit('rename', $event)"
        @delete="$emit('delete', $event)"
      />
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
  selected: { type: Object, default: null },
  workdirPath: { type: String, default: '' }
})

const emit = defineEmits(['select', 'toggle', 'create-file', 'create-folder', 'rename', 'delete'])

const showMenu = ref(false)
const menuX = ref(0)
const menuY = ref(0)

const FILE_BADGES = {
  js:   { bg: '#f4d35e', color: '#4a3b06', label: 'JS' },
  jsx:  { bg: '#f4d35e', color: '#4a3b06', label: 'RX' },
  ts:   { bg: '#3178c6', color: '#ffffff', label: 'TS' },
  tsx:  { bg: '#3178c6', color: '#ffffff', label: 'TX' },
  json: { bg: '#eab308', color: '#3d2b06', label: '{}' },
  vue:  { bg: '#42b883', color: '#ffffff', label: 'V' },
  py:   { bg: '#4a9d6d', color: '#ffffff', label: 'PY' },
  go:   { bg: '#00add8', color: '#ffffff', label: 'Go' },
  rs:   { bg: '#dea584', color: '#1a1a2e', label: 'RS' },
  java: { bg: '#b07219', color: '#ffffff', label: 'JV' },
  kt:   { bg: '#7f52ff', color: '#ffffff', label: 'KT' },
  css:  { bg: '#663399', color: '#ffffff', label: '#' },
  scss: { bg: '#c6538c', color: '#ffffff', label: '$' },
  html: { bg: '#e44d26', color: '#ffffff', label: 'H' },
  md:   { bg: '#4a9dae', color: '#ffffff', label: 'M' },
  yaml: { bg: '#6cb2d6', color: '#1a1a2e', label: 'Y' },
  yml:  { bg: '#6cb2d6', color: '#1a1a2e', label: 'Y' },
  toml: { bg: '#8bc34a', color: '#1a1a2e', label: 'T' },
  sh:   { bg: '#4CAF50', color: '#ffffff', label: '>' },
  bash: { bg: '#4CAF50', color: '#ffffff', label: '>' },
  zsh:  { bg: '#4CAF50', color: '#ffffff', label: '>' },
  ps1:  { bg: '#5b8def', color: '#ffffff', label: 'PS' },
  bat:  { bg: '#2d2d2d', color: '#ffffff', label: '&' },
  cmd:  { bg: '#2d2d2d', color: '#ffffff', label: '&' },
  dockerfile: { bg: '#2496ed', color: '#ffffff', label: 'D' },
  yml:  { bg: '#2496ed', color: '#ffffff', label: 'D' },
  gitignore:  { bg: '#e34c26', color: '#ffffff', label: '!' },
  env:  { bg: '#f5a623', color: '#1a1a2e', label: '☰' },
  ini:  { bg: '#9e9e9e', color: '#ffffff', label: '⚙' },
  cfg:  { bg: '#9e9e9e', color: '#ffffff', label: '⚙' },
  conf: { bg: '#9e9e9e', color: '#ffffff', label: '⚙' },
  txt:  { bg: '#9a958a', color: '#ffffff', label: '≡' },
  log:  { bg: '#9a958a', color: '#ffffff', label: '📋' },
  sql:  { bg: '#e38d13', color: '#ffffff', label: 'SQ' },
  csv:  { bg: '#217346', color: '#ffffff', label: 'CS' },
  xml:  { bg: '#0060ac', color: '#ffffff', label: '<>' },
  svg:  { bg: '#ffb13b', color: '#1a1a2e', label: 'SV' },
  png:  { bg: '#a174d6', color: '#ffffff', label: '🖼' },
  jpg:  { bg: '#a174d6', color: '#ffffff', label: '🖼' },
  jpeg: { bg: '#a174d6', color: '#ffffff', label: '🖼' },
  webp: { bg: '#a174d6', color: '#ffffff', label: '🖼' },
  gif:  { bg: '#a174d6', color: '#ffffff', label: '🖼' },
  ico:  { bg: '#a174d6', color: '#ffffff', label: '🖼' },
  pdf:  { bg: '#d32f2f', color: '#ffffff', label: 'PDF' },
  zip:  { bg: '#ff9800', color: '#ffffff', label: '📦' },
  tar:  { bg: '#ff9800', color: '#ffffff', label: '📦' },
  gz:   { bg: '#ff9800', color: '#ffffff', label: '📦' },
  lock: { bg: '#607d8b', color: '#ffffff', label: '🔒' },
}
const DEFAULT_BADGE = { bg: '#9a958a', color: '#ffffff', label: '•' }

function fileBadge(name) {
  if (/^LICENSE$/i.test(name)) return DEFAULT_BADGE
  const ext = name.split('.').pop()?.toLowerCase()
  return FILE_BADGES[ext] || DEFAULT_BADGE
}

function handleClick() {
  if (props.node.type === 'folder') {
    emit('toggle', props.node)
  } else {
    emit('select', props.node)
  }
}

function onRightClick(event) {
  event.stopPropagation()
  menuX.value = event.clientX
  menuY.value = event.clientY
  showMenu.value = true
  // 延迟注册全局点击监听，避免右键点击本身触发关闭
  setTimeout(() => {
    document.addEventListener('click', closeMenu, { once: true })
  }, 0)
}

function closeMenu(event) {
  // 如果点击的是菜单内部元素，不关闭
  if (event.target.closest('.file-context-menu')) return
  showMenu.value = false
}

function handleCopyPath() {
  const relativePath = props.node.path || props.node.name
  const root = props.workdirPath || document.querySelector('.file-tool-tree-project span')?.textContent || ''
  const absolutePath = root ? `${root.replace(/[/\\]$/, '')}/${relativePath}` : relativePath
  navigator.clipboard.writeText(absolutePath)
  showMenu.value = false
}

function handleCopyName() {
  navigator.clipboard.writeText(props.node.name)
  showMenu.value = false
}

function handleOpenFile() {
  emit('select', props.node)
  showMenu.value = false
}

function emitAction(action) {
  emit(action, props.node)
  showMenu.value = false
}

onMounted(() => {
  // 不需要全局监听，改用单次监听方式
})

onUnmounted(() => {
  document.removeEventListener('click', closeMenu)
})
</script>

<style scoped>
.tree-node {
  display: flex; align-items: center; padding: 4px 8px;
  cursor: pointer; font-size: 12px; color: var(--app-text);
  border-radius: 4px; margin: 0 4px;
}
.tree-node:hover { background: var(--app-surface-3); }
.tree-node.active { background: var(--app-surface-3); font-weight: 600; }
.node-icon { margin-right: 6px; flex-shrink: 0; }
.file-badge {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  margin-right: 6px;
  border-radius: 4px;
  font-size: 8.5px;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
  font-family: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.node-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.file-context-menu {
  position: fixed;
  z-index: 9999;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 6px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.15);
  display: flex;
  flex-direction: column;
  padding: 4px 0;
  min-width: 140px;
}
.file-context-menu button {
  padding: 6px 16px;
  text-align: left;
  border: none;
  background: none;
  font-size: 12px;
  cursor: pointer;
  color: var(--app-text);
}
.file-context-menu button:hover {
  background: var(--app-surface-3);
}
.file-context-menu .danger { color: #d94834; }
.file-context-menu .danger:hover { background: rgba(217, 72, 52, .1); }
.menu-separator { height: 1px; margin: 4px 0; background: var(--app-border); }
</style>
