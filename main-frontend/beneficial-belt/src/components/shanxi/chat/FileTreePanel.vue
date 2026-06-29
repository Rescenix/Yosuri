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
      <div class="git-title">Git</div>

      <div class="git-branch">
        <Icon icon="mdi:source-branch" width="14" />
        <span class="branch-name">{{ gitStatus.branch || '...' }}</span>
        <span class="branch-dot"></span>
      </div>

      <div class="git-count">
        {{ (gitStatus.modified?.length || 0) + (gitStatus.untracked?.length || 0) }} changes
      </div>

      <div class="git-changes">
        <div class="change-item" v-for="f in (gitStatus.modified || [])" :key="'m-'+f">
          <span class="change-icon modified">M</span>
          <span class="change-file" :title="f">{{ getFileName(f) }}</span>
        </div>
        <div class="change-item" v-for="f in (gitStatus.untracked || [])" :key="'u-'+f">
          <span class="change-icon added">U</span>
        <span class="change-file" :title="f">{{ getFileName(f) }}</span>
        </div>
      </div>

      <div class="commit-input-area">
        <textarea
          v-model="commitMsg"
          class="commit-textarea"
          placeholder="Commit message (支持多行)"
          rows="3"
        ></textarea>
        <button class="commit-more-btn" @click="toggleMore" title="更多操作">
          <Icon icon="mdi:chevron-down" width="16" />
        </button>

        <!-- ★ 悬浮菜单：绝对定位在按钮下方 -->
        <div v-if="showMore" class="floating-menu">
          <button class="git-btn" @click="gitAddAll">Add All</button>
          <button class="git-btn" @click="gitPush">Push</button>
        </div>
      </div>

      <button class="commit-main-btn" @click="gitCommit" :disabled="!commitMsg.trim()">
        Commit
      </button>
    </div>
  </aside>
</template>

<script setup>
import { ref, onMounted } from 'vue'
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
const gitStatus = ref({ branch: '', modified: [], untracked: [] })

function toggleMore() {
  showMore.value = !showMore.value
}

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

async function gitAddAll() {
  await fetch('/api/git/add-all', { method: 'POST' })
  showMore.value = false
  fetchGitStatus()
}
function getFileName(fullPath) {
  return fullPath.split('/').pop() || fullPath
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
  showMore.value = false
}

onMounted(() => {
  fetchGitStatus()
  setInterval(fetchGitStatus, 30000)
})
</script>

<style scoped>
/* ========== 面板整体 ========== */
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
  min-height: 0;
}

/* ========== Git 面板 ========== */
.git-panel {
  padding: 10px 12px;
  border-top: 1px solid #e4dfd4;
  background: #f7f6f3;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 45vh;
  overflow-y: auto;
  box-sizing: border-box;
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
  max-height: 100px;
  overflow-y: auto;
  border-top: 1px solid #e4dfd4;
  padding-top: 4px;
}

.change-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0;
}

.change-icon {
  width: 16px;
  height: 16px;
  font-size: 10px;
  font-weight: 700;
  text-align: center;
  line-height: 16px;
  border-radius: 3px;
  flex-shrink: 0;
}
.change-icon.modified {
  background: #fef3c7;
  color: #b45309;
}
.change-icon.added {
  background: #d1fae5;
  color: #065f46;
}

.change-file {
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: default;
}

/* ========== Commit 输入区域 ========== */
.commit-input-area {
  position: relative;
}

.commit-textarea {
  width: 100%;
  padding: 6px 30px 6px 8px;
  font-size: 12px;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  resize: vertical;
  font-family: inherit;
  box-sizing: border-box;
  background: #fff;
}

.commit-more-btn {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  color: #6a665e;
}
.commit-more-btn:hover {
  background: #e8e3d8;
}

/* ★ 悬浮菜单 */
.floating-menu {
  position: absolute;
  top: 32px;
  right: 4px;
  z-index: 10;
  background: #fff;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  padding: 4px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 100px;
}

.floating-menu .git-btn {
  display: block;
  width: 100%;
  padding: 6px 10px;
  font-size: 12px;
  border: none;
  background: #fff;
  cursor: pointer;
  text-align: left;
  border-radius: 4px;
}
.floating-menu .git-btn:hover {
  background: #f0ede5;
}

/* Commit 主按钮 */
.commit-main-btn {
  width: 100%;
  padding: 6px 0;
  font-size: 12px;
  border: 1px solid #d4cfc4;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
}
.commit-main-btn:hover {
  background: #f0ede5;
}
.commit-main-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>