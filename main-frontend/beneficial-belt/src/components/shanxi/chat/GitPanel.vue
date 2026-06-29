<template>
  <aside class="git-panel">
    <div class="panel-header">
      <Icon icon="mdi:source-branch" width="16" />
      <span>{{ status.branch || '...' }}</span>
      <span class="dot" :style="{ background: dirty ? '#f59e0b' : '#12b76a' }"></span>
    </div>

    <!-- 变更文件列表 -->
    <div class="section">
      <div class="section-title">暂存区 & 工作区</div>
      <div v-if="!dirty" class="clean-msg">工作区干净</div>
      <div
        v-for="f in status.modified"
        :key="'m-'+f"
        class="file-row"
        :class="{ active: selectedFile === f }"
        @click="selectFile(f)"
      >
        <Icon icon="mdi:file-document-edit-outline" width="14" color="#f59e0b" />
        <span>{{ f }}</span>
      </div>
      <div
        v-for="f in status.untracked"
        :key="'u-'+f"
        class="file-row"
        :class="{ active: selectedFile === f }"
        @click="selectFile(f)"
      >
        <Icon icon="mdi:plus-circle-outline" width="14" color="#12b76a" />
        <span>{{ f }}</span>
      </div>
    </div>

    <!-- Diff 预览 -->
    <div v-if="diffContent" class="section">
      <div class="section-title">Diff — {{ selectedFile }}</div>
      <pre class="diff-view"><code v-html="diffContent"></code></pre>
    </div>

    <!-- AI 快捷操作 -->
    <div class="actions">
      <button class="action-btn" @click="stageAll">Stage All</button>
      <button class="action-btn primary" @click="aiCommit">AI 生成 Commit</button>
    </div>
  </aside>
</template>

<script setup>
import { ref, computed, onMounted, defineEmits } from 'vue'
import { Icon } from '@iconify/vue'

const emit = defineEmits(['ai-commit'])

const status = ref({ branch: '', modified: [], untracked: [] })
const selectedFile = ref(null)
const diffContent = ref('')

const dirty = computed(() => status.value.modified.length > 0 || status.value.untracked.length > 0)

async function fetchStatus() {
  const res = await fetch('/api/git-status')
  if (res.ok) status.value = await res.json()
}

async function selectFile(file) {
  selectedFile.value = file
  const res = await fetch(`/api/git-diff?file=${encodeURIComponent(file)}`)
  if (res.ok) {
    const data = await res.json()
    diffContent.value = data.diff
  }
}

async function stageAll() {
  await fetch('/api/git-stage-all', { method: 'POST' })
  fetchStatus()
}

function aiCommit() {
  emit('ai-commit', status.value)
}

onMounted(fetchStatus)
</script>

<style scoped>
.git-panel {
  width: 320px; height: 100%;
  border-left: 1px solid #e4dfd4;
  background: #faf9f6;
  display: flex; flex-direction: column;
  font-size: 12px; color: #4a4540;
  overflow-y: auto;
}
.panel-header {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 12px; font-weight: 600;
  border-bottom: 1px solid #e4dfd4;
}
.dot { width: 8px; height: 8px; border-radius: 50%; margin-left: auto; }
.section { padding: 8px 12px; border-bottom: 1px solid #eee; }
.section-title { font-weight: 600; margin-bottom: 6px; color: #696259; }
.clean-msg { color: #12b76a; padding: 4px 0; }
.file-row {
  display: flex; align-items: center; gap: 6px;
  padding: 3px 4px; cursor: pointer; border-radius: 4px;
}
.file-row:hover { background: #f0ede5; }
.file-row.active { background: #e8e3d8; font-weight: 600; }
.diff-view {
  background: #1e1e2e; color: #cdd6f4; padding: 8px;
  border-radius: 4px; font-size: 11px; overflow-x: auto;
  max-height: 200px; white-space: pre-wrap;
}
.actions {
  display: flex; gap: 8px; padding: 12px;
  margin-top: auto;
}
.action-btn {
  flex: 1; padding: 6px 0;
  border: 1px solid #ccc; border-radius: 6px;
  background: #fff; font-size: 11px; cursor: pointer;
}
.action-btn.primary {
  background: #1b1a18; color: #fff; border: none;
}
</style>