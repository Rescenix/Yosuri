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
  selected: { type: Object, default: null }
})

const emit = defineEmits(['select', 'toggle'])

const showMenu = ref(false)
const menuX = ref(0)
const menuY = ref(0)

const FILE_BADGES = {
  js: { bg: '#f4d35e', color: '#4a3b06', label: 'JS' },
  json: { bg: '#eab308', color: '#3d2b06', label: '{}' },
  vue: { bg: '#42b883', color: '#ffffff', label: 'V' },
  py: { bg: '#4a9d6d', color: '#ffffff', label: 'PY' },
  ps1: { bg: '#5b8def', color: '#ffffff', label: 'PS' },
  txt: { bg: '#9a958a', color: '#ffffff', label: '≡' }
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
  const absolutePath = `C:\\Pro2026\\re0\\${relativePath}`
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
  cursor: pointer; font-size: 12px; color: #475569;
  border-radius: 4px; margin: 0 4px;
}
.tree-node:hover { background: #e2e8f0; }
.tree-node.active { background: #e2e8f0; font-weight: 600; }
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
  background: #fff;
  border: 1px solid #e5e5e5;
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
  color: #475569;
}
.file-context-menu button:hover {
  background: #f1f5f9;
}
</style>