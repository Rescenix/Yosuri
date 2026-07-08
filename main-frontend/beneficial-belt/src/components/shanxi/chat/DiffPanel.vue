<template>
  <div class="diff-panel">
    <div v-if="files.length === 0" class="diff-empty">
      <Icon icon="mdi:file-compare" width="24" color="#c4c4c4" />
      <span>本次会话还没有文件改动</span>
    </div>
    <div v-else class="diff-body">
      <div class="diff-file-card" v-for="df in files" :key="df.path">
        <div class="diff-file-head" @click="$emit('toggle-file', df.path)">
          <span class="diff-chev" :class="{ open: !!expandedDiffs[df.path] }">›</span>
          <span class="diff-file-name">{{ fileBaseName(df.path) }}</span>
          <span class="diff-adds">+{{ stats(df).added }}</span>
          <span class="diff-dels">−{{ stats(df).removed }}</span>
        </div>
        <div v-if="expandedDiffs[df.path]" class="diff-rows">
          <DiffViewer :old-content="df.oldContent" :new-content="df.newContent" :path="df.path" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { Icon } from '@iconify/vue'
import { diffLines } from 'diff'
import { fileBaseName } from './toolArgs.js'
import DiffViewer from './DiffViewer.vue'

defineProps({
  files: { type: Array, default: () => [] },
  expandedDiffs: { type: Object, default: () => ({}) }
})
defineEmits(['toggle-file'])

// 只给文件行标题的 +N/-N 用，真正的逐行渲染在 DiffViewer 里自己算一遍
function stats(df) {
  const parts = diffLines(df.oldContent || '', df.newContent || '')
  let added = 0, removed = 0
  for (const p of parts) {
    const lines = p.value.split('\n')
    if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop()
    if (p.added) added += lines.length
    else if (p.removed) removed += lines.length
  }
  return { added, removed }
}
</script>

<style scoped>
.diff-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.diff-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #a3a3a3;
  font-size: 12.5px;
}

.diff-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
  padding: 10px 10px 14px;
}

.diff-file-card {
  border: 1px solid #e5e5e5;
  border-radius: 10px;
  margin-bottom: 10px;
  overflow: hidden;
  background: #ffffff;
}
.diff-file-head {
  display: flex;
  align-items: baseline;
  gap: 7px;
  padding: 8px 12px;
  cursor: pointer;
  background: #f5f5f5;
}
.diff-chev { align-self: center; }
.diff-chev {
  display: inline-block;
  font-size: 12px;
  color: #a3a3a3;
  transition: transform 0.15s ease;
}
.diff-chev.open { transform: rotate(90deg); }
.diff-file-name {
  flex: 1;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.diff-adds, .diff-dels {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
.diff-adds { color: #12b76a; }
.diff-dels { color: #d94834; }

.diff-rows { border-top: 1px solid #e5e5e5; }
</style>
