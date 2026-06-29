<template>
  <aside class="file-tree-panel">
    <!-- Header -->
    <div class="file-tree-header">
      <span>📁 {{ projectName || '项目' }}</span>
    </div>

    <!-- File Tree -->
    <div class="file-tree-body">
      <FileTreeNode
        v-for="node in files"
        :key="node.name"
        :node="node"
        :depth="0"
        :selected="selected"
        @select="$emit('select', $event)"
        @toggle="$emit('toggle', $event)"
      />
    </div>

    <!-- Git Panel -->
    <div class="git-panel">

      <!-- Git Title -->
      <div class="git-title">Git</div>

      <!-- Branch -->
      <div class="git-branch">
        <Icon icon="mdi:source-branch" width="14" />
        <span class="branch-name">{{ gitStatus.branch || '...' }}</span>
        <span class="branch-dot"></span>
      </div>

      <!-- Changes Count -->
  <div class="git-count">
  {{ (gitStatus.modified?.length || 0) + (gitStatus.untracked?.length || 0) }} changes
</div>

      <!-- Changes List -->
      <div class="git-changes">
   <div class="change-item" v-for="f in (gitStatus.modified || [])" :key="'m-'+f">
          <Icon icon="mdi:file-document-edit-outline" width="12" class="change-icon modified" />
          <span class="change-file">{{ f }}</span>
        </div>
       <div class="change-item" v-for="f in (gitStatus.untracked || [])" :key="'u-'+f">
          <Icon icon="mdi:plus-circle-outline" width="12" class="change-icon added" />
          <span class="change-file">{{ f }}</span>
        </div>
      </div>

      <!-- Commit Input -->
      <input
        v-model="commitMsg"
        class="commit-input"
        placeholder="message..."
        @keyup.enter="gitCommit"
      />

      <!-- Commit Button + Dropdown -->
      <div class="commit-actions">
        <button class="commit-main-btn" @click="gitCommit" :disabled="!commitMsg.trim()">
          Commit
        </button>

        <button class="commit-dropdown-btn" @click="toggleMore">
          ▼
        </button>
      </div>

      <!-- Dropdown Options -->
      <div v-if="showMore" class="commit-more">
        <button class="git-btn" @click="gitAddAll">Add All</button>
        <button class="git-btn" @click="gitPush">Push</button>
      </div>

    </div>
  </aside>
</template>

<script setup>
import { ref, onMounted, defineProps, defineEmits } from 'vue'
import { Icon } from '@iconify/vue'
import FileTreeNode from './FileTreeNode.vue'

const props = defineProps({
  projectName: { type: String, default: '' },
  files: { type: Array, required: true },
  selected: { type: Object, default: null }
})

defineEmits(['select', 'toggle'])
const commitMsg = ref('')
const showMore = ref(false)
function toggleMore() {
  showMore.value = !showMore.value
}
async function gitAddAll() {
  await fetch('/api/git/add-all', { method: 'POST' })
  fetchGitStatus()
}

async function gitCommit() {
  if (!commitMsg.value.trim()) return
  await fetch('/api/git/commit', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message: commitMsg.value.trim() })
  })
  commitMsg.value = ''
  fetchGitStatus()
}

async function gitPush() {
  await fetch('/api/git/push', { method: 'POST' })
  // 可以加个简单的提示
}
// ★ 真实 Git 数据
const gitStatus = ref({ branch: '', modified: [], untracked: [] })

async function fetchGitStatus() {
  try {
    const res = await fetch('/api/git-status')
    if (res.ok) {
      gitStatus.value = await res.json()
    }
  } catch (e) {
    console.error('Git status fetch failed', e)
  }
}

onMounted(() => {
  fetchGitStatus()
  // 可选：每 30 秒自动刷新
  setInterval(fetchGitStatus, 30000)
})
</script>

<style scoped>
/* 样式和之前一模一样，不动 */
.file-tree-panel {
  width: 220px;
  height: 100%;
  border-right: 1px solid #e4dfd4;
  background: #faf9f6;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
.file-tree-header {
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  color: #696259;
  border-bottom: 1px solid #e4dfd4;
}
.file-tree-body {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
  min-height: 0;             /* 关键：允许 flex 子项收缩 */
}
.file-tree-footer {
  margin-top: auto;
  padding: 8px 12px 12px;
  border-top: 1px solid #e4dfd4;
  font-size: 11px;
  color: #696259;
  max-height: 30vh;          /* 最多占 30% 视图高度 */
  overflow-y: auto;          /* 超出滚动 */
  flex-shrink: 0;            /* 不被压缩 */
}
.git-branch {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  color: #1b1a18;
  margin-bottom: 4px;
}
.git-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-left: auto;
}
.git-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0;
}

.git-file {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.git-file-list {
  max-height: 100px;
  overflow-y: auto;
  margin: 4px 0;
}

.git-actions {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
}

.git-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: #fff;
  font-size: 10px;
  cursor: pointer;
}

.git-btn:hover {
  background: #f0ede5;
}

.commit-row {
  display: flex;
  gap: 4px;
}

.commit-input {
  flex: 1;
  border: 1px solid #ccc;
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 10px;
  outline: none;
}

.git-actions {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}

.git-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  background: #fff;
  font-size: 12px;
  color: #4a4540;
  cursor: pointer;
  transition: background 0.15s;
  width: 100%;
  justify-content: center;
}

.git-btn:hover {
  background: #f0ede5;
}

.git-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.commit-row {
  display: flex;
  gap: 4px;
  width: 100%;
}

.commit-input {
  flex: 1;
  min-width: 0;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  padding: 4px 8px;
  font-size: 12px;
  outline: none;
  color: #4a4540;
}

.commit-btn {
  width: 32px;
  height: 32px;
  padding: 0;
  justify-content: center;
  flex-shrink: 0;
}
.git-panel {
  padding: 10px 12px;
  border-top: 1px solid #e4dfd4;
  background: #f7f6f3;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.git-title {
  font-size: 12px;
  font-weight: 600;
  color: #4a473f;
}

.git-branch {
  display: flex;
  align-items: center;
  gap: 6px;
}

.branch-name {
  font-size: 12px;
  font-weight: 600;
}

.branch-dot {
  width: 6px;
  height: 6px;
  background: #12b76a;
  border-radius: 50%;
  margin-left: auto;
}

.git-count {
  font-size: 11px;
  color: #6a665e;
}

.git-changes {
  max-height: 120px;
  overflow-y: auto;
  border-top: 1px solid #e4dfd4;
  padding-top: 6px;
}

.change-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0;
}

.change-file {
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.commit-input {
  width: 100%;
  padding: 6px 8px;
  font-size: 12px;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
}

.commit-actions {
  display: flex;
  gap: 4px;
}

.commit-main-btn {
  flex: 1;
  padding: 6px 8px;
  font-size: 12px;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
}

.commit-dropdown-btn {
  width: 32px;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
}

.commit-more {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.git-btn {
  padding: 6px 8px;
  font-size: 12px;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
}

</style>