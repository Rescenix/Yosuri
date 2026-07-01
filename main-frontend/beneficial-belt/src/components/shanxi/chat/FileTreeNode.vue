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
      <Icon v-else icon="mdi:file-outline" width="16" class="node-icon" />
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
  cursor: pointer; font-size: 12px; color: #4a4540;
  border-radius: 4px; margin: 0 4px;
}
.tree-node:hover { background: #f2ede3; }
.tree-node.active { background: #e8e3d8; font-weight: 600; }
.node-icon { margin-right: 6px; flex-shrink: 0; }
.node-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.file-context-menu {
  position: fixed;
  z-index: 9999;
  background: #fff;
  border: 1px solid #d4cfc4;
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
  color: #4a4540;
}
.file-context-menu button:hover {
  background: #f0ede3;
}
</style>