<template>
  <div class="diff-panel">
    <!-- 顶部：分支 → working tree + 搜索 + 刷新 -->
    <div class="diff-toolbar">
      <div class="diff-branch-row">
        <Icon icon="mdi:source-branch" width="13" color="#94a3b8" />
        <span class="diff-branch">{{ branch || '...' }}</span>
        <Icon icon="mdi:arrow-right-thin" width="14" color="#a3a3a3" />
        <span class="diff-worktree">working tree</span>
        <span class="diff-totals">
          <span class="diff-adds">+{{ totals.add }}</span>
          <span class="diff-dels">−{{ totals.del }}</span>
        </span>
        <button class="diff-refresh-btn" @click="fetchList" title="刷新">
          <Icon icon="mdi:refresh" width="14" :class="{ 'diff-spin': listLoading }" />
        </button>
      </div>
      <div class="diff-search-wrap">
        <Icon icon="mdi:magnify" width="13" color="#a3a3a3" />
        <input
          v-model="searchQuery"
          class="diff-search-input"
          type="text"
          placeholder="搜索文件名定位…"
          spellcheck="false"
        />
        <button v-if="searchQuery" class="diff-search-clear" @click="searchQuery = ''">
          <Icon icon="mdi:close" width="12" />
        </button>
      </div>
    </div>

    <div v-if="!listLoading && filteredFiles.length === 0" class="diff-empty">
      <Icon icon="mdi:file-compare" width="24" color="#c4c4c4" />
      <span>{{ files.length === 0 ? '工作树没有未提交改动' : '没有匹配的文件' }}</span>
    </div>

    <div v-else class="diff-body">
      <div class="diff-file-card" v-for="df in filteredFiles" :key="df.path">
        <div class="diff-file-head" @click="toggleFile(df)">
          <span class="diff-chev" :class="{ open: !!expanded[df.path] }">›</span>
          <span class="diff-status-badge" :class="'st-' + df.status">{{ df.status }}</span>
          <span class="diff-file-name" :title="df.path">
            <span class="diff-file-dir" v-if="fileDir(df.path)">{{ fileDir(df.path) }}/</span>{{ fileBaseName(df.path) }}
          </span>
          <span class="diff-adds">+{{ df.additions }}</span>
          <span class="diff-dels">−{{ df.deletions }}</span>
        </div>
        <div v-if="expanded[df.path]" class="diff-rows">
          <div v-if="contentLoading[df.path]" class="diff-file-hint">加载 diff…</div>
          <div v-else-if="contents[df.path]?.binary" class="diff-file-hint">二进制文件，不展示 diff</div>
          <div v-else-if="contents[df.path]?.too_large" class="diff-file-hint">文件过大（&gt;300KB），不展示 diff</div>
          <DiffViewer
            v-else-if="contents[df.path]"
            :old-content="contents[df.path].old_content"
            :new-content="contents[df.path].new_content"
            :path="df.path"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { fileBaseName } from './toolArgs.js'
import DiffViewer from './DiffViewer.vue'

// git 工作树全量 diff：列表秒出（只有元数据），文件内容点击展开时按需拉取
const branch = ref('')
const files = ref([])
const listLoading = ref(true)
const searchQuery = ref('')
const expanded = reactive({})
const contents = reactive({})
const contentLoading = reactive({})

async function fetchList() {
  listLoading.value = true
  try {
    const res = await fetch('/api/git/working-diff')
    const data = await res.json()
    branch.value = data.branch || ''
    files.value = data.files || []
  } catch {
    files.value = []
  } finally {
    listLoading.value = false
  }
}
onMounted(fetchList)

const filteredFiles = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return files.value
  return files.value.filter(f => f.path.toLowerCase().includes(q))
})

const totals = computed(() => {
  let add = 0, del = 0
  for (const f of files.value) { add += f.additions; del += f.deletions }
  return { add, del }
})

function fileDir(path) {
  const i = path.lastIndexOf('/')
  return i > 0 ? path.slice(0, i) : ''
}

async function toggleFile(df) {
  expanded[df.path] = !expanded[df.path]
  if (!expanded[df.path] || contents[df.path] || df.binary) return
  contentLoading[df.path] = true
  try {
    const res = await fetch(`/api/git/working-diff/file?path=${encodeURIComponent(df.path)}`)
    contents[df.path] = await res.json()
  } catch {
    contents[df.path] = { old_content: '', new_content: '', binary: false, too_large: false }
  } finally {
    contentLoading[df.path] = false
  }
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

/* ---------- 顶部工具栏 ---------- */
.diff-toolbar {
  flex-shrink: 0;
  padding: 8px 10px 6px;
  border-bottom: 1px solid #ececec;
}
.diff-branch-row {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 7px;
}
.diff-branch, .diff-worktree {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  color: #1e293b;
  font-weight: 600;
}
.diff-worktree { color: #94a3b8; font-weight: 500; }
.diff-totals { flex: 1; text-align: right; display: flex; gap: 6px; justify-content: flex-end; }
.diff-refresh-btn {
  display: inline-flex;
  align-items: center;
  border: none;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  border-radius: 5px;
  padding: 2px;
}
.diff-refresh-btn:hover { background: #f0f0f0; }
.diff-spin { animation: diff-rotate 0.9s linear infinite; }
@keyframes diff-rotate { from { transform: rotate(0); } to { transform: rotate(360deg); } }

.diff-search-wrap {
  display: flex;
  align-items: center;
  gap: 5px;
  background: #fff;
  border: 1px solid #e5e5e5;
  border-radius: 7px;
  padding: 4px 8px;
}
.diff-search-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12px;
  color: #1e293b;
}
.diff-search-clear {
  display: inline-flex;
  border: none;
  background: transparent;
  color: #a3a3a3;
  cursor: pointer;
  padding: 1px;
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
  margin-bottom: 8px;
  overflow: hidden;
  background: #ffffff;
}
.diff-file-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 12px;
  cursor: pointer;
  background: #f8fafc;
}
.diff-chev {
  display: inline-block;
  font-size: 12px;
  color: #a3a3a3;
  transition: transform 0.15s ease;
}
.diff-chev.open { transform: rotate(90deg); }
.diff-status-badge {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.st-M { background: #d99c2b; }
.st-A, .st-U { background: #12b76a; }
.st-D { background: #d94834; }
.st-R, .st-C { background: #8b5cf6; }
.diff-file-name {
  flex: 1;
  min-width: 0;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  direction: rtl;      /* 路径太长时优先保住文件名尾部 */
  text-align: left;
}
.diff-file-dir { color: #a3a3a3; font-weight: 400; }
.diff-adds, .diff-dels {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
.diff-adds { color: #12b76a; }
.diff-dels { color: #d94834; }

.diff-rows { border-top: 1px solid #e5e5e5; }
.diff-file-hint {
  padding: 12px;
  font-size: 12px;
  color: #a3a3a3;
}
</style>
