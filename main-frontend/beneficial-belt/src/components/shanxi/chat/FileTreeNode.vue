<template>
  <div>
    <div
      class="tree-node"
      :style="{ paddingLeft: depth * 16 + 8 + 'px' }"
      :class="{ active: selected?.name === node.name && node.type === 'file' }"
      @click="handleClick"
    >
      <Icon
        v-if="node.type === 'folder'"
        :icon="node.expanded ? 'mdi:folder-open-outline' : 'mdi:folder-outline'"
        width="16" class="node-icon"
      />
      <Icon v-else icon="mdi:file-outline" width="16" class="node-icon" />
      <span class="node-name">{{ node.name }}</span>
    </div>
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
import { Icon } from '@iconify/vue'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
  selected: { type: Object, default: null }
})

const emit = defineEmits(['select', 'toggle'])

function handleClick() {
  if (props.node.type === 'folder') {
    emit('toggle', props.node)
  } else {
    emit('select', props.node)
  }
}
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
</style>