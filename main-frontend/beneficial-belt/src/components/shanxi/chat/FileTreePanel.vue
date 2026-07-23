<template>
  <aside class="sidebar-panel">
    <!-- Files -->
    <section class="accordion-section">
      <div class="section-header" @click="toggleSection('files')">
        <Icon icon="mdi:chevron-right" width="14" class="chevron" :class="{ rotated: isOpen('files') }" />
        <span class="section-label">FILES</span>
        <button class="section-more-btn" title="刷新" @click.stop="$emit('refresh-tree')">
          <Icon icon="mdi:dots-horizontal" width="14" />
        </button>
      </div>
      <div class="section-body-wrap" :class="{ expanded: isOpen('files') }">
        <div class="section-body">
          <div class="root-row" @click="rootExpanded = !rootExpanded">
            <Icon
              :icon="rootExpanded ? 'mdi:folder-open-outline' : 'mdi:folder-outline'"
              width="16" style="color:#f59e0b; margin-right:6px; flex-shrink:0"
            />
            <span class="root-name">{{ projectName || '项目' }}</span>
          </div>
          <div v-if="rootExpanded" class="tree-body">
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
        </div>
      </div>
    </section>

    <!-- Git -->
    <section class="accordion-section">
      <div class="section-header" @click="toggleSection('git')">
        <Icon icon="mdi:chevron-right" width="14" class="chevron" :class="{ rotated: isOpen('git') }" />
        <span class="section-label">GIT</span>
        <span v-if="gitChangeCount > 0" class="section-badge">{{ gitChangeCount }}</span>
      </div>
      <div class="section-body-wrap" :class="{ expanded: isOpen('git') }">
        <div class="section-body git-body">
          <div class="git-branch-row">
            <span class="branch-name">{{ gitStatus.branch || '...' }}</span>
            <span class="branch-dot"></span>
          </div>

          <div class="git-changes">
            <div class="change-item" v-for="f in (gitStatus.modified || [])" :key="'m-' + f">
              <span class="change-icon modified">M</span>
              <span class="change-file" :title="f">{{ getFileName(f) }}</span>
            </div>
            <div class="change-item" v-for="f in (gitStatus.untracked || [])" :key="'u-' + f">
              <span class="change-icon added">U</span>
              <span class="change-file" :title="f">{{ getFileName(f) }}</span>
            </div>
            <div v-if="gitChangeCount === 0" class="clean-msg">工作区干净</div>
          </div>

          <div class="commit-input-area">
            <textarea
              v-model="commitMsg"
              class="commit-textarea"
              placeholder="Commit message (支持多行)"
              rows="3"
            ></textarea>
            <button class="commit-more-btn" @click.stop="toggleMore" title="更多操作">
              <Icon icon="mdi:chevron-down" width="16" />
            </button>
            <div v-if="showMore" class="floating-menu" @click.stop>
              <button class="git-btn" @click="gitAddAll">Add All</button>
              <button class="git-btn" @click="gitPush">Push</button>
            </div>
          </div>

          <div v-if="actionFeedback.show" class="action-feedback" :class="actionFeedback.type">
            {{ actionFeedback.message }}
          </div>
          <button v-else class="commit-main-btn" @click="gitCommit" :disabled="!commitMsg.trim()">
            Commit
          </button>
        </div>
      </div>
    </section>

    <!-- Pinned -->
    <section class="accordion-section" v-if="pinnedFiles.length">
      <div class="section-header" @click="toggleSection('pinned')">
        <Icon icon="mdi:chevron-right" width="14" class="chevron" :class="{ rotated: isOpen('pinned') }" />
        <Icon icon="mdi:pin-outline" width="13" style="margin-right:4px; opacity:.75" />
        <span class="section-label">PINNED</span>
        <span class="section-badge">{{ pinnedFiles.length }}</span>
      </div>
      <div class="section-body-wrap" :class="{ expanded: isOpen('pinned') }">
        <div class="section-body">
          <div class="pinned-row" v-for="f in pinnedFiles" :key="f.path">
            <span
              class="file-badge"
              :style="{ background: fileBadge(f.name).bg, color: fileBadge(f.name).color }"
            >{{ fileBadge(f.name).label }}</span>
            <span class="pinned-name" :title="f.path" @click="$emit('select', f)">{{ f.name }}</span>
            <button class="unpin-btn" title="取消固定" @click="$emit('unpin-file', f)">
              <Icon icon="mdi:close" width="12" />
            </button>
          </div>
        </div>
      </div>
    </section>
  </aside>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import FileTreeNode from './FileTreeNode.vue'

const props = defineProps({
  projectName: { type: String, default: '' },
  files: { type: Array, required: true },
  selected: { type: Object, default: null },
  pinnedFiles: { type: Array, default: () => [] }
})

defineEmits(['select', 'toggle', 'unpin-file', 'refresh-tree'])

const rootExpanded = ref(true)
const openSections = ref(new Set(['files', 'git']))

function isOpen(name) {
  return openSections.value.has(name)
}
function toggleSection(name) {
  const next = new Set(openSections.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  openSections.value = next
}

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

function getFileName(fullPath) {
  return fullPath.split('/').pop() || fullPath
}

// ---- Git (merged from former GitPanel.vue) ----
const commitMsg = ref('')
const showMore = ref(false)
const gitStatus = ref({ branch: '', modified: [], untracked: [] })
const gitChangeCount = computed(
  () => (gitStatus.value.modified?.length || 0) + (gitStatus.value.untracked?.length || 0)
)

function toggleMore() {
  showMore.value = !showMore.value
}

async function fetchGitStatus() {
  try {
    const res = await fetch('/api/git-status')
    if (res.ok) gitStatus.value = await res.json()
  } catch (e) {
    console.error('Git status fetch failed', e)
  }
}

const actionFeedback = ref({ show: false, message: '', type: 'success' })
let feedbackTimer = null
function showFeedback(msg, type = 'success') {
  actionFeedback.value = { show: true, message: msg, type }
  clearTimeout(feedbackTimer)
  feedbackTimer = setTimeout(() => { actionFeedback.value.show = false }, 2500)
}

async function gitAddAll() {
  try {
    const res = await fetch('/api/git/add-all', { method: 'POST' })
    showFeedback(res.ok ? 'git add -A 成功' : 'Add 失败', res.ok ? 'success' : 'error')
  } catch (e) {
    showFeedback('网络错误', 'error')
  }
  showMore.value = false
  fetchGitStatus()
}

async function gitCommit() {
  if (!commitMsg.value.trim()) return
  try {
    const res = await fetch('/api/git/commit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: commitMsg.value.trim() })
    })
    if (res.ok) {
      showFeedback('Commit 成功')
      commitMsg.value = ''
    } else {
      showFeedback(`Commit 失败: ${await res.text()}`, 'error')
    }
  } catch (e) {
    showFeedback('网络错误', 'error')
  }
  showMore.value = false
  fetchGitStatus()
}

async function gitPush() {
  actionFeedback.value = { show: true, message: 'Pushing...', type: 'loading' }
  showMore.value = false
  try {
    const res = await fetch('/api/git/push', { method: 'POST' })
    actionFeedback.value = res.ok
      ? { show: true, message: 'Push 成功', type: 'success' }
      : { show: true, message: `Push 失败: ${await res.text()}`, type: 'error' }
  } catch (e) {
    actionFeedback.value = { show: true, message: '网络错误', type: 'error' }
  }
  clearTimeout(feedbackTimer)
  feedbackTimer = setTimeout(() => { actionFeedback.value.show = false }, 2500)
}

function handleClickOutside() {
  if (showMore.value) showMore.value = false
}

let statusPoll = null
onMounted(() => {
  fetchGitStatus()
  statusPoll = setInterval(fetchGitStatus, 30000)
  document.addEventListener('click', handleClickOutside)
})
onUnmounted(() => {
  clearInterval(statusPoll)
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.sidebar-panel {
  width: 240px;
  height: 100%;
  border-right: 1px solid var(--app-border);
  background: #f8fafc;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow-y: auto;
}

/* ========== Accordion ========== */
.accordion-section {
  flex-shrink: 0;
  border-bottom: 1px solid #eee7da;
}

.section-header {
  height: 34px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 10px;
  cursor: pointer;
  user-select: none;
}
.section-header:hover { background: #f1f5f9; }

.chevron { transition: transform 180ms ease; flex-shrink: 0; color: var(--app-text-faint); }
.chevron.rotated { transform: rotate(90deg); }

.section-label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.6px;
  color: var(--app-text-soft);
  flex: 1;
}

.section-badge {
  font-size: 10px;
  font-weight: 700;
  background: #e8935a;
  color: #fff;
  border-radius: 8px;
  padding: 1px 6px;
  line-height: 1.4;
}

.section-more-btn {
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  cursor: pointer;
  display: flex;
  align-items: center;
  border-radius: 4px;
  padding: 2px;
}
.section-more-btn:hover { background: #e2e8f0; }

/* Smooth accordion: grid-rows 0fr -> 1fr, no display:none jump */
.section-body-wrap {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 220ms ease;
}
.section-body-wrap.expanded {
  grid-template-rows: 1fr;
}
.section-body-wrap > .section-body {
  overflow: hidden;
  min-height: 0;
}

/* ========== Files ========== */
.root-row {
  display: flex;
  align-items: center;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  color: var(--app-text);
}
.root-row:hover { background: #f1f5f9; }
.tree-body { padding: 2px 0 6px; }

/* ========== Git ========== */
.git-body {
  padding: 8px 12px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.git-branch-row { display: flex; align-items: center; gap: 6px; }
.branch-name { font-size: 12px; font-weight: 600; font-family: "JetBrains Mono", monospace; }
.branch-dot { width: 6px; height: 6px; background: #12b76a; border-radius: 50%; margin-left: auto; }

.git-changes {
  max-height: 100px;
  overflow-y: auto;
}
.clean-msg { color: #12b76a; font-size: 11px; padding: 2px 0; }
.change-item { display: flex; align-items: center; gap: 6px; padding: 2px 0; }
.change-icon {
  width: 16px; height: 16px; font-size: 10px; font-weight: 700;
  text-align: center; line-height: 16px; border-radius: 3px; flex-shrink: 0;
}
.change-icon.modified { background: #fef3c7; color: #b45309; }
.change-icon.added { background: #d1fae5; color: #065f46; }
.change-file {
  font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: default;
}

.commit-input-area { position: relative; }
.commit-textarea {
  width: 100%;
  padding: 6px 30px 6px 8px;
  font-size: 12px;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  resize: vertical;
  font-family: inherit;
  box-sizing: border-box;
  background: var(--app-surface);
}
.commit-more-btn {
  position: absolute; top: 4px; right: 4px;
  width: 24px; height: 24px; border: none; background: transparent;
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  border-radius: 4px; color: var(--app-text-soft);
}
.commit-more-btn:hover { background: #e2e8f0; }

.floating-menu {
  position: absolute; top: -70px; right: -10px; z-index: 10;
  background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1); padding: 4px;
  display: flex; flex-direction: column; gap: 2px; min-width: 100px;
}
.floating-menu .git-btn {
  display: block; width: 100%; padding: 6px 10px; font-size: 12px;
  border: none; background: var(--app-surface); cursor: pointer; text-align: left; border-radius: 4px;
}
.floating-menu .git-btn:hover { background: #f1f5f9; }

.commit-main-btn, .action-feedback {
  display: block; width: 100%; padding: 6px 0; font-size: 12px;
  border: 1px solid var(--app-border); border-radius: 6px; text-align: center;
  box-sizing: border-box; line-height: 1.4; cursor: pointer;
  background: var(--app-surface); color: inherit; transition: background 0.15s, border-color 0.15s;
}
.commit-main-btn:hover { background: #f1f5f9; }
.commit-main-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.action-feedback.success { background: #d1fae5; color: #065f46; border-color: #a7f3d0; }
.action-feedback.error { background: #fee2e2; color: #991b1b; border-color: #fecaca; }
.action-feedback.loading { background: #f1f5f9; color: var(--app-text-soft); border-color: var(--app-border); }
.action-feedback.loading::after { content: '...'; animation: loading-dots 1.5s infinite; }
@keyframes loading-dots {
  0% { content: '.'; } 33% { content: '..'; } 66% { content: '...'; }
}

/* ========== Pinned ========== */
.pinned-row {
  display: flex; align-items: center; gap: 6px;
  padding: 4px 10px; font-size: 12px; cursor: default;
}
.pinned-row:hover { background: #f1f5f9; }
.pinned-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
.unpin-btn {
  border: none; background: transparent; color: var(--app-text-faint); cursor: pointer;
  display: flex; align-items: center; border-radius: 3px; flex-shrink: 0;
}
.unpin-btn:hover { background: #e2e8f0; color: var(--app-text); }

.file-badge {
  flex-shrink: 0; width: 16px; height: 16px; border-radius: 4px;
  font-size: 8.5px; font-weight: 700; line-height: 16px; text-align: center;
  font-family: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
